package ops

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/kit/durable"
)

// ---------------------------------------------------------------- the ground the editor already paid (#777, refs #655, #776)
//
// A board raise pays the missions it stands for (Solari, House Scrip, specialization XP), the reward rows of the tiers it crosses and, while the house is open, the same ground
// again on the faction's progress. Since #776 the editor writes no progress event, so the game's own record no longer says what the editor paid, and a tier removal followed by the
// same raise paid everything again. The editor therefore remembers the ground it paid itself, outside the game's save and outside the game's tables: a small file of its own next to
// its other state files (the journal, the write log), written only after the edit that paid it is in the game's file.
//
//	file          landsraad-paid-<hash of the save's path>.json in the editor's own folder; the hash is the one the journal's name uses
//	identity      the save's path (lower case) is inside the file and must be this save's, else the file is ignored; each entry carries the character (player controller) too
//	key           term + house + faction + character; the value is the highest contribution whose ground the editor paid there (a tier's threshold). A lookup takes the highest
//	              under any faction: the ground was paid to the character, whatever faction it was in
//	written       through the repo's durable replace (temp file, flush, rename, folder flush), by the sink of the save's notes, after the file of the game was written and never before
//	read          a file that is missing, not this format or of another save is read as "nothing paid": the guards of #776 (the character's own events, the ground
//	              under another faction) still apply, and a raise then pays as it did before this file existed. A read never repairs, moves or deletes it. An entry that is not
//	              plausible (no term, no house, a ground that is not a number or beyond any tier) is left out. A file of a NEWER version (a later build wrote it: a version above
//	              1, or one that is not a whole number, of any size) is not read as nothing: a raise is refused and nothing is written or replaced, so the newer record survives until a
//	              build that reads it, or the person, deals with it. A file larger than paidMaxBytes whose version key is cut by that size (its value, or the key itself, straddles
//	              the byte after the cap) counts as newer too: what it says is not known. A file that is whole JSON must carry its version in the same whole (a byte order mark or text
//	              after the object makes it damaged, not newer)
//	versions      paidFileVersion is the version this build writes. A file of an older version (1 up to paidFileVersion-1) is brought up to date by the steps of paidMigrations, one
//	              version at a time, before it is used; an older version with no step is not read and not replaced (as a newer one). Version 0, a negative one or none is damaged.
//	              Whoever raises paidFileVersion adds the step of the version it leaves in the same change (a test fails otherwise)
//	a file kept   a file that is damaged, larger than paidMaxBytes with no version that is newer, of another save or of a version below 1 is not destroyed by the next write: it is
//	              moved aside (its name + ".damaged-" + the time, in the same folder, by the durable rename) before the new file is written, and the write is an error if it cannot be moved
//	              (#811). The newest paidKeepAside copies are kept, older ones are removed, and the person is told of the copy in the state of the page (Notice)
//	a removal     lowers the contribution and nothing here: the record is what was paid, not what stands
//	a restore     is not signalled: a backup put back (by the editor or by hand) is told apart from the file it replaced only by the cap below, which cuts the record to the rows the restored file shows and keeps it where they stand (an undo of a restore must keep it)
//	trusted only  as far as the save shows it (see standingTier): the record is capped at the highest reward tier of the house whose item has a reward row of this character written in
//	              this term. A save put back by other means (a backup copied by hand, a game rollback, another character in the same file) then has no row for what the record says, and
//	              the ground above the cap is paid as if the record were not there. When a raise pays under a record that was capped, its note REPLACES what was recorded for the house
//	              (the old high-water mark is of a save that is gone) instead of being merged into it.
//	terms         the term is part of the key, so a write in another term (a backup of an earlier term put back and raised once) leaves this term's entries alone; only the bound of
//	              4,096 entries evicts, the oldest terms first and never the term being written
//
// The cap is an upper bound and not a proof: the same item at two tiers, or a row the game wrote for the same item, can stand for a tier it did not pay.

const (
	paidNoteKind    = "landsraad.paid" // the kind of the note a raise attaches to its edit (save.Mut.Note)
	paidFileVersion = 1
	paidMaxEntries  = 4096
	paidMaxHouseLen = 80
	// paidMaxEntryBytes bounds one entry as the editor writes it: a house name of paidMaxHouseLen bytes that JSON escapes to 6 bytes a byte (paidMaxHouseLen*6 + 2), the seven keys and
	// separators (about 80), and six numbers of at most 24 characters each (about 150). 768 holds the worst case; a test builds it. paidMaxHeaderBytes is the room for the rest of the file:
	// the version and the save's path (at most 32,767 characters on Windows, each escaped to at most 6 bytes).
	paidMaxEntryBytes  = 768
	paidMaxHeaderBytes = 256 << 10
	// paidMaxBytes is the size of a file the editor reads: the most paidMaxEntries entries of the worst case can come to, so a file the editor wrote itself is never "too large" (#838, #840).
	paidMaxBytes = paidMaxEntries*paidMaxEntryBytes + paidMaxHeaderBytes
)

// paidReadTries and paidReadPause are how long a read tries a file that cannot be read (another program holds it for a moment) before it gives up: two tries and one pause of 50 ms, because
// the save's lock is held while it waits. A test sets the pause to 0.
var (
	paidReadTries = 2
	paidReadPause = 50 * time.Millisecond
)

// paidOpen opens the file and paidReads counts the reads; a test replaces the first to make a read fail and looks at the second. Production code never changes them.
var (
	paidOpen  = os.Open
	paidReads atomic.Int64
)

// paidNote is what a raise asks to be remembered, and one entry of the file.
type paidNote struct {
	Player  int64   `json:"player"`
	Term    int64   `json:"term"`
	House   string  `json:"house"`
	Faction int64   `json:"faction"`
	Paid    float64 `json:"paid"`
	// Back is the highest tier whose waiting reward rows the editor took back on purpose with a tier removal (#944): the ground below it was paid and the rows are gone on purpose, so the cap of
	// trustedPaid does not cut the record there. It is 0 (and left out of the file) when no take-back happened. Only ever raised: the highest value recorded for the house counts, whatever the faction.
	Back float64 `json:"back,omitempty"`
	// Floor is the ground the editor started paying from on the house (the ground its first raise there began at, the character's own contribution or what its own missions covered): the
	// tiers above it are the ones the record vouches for as the editor's own, and the tiers at or below it may hold a reward the game wrote (#944). Absent (nil) means unknown: a record of an
	// older build, or one an older build rewrote, and then no tier is vouched for. Merged by the lowest value; a raise that begins above everything recorded (a gap) lifts it to where that raise began.
	Floor *float64 `json:"floor,omitempty"`
	// Replace, on a note, says that what was recorded for the character on the house in the term is of a save that is gone and is dropped before this is recorded. It is not stored.
	Replace bool `json:"replace,omitempty"`
}

// valid says whether an entry is plausible: a term, a character, a house and a ground that is a number from above 0 to the largest tier a request may name.
func (e paidNote) valid() bool {
	return e.Term > 0 && e.Player > 0 && e.House != "" && len(e.House) <= paidMaxHouseLen && !math.IsNaN(e.Paid) && e.Paid > 0 && e.Paid <= maxTier &&
		!math.IsNaN(e.Back) && e.Back >= 0 && e.Back <= maxTier &&
		(e.Floor == nil || (!math.IsNaN(*e.Floor) && *e.Floor >= 0 && *e.Floor <= maxTier))
}

// norm drops a floor that is above the ground paid: such a start says nothing a reader can use, and no tier is then vouched for.
func (e paidNote) norm() paidNote {
	if e.Floor != nil && *e.Floor > e.Paid {
		e.Floor = nil
	}
	return e
}

type paidFile struct {
	Version int        `json:"version"`
	Save    string     `json:"save"` // the save's path, lower case: the file is ignored for any other save
	Entries []paidNote `json:"entries"`
}

// PaidLedger is the editor's own record of the Landsraad ground it paid, in one file.
type PaidLedger struct {
	path   string
	save   string
	mu     sync.Mutex
	opts   durable.Options
	notice atomic.Value // string: what the person is told about a file that was moved aside, for the state of the page (Notice); read without the lock
}

// paidKeepAside is how many files moved aside (".damaged-*", see moveAside) are kept next to the record: the newest ones. Older copies are removed when a new one is made.
const paidKeepAside = 10

// Notice is what the person should know about the record: "" while there is nothing, else a sentence that names a file moved aside (its name, not its folder). It is set when a record
// that was damaged, too large, of another save or of a version below 1 is kept aside and a new one started, and lasts until the editor is restarted. A nil ledger has none.
func (l *PaidLedger) Notice() string {
	if l == nil {
		return ""
	}
	s, _ := l.notice.Load().(string)
	return s
}

// NewPaidLedger keeps the record in path for the save at savePath ("" for path gives a ledger that records and reads nothing).
func NewPaidLedger(path, savePath string) *PaidLedger {
	return &PaidLedger{path: path, save: strings.ToLower(savePath), opts: durable.Defaults()}
}

// PaidLedgerName is the name of the file for a save, from the hash the journal's name uses (h is the first six bytes of the hash, as hex).
func PaidLedgerName(hash string) string { return "landsraad-paid-" + hash + ".json" }

var errPaidFile = errors.New("the Landsraad paid-ground file is not usable")

// errPaidNewer is a file this build cannot tell what it holds: its version is above paidFileVersion (a later build of the editor wrote it), is not a whole number, is cut by the size
// limit, or is below paidFileVersion with no step that brings it up to date. It is not read as "nothing paid" and is never replaced (#811).
var errPaidNewer = errors.New("the Landsraad paid-ground file was written by a newer version of the editor")

// errPaidNewerRefused is what a raise is refused with when the record is of a newer version: paying on, as if nothing had been paid, could pay the same ground twice, and the next write
// would replace the newer record.
var errPaidNewerRefused = errors.New("the editor's own record of the Landsraad ground it paid was written by a newer version of the editor (or in a format this version cannot bring up to date), so this raise was refused and nothing was written. Use the newer version, or move the file landsraad-paid-*.json in the editor's own folder aside (the raise then pays as if nothing had been recorded)")

// paidClock is the time the name of a file that is moved aside carries; a test replaces it.
var paidClock = time.Now

// paidMigrations brings a record of one version up to the next: paidMigrations[v] turns a file of version v into one of version v+1 (its Version set to v+1). It is empty while
// paidFileVersion is 1. The change that raises paidFileVersion adds the step for the version it leaves (TestPaidEveryOlderVersionHasAStep fails otherwise), so a record of the old
// version is carried over, and is not read as damaged, moved aside and paid again (#838). A step must keep what the record vouches for: it may only lower what it says, never raise a
// ground, a take-back ceiling or a floor.
var paidMigrations = map[int]func(paidFile) (paidFile, error){}

// migratePaid brings pf up to version to by steps, one version at a time. A version with no step, or a step that fails or does not move the version by one, is an error that wraps
// errPaidNewer: the file is neither read nor replaced.
func migratePaid(pf paidFile, to int, steps map[int]func(paidFile) (paidFile, error)) (paidFile, error) {
	for pf.Version < to {
		step, ok := steps[pf.Version]
		if !ok {
			return pf, fmt.Errorf("%w (version %d, and this build has no step that reads it)", errPaidNewer, pf.Version)
		}
		next, err := step(pf)
		if err != nil {
			return pf, fmt.Errorf("%w (the step from version %d failed: %v)", errPaidNewer, pf.Version, err)
		}
		if next.Version != pf.Version+1 {
			return pf, fmt.Errorf("%w (the step from version %d did not lead to the next version)", errPaidNewer, pf.Version)
		}
		pf = next
	}
	return pf, nil
}

// read returns the valid entries of the file. A file that does not exist, or is not a whole file of this format for this save, is an error of its own kind (errPaidFile or
// os.ErrNotExist): the caller reads that as "nothing paid". A file that carries a version this build does not read as its own (above paidFileVersion, or not a whole number, or one
// that the size limit cuts), however large it is, is errPaidNewer: it is neither read nor replaced. Any other error is a file that could not be read this time.
func (l *PaidLedger) read() ([]paidNote, error) {
	paidReads.Add(1)
	f, err := paidOpen(l.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, paidMaxBytes+1)) // never more than the cap is held
	if err != nil {
		return nil, err
	}
	over := len(b) > paidMaxBytes
	// the version first, from the one field, before the size verdict: a later build may have changed the rest of the format (or made the file larger), and such a file must not read
	// as damaged. A file over the cap is probed in the part that was read: a version that is there and is not read the same way is a newer one, and so is one that the cut leaves
	// open (a number that may go on, a string or list that is not closed, a key cut in two, #840).
	if over || json.Valid(b) {
		v, ok, cut := paidVersionOf(b, over)
		if cut {
			return nil, fmt.Errorf("%w (its version is cut by the size limit of %d bytes)", errPaidNewer, paidMaxBytes)
		}
		if ok && paidVersionIsNewer(v) {
			return nil, fmt.Errorf("%w (version %.40q)", errPaidNewer, string(v))
		}
	}
	if over {
		return nil, fmt.Errorf("%w: it is larger than %d bytes", errPaidFile, paidMaxBytes)
	}
	var pf paidFile
	if err := json.Unmarshal(b, &pf); err != nil {
		return nil, fmt.Errorf("%w: %v", errPaidFile, err)
	}
	if pf.Save != l.save || pf.Version < 1 {
		return nil, fmt.Errorf("%w: another version or another save", errPaidFile)
	}
	if pf.Version > paidFileVersion { // not reachable (paidVersionIsNewer says so first); never read as damaged
		return nil, fmt.Errorf("%w (version %d)", errPaidNewer, pf.Version)
	}
	if pf, err = migratePaid(pf, paidFileVersion, paidMigrations); err != nil {
		return nil, err
	}
	out := pf.Entries[:0:0]
	for _, e := range pf.Entries {
		if e.valid() {
			e.Replace = false
			out = append(out, e.norm())
		}
	}
	return out, nil
}

// paidVersionOf scans the top-level keys of an object in b and returns the raw value of the last key that reads as "version" (any letter case, as the decoder matches field names).
// ok is false when b is not an object or has no such key in the part that can be read; a scan that ends early (b is cut off) keeps what it found before. capped says that b is the
// first part of a larger file: cut is then true when the scan ended inside what may be the version, so that it is not known (the version key's value is not whole, or ends where b
// ends and may go on, or the key itself is cut in two). A version that lies wholly past the end of b is not seen at all.
func paidVersionOf(b []byte, capped bool) (v json.RawMessage, ok, cut bool) {
	dec := json.NewDecoder(bytes.NewReader(b))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, false, false
	}
	for dec.More() {
		off := dec.InputOffset()
		k, err := dec.Token()
		if err != nil {
			return v, ok, capped && keyMayBeVersion(b[off:])
		}
		key, isKey := k.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil || !isKey {
			return v, ok, capped && isKey && err != nil && strings.EqualFold(key, "version")
		}
		if strings.EqualFold(key, "version") {
			v, ok = raw, true
			cut = capped && dec.InputOffset() >= int64(len(b)) // the value ends where the bytes end: a number may go on (1|0, 1|.5)
		}
	}
	return v, ok, cut
}

// keyMayBeVersion says whether rest, the bytes from where a key should begin to the end of what was read, is the start of a key that may be "version": it opens a string that is not
// closed and whose text so far is a start of "version" (any letter case), is empty, or has an escape or a byte that is not ASCII (it cannot be told).
func keyMayBeVersion(rest []byte) bool {
	rest = bytes.TrimLeft(rest, " \t\r\n,")
	if len(rest) == 0 || rest[0] != '"' {
		return false
	}
	part := rest[1:]
	for _, c := range part {
		if c == '\\' || c >= 0x80 {
			return true
		}
		if c == '"' {
			return false // the key is whole
		}
	}
	return len(part) <= len("version") && strings.HasPrefix("version", strings.ToLower(string(part)))
}

// paidVersionIsNewer says whether a version value is one this build must not treat as damaged: anything that is not a whole number at or below paidFileVersion (a larger number,
// a number too large to hold, a fraction or an exponent, text, null, a list). A whole number at or below it (0, 1, a negative one) is left to the checks that follow. It looks at the
// digits and builds no number, so a version of a million digits costs one pass over it (#840).
func paidVersionIsNewer(v json.RawMessage) bool {
	s := string(bytes.TrimSpace(v))
	neg := strings.HasPrefix(s, "-")
	digits := strings.TrimPrefix(s, "-")
	if digits == "" {
		return true
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			return true // not a whole number
		}
	}
	if neg {
		return false // a negative whole number (or -0) is at or below paidFileVersion
	}
	digits = strings.TrimLeft(digits, "0")
	if len(digits) > 9 {
		return true // far above any version, and not worth turning into a number
	}
	if digits == "" {
		return false // 0
	}
	n, err := strconv.Atoi(digits)
	return err != nil || n > paidFileVersion
}

// readRetry reads the file, trying again once when it could not be read this time. A file that does not exist reads as nothing recorded (nil, nil). A file that is not a whole file of
// this format for this save (errPaidFile) or is of a newer version (errPaidNewer) is returned as that error, for the caller to decide: a read takes the first as nothing paid, a write
// moves it aside, and the newer one is never replaced. Any other failure is an error too, so nothing is built on, or from, a record that was not seen.
func (l *PaidLedger) readRetry() ([]paidNote, error) {
	var es []paidNote
	var err error
	for try := 0; try < paidReadTries; try++ {
		if try > 0 {
			time.Sleep(paidReadPause)
		}
		es, err = l.read()
		if err == nil {
			return es, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		if errors.Is(err, errPaidFile) || errors.Is(err, errPaidNewer) {
			return nil, err
		}
	}
	return nil, err
}

// moveAside renames a file that is of no use to this build (damaged, too large, of another save, or of a version below 1) to a name next to it, so that the next write does not destroy
// it (#811), and returns the base name it was kept under ("" when the file was gone by then: there is nothing to keep). The rename is the durable one with the ledger's options (#837): a
// program that holds the file for a moment (a scanner) is waited for as a write of the record is, and the folder is flushed after it. A name already taken is not replaced (the time has
// microseconds, and a counter follows when two files are moved in the same instant). Two editors on one save are kept out by the save's lock (save.LockSave), so nothing else moves or
// rewrites this file between the read that found it unusable and this rename.
func (l *PaidLedger) moveAside() (string, error) {
	stamp := paidClock().UTC().Format("20060102T150405.000000")
	to := l.path + ".damaged-" + stamp
	for i := 1; ; i++ {
		if _, err := os.Lstat(to); errors.Is(err, os.ErrNotExist) {
			break
		}
		if i > 99 {
			return "", fmt.Errorf("no free name to keep the old file under (%s)", filepath.Base(to))
		}
		to = fmt.Sprintf("%s.damaged-%s-%d", l.path, stamp, i)
	}
	o := l.opts
	o.Retry = func(err error) bool { return !errors.Is(err, os.ErrNotExist) } // a file that is gone is not waited for
	if err := durable.Rename(l.path, to, o); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	if o.SyncDir {
		_ = durable.SyncDir(filepath.Dir(l.path)) // best effort, as WriteFile does: the rename is done
	}
	l.pruneAside(to)
	return filepath.Base(to), nil
}

// pruneAside removes the oldest of the files moved aside next to the record, so that the newest paidKeepAside stay (#838). The names carry the time (and a counter) and sort in the
// order they were made; keep is the copy just made and is never removed. A copy that cannot be removed is left (it is logged) and does not fail the write.
func (l *PaidLedger) pruneAside(keep string) {
	entries, err := os.ReadDir(filepath.Dir(l.path))
	if err != nil {
		return
	}
	prefix := filepath.Base(l.path) + ".damaged-"
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for len(names) > paidKeepAside {
		old := names[0]
		names = names[1:]
		if old == filepath.Base(keep) {
			continue
		}
		if err := os.Remove(filepath.Join(filepath.Dir(l.path), old)); err != nil {
			log.Printf("an old copy of the Landsraad paid-ground file could not be removed (%s): %v", old, err)
		}
	}
}

// errPaidUnreadable is what a raise is refused with when the record could not be read: paying on, as if nothing had been paid, could pay the same ground twice.
var errPaidUnreadable = errors.New("the editor's own record of the Landsraad ground it paid could not be read (another program may be holding the file), so this raise was refused and nothing was written; try again in a moment")

// errPaidNotesDropped is what a raise is refused with when a record that kept failing lost notes of ground already paid: paying on could pay the same ground twice.
var errPaidNotesDropped = errors.New("the editor could not record the Landsraad ground it paid for a long time, and some of what it paid is no longer known, so this raise was refused and nothing was written. Raises stay refused until the editor is restarted. First make sure its own folder can be written to (another program may be holding the file, or the disk may be full), then make one more change and wait until the banner about the record is gone: only a write hands over what is still kept. Then restart the editor")

// snapshot is what the file records now, read once: a request takes it once and asks it for every house. A ledger that is nil or has no path gives nothing, and so does a file that is
// missing or not usable. A file of a newer version is an error (errPaidNewerRefused): the raise is refused and the file is left as it is. A file that could not be read (another
// program holds it) is an error after a short retry: the caller refuses the raise and says why.
func (l *PaidLedger) snapshot() ([]paidNote, error) {
	if l == nil || l.path == "" {
		return nil, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	es, err := l.readRetry()
	switch {
	case err == nil:
		return es, nil
	case errors.Is(err, errPaidNewer):
		return nil, errPaidNewerRefused
	case errors.Is(err, errPaidFile):
		return nil, nil // read as nothing paid; the next write moves the file aside
	}
	return nil, errPaidUnreadable
}

// recorded is the highest ground recorded for the character on the house in the term, under any faction, or 0. It reads the file; the editor's own code asks a snapshot.
func (l *PaidLedger) recorded(player, term int64, house string) float64 {
	es, _ := l.snapshot()
	return highestPaid(es, player, term, house)
}

func highestPaid(es []paidNote, player, term int64, house string) float64 {
	best := 0.0
	for _, e := range es {
		if e.Player == player && e.Term == term && e.House == house && e.Paid > best {
			best = e.Paid
		}
	}
	return best
}

// highestBack is the highest tier whose rows a take-back removed on purpose for the character on the house in the term, under any faction, or 0.
func highestBack(es []paidNote, player, term int64, house string) float64 {
	best := 0.0
	for _, e := range es {
		if e.Player == player && e.Term == term && e.House == house && e.Back > best {
			best = e.Back
		}
	}
	return best
}

// floorOf is the lowest ground the editor started paying from for the character on the house in the term, under any faction, and whether it is known: it is not when nothing is
// recorded or when any entry carries no floor (a record of an older build). Tiers at or below it are not vouched for.
func floorOf(es []paidNote, player, term int64, house string) (float64, bool) {
	lo, any := math.Inf(1), false
	for _, e := range es {
		if e.Player == player && e.Term == term && e.House == house && e.Paid > 0 {
			if e.Floor == nil {
				return 0, false
			}
			any = true
			lo = math.Min(lo, *e.Floor)
		}
	}
	return lo, any
}

// mergeFloor is the floor of an entry after a note is merged into it: the lowest of the two when the note continues the ground (begins at or below what the entry paid), the note's when it
// begins above it (a gap: the ground in between is not the editor's), unknown when either does not know. A note with no floor that takes the ground higher leaves it unknown.
func mergeFloor(e, n paidNote) *float64 {
	switch {
	case e.Floor == nil:
		return nil
	case n.Floor == nil:
		if n.Paid > e.Paid {
			return nil
		}
		return e.Floor
	case *n.Floor > e.Paid:
		f := *n.Floor
		return &f
	}
	f := math.Min(*e.Floor, *n.Floor)
	return &f
}

// mergePaid puts notes, in order, into entries: the highest ground per character, term, house and faction; a note that replaces drops everything recorded for the character on the
// house in the term (under every faction) first. Invalid notes are left out.
func mergePaid(entries, notes []paidNote) []paidNote {
	out := append([]paidNote(nil), entries...)
	for _, n := range notes {
		if !n.valid() {
			continue
		}
		if n.Replace {
			kept := out[:0]
			for _, e := range out {
				if !(e.Player == n.Player && e.Term == n.Term && e.House == n.House) {
					kept = append(kept, e)
				}
			}
			out = kept
		}
		n.Replace = false
		n = n.norm()
		found := false
		for i := range out {
			if e := &out[i]; e.Player == n.Player && e.Term == n.Term && e.House == n.House && e.Faction == n.Faction {
				e.Floor = mergeFloor(*e, n)
				e.Paid = max(e.Paid, n.Paid)
				e.Back = max(e.Back, n.Back)
				found = true
				break
			}
		}
		if !found {
			out = append(out, n)
		}
	}
	return out
}

// evictPaid keeps entries within max: the entries of the oldest terms go first, whole terms at a time as far as that is needed, and the entries of keepTerm (the term being written) never
// go while any other remains. Only when keepTerm alone is above the bound are its oldest entries dropped.
func evictPaid(entries []paidNote, keepTerm int64, max int) []paidNote {
	if len(entries) <= max {
		return entries
	}
	seen := map[int64]bool{}
	var terms []int64
	for _, e := range entries {
		if e.Term != keepTerm && !seen[e.Term] {
			seen[e.Term] = true
			terms = append(terms, e.Term)
		}
	}
	sort.Slice(terms, func(i, j int) bool { return terms[i] < terms[j] })
	for _, term := range terms {
		if len(entries) <= max {
			break
		}
		kept := entries[:0:0]
		drop := len(entries) - max
		for _, e := range entries {
			if e.Term == term && drop > 0 {
				drop--
				continue
			}
			kept = append(kept, e)
		}
		entries = kept
	}
	if len(entries) > max {
		entries = entries[len(entries)-max:]
	}
	return entries
}

// decodePaid reads the paid-ground content of a note; ok is false when the note is of another kind or is not readable. A note that comes again (Note.Again) may have been recorded before,
// so what it asks to replace is not replaced: the record's merge and the readers of pending notes go through here, so that they cannot differ on that.
func decodePaid(n save.Note) (p paidNote, ok bool) {
	if n.Kind != paidNoteKind || json.Unmarshal(n.Data, &p) != nil {
		return paidNote{}, false
	}
	if n.Again {
		p.Replace = false
	}
	return p, true
}

// pendingPaid decodes the notes the file does not hold yet: the ones a failing record kept after their edits were written, and those of the queued edits.
func pendingPaid(notes []save.Note) []paidNote {
	var out []paidNote
	for _, n := range notes {
		if p, ok := decodePaid(n); ok {
			out = append(out, p)
		}
	}
	return out
}

// Record is the sink of the save's notes: it merges the paid-ground notes of the edits that were just written into the file, in order. The entries of other
// terms are left as they are (the term is part of the key); only the bound of 4,096 entries evicts, oldest terms first. Notes of other kinds are not its concern. A note that comes again (it may have been recorded before: Note.Again) merges only: what it asks to replace is not replaced, or
// a later, higher state would be lowered by the first part of the batch. A file that is missing is started anew; one that is damaged, too large, or not this format or this save's, is moved aside first (moveAside, #811, and Notice says so), and one that is of a newer version (or one this build cannot tell, see read) is left as it is with an error;
// a file that could not be read
// (another program holds it) is tried again once and then left as it is with an error, which the save keeps the notes for: a record is never rebuilt from an unreadable file, or the
// other houses' ground would be lost. A file that cannot be written is an error too.
func (l *PaidLedger) Record(notes []save.Note) error {
	if l == nil || l.path == "" {
		return nil
	}
	var seq []paidNote
	cur := int64(0)
	for _, n := range notes {
		if p, ok := decodePaid(n); ok && p.valid() {
			seq = append(seq, p)
			cur = max(cur, p.Term)
		}
	}
	if len(seq) == 0 {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	old, err := l.readRetry()
	switch {
	case err == nil:
	case errors.Is(err, errPaidNewer):
		return fmt.Errorf("the record of the Landsraad ground paid was left as it is: %w", err)
	case errors.Is(err, errPaidFile):
		kept, mv := l.moveAside()
		if mv != nil {
			return fmt.Errorf("the record of the Landsraad ground paid is not usable (%v) and could not be kept aside: %w", err, mv)
		}
		log.Printf("the Landsraad paid-ground file was not usable (%v); it was moved aside (%s) and a new one is started", err, kept)
		if kept != "" {
			why := err.Error()
			if len(why) > 160 {
				why = why[:160] + "..."
			}
			l.notice.Store(fmt.Sprintf("the editor's own record of the Landsraad ground it paid could not be used (%s), so a new record was started and the old file was kept as %s in the editor's own folder", why, kept))
		}
		old = nil
	default:
		return fmt.Errorf("the record of the Landsraad ground paid could not be read: %w", err)
	}
	// the term is part of the key, so the entries of other terms are left as they are: a write in another term (a backup of last week put back, one raise there, and the file put back
	// again) must not take this term's record with it. Only the bound makes room, oldest terms first and never the term of these notes.
	entries := evictPaid(mergePaid(old, seq), cur, paidMaxEntries)
	b, err := json.Marshal(paidFile{Version: paidFileVersion, Save: l.save, Entries: entries})
	if err != nil {
		return err
	}
	if len(b) > paidMaxBytes { // never write what a read would call too large (paidMaxBytes is derived from the entry bound, so this is not reachable)
		return fmt.Errorf("the record of the Landsraad ground paid would be %d bytes, more than the %d the editor reads", len(b), paidMaxBytes)
	}
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return err
	}
	return durable.WriteFile(l.path, b, 0o600, l.opts)
}

// standingTierSQL is the highest reward tier of the task whose item has a reward row of the character, for the house, written in this term (last_updated at or after the start of the
// term), or 0. The item of a tier is the task's own item (landsraad_task_rewards) or, for the highest tier, the house swatch (landsraad_one_time_house_rewards, when the save has
// it). one is Mut.One or Save.One.
func standingTierSQL(one func(string, ...any) (save.Row, error), player, taskID int64, house string, termStart int64) (float64, error) {
	r, err := one(`select coalesce(max(r.threshold),0) t from landsraad_task_rewards r where r.task_id=? and exists
		(select 1 from landsraad_house_rewards h where h.player_id=? and h.house_name=? and h.template_id=r.template_id and h.last_updated>=?)`, taskID, player, house, termStart)
	if err != nil {
		return 0, err
	}
	best, _ := toFloat(r["t"])
	if has, err := one(`select 1 x from sqlite_master where type='table' and name='landsraad_one_time_house_rewards'`); err != nil {
		return 0, err
	} else if has != nil {
		s, err := one(`select coalesce(max(o.threshold),0) t from landsraad_one_time_house_rewards o where o.house_name=? and exists
			(select 1 from landsraad_house_rewards h where h.player_id=? and h.house_name=o.house_name and h.template_id=o.template_id and h.last_updated>=?)`, house, player, termStart)
		if err != nil {
			return 0, err
		}
		t, _ := toFloat(s["t"])
		best = max(best, t)
	}
	return best, nil
}

// standingTierRows is standingTierSQL for the board read, from the rows it already holds: tiers (rows of landsraad_task_rewards), the house's swatch row (nil when it has none) and
// delivered (the character's rows of landsraad_house_rewards keyed by house and item).
func standingTierRows(tiers []any, swatch save.Row, delivered map[string]save.Row, house string, termStart float64) float64 {
	stands := func(tmpl any) bool {
		d, ok := delivered[house+"\x00"+fmt.Sprint(tmpl)]
		if !ok {
			return false
		}
		upd, _ := toFloat(d["last_updated"])
		return upd >= termStart
	}
	best := 0.0
	for _, x := range tiers {
		r := x.(save.Row)
		if th, _ := toFloat(r["threshold"]); th > best && stands(r["template_id"]) {
			best = th
		}
	}
	if swatch != nil {
		if th, _ := toFloat(swatch["threshold"]); th > best && stands(swatch["template_id"]) {
			best = th
		}
	}
	return best
}

// groundPaid is what counts of the editor's record for a house, as the save shows it.
//
//	ground      the ground whose missions and faction progress count as paid: the record, cut at the highest tier whose reward row stands, or at the tier a take-back removed rows up to (back),
//	            whichever is higher. A take-back removes the rows on purpose and leaves the missions paid, so the rows missing there are not a sign of a restored save (#944).
//	rows        the ground whose reward rows count as delivered: the record cut at the highest tier whose reward row stands, and nothing more. The rows a take-back removed are written again by the next raise.
//	back        the take-back ceiling the record carries (never above what was paid)
//	untrusted   the cut took something off the record, so a raise that pays under it replaces it
//	floor       the ground the editor started paying from, when known (floorKnown): only the tiers above it are vouched for as its own (#944)
type groundPaid struct {
	ground, rows, back, floor float64
	floorKnown                bool
	untrusted                 bool
}

// trustedPaid is what counts of the record: the ground the file and the notes it has not taken yet (kept after a write, or queued) record for the character on the house, held to the highest
// tier the save shows a reward row for or the tier a take-back removed rows up to (see groundPaid).
func trustedPaid(file []paidNote, pending []paidNote, player, term int64, house string, standing float64) groundPaid {
	all := mergePaid(file, pending)
	rec := highestPaid(all, player, term, house)
	if rec <= 0 {
		return groundPaid{}
	}
	back := min(highestBack(all, player, term, house), rec)
	limit := max(standing, back)
	floor, known := floorOf(all, player, term, house)
	return groundPaid{ground: min(rec, limit), rows: min(rec, standing), back: back, floor: floor, floorKnown: known, untrusted: rec > limit}
}

// paidGround is what the editor paid before for the character on the house, inside a raise's edit (see trustedPaid). The save is asked for its reward rows only when something is
// recorded, so a save with no record is read as before.
func paidGround(m *save.Mut, file []paidNote, me myLandsraad, taskID, termID int64, house string, clock termClock) (groundPaid, error) {
	// a failing sink dropped notes of written edits (past its bound): the ground they stood for is counted nowhere, so a raise is refused rather than paid on a short record (#810)
	if m.NotesDropped() {
		return groundPaid{}, apierr.Wrap(apierr.ErrWriteFailed, errPaidNotesDropped.Error(), errPaidNotesDropped)
	}
	pending := pendingPaid(m.Pending(paidNoteKind))
	if highestPaid(mergePaid(file, pending), me.player, termID, house) <= 0 {
		return groundPaid{}, nil
	}
	standing, err := standingTierSQL(m.One, me.player, taskID, house, clock.start)
	if err != nil {
		return groundPaid{}, err
	}
	return trustedPaid(file, pending, me.player, termID, house, standing), nil
}

// notePaidBefore says what part of a raise was not paid because the editor paid it before: lo0 is the ground the game's own events and the other factions cover, lo the same with
// the editor's record, after the contribution the raise goes to. The ground between lo0 and the smaller of after and lo, and the missions it stands for, are the part.
func (r *rewardResult) notePaidBefore(after, lo0, lo float64, g GrantSettings) {
	top := min(after, lo)
	if lo <= lo0 || top <= lo0 {
		return
	}
	r.paidGround = top - lo0
	if step := missionContribution * g.ContributionMultiplier; step > 0 {
		if n := ceilSteps(top, step) - ceilSteps(lo0, step); n > 0 {
			r.paidMissions = int64(min(n, 1e9))
		}
	}
}
