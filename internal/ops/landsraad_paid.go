package ops

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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
//	              plausible (no term, no house, a ground that is not a number or beyond any tier) is left out
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
	paidMaxBytes    = 1 << 20
	paidMaxEntries  = 4096
	paidMaxHouseLen = 80
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
	// Replace, on a note, says that what was recorded for the character on the house in the term is of a save that is gone and is dropped before this is recorded. It is not stored.
	Replace bool `json:"replace,omitempty"`
}

// valid says whether an entry is plausible: a term, a character, a house and a ground that is a number from above 0 to the largest tier a request may name.
func (e paidNote) valid() bool {
	return e.Term > 0 && e.Player > 0 && e.House != "" && len(e.House) <= paidMaxHouseLen && !math.IsNaN(e.Paid) && e.Paid > 0 && e.Paid <= maxTier
}

type paidFile struct {
	Version int        `json:"version"`
	Save    string     `json:"save"` // the save's path, lower case: the file is ignored for any other save
	Entries []paidNote `json:"entries"`
}

// PaidLedger is the editor's own record of the Landsraad ground it paid, in one file.
type PaidLedger struct {
	path string
	save string
	mu   sync.Mutex
	opts durable.Options
}

// NewPaidLedger keeps the record in path for the save at savePath ("" for path gives a ledger that records and reads nothing).
func NewPaidLedger(path, savePath string) *PaidLedger {
	return &PaidLedger{path: path, save: strings.ToLower(savePath), opts: durable.Defaults()}
}

// PaidLedgerName is the name of the file for a save, from the hash the journal's name uses (h is the first six bytes of the hash, as hex).
func PaidLedgerName(hash string) string { return "landsraad-paid-" + hash + ".json" }

var errPaidFile = errors.New("the Landsraad paid-ground file is not usable")

// read returns the valid entries of the file. A file that does not exist, or is not a whole file of this format for this save, is an error of its own kind (errPaidFile or
// os.ErrNotExist): the caller reads that as "nothing paid". Any other error is a file that could not be read this time.
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
	if len(b) > paidMaxBytes {
		return nil, fmt.Errorf("%w: it is larger than %d bytes", errPaidFile, paidMaxBytes)
	}
	var pf paidFile
	if err := json.Unmarshal(b, &pf); err != nil {
		return nil, fmt.Errorf("%w: %v", errPaidFile, err)
	}
	if pf.Version != paidFileVersion || pf.Save != l.save {
		return nil, fmt.Errorf("%w: another version or another save", errPaidFile)
	}
	out := pf.Entries[:0:0]
	for _, e := range pf.Entries {
		if e.valid() {
			e.Replace = false
			out = append(out, e)
		}
	}
	return out, nil
}

// readRetry reads the file, trying again once when it could not be read this time. A file that does not exist, or is not a whole file of this format for this save, reads as nothing
// recorded (nil, nil); any other failure is returned as an error, so nothing is built on, or from, a record that was not seen.
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
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, errPaidFile) {
			return nil, nil
		}
	}
	return nil, err
}

// errPaidUnreadable is what a raise is refused with when the record could not be read: paying on, as if nothing had been paid, could pay the same ground twice.
var errPaidUnreadable = errors.New("the editor's own record of the Landsraad ground it paid could not be read (another program may be holding the file), so this raise was refused and nothing was written; try again in a moment")

// snapshot is what the file records now, read once: a request takes it once and asks it for every house. A ledger that is nil or has no path gives nothing, and so does a file that is
// missing or not usable. A file that could not be read (another program holds it) is an error after a short retry: the caller refuses the raise and says why.
func (l *PaidLedger) snapshot() ([]paidNote, error) {
	if l == nil || l.path == "" {
		return nil, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	es, err := l.readRetry()
	if err != nil {
		return nil, errPaidUnreadable
	}
	return es, nil
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
		found := false
		for i := range out {
			if e := &out[i]; e.Player == n.Player && e.Term == n.Term && e.House == n.House && e.Faction == n.Faction {
				e.Paid = max(e.Paid, n.Paid)
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

// pendingPaid decodes the notes of queued edits that are not written yet.
func pendingPaid(raw []json.RawMessage) []paidNote {
	var out []paidNote
	for _, r := range raw {
		var p paidNote
		if json.Unmarshal(r, &p) == nil {
			out = append(out, p)
		}
	}
	return out
}

// Record is the sink of the save's notes: it merges the paid-ground notes of the edits that were just written into the file, in order. The entries of other
// terms are left as they are (the term is part of the key); only the bound of 4,096 entries evicts, oldest terms first. Notes of other kinds are not its concern. A note that comes again (it may have been recorded before: Note.Again) merges only: what it asks to replace is not replaced, or
// a later, higher state would be lowered by the first part of the batch. A file that is missing, or not this format or this save's, is started anew; a file that could not be read
// (another program holds it) is tried again once and then left as it is with an error, which the save keeps the notes for: a record is never rebuilt from an unreadable file, or the
// other houses' ground would be lost. A file that cannot be written is an error too.
func (l *PaidLedger) Record(notes []save.Note) error {
	if l == nil || l.path == "" {
		return nil
	}
	var seq []paidNote
	cur := int64(0)
	for _, n := range notes {
		if n.Kind != paidNoteKind {
			continue
		}
		var p paidNote
		if err := json.Unmarshal(n.Data, &p); err == nil && p.valid() {
			if n.Again {
				p.Replace = false
			}
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
	if err != nil {
		return fmt.Errorf("the record of the Landsraad ground paid could not be read: %w", err)
	}
	// the term is part of the key, so the entries of other terms are left as they are: a write in another term (a backup of last week put back, one raise there, and the file put back
	// again) must not take this term's record with it. Only the bound makes room, oldest terms first and never the term of these notes.
	entries := evictPaid(mergePaid(old, seq), cur, paidMaxEntries)
	b, err := json.Marshal(paidFile{Version: paidFileVersion, Save: l.save, Entries: entries})
	if err != nil {
		return err
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

// trustedPaid is what counts of the record: the ground the file and the queued edits record for the character on the house, held to the highest tier the save shows a reward row
// for. untrusted says that the cap cut the record, so a raise that pays under it replaces it.
func trustedPaid(file []paidNote, pending []paidNote, player, term int64, house string, standing float64) (paid float64, untrusted bool) {
	rec := highestPaid(mergePaid(file, pending), player, term, house)
	if rec <= 0 {
		return 0, false
	}
	if rec > standing {
		return standing, true
	}
	return rec, false
}

// paidGround is what the editor paid before for the character on the house, inside a raise's edit (see trustedPaid). The save is asked for its reward rows only when something is
// recorded, so a save with no record is read as before.
func paidGround(m *save.Mut, file []paidNote, me myLandsraad, taskID, termID int64, house string, clock termClock) (paid float64, untrusted bool, err error) {
	pending := pendingPaid(m.Pending(paidNoteKind))
	if highestPaid(mergePaid(file, pending), me.player, termID, house) <= 0 {
		return 0, false, nil
	}
	standing, err := standingTierSQL(m.One, me.player, taskID, house, clock.start)
	if err != nil {
		return 0, false, err
	}
	paid, untrusted = trustedPaid(file, pending, me.player, termID, house, standing)
	return paid, untrusted, nil
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
