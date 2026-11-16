package save

import (
	"encoding/json"
	"log"
)

// ---------------------------------------------------------------- notes: what an edit asks to be remembered once it is in the game's file (#777)
//
// An edit may attach a note to itself (Mut.Note). The note travels with the edit: it is kept in the journal with the edit's statements, goes back with it when the edit is put onto a
// newer file, and is thrown away with it when the queue is discarded. When the edit has been written to the game's file, and only then, the notes of the written edits are handed to
// the sink (SetNoteSink), once, in the order the edits were made. Nothing here reads or writes the game's tables: a note is for something that lives outside the save (the Landsraad
// board keeps what it already paid in a small file of the editor's own, landsraad_paid.go in the ops package).

// Note is one note of an edit: what kind it is, and its content as JSON.
type Note struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
	// Again says that the note may have been handed to the sink before: it comes from a journal that outlived the write (the editor was closed between the write and the removal of the
	// journal), or from a write that was recognised afterwards, and the journal may hold only a first part of the edits that write held. A sink treats such a note as one it may already
	// have, so only what is safe to apply twice, or after a later state, may come of it. It is not kept in the journal.
	Again bool `json:"-"`
}

// Note attaches a note to the edit. It is kept only if the edit is committed and changed something; a note of an edit that fails is dropped with it.
func (m *Mut) Note(kind string, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	m.notes = append(m.notes, Note{Kind: kind, Data: b})
	return nil
}

// Pending returns the notes of the kind that the sink has not recorded yet, oldest first: first the notes of edits that are already written to the game's file but that the sink could
// not take (they wait for the next write), then the notes of the edits queued (made, not yet written). It is read inside an edit, so it sees what the edits before it asked to be
// remembered, which is not in the sink's store until they are written and handed over.
func (m *Mut) Pending(kind string) []json.RawMessage {
	if m.s == nil {
		return nil
	}
	return m.s.pendingNotes(kind)
}

// PendingNotes is Pending for a reader that is not inside an edit.
func (s *Save) PendingNotes(kind string) []json.RawMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pendingNotes(kind)
}

// pendingNotes: the caller holds s.mu. A queue that is held (it could not be put onto the game's newer file) is not pending: none of it is in the working copy. The notes a failing
// sink kept (undelivered) come first: their edits are in the game's file already, so a reader that left them out would count the ground they stand for nowhere (#810).
func (s *Save) pendingNotes(kind string) []json.RawMessage {
	var out []json.RawMessage
	for _, n := range s.undelivered {
		if n.Kind == kind {
			out = append(out, n.Data)
		}
	}
	for _, o := range s.ops {
		for _, n := range o.notes {
			if n.Kind == kind {
				out = append(out, n.Data)
			}
		}
	}
	return out
}

// SetNoteSink sets the function that receives the notes of the edits that were written to the game's file. It is called with the lock of the save held, so it must only record
// the notes (it must not call back into the save), and an error it returns is logged: the write it follows has happened and cannot be undone.
func (s *Save) SetNoteSink(f func([]Note) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sink = f
}

// notesOf collects the notes of edits, in order.
func notesOf(ops []Op) []Note {
	var out []Note
	for _, o := range ops {
		out = append(out, o.notes...)
	}
	return out
}

// maxUndelivered bounds the notes kept for a sink that keeps failing. Past it the oldest are dropped, and NotesDropped says so for the rest of the session, so that a reader that counts on
// the notes (a raise that must not pay the same ground twice) can refuse instead of counting less than was written (#810).
const maxUndelivered = 1000

// deliverNotes hands the notes of edits that have just been written to the sink, after the ones an earlier delivery could not hand over. The caller holds s.mu. A sink that fails keeps
// every note for the next delivery (the file is written, the edits cannot be written again); it returns a short sentence for the answer of the write, or "" when all went through.
func (s *Save) deliverNotes(ops []Op) string {
	if s.sink == nil {
		return ""
	}
	return s.deliver(notesOf(ops))
}

func (s *Save) deliver(add []Note) string {
	n := append(append([]Note(nil), s.undelivered...), add...)
	if len(n) == 0 {
		return ""
	}
	if err := s.sink(n); err != nil {
		if len(n) > maxUndelivered {
			n = n[len(n)-maxUndelivered:]
			s.notesDropped = true
		}
		s.undelivered = n
		problem := "a note of the written edits could not be recorded (" + shortErr(err) + "); it is tried again at the next write"
		log.Printf("notes of written edits could not be recorded (kept for the next write): %s", shortErr(err))
		s.notesProblem = problem
		return problem
	}
	s.undelivered = nil
	s.notesProblem = ""
	return ""
}

// NotesProblem says why the notes of written edits could not be handed over ("" when they were, or when there is no sink). It outlives the write that found it, so a path that has no answer
// to carry a warning (the settling of an unverified write, a journal that outlived a write at start) is still seen, and it goes when a later delivery succeeds. The notes are kept in memory
// only: a restart before that delivery loses them.
func (s *Save) NotesProblem() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.notesProblem
}

// NotesDropped says that notes of written edits were dropped because a sink kept failing past the bound (maxUndelivered): what those notes recorded is known to nobody now, and it stays
// so until the editor is restarted. A raise that needs the record of what was paid refuses while it is true.
func (s *Save) NotesDropped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.notesDropped
}

// NotesDropped is Save.NotesDropped for an edit (the lock of the save is held already).
func (m *Mut) NotesDropped() bool {
	return m.s != nil && m.s.notesDropped
}

// deliverRecNotes is deliverNotes for edits kept in the journal (the editor was closed between the write and the removal of the journal). The caller holds s.mu.
func (s *Save) deliverRecNotes(recs []opRec) {
	ops := make([]Op, len(recs))
	for i, r := range recs {
		ops[i] = Op{notes: make([]Note, len(r.Notes))}
		for j, n := range r.Notes {
			n.Again = true // the journal outlived the write: they may have been handed over before
			ops[i].notes[j] = n
		}
	}
	s.deliverNotes(ops)
}
