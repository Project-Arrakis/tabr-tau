package save

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// The pending edits are kept in a journal file so they survive the editor being closed, and can be put back onto a newer file when the game has saved since.
//
// What is kept is what each edit ran: the statements and their arguments, in order, with the edit's description and how many rows it changed. Nothing else about
// the save is copied. Putting the edits back runs the statements again, each edit in its own transaction through the same Mutate as a new edit (so the same checks
// apply). What an edit read is kept too, as a hash of the rows each of its queries returned, and asked again first: an edit that worked a value out from rows the
// game has changed since (Solari plus 250, the next free slot, the next item id) is not applied over them. An edit whose statements fail or change a different number
// of rows than before is not applied either: it is reported, not forced. What was put back is written like any other edit (Flush): only when the game-running checks pass and
// the file is unchanged, and the page lists what is waiting.
//
// The journal holds values from the save (it is a file in the editor's own folder, created with mode 0600 (which Windows does not apply), deleted when the edits are saved or discarded). Only INSERT,
// UPDATE, DELETE and REPLACE statements are journaled or replayed: an edit that ran anything else (a script, DDL, a pragma) turns the journal off for the session
// rather than keep part of the edits.

const (
	journalVersion  = 1
	journalMaxStmts = 200000
	journalMaxBytes = 64 << 20
)

// jarg is one statement argument with its type kept (JSON alone would turn every number into a float).
type jarg struct {
	T string `json:"t"` // n null, i integer, f float, s text, t text that is not valid UTF-8 (its bytes, base64), x blob, o boolean
	V string `json:"v,omitempty"`
}

type stmtRec struct {
	Q string `json:"q"`
	A []jarg `json:"a,omitempty"`
	// Rd marks a query the edit made (not a change), and H is a hash of the rows it got. Putting the edit back asks the same query of the newer file first and stops
	// at the first difference: the edit was worked out from rows that are not the same any more.
	Rd bool   `json:"rd,omitempty"`
	H  string `json:"h,omitempty"`
}

// opRec is one edit as it ran.
type opRec struct {
	Desc    string    `json:"desc"`
	Rows    int64     `json:"rows"`
	At      time.Time `json:"at"`
	Cascade bool      `json:"cascade,omitempty"`
	Stmts   []stmtRec `json:"stmts"`
}

type journalFile struct {
	Version  int       `json:"version"`
	Path     string    `json:"path"`     // the save the edits were made on
	BaseHash string    `json:"baseHash"` // the content of that file when the edits began
	Ops      []opRec   `json:"ops"`
	Saved    time.Time `json:"saved"`
	// Writing is the hash of the file the edits were being written as when this was kept. If the file is that one when the editor starts again, the edits are in it already (the journal
	// outlived the write: it could not be removed, or the editor was stopped between the two) and they are not put back a second time.
	Writing string `json:"writing,omitempty"`
	// WritingOps is how many of the Ops that write held (0, as in a journal of an earlier version: all of them). Edits made after an unverified write are kept after those.
	WritingOps int `json:"writingOps,omitempty"`
}

// Conflict is an edit that could not be put back, and why.
type Conflict struct {
	Desc string `json:"desc"`
	Why  string `json:"why"`
}

// Restored says what became of the edits found in the journal when it was read.
type Restored struct {
	Applied   int        `json:"applied"`
	Conflicts []Conflict `json:"conflicts,omitempty"`
	Rebased   bool       `json:"rebased"` // the file had changed since the edits were made
	// Written says that the edits that were put back have been written since. The notice of the ones that were not put back (Conflicts) outlives that write, so the person
	// still learns which edits were dropped; it goes when it is dismissed, when a queue is discarded or when the file is reloaded.
	Written bool `json:"written,omitempty"`
}

// ErrNotKept is the error of a queue the editor did not keep (the journal is off or not enabled): it cannot be put back onto a newer file, only discarded.
var ErrNotKept = errors.New("the pending edits were not kept")

type notKeptError struct{ msg string }

func (e notKeptError) Error() string        { return e.msg }
func (e notKeptError) Is(target error) bool { return target == ErrNotKept }

func encodeArg(v any) (jarg, bool) {
	switch x := v.(type) {
	case nil:
		return jarg{T: "n"}, true
	case int:
		return jarg{"i", strconv.FormatInt(int64(x), 10)}, true
	case int8:
		return jarg{"i", strconv.FormatInt(int64(x), 10)}, true
	case int16:
		return jarg{"i", strconv.FormatInt(int64(x), 10)}, true
	case int32:
		return jarg{"i", strconv.FormatInt(int64(x), 10)}, true
	case int64:
		return jarg{"i", strconv.FormatInt(x, 10)}, true
	case uint:
		return jarg{"i", strconv.FormatUint(uint64(x), 10)}, true
	case uint8:
		return jarg{"i", strconv.FormatUint(uint64(x), 10)}, true
	case uint16:
		return jarg{"i", strconv.FormatUint(uint64(x), 10)}, true
	case uint32:
		return jarg{"i", strconv.FormatUint(uint64(x), 10)}, true
	case uint64:
		return jarg{"i", strconv.FormatUint(x, 10)}, true
	case float32:
		// The database stores a REAL as a 64-bit float, so the value that was run is float64(x), with every digit it has (0.1 as a 32-bit float is 0.10000000149011612). Writing the
		// shortest text that a 32-bit float reads back from would put 0.1 into the file when the edit was run again.
		return jarg{"f", strconv.FormatFloat(float64(x), 'g', -1, 64)}, true
	case float64:
		return jarg{"f", strconv.FormatFloat(x, 'g', -1, 64)}, true
	case string:
		if !utf8.ValidString(x) { // JSON would replace the bad bytes with U+FFFD: the text is kept as the bytes it is
			return jarg{"t", base64.StdEncoding.EncodeToString([]byte(x))}, true
		}
		return jarg{"s", x}, true
	case []byte:
		if x == nil { // the driver binds a nil slice as NULL and an empty one as an empty blob: the two are kept apart
			return jarg{T: "n"}, true
		}
		return jarg{"x", base64.StdEncoding.EncodeToString(x)}, true
	case bool:
		return jarg{"o", strconv.FormatBool(x)}, true
	}
	return jarg{}, false
}

func decodeArg(a jarg) (any, error) {
	switch a.T {
	case "n":
		return nil, nil
	case "i":
		return strconv.ParseInt(a.V, 10, 64)
	case "f":
		return strconv.ParseFloat(a.V, 64)
	case "s":
		return a.V, nil
	case "t":
		b, err := base64.StdEncoding.DecodeString(a.V)
		return string(b), err
	case "x":
		return base64.StdEncoding.DecodeString(a.V)
	case "o":
		return strconv.ParseBool(a.V)
	}
	return nil, fmt.Errorf("unknown argument type %q", a.T)
}

// journalable says whether a statement is one the journal keeps: a single INSERT, UPDATE, DELETE or REPLACE (a WITH clause may lead into one).
func journalable(q string) bool {
	s := strings.TrimSpace(q)
	s = strings.TrimRight(s, "; \t\r\n")
	if s == "" || !oneStatement(q) { // a script of several statements (a ';' inside a quoted value or a comment is not the end of one)
		return false
	}
	w := strings.ToLower(s)
	if i := strings.IndexFunc(w, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '(' }); i > 0 {
		w = w[:i]
	}
	switch w {
	case "insert", "update", "delete", "replace", "with":
		return true
	}
	return false
}

// EnableJournal keeps the pending edits in the file at path from now on. It is called once, right after Open, by whoever owns the editor's own folder.
func (s *Save) EnableJournal(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.journalPath = path
}

// JournalOff says why the journal stopped for this session ("" while it is on or was never enabled).
func (s *Save) JournalOff() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.journalOff
}

// Restored says what became of the edits put back at start (nil when there were none or the notice was dismissed).
func (s *Save) Restored() *Restored {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.restored == nil {
		return nil
	}
	r := *s.restored
	return &r
}

// DismissRestored clears the notice about the edits put back.
func (s *Save) DismissRestored() {
	s.mu.Lock()
	s.restored = nil
	s.mu.Unlock()
}

// recordOp keeps an edit that has just run, and writes the journal. The caller holds s.mu.
func (s *Save) recordOp(o opRec) {
	if s.journalPath == "" || s.journalOff != "" {
		return
	}
	for _, st := range o.Stmts {
		if !(st.Rd && readable(st.Q)) && !(!st.Rd && journalable(st.Q)) {
			s.stopJournal("an edit ran a statement the journal does not keep (" + o.Desc + ")")
			return
		}
	}
	s.recs = append(s.recs, o)
	s.recStmts += len(o.Stmts)
	if s.recStmts > journalMaxStmts {
		s.stopJournal("too many statements to keep")
		return
	}
	s.writeJournal()
}

// stopJournal turns the journal off for the session and removes the file: a journal that holds only some of the edits would put back a state the person never had.
func (s *Save) stopJournal(why string) {
	s.journalOff = why
	s.recs, s.recStmts = nil, 0
	if s.journalPath == "" {
		return
	}
	if err := os.Remove(s.journalPath); err != nil && !os.IsNotExist(err) {
		// a file that cannot be removed (another program holds it) must not be put back later as if it were the whole queue: it is marked incomplete (see incompleteMark)
		s.markIncomplete(s.writing, why+"; the kept file could not be removed ("+shortErr(err)+")")
	}
}

// dropJournal forgets the kept edits and removes the file (the edits were saved or discarded). The caller holds s.mu.
func (s *Save) dropJournal() {
	// nothing is pending any more, so whatever turned the journal off (an edit it could not keep) no longer matters: the next edit is kept again. The notice of the edits
	// that were put back or dropped (restored) is not touched: whoever ends the queue decides.
	writing := s.writing
	s.recs, s.recStmts, s.journalOff, s.writing, s.held = nil, 0, "", "", nil
	s.writingOps, s.writingRecs, s.writingBackup = 0, 0, ""
	if s.journalPath == "" {
		return
	}
	if err := os.Remove(s.journalPath); err != nil && !os.IsNotExist(err) {
		// the queue is over but its file cannot be removed: marked, with the file the edits were written as, so a restart that finds that file drops it quietly and any other file reports it
		s.markIncomplete(writing, "the kept file could not be removed ("+shortErr(err)+")")
		return
	}
	os.Remove(s.markPath())
}

// noteWriting records in the journal, before the file is replaced, which file the edits are being written as (see journalFile.Writing).
func (s *Save) noteWriting(blob []byte) {
	h := hashOf(blob)
	s.writing = hex.EncodeToString(h[:])
	s.writingOps, s.writingRecs, s.writingBackup = len(s.ops), len(s.recs), ""
	if s.journalOff != "" {
		// the journal is off, so the marker cannot go into it; if an old journal is still on disk (marked incomplete), the mark carries the marker instead
		if m, ok := s.readMark(); ok {
			s.markIncomplete(s.writing, m.Reason)
		}
		return
	}
	s.writeJournal()
}

// writeJournal replaces the file with the edits kept so far (never in place: a crash leaves the old one). The caller holds s.mu.
func (s *Save) writeJournal() {
	if s.journalPath == "" || s.journalOff != "" || s.replaying {
		return
	}
	if len(s.recs) == 0 {
		os.Remove(s.journalPath)
		return
	}
	j := journalFile{Version: journalVersion, Path: s.Path, BaseHash: hex.EncodeToString(s.diskHash[:]), Ops: s.recs, Saved: time.Now().UTC(), Writing: s.writing}
	if s.writing != "" {
		j.WritingOps = s.writingRecs
	}
	b, err := json.Marshal(j)
	if err != nil || len(b) > journalMaxBytes {
		s.stopJournal("the pending edits are too large to keep")
		return
	}
	// A journal that cannot be written is not left to look as if it were kept: it is turned off with the reason, and the page says that the queue is lost if the editor closes.
	if err := os.MkdirAll(filepath.Dir(s.journalPath), 0o700); err != nil {
		s.stopJournal("the journal could not be written (" + shortErr(err) + ")")
		return
	}
	tmp := s.journalPath + ".tmp"
	// Written and flushed to the disk before it replaces the old one, so a power cut leaves the old journal or the new one, never half of one. A virus scanner, an indexer or a
	// backup program can hold either file for a moment, so a failed attempt is made again a few times before it counts; and when the new journal still cannot replace the old,
	// the old one stays (it holds the edits up to the last write that worked): deleting it would turn one slow moment into the loss of every kept edit.
	var lastErr error
	for try := 0; try < journalWriteTries; try++ {
		if try > 0 {
			time.Sleep(renameBackoff * time.Duration(try))
		}
		os.Remove(tmp) // a leftover of a stopped run, or of the attempt before
		if err := writeDurable(tmp, b, 0o600); err != nil {
			lastErr = err
			continue
		}
		if err := fsRename(tmp, s.journalPath); err != nil {
			os.Remove(tmp)
			lastErr = err
			continue
		}
		os.Remove(s.markPath()) // this journal is the whole queue again
		return
	}
	s.journalOff = "the journal could not be written (" + shortErr(lastErr) + "); the edits made since the last copy that was kept are not kept"
	s.recs, s.recStmts = nil, 0
	// The old journal stays (deleting it could turn one slow moment into the loss of every kept edit), but it holds only the edits up to the last write that worked, and a restart
	// must not put it back as if it were the whole queue: it is marked incomplete, and the restart reports it.
	s.markIncomplete(s.writing, s.journalOff)
}

// incompleteMark is a file next to the journal that says the journal is not the whole queue: the editor could not keep every edit (the journal could not be written), or could not
// remove the file of a queue that is over. A restart that finds it reports the journal (kept aside, not put back) unless Writing is the file on disk, which means the edits were written.
// It is written in place and flushed; a mark that cannot be read counts as a mark with no Writing.
type incompleteMark struct {
	Version int       `json:"version"`
	At      time.Time `json:"at"`
	Reason  string    `json:"reason"`
	Writing string    `json:"writing,omitempty"`
}

func (s *Save) markPath() string { return s.journalPath + ".incomplete" }

// markIncomplete writes the mark. It is not written by renaming a temp file over the old one: the failures it answers are those of replacing the journal, and a mark that is half
// written is read as a mark that cannot be read, which is still a mark.
func (s *Save) markIncomplete(writing, reason string) {
	if s.journalPath == "" {
		return
	}
	b, err := json.Marshal(incompleteMark{Version: journalVersion, At: time.Now().UTC(), Reason: reason, Writing: writing})
	if err != nil {
		return
	}
	for try := 0; try < journalWriteTries; try++ {
		if try > 0 {
			time.Sleep(renameBackoff * time.Duration(try))
		}
		f, err := os.OpenFile(s.markPath(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			continue
		}
		_, werr := f.Write(b)
		serr := syncFile(f)
		cerr := f.Close()
		if werr == nil && serr == nil && cerr == nil {
			return
		}
	}
}

// readMark reads the mark; ok is false when there is none. A mark that cannot be read as JSON is returned empty (ok true).
func (s *Save) readMark() (incompleteMark, bool) {
	b, err := os.ReadFile(s.markPath())
	if err != nil {
		return incompleteMark{}, !os.IsNotExist(err)
	}
	var m incompleteMark
	if json.Unmarshal(b, &m) != nil {
		return incompleteMark{Reason: "the mark could not be read"}, true
	}
	return m, true
}

// journalWriteTries is how many times a journal write is attempted before the journal is given up for the session.
const journalWriteTries = 4

// shortErr is an error's message without the folder names of a path error, which the page would show.
func shortErr(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Op + " " + filepath.Base(pe.Path) + ": " + pe.Err.Error()
	}
	return err.Error()
}

// oneStatement says whether q is a single SQL statement: a ';' ends a statement only outside a quoted text or name ('...', "...", `...`, [...]) and outside a comment
// (-- to the end of the line, /* ... */), and only blanks and comments may follow it. A quote or comment that is not closed is not one statement.
func oneStatement(q string) bool {
	ended := false // a statement has ended; only blanks, comments and more ';' may follow
	for i := 0; i < len(q); {
		c := q[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == ';':
			if c == ';' {
				ended = true
			}
			i++
		case c == '-' && i+1 < len(q) && q[i+1] == '-':
			for i < len(q) && q[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(q) && q[i+1] == '*':
			end := strings.Index(q[i+2:], "*/")
			if end < 0 { // SQLite lets a comment run to the end of the text
				return true
			}
			i += end + 4
		case c == '\'' || c == '"' || c == '`' || c == '[':
			if ended {
				return false
			}
			closer := c
			if c == '[' {
				closer = ']'
			}
			j := i + 1
			for j < len(q) && q[j] != closer { // a doubled quote closes and reopens, which comes to the same
				j++
			}
			if j >= len(q) {
				return false
			}
			i = j + 1
		default:
			if ended {
				return false
			}
			i++
		}
	}
	return true
}

var errConflict = errors.New("conflict")

// samePath says whether two spellings name the same save: after cleaning, equal, and on Windows (whose paths do not distinguish case) equal ignoring case.
func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// skipJournal reports a journal this run cannot use and keeps it: the file is moved aside (named ...skipped, then ...skipped.2 and so on), not deleted, because the next edit would
// write a new journal over it. The page shows it as a queued change that could not be put back, with the reason.
func (s *Save) skipJournal(why string) *Restored {
	aside := s.journalPath + ".skipped"
	for i := 2; i < 100000; i++ { // the first free name: a journal that was set aside before is never overwritten
		if _, err := os.Lstat(aside); err != nil {
			break
		}
		aside = s.journalPath + ".skipped." + strconv.Itoa(i)
	}
	if err := os.Rename(s.journalPath, aside); err != nil {
		why += " (it could not be moved aside: " + shortErr(err) + ")"
	} else {
		why += " (it is kept as " + filepath.Base(aside) + " in the editor's folder)"
	}
	r := &Restored{Conflicts: []Conflict{{Desc: "the queued changes kept by an earlier run", Why: why}}}
	s.restored = r
	return r
}

// RestoreJournal puts the edits found in the journal back onto the file as it is now, all of them or none. It is for start-up (the journal is of an earlier run) and does
// nothing when there is no journal, it is of another save, or this save already has edits.
func (s *Save) RestoreJournal() (*Restored, error) {
	s.flushMu.Lock()
	defer s.flushMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.journalPath == "" || s.dirty || len(s.recs) > 0 {
		return nil, nil
	}
	if m, ok := s.readMark(); ok {
		return s.restoreIncomplete(m), nil
	}
	b, err := os.ReadFile(s.journalPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var j journalFile
	if err := json.Unmarshal(b, &j); err != nil {
		return s.skipJournal("the file could not be read as a list of changes"), nil
	}
	switch {
	case j.Version != journalVersion:
		return s.skipJournal("it was kept by a different version of the editor"), nil
	case len(j.Ops) == 0:
		return nil, nil // nothing was kept in it
	case !samePath(j.Path, s.Path):
		return s.skipJournal("it was kept for a save with another path or spelling of the path"), nil
	}
	if j.Writing != "" && j.Writing == hex.EncodeToString(s.diskHash[:]) { // the file is the one these edits were written as: they are in it
		if n := j.WritingOps; n > 0 && n < len(j.Ops) {
			// the edits made after that write (it could not be read back, and the person went on) are not in it: they are put back onto it
			r, err := s.putBackAtomic(j.Ops[n:], false, Restored{})
			if err == nil && s.held != nil {
				r.Applied = 0
			}
			return r, err
		}
		s.dropJournal()
		return nil, nil
	}
	// All of it or none, as the automatic write does: a queue with an edit that does not apply is held, with its journal, and reported. The edits that did apply are not left to
	// be written alone, with the rest only a notice that is gone when the editor closes (and the journal deleted when none applied).
	r, err := s.putBackAtomic(j.Ops, j.BaseHash != hex.EncodeToString(s.diskHash[:]), Restored{})
	if err == nil && s.held != nil {
		r.Applied = 0 // the ones that applied were undone with the rest: nothing was put back, the whole queue is held
	}
	return r, err
}

// Rebase reads the file again, as Discard does, and puts the pending edits back onto it where they still apply (the game saved since they were made).
func (s *Save) Rebase() (*Restored, error) {
	s.flushMu.Lock()
	defer s.flushMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	// a write that could not be read back is settled first: the edits it held are not put back a second time
	s.settleWrittenLocked()
	if s.held != nil { // a held queue is not in s.recs: reading the file again and putting nothing back would throw it away without a word
		return nil, apierr.New(apierr.ErrQueueHeld, "the queued changes conflict with what the game saved and cannot be put back: discard them first, then reload")
	}
	kept := append([]opRec(nil), s.recs...)
	if s.dirty && len(kept) == 0 { // edits were made but not kept (the journal is off, or was never enabled): they cannot be put back
		why := s.journalOff
		if why == "" {
			why = "the journal is not enabled"
		}
		return nil, notKeptError{"the pending edits were not kept (" + why + "): they cannot be put back; use Discard to read the file again"}
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	s.restored = nil
	if len(kept) == 0 {
		s.dropJournal()
		return nil, nil
	}
	// the journal file stays until the edits that go back have replaced it (a stop in between must not lose the queue)
	s.recs, s.recStmts, s.journalOff, s.writing, s.held = nil, 0, "", "", nil
	return s.replayLocked(kept, true, true), nil
}

// Held is a queue that could not be put back onto the game's newer file: what it was, and which edits conflicted and why.
type Held struct {
	ops       []opRec
	Conflicts []Conflict `json:"conflicts"`
	base      [32]byte   // the file the game had when the queue was held: while it is that one, trying again would give the same answer
}

// Held says whether the queue is held, and why.
func (s *Save) Held() *Held {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.held == nil {
		return nil
	}
	h := *s.held
	return &h
}

// rebaseAtomic is Rebase for the automatic write: all of the queue goes back onto the game's newer file or none of it is written. An edit that does not apply (it changed a
// different number of rows, a row the game changed, an id the game used since) could leave the ones after it acting on the game's own rows, so the first conflict undoes the lot:
// the file is read again as the game has it, the queue is held (kept in the journal, not written) and the conflicts are reported.
func (s *Save) rebaseAtomic() (*Restored, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := append([]opRec(nil), s.recs...)
	if s.held != nil {
		kept = append([]opRec(nil), s.held.ops...)
	}
	if len(kept) == 0 {
		why := s.journalOff
		if why == "" {
			why = "the journal is not enabled"
		}
		return nil, notKeptError{"the pending edits were not kept (" + why + "): they cannot be put back onto the game's newer file; use Discard to read the file again"}
	}
	dropped := Restored{}
	if s.restored != nil {
		dropped.Conflicts = s.restored.Conflicts // edits an earlier reload dropped: still to be seen
	}
	// Nothing about the queue changes until the file has been read: a failed read leaves it as it was (held, with its journal).
	if err := s.load(); err != nil {
		return nil, err
	}
	s.held = nil
	return s.putBackAtomic(kept, true, dropped)
}

// putBackAtomic puts the queue onto the file as the game has it (just read, nothing queued), all of it or none: the first edit that does not apply holds the whole queue (kept in
// the journal, not written) and the conflicts are reported. dropped is the edits an earlier reload dropped, still to be seen. The caller holds s.mu.
func (s *Save) putBackAtomic(kept []opRec, rebased bool, dropped Restored) (*Restored, error) {
	// The journal is not written by the put-back itself: until the outcome is known it holds the whole queue, and a stop or a failure below must find that and not the
	// part of it that happened to apply.
	res := s.replayLocked(kept, rebased, false)
	if len(res.Conflicts) == 0 {
		s.writeJournal()
		if len(dropped.Conflicts) > 0 {
			n := *res
			n.Conflicts = dropped.Conflicts
			s.restored = &n
		}
		return res, nil
	}
	// nothing is written: back to the file as the game has it, the queue held as it was
	held := &Held{ops: kept, Conflicts: res.Conflicts, base: s.diskHash}
	s.recs, s.recStmts = nil, 0
	s.restored = nil
	if len(dropped.Conflicts) > 0 {
		s.restored = &dropped
	}
	if err := s.load(); err != nil {
		// The working copy holds only the edits that applied. The queue is held whole all the same (the journal still has all of it, and nothing is written while it is held),
		// so the partial copy is never written; the error is reported, and a later attempt reads the file again from the held queue.
		s.held = held
		s.recs = nil
		return nil, err
	}
	s.recs = kept
	s.writeJournal()
	s.recs = nil
	held.base = s.diskHash
	s.held = held
	return res, nil
}

// replayStep, when set (by a test), is called after each edit has been put back, with s.mu held.
var replayStep func()

// replayLocked runs the edits again, each in its own transaction, and keeps the result as the pending edits. The caller holds s.mu, and it is held to the end: a new edit, a
// discard or a status read cannot come in between two of the edits being put back.
//
// An edit that does not apply is not forced, and neither is an edit that works on what it was to leave: one that touches a table the failed edit touched and has a value in
// common with it (the id of the row the failed edit was to add, the item it was to change) would otherwise act on the game's own rows with that id. It is reported as not put
// back, naming the edit it depends on.
func (s *Save) replayLocked(ops []opRec, rebased, journal bool) *Restored {
	res := &Restored{Rebased: rebased}
	s.replaying = true
	var failed []opRec
	for _, o := range ops {
		if dep := dependsOnAny(o, failed); dep != "" {
			res.Conflicts = append(res.Conflicts, Conflict{Desc: o.Desc, Why: "it works on the same rows as \"" + dep + "\", which could not be put back"})
			failed = append(failed, o)
			continue
		}
		if why := s.replayOp(o); why != "" {
			res.Conflicts = append(res.Conflicts, Conflict{Desc: o.Desc, Why: why})
			failed = append(failed, o)
			continue
		}
		res.Applied++
		if replayStep != nil {
			replayStep()
		}
	}
	s.replaying = false
	if journal {
		s.writeJournal()
	}
	if res.Applied > 0 || len(res.Conflicts) > 0 {
		s.restored = res
	}
	return res
}

func (s *Save) replayOp(o opRec) string {
	run := func(m *Mut) error {
		var before, after int64
		var err error
		if before, err = m.changes(); err != nil {
			return err
		}
		for _, st := range o.Stmts {
			args := make([]any, len(st.A))
			for i, a := range st.A {
				v, err := decodeArg(a)
				if err != nil {
					return err
				}
				args[i] = v
			}
			if st.Rd {
				// what the edit looked at must be what the newer file shows now, or the values it worked out from it (a total plus 250, the next free slot, the next id) are stale
				if !readable(st.Q) {
					return fmt.Errorf("a statement that is not kept: %w", errConflict)
				}
				_, h, err := m.queryHashed(st.Q, args...)
				if err != nil {
					return err
				}
				if h != st.H {
					return fmt.Errorf("the game changed what it was worked out from: %w", errConflict)
				}
				continue
			}
			if !journalable(st.Q) {
				return fmt.Errorf("a statement that is not kept: %w", errConflict)
			}
			if _, err := m.Exec(st.Q, args...); err != nil {
				return err
			}
		}
		if after, err = m.changes(); err != nil {
			return err
		}
		if after-before != o.Rows {
			return fmt.Errorf("it changed %d rows now and %d before: %w", after-before, o.Rows, errConflict)
		}
		m.Desc = o.Desc
		return nil
	}
	_, err := s.mutateLocked(o.Desc, o.Cascade, run)
	if err == nil {
		return ""
	}
	if errors.Is(err, errConflict) {
		return strings.TrimSuffix(err.Error(), ": conflict")
	}
	return "the save no longer accepts it: " + err.Error()
}

// restoreIncomplete is RestoreJournal for a journal that is marked incomplete. If the mark carries the hash of the file on disk the edits were written and there is nothing to do.
// Otherwise the journal is not put back (it holds some of the edits, and putting back some of them is a state the person never had): it is set aside and reported, with the edits it
// holds, and the person is told that edits after those may be missing.
func (s *Save) restoreIncomplete(m incompleteMark) *Restored {
	if m.Writing != "" && m.Writing == hex.EncodeToString(s.diskHash[:]) {
		os.Remove(s.journalPath)
		os.Remove(s.markPath())
		return nil
	}
	var held []string
	if b, err := os.ReadFile(s.journalPath); err == nil {
		var j journalFile
		if json.Unmarshal(b, &j) == nil {
			for i, o := range j.Ops {
				if i == 5 {
					held = append(held, "and "+strconv.Itoa(len(j.Ops)-i)+" more")
					break
				}
				held = append(held, o.Desc)
			}
		}
	}
	why := "the editor could not keep every change that was queued when it was closed (" + m.Reason + "), so what was kept is not the whole queue and was not put back"
	if len(held) > 0 {
		why += "; it held: " + strings.Join(held, "; ") + ". Changes made after those may be missing"
	} else {
		why += "; none of the queued changes may have been kept"
	}
	var r *Restored
	if _, err := os.Lstat(s.journalPath); err == nil {
		r = s.skipJournal(why)
	} else {
		r = &Restored{Conflicts: []Conflict{{Desc: "the queued changes kept by an earlier run", Why: why}}}
		s.restored = r
	}
	os.Remove(s.markPath())
	return r
}
