package ops

import (
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"math"
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

// earnedOf is the number of waiting rewards that stay because the character's own missions cover their tier.
func earnedOf(m map[string]any) int64 { return m["rewards_kept_earned"].(int64) }

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
	if got := counts(m); got != [4]int64{2, 0, 0, 0} || earnedOf(m) != 1 { // 2,400 and 3,600 are taken back; the waiting row of the first tier (SolarisCoin) is covered by the events and stays, said apart from "not certain"
		t.Fatalf("answer: %v", m)
	}
	if d := lastEdit(t, o); !strings.Contains(d, "1 waiting reward stays (your own missions cover the tier)") || strings.Contains(d, "not certain") {
		t.Fatalf("the description: %s", d)
	}
	if rowOf(t, o, "SolarisCoin") == "" {
		t.Fatal("the row of a tier the character's own events cover was taken back")
	}
	// the ground under another faction counts the same way
	o = rewardOps(t, "")
	kenolaFromZero(t, o, 3600)
	setup(t, o, `insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) values (1, 1, 175, 2400)`)
	m = removeKenola(t, o, 3600, 0)
	if got := counts(m); got != [4]int64{1, 0, 0, 0} || earnedOf(m) != 2 { // only the 3,600 tier is above what the other faction's row covers
		t.Fatalf("answer: %v", m)
	}
	// a covered tier with no waiting row (or a collected one) is nothing at all: it is not counted as left alone, and it is not "not certain"
	o = rewardOps(t, "")
	kenolaFromZero(t, o, 3600)
	setup(t, o, `insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) values (1, 1, 175, 2400)`)
	setup(t, o, `delete from landsraad_house_rewards where player_id=1 and template_id='SolarisCoin'`)
	setup(t, o, `update landsraad_house_rewards set amount=0 where player_id=1 and template_id='Schematic.K'`)
	m = removeKenola(t, o, 3600, 0)
	if got := counts(m); got != [4]int64{1, 0, 0, 0} || earnedOf(m) != 0 {
		t.Fatalf("answer: %v", m)
	}
	if d := lastEdit(t, o); strings.Contains(d, "left alone") || strings.Contains(d, "stay") {
		t.Fatalf("the description names rewards that were not there: %s", d)
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

// A damaged record does not stop a removal; nothing vouches for the waiting rewards then, so they all stay and the answer says so.
func TestTakeBackWithAnUnreadableRecordStillRemovesAndKeepsTheRewards(t *testing.T) {
	o, path := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	damageRecord(t, path)
	m := removeKenola(t, o, 6000, 3600)
	if got := counts(m); got != [4]int64{0, 0, 2, 0} || m["contribution_lowered"].(int64) != 1 {
		t.Fatalf("answer: %v", m)
	}
	if rowOf(t, o, "Schematic.L") == "" || rowOf(t, o, "Kenola_Placeables_Swatch") == "" {
		t.Fatal("a reward was taken back although nothing vouches that the editor wrote it")
	}
	// the damaged file is left as it is by a removal: no note is written for it
	if b, _ := os.ReadFile(path); string(b) != "not json" {
		t.Fatalf("the record was touched: %q", b)
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
	if got, why := say(6000); got != tbUnsure || why != tbWhySwatch {
		t.Fatalf("the swatch: %q %q", got, why)
	}
}

// A raise after a take-back writes the reward rows again and pays NOTHING else (#944): the removal took back the rewards, not the missions, so the Solari, House Scrip, XP and the faction's
// progress of the removed tiers stay paid. The same is true however often the tier is removed and raised, with the record in the queue, in the file, and with the edit's note kept.
func TestTakeBackARaiseAfterATakeBackWritesTheRowsAgainAndPaysNothingElse(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	after := rewardRows(t, o)
	solari, scrip := wallet(t, o, 0), wallet(t, o, 1)
	spec, _, _ := track(t, o, 2)
	facBefore, _ := progress(t, o, 2, 175)
	for cycle := 1; cycle <= 3; cycle++ {
		removeKenola(t, o, 6000, 0)
		if rowOf(t, o, "Kenola_Placeables_Swatch") != "" || wallet(t, o, 0) != solari {
			t.Fatalf("cycle %d: the removal left a row or took back Solari", cycle)
		}
		m := applyWith(t, o, doubled(), house(175, Args{"mine_to": 6000.0}))
		if m["reward_rows"].(int64) != 5 || m["swatches"].(int64) != 1 || m["missions"].(int64) != 0 || m["solari"].(int64) != 0 || m["house_scrip"].(int64) != 0 || m["missions_paid_before"].(int64) != 3 {
			t.Fatalf("cycle %d: answer %v", cycle, m)
		}
		if wallet(t, o, 0) != solari || wallet(t, o, 1) != scrip {
			t.Fatalf("cycle %d: wallets %d %d, were %d %d", cycle, wallet(t, o, 0), wallet(t, o, 1), solari, scrip)
		}
		if xp, _, _ := track(t, o, 2); xp != spec {
			t.Fatalf("cycle %d: specialization XP %d, was %d", cycle, xp, spec)
		}
		if v, _ := progress(t, o, 2, 175); v != facBefore {
			t.Fatalf("cycle %d: the faction's progress %v, was %v", cycle, v, facBefore)
		}
		if got := rewardRows(t, o); got != after {
			t.Fatalf("cycle %d: the rows are not what they were:\n%s\n%s", cycle, after, got)
		}
	}
}

// The same with the record in its file (written after each edit), with the note the file could not take kept for the next write, and over a restart of the editor's own state: the
// take-back ceiling lives in the record, so what the record says is what a raise believes.
func TestTakeBackTheCeilingIsKeptInTheRecordAndCountsWhereverTheRecordIs(t *testing.T) {
	o, path := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	if f := readPaid(t, path); len(f.Entries) != 1 || f.Entries[0].Back != 0 || f.Version != 1 {
		t.Fatalf("a record with no take-back carries one: %+v", f)
	}
	solari := wallet(t, o, 0)
	removeTo(t, o, 6000, 2400)
	// queued, not written: the board and the raise read the note the removal attached
	pg := taskByID(t, o, 175)
	if pg["my_paid"] != 6000.0 || pg["my_paid_rows"] != 2400.0 {
		t.Fatalf("the board after a queued removal: paid %v, rows %v", pg["my_paid"], pg["my_paid_rows"])
	}
	flush(t, o)
	f := readPaid(t, path)
	if len(f.Entries) != 1 || f.Entries[0].Paid != 6000 || f.Entries[0].Back != 6000 || f.Version != 1 {
		t.Fatalf("the record after the removal: %+v", f)
	}
	// the rows of the tiers above 2,400 are gone; the raise writes the three of them again and nothing else
	m := fillKenolaDoubled(t, o, 2400, 6000)
	if m["reward_rows"].(int64) != 3 || m["missions"].(int64) != 0 || wallet(t, o, 0) != solari || m["missions_paid_before"].(int64) != 1 || m["ground_paid_before"].(float64) != 3600 {
		t.Fatalf("answer: %v", m)
	}
	flush(t, o)
	// a higher ground than ever paid pays only the new missions: raise to the top after the record only reached 4,800
	o2, _ := paidOps(t, "")
	fillKenolaDoubled(t, o2, 1300, 4800)
	flush(t, o2)
	removeTo(t, o2, 4800, 0)
	flush(t, o2)
	s2 := wallet(t, o2, 0)
	m = fillKenolaDoubled(t, o2, 0, 6000)
	// step 2,000: ceil(6000/2000) - ceil(4800/2000) = 0 new missions; the five rewards of 1,200 to 6,000 are written, nothing is paid for 1,200 to 4,800
	if m["reward_rows"].(int64) != 5 || m["missions"].(int64) != 0 || m["ground_paid_before"].(float64) != 4800 || wallet(t, o2, 0) != s2 {
		t.Fatalf("answer: %v", m)
	}
}

// A higher raise after a take-back pays only the ground above what was paid before, and writes every row below it again.
func TestTakeBackAHigherRaiseAfterATakeBackPaysOnlyTheNewGround(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 2400) // paid: 1,200 and 2,400 (step 2,000: 2 missions)
	solari := wallet(t, o, 0)
	removeKenola(t, o, 2400, 0)
	m := applyWith(t, o, doubled(), house(175, Args{"mine_to": 4800.0}))
	// 4,800 is 3 missions at the step 2,000, 2,400 was 2: one mission is new; the rows of the four tiers are written (the two taken back and the two new ones)
	if m["missions"].(int64) != 1 || m["reward_rows"].(int64) != 4 || m["missions_paid_before"].(int64) != 2 || m["ground_paid_before"].(float64) != 2400 || wallet(t, o, 0) != solari+6000 {
		t.Fatalf("answer: %v", m)
	}
}

// With the rewards collected the same raise writes no row and pays nothing (#777): the take-back is not involved.
func TestTakeBackARaiseOverCollectedRewardsPaysNothingAndWritesNoRow(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	collectRewards(t, o)
	removeKenola(t, o, 6000, 0)
	if m := applyWith(t, o, doubled(), house(175, Args{"mine_to": 6000.0})); m["reward_rows"].(int64) != 0 || m["missions"].(int64) != 0 || m["rewards_already_given"].(int64) != 5 {
		t.Fatalf("a raise over collected rewards: %v", m)
	}
}

// The trusted ground of the record, in numbers: the cut at the rows that stand, the take-back ceiling that lifts it for the missions only, and what it never lifts.
func TestTakeBackTheCeilingLiftsTheGroundForMissionsAndNeverForRows(t *testing.T) {
	rec := func(paid, back float64) []paidNote {
		return []paidNote{{Player: 1, Term: 7, House: "A", Faction: 2, Paid: paid, Back: back}}
	}
	cases := []struct {
		name               string
		rec                []paidNote
		standing           float64
		ground, rows, back float64
		untrusted          bool
	}{
		{"no take-back, all rows stand", rec(6000, 0), 6000, 6000, 6000, 0, false},
		{"no take-back, rows missing: the record is cut", rec(6000, 0), 2400, 2400, 2400, 0, true},
		{"rows taken back on purpose: the missions stay paid, the rows are written again", rec(6000, 6000), 2400, 6000, 2400, 6000, false},
		{"a ceiling below the paid ground lifts only to the ceiling", rec(6000, 3600), 1200, 3600, 1200, 3600, true},
		{"a ceiling above the paid ground is cut to what was paid", rec(3600, 6000), 0, 3600, 0, 3600, false},
		{"no record", nil, 6000, 0, 0, 0, false},
	}
	for _, c := range cases {
		g := trustedPaid(c.rec, nil, 1, 7, "A", c.standing)
		if g.ground != c.ground || g.rows != c.rows || g.back != c.back || g.untrusted != c.untrusted {
			t.Errorf("%s: %+v, want ground %v rows %v back %v untrusted %v", c.name, g, c.ground, c.rows, c.back, c.untrusted)
		}
	}
	// another house, another term and another character never share it
	if g := trustedPaid(rec(6000, 6000), nil, 1, 7, "B", 0); g.ground != 0 {
		t.Errorf("another house: %+v", g)
	}
	if g := trustedPaid(rec(6000, 6000), nil, 2, 7, "A", 0); g.ground != 0 {
		t.Errorf("another character: %+v", g)
	}
	if g := trustedPaid(rec(6000, 6000), nil, 1, 8, "A", 0); g.ground != 0 {
		t.Errorf("another term: %+v", g)
	}
}

// The note's Back is only ever raised, it counts under any faction, a note that replaces carries it, and an entry whose ceiling is not a number from 0 to the largest tier is left out.
func TestTakeBackTheCeilingMergesOnlyUpwardAndAnImplausibleOneIsLeftOut(t *testing.T) {
	es := mergePaid([]paidNote{{Player: 1, Term: 7, House: "A", Faction: 1, Paid: 6000, Back: 3600}}, []paidNote{
		{Player: 1, Term: 7, House: "A", Faction: 1, Paid: 6000, Back: 2400}, // lower: does not lower
		{Player: 1, Term: 7, House: "A", Faction: 2, Paid: 4800, Back: 4800},
	})
	if got := highestBack(es, 1, 7, "A"); got != 4800 || es[0].Back != 3600 {
		t.Fatalf("back %v, first entry %+v", got, es[0])
	}
	es = mergePaid(es, []paidNote{{Player: 1, Term: 7, House: "A", Faction: 2, Paid: 6000, Back: 4800, Replace: true}})
	if len(es) != 1 || es[0].Back != 4800 || es[0].Paid != 6000 {
		t.Fatalf("a note that replaces: %+v", es)
	}
	for name, e := range map[string]paidNote{
		"negative":     {Player: 1, Term: 7, House: "A", Paid: 6000, Back: -1},
		"above a tier": {Player: 1, Term: 7, House: "A", Paid: 6000, Back: maxTier + 1},
		"not a number": {Player: 1, Term: 7, House: "A", Paid: 6000, Back: math.NaN()},
	} {
		if e.valid() {
			t.Errorf("%s: an entry with this ceiling is taken", name)
		}
	}
	if (paidNote{Player: 1, Term: 7, House: "A", Paid: 6000, Back: 6000}).valid() == false {
		t.Error("a plausible ceiling is refused")
	}
}

// The ceiling is attached to the removal's own edit: a refused or failed removal leaves no note.
func TestTakeBackNoNoteWhenNothingIsTakenBack(t *testing.T) {
	o, path := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	collectRewards(t, o)
	flush(t, o)
	removeTo(t, o, 6000, 0)
	flush(t, o)
	if f := readPaid(t, path); len(f.Entries) != 1 || f.Entries[0].Back != 0 {
		t.Fatalf("a removal of collected rewards recorded a ceiling: %+v", f)
	}
}

// Rows, not tiers (#944): two removed tiers that give one item are one row, and the answer counts it once.
func TestTakeBackCountsRowsNotTiers(t *testing.T) {
	o := rewardOps(t, "")
	setup(t, o, `update landsraad_task_rewards set template_id='Schematic.K' where task_id=175 and threshold=3600`) // the tiers 2,400 and 3,600 now give one item
	kenolaFromZero(t, o, 6000)
	if got := rows(t, o, `select amount from landsraad_house_rewards where player_id=1 and template_id='Schematic.K'`); got != "map[amount:51]\n" {
		t.Fatalf("the two tiers are one row: %s", got)
	}
	m := removeKenola(t, o, 6000, 1200) // removes the tiers 2,400, 3,600, 4,800 and the swatch: four tiers, three rows
	if got := counts(m); got != [4]int64{3, 0, 0, 1} {
		t.Fatalf("answer: %v", m)
	}
	if d := lastEdit(t, o); !strings.Contains(d, "took back 3 waiting rewards") {
		t.Fatalf("the description: %s", d)
	}
}

// The same item at two removed tiers, one of them collected: one row, one collected, nothing taken for it.
func TestTakeBackCountsACollectedSharedRowOnce(t *testing.T) {
	o := rewardOps(t, "")
	setup(t, o, `update landsraad_task_rewards set template_id='Schematic.K' where task_id=175 and threshold=3600`)
	kenolaFromZero(t, o, 6000)
	setup(t, o, `update landsraad_house_rewards set amount=0 where player_id=1 and template_id='Schematic.K'`)
	m := removeKenola(t, o, 6000, 1200)
	if got := counts(m); got != [4]int64{2, 1, 0, 1} {
		t.Fatalf("answer: %v", m)
	}
}

// Only a row the editor's own record vouches for is taken back (#944): the record says up to which ground the editor paid, and a waiting row above it (the game wrote it, or an earlier
// build did) is not known to be the editor's and stays, with the reason on the board and in the answer.
func TestTakeBackOnlyTheRowsTheEditorsRecordVouchesFor(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 2400) // the editor paid 1,200 and 2,400
	// the character stands at 4,800 and the game wrote the rows of 3,600 and 4,800 itself: no event, no record of those tiers
	setup(t, o, `update landsraad_task_player_contributions set amount=4800 where player_id=1 and task_id=175`)
	setup(t, o, `insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', 50, 'T6RangeFinder', 150), (1, 'DA_HouseKenola', 1, 'Schematic.L', 150)`)
	r := func(th int64) (string, string) {
		x := tierOf(t, taskByID(t, o, 175), th)
		a, _ := x["takeback"].(string)
		b, _ := x["takeback_why"].(string)
		return a, b
	}
	if a, w := r(4800); a != tbUnsure || w != tbWhyUnproven {
		t.Fatalf("the board, tier 4,800: %q %q", a, w)
	}
	if a, w := r(2400); a != tbTake || w != "" {
		t.Fatalf("the board, tier 2,400: %q %q", a, w)
	}
	m := removeKenola(t, o, 4800, 0)
	if got := counts(m); got != [4]int64{2, 0, 2, 0} {
		t.Fatalf("answer: %v", m)
	}
	if rowOf(t, o, "T6RangeFinder") == "" || rowOf(t, o, "Schematic.L") == "" || rowOf(t, o, "SolarisCoin") != "" || rowOf(t, o, "Schematic.K") != "" {
		t.Fatal("the rows the record does not vouch for must stay and the others go")
	}
}

// The part of a row that is not the removed tier's stays: a row of 6 of which the removed tier paid 1 goes down to 5, and its time is not touched.
func TestTakeBackTheRemainderOfARowStaysAndKeepsItsTime(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	setup(t, o, `update landsraad_house_rewards set amount=6, last_updated=160 where player_id=1 and template_id='Schematic.L'`) // 1 of the tier 4,800 and 5 of another origin
	m := removeKenola(t, o, 6000, 3600)
	if got := counts(m); got != [4]int64{2, 0, 0, 1} {
		t.Fatalf("answer: %v", m)
	}
	if got := rowOf(t, o, "Schematic.L"); got != "map[amount:5 last_updated:160]\n" {
		t.Fatalf("the row after the removal: %s", got)
	}
	// the next removal finds 5 waiting for a tier that paid 1 and nothing below: it is the same item, no other tier, so it goes down to 4 and is never deleted by a tier of 1
	m = removeKenola(t, o, 3600, 0)
	if rowOf(t, o, "Schematic.L") != "map[amount:5 last_updated:160]\n" || counts(m)[0] != 3 {
		t.Fatalf("a tier below must not touch the row of an item it does not give: %v %s", m, rowOf(t, o, "Schematic.L"))
	}
}

// A write that fails halfway leaves nothing behind: the contribution, the rows and the one-time row are what they were (one transaction), and the removal is refused with the error.
func TestTakeBackAFailingWriteRollsEverythingBack(t *testing.T) {
	cases := []struct {
		name, trigger, prep string
	}{
		{"the update of a row that keeps a remainder", `create trigger boom before update on landsraad_house_rewards begin select raise(abort, 'boom'); end`,
			`update landsraad_house_rewards set amount=6 where player_id=1 and template_id='Schematic.L'`},
		{"the delete of a row", `create trigger boom before delete on landsraad_house_rewards begin select raise(abort, 'boom'); end`, ``},
		{"the delete of the one-time swatch row, after the swatch row was deleted", `create trigger boom before delete on landsraad_player_one_time_house_rewards begin select raise(abort, 'boom'); end`, ``},
	}
	for _, c := range cases {
		o := rewardOps(t, "")
		kenolaFromZero(t, o, 6000)
		if c.prep != "" {
			setup(t, o, c.prep)
		}
		setup(t, o, c.trigger)
		rowsBefore, oneBefore, edits := rewardRows(t, o), oneTimeRows(t, o), len(o.S.Pending())
		_, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 6000.0, "mine_to": 3600.0, "lower": true})))
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("%s: the write did not fail with the error: %v", c.name, err)
		}
		if got := contribution(t, o, 1, 175); got != 6000 {
			t.Fatalf("%s: the contribution went down although the write failed: %v", c.name, got)
		}
		if rewardRows(t, o) != rowsBefore || oneTimeRows(t, o) != oneBefore || len(o.S.Pending()) != edits {
			t.Fatalf("%s: something was written by a failed removal", c.name)
		}
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

// A record that lost notes of ground it paid (a failing sink past its bound) does not say what it vouches for: a raise is refused, a removal still goes through and no waiting reward is taken back.
func TestTakeBackWithADroppedNoteStillRemovesAndKeepsTheRewards(t *testing.T) {
	o, _ := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	fail := true
	failingRecord(o, &fail)
	for i := 0; i < 1001; i++ {
		if _, err := o.S.Mutate("note", func(m *save.Mut) error {
			if _, err := m.Exec(`update player_virtual_currency_balances set balance=balance+1 where player_controller_id=1 and currency_id=0`); err != nil {
				return err
			}
			return m.Note(paidNoteKind, paidNote{Player: 1, Term: 7, House: "DA_HouseOther", Faction: 2, Paid: 100})
		}); err != nil {
			t.Fatal(err)
		}
	}
	flushWithWarning(t, o)
	if !o.S.NotesDropped() {
		t.Fatal("the save does not say that notes were dropped")
	}
	m := removeKenola(t, o, 6000, 3600)
	if got := counts(m); got != [4]int64{0, 0, 2, 0} || m["contribution_lowered"].(int64) != 1 {
		t.Fatalf("answer: %v", m)
	}
	if rowOf(t, o, "Schematic.L") == "" || rowOf(t, o, "Kenola_Placeables_Swatch") == "" {
		t.Fatal("a reward was taken back although the record cannot say what it vouches for")
	}
}
