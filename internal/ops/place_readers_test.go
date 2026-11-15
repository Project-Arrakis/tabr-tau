package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The player page, the summary and the saved map take the character's place from readCharacterPlace, as Bring, Create, Duplicate, Teleport and the place list do (#699): the map is the
// controller's, the position is the body's only when the three actors agree. The fixture is the one of #654: the controller and the player state on the Deep Desert, the body on the
// travel map with a state of 1.

func TestPlayerAnswerTakesTheCharactersPlaceFromTheController(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, parkedBodySQL)}
	r, err := o.Player()
	if err != nil {
		t.Fatal(err)
	}
	a, _ := r.(map[string]any)["actor"].(map[string]any)
	if a["map"] != "DeepDesert" || a["x"] != -300000.0 || a["y"] != -590000.0 || a["z"] != 3500.0 {
		t.Errorf("the actor is the player's place (the controller's), not the body's: %v", a)
	}
	if s, _ := a["parked"].(string); !strings.Contains(s, "Nothing was changed") || a["bodyMap"] != "Overland" {
		t.Errorf("the answer says that the three do not agree, and where the body is: %v", a)
	}
}

func TestPlayerAnswerOfAnOrdinaryCharacterIsTheBodyAndSaysNothingParked(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `update actors set location_x=11.0, location_y=22.0, location_z=33.0 where id=2;`)}
	r, err := o.Player()
	if err != nil {
		t.Fatal(err)
	}
	a, _ := r.(map[string]any)["actor"].(map[string]any)
	if a["map"] != "HaggaBasin" || a["x"] != 11.0 || a["y"] != 22.0 || a["z"] != 33.0 {
		t.Errorf("the actor is the body's place: %v", a)
	}
	if _, has := a["parked"]; has {
		t.Errorf("an ordinary character is not reported as parked: %v", a)
	}
}

func TestSummaryMapIsThePlayersMapWhenTheBodyIsElsewhere(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, parkedBodySQL)}
	r, err := o.Summary()
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	if m["map"] != "DeepDesert" {
		t.Errorf("the summary's map is the player's, not the body's: %v", m["map"])
	}
	if s, _ := m["parked"].(string); s == "" {
		t.Errorf("the summary says that the body is not where the player is: %v", m["parked"])
	}
	o = &Ops{S: testsave.Player(t)}
	r, err = o.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if _, has := r.(map[string]any)["parked"]; has || r.(map[string]any)["map"] != "HaggaBasin" {
		t.Errorf("an ordinary character: map %v, parked %v", r.(map[string]any)["map"], r.(map[string]any)["parked"])
	}
}

func TestSavedMapDrawsTheCharacterOnThePlayersMapNotTheBodys(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, parkedBodySQL)}
	live := func(key string) map[string]any {
		t.Helper()
		r, err := o.LiveMap(key)
		if err != nil {
			t.Fatal(err)
		}
		return r.(map[string]any)
	}
	dd := live("DeepDesert")
	ch, _ := dd["character"].(map[string]any)
	if ch == nil || ch["x"] != -300000.0 || ch["y"] != -590000.0 {
		t.Errorf("the character is on the player's map, at the player's position: %v", dd["character"])
	}
	if s, _ := dd["parked"].(string); s == "" {
		t.Errorf("the answer says that the body is not where the player is: %v", dd["parked"])
	}
	if hb := live("HaggaBasin"); hb["character"] != nil {
		t.Errorf("the body's map is not the player's: no character on it: %v", hb["character"])
	}
}

func TestSavedMapOfAnOrdinaryCharacterIsTheBodyOnItsMap(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `update actors set location_x=11.0, location_y=22.0 where id=2;`)}
	r, err := o.LiveMap("HaggaBasin")
	if err != nil {
		t.Fatal(err)
	}
	m := r.(map[string]any)
	ch, _ := m["character"].(map[string]any)
	if ch == nil || ch["x"] != 11.0 || ch["y"] != 22.0 {
		t.Errorf("the character: %v", m["character"])
	}
	if _, has := m["parked"]; has {
		t.Errorf("an ordinary character is not reported as parked: %v", m["parked"])
	}
}
