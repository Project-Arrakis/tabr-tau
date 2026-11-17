package ops

import (
	"regexp"
	"testing"
)

// #618: Go to a place shows a place by its name, not by the numbers and keys the game keeps. The label stays the place's identity (the page keeps a chosen place by its group and label);
// Display is what is shown and Detail the ids for a tooltip.

var idInText = regexp.MustCompile(`#\d|\(key |\(marker |[0-9A-F]{16}`)

func byLabel(d []Destination) map[string]Destination {
	out := map[string]Destination{}
	for _, x := range d {
		out[x.Label] = x
	}
	return out
}

func TestPlacesAreShownByNameWithTheIdsInTheDetail(t *testing.T) {
	_, d := namedPlacesOf(t, placesOps(t, respawnWorldForPlaces), "", "")
	at := byLabel(d)
	for label, want := range map[string][2]string{
		"TradingPost (key " + locUnknownPost + ")": {"Unnamed trading post (1 of 2)", "Location key " + locUnknownPost},
		"TradingPost (marker 5)":                   {"Unnamed trading post (2 of 2)", "Marker 5"},
		"Griffin's Reach Tradepost":                {"Griffin's Reach Tradepost", "Location key " + locGriffin},
		"Spice Camp #602":                          {"Spice Camp", "Base number 602"},
		"Advanced Sub-Fief #601":                   {"Advanced Sub-Fief", "Base number 601"},
		"Sandbike #801":                            {"Sandbike", "Vehicle number 801"},
		"Red Runner #802":                          {"Red Runner", "Vehicle number 802"},
		"Respawn point #901":                       {"Respawn point", "Respawn point of actor 901"},
	} {
		got, ok := at[label]
		if !ok {
			t.Errorf("no place with the label %q: %v", label, labelsOf(d))
			continue
		}
		if got.Display != want[0] || got.Detail != want[1] {
			t.Errorf("%q: display %q, detail %q; want %q, %q", label, got.Display, got.Detail, want[0], want[1])
		}
	}
	for _, x := range d {
		if x.Display != "" && idInText.MatchString(x.Display) {
			t.Errorf("a number or key in what the page shows: %+v", x)
		}
	}
}

func TestTwoPlacesOfOneNameAreNumberedNotShownByTheirIds(t *testing.T) {
	_, d := namedPlacesOf(t, placesOps(t, `
insert into actors(id,class,map,location_x,location_y,location_z,state) values (806,'/Game/Vehicles/BP_Sandbike.BP_Sandbike_C','HaggaBasin',310,410,50,0);
insert into vehicles(id) values (806);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (806,'##SandbikeChoam',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (806,1,1);`), "", "")
	at := byLabel(d)
	a, b := at["Sandbike #801"], at["Sandbike #806"]
	if a.Display != "Sandbike (1 of 2)" || b.Display != "Sandbike (2 of 2)" {
		t.Errorf("two sandbikes: %q and %q (labels %v)", a.Display, b.Display, labelsOf(d))
	}
	if a.Detail != "Vehicle number 801" || b.Detail != "Vehicle number 806" {
		t.Errorf("ids: %q, %q", a.Detail, b.Detail)
	}
	if got := at["Red Runner #802"].Display; got != "Red Runner" {
		t.Errorf("a name that is alone is not numbered: %q", got)
	}
}

func TestPlaceFilterMatchesTheShownNameAndStillTheLabel(t *testing.T) {
	o := placesOps(t)
	for q, want := range map[string]int{"unnamed trading": 2, "602": 1, "spice camp": 1, "sandbike": 1} {
		if _, d := namedPlacesOf(t, o, q, ""); len(d) != want {
			t.Errorf("filter %q: %d places %v, want %d", q, len(d), labelsOf(d), want)
		}
	}
}
