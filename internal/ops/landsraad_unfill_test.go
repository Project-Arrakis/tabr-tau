package ops

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// Unfill chosen (#932): the character's own contribution goes to 0 on every chosen House, in one edit, and the rewards of the removed tiers follow the one take-back rule
// (landsraad_takeback.go). The board's read says before the click what that does (boardUnfill: the same rule, to 0), and the request carries the number of rewards it said would be taken
// back (was_taken), so a write that would take back another number is refused. World: rewardOps (landsraad_rewards_test.go): Kenola (175) 1,300, Vernius (176) with no row, Maros (174) 7,850
// with five waiting rows the game wrote.

// unfillOf is the board's read of what an Unfill would do on one house.
func unfillOf(t *testing.T, o *Ops, id int64) map[string]any {
	t.Helper()
	u, ok := taskByID(t, o, id)["unfill"].(map[string]any)
	if !ok {
		t.Fatalf("house %d: the read carries no unfill", id)
	}
	return u
}

// unfillHouses is the request items of "Unfill chosen" the page builds from the read: every house with a contribution above 0, set to 0 with lower: true, carrying what it was shown.
func unfillHouses(t *testing.T, o *Ops, ids ...int64) []map[string]any {
	t.Helper()
	var hs []map[string]any
	for _, id := range ids {
		task := taskByID(t, o, id)
		mine := task["my_contribution"].(float64)
		if mine <= 0 {
			continue
		}
		var win any // JSON carries a number or null
		if w, ok := task["winning_faction_id"].(int64); ok {
			win = float64(w)
		}
		hs = append(hs, house(float64(id), Args{"was_mine": mine, "was_done": task["completed"] == int64(1), "was_win": win, "mine_to": 0.0, "lower": true, "was_taken": float64(unfillOf(t, o, id)["take"].(int64))}))
	}
	return hs
}

// unfill plays the click: the request is built from the read, and the answer is returned.
func unfill(t *testing.T, o *Ops, ids ...int64) map[string]any {
	t.Helper()
	return applyWith(t, o, doubled(), unfillHouses(t, o, ids...)...)
}

// the reward rows of the character, by house and item
func ownRows(t *testing.T, o *Ops) map[string]int64 {
	t.Helper()
	rs, err := o.S.Query(`select house_name, template_id, amount from landsraad_house_rewards where player_id=1`)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int64{}
	for _, r := range rs {
		out[fmt.Sprint(r["house_name"], "/", r["template_id"])] = r["amount"].(int64)
	}
	return out
}

func answerCounts(m map[string]any) [5]int64 {
	return [5]int64{m["rewards_taken_back"].(int64), m["rewards_kept_collected"].(int64), m["rewards_kept_unsure"].(int64), m["rewards_kept_earned"].(int64), m["swatches_taken_back"].(int64)}
}

func previewCounts(u map[string]any) [5]int64 {
	return [5]int64{u["take"].(int64), u["collected"].(int64), u["unsure"].(int64), u["earned"].(int64), u["swatches"].(int64)}
}

// Several houses are one edit: each contribution goes to 0, the waiting rewards of the editor's own raises go, the pay-out of the missions and the completed state of a house stay, and
// the answer counts what the board said it would.
func TestUnfillSeveralHousesIsOneEditThatTakesBackWhatTheRecordVouchesFor(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	applyWith(t, o, doubled(), house(176, Args{"was_mine": 0.0, "mine_to": 3600.0}))
	solari, scrip := wallet(t, o, 0), wallet(t, o, 1)
	xp2, _, _ := track(t, o, 2)
	facK, _ := progress(t, o, 2, 175)
	facM, _ := progress(t, o, 2, 174)
	maros := houseRow(t, o, 174)
	pk, pv, pm := unfillOf(t, o, 175), unfillOf(t, o, 176), unfillOf(t, o, 174)
	if previewCounts(pk) != [5]int64{5, 0, 0, 0, 1} || previewCounts(pv) != [5]int64{3, 0, 0, 0, 0} {
		t.Fatalf("the board's preview: %v %v", pk, pv)
	}
	if previewCounts(pm) != [5]int64{0, 0, 4, 0, 0} {
		t.Fatalf("the rows the game wrote on Maros stay (the record has no word for them; the swatch has no row): %v", pm)
	}
	before := ownRows(t, o)
	m := unfill(t, o, 174, 175, 176)
	if m["contribution_lowered"].(int64) != 3 || m["contribution_raised"].(int64) != 0 || m["houses"].(int64) != 3 || m["changed"].(int64) != 3 {
		t.Fatalf("answer: %v", m)
	}
	if got, want := answerCounts(m), [5]int64{8, 0, 4, 0, 1}; got != want {
		t.Fatalf("counts %v, want %v: %v", got, want, m)
	}
	for _, id := range []int64{174, 175, 176} {
		if rows(t, o, `select 1 from landsraad_task_player_contributions where player_id=1 and task_id=?`, id) != "" {
			t.Fatalf("house %d: the row of tier 1 stays (a contribution of 0 is no row)", id)
		}
	}
	after := ownRows(t, o)
	for k, v := range before {
		switch {
		case k[:len("DA_HouseMaros")] == "DA_HouseMaros":
			if after[k] != v {
				t.Fatalf("a row the editor has no record of changed: %s %d -> %d", k, v, after[k])
			}
		default:
			if _, there := after[k]; there {
				t.Fatalf("a row the editor wrote and that waited stays: %s", k)
			}
		}
	}
	if contribution(t, o, 50, 175) != 3000 {
		t.Fatal("another player's contribution was touched")
	}
	if wallet(t, o, 0) != solari || wallet(t, o, 1) != scrip {
		t.Fatal("an unfill takes back no Solari or House Scrip")
	}
	if xp, _, _ := track(t, o, 2); xp != xp2 {
		t.Fatal("an unfill takes back no specialization XP")
	}
	if v, _ := progress(t, o, 2, 175); v != facK {
		t.Fatalf("the faction's progress moved: %v -> %v", facK, v)
	}
	if v, _ := progress(t, o, 2, 174); v != facM {
		t.Fatalf("the faction's progress moved: %v -> %v", facM, v)
	}
	if got := houseRow(t, o, 174); fmt.Sprint(got) != fmt.Sprint(maros) {
		t.Fatalf("a House's completed state is not touched by an unfill: %v -> %v", maros, got)
	}
}

// A collected reward is never touched: not deleted, not set, not counted as taken back; the question names it (collected) and so does the answer.
func TestUnfillNeverTouchesACollectedRewardAndNamesIt(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	setup(t, o, `update landsraad_house_rewards set amount=0 where player_id=1 and template_id in ('Schematic.L', 'SolarisCoin')`)
	u := unfillOf(t, o, 175)
	if previewCounts(u) != [5]int64{3, 2, 0, 0, 1} {
		t.Fatalf("preview: %v", u)
	}
	seen := map[string]string{}
	for _, x := range u["rows"].([]any) {
		r := x.(map[string]any)
		seen[r["template_id"].(string)] = r["outcome"].(string)
	}
	if seen["SolarisCoin"] != tbCollected || seen["Schematic.L"] != tbCollected || seen["Schematic.K"] != tbTake {
		t.Fatalf("the rows say: %v", seen)
	}
	m := unfill(t, o, 175)
	if answerCounts(m) != [5]int64{3, 2, 0, 0, 1} {
		t.Fatalf("answer: %v", m)
	}
	after := ownRows(t, o)
	for _, k := range []string{"DA_HouseKenola/SolarisCoin", "DA_HouseKenola/Schematic.L"} {
		if v, there := after[k]; !there || v != 0 {
			t.Fatalf("the collected row %s is %v (there: %v)", k, v, there)
		}
	}
	if len(after) != 3 { // the two collected rows of Kenola and the simulated guild's row stay in the table of the character (player 1: Maros has five rows)
		// Maros has its five rows too: 2 + 5 = 7
		if len(after) != 7 {
			t.Fatalf("rows left: %v", after)
		}
	}
}

// A request with a number of taken-back rewards other than the plan's is refused as a whole: nothing of the other houses is written either.
func TestUnfillIsRefusedWhenItWouldTakeBackAnotherNumberThanTheQuestionSaid(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	applyWith(t, o, doubled(), house(176, Args{"was_mine": 0.0, "mine_to": 3600.0}))
	// the question said 3 for Vernius, and a reward was collected meanwhile: now 2 would be taken back
	hs := unfillHouses(t, o, 175, 176)
	setup(t, o, `update landsraad_house_rewards set amount=0 where player_id=1 and house_name='DA_HouseVernius' and template_id='Schematic.V'`)
	before := boardRows(t, o)
	_, err := o.LandsraadApplyWith(doubled(), applyArgs(hs...))
	if !errors.Is(err, apierr.ErrConflict) {
		t.Fatalf("a stale question must be a conflict: %v", err)
	}
	if after := boardRows(t, o); after != before {
		t.Fatalf("a refused unfill wrote something:\n%s\n---\n%s", before, after)
	}
	if c := contribution(t, o, 1, 175); c != 6000 {
		t.Fatalf("Kenola was lowered although the request was refused: %v", c)
	}
	// built again from the read it is accepted
	m := unfill(t, o, 175, 176)
	if m["contribution_lowered"].(int64) != 2 || answerCounts(m)[0] != 7 || answerCounts(m)[1] != 1 {
		t.Fatalf("answer: %v", m)
	}
}

// A request without was_taken is not refused for it (a removal of one tier does not carry it), and a value that is not a number of rewards is a bad argument.
func TestUnfillWasTakenIsOptionalAndMustBeAWholeNumber(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	for _, bad := range []any{-1.0, 1.5, "x"} {
		wantApplyError(t, o, applyArgs(house(175, Args{"was_mine": 6000.0, "mine_to": 0.0, "lower": true, "was_taken": bad})), apierr.ErrBadArgument)
	}
	m := mustApply(t, o, applyArgs(house(175, Args{"was_mine": 6000.0, "mine_to": 0.0, "lower": true})))
	if answerCounts(m)[0] != 5 {
		t.Fatalf("answer: %v", m)
	}
	// 0 is a number too: a question that said "nothing is taken back" is refused when something would be
	o = rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	_, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 6000.0, "mine_to": 0.0, "lower": true, "was_taken": 0.0})))
	if !errors.Is(err, apierr.ErrConflict) {
		t.Fatalf("was_taken 0 against 5: %v", err)
	}
}

// A house with no reward tiers at all takes nothing back, and a question that said otherwise was about another house.
func TestUnfillAHouseWithNoTiersTakesNothingBackAndAStaleNumberIsRefused(t *testing.T) {
	o := rewardOps(t, `insert into landsraad_tasks(id, term_id, board_index, house_name, goal_amount) values (177, 7, 3, 'DA_HouseNoTiers', 4000);
		insert into landsraad_task_player_contributions(player_id, faction_id, task_id, amount) values (1, 2, 177, 700);`)
	if u := unfillOf(t, o, 177); u["tiers"].(int64) != 0 || u["take"].(int64) != 0 {
		t.Fatalf("preview: %v", u)
	}
	_, err := o.LandsraadApplyWith(doubled(), applyArgs(house(177, Args{"was_mine": 700.0, "mine_to": 0.0, "lower": true, "was_taken": 2.0})))
	if !errors.Is(err, apierr.ErrConflict) {
		t.Fatalf("a number that does not fit: %v", err)
	}
	if c := contribution(t, o, 1, 177); c != 700 {
		t.Fatalf("a refused unfill lowered the house: %v", c)
	}
	m := unfill(t, o, 177)
	if m["contribution_lowered"].(int64) != 1 || answerCounts(m) != [5]int64{} {
		t.Fatalf("answer: %v", m)
	}
}

// Unfill sets 0 only: it never raises, never completes and never reopens, and a house with nothing of the character's is not in the request.
func TestUnfillADoneHouseKeepsItsWinAndAHouseAtZeroIsLeftOut(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	was := houseRow(t, o, 175) // the fill took the house: the faction's share reached the goal
	if was["c"].(int64) != 1 {
		t.Fatalf("setup: %v", was)
	}
	hs := unfillHouses(t, o, 175, 176) // 176 has no contribution: left out
	if len(hs) != 1 {
		t.Fatalf("items: %v", hs)
	}
	m := applyWith(t, o, doubled(), hs...)
	if m["contribution_lowered"].(int64) != 1 || m["completed"].(int64) != 0 || m["reopened"].(int64) != 0 {
		t.Fatalf("answer: %v", m)
	}
	if got := houseRow(t, o, 175); fmt.Sprint(got) != fmt.Sprint(was) {
		t.Fatalf("the win changed: %v -> %v", was, got)
	}
}

// The same unfill again finds nothing to do (the page leaves the house out; a request that names it with a stale number is refused, not a no-op that lowers).
func TestUnfillAfterAnUnfillTakesBackNothingMoreAndAStaleRequestIsRefused(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	stale := unfillHouses(t, o, 175)
	unfill(t, o, 175)
	if hs := unfillHouses(t, o, 175); len(hs) != 0 {
		t.Fatalf("a house at 0 is not in the request: %v", hs)
	}
	_, err := o.LandsraadApplyWith(doubled(), applyArgs(stale...))
	if !errors.Is(err, apierr.ErrConflict) {
		t.Fatalf("the contribution is not what the page showed: %v", err)
	}
}

// What the board says an unfill does is what the write does, in every state the rule tells apart.
func TestUnfillThePreviewIsTheRuleOfTheWrite(t *testing.T) {
	cases := map[string]func(t *testing.T, o *Ops){
		"all waiting and vouched": func(t *testing.T, o *Ops) {},
		"some collected":          func(t *testing.T, o *Ops) { setup(t, o, `update landsraad_house_rewards set amount=0 where player_id=1 and template_id in ('Schematic.K', 'T6RangeFinder')`) },
		"all collected":           func(t *testing.T, o *Ops) { collectRewards(t, o) },
		"a row gone":              func(t *testing.T, o *Ops) { setup(t, o, `delete from landsraad_house_rewards where player_id=1 and template_id='SolarisCoin'`) },
		"the swatch not the editor's": func(t *testing.T, o *Ops) {
			setup(t, o, `delete from landsraad_player_one_time_house_rewards where player_id=1`)
		},
		"less waits than the tier paid": func(t *testing.T, o *Ops) { setup(t, o, `update landsraad_house_rewards set amount=1 where player_id=1 and template_id='T6RangeFinder'`) },
		"a row of an earlier term":      func(t *testing.T, o *Ops) { setup(t, o, `update landsraad_house_rewards set last_updated=50 where player_id=1 and template_id='Schematic.K'`) },
		"the character's own missions cover the tiers": func(t *testing.T, o *Ops) {
			setup(t, o, `insert into landsraad_task_progress(id, faction_id, task_id, faction_progress, guild_progress, player_progress, timestamp) values (9, 2, 175, 0, 0, 3600, 120);
				insert into landsraad_task_progress_player(progress_id, player_id) values (9, 1)`)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			o := rewardOps(t, "")
			kenolaFromZero(t, o, 6000)
			mutate(t, o)
			u := unfillOf(t, o, 175)
			if u["tiers"].(int64) != 5 {
				t.Fatalf("tiers removed: %v", u)
			}
			before := ownRows(t, o)
			m := unfill(t, o, 175)
			if previewCounts(u) != answerCounts(m) {
				t.Fatalf("the board said %v, the write did %v", previewCounts(u), answerCounts(m))
			}
			after := ownRows(t, o)
			gone := int64(0)
			for k := range before {
				if _, there := after[k]; !there {
					gone++
				}
			}
			if gone > u["take"].(int64) {
				t.Fatalf("%d rows are gone, the preview said %d would be taken back", gone, u["take"])
			}
			for k, v := range before {
				if v == 0 && after[k] != 0 {
					t.Fatalf("a collected row changed: %s", k)
				}
			}
		})
	}
}

// The preview and the write read one record: with the record unreadable nothing is vouched for, so the board says nothing is taken back and the write takes nothing.
func TestUnfillWithoutARecordTakesNothingBackAndSaysSo(t *testing.T) {
	o, path := paidOps(t, "")
	kenolaFromZero(t, o, 6000)
	flush(t, o)
	damageRecord(t, path)
	u := unfillOf(t, o, 175)
	if u["take"].(int64) != 0 || u["unsure"].(int64) == 0 {
		t.Fatalf("with the record unreadable: %v", u)
	}
	before := ownRows(t, o)
	m := unfill(t, o, 175)
	if m["rewards_taken_back"].(int64) != 0 {
		t.Fatalf("answer: %v", m)
	}
	if after := ownRows(t, o); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Fatalf("rows changed without a record: %v -> %v", before, after)
	}
}

// Raising again after an unfill writes the rewards that were taken back and pays no mission a second time (the record's second ground, #944), however often it is repeated.
func TestUnfillThenFillAgainPaysNoMissionTwice(t *testing.T) {
	o := rewardOps(t, "")
	kenolaFromZero(t, o, 6000)
	solari, scrip := wallet(t, o, 0), wallet(t, o, 1)
	xp, _, _ := track(t, o, 2)
	rowsAfterFirst := ownRows(t, o)
	for cycle := 1; cycle <= 3; cycle++ {
		unfill(t, o, 175)
		m := applyWith(t, o, doubled(), house(175, Args{"was_mine": 0.0, "mine_to": 6000.0}))
		if m["missions"].(int64) != 0 || m["reward_rows"].(int64) != 5 {
			t.Fatalf("cycle %d: answer %v", cycle, m)
		}
		if wallet(t, o, 0) != solari || wallet(t, o, 1) != scrip {
			t.Fatalf("cycle %d: wallets moved", cycle)
		}
		if x, _, _ := track(t, o, 2); x != xp {
			t.Fatalf("cycle %d: XP moved", cycle)
		}
		if got := ownRows(t, o); fmt.Sprint(got) != fmt.Sprint(rowsAfterFirst) {
			t.Fatalf("cycle %d: the rows are not what the first fill wrote:\n%v\n%v", cycle, rowsAfterFirst, got)
		}
	}
}

// A sequence of fills, single removals, unfills and the game collecting rewards, in a seeded random order over two houses: whatever the order,
//   - the board's preview of an unfill is the write (counts, and exactly the rows it says it takes back are the rows that go),
//   - a collected reward is never touched by a removal or an unfill,
//   - no wallet, XP or faction progress ever goes down, and
//   - the missions are paid once: the Solari at the end is the missions of the highest ground ever reached, however many removals and fills came between.
func TestUnfillSequencesAfterFillsKeepTheRuleAndPayOnce(t *testing.T) {
	ths := []float64{1200, 2400, 3600, 4800, 6000}
	var unfills, withTakes, withCollected, afterRaise int // what the sequences covered, so that the property is not vacuous
	for seed := int64(1); seed <= 40; seed++ {
		t.Run(fmt.Sprint("seed ", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			o := rewardOps(t, "")
			setup(t, o, `delete from landsraad_task_player_contributions where player_id=1 and task_id=175`)
			ids := []int64{175, 176}
			maxG := map[int64]float64{}
			lastSolari, lastScrip := wallet(t, o, 0), wallet(t, o, 1)
			lastXP := totalXP(t, o)
			lastFac := map[int64]float64{}
			mineOf := func(id int64) float64 { return taskByID(t, o, id)["my_contribution"].(float64) }
			for step := 0; step < 14; step++ {
				id := ids[rng.Intn(len(ids))]
				before := ownRows(t, o)
				var what string
				removal := false // a removal or an unfill: nothing collected may change
				switch op := rng.Intn(10); {
				case op < 4: // a raise to a random tier above the contribution
					to := ths[rng.Intn(len(ths))]
					cur := mineOf(id)
					if to <= cur {
						continue
					}
					what = fmt.Sprintf("raise %d %v->%v", id, cur, to)
					applyWith(t, o, doubled(), house(float64(id), Args{"was_mine": cur, "mine_to": to}))
					maxG[id] = max(maxG[id], to)
				case op < 6: // remove down to a random lower tier, or 0
					cur := mineOf(id)
					if cur <= 0 {
						continue
					}
					to := 0.0
					for _, th := range ths {
						if th < cur && rng.Intn(2) == 0 {
							to = th
						}
					}
					what = fmt.Sprintf("remove %d %v->%v", id, cur, to)
					removal = true
					applyWith(t, o, doubled(), house(float64(id), Args{"was_mine": cur, "mine_to": to, "lower": true}))
				case op < 9: // unfill both houses, as the page builds the request
					what = "unfill"
					removal = true
					hs := unfillHouses(t, o, ids...)
					if len(hs) == 0 {
						continue
					}
					previews := map[int64]map[string]any{}
					for _, h := range hs {
						previews[int64(h["task_id"].(float64))] = unfillOf(t, o, int64(h["task_id"].(float64)))
					}
					m := applyWith(t, o, doubled(), hs...)
					unfills++
					if answerCounts(m)[0] > 0 {
						withTakes++
					}
					if answerCounts(m)[1] > 0 {
						withCollected++
					}
					if len(maxG) > 0 {
						afterRaise++
					}
					var want [5]int64
					for _, p := range previews {
						c := previewCounts(p)
						for i := range want {
							want[i] += c[i]
						}
					}
					if answerCounts(m) != want {
						t.Fatalf("step %d unfill: the board said %v, the write did %v", step, want, answerCounts(m))
					}
					// exactly the rows the preview says are taken are the rows that went
					after := ownRows(t, o)
					for _, h := range hs {
						tid := int64(h["task_id"].(float64))
						name := map[int64]string{175: "DA_HouseKenola", 176: "DA_HouseVernius"}[tid]
						takes := map[string]bool{}
						for _, x := range previews[tid]["rows"].([]any) {
							r := x.(map[string]any)
							if r["outcome"] == tbTake {
								takes[name+"/"+r["template_id"].(string)] = true
							}
						}
						for k, v := range before {
							if len(k) <= len(name) || k[:len(name)+1] != name+"/" {
								continue
							}
							if _, there := after[k]; !there != takes[k] {
								t.Fatalf("step %d unfill: row %s (amount %d) gone=%v but the preview says taken=%v", step, k, v, !there, takes[k])
							}
						}
					}
				default: // the game collects what waits
					what = "collect"
					collectRewards(t, o)
				}
				after := ownRows(t, o)
				if removal {
					for k, v := range before {
						if v == 0 && after[k] != 0 {
							t.Fatalf("step %d %s: the collected row %s was changed", step, what, k)
						}
						if v == 0 {
							if _, there := after[k]; !there {
								t.Fatalf("step %d %s: the collected row %s was deleted", step, what, k)
							}
						}
					}
				}
				if s, sc := wallet(t, o, 0), wallet(t, o, 1); s < lastSolari || sc < lastScrip {
					t.Fatalf("step %d %s: a wallet went down (%d, %d from %d, %d)", step, what, s, sc, lastSolari, lastScrip)
				} else {
					lastSolari, lastScrip = s, sc
				}
				if x := totalXP(t, o); x < lastXP {
					t.Fatalf("step %d %s: XP went down", step, what)
				} else {
					lastXP = x
				}
				for _, tid := range ids {
					if v, _ := progress(t, o, 2, tid); v < lastFac[tid] {
						t.Fatalf("step %d %s: the faction's progress went down", step, what)
					} else {
						lastFac[tid] = v
					}
				}
			}
			// the missions are paid once, whatever came between: 3 steps of 2,000 up to 6,000 (a mission is 6,000 Solari)
			want := int64(0)
			for _, id := range ids {
				want += int64(ceilSteps(maxG[id], 2000)) * 6000
			}
			if got := wallet(t, o, 0); got != want {
				t.Fatalf("Solari paid %d, the missions of the highest ground reached stand for %d", got, want)
			}
		})
	}
	t.Logf("covered: %d unfills, %d that took rewards back, %d that left a collected one, %d after a raise", unfills, withTakes, withCollected, afterRaise)
	if unfills < 40 || withTakes < 20 || withCollected < 5 || afterRaise < 40 {
		t.Fatalf("the sequences covered too little: %d unfills, %d that took rewards back, %d that left a collected one, %d after a raise", unfills, withTakes, withCollected, afterRaise)
	}
}

// totalXP is the experience of every specialization track of the character.
func totalXP(t *testing.T, o *Ops) int64 {
	t.Helper()
	r, err := o.S.One(`select coalesce(sum(xp_amount),0) x from specialization_tracks where player_id=1`)
	if err != nil {
		t.Fatal(err)
	}
	return asInt(r["x"])
}
