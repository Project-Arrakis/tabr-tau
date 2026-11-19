//go:build windows

package save

import (
	"os"
	"strings"
	"testing"
)

// A write that could not be read back has its edits in the game's file and their notes still to hand over (settle.go). A Discard must not throw them away with the queue (#799): the
// file is looked at first, and when it is the one that was written the notes go to the sink once.
func TestADiscardAfterAWriteThatCouldNotBeReadBackHandsOverTheNotes(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s := openJournaled(t, p, t.TempDir())
	var k sunk
	s.SetNoteSink(k.sink)
	noteEdit(t, s, 1, "x", 7)
	release := holdAfterRename(t, p)
	if f := s.Flush(); f.Saved {
		t.Fatalf("a write that could not be checked is not saved: %+v", f)
	}
	if len(k.got) != 0 {
		t.Fatalf("a note was handed over before the write was known to be in the file: %v", k.got)
	}
	release()
	dropped, err := s.DiscardReport()
	if err != nil {
		t.Fatal(err)
	}
	if dropped {
		t.Fatal("the edit was written, so nothing was thrown away")
	}
	if got := values(t, k.got); len(got) != 1 || got[0] != 7 {
		t.Fatalf("the note of the written edit was lost by the Discard: %v", got)
	}
	// once only: a later write hands over nothing more of it
	s.Flush()
	if got := values(t, k.got); len(got) != 1 {
		t.Fatalf("the note was handed over again: %v", got)
	}
	if s.Dirty() {
		t.Fatal("nothing is pending")
	}
}

// While the file still cannot be read back nothing is discarded and the queue is as it was; once it can be read the next attempt settles it and the note goes over.
func TestADiscardWhileTheWriteIsStillUnverifiedThrowsNothingAway(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s := openJournaled(t, p, t.TempDir())
	var k sunk
	s.SetNoteSink(k.sink)
	noteEdit(t, s, 1, "x", 7)
	release := holdAfterRename(t, p)
	if f := s.Flush(); f.Saved {
		t.Fatalf("unverified: %+v", f)
	}
	if err := s.Discard(); err == nil || !strings.Contains(err.Error(), "could not be verified") {
		t.Fatalf("a Discard while the file cannot be read says why: %v", err)
	}
	if !s.Dirty() || len(k.got) != 0 {
		t.Fatalf("the queue was thrown away or a note was handed over early: dirty %v, notes %v", s.Dirty(), k.got)
	}
	release()
	if f := s.Flush(); !f.Saved {
		t.Fatalf("the written file is recognised: %+v", f)
	}
	if got := values(t, k.got); len(got) != 1 || got[0] != 7 {
		t.Fatalf("the note of the written edit: %v", got)
	}
}

// A Discard when the file is not the one that was written (the game saved since) drops the edits as it always did, and does not hand over notes of edits that are not in the file.
func TestADiscardWhenTheFileIsNotTheWrittenOneStillDrops(t *testing.T) {
	defer NoGameRunningForTests()()
	p := makeSave(t, journalDDL)
	s := openJournaled(t, p, t.TempDir())
	var k sunk
	s.SetNoteSink(k.sink)
	noteEdit(t, s, 1, "x", 7)
	release := holdAfterRename(t, p)
	s.Flush()
	release()
	other := makeSave(t, journalDDL) // a different file put in place of the written one
	b, err := os.ReadFile(other)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DiscardReport(); err != nil {
		t.Fatal(err)
	}
	if len(k.got) != 0 || s.Dirty() {
		t.Fatalf("notes of edits that are not in the file were handed over, or the queue stayed: %v %v", k.got, s.Dirty())
	}
}
