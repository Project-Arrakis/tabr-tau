package ops

import (
	"os"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// damageRecord makes the editor's record file unreadable (not JSON).
func damageRecord(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// What a tier removal takes back (#944, landsraad_takeback.go). World: rewardOps (landsraad_rewards_test.go). The character is player 1 in Harkonnen (2); Kenola (175) has the tiers
// 1,200 SolarisCoin 10,000 / 2,400 Schematic.K / 3,600 T6RangeFinder 50 / 4,800 Schematic.L / 6,000 its own item Swatch.Kenola, whose swatch is Kenola_Placeables_Swatch; the character
// is at 1,300 with no reward row. The simulated guild 50 has a SolarisCoin row on Kenola, Maros (174) holds the five rows the game wrote for 7,850.

// collectRewards is the game's collecting of every reward of the character: each row's amount goes to 0 and the row stays.
func collectRewards(t *testing.T, o *Ops) {
	t.Helper()
	setup(t, o, `update landsraad_house_rewards set amount=0 where player_id=1`)
}

// kenolaFromZero is a Kenola at 0 raised to to, so that the first tier's reward is written too.
func kenolaFromZero(t *testing.T, o *Ops, to float64) map[string]any {
	t.Helper()
	setup(t, o, `delete from landsraad_task_player_contributions where player_id=1 and task_id=175`)
	return applyWith(t, o, doubled(), house(175, Args{"was_mine": 0.0, "mine_to": to}))
}

func rowOf(t *testing.T, o *Ops, tmpl string) string {
	t.Helper()
	return rows(t, o, `select amount, last_updated from landsraad_house_rewards where player_id=1 and house_name='DA_HouseKenola' and template_id=?`, tmpl)
}

func removeKenola(t *testing.T, o *Ops, was, to float64) map[string]any {
	t.Helper()
	return applyWith(t, o, doubled(), house(175, Args{"was_mine": was, "mine_to": to, "lower": true}))
}

func counts(m map[string]any) [4]int64 {
	return [4]int64{m["rewards_taken_back"].(int64), m["rewards_kept_collected"].(int64), m["rewards_kept_unsure"].(int64), m["swatches_taken_back"].(int64)}
}

// The top tier (the swatch) removed while it still waits: the swatch row and the character's one-time row of this term go, the rows of the tiers below stay, and nothing else changes.
func TestTakeBackTheWaitingSwatchOfTheRemovedTopTier(t *testing.T) {
	o := rewardOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	if oneTimeRows(t, o) == "" {
		t.Fatal("the raise did not write the one-time row")
	}
	solari, scrip := wallet(t, o, 0), wallet(t, o, 1)
	maros, guild := houseRewards(t, o, "DA_HouseMaros"), rows(t, o, `select * from landsraad_house_rewards where player_id=50`)
	below := rowOf(t, o, "Schematic.L") + rowOf(t, o, "T6RangeFinder") + rowOf(t, o, "Schematic.K")
	m := removeKenola(t, o, 6000, 4800)
	if got := counts(m); got != [4]int64{1, 0, 0, 1} || m["contribution_lowered"].(int64) != 1 {
		t.Fatalf("answer: %v", m)
	}
	if rowOf(t, o, "Kenola_Placeables_Swatch") != "" {
		t.Fatal("the waiting swatch was not taken back")
	}
	if got := oneTimeRows(t, o); got != "" {
		t.Fatalf("the one-time row stays, so the swatch could not be won again: %s", got)
	}
	if rowOf(t, o, "Schematic.L")+rowOf(t, o, "T6RangeFinder")+rowOf(t, o, "Schematic.K") != below {
		t.Fatal("the rows of the tiers that stay were changed")
	}
	if wallet(t, o, 0) != solari || wallet(t, o, 1) != scrip {
		t.Fatal("a wallet changed: the pay-out of the missions is not taken back")
	}
	if houseRewards(t, o, "DA_HouseMaros") != maros || rows(t, o, `select * from landsraad_house_rewards where player_id=50`) != guild {
		t.Fatal("another house's or another player's rows were touched")
	}
	if c := contribution(t, o, 1, 175); c != 4800 {
		t.Fatalf("contribution: %v", c)
	}
	if d := lastEdit(t, o); !strings.Contains(d, "took back 1 waiting reward") {
		t.Fatalf("the queued change does not say what was taken back: %s", d)
	}
}

// Several tiers at once (5 to 3), a removal down to nothing, and the same item at nothing else: every waiting row of the removed tiers goes and only those.
func TestTakeBackSeveralTiersAndEverything(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	m := removeKenola(t, o, 6000, 3600) // tiers 4 and 5 (Schematic.L and the swatch) are removed
	if got := counts(m); got != [4]int64{2, 0, 0, 1} {
		t.Fatalf("answer: %v", m)
	}
	if rowOf(t, o, "Schematic.L") != "" || rowOf(t, o, "Kenola_Placeables_Swatch") != "" {
		t.Fatal("the rows of the removed tiers stay")
	}
	for _, tmpl := range []string{"SolarisCoin", "Schematic.K", "T6RangeFinder"} {
		if rowOf(t, o, tmpl) == "" {
			t.Fatalf("the row of a tier that stays (%s) is gone", tmpl)
		}
	}
	m = removeKenola(t, o, 3600, 0) // the rest
	if got := counts(m); got != [4]int64{3, 0, 0, 0} {
		t.Fatalf("answer: %v", m)
	}
	if got := rows(t, o, `select template_id from landsraad_house_rewards where player_id=1 and house_name='DA_HouseKenola'`); got != "" {
		t.Fatalf("rows left for Kenola: %s", got)
	}
	if got := rows(t, o, `select amount from landsraad_house_rewards where player_id=50`); got != "map[amount:10000]\n" {
		t.Fatalf("the guild's row: %s", got)
	}
}

// Collected rewards stay and are counted; waiting ones go; a row of the removed tier that is not this term's is not touched.
func TestTakeBackLeavesCollectedRewardsAndSaysSo(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	setup(t, o, `update landsraad_house_rewards set amount=0 where player_id=1 and template_id in ('Schematic.L', 'SolarisCoin')`)
	m := removeKenola(t, o, 6000, 3600)
	if got := counts(m); got != [4]int64{1, 1, 0, 1} {
		t.Fatalf("answer: %v", m)
	}
	if rowOf(t, o, "Schematic.L") != "map[amount:0 last_updated:150]\n" {
		t.Fatalf("the collected row: %s", rowOf(t, o, "Schematic.L"))
	}
	if d := lastEdit(t, o); !strings.Contains(d, "1 collected reward stay") {
		t.Fatalf("the description: %s", d)
	}
	// everything collected: nothing is taken back and nothing is written to the rows
	o = rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	collectRewards(t, o)
	before := rewardRows(t, o)
	m = removeKenola(t, o, 6000, 0)
	if got := counts(m); got != [4]int64{0, 5, 0, 0} || rewardRows(t, o) != before {
		t.Fatalf("answer %v; rows changed:\n%s\n%s", m, before, rewardRows(t, o))
	}
	if got := oneTimeRows(t, o); got == "" {
		t.Fatal("the one-time row of a collected swatch was deleted")
	}
}

// A row for a tier below the removed ones is never touched, and a removal of a tier whose item has no row says nothing is waiting.
func TestTakeBackNoRowNothingToTakeAndNoRowsOfOtherTiersTouched(t *testing.T) {
	o := rewardOps(t, "")
	m := removeKenola(t, o, 1300, 0) // Kenola at 1,300 has no row at all (the character got there with no reward write)
	if got := counts(m); got != [4]int64{0, 0, 0, 0} || m["contribution_lowered"].(int64) != 1 {
		t.Fatalf("answer: %v", m)
	}
	if got := rewardRows(t, o); !strings.Contains(got, "DA_HouseMaros") || strings.Contains(got, "Kenola") && !strings.Contains(got, "player_id:50") {
		t.Fatalf("rows: %s", got)
	}
}

// The same request twice, and a removal that goes on from where an earlier one stopped, take back each reward once.
func TestTakeBackIsIdempotent(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	removeKenola(t, o, 6000, 4800)
	after := rewardRows(t, o)
	// the page shows 4,800 now; the same removal request (still saying 6,000) is a conflict and writes nothing
	if _, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 6000.0, "mine_to": 4800.0, "lower": true}))); err == nil {
		t.Fatal("a stale removal was accepted")
	}
	if rewardRows(t, o) != after {
		t.Fatal("a refused removal changed rows")
	}
	// a removal to where the contribution already is changes nothing
	m := removeKenola(t, o, 4800, 4800)
	if m["changed"].(int64) != 0 || counts(m) != [4]int64{} || rewardRows(t, o) != after {
		t.Fatalf("answer: %v", m)
	}
	// the next tier goes once
	m = removeKenola(t, o, 4800, 3600)
	if counts(m) != [4]int64{1, 0, 0, 0} || rowOf(t, o, "Schematic.L") != "" {
		t.Fatalf("answer: %v", m)
	}
	m = removeKenola(t, o, 3600, 3600)
	if m["changed"].(int64) != 0 {
		t.Fatalf("answer: %v", m)
	}
}

// What the take-back cannot tie to the removed tier with certainty stays, and the answer says so.
func TestTakeBackLeavesWhatItCannotTieToTheTier(t *testing.T) {
	cases := []struct {
		name  string
		setup string
		to    float64
		stays string // the item whose row must still be there
		want  [4]int64
	}{
		// tier 4,800 now has the item of tier 2,400, which stays: its waiting amount cannot be told from that tier's; the swatch of the 6,000 tier is taken back
		{"the same item is the reward of a tier that stays", `update landsraad_task_rewards set template_id='Schematic.K' where task_id=175 and threshold=4800`, 3600, "Schematic.K", [4]int64{1, 0, 1, 1}},
		// the tier 3,600 paid 50 and 10 wait: the 4,800 and the swatch are taken back, the 3,600 stays
		{"less is waiting than the tier paid", `update landsraad_house_rewards set amount=10 where player_id=1 and template_id='T6RangeFinder'`, 2400, "T6RangeFinder", [4]int64{2, 0, 1, 1}},
		// the row of the 3,600 tier was written before this term started
		{"the waiting row is of an earlier term", `update landsraad_house_rewards set last_updated=50 where player_id=1 and template_id='T6RangeFinder'`, 2400, "T6RangeFinder", [4]int64{2, 0, 1, 1}},
	}
	for _, c := range cases {
		o := rewardOps(t, "")
		kenolaFromZero(t, o, 6000)
		setup(t, o, c.setup)
		m := removeKenola(t, o, 6000, c.to)
		if got := counts(m); got != c.want {
			t.Fatalf("%s: answer %v (counts %v, want %v)", c.name, m, got, c.want)
		}
		if rowOf(t, o, c.stays) == "" {
			t.Fatalf("%s: the row of %s was taken although it cannot be tied to the tier", c.name, c.stays)
		}
		if d := lastEdit(t, o); !strings.Contains(d, "left alone") {
			t.Fatalf("%s: the description does not say what stays: %s", c.name, d)
		}
	}
}

// Tiers the game paid for the character's own missions (progress events) stay, whatever the row says.
func TestTakeBackLeavesTiersTheCharactersOwnMissionsCover(t *testing.T) {
	o := rewardOps(t, kenolaEvents) // events of 1,300 cover the first tier only
	kenolaFromZero(t, o, 3600)
	// the game wrote the row of the first tier for those missions, and it waits
	setup(t, o, `insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', 10000, 'SolarisCoin', 150)`)
	m := removeKenola(t, o, 3600, 0)
	if got := counts(m); got != [4]int64{2, 0, 1, 0} { // 2,400 and 3,600 are taken back; the first tier (SolarisCoin) is covered by the events
		t.Fatalf("answer: %v", m)
	}
	if rowOf(t, o, "SolarisCoin") == "" {
		t.Fatal("the row of a tier the character's own events cover was taken back")
	}
	// the ground under another faction counts the same way
	o = rewardOps(t, "")
	kenolaFromZero(t, o, 3600)
	setup(t, o, `insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) values (1, 1, 175, 2400)`)
	m = removeKenola(t, o, 3600, 0)
	if got := counts(m); got != [4]int64{1, 0, 2, 0} { // only the 3,600 tier is above what the other faction's row covers
		t.Fatalf("answer: %v", m)
	}
}

// The house swatch is taken back only when the editor's own record says it paid that ground: a swatch the game wrote (no record) stays.
func TestTakeBackLeavesASwatchTheEditorDidNotWrite(t *testing.T) {
	o := rewardOps(t, `insert into landsraad_player_one_time_house_rewards(player_id, house_name, template_id, term_id) values (1, 'DA_HouseKenola', 'Kenola_Placeables_Swatch', 7);
		insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', 1, 'Kenola_Placeables_Swatch', 150);
		update landsraad_task_player_contributions set amount=6000 where player_id=1 and task_id=175;`)
	before := oneTimeRows(t, o)
	m := removeKenola(t, o, 6000, 4800)
	if got := counts(m); got != [4]int64{0, 0, 1, 0} {
		t.Fatalf("answer: %v", m)
	}
	if rowOf(t, o, "Kenola_Placeables_Swatch") == "" || oneTimeRows(t, o) != before {
		t.Fatal("a swatch the editor did not write was taken back")
	}
	// a record that says the editor paid it, written to the file: the swatch goes
	o, _ = paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	if m = removeKenola(t, o, 6000, 4800); counts(m) != [4]int64{1, 0, 0, 1} {
		t.Fatalf("answer with the record in its file: %v", m)
	}
}

// A damaged record does not stop a removal; it only keeps the swatch.
func TestTakeBackWithAnUnreadableRecordStillRemovesAndKeepsTheSwatch(t *testing.T) {
	o, path := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	damageRecord(t, path)
	m := removeKenola(t, o, 6000, 3600)
	if got := counts(m); got != [4]int64{1, 0, 1, 0} || m["contribution_lowered"].(int64) != 1 {
		t.Fatalf("answer: %v", m)
	}
	if rowOf(t, o, "Schematic.L") != "" || rowOf(t, o, "Kenola_Placeables_Swatch") == "" {
		t.Fatal("Schematic.L should go and the swatch stay")
	}
}

// The board says before the click what the removal does, by the same rule: take, collected, none, or not certain with the reason.
func TestTakeBackTheBoardSaysWhatARemovalWouldDo(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	setup(t, o, `update landsraad_house_rewards set amount=0 where player_id=1 and template_id='Schematic.L'`)
	setup(t, o, `delete from landsraad_house_rewards where player_id=1 and template_id='SolarisCoin'`)
	say := func(th int64) (string, string) {
		r := tierOf(t, taskByID(t, o, 175), th)
		s, _ := r["takeback"].(string)
		w, _ := r["takeback_why"].(string)
		return s, w
	}
	for th, want := range map[int64]string{1200: tbNone, 2400: tbTake, 3600: tbTake, 4800: tbCollected, 6000: tbTake} {
		if got, why := say(th); got != want || (want != tbUnsure && why != "") {
			t.Fatalf("tier %d: %q %q, want %q", th, got, why, want)
		}
	}
	// the answer of the write for the top tier agrees with what the board said
	if m := removeKenola(t, o, 6000, 4800); counts(m) != [4]int64{1, 0, 0, 1} {
		t.Fatalf("answer: %v", m)
	}
	// a tier that is not reached has no outcome
	if got, _ := say(6000); got != "" {
		t.Fatalf("a tier that is not reached says %q", got)
	}
	// a swatch the record does not vouch for is "not certain" on the board too
	o = rewardOps(t, `insert into landsraad_player_one_time_house_rewards(player_id, house_name, template_id, term_id) values (1, 'DA_HouseKenola', 'Kenola_Placeables_Swatch', 7);
		insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', 1, 'Kenola_Placeables_Swatch', 150);
		update landsraad_task_player_contributions set amount=6000 where player_id=1 and task_id=175;`)
	if got, why := say(6000); got != tbUnsure || why != tbWhyUnproven {
		t.Fatalf("the swatch: %q %q", got, why)
	}
}

// A raise after a take-back pays the taken-back tiers again: the reward rows and, with them, the missions (their Solari, House Scrip and XP were not taken back). This is the
// consequence of the one rule (the record is trusted as far as the save shows reward rows), and it is the reason the question before a removal says the pay-out of the missions stays.
func TestTakeBackARaiseAfterATakeBackPaysTheTiersAgain(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	solari := wallet(t, o, 0)
	removeKenola(t, o, 6000, 0)
	if wallet(t, o, 0) != solari {
		t.Fatal("the removal took back Solari")
	}
	m := applyWith(t, o, doubled(), house(175, Args{"mine_to": 6000.0}))
	if m["reward_rows"].(int64) != 5 || m["missions"].(int64) != 3 || wallet(t, o, 0) != solari+18000 {
		t.Fatalf("answer: %v, Solari %d -> %d", m, solari, wallet(t, o, 0))
	}
	// with the rewards collected the same raise pays nothing again (#777)
	o = rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	collectRewards(t, o)
	removeKenola(t, o, 6000, 0)
	if m := applyWith(t, o, doubled(), house(175, Args{"mine_to": 6000.0})); m["reward_rows"].(int64) != 0 || m["missions"].(int64) != 0 {
		t.Fatalf("a raise over collected rewards paid again: %v", m)
	}
}

// Without the swatch tables (the older layout) the top tier is the task's own item and is taken back like any other.
func TestTakeBackWithoutSwatchTables(t *testing.T) {
	o := mineOps(t, goalWorld)
	mustApply(t, o, applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0})))
	m := mustApply(t, o, applyArgs(house(175, Args{"was_mine": 6000.0, "mine_to": 4800.0, "lower": true})))
	if m["rewards_taken_back"].(int64) != 1 || m["swatches_taken_back"].(int64) != 0 {
		t.Fatalf("answer: %v", m)
	}
	if got := rows(t, o, `select amount from landsraad_house_rewards where player_id=1 and template_id='Swatch.Kenola'`); got != "" {
		t.Fatalf("the task's own top item stays: %s", got)
	}
}

// A request that is refused writes nothing, rows included.
func TestTakeBackARefusedRemovalTakesNothingBack(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	wantApplyError(t, o, applyArgs(house(175, Args{"was_mine": 6000.0, "mine_to": 3600.0})), apierr.ErrBadArgument)
	wantApplyError(t, o, applyArgs(house(175, Args{"was_mine": 5000.0, "mine_to": 3600.0, "lower": true})), apierr.ErrConflict)
}
