package ops

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Clearing the Deep Desert markers (#1216), on made-up saves of both layouts (markers keyed by map name, and by map id through map_names). Written for the older layout; testsave
// converts the world for the newer one.

// ddMarkerWorld has, besides the player: markers on three maps and one map whose name only starts like the Deep Desert's; what the character discovered on each; and resource fields.
// The marker hash 1 is on the Hagga Basin and on the Deep Desert (the same hash on two maps is two markers).
const ddMarkerWorld = `
insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) values
  (1,0,'HaggaBasin','AzuritePickup',-1000,2000,0),
  (2,0,'HaggaBasin','EnemyCamp',3000,4000,0),
  (1,0,'DeepDesert','Ecolab',5,5,0),
  (3,0,'DeepDesert','Cave',6,6,0),
  (4,1,'DeepDesert','SpiceField',7,7,0),
  (9,0,'DeepDesertTravel','Gate',8,8,0),
  (10,0,'Arrakeen','Vendor',9,9,0);
insert into player_markers(player_id,marker_hash_id,dimension_index,map_name,discovery_level,discovery_method) values
  (1,1,0,'HaggaBasin',3,1),
  (1,1,0,'DeepDesert',2,1),
  (1,3,0,'DeepDesert',1,1),
  (1,9,0,'DeepDesertTravel',1,1),
  (1,10,0,'Arrakeen',1,1);
insert into resourcefield_state(field_id,map,dimension_index,spawn_time,value_remaining) values
  (10,'HaggaBasin',0,100.5,40),
  (11,'DeepDesert',0,100.5,50),
  (12,'DeepDesert',0,200.5,60),
  (11,'DeepDesert',1,300.5,70),
  (13,'DeepDesertTravel',0,100.5,80),
  (11,'Arrakeen',0,100.5,90);`

// othersDump is every row of the three tables that is not of the Deep Desert, whole, in a fixed order, with the map as text in either layout.
func othersDump(t *testing.T, s *save.Save) []save.Row {
	t.Helper()
	var out []save.Row
	for _, q := range []string{
		`select m.* from ` + s.MarkersTable() + ` m where m.map_name is not 'DeepDesert' order by m.map_name, m.marker_hash_id, m.dimension_index`,
		`select p.* from ` + s.PlayerMarkersTable() + ` p where p.map_name is not 'DeepDesert' order by p.map_name, p.marker_hash_id, p.dimension_index, p.player_id`,
		`select * from resourcefield_state where map is not 'DeepDesert' order by map, field_id, dimension_index`,
	} {
		rows, err := s.Query(q)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, rows...)
	}
	return out
}

// tableCount is the number of rows of a table (any map).
func tableCount(t *testing.T, s *save.Save, table string) int64 {
	t.Helper()
	r, err := s.One(`select count(*) c from ` + table)
	if err != nil || r == nil {
		t.Fatal(table, err)
	}
	return r["c"].(int64)
}

func TestTheSurveyCountsTheDeepDesertAndTheOthersAndChangesNothing(t *testing.T) {
	inEachLayout(t, ddMarkerWorld, func(t *testing.T, o *Ops) {
		sv, err := o.SurveyDeepDesertMarkers()
		if err != nil {
			t.Fatal(err)
		}
		want := MarkerSurvey{DeepDesert: MarkerCounts{Markers: 3, Discovered: 2, ResourceFields: 3}, Others: MarkerCounts{Markers: 4, Discovered: 3, ResourceFields: 3}}
		if sv != want {
			t.Fatalf("survey %+v, want %+v", sv, want)
		}
		if o.S.Dirty() {
			t.Fatal("counting changed the save")
		}
	})
}

// The whole point: the Deep Desert's rows of the three tables go, every other row of them stays exactly as it was (compared whole), and nothing else is touched.
func TestOnlyTheDeepDesertRowsAreRemovedInEitherLayout(t *testing.T) {
	inEachLayout(t, ddMarkerWorld, func(t *testing.T, o *Ops) {
		othersBefore := othersDump(t, o.S)
		if len(othersBefore) != 10 {
			t.Fatalf("the test world has %d rows of other maps, want 10", len(othersBefore))
		}
		actors := tableCount(t, o.S, "actors")
		items := tableCount(t, o.S, "items")
		before, err := o.ClearDeepDesertMarkers()
		if err != nil {
			t.Fatal(err)
		}
		if before.DeepDesert != (MarkerCounts{Markers: 3, Discovered: 2, ResourceFields: 3}) {
			t.Fatalf("what was removed: %+v", before.DeepDesert)
		}
		sv, err := o.SurveyDeepDesertMarkers()
		if err != nil {
			t.Fatal(err)
		}
		if sv.DeepDesert.Total() != 0 || sv.Others != before.Others {
			t.Fatalf("after: %+v (before %+v)", sv, before)
		}
		if after := othersDump(t, o.S); !reflect.DeepEqual(after, othersBefore) {
			t.Fatalf("rows of the other maps changed:\nbefore %v\nafter  %v", othersBefore, after)
		}
		if tableCount(t, o.S, "actors") != actors || tableCount(t, o.S, "items") != items {
			t.Fatal("actors or items changed")
		}
		if !o.S.Dirty() {
			t.Fatal("the edit is not queued")
		}
		if ops := o.S.Ops(); len(ops) != 1 || !strings.Contains(ops[0].Desc, "3 markers, 2 discovered, 3 resource fields") {
			t.Fatalf("the edit is not described: %v", ops)
		}
		if err := o.VerifyDeepDesertMarkersCleared(before); err != nil {
			t.Fatal(err)
		}
	})
}

// A save whose Deep Desert has nothing: the edit changes no row, and the save stays clean (nothing is queued to be written).
func TestASaveWithNoDeepDesertMarkersIsLeftClean(t *testing.T) {
	inEachLayout(t, strings.ReplaceAll(strings.ReplaceAll(ddMarkerWorld, "'DeepDesert',", "'HaggaBasin2',"), "'DeepDesert')", "'HaggaBasin2')"), func(t *testing.T, o *Ops) {
		othersBefore := othersDump(t, o.S)
		before, err := o.ClearDeepDesertMarkers()
		if err != nil {
			t.Fatal(err)
		}
		if before.DeepDesert.Total() != 0 || before.Others.Total() == 0 {
			t.Fatalf("survey %+v", before)
		}
		if o.S.Dirty() || len(o.S.Ops()) != 0 {
			t.Fatal("an edit was queued although nothing was to remove")
		}
		if !reflect.DeepEqual(othersDump(t, o.S), othersBefore) {
			t.Fatal("rows changed")
		}
	})
}

// A save with no marker at all (the newer layout then has an empty map_names).
func TestASaveWithNoMarkersAtAll(t *testing.T) {
	inEachLayout(t, "", func(t *testing.T, o *Ops) {
		before, err := o.ClearDeepDesertMarkers()
		if err != nil || before != (MarkerSurvey{}) || o.S.Dirty() {
			t.Fatalf("%+v %v dirty=%v", before, err, o.S.Dirty())
		}
	})
}

// The read-back rule, on its own: a Deep Desert row left, or a count of the others that differs either way, is a failure.
func TestTheReadBackRule(t *testing.T) {
	before := MarkerSurvey{DeepDesert: MarkerCounts{3, 2, 3}, Others: MarkerCounts{4, 3, 3}}
	for _, c := range []struct {
		name  string
		after MarkerSurvey
		bad   string
	}{
		{"cleared", MarkerSurvey{Others: MarkerCounts{4, 3, 3}}, ""},
		{"a marker is left", MarkerSurvey{DeepDesert: MarkerCounts{Markers: 1}, Others: MarkerCounts{4, 3, 3}}, "still has"},
		{"a discovered row is left", MarkerSurvey{DeepDesert: MarkerCounts{Discovered: 1}, Others: MarkerCounts{4, 3, 3}}, "still has"},
		{"a resource field is left", MarkerSurvey{DeepDesert: MarkerCounts{ResourceFields: 1}, Others: MarkerCounts{4, 3, 3}}, "still has"},
		{"another map lost a marker", MarkerSurvey{Others: MarkerCounts{3, 3, 3}}, "other maps changed"},
		{"another map lost a discovered row", MarkerSurvey{Others: MarkerCounts{4, 2, 3}}, "other maps changed"},
		{"another map lost a resource field", MarkerSurvey{Others: MarkerCounts{4, 3, 2}}, "other maps changed"},
		{"another map gained a row", MarkerSurvey{Others: MarkerCounts{4, 3, 4}}, "other maps changed"},
	} {
		err := checkCleared(before, c.after)
		if (c.bad == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), c.bad)) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

// The verify on a save that was not cleared fails; on a save that lost a row of another map as well it fails too.
func TestTheVerifyOnTheSaveAsItIs(t *testing.T) {
	inEachLayout(t, ddMarkerWorld, func(t *testing.T, o *Ops) {
		before, err := o.SurveyDeepDesertMarkers()
		if err != nil {
			t.Fatal(err)
		}
		if err := o.VerifyDeepDesertMarkersCleared(before); err == nil || !strings.Contains(err.Error(), "still has") {
			t.Fatalf("an uncleared save verified: %v", err)
		}
		if _, err := o.ClearDeepDesertMarkers(); err != nil {
			t.Fatal(err)
		}
		if err := o.VerifyDeepDesertMarkersCleared(before); err != nil {
			t.Fatal(err)
		}
		testsave.Exec(t, o.S, `delete from resourcefield_state where map='Arrakeen'`)
		if err := o.VerifyDeepDesertMarkersCleared(before); err == nil || !strings.Contains(err.Error(), "other maps changed") {
			t.Fatalf("a lost row of another map verified: %v", err)
		}
	})
}

// A save of the newer layout whose map_names has no Deep Desert: nothing is selected for it, and the other maps keep every row.
func TestANewerSaveWhoseMapNamesLacksTheDeepDesert(t *testing.T) {
	inEachLayout(t, `insert into markers(marker_hash_id,dimension_index,map_name,marker_type,x,y,z) values (1,0,'HaggaBasin','Cave',1,1,0);
insert into resourcefield_state(field_id,map,dimension_index,spawn_time,value_remaining) values (1,'DeepDesert',0,1.5,5),(2,'HaggaBasin',0,1.5,5);`, func(t *testing.T, o *Ops) {
		before, err := o.ClearDeepDesertMarkers()
		if err != nil {
			t.Fatal(err)
		}
		if before.DeepDesert != (MarkerCounts{ResourceFields: 1}) || before.Others != (MarkerCounts{Markers: 1, ResourceFields: 1}) {
			t.Fatalf("%+v", before)
		}
		if tableCount(t, o.S, "markers") != 1 || tableCount(t, o.S, "resourcefield_state") != 1 {
			t.Fatal("the wrong rows were removed")
		}
	})
}
