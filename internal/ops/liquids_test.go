package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Fill all liquids (issue #349): the water refill, the blood-sack fill and the refuel as one edit. The world is the blood-sack world (7 sacks to fill, one Literjon at 500)
// with two more water containers to fill and the vehicles of the refuel tests (7 to fill).
const liquidsWorld = bloodWorld + `
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (401,1,1,140,'Literjon',0,1790000000,'{"FItemStackAndDurabilityStats":[[],{}]}',0),
  (402,2,1,141,'Decajon',0,1790000000,'{"FFillableItemStats":[[],{"CurrentAmount":5000.0,"FillableType":"Water"}]}',0);`

func liquidsOps(t *testing.T) *Ops {
	t.Helper()
	return &Ops{S: testsave.PlayerWithSQL(t, liquidsWorld+refuelWorldSQL())}
}

// refuelWorldSQL is the vehicle part of the refuel tests without the vehicleWorld rows that the blood-sack world already has.
func refuelWorldSQL() string { return refuelWorld(refuelVehicles) + vehicleWorldFuel }

func TestFillAllLiquidsDoesAllThreePartsInOneEditWithOneLogLine(t *testing.T) {
	o := liquidsOps(t)
	r, err := o.FillAllLiquids(Args{})
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	water, blood, veh := res["water"].(map[string]any), res["bloodSacks"].(map[string]any), res["vehicles"].(map[string]any)
	if water["filled"].(int) != 3 || blood["filled"].(int) != 7 || veh["filled"].(int) != 7 || res["vehiclesIncluded"] != true {
		t.Fatalf("answer: %v", res)
	}
	if got := sackStats(t, o, 301); got == "" || fuelOf(t, o, 301) != 750.0 {
		t.Fatalf("a sack and a vehicle are filled: %s %v", got, fuelOf(t, o, 301))
	}
	if p := o.S.Pending(); len(p) != 1 || p[0] != "filled all liquids: 3 water containers, 7 blood sacks, 7 vehicles" {
		t.Fatalf("one write-log entry: %q", p)
	}
	assertDBSound(t, o)
}

// The combined edit does what the three buttons do one after the other: the same rows come out. (This is what stops the combined action from growing its own copy
// of the fill rules.)
func TestFillAllLiquidsEqualsTheThreeSingleButtons(t *testing.T) {
	a, b := liquidsOps(t), liquidsOps(t)
	if _, err := a.FillAllLiquids(Args{}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []func() (any, error){b.RefillContainers, b.FillBloodSacks, b.RefuelVehicles} {
		if _, err := f(); err != nil {
			t.Fatal(err)
		}
	}
	if digest(t, a, nil, 0) != digest(t, b, nil, 0) {
		t.Fatal("the combined fill and the three single fills left different rows")
	}
	if len(b.S.Pending()) != 3 || len(a.S.Pending()) != 1 {
		t.Fatalf("one entry against three: %q / %q", a.S.Pending(), b.S.Pending())
	}
}

func TestFillAllLiquidsWithTheVehiclesBoxOffLeavesTheVehiclesAlone(t *testing.T) {
	o := liquidsOps(t)
	vehiclesBefore := entityText(t, o, 301) + entityText(t, o, 201)
	r, err := o.FillAllLiquids(Args{"vehicles": false})
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	if _, has := res["vehicles"]; has || res["vehiclesIncluded"] != false || res["water"].(map[string]any)["filled"].(int) != 3 || res["bloodSacks"].(map[string]any)["filled"].(int) != 7 {
		t.Fatalf("answer: %v", res)
	}
	if entityText(t, o, 301)+entityText(t, o, 201) != vehiclesBefore {
		t.Fatal("the vehicles were refuelled although the box was off")
	}
	if p := o.S.Pending(); len(p) != 1 || p[0] != "filled all liquids: 3 water containers, 7 blood sacks" {
		t.Fatalf("the line leaves the vehicles out: %q", p)
	}
}

// A failure in the third part puts the first two back, and nothing is queued or logged.
func TestAFailureInTheVehiclePartRollsBackTheWaterAndTheBloodSacks(t *testing.T) {
	o := liquidsOps(t)
	if _, err := o.S.Mutate("fixture", func(m *save.Mut) error {
		_, err := m.Exec(`create trigger forced_failure before update on fgl_entities begin select raise(abort,'forced failure'); end`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	was, pending := digest(t, o, nil, 0), len(o.S.Pending())
	if _, err := o.FillAllLiquids(Args{}); err == nil {
		t.Fatal("the forced failure must fail the action")
	}
	if digest(t, o, nil, 0) != was || len(o.S.Pending()) != pending {
		t.Fatal("the water and the blood sacks were not put back")
	}
	// with the vehicles box off the third part never runs, so the same save fills the first two
	if r, err := o.FillAllLiquids(Args{"vehicles": false}); err != nil || r.(map[string]any)["water"].(map[string]any)["filled"].(int) != 3 {
		t.Fatalf("%v %v", r, err)
	}
}

func TestFillAllLiquidsRefusesABoxThatIsNotTrueOrFalse(t *testing.T) {
	o := liquidsOps(t)
	was := digest(t, o, nil, 0)
	for _, v := range []any{"yes", 1.0, nil} {
		_, err := o.FillAllLiquids(Args{"vehicles": v})
		wantRefusal(t, err, apierr.ErrBadArgument, "vehicles box")
	}
	if digest(t, o, nil, 0) != was || o.S.Dirty() {
		t.Fatal("a refused request changes nothing")
	}
}

func TestFillAllLiquidsWithNothingToFillWritesNothing(t *testing.T) {
	o := liquidsOps(t)
	if _, err := o.FillAllLiquids(Args{}); err != nil {
		t.Fatal(err)
	}
	was, pending := digest(t, o, nil, 0), len(o.S.Pending())
	r, err := o.FillAllLiquids(Args{})
	if err != nil {
		t.Fatal(err)
	}
	res := r.(map[string]any)
	if res["water"].(map[string]any)["filled"].(int) != 0 || res["bloodSacks"].(map[string]any)["filled"].(int) != 0 || res["vehicles"].(map[string]any)["filled"].(int) != 0 {
		t.Fatalf("second run: %v", res)
	}
	if digest(t, o, nil, 0) != was || len(o.S.Pending()) != pending {
		t.Fatal("nothing to fill means nothing written and no log entry")
	}
}

func TestLiquidsStatusCountsWhatTheButtonWouldFill(t *testing.T) {
	o := liquidsOps(t)
	was := digest(t, o, nil, 0)
	r, err := o.LiquidsStatus()
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	water, blood, veh := m["water"].(map[string]any), m["bloodSacks"].(map[string]any), m["vehicles"].(map[string]any)
	if water["notFull"].(int) != 3 || blood["notFull"].(int) != 7 || veh["notFull"].(int) != 7 || veh["total"].(int) != 12 || blood["unknown"].(int) != 1 {
		t.Fatalf("count: %v", m)
	}
	if digest(t, o, nil, 0) != was || o.S.Dirty() {
		t.Fatal("counting changes nothing")
	}
}
