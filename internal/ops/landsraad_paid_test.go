package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// The ground the editor paid is remembered in a file of its own (#777, landsraad_paid.go): a tier removal followed by the same raise pays only what was not paid before.

// paidOps is rewardOps with the editor's own record switched on, in a temp folder, and the save's notes going to it as they do in the editor (Server.KeepPendingEdits).
func paidOps(t *testing.T, extra string) (*Ops, string) {
	t.Helper()
	o := rewardOps(t, extra)
	path := filepath.Join(t.TempDir(), "landsraad-paid.json")
	o.Paid = NewPaidLedger(path, o.S.Path)
	o.S.SetNoteSink(o.Paid.Record)
	return o, path
}

func flush(t *testing.T, o *Ops) {
	t.Helper()
	if f := o.S.Flush(); !f.Saved {
		t.Fatalf("the edit was not written: %+v", f)
	}
}

func readPaid(t *testing.T, path string) paidFile {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f paidFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func fillKenolaDoubled(t *testing.T, o *Ops, was, to float64) map[string]any {
	return applyWith(t, o, doubled(), house(175, Args{"was_mine": was, "mine_to": to}))
}

func removeTo(t *testing.T, o *Ops, was, to float64) {
	applyWith(t, o, doubled(), house(175, Args{"was_mine": was, "mine_to": to, "lower": true}))
}

func TestPaidTheRecordIsWrittenOnlyAfterTheGamesFileIsWritten(t *testing.T) {
	o, path := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	if _, err := os.Stat(path); err == nil {
		t.Fatal("the record was written while the edit was only queued")
	}
	flush(t, o)
	f := readPaid(t, path)
	if len(f.Entries) != 1 || f.Entries[0] != (paidNote{Player: 1, Term: 7, House: "DA_HouseKenola", Faction: 2, Paid: 6000}) || f.Version != 1 || f.Save != strings.ToLower(o.S.Path) {
		t.Fatalf("the record: %+v", f)
	}
}

func TestPaidADiscardedRaiseLeavesNoRecord(t *testing.T) {
	o, path := paidOps(t, "")
	if m := fillKenolaDoubled(t, o, 1300, 6000); m["missions"].(int64) != 2 {
		t.Fatalf("answer: %v", m)
	}
	if got := taskByID(t, o, 175)["my_paid"]; got != 6000.0 {
		t.Fatalf("my_paid of the queued raise: %v", got)
	}
	if err := o.S.Discard(); err != nil {
		t.Fatal(err)
	}
	if got := taskByID(t, o, 175)["my_paid"]; got != 0.0 {
		t.Fatalf("my_paid after the queue was discarded: %v", got)
	}
	o.S.Flush()
	if _, err := os.Stat(path); err == nil {
		t.Fatal("a discarded raise left a record")
	}
	// so the same raise pays in full
	if m := fillKenolaDoubled(t, o, 1300, 6000); m["missions"].(int64) != 2 || m["missions_paid_before"].(int64) != 0 {
		t.Fatalf("a raise after a discard: %v", m)
	}
}

func TestPaidARemovalAndTheSameRaiseAfterTheFileWasWrittenPaysNothingAndSaysSo(t *testing.T) {
	o, _ := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	solari, scrip, rewards := wallet(t, o, 0), wallet(t, o, 1), houseRewards(t, o, "DA_HouseKenola")
	facBefore, _ := progress(t, o, 2, 175)
	removeTo(t, o, 6000, 0)
	m := fillKenolaDoubled(t, o, 0, 6000)
	if m["missions"].(int64) != 0 || m["reward_rows"].(int64) != 0 || m["solari"].(int64) != 0 {
		t.Fatalf("the same raise paid again: %v", m)
	}
	if m["missions_paid_before"].(int64) != 3 || m["ground_paid_before"].(float64) != 6000 || m["rewards_already_given"].(int64) != 5 {
		t.Fatalf("the answer does not say what was not paid: %v", m)
	}
	if wallet(t, o, 0) != solari || wallet(t, o, 1) != scrip || houseRewards(t, o, "DA_HouseKenola") != rewards {
		t.Fatal("wallets or reward rows changed")
	}
	// the faction's progress is not added to again either
	if v, _ := progress(t, o, 2, 175); v != facBefore {
		t.Fatalf("faction progress: %v, was %v", v, facBefore)
	}
}

func TestPaidAHigherRaiseAfterARemovalPaysOnlyTheNewTiers(t *testing.T) {
	o, path := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 2400)
	flush(t, o)
	removeTo(t, o, 2400, 0)
	flush(t, o)
	solari := wallet(t, o, 0)
	m := fillKenolaDoubled(t, o, 0, 4800)
	// step 2,000: ceil(4800/2000) - ceil(2400/2000) = 1 mission; the tiers 3,600 and 4,800 are written, 1,200 and 2,400 were paid
	if m["missions"].(int64) != 1 || m["reward_rows"].(int64) != 2 || m["missions_paid_before"].(int64) != 2 || m["ground_paid_before"].(float64) != 2400 {
		t.Fatalf("answer: %v", m)
	}
	if wallet(t, o, 0) != solari+6000 {
		t.Fatalf("Solari: %d, was %d", wallet(t, o, 0), solari)
	}
	flush(t, o)
	if f := readPaid(t, path); f.Entries[0].Paid != 4800 {
		t.Fatalf("the record grows with what was paid: %+v", f)
	}
}

func TestPaidAClickOnTheHighestTierTwiceInOneQueueDoesNotPayTwice(t *testing.T) {
	o, _ := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	// not written yet: the queued edit carries the record, so the tier box (a toggle) cannot pay twice in the meantime
	removeTo(t, o, 6000, 4800)
	m := fillKenolaDoubled(t, o, 4800, 6000)
	if m["missions"].(int64) != 0 || m["reward_rows"].(int64) != 0 || m["ground_paid_before"].(float64) != 1200 {
		t.Fatalf("a repeated click paid again: %v", m)
	}
}

func TestPaidAFileThatIsDamagedOrOfAnotherSaveIsIgnoredAndLeftAsItIs(t *testing.T) {
	entry := `"entries":[{"player":1,"term":7,"house":"DA_HouseKenola","faction":2,"paid":6000}]`
	mine := `C:\\Users\\x\\game.db`
	for name, content := range map[string]string{
		"garbage":          "{not json",
		"empty":            "",
		"another save":     `{"version":1,"save":"c:\\other\\game.db",` + entry + `}`,
		"another version":  `{"version":9,"save":"` + strings.ToLower(mine) + `",` + entry + `}`,
		"entries not list": `{"version":1,"save":"` + strings.ToLower(mine) + `","entries":7}`,
		"too large":        `{"version":1,"save":"` + strings.ToLower(mine) + `",` + entry + `,"pad":"` + strings.Repeat("x", paidMaxBytes) + `"}`,
	} {
		path := filepath.Join(t.TempDir(), "p.json")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		l := NewPaidLedger(path, `C:\Users\x\game.db`)
		if v := l.recorded(1, 7, "DA_HouseKenola"); v != 0 {
			t.Errorf("%s: the file was trusted: %v", name, v)
		}
		if after, _ := os.ReadFile(path); string(after) != content {
			t.Errorf("%s: a read changed the file", name)
		}
	}
	// the same content for this save is trusted: the cases above fail because of the file, not because of the test's shape
	path := filepath.Join(t.TempDir(), "p.json")
	os.WriteFile(path, []byte(`{"version":1,"save":"c:\\users\\x\\game.db",`+entry+`}`), 0o600)
	if v := NewPaidLedger(path, `C:\Users\x\game.db`).recorded(1, 7, "DA_HouseKenola"); v != 6000 {
		t.Fatalf("a good file for this save is not read: %v", v)
	}
}

func TestPaidAUnusableFileMeansAnotherRaiseAfterARemovalPaysAgainAsBefore(t *testing.T) {
	o, path := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	flush(t, o) // the write replaces the unusable file with one holding what this write paid
	if f := readPaid(t, path); len(f.Entries) != 1 || f.Entries[0].Paid != 6000 {
		t.Fatalf("the unusable file was not replaced by a whole one: %+v", f)
	}
	// now damage it after the write: the record is lost, nothing is trusted, and the raise pays again
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	removeTo(t, o, 6000, 0)
	solari := wallet(t, o, 0)
	m := fillKenolaDoubled(t, o, 0, 6000)
	if m["missions"].(int64) != 3 || m["missions_paid_before"].(int64) != 0 || wallet(t, o, 0) != solari+18000 {
		t.Fatalf("with no usable record the raise pays as it did before: %v", m)
	}
}

// A save put back from a backup made before the raise has no reward row of this term: the record is not trusted there, or a mission the restored save never received would be withheld.
func TestPaidTheRecordIsNotTrustedWhereTheSaveShowsNoRewardRowOfThisTerm(t *testing.T) {
	o, path := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	if f := readPaid(t, path); len(f.Entries) != 1 {
		t.Fatalf("record: %+v", f)
	}
	// the save as a backup of before the raise holds it: the character's contribution and reward rows of the house are as they were before
	setup(t, o, `delete from landsraad_house_rewards where player_id=1 and house_name='DA_HouseKenola'`)
	setup(t, o, `update landsraad_task_player_contributions set amount=0 where player_id=1 and task_id=175`)
	flush(t, o)
	m := fillKenolaDoubled(t, o, 0, 6000)
	if m["missions"].(int64) != 3 || m["missions_paid_before"].(int64) != 0 {
		t.Fatalf("a record of a raise the save no longer shows was trusted: %v", m)
	}
}

func TestPaidAFactionChangeKeepsTheRecord(t *testing.T) {
	o, _ := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	setup(t, o, `update player_faction set faction_id=1 where actor_id=1`)
	flush(t, o)
	m := applyWith(t, o, doubled(), house(175, Args{"was_mine": 0.0, "mine_to": 6000.0}))
	if m["missions"].(int64) != 0 || m["reward_rows"].(int64) != 0 {
		t.Fatalf("a raise under the other faction paid again: %v", m)
	}
}

func TestPaidTheBoardShowsTheGroundPaidForThePageToCount(t *testing.T) {
	o, _ := paidOps(t, "")
	if got := taskByID(t, o, 175)["my_paid"]; got != 0.0 {
		t.Fatalf("my_paid before any raise: %v", got)
	}
	fillKenolaDoubled(t, o, 1300, 4800)
	if got := taskByID(t, o, 175)["my_paid"]; got != 4800.0 {
		t.Fatalf("my_paid of a queued raise: %v", got)
	}
	flush(t, o)
	removeTo(t, o, 4800, 1200)
	if got := taskByID(t, o, 175)["my_paid"]; got != 4800.0 {
		t.Fatalf("my_paid after a removal and the write: %v", got)
	}
	if got := taskByID(t, o, 176)["my_paid"]; got != 0.0 {
		t.Fatalf("another house: %v", got)
	}
}

func TestPaidTheRecordKeepsTheHighestAndDropsEarlierTerms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	l := NewPaidLedger(path, `C:\X\game.db`)
	note := func(term, faction int64, house string, paid float64) save.Note {
		b, _ := json.Marshal(paidNote{Player: 1, Term: term, House: house, Faction: faction, Paid: paid})
		return save.Note{Kind: paidNoteKind, Data: b}
	}
	if err := l.Record([]save.Note{note(6, 2, "A", 3000), {Kind: "other", Data: []byte(`{}`)}, note(6, 2, "A", 1000), note(6, 1, "A", 500)}); err != nil {
		t.Fatal(err)
	}
	if got := l.recorded(1, 6, "A"); got != 3000 {
		t.Fatalf("the highest under any faction: %v", got)
	}
	if err := l.Record([]save.Note{note(7, 2, "B", 1200)}); err != nil {
		t.Fatal(err)
	}
	if l.recorded(1, 6, "A") != 0 || l.recorded(1, 7, "B") != 1200 {
		t.Fatalf("an earlier term's record is dropped: %v %v", l.recorded(1, 6, "A"), l.recorded(1, 7, "B"))
	}
	if l.recorded(2, 7, "B") != 0 {
		t.Fatal("another character's record counted")
	}
	// nothing to record: no file write
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := l.Record([]save.Note{{Kind: "other", Data: []byte(`{}`)}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("a note of another kind wrote the file")
	}
	// a ledger with no path, or none at all, records and reads nothing
	var none *PaidLedger
	if none.recorded(1, 7, "B") != 0 || none.Record([]save.Note{note(7, 2, "B", 1)}) != nil || NewPaidLedger("", "x").recorded(1, 7, "B") != 0 {
		t.Fatal("a ledger with no file recorded something")
	}
}
