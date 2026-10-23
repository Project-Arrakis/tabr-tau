package save

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
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
// The journal holds values from the save (it is a private file in the editor's own folder, 0600, deleted when the edits are saved or discarded). Only INSERT,
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
	if s.journalPath != "" {
		os.Remove(s.journalPath)
	}
}

// dropJournal forgets the kept edits and removes the file (the edits were saved or discarded). The caller holds s.mu.
func (s *Save) dropJournal() {
	// nothing is pending any more, so whatever turned the journal off (an edit it could not keep) no longer matters: the next edit is kept again. The notice of the edits
	// that were put back or dropped (restored) is not touched: whoever ends the queue decides.
	s.recs, s.recStmts, s.journalOff, s.writing, s.held = nil, 0, "", "", nil
	if s.journalPath != "" {
		os.Remove(s.journalPath)
	}
}

// noteWriting records in the journal, before the file is replaced, which file the edits are being written as (see journalFile.Writing).
func (s *Save) noteWriting(blob []byte) {
	h := hashOf(blob)
	s.writing = hex.EncodeToString(h[:])
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
	// written and flushed to the disk before it replaces the old one, so a power cut leaves the old journal or the new one, never half of one
	os.Remove(tmp) // a leftover of a stopped run
	if err := writeDurable(tmp, b, 0o600); err != nil {
		s.stopJournal("the journal could not be written (" + shortErr(err) + ")")
		return
	}
	if err := os.Rename(tmp, s.journalPath); err != nil {
		os.Remove(tmp)
		s.stopJournal("the journal could not be written (" + shortErr(err) + ")")
	}
}

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

// RestoreJournal puts the edits found in the journal back onto the file as it is now, edit by edit. It is for start-up (the journal is of an earlier run) and does
// nothing when there is no journal, it is of another save, or this save already has edits.
func (s *Save) RestoreJournal() (*Restored, error) {
	s.flushMu.Lock()
	defer s.flushMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.journalPath == "" || s.dirty || len(s.recs) > 0 {
		return nil, nil
	}
	b, err := os.ReadFile(s.journalPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var j journalFile
	if err := json.Unmarshal(b, &j); err != nil || j.Version != journalVersion || j.Path != s.Path || len(j.Ops) == 0 {
		return nil, nil // not ours to use: left as it is (the next edit replaces it)
	}
	if j.Writing != "" && j.Writing == hex.EncodeToString(s.diskHash[:]) { // the file is the one these edits were written as: they are in it
		s.dropJournal()
		return nil, nil
	}
	return s.replayLocked(j.Ops, j.BaseHash != hex.EncodeToString(s.diskHash[:])), nil
}

// Rebase reads the file again, as Discard does, and puts the pending edits back onto it where they still apply (the game saved since they were made).
func (s *Save) Rebase() (*Restored, error) {
	s.flushMu.Lock()
	defer s.flushMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
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
	return s.replayLocked(kept, true), nil
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
	s.held = nil
	dropped := Restored{}
	if s.restored != nil {
		dropped.Conflicts = s.restored.Conflicts // edits an earlier reload dropped: still to be seen
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	res := s.replayLocked(kept, true)
	if len(res.Conflicts) == 0 {
		if len(dropped.Conflicts) > 0 {
			n := *res
			n.Conflicts = dropped.Conflicts
			s.restored = &n
		}
		return res, nil
	}
	// nothing is written: back to the file as the game has it, the queue held as it was
	if err := s.load(); err != nil {
		return nil, err
	}
	s.recs = kept
	s.writeJournal()
	s.recs = nil
	s.held = &Held{ops: kept, Conflicts: res.Conflicts, base: s.diskHash}
	s.restored = nil
	if len(dropped.Conflicts) > 0 {
		s.restored = &dropped
	}
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
func (s *Save) replayLocked(ops []opRec, rebased bool) *Restored {
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
	s.writeJournal()
	if res.Applied > 0 || len(res.Conflicts) > 0 {
		s.restored = res
	}
	return res
}

func (s *Save) replayOp(o opRec) string {
	run := func(m *Mut) error {
		var before, after int64
		if err := m.tx.QueryRow("select total_changes()").Scan(&before); err != nil {
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
				rows, err := m.Query(st.Q, args...)
				if err != nil {
					return err
				}
				if hashRows(rows) != st.H {
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
		if err := m.tx.QueryRow("select total_changes()").Scan(&after); err != nil {
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
