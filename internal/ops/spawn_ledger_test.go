package ops

import (
	"errors"
	"path/filepath"
	"strings"
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
	if seen, order := ledgerEntries(o.S); seen != 1 || order != 1 {
		t.Fatalf("one entry for the form, in the answers and in the order: %d, %d", seen, order)
	}
	if third, ans := spawn(t, o, a); third != again || ans["repeat"] != true {
		t.Fatalf("the new answer is what a repeat gets: %d %d %v", first, third, ans["repeat"])
	}
	// The stale answer's id is not the id of the new vehicle (another vehicle is above it): the old entry must be replaced, not left in the order beside the new one.
	second, _ := spawn(t, o, Args{"class": "Buggy", "template": "T0", "requestId": "form-s"})
	if _, err := o.S.MutateCascade("remove a vehicle", func(m *save.Mut) error {
		_, err := m.Exec(`delete from vehicles where id=?`, again)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	fresh, ans := spawn(t, o, a)
	if ans["repeat"] == true || fresh == again || fresh == second {
		t.Fatalf("the stale answer is not a repeat and the vehicle has an id of its own: %d %d %d %v", again, second, fresh, ans["repeat"])
	}
	if seen, order := ledgerEntries(o.S); seen != 2 || order != 2 {
		t.Fatalf("two forms, each once in the answers and in the order: %d, %d", seen, order)
	}
}

func ledgerEntries(s *save.Save) (seen, order int) {
	spawnAnswers.Lock()
	defer spawnAnswers.Unlock()
	for k := range spawnAnswers.seen {
		if k.s == s {
			seen++
		}
	}
	for _, k := range spawnAnswers.order {
		if k.s == s {
			order++
		}
	}
	return
}

// Ids are reused (the next one is the highest plus one). An answer whose vehicle left the working copy without the ledger being told (a file replaced from outside and read
// again, a restore that could not be read back) must not be handed out for another form's vehicle that took the same id (#760).
func TestAStaleAnswerIsNotHandedOutForAnotherFormsVehicle(t *testing.T) {
	o := spawnOps(t)
	vehicles := func() int64 { return mustOne(t, o, `select count(*) c from vehicles`)["c"].(int64) }
	before := vehicles()
	first, _ := spawn(t, o, Args{"class": "Buggy", "template": "T0", "requestId": "form-1"})
	if err := o.S.Discard(); err != nil { // the vehicle is gone; the ledger is not told
		t.Fatal(err)
	}
	second, _ := spawn(t, o, Args{"class": "Buggy", "template": "T0", "requestId": "form-2"})
	if second != first {
		t.Fatalf("the second form takes the id of the first (this is what the test needs): %d %d", first, second)
	}
	if ledgerHas(o.S, "form-1") {
		t.Fatal("the answer that names the id the new vehicle took is dropped when it is written")
	}
	again, ans := spawn(t, o, Args{"class": "Buggy", "template": "T0", "requestId": "form-1"})
	if ans["repeat"] == true || again == second || vehicles() != before+2 {
		t.Fatalf("the retry of the first form writes its own vehicle: %d, repeat %v, %d vehicles", again, ans["repeat"], vehicles())
	}
	// and an answer of another save that names the same id is not touched by this save's write
	a, b := spawnOps(t), spawnOps(t)
	x, _ := spawn(t, b, Args{"class": "Buggy", "template": "T0", "requestId": "form-x"})
	y, _ := spawn(t, a, Args{"class": "Buggy", "template": "T0", "requestId": "form-y"})
	if x != y {
		t.Fatalf("the two saves give the same id (this is what the test needs): %d %d", x, y)
	}
	if !ledgerHas(b.S, "form-x") || !ledgerHas(a.S, "form-y") {
		t.Fatal("another save's answers are not touched")
	}
}

// PruneSpawnAnswers drops the answers whose vehicle is not in the working copy and keeps the others, of its own save only.
func TestPruneSpawnAnswersKeepsTheAnswersOfVehiclesThatAreThere(t *testing.T) {
	o := spawnOps(t)
	other := spawnOps(t)
	keep, _ := spawn(t, o, Args{"class": "Buggy", "template": "T0", "requestId": "form-k"})
	gone, _ := spawn(t, o, Args{"class": "Buggy", "template": "T0", "requestId": "form-g"})
	// the other save's vehicle has an id this save does not have
	spawn(t, other, Args{"class": "Buggy", "template": "T0"})
	spawn(t, other, Args{"class": "Buggy", "template": "T0"})
	theirs, _ := spawn(t, other, Args{"class": "Buggy", "template": "T0", "requestId": "form-o"})
	if theirs == keep || theirs == gone {
		t.Fatalf("the other save's id is one this save lacks: %d %d %d", keep, gone, theirs)
	}
	if _, err := o.S.MutateCascade("remove a vehicle", func(m *save.Mut) error {
		_, err := m.Exec(`delete from vehicles where id=?`, gone)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	PruneSpawnAnswers(o.S)
	if !ledgerHas(o.S, "form-k") || ledgerHas(o.S, "form-g") || !ledgerHas(other.S, "form-o") {
		t.Fatalf("the answer of the vehicle that is there stays, the other goes, another save is untouched (%d %d)", keep, gone)
	}
	if seen, order := ledgerEntries(o.S); seen != 1 || order != 1 {
		t.Fatalf("the ledger stays consistent: %d answers, %d in the order", seen, order)
	}
}

// gameTakesTheId is the game writing a vehicle of its own under an id: a row in actors and in vehicles, of the class given.
func gameTakesTheId(t *testing.T, o *Ops, id int64, class string) {
	t.Helper()
	if _, err := o.S.Mutate("a vehicle of the game's", func(m *save.Mut) error {
		if _, err := m.Exec(`insert into actors(id,class,map,state) values (?,?,'HaggaBasin',0)`, id, class); err != nil {
			return err
		}
		_, err := m.Exec(`insert into vehicles(id) values (?)`, id)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

// A vehicle the game made under the id of a Create that was dropped is not that Create's vehicle: the answer is dropped by a prune and a retry writes the Create again (#760). It
// is the case with another class and the worst case, the same class (the entities tell them apart).
func TestAVehicleOfTheGameUnderTheIdOfADroppedCreateIsNotTakenForIt(t *testing.T) {
	for name, class := range map[string]string{"another class": "/Game/Dune/Systems/Vehicles/Blueprints/GroundVehicles/BP_Sandbike.BP_Sandbike_C", "the same class": ""} {
		t.Run(name, func(t *testing.T) {
			for _, how := range []string{"prune", "retry"} {
				t.Run(how, func(t *testing.T) {
					o := spawnOps(t)
					a := Args{"class": "Buggy", "template": "T0", "requestId": "form-1"}
					first, ans := spawn(t, o, a)
					if class == "" {
						class = ans["rows"].(map[string]any)["actors"].([]save.Row)[0]["class"].(string)
					}
					if err := o.S.Discard(); err != nil { // the Create is dropped
						t.Fatal(err)
					}
					gameTakesTheId(t, o, first, class)
					if how == "prune" {
						PruneSpawnAnswers(o.S)
						if ledgerHas(o.S, "form-1") {
							t.Fatal("the answer names a vehicle that is not the Create's: it is dropped")
						}
					}
					again, ans := spawn(t, o, a)
					if ans["repeat"] == true || again == first {
						t.Fatalf("the retry writes the Create again, it is not answered with the game's vehicle: %d, repeat %v", again, ans["repeat"])
					}
					if mustOne(t, o, `select count(*) c from vehicles where id=?`, again)["c"].(int64) != 1 {
						t.Fatal("the vehicle of the retry is in the working copy")
					}
				})
			}
		})
	}
}

// The Create's own vehicle that was put back, or is in the file, keeps its answer through a prune.
func TestPruneSpawnAnswersKeepsTheCreatesOwnVehicle(t *testing.T) {
	o := spawnOps(t)
	first, _ := spawn(t, o, Args{"class": "Buggy", "template": "T0", "requestId": "form-1"})
	PruneSpawnAnswers(o.S)
	again, ans := spawn(t, o, Args{"class": "Buggy", "template": "T0", "requestId": "form-1"})
	if again != first || ans["repeat"] != true {
		t.Fatalf("the answer of a vehicle that is there stays: %d %d %v", first, again, ans["repeat"])
	}
}

// A queue that is held shows the game's file, which has none of the held Creates: nothing is pruned then.
func TestPruneSpawnAnswersDropsNothingWhileTheQueueIsHeld(t *testing.T) {
	o := spawnOps(t)
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	restore := save.ActiveSessionForTests()
	spawn(t, o, Args{"class": "Buggy", "template": "T0", "requestId": "form-h"})
	restore()
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
		t.Fatalf("the queue is held: %+v", f)
	}
	PruneSpawnAnswers(o.S)
	if !ledgerHas(o.S, "form-h") {
		t.Fatal("a held queue's answers are kept")
	}
}

// A vehicle made without the memory (a Create without an id, Duplicate vehicle) that takes the id of an answer drops that answer: the memory only forgets by id otherwise (#760).
func TestAVehicleMadeOutsideTheMemoryDropsTheAnswersThatNameItsId(t *testing.T) {
	for _, how := range []string{"create without an id", "duplicate vehicle"} {
		t.Run(how, func(t *testing.T) {
			o := spawnOps(t)
			first, _ := spawn(t, o, Args{"class": "Buggy", "template": "T0", "requestId": "form-1"})
			if err := o.S.Discard(); err != nil { // the Create is dropped and the memory is not told
				t.Fatal(err)
			}
			if !ledgerHas(o.S, "form-1") {
				t.Fatal("the answer is still there (this is what the test needs)")
			}
			var taken int64
			if how == "duplicate vehicle" {
				r, err := o.DuplicateVehicle(Args{"id": 578.0})
				if err != nil {
					t.Fatal(err)
				}
				taken = r.(map[string]any)["id"].(int64)
			} else {
				taken, _ = spawn(t, o, Args{"class": "Buggy", "template": "T0"})
			}
			if taken != first {
				t.Fatalf("the new vehicle takes the id (this is what the test needs): %d %d", first, taken)
			}
			if ledgerHas(o.S, "form-1") {
				t.Fatal("the answer that names the id the new vehicle took is dropped")
			}
		})
	}
}

// failTheCheckRead makes the read of the repeat check fail for the statements that contain the text given ("" for every one), and puts the real read back when the test ends.
func failTheCheckRead(t *testing.T, containing string) (restore func()) {
	t.Helper()
	real := spawnCheckQuery
	spawnCheckQuery = func(s *save.Save, q string, args ...any) ([]save.Row, error) {
		if containing == "" || strings.Contains(q, containing) {
			return nil, errors.New("the working copy could not be read")
		}
		return real(s, q, args...)
	}
	restore = func() { spawnCheckQuery = real }
	t.Cleanup(restore)
	return restore
}

// The two reads of the check: the vehicle's row, then its entity links (the second is reached only when the first finds the vehicle).
var checkReads = map[string]string{"the vehicle's row": "from actors", "the entity links": "actor_fgl_entities"}

// A repeat of a Create whose check could not read the save is refused: nothing is written, the answer stays, and once the save can be read again the same retry is answered with the
// first vehicle (#790). Before, the failed read counted as "the vehicle is gone" and the retry wrote a second vehicle.
func TestARepeatWhoseCheckCannotReadTheSaveIsRefusedAndWritesNothing(t *testing.T) {
	for name, containing := range checkReads {
		t.Run(name, func(t *testing.T) {
			o := spawnOps(t)
			a := Args{"class": "Buggy", "template": "T0", "requestId": "form-u"}
			first, _ := spawn(t, o, a)
			count := func() int64 { return mustOne(t, o, `select count(*) c from vehicles`)["c"].(int64) }
			actors := func() int64 { return mustOne(t, o, `select count(*) c from actors`)["c"].(int64) }
			vehiclesBefore, actorsBefore, opsBefore := count(), actors(), len(o.S.Pending())
			restore := failTheCheckRead(t, containing)
			res, err := o.SpawnVehicle(a)
			if !errors.Is(err, errSpawnCheckUnread) || res != nil {
				t.Fatalf("the repeat is refused with the read answer: %v, %v", res, err)
			}
			if msg := err.Error(); !strings.Contains(msg, "nothing was written") || !strings.Contains(msg, "could not read the save") {
				t.Fatalf("the answer says what happened: %q", msg)
			}
			if count() != vehiclesBefore || actors() != actorsBefore || len(o.S.Pending()) != opsBefore {
				t.Fatalf("nothing was written: %d vehicles, %d actors, %d edits (before %d, %d, %d)", count(), actors(), len(o.S.Pending()), vehiclesBefore, actorsBefore, opsBefore)
			}
			if !ledgerHas(o.S, "form-u") {
				t.Fatal("the answer is kept")
			}
			if seen, order := ledgerEntries(o.S); seen != 1 || order != 1 {
				t.Fatalf("the ledger stays consistent: %d answers, %d in the order", seen, order)
			}
			restore() // the save can be read again
			again, ans := spawn(t, o, a)
			if again != first || ans["repeat"] != true || count() != vehiclesBefore {
				t.Fatalf("the retry is now answered with the first vehicle: %d (first %d), repeat %v, %d vehicles", again, first, ans["repeat"], count())
			}
		})
	}
}

// A prune whose check could not read the save keeps every answer: it does not know that the vehicles are gone (#790; the prune always did, this pins it).
func TestPruneSpawnAnswersKeepsTheAnswersWhenTheCheckCannotReadTheSave(t *testing.T) {
	for name, containing := range map[string]string{"every read": "", "the vehicle's row": "from actors", "the entity links": "actor_fgl_entities"} {
		t.Run(name, func(t *testing.T) {
			o := spawnOps(t)
			spawn(t, o, Args{"class": "Buggy", "template": "T0", "requestId": "form-p"})
			spawn(t, o, Args{"class": "Buggy", "template": "T0", "requestId": "form-q"})
			if containing != "actor_fgl_entities" { // the vehicles are gone from the working copy; for the read of the links they stay, so that the read is reached
				if err := o.S.Discard(); err != nil {
					t.Fatal(err)
				}
			}
			failTheCheckRead(t, containing)
			PruneSpawnAnswers(o.S)
			if !ledgerHas(o.S, "form-p") || !ledgerHas(o.S, "form-q") {
				t.Fatal("the answers are kept when the working copy could not be read")
			}
			if seen, order := ledgerEntries(o.S); seen != 2 || order != 2 {
				t.Fatalf("the ledger stays consistent: %d answers, %d in the order", seen, order)
			}
		})
	}
}

// A read that worked and did not find the vehicle makes the repeat a new request: it writes, and the entry is replaced (the prune drops it). A read that found the vehicle answers
// the repeat and writes nothing, and the prune keeps the entry (#790).
func TestARepeatIsAnsweredWhileItsVehicleIsThereAndWrittenOnlyWhenTheReadFindsItGone(t *testing.T) {
	o := spawnOps(t)
	a := Args{"class": "Buggy", "template": "T0", "requestId": "form-v"}
	first, _ := spawn(t, o, a)
	count := func() int64 { return mustOne(t, o, `select count(*) c from vehicles`)["c"].(int64) }
	n := count()
	if same, ans := spawn(t, o, a); same != first || ans["repeat"] != true || count() != n {
		t.Fatalf("still there: answered with the first vehicle, nothing written: %d, %v, %d vehicles", same, ans["repeat"], count())
	}
	PruneSpawnAnswers(o.S)
	if !ledgerHas(o.S, "form-v") {
		t.Fatal("the prune keeps the answer of a vehicle that is there")
	}
	if _, err := o.S.MutateCascade("remove the vehicle", func(m *save.Mut) error {
		_, err := m.Exec(`delete from vehicles where id=?`, first)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if count() != n-1 {
		t.Fatal("the vehicle is gone (this is what the test needs)")
	}
	again, ans := spawn(t, o, a)
	if ans["repeat"] == true || count() != n || !ledgerHas(o.S, "form-v") {
		t.Fatalf("gone, and the read worked: the retry is a new request and writes: %d, %v, %d vehicles", again, ans["repeat"], count())
	}
	if seen, order := ledgerEntries(o.S); seen != 1 || order != 1 {
		t.Fatalf("the entry was replaced, not doubled: %d answers, %d in the order", seen, order)
	}
}
