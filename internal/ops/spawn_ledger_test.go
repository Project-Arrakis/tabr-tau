package ops

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

func ledgerHas(s *save.Save, id string) bool {
	spawnAnswers.Lock()
	defer spawnAnswers.Unlock()
	for k := range spawnAnswers.seen {
		if k.s == s && k.id == id {
			return true
		}
	}
	return false
}

// With the queue held, the working copy is the game's file: the vehicle the held Create made is not in it. A retry of that form is refused (a held queue takes no new edit) and its
// answer is not dropped by the refused attempt (#760): the vehicle is still in the held edits, and a hold that clears later must find the answer.
func TestACreateRefusedBecauseTheQueueIsHeldKeepsItsAnswer(t *testing.T) {
	o := spawnOps(t)
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	restore := save.ActiveSessionForTests()
	a := Args{"class": "Buggy", "template": "T0", "requestId": "form-h"}
	first, _ := spawn(t, o, a)
	if !o.S.Dirty() || !ledgerHas(o.S, "form-h") {
		t.Fatal("the Create is queued and remembered")
	}
	restore() // the game closes, having written the file meanwhile
	other, err := save.Open(o.S.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Mutate("the game's entity", func(m *save.Mut) error {
		_, err := m.Exec(`insert into fgl_entities(entity_id,components) values (8300000000000099, jsonb('{}'))`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := other.CommitForTests(); err != nil {
		t.Fatal(err)
	}
	other.Close()
	if f := o.S.Flush(); f.Saved || o.S.Held() == nil {
		t.Fatalf("the queued Create conflicts with the game's file and is held: %+v", f)
	}
	if mustOne(t, o, `select count(*) c from vehicles where id=?`, first)["c"].(int64) != 0 {
		t.Fatal("the held vehicle is not in the working copy")
	}
	_, err = o.SpawnVehicle(a)
	if !errors.Is(err, apierr.ErrQueueHeld) {
		t.Fatalf("a held queue refuses the retry: %v", err)
	}
	if !ledgerHas(o.S, "form-h") {
		t.Fatal("the refused retry must not drop the answer of the Create the held queue still has")
	}
	// another form is not affected by the entry
	if _, err := o.SpawnVehicle(Args{"class": "Buggy", "template": "T0", "requestId": "form-i"}); !errors.Is(err, apierr.ErrQueueHeld) || ledgerHas(o.S, "form-i") {
		t.Fatalf("a refused Create of another form is not remembered: %v", err)
	}
}

// A stale answer (its vehicle is not in the working copy) is replaced by the new one when the write works.
func TestAStaleAnswerIsReplacedWhenTheCreateWritesAgain(t *testing.T) {
	o := spawnOps(t)
	a := Args{"class": "Buggy", "template": "T0", "requestId": "form-r"}
	first, _ := spawn(t, o, a)
	if err := o.S.Discard(); err != nil { // the ledger is not told: the entry is stale
		t.Fatal(err)
	}
	again, ans := spawn(t, o, a)
	if ans["repeat"] == true || !ledgerHas(o.S, "form-r") {
		t.Fatalf("the write is new and remembered: %v", ans)
	}
	spawnAnswers.Lock()
	n := 0
	for k := range spawnAnswers.seen {
		if k.s == o.S {
			n++
		}
	}
	spawnAnswers.Unlock()
	if n != 1 {
		t.Fatalf("one entry for the form, got %d", n)
	}
	if third, ans := spawn(t, o, a); third != again || ans["repeat"] != true {
		t.Fatalf("the new answer is what a repeat gets: %d %d %v", first, third, ans["repeat"])
	}
}
