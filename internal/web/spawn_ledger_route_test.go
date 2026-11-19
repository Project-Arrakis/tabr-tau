package web

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The repeat ledger of Create a vehicle (#740) is emptied when edits are dropped and only then (#760). Each route that reads the file again is run with a queue and without one:
// a reload that dropped a Create must let the same form write again, even when another form has taken its vehicle id; a reload that dropped nothing must keep the answer, so a
// retry after a lost answer does not write a second vehicle. Every test works on a temporary copy; no game is involved (a running game is a flag the tests set).

const (
	ledgerForm1 = `{"class":"Sandbike","template":"T0","requestId":"form-1"}`
	ledgerForm2 = `{"class":"Sandbike","template":"T0","requestId":"form-2"}`
)

// create sends a Create and returns its answer; it must have worked.
func create(t *testing.T, s *Server, body string) map[string]any {
	t.Helper()
	w := postEdit(t, s, "/api/vehicles/spawn", body)
	m := decodeBody(t, w)
	if w.Code != 200 || m["ok"] != true {
		t.Fatalf("Create: %d %s", w.Code, w.Body.String())
	}
	return m
}

// ledgerServer is a server with a journal, so that a queue can be put back by a reload.
func ledgerServer(t *testing.T) *Server {
	t.Helper()
	s := spawnServer(t)
	s.Save.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	return s
}

// gameAddsAnEntity is the game writing the file while edits wait: a row in a table the queued Create read, so its put-back conflicts.
func gameAddsAnEntity(t *testing.T, path string) {
	t.Helper()
	other, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err := other.Mutate("the game's entity", func(m *save.Mut) error {
		_, err := m.Exec(`insert into fgl_entities(entity_id,components) values (8300000000000099, jsonb('{}'))`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := other.CommitForTests(); err != nil {
		t.Fatal(err)
	}
}

// The sequence of the review: form-1 queued (it has an id), Discard, form-2 takes the id, the retry of form-1 writes again.
func TestACreateThatWasDiscardedWritesAgainEvenWhenAnotherFormTookItsId(t *testing.T) {
	restore := save.ActiveSessionForTests()
	defer restore()
	s := spawnServer(t)
	a := create(t, s, ledgerForm1)
	if a["outcome"] != "queued" {
		t.Fatalf("the Create waits in the queue: %v", a)
	}
	if w := postEdit(t, s, "/api/save/discard", `{}`); w.Code != 200 {
		t.Fatalf("discard: %d %s", w.Code, w.Body.String())
	}
	if n := ops.SpawnAnswerCount(s.Save); n != 0 {
		t.Fatalf("the Discard dropped the answer of the Create it threw away, %d left", n)
	}
	b := create(t, s, ledgerForm2)
	if b["id"] != a["id"] {
		t.Fatalf("the second form takes the same id (this is what the test needs): %v %v", a["id"], b["id"])
	}
	c := create(t, s, ledgerForm1)
	if c["repeat"] == true || c["outcome"] != "queued" {
		t.Fatalf("the retry of the discarded form writes again, it is not answered with the other form's vehicle: %v", c)
	}
	if p := s.Save.Pending(); len(p) != 2 {
		t.Fatalf("two vehicles wait, got %v", p)
	}
}

// A Discard with nothing queued reads the file and drops nothing: the answer of a Create that was written stays.
func TestADiscardWithNothingQueuedKeepsTheAnswerOfAWrittenCreate(t *testing.T) {
	s := spawnServer(t)
	a := create(t, s, ledgerForm1)
	if a["outcome"] != "written" || vehiclesInFile(t, s.Save.Path) != 1 {
		t.Fatalf("the Create is in the file: %v", a)
	}
	if w := postEdit(t, s, "/api/save/discard", `{}`); w.Code != 200 {
		t.Fatalf("discard: %d %s", w.Code, w.Body.String())
	}
	b := create(t, s, ledgerForm1)
	if b["repeat"] != true || b["id"] != a["id"] || vehiclesInFile(t, s.Save.Path) != 1 {
		t.Fatalf("the retry after the reload gets the first answer and writes nothing: %v", b)
	}
}

// The automatic refresh sends ifClean: with a queue it keeps everything (so does the ledger); with nothing queued it reads the file and drops nothing.
func TestTheCleanRefreshKeepsTheAnswersOfCreates(t *testing.T) {
	t.Run("queue kept", func(t *testing.T) {
		restore := save.ActiveSessionForTests()
		defer restore()
		s := spawnServer(t)
		a := create(t, s, ledgerForm1)
		w := postEdit(t, s, "/api/save/discard", `{"ifClean":true}`)
		if m := decodeBody(t, w); w.Code != 200 || m["kept"] != true {
			t.Fatalf("a queued edit is kept: %d %s", w.Code, w.Body.String())
		}
		b := create(t, s, ledgerForm1)
		if b["repeat"] != true || b["id"] != a["id"] || len(s.Save.Pending()) != 1 {
			t.Fatalf("the retry is answered and writes nothing: %v", b)
		}
	})
	t.Run("clean", func(t *testing.T) {
		s := spawnServer(t)
		a := create(t, s, ledgerForm1)
		w := postEdit(t, s, "/api/save/discard", `{"ifClean":true}`)
		if m := decodeBody(t, w); w.Code != 200 || m["kept"] == true {
			t.Fatalf("nothing is queued, so the file is read again: %d %s", w.Code, w.Body.String())
		}
		b := create(t, s, ledgerForm1)
		if b["repeat"] != true || b["id"] != a["id"] || vehiclesInFile(t, s.Save.Path) != 1 {
			t.Fatalf("a lost answer retried after the refresh does not write a second vehicle: %v", b)
		}
	})
}

// The automatic pass that waited for the game reads the file again (nothing queued) before it runs: that reload drops nothing either.
func TestTheAutomaticRefreshOfAWaitingPassKeepsTheAnswersOfCreates(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, spawnRouteWorld), config.Dir{Path: t.TempDir()})
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	if err := settings.Save(s.SettingsPath, settings.Settings{AutoRefillOnOpen: true}); err != nil {
		t.Fatal(err)
	}
	a := create(t, s, ledgerForm1)
	if vehiclesInFile(t, s.Save.Path) != 1 {
		t.Fatal("the Create is in the file")
	}
	s.startupMu.Lock()
	s.autoWaiting = true
	s.startupMu.Unlock()
	s.retryWaitingPass()
	if isWaiting(s) {
		t.Fatal("the pass ran, so the file was read again")
	}
	b := create(t, s, ledgerForm1)
	if b["repeat"] != true || b["id"] != a["id"] || vehiclesInFile(t, s.Save.Path) != 1 {
		t.Fatalf("the retry after the automatic refresh gets the first answer: %v", b)
	}
}

// Reload now (Rebase) that puts every edit back drops nothing: the queued Create is still there and a retry is answered with it.
func TestAReloadThatPutsTheCreateBackKeepsItsAnswer(t *testing.T) {
	restore := save.ActiveSessionForTests()
	defer restore()
	s := ledgerServer(t)
	a := create(t, s, ledgerForm1)
	w := postEdit(t, s, "/api/save/rebase", `{}`)
	m := decodeBody(t, w)
	r, _ := m["restored"].(map[string]any)
	if w.Code != 200 || r == nil || r["applied"] != float64(1) || r["conflicts"] != nil {
		t.Fatalf("the Create is put back: %d %s", w.Code, w.Body.String())
	}
	if n := ops.SpawnAnswerCount(s.Save); n != 1 {
		t.Fatalf("the answer of the Create that was put back stays, %d answers", n)
	}
	b := create(t, s, ledgerForm1)
	if b["repeat"] != true || b["id"] != a["id"] || len(s.Save.Pending()) != 1 {
		t.Fatalf("the retry is answered with the vehicle that was put back: %v", b)
	}
}

// A reload that cannot put the Create back drops it: the same form writes again, even when another form took the vehicle id since.
func TestAReloadThatDroppedTheCreateLetsItWriteAgain(t *testing.T) {
	restore := save.ActiveSessionForTests()
	s := ledgerServer(t)
	a := create(t, s, ledgerForm1)
	restore() // the game closes, having written the file meanwhile
	gameAddsAnEntity(t, s.Save.Path)
	w := postEdit(t, s, "/api/save/rebase", `{}`)
	m := decodeBody(t, w)
	r, _ := m["restored"].(map[string]any)
	if w.Code != 200 || r == nil || r["applied"] != float64(0) || r["conflicts"] == nil {
		t.Fatalf("the Create conflicts with the game's file and is dropped: %d %s", w.Code, w.Body.String())
	}
	if s.Save.Dirty() || vehiclesInFile(t, s.Save.Path) != 0 {
		t.Fatal("nothing of it is left")
	}
	if n := ops.SpawnAnswerCount(s.Save); n != 0 {
		t.Fatalf("the answer of the Create that was dropped is gone, %d answers", n)
	}
	b := create(t, s, ledgerForm2)
	if b["id"] != a["id"] {
		t.Fatalf("the second form takes the same id (this is what the test needs): %v %v", a["id"], b["id"])
	}
	c := create(t, s, ledgerForm1)
	if c["repeat"] == true || vehiclesInFile(t, s.Save.Path) != 2 {
		t.Fatalf("the retry of the dropped form writes again: %v, %d vehicles", c, vehiclesInFile(t, s.Save.Path))
	}
}

// A refused reload (nothing to put back because the edits were not kept) read nothing and dropped nothing: the answer stays.
func TestARefusedReloadKeepsTheAnswerOfACreate(t *testing.T) {
	restore := save.ActiveSessionForTests()
	defer restore()
	s := spawnServer(t) // no journal: the queued edit cannot be put back, so Reload now is refused and the queue is as it was
	a := create(t, s, ledgerForm1)
	if w := postEdit(t, s, "/api/save/rebase", `{}`); w.Code < 400 {
		t.Fatalf("a reload that cannot keep the edits is refused: %d %s", w.Code, w.Body.String())
	}
	b := create(t, s, ledgerForm1)
	if b["repeat"] != true || b["id"] != a["id"] || len(s.Save.Pending()) != 1 {
		t.Fatalf("the Create is still queued and the retry is answered: %v", b)
	}
}

// Restoring a backup that was made before the Create replaces the file: the vehicle is gone, and so is its answer, even when another form took the id.
func TestARestoreForgetsTheAnswersOfCreatesAndARefusedOneKeepsThem(t *testing.T) {
	s := spawnServer(t)
	a := create(t, s, ledgerForm1)
	backups := s.Save.Backups()
	if a["outcome"] != "written" || len(backups) == 0 || vehiclesInFile(t, s.Save.Path) != 1 {
		t.Fatalf("the Create is written after a backup: %v %v", a, backups)
	}
	name, _ := backups[0]["name"].(string)

	// a restore refused because an edit is queued reads nothing: the answer stays
	if _, err := s.Save.Mutate("a waiting edit", func(m *save.Mut) error {
		_, err := m.Exec(`update items set stack_size=stack_size+1 where template_id='SolarisCoin'`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	w := postEdit(t, s, "/api/save/restore", `{"name":"`+name+`"}`)
	if m := decodeBody(t, w); w.Code != 409 || m["code"] != "queued" {
		t.Fatalf("the restore is refused while an edit is queued: %d %s", w.Code, w.Body.String())
	}
	if b := create(t, s, ledgerForm1); b["repeat"] != true || b["id"] != a["id"] || ops.SpawnAnswerCount(s.Save) != 1 {
		t.Fatalf("a refused restore keeps the answer: %v", b)
	}
	if err := s.Save.Discard(); err != nil { // the waiting edit goes; the Create is in the file
		t.Fatal(err)
	}

	w = postEdit(t, s, "/api/save/restore", `{"name":"`+name+`"}`)
	if w.Code != 200 || vehiclesInFile(t, s.Save.Path) != 0 {
		t.Fatalf("the restore put the older file back: %d %s", w.Code, w.Body.String())
	}
	if n := ops.SpawnAnswerCount(s.Save); n != 0 {
		t.Fatalf("the restore replaced the file and the memory is emptied, %d answers", n)
	}
	b := create(t, s, ledgerForm2)
	if b["id"] != a["id"] {
		t.Fatalf("the second form takes the same id (this is what the test needs): %v %v", a["id"], b["id"])
	}
	c := create(t, s, ledgerForm1)
	if c["repeat"] == true || vehiclesInFile(t, s.Save.Path) != 2 {
		t.Fatalf("after a restore the retry of the first form writes again: %v, %d vehicles", c, vehiclesInFile(t, s.Save.Path))
	}
}

// A queue that is held takes no new Create. The retry is refused, and the answer is not lost: when the held queue is thrown away, the Create it held is dropped with it and the same
// form writes again, even when another form has taken the id.
func TestACreateRetriedWhileTheQueueIsHeldIsRefusedAndTheQueueIsDiscardedLater(t *testing.T) {
	restore := save.ActiveSessionForTests()
	s := ledgerServer(t)
	a := create(t, s, ledgerForm1)
	restore()
	gameAddsAnEntity(t, s.Save.Path)
	if f := s.Save.Flush(); f.Saved || s.Save.Held() == nil {
		t.Fatalf("the queued Create conflicts with the game's file and is held: %+v", f)
	}
	w := postEdit(t, s, "/api/vehicles/spawn", ledgerForm1)
	if m := decodeBody(t, w); w.Code != 409 || m["code"] != "queue_held" {
		t.Fatalf("a held queue takes no Create: %d %s", w.Code, w.Body.String())
	}
	if w := postEdit(t, s, "/api/save/discard", `{}`); w.Code != 200 || s.Save.Held() != nil {
		t.Fatalf("the held queue is discarded: %d %s", w.Code, w.Body.String())
	}
	if n := ops.SpawnAnswerCount(s.Save); n != 0 {
		t.Fatalf("the discarded queue took its answers with it, %d left", n)
	}
	b := create(t, s, ledgerForm2)
	if b["id"] != a["id"] {
		t.Fatalf("the second form takes the same id (this is what the test needs): %v %v", a["id"], b["id"])
	}
	c := create(t, s, ledgerForm1)
	if c["repeat"] == true || vehiclesInFile(t, s.Save.Path) != 2 {
		t.Fatalf("after the discard the retry writes again: %v, %d vehicles", c, vehiclesInFile(t, s.Save.Path))
	}
}

// Ids are reused: the next vehicle takes the highest id plus one. An answer whose vehicle left the working copy without any route being told (the file was replaced from
// outside and read again; the working copy rebased onto a newer file) must not be handed out for another form's vehicle that took the same id (#760).

func replaceSaveFile(t *testing.T, path string, with []byte) {
	t.Helper()
	if err := os.WriteFile(path, with, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAnAnswerOfAVehicleThatLeftTheFileIsNotHandedOutAfterACleanReload(t *testing.T) {
	s := spawnServer(t)
	older, err := os.ReadFile(s.Save.Path)
	if err != nil {
		t.Fatal(err)
	}
	a := create(t, s, ledgerForm1)
	if a["outcome"] != "written" || vehiclesInFile(t, s.Save.Path) != 1 {
		t.Fatalf("the Create is in the file: %v", a)
	}
	replaceSaveFile(t, s.Save.Path, older) // an older file put back from outside
	if w := postEdit(t, s, "/api/save/discard", `{"ifClean":true}`); w.Code != 200 {
		t.Fatalf("the refresh: %d %s", w.Code, w.Body.String())
	}
	if vehiclesInFile(t, s.Save.Path) != 0 {
		t.Fatal("the refresh read the older file")
	}
	b := create(t, s, ledgerForm2)
	if b["id"] != a["id"] {
		t.Fatalf("the second form takes the same id (this is what the test needs): %v %v", a["id"], b["id"])
	}
	c := create(t, s, ledgerForm1)
	if c["repeat"] == true || vehiclesInFile(t, s.Save.Path) != 2 {
		t.Fatalf("the retry of the first form writes its own vehicle: %v, %d vehicles", c, vehiclesInFile(t, s.Save.Path))
	}
}

func TestAnAnswerOfAVehicleThatLeftTheFileIsNotHandedOutAfterAWaitingEditWasRebased(t *testing.T) {
	s := ledgerServer(t)
	older, err := os.ReadFile(s.Save.Path)
	if err != nil {
		t.Fatal(err)
	}
	a := create(t, s, ledgerForm1)
	if a["outcome"] != "written" {
		t.Fatalf("the Create is written: %v", a)
	}
	if _, err := s.Save.Mutate("a waiting edit", func(m *save.Mut) error {
		_, err := m.Exec(`update items set stack_size=stack_size+1 where template_id='SolarisCoin'`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	replaceSaveFile(t, s.Save.Path, older) // the file is another one when the waiting edit is written
	if f := s.Save.Flush(); !f.Saved {
		t.Fatalf("the waiting edit is put onto the newer file and written: %+v", f)
	}
	if vehiclesInFile(t, s.Save.Path) != 0 {
		t.Fatal("the file the edit was put onto has no vehicle")
	}
	b := create(t, s, ledgerForm2)
	if b["id"] != a["id"] {
		t.Fatalf("the second form takes the same id (this is what the test needs): %v %v", a["id"], b["id"])
	}
	c := create(t, s, ledgerForm1)
	if c["repeat"] == true || vehiclesInFile(t, s.Save.Path) != 2 {
		t.Fatalf("the retry of the first form writes its own vehicle: %v, %d vehicles", c, vehiclesInFile(t, s.Save.Path))
	}
}

// Reload now that drops one edit and puts the other back (a Create and an unrelated edit, of which the game's file no longer has the row): the Create is still queued and a
// retry of it is answered; the answer is not forgotten with the edit that was dropped.
func TestAReloadThatDroppedAnotherEditKeepsTheAnswerOfTheCreateThatWasPutBack(t *testing.T) {
	restore := save.ActiveSessionForTests()
	s := ledgerServer(t)
	a := create(t, s, ledgerForm1)
	if _, err := s.Save.Mutate("an unrelated edit", func(m *save.Mut) error {
		_, err := m.Exec(`update items set stack_size=stack_size+1 where template_id='SolarisCoin'`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	restore() // the game closes, having removed the row the other edit works on
	other, err := save.Open(s.Save.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Mutate("the game's change", func(m *save.Mut) error {
		_, err := m.Exec(`delete from items where template_id='SolarisCoin'`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := other.CommitForTests(); err != nil {
		t.Fatal(err)
	}
	other.Close()
	w := postEdit(t, s, "/api/save/rebase", `{}`)
	m := decodeBody(t, w)
	r, _ := m["restored"].(map[string]any)
	if w.Code != 200 || r == nil || r["applied"] != float64(1) || r["conflicts"] == nil {
		t.Fatalf("the Create is put back and the other edit is dropped: %d %s", w.Code, w.Body.String())
	}
	if n := ops.SpawnAnswerCount(s.Save); n != 1 {
		t.Fatalf("the answer of the Create that was put back stays, %d answers", n)
	}
	b := create(t, s, ledgerForm1)
	if b["repeat"] != true || b["id"] != a["id"] {
		t.Fatalf("the retry is answered with the Create that was put back: %v", b)
	}
	// the put-back Create is written by the request that follows (the game is closed); the retry wrote nothing more
	if n := vehiclesInFile(t, s.Save.Path); n != 1 || len(s.Save.Pending()) != 0 {
		t.Fatalf("one vehicle in the file and nothing waiting, got %d vehicles, %v waiting", n, s.Save.Pending())
	}
}

// A Create retried while the queue is held is refused; when the game's next file no longer conflicts the queue is written, and the retry that follows is answered with that
// vehicle: exactly one vehicle in the file. (The refused retry must not have forgotten the answer.)
func TestACreateRefusedWhileTheQueueWasHeldIsAnsweredAfterTheQueueIsWritten(t *testing.T) {
	restore := save.ActiveSessionForTests()
	s := ledgerServer(t)
	a := create(t, s, ledgerForm1)
	restore()
	gameAddsAnEntity(t, s.Save.Path)
	if f := s.Save.Flush(); f.Saved || s.Save.Held() == nil {
		t.Fatalf("the queued Create conflicts with the game's file and is held: %+v", f)
	}
	if w := postEdit(t, s, "/api/vehicles/spawn", ledgerForm1); w.Code != 409 || decodeBody(t, w)["code"] != "queue_held" {
		t.Fatalf("a held queue takes no Create: %d %s", w.Code, w.Body.String())
	}
	// the game's next file no longer has what the queue conflicted with
	other, err := save.Open(s.Save.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Mutate("the game's change", func(m *save.Mut) error {
		_, err := m.Exec(`delete from fgl_entities where entity_id=8300000000000099`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := other.CommitForTests(); err != nil {
		t.Fatal(err)
	}
	other.Close()
	if f := s.Save.Flush(); !f.Saved || s.Save.Held() != nil {
		t.Fatalf("the queue no longer conflicts and is written: %+v", f)
	}
	if vehiclesInFile(t, s.Save.Path) != 1 {
		t.Fatalf("the held Create is in the file, got %d vehicles", vehiclesInFile(t, s.Save.Path))
	}
	b := create(t, s, ledgerForm1)
	if b["repeat"] != true || b["id"] != a["id"] || vehiclesInFile(t, s.Save.Path) != 1 {
		t.Fatalf("the retry gets the first answer and writes nothing: %v, %d vehicles", b, vehiclesInFile(t, s.Save.Path))
	}
}

// A Discard that throws away an unrelated edit leaves the answer of a Create that is already in the file (a retry after a lost answer still gets its answer and writes nothing),
// and one that throws away a queued Create takes only that answer.
func TestADiscardKeepsTheAnswersOfWrittenCreatesAndDropsThoseOfQueuedOnes(t *testing.T) {
	s := spawnServer(t)
	a := create(t, s, ledgerForm1) // written
	if a["outcome"] != "written" {
		t.Fatalf("the Create is in the file: %v", a)
	}
	if _, err := s.Save.Mutate("an unrelated edit", func(m *save.Mut) error {
		_, err := m.Exec(`update items set stack_size=stack_size+1 where template_id='SolarisCoin'`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if w := postEdit(t, s, "/api/save/discard", `{}`); w.Code != 200 {
		t.Fatalf("discard: %d %s", w.Code, w.Body.String())
	}
	if n := ops.SpawnAnswerCount(s.Save); n != 1 {
		t.Fatalf("the answer of the Create in the file stays, %d answers", n)
	}
	if b := create(t, s, ledgerForm1); b["repeat"] != true || b["id"] != a["id"] || vehiclesInFile(t, s.Save.Path) != 1 {
		t.Fatalf("the retry gets the first answer and writes nothing: %v, %d vehicles", b, vehiclesInFile(t, s.Save.Path))
	}
	// a queued Create is thrown away with its answer, the written one keeps its own
	restore := save.ActiveSessionForTests()
	defer restore()
	q := create(t, s, ledgerForm2)
	if q["outcome"] != "queued" || ops.SpawnAnswerCount(s.Save) != 2 {
		t.Fatalf("the second Create waits: %v", q)
	}
	if w := postEdit(t, s, "/api/save/discard", `{}`); w.Code != 200 {
		t.Fatalf("discard: %d %s", w.Code, w.Body.String())
	}
	if n := ops.SpawnAnswerCount(s.Save); n != 1 {
		t.Fatalf("only the answer of the queued Create goes, %d answers", n)
	}
	if b := create(t, s, ledgerForm1); b["repeat"] != true || b["id"] != a["id"] {
		t.Fatalf("the written Create keeps its answer: %v", b)
	}
}

// A vehicle the game wrote under the id of a Create that was dropped is not that Create's vehicle (#760): after Discard and after Reload now the answer is dropped, and a retry
// writes the Create again instead of being answered with the game's vehicle. The game's vehicle carries the Actor and VehicleInventory entities every vehicle has, and is told apart
// by its class and by the ids of those entities, each on its own (#793): the same class under other entity ids, and another class under the very ids of the Create's entities.
func TestAVehicleOfTheGameUnderTheIdOfADroppedCreateIsNotTakenForIt(t *testing.T) {
	for _, tc := range []struct {
		name     string
		sameCls  bool
		ownLinks bool
	}{{"the same class, other entities", true, false}, {"another class, the Create's own entities", false, true}} {
		for _, how := range []string{"discard", "reload"} {
			t.Run(tc.name+"/"+how, func(t *testing.T) {
				restore := save.ActiveSessionForTests()
				s := ledgerServer(t)
				a := create(t, s, ledgerForm1) // queued
				rows := a["rows"].(map[string]any)
				class := "/Game/Dune/Systems/Vehicles/Blueprints/GroundVehicles/BP_Sandbike.BP_Sandbike_C"
				if tc.sameCls {
					class = rows["actors"].([]any)[0].(map[string]any)["class"].(string)
				}
				links := map[string]int64{"Actor": 8300000000000201, "VehicleInventory": 8300000000000202} // ids no entity of the Create has
				if tc.ownLinks {
					links = map[string]int64{}
					for _, e := range rows["entities"].([]any) {
						e := e.(map[string]any)
						links[e["slot_name"].(string)] = int64(e["entity_id"].(float64)) // test ids stay below 2^53, so the JSON number is exact
					}
				}
				if len(links) != 2 {
					t.Fatalf("a Create links an Actor and a VehicleInventory entity: %v", links)
				}
				restore() // the game closes, having written a vehicle of its own under the id of the queued one
				other, err := save.Open(s.Save.Path)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := other.Mutate("the game's vehicle", func(m *save.Mut) error {
					if _, err := m.Exec(`insert into actors(id,class,map,state) values (?,?,'HaggaBasin',0)`, a["id"], class); err != nil {
						return err
					}
					if _, err := m.Exec(`insert into vehicles(id) values (?)`, a["id"]); err != nil {
						return err
					}
					for slot, ent := range links {
						if _, err := m.Exec(`insert into fgl_entities(entity_id,components) values (?, jsonb('{}'))`, ent); err != nil {
							return err
						}
						if _, err := m.Exec(`insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (?,?,?)`, a["id"], ent, slot); err != nil {
							return err
						}
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				if _, err := other.CommitForTests(); err != nil {
					t.Fatal(err)
				}
				other.Close()
				route := "/api/save/discard"
				if how == "reload" {
					route = "/api/save/rebase"
				}
				if w := postEdit(t, s, route, `{}`); w.Code != 200 {
					t.Fatalf("%s: %d %s", route, w.Code, w.Body.String())
				}
				if n := ops.SpawnAnswerCount(s.Save); n != 0 {
					t.Fatalf("the answer names the game's vehicle, not the Create's: it is dropped, %d answers", n)
				}
				b := create(t, s, ledgerForm1)
				if b["repeat"] == true || b["id"] == a["id"] || vehiclesInFile(t, s.Save.Path) != 2 {
					t.Fatalf("the retry writes the Create again next to the game's vehicle: %v, %d vehicles", b, vehiclesInFile(t, s.Save.Path))
				}
			})
		}
	}
}
