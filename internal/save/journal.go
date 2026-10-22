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
)

// The pending edits are kept in a journal file so they survive the editor being closed, and can be put back onto a newer file when the game has saved since.
//
// What is kept is what each edit ran: the statements and their arguments, in order, with the edit's description and how many rows it changed. Nothing else about
// the save is copied. Putting the edits back runs the statements again, each edit in its own transaction through the same Mutate as a new edit (so the same checks
// apply), and an edit whose statements fail or change a different number of rows than before is not applied: it is reported, not forced. The review pane shows the
// diff of whatever was put back, and saving still needs the review, the game-running checks and an unchanged file, as for any edit.
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
	T string `json:"t"` // n null, i integer, f float, s text, x blob, o boolean
	V string `json:"v,omitempty"`
}

type stmtRec struct {
	Q string `json:"q"`
	A []jarg `json:"a,omitempty"`
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
}

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
		return jarg{"f", strconv.FormatFloat(float64(x), 'g', -1, 32)}, true
	case float64:
		return jarg{"f", strconv.FormatFloat(x, 'g', -1, 64)}, true
	case string:
		return jarg{"s", x}, true
	case []byte:
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
	if s == "" || strings.Contains(s, ";") { // a script of several statements (a ';' inside a quoted value also lands here, and is simply not journaled)
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
		if !journalable(st.Q) {
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
	// nothing is pending any more, so whatever turned the journal off (an edit it could not keep) no longer matters: the next edit is kept again
	s.recs, s.recStmts, s.restored, s.journalOff, s.writing, s.held = nil, 0, nil, "", "", nil
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
	if err := os.MkdirAll(filepath.Dir(s.journalPath), 0o700); err != nil {
		return
	}
	tmp := s.journalPath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return
	}
	if err := os.Rename(tmp, s.journalPath); err != nil {
		os.Remove(tmp)
	}
}

var errConflict = errors.New("conflict")

// RestoreJournal puts the edits found in the journal back onto the file as it is now, edit by edit. It is for start-up (the journal is of an earlier run) and does
// nothing when there is no journal, it is of another save, or this save already has edits.
func (s *Save) RestoreJournal() (*Restored, error) {
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
		return nil, errors.New("the pending edits were not kept (" + why + "): they cannot be put back; use Discard to read the file again")
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	s.dropJournal()
	if len(kept) == 0 {
		return nil, nil
	}
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
		return nil, errors.New("the pending edits were not kept (" + why + "): they cannot be put back onto the game's newer file; use Discard to read the file again")
	}
	s.held = nil
	if err := s.load(); err != nil {
		return nil, err
	}
	res := s.replayLocked(kept, true)
	if len(res.Conflicts) == 0 {
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
	return res, nil
}

// replayLocked runs the edits again, each in its own transaction, and keeps the result as the pending edits. The caller holds s.mu.
func (s *Save) replayLocked(ops []opRec, rebased bool) *Restored {
	res := &Restored{Rebased: rebased}
	s.replaying = true
	s.mu.Unlock() // Mutate takes the lock itself
	for _, o := range ops {
		if why := s.replayOp(o); why != "" {
			res.Conflicts = append(res.Conflicts, Conflict{Desc: o.Desc, Why: why})
			continue
		}
		res.Applied++
	}
	s.mu.Lock()
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
			if !journalable(st.Q) {
				return fmt.Errorf("a statement that is not kept: %w", errConflict)
			}
			args := make([]any, len(st.A))
			for i, a := range st.A {
				v, err := decodeArg(a)
				if err != nil {
					return err
				}
				args[i] = v
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
	var err error
	if o.Cascade {
		_, err = s.MutateCascade(o.Desc, run)
	} else {
		_, err = s.Mutate(o.Desc, run)
	}
	if err == nil {
		return ""
	}
	if errors.Is(err, errConflict) {
		return strings.TrimSuffix(err.Error(), ": conflict")
	}
	return "the save no longer accepts it: " + err.Error()
}
