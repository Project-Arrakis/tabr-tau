package ops

import (
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The character (controller 1, pawn 2, account 1) with the same kinds of values a real save holds: FLevelComponent and
// FHealthComponent on the DuneCharacter entity, Intel in actors.properties, hydration in gas_attributes, a faction row owned
// by the controller and a virtual currency balance. The fixture already gives the pawn 100 Solari Coin.
const summaryWorld = `
insert into fgl_entities(entity_id,components) values (-10, jsonb('{"FLevelComponent":[[],{"TotalXPEarned":41221,"TotalSkillPoints":70,"UnspentSkillPoints":9}],"FHealthComponent":[[],{"m_CurrentHealth":112.5}]}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (2,-10,'DuneCharacter');
update actors set properties=jsonb('{"TechKnowledgePlayerComponent":{"m_TechKnowledgePoints":333}}'),
  gas_attributes=jsonb('{"DuneHydrationAttributeSet":{"CurrentHydration":{"CurrentValue":52.5}},"DuneSpiceAddictionAttributeSet":{"SpiceAddictionLevel":{"CurrentValue":0.0}}}') where id=2;
insert into factions(id,name) values (2,'Harkonnen');
insert into player_faction(actor_id,faction_id,utc_time_faction_change) values (1,2,1);
insert into player_virtual_currency_balances(player_controller_id,currency_id,balance) values (1,0,591227);`

func TestSummaryReadsTheCharacterFromTheSave(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, summaryWorld)}
	r, err := o.Summary()
	if err != nil {
		t.Fatal(err)
	}
	s := r.(map[string]any)
	if s["name"] != "Tester" || s["map"] != "HaggaBasin" || s["faction"] != "Harkonnen" || s["status"] != "Offline" {
		t.Fatalf("identity: %v", s)
	}
	// 41,221 XP is level 71, and a level-71 character has 70 skill points in total.
	if s["xp"].(int64) != 41221 || s["level"].(int) != 71 {
		t.Fatalf("xp and level: %v %v", s["xp"], s["level"])
	}
	if sp := s["skillPoints"].(map[string]any); sp["unspent"].(int64) != 9 || sp["total"].(int64) != 70 {
		t.Fatalf("skill points: %v", sp)
	}
	if in := s["intel"].(map[string]any); in["points"].(int64) != 333 || in["max"].(int) != 2779 {
		t.Fatalf("intel: %v", in)
	}
	v := s["vitals"].(map[string]any)
	if v["health"].(float64) != 112.5 || v["hydration"].(float64) != 52.5 || v["spiceAddiction"].(float64) != 0 || v["healthMinMax"].(int) != 150 {
		t.Fatalf("vitals: %v", v)
	}
	ids := s["ids"].(map[string]any)
	if ids["actor"].(int64) != 2 || ids["account"].(int64) != 1 || ids["controller"].(int64) != 1 {
		t.Fatalf("ids: %v", ids)
	}
	id := s["identity"].(map[string]any)
	if id["platform"] != "Tester" || id["platformId"] != "PLATFORM-TEST" || id["funcomId"] != "FUNCOM-TEST" || id["flsId"] != "u" {
		t.Fatalf("identity block: %v", id)
	}
	// Solari Credit (virtual currency), Solari Coin (item stacks), then the secondary wallet, always shown.
	tiles := s["currency"].([]map[string]any)
	want := []struct {
		label string
		n     int64
	}{{"Solari Credit", 591227}, {"Solari Coin", 100}, {"House Credit", 0}}
	if len(tiles) != len(want) {
		t.Fatalf("currency tiles: %v", tiles)
	}
	for i, w := range want {
		if tiles[i]["label"] != w.label || tiles[i]["balance"].(int64) != w.n {
			t.Errorf("tile %d = %v, want %s %d", i, tiles[i], w.label, w.n)
		}
	}
}

// A save missing the components still answers: absent values are left out, never invented.
func TestSummaryWithNothingButTheCharacter(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	r, err := o.Summary()
	if err != nil {
		t.Fatal(err)
	}
	s := r.(map[string]any)
	if s["name"] != "Tester" {
		t.Fatalf("%v", s)
	}
	for _, k := range []string{"level", "xp", "skillPoints", "intel", "faction"} {
		if _, has := s[k]; has {
			t.Errorf("%s must be absent when the save has no data for it: %v", k, s[k])
		}
	}
}

func TestXPToLevel(t *testing.T) {
	cases := map[int64]int{-5: 0, 0: 0, 39: 0, 40: 1, 214: 1, 215: 2, 344440: 200, 9999999: 200}
	for xp, want := range cases {
		if got := xpToLevel(xp); got != want {
			t.Errorf("xpToLevel(%d) = %d, want %d", xp, got, want)
		}
	}
}
