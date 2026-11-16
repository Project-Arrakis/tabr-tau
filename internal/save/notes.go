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

// Pending returns the notes of the kind that edits already queued (made, not yet written) carry, oldest first. It is read inside an edit, so it sees what the edits before it asked
// to be remembered, which is not in the sink's store until they are written.
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

// pendingNotes: the caller holds s.mu. A queue that is held (it could not be put onto the game's newer file) is not pending: none of it is in the working copy.
func (s *Save) pendingNotes(kind string) []json.RawMessage {
	var out []json.RawMessage
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

// deliverNotes hands the notes of edits that have just been written to the sink. The caller holds s.mu.
func (s *Save) deliverNotes(ops []Op) {
	if s.sink == nil {
		return
	}
	n := notesOf(ops)
	if len(n) == 0 {
		return
	}
	if err := s.sink(n); err != nil {
		log.Printf("notes of written edits could not be recorded: %v", err)
	}
}

// deliverRecNotes is deliverNotes for edits kept in the journal (the editor was closed between the write and the removal of the journal). The caller holds s.mu.
func (s *Save) deliverRecNotes(recs []opRec) {
	ops := make([]Op, len(recs))
	for i, r := range recs {
		ops[i] = Op{notes: r.Notes}
	}
	s.deliverNotes(ops)
}
