package ops

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The character's place (#654, audit finding G-001). A real save can hold the character's body (and the vehicle it sits in) on the travel map with a state of 1, while the controller
// and the player state are on the map the player is on. Bring vehicle, Create vehicle, Duplicate vehicle and a Teleport read the place from the body alone and wrote a vehicle
// onto the travel map, where the player cannot walk to it. The fixtures here have that shape: actors 1 (controller) and 3 (player state) on the Deep Desert, actor 2 (the body)
// on "Overland" with state 1.

const parkedBodySQL = `
update actors set map='DeepDesert', location_x=-300000.0, location_y=-590000.0, location_z=3500.0 where id in (1,3);
update actors set map='Overland', location_x=50.0, location_y=60.0, location_z=70.0, state=1 where id=2;`

// the same character, the three actors on the Deep Desert and the body standing (state 0): the ordinary case, on a map other than Hagga Basin
const standingOnDeepDesertSQL = `
update actors set map='DeepDesert', location_x=-300000.0, location_y=-590000.0, location_z=3500.0 where id in (1,2,3);`

// the body is on the player's own map but not standing in the world (state 1)
const bodyNotStandingSQL = `update actors set state=1 where id=2;`

func characterActors(t *testing.T, o *Ops) string {
	t.Helper()
	rows, err := o.S.Query(`select id, map, dimension_index d, location_x x, location_y y, location_z z, state st from actors order by id`)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "%v:%v/%v@%v,%v,%v/%v ", r["id"], r["map"], r["d"], r["x"], r["y"], r["z"], r["st"])
	}
	return b.String()
}

func mustRefuseAsParked(t *testing.T, what string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: a body that is not where the player is must refuse \"next to the character\"", what)
	}
	if !errors.Is(err, apierr.ErrBadArgument) || !strings.Contains(err.Error(), "Nothing was changed") {
		t.Errorf("%s: want a plain sentence of kind bad argument that says nothing was changed, got %v", what, err)
	}
}

func TestBringVehicleRefusesWhenTheBodyIsParkedOnAnotherMapAndChangesNothing(t *testing.T) {
	for name, sql := range map[string]string{"body on the travel map": parkedBodySQL, "body on the player's map but not standing": bodyNotStandingSQL} {
		o := &Ops{S: testsave.PlayerWithSQL(t, vehicleWorld+sql)}
		before := characterActors(t, o)
		_, err := o.BringVehicle(Args{"id": 201.0})
		mustRefuseAsParked(t, name, err)
		if after := characterActors(t, o); after != before {
			t.Errorf("%s: a refusal changes nothing:\n before %s\n after  %s", name, before, after)
		}
		if o.S.Dirty() {
			t.Errorf("%s: a refusal leaves no pending edit", name)
		}
	}
}

func TestDuplicateVehicleRefusesWhenTheBodyIsParkedAndChangesNothing(t *testing.T) {
	for name, sql := range map[string]string{"body on the travel map": parkedBodySQL, "body on the player's map but not standing": bodyNotStandingSQL} {
		o := &Ops{S: testsave.PlayerWithSQL(t, duplicateWorld+sql)}
		before := characterActors(t, o)
		_, err := o.DuplicateVehicle(Args{"id": 201.0})
		mustRefuseAsParked(t, name, err)
		if after := characterActors(t, o); after != before {
			t.Errorf("%s: a refusal changes nothing:\n before %s\n after  %s", name, before, after)
		}
	}
}

func TestCreateVehicleRefusesWhenTheBodyIsParkedUnlessThePlaceIsGiven(t *testing.T) {
	for name, sql := range map[string]string{"body on the travel map": parkedBodySQL, "body on the player's map but not standing": bodyNotStandingSQL} {
		o := &Ops{S: testsave.PlayerWithSQL(t, spawnWorldSQL()+sql)}
		before := characterActors(t, o)
		a := Args{"class": "Sandbike", "template": "T0"}
		for what, call := range map[string]func(Args) (any, error){"create": o.SpawnVehicle, "preview": o.SpawnVehiclePreview} {
			_, err := call(a)
			mustRefuseAsParked(t, name+", "+what, err)
		}
		if after := characterActors(t, o); after != before {
			t.Errorf("%s: a refusal changes nothing:\n before %s\n after  %s", name, before, after)
		}
	}
	// a place the person gave is theirs: it is written, with the player's own dimension and not the parked body's map
	o := &Ops{S: testsave.PlayerWithSQL(t, spawnWorldSQL()+parkedBodySQL+`update actors set dimension_index=7 where id in (1,3);`)}
	id, _ := spawn(t, o, Args{"class": "Sandbike", "template": "T0", "place": map[string]any{"map": "DeepDesert", "x": -1000.5, "y": 2000.0, "z": 3000.0}})
	if a := mustOne(t, o, `select map, location_x x, dimension_index d from actors where id=?`, id); a["map"] != "DeepDesert" || a["x"] != -1000.5 || a["d"] != int64(7) {
		t.Errorf("a vehicle placed by hand: %v", a)
	}
}

// The character stands in the Deep Desert in a save that holds all three actors there: the vehicle goes next to the character on that map, in the player's dimension, and the
// rule is the same for the three actions.
func TestNextToTheCharacterUsesThePlayersMapAndDimension(t *testing.T) {
	const dim = `update actors set dimension_index=3 where id in (1,2,3);`
	o := &Ops{S: testsave.PlayerWithSQL(t, vehicleWorld+standingOnDeepDesertSQL+dim)}
	if _, err := o.BringVehicle(Args{"id": 201.0}); err != nil {
		t.Fatal(err)
	}
	if a := mustOne(t, o, `select map, dimension_index d, location_x x from actors where id=201`); a["map"] != "DeepDesert" || a["d"] != int64(3) || a["x"].(float64) < -300000 || a["x"].(float64) > -290000 {
		t.Errorf("Bring vehicle: %v", a)
	}
	o = &Ops{S: testsave.PlayerWithSQL(t, duplicateWorld+standingOnDeepDesertSQL+dim)}
	r, err := o.DuplicateVehicle(Args{"id": 201.0})
	if err != nil {
		t.Fatal(err)
	}
	if a := mustOne(t, o, `select map, dimension_index d from actors where id=?`, newVehicleID(t, r)); a["map"] != "DeepDesert" || a["d"] != int64(3) {
		t.Errorf("Duplicate vehicle: %v", a)
	}
	o = &Ops{S: testsave.PlayerWithSQL(t, spawnWorldSQL()+standingOnDeepDesertSQL+dim)}
	id, _ := spawn(t, o, Args{"class": "Sandbike", "template": "T0"})
	if a := mustOne(t, o, `select map, dimension_index d from actors where id=?`, id); a["map"] != "DeepDesert" || a["d"] != int64(3) {
		t.Errorf("Create vehicle: %v", a)
	}
}

// The body is the position when the three agree: the editor's move on one map writes the body only, so a vehicle brought after it goes next to where the body now is.
func TestBringVehicleGoesNextToTheBodyAfterAMoveOnOneMap(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, vehicleWorld)}
	if _, err := o.Teleport(Args{"x": 5000.0, "y": 6000.0, "z": 300.0}); err != nil {
		t.Fatal(err)
	}
	if _, err := o.BringVehicle(Args{"id": 201.0}); err != nil {
		t.Fatal(err)
	}
	if a := mustOne(t, o, `select location_x x from actors where id=201`); a["x"].(float64) < 5000 || a["x"].(float64) > 6000 {
		t.Errorf("the vehicle is next to the body: %v", a)
	}
}

func TestCharacterPlaceReadsControllerBodyAndPlayerState(t *testing.T) {
	cases := []struct {
		name, sql string
		parked    bool
		parkedMap bool
		mapWant   string
	}{
		{"all three on one map, standing", ``, false, false, "HaggaBasin"},
		{"body on another map", `update actors set map='Overland' where id=2;`, true, true, "HaggaBasin"},
		{"player state on another map", `update actors set map='Overland' where id=3;`, true, true, "HaggaBasin"},
		{"controller on another map than the body", `update actors set map='DeepDesert' where id=1;`, true, true, "DeepDesert"},
		{"body not standing", `update actors set state=4 where id=2;`, true, false, "HaggaBasin"},
		{"body without a map", `update actors set map=NULL where id=2;`, true, true, "HaggaBasin"},
		{"controller without a map", `update actors set map=NULL where id=1;`, true, false, ""},
	}
	for _, c := range cases {
		o := &Ops{S: testsave.PlayerWithSQL(t, c.sql)}
		p, err := o.player()
		if err != nil {
			t.Fatal(err)
		}
		cp, err := readCharacterPlace(o.S.One, p, true)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if (cp.Parked != nil) != c.parked || cp.ParkedMap != c.parkedMap || cp.Map != c.mapWant {
			t.Errorf("%s: parked=%v parkedMap=%v map=%q, want %v %v %q (%v)", c.name, cp.Parked != nil, cp.ParkedMap, cp.Map, c.parked, c.parkedMap, c.mapWant, cp.Parked)
		}
		if c.parked && !errors.Is(cp.Parked, apierr.ErrBadArgument) {
			t.Errorf("%s: the refusal is of kind bad argument: %v", c.name, cp.Parked)
		}
	}
}

func TestTeleportOfAParkedBodyMovesAllThreeActorsOrRefuses(t *testing.T) {
	// a move on one map would move the body only and leave it where the player is not: no map, no move
	o := &Ops{S: testsave.PlayerWithSQL(t, parkedBodySQL)}
	before := characterActors(t, o)
	if _, err := o.Teleport(Args{"x": 10.0, "y": 20.0, "z": 30.0}); err == nil || !strings.Contains(err.Error(), "Choose the map") {
		t.Fatalf("a move of a parked body on no named map must be refused with the way out: %v", err)
	}
	if after := characterActors(t, o); after != before {
		t.Errorf("a refusal changes nothing:\n before %s\n after  %s", before, after)
	}
	// the map the player is on, named: the move is a move between maps and puts the three actors on it together (the state of the body is not touched)
	if _, err := o.Teleport(Args{"x": -300500.0, "y": -590500.0, "z": 1500.0, "map": "DeepDesert"}); err != nil {
		t.Fatal(err)
	}
	if got, want := playerActorMaps(t, o), "1:DeepDesert/0@-300500,-590500,1500 2:DeepDesert/0@-300500,-590500,1500 3:DeepDesert/0@-300500,-590500,1500"; got != want {
		t.Errorf("the three actors after the move:\n got %s\nwant %s", got, want)
	}
	// a body that is not standing in the world is not moved by the editor at all
	o = &Ops{S: testsave.PlayerWithSQL(t, bodyNotStandingSQL)}
	before = characterActors(t, o)
	if _, err := o.Teleport(Args{"x": 10.0, "y": 20.0, "z": 30.0}); err == nil {
		t.Fatal("a body that is not standing must not be moved")
	}
	if _, err := o.Teleport(Args{"x": -300500.0, "y": -590500.0, "z": 1500.0, "map": "DeepDesert"}); err == nil {
		t.Fatal("a body that is not standing must not be moved to another map either")
	}
	if after := characterActors(t, o); after != before {
		t.Errorf("a refusal changes nothing:\n before %s\n after  %s", before, after)
	}
}

func TestDestinationsOfAParkedBodyAreFromThePlayerNotTheBody(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, parkedBodySQL)}
	r := destinationsOn(t, o, "")
	if r["map"] != "DeepDesert" || r["sameMap"] != true {
		t.Errorf("the player is on the Deep Desert, not where the parked body is: %v", r["map"])
	}
	if s, _ := r["parked"].(string); s == "" {
		t.Errorf("the answer says that the body is parked: %v", r)
	}
	if _, has := destinationsOn(t, &Ops{S: testsave.Player(t)}, "")["parked"]; has {
		t.Error("an ordinary character is not reported as parked")
	}
}

// A same-map move that is queued (a game is running) is put back onto the game's newer file only when the character is still on the map it was worked out for (G-008).
func TestAQueuedSameMapTeleportIsHeldWhenTheCharacterChangedMap(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := &Ops{S: testsave.PlayerWithSQL(t, ``)}
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	if _, err := o.Teleport(Args{"x": 5.0, "y": 6.0, "z": 7.0}); err != nil {
		t.Fatal(err)
	}
	gameChanges(t, o.S.Path, `update actors set map='DeepDesert' where id in (1,2,3)`)
	mustBeHeld(t, o.S)
}

// ... and it is not held for a change that does not matter to it: the character walking about on the same map.
func TestAQueuedSameMapTeleportIsNotHeldWhenTheCharacterOnlyMoved(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := &Ops{S: testsave.PlayerWithSQL(t, ``)}
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	if _, err := o.Teleport(Args{"x": 5.0, "y": 6.0, "z": 7.0}); err != nil {
		t.Fatal(err)
	}
	gameChanges(t, o.S.Path, `update actors set location_x=location_x+5000 where id=2`)
	if f := o.S.Flush(); !f.Saved || o.S.Held() != nil {
		t.Fatalf("the move on one map does not depend on where the body was: %+v", f)
	}
}

// A queued Bring vehicle is held when the character's map changed (it was worked out from the player's place).
func TestAQueuedBringVehicleIsHeldWhenTheCharacterChangedMap(t *testing.T) {
	defer save.NoGameRunningForTests()()
	o := &Ops{S: testsave.PlayerWithSQL(t, vehicleWorld)}
	o.S.EnableJournal(filepath.Join(t.TempDir(), "pending.json"))
	if _, err := o.BringVehicle(Args{"id": 201.0}); err != nil {
		t.Fatal(err)
	}
	gameChanges(t, o.S.Path, `update actors set map='DeepDesert' where id in (1,3)`)
	mustBeHeld(t, o.S)
}
