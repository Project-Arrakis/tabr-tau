package save

import (
	"errors"
	"testing"
)

func tokenOf(t *testing.T, s *Save) string {
	t.Helper()
	tok, err := s.ReviewToken()
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// The token binds the data, not just the descriptions: two different edits with the same description and the same
// count give different tokens, and an edit with no description at all still changes it.
func TestReviewTokenBindsTheData(t *testing.T) {
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	base := tokenOf(t, s)
	s.Mutate("same words", func(m *Mut) error { _, err := m.Exec(`update t set a=2`); return err })
	t2 := tokenOf(t, s)
	if t2 == base {
		t.Fatal("an edit must change the token")
	}
	if err := s.Discard(); err != nil {
		t.Fatal(err)
	}
	s.Mutate("same words", func(m *Mut) error { _, err := m.Exec(`update t set a=3`); return err })
	if t3 := tokenOf(t, s); t3 == t2 {
		t.Fatal("same description and count but different data must give a different token")
	}
	if err := s.Discard(); err != nil {
		t.Fatal(err)
	}
	if tokenOf(t, s) != base {
		t.Fatal("a discarded session must return to the original token")
	}
	s.Mutate("", func(m *Mut) error { _, err := m.Exec(`update t set a=9`); return err }) // dirty, but no recorded op
	if !s.Dirty() || len(s.Ops()) != 0 {
		t.Fatal("test setup: expected a dirty save with no recorded op")
	}
	if tokenOf(t, s) == base {
		t.Fatal("an undescribed edit must still change the token")
	}
}

// The token is checked under the same lock as the write: a token from before an edit cannot save that edit.
func TestCommitReviewedRefusesStaleAndMissingTokens(t *testing.T) {
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	s.Mutate("first", func(m *Mut) error { _, err := m.Exec(`update t set a=2`); return err })
	reviewed := tokenOf(t, s)
	s.Mutate("sneaks in after the review", func(m *Mut) error { _, err := m.Exec(`update t set a=3`); return err })
	if _, err := s.CommitReviewed(reviewed, false); !errors.Is(err, ErrReviewChanged) {
		t.Fatalf("want ErrReviewChanged, got %v", err)
	}
	if _, err := s.CommitReviewed("", false); err == nil {
		t.Fatal("an empty token must be refused")
	}
	if !s.Dirty() {
		t.Fatal("refused saves must leave the edits pending")
	}
	if res, err := s.CommitReviewed(tokenOf(t, s), false); err != nil || res["saved"] != true {
		t.Fatalf("the current token must save: %v %v", res, err)
	}
}

// Restoring a backup reloads the file, which would drop pending edits; it refuses instead.
func TestRestoreRefusesWithPendingEdits(t *testing.T) {
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	s.Exec(`update t set a=2`)
	if _, err := s.Commit(false); err != nil {
		t.Fatal(err)
	}
	name := s.Backups()[0]["name"].(string)
	s.Exec(`update t set a=3`)
	if err := s.Restore(name); err == nil {
		t.Fatal("restore must refuse while there are unsaved edits")
	}
	if !s.Dirty() {
		t.Fatal("the pending edit must survive the refusal")
	}
}
