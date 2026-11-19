package ops

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// The record trusted only as far as the save shows it (review of #798): a save put back, or a reward row the game left, must not make the editor withhold what it never paid.

// putBackBeforeTheRaise sets the save to what it was before a raise that paid the house: the character's rows of the house are gone and the contribution is what it was.
func putBackBeforeTheRaise(t *testing.T, o *Ops, was float64) {
	t.Helper()
	setup(t, o, `delete from landsraad_house_rewards where player_id=1 and house_name='DA_HouseKenola'`)
	setup(t, o, `delete from landsraad_player_one_time_house_rewards where player_id=1 and house_name='DA_HouseKenola'`)
	setup(t, o, `update landsraad_task_player_contributions set amount=? where player_id=1 and task_id=175`, was)
	flush(t, o)
}

func i64(t *testing.T, m map[string]any, k string) int64 {
	t.Helper()
	v, ok := m[k].(int64)
	if !ok {
		t.Fatalf("%s is not a count in %v", k, m)
	}
	return v
}

// Repro A: after a put-back the first raise is capped (nothing of the record stands), pays, and its note REPLACES the record. Merging it into the old high-water mark would leave 6000
// on record, and the next raise would be trusted once a row stands again.
func TestPaidAPutBackSaveRaisedInTwoStepsIsPaidInFull(t *testing.T) {
	o, path := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	putBackBeforeTheRaise(t, o, 0)
	if got := taskByID(t, o, 175)["my_paid"]; got != 0.0 {
		t.Fatalf("the board trusts a record that nothing in the save stands for: %v", got)
	}
	m := fillKenolaDoubled(t, o, 0, 1200)
	if i64(t, m, "reward_rows") != 1 {
		t.Fatalf("tier 1 after the put-back is paid: %v", m)
	}
	flush(t, o)
	if f := readPaid(t, path); len(f.Entries) != 1 || f.Entries[0].Paid != 1200 {
		t.Fatalf("the record was merged into the old high-water mark instead of replaced: %+v", f)
	}
	m = fillKenolaDoubled(t, o, 1200, 6000)
	if i64(t, m, "reward_rows") != 4 || i64(t, m, "missions") == 0 {
		t.Fatalf("the rest of the raise is paid: %v", m)
	}
	flush(t, o)
	if f := readPaid(t, path); f.Entries[0].Paid != 6000 {
		t.Fatalf("the record follows what was paid: %+v", f)
	}
}

// Repro B: the game left a reward row of this term for tier 1, the editor raised the house, and the editor's own backup from before the write was put back. A row stands, so "any row"
// would trust the whole record; only what the save shows counts: the highest tier whose item has a row.
func TestPaidAGameRowDoesNotMakeAPutBackRecordTrusted(t *testing.T) {
	o, _ := paidOps(t, `insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', 10000, 'SolarisCoin', 120);`)
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	// back to before the raise: the game's own tier 1 row stays, the editor's rows are gone
	setup(t, o, `delete from landsraad_house_rewards where player_id=1 and house_name='DA_HouseKenola' and template_id<>'SolarisCoin'`)
	setup(t, o, `delete from landsraad_player_one_time_house_rewards where player_id=1 and house_name='DA_HouseKenola'`)
	setup(t, o, `update landsraad_task_player_contributions set amount=1300 where player_id=1 and task_id=175`)
	flush(t, o)
	if got := taskByID(t, o, 175)["my_paid"]; got != 1200.0 {
		t.Fatalf("the board counts the record only as far as the save shows it: %v", got)
	}
	m := fillKenolaDoubled(t, o, 1300, 6000)
	if i64(t, m, "reward_rows") != 4 || i64(t, m, "missions") == 0 {
		t.Fatalf("the raise is paid above what the save shows: %v", m)
	}
}

// What the cap reads in a write (SQL) and on the board (rows it already holds) is the same, case by case.
func TestPaidTheCapIsTheSameInTheWriteAndOnTheBoard(t *testing.T) {
	o, _ := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	cases := []struct {
		name, del string
		want      float64
	}{
		{"every row stands", ``, 6000},
		{"the swatch alone", `delete from landsraad_house_rewards where player_id=1 and house_name='DA_HouseKenola' and template_id<>'Kenola_Placeables_Swatch'`, 6000},
		{"the swatch and the tiers below it are gone", `delete from landsraad_house_rewards where player_id=1 and house_name='DA_HouseKenola'; insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', 1, 'Schematic.L', 150)`, 4800},
		{"a row of an earlier term only", `delete from landsraad_house_rewards where player_id=1 and house_name='DA_HouseKenola'; insert into landsraad_house_rewards(player_id, house_name, amount, template_id, last_updated) values (1, 'DA_HouseKenola', 1, 'Schematic.L', 50)`, 0},
		{"no row", `delete from landsraad_house_rewards where player_id=1 and house_name='DA_HouseKenola'`, 0},
	}
	for _, c := range cases {
		if c.del != "" {
			for _, q := range strings.Split(c.del, ";") {
				setup(t, o, q)
			}
		}
		got, err := standingTierSQL(o.S.One, 1, 175, "DA_HouseKenola", 100)
		if err != nil {
			t.Fatal(err)
		}
		if got != c.want {
			t.Fatalf("%s: the write's cap is %v, want %v", c.name, got, c.want)
		}
		board := taskByID(t, o, 175)["my_paid"]
		if board != c.want {
			t.Fatalf("%s: the board says %v, the write says %v", c.name, board, c.want)
		}
	}
}

// An editor restore does not empty the record (review of #798, N1): the restored file shows by its own rows how far the record is still true. Undoing a restore brings back the file the
// raise left, so the record of that raise must still be there; restoring the backup from before the raise cuts the record to nothing by the cap, and a raise then pays in full.
func TestPaidUndoingAnEditorRestoreKeepsTheRecordAndRestoringBeforeTheRaiseDoesNot(t *testing.T) {
	o, path := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	bs := o.S.Backups()
	if len(bs) == 0 {
		t.Fatal("no backup")
	}
	if err := o.S.Restore(bs[0]["name"].(string)); err != nil { // the file as it was before the raise
		t.Fatal(err)
	}
	if got := taskByID(t, o, 175)["my_paid"]; got != 0.0 {
		t.Fatalf("the board trusts a record of a raise the restored file does not hold: %v", got)
	}
	bs = o.S.Backups() // the newest is the one the restore took: the file the raise left
	if err := o.S.Restore(bs[0]["name"].(string)); err != nil {
		t.Fatal(err)
	}
	if f := readPaid(t, path); len(f.Entries) != 1 || f.Entries[0].Paid != 6000 {
		t.Fatalf("the record was lost by a restore: %+v", f)
	}
	if got := taskByID(t, o, 175)["my_paid"]; got != 6000.0 {
		t.Fatalf("the file is the one the raise left, so its record counts: %v", got)
	}
	removeTo(t, o, 6000, 1200)
	flush(t, o)
	m := fillKenolaDoubled(t, o, 1200, 6000)
	if i64(t, m, "missions") != 0 || i64(t, m, "reward_rows") != 4 || i64(t, m, "solari") != 0 { // the four rewards the removal took back are written again, and no mission is paid
		t.Fatalf("a raise on the file the undo brought back paid again: %v", m)
	}
}

// House A paid on Monday, house B on Tuesday; Tuesday's backup from before the write is put back. A's record stays (its rows stand), B's is cut (its rows are gone) and B is paid in full.
func TestPaidRestoringTheBackupBeforeASecondHousePaidKeepsTheFirstHousesRecord(t *testing.T) {
	o, path := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000) // Monday
	flush(t, o)
	bk := filepath.Join(filepath.Dir(o.S.Path), "tabr-tau-backups")
	if err := os.MkdirAll(bk, 0o700); err != nil {
		t.Fatal(err)
	}
	pre, err := os.ReadFile(o.S.Path) // the file as Tuesday's write finds it
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bk, "tuesday-before.db"), pre, 0o600); err != nil {
		t.Fatal(err)
	}
	applyWith(t, o, doubled(), house(176, Args{"was_mine": 0.0, "mine_to": 4800.0})) // Tuesday
	flush(t, o)
	if f := readPaid(t, path); len(f.Entries) != 2 {
		t.Fatalf("both houses are on record: %+v", f)
	}
	if err := o.S.Restore("tuesday-before.db"); err != nil {
		t.Fatal(err)
	}
	if got := taskByID(t, o, 175)["my_paid"]; got != 6000.0 {
		t.Fatalf("Monday's house is on the restored file and stays on record: %v", got)
	}
	if got := taskByID(t, o, 176)["my_paid"]; got != 0.0 {
		t.Fatalf("Tuesday's house is not on the restored file: %v", got)
	}
	removeTo(t, o, 6000, 1200)
	flush(t, o)
	if m := fillKenolaDoubled(t, o, 1200, 6000); i64(t, m, "missions") != 0 || i64(t, m, "reward_rows") != 4 {
		t.Fatalf("Monday's house paid again after the restore: %v", m)
	}
	m := applyWith(t, o, doubled(), house(176, Args{"was_mine": 0.0, "mine_to": 4800.0}))
	if i64(t, m, "reward_rows") != 4 {
		t.Fatalf("Tuesday's house is paid in full on the restored file: %v", m)
	}
}

func TestPaidANoteThatReplacesDropsTheOtherFactionsEntryToo(t *testing.T) {
	es := []paidNote{{Player: 1, Term: 7, House: "A", Faction: 1, Paid: 6000}, {Player: 1, Term: 7, House: "A", Faction: 2, Paid: 3000}, {Player: 1, Term: 7, House: "B", Faction: 2, Paid: 900}}
	got := mergePaid(es, []paidNote{{Player: 1, Term: 7, House: "A", Faction: 2, Paid: 1200, Replace: true}})
	if highestPaid(got, 1, 7, "A") != 1200 || highestPaid(got, 1, 7, "B") != 900 || len(got) != 2 {
		t.Fatalf("replace: %+v", got)
	}
	if got := mergePaid(es, []paidNote{{Player: 1, Term: 7, House: "A", Faction: 2, Paid: 1200}}); highestPaid(got, 1, 7, "A") != 6000 {
		t.Fatalf("a plain note merges: %+v", got)
	}
}

// Record keeps the entries of the term its notes belong to: a term that went backwards (a backup of an earlier term was put back) still records, and an absurd term in the file does
// not stop it.
func TestPaidATermThatGoesBackwardsOrIsAbsurdDoesNotStopRecordingOrTakeOtherTermsAway(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	l := NewPaidLedger(path, `C:\X\game.db`)
	note := func(term int64, house string, paid float64) save.Note {
		b, _ := json.Marshal(paidNote{Player: 1, Term: term, House: house, Faction: 2, Paid: paid})
		return save.Note{Kind: paidNoteKind, Data: b}
	}
	if err := l.Record([]save.Note{note(9, "A", 3000), note(9, "B", 2000)}); err != nil {
		t.Fatal(err)
	}
	if err := l.Record([]save.Note{note(7, "A", 1200)}); err != nil {
		t.Fatal(err)
	}
	if l.recorded(1, 7, "A") != 1200 || l.recorded(1, 9, "A") != 3000 || l.recorded(1, 9, "B") != 2000 {
		t.Fatalf("a note of an earlier term is lost, or the later term's record went with it: %v %v %v", l.recorded(1, 7, "A"), l.recorded(1, 9, "A"), l.recorded(1, 9, "B"))
	}
	huge := `{"version":1,"save":"c:\\x\\game.db","entries":[{"player":1,"term":9000000000000000000,"house":"Z","faction":2,"paid":1200},{"player":1,"term":7,"house":"A","faction":2,"paid":1200}]}`
	if err := os.WriteFile(path, []byte(huge), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := l.Record([]save.Note{note(8, "C", 600)}); err != nil {
		t.Fatal(err)
	}
	if l.recorded(1, 8, "C") != 600 || l.recorded(1, 7, "A") != 1200 {
		t.Fatalf("an absurd term in the file stopped the recording: %+v", readPaid(t, path))
	}
}

func TestPaidAnImplausibleEntryIsLeftOutWhenTheFileIsRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	l := NewPaidLedger(path, `C:\X\game.db`)
	long := strings.Repeat("h", paidMaxHouseLen+1)
	content := `{"version":1,"save":"c:\\x\\game.db","entries":[` +
		`{"player":1,"term":7,"house":"Good","faction":2,"paid":1200},` +
		`{"player":1,"term":0,"house":"NoTerm","faction":2,"paid":1200},` +
		`{"player":0,"term":7,"house":"NoPlayer","faction":2,"paid":1200},` +
		`{"player":1,"term":7,"house":"","faction":2,"paid":1200},` +
		`{"player":1,"term":7,"house":"` + long + `","faction":2,"paid":1200},` +
		`{"player":1,"term":7,"house":"Negative","faction":2,"paid":-5},` +
		`{"player":1,"term":7,"house":"Beyond","faction":2,"paid":1e30},` +
		`{"player":1,"term":7,"house":"Zero","faction":2,"paid":0}]}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := l.snapshot(); len(got) != 1 || got[0].House != "Good" {
		t.Fatalf("only the plausible entry is read: %+v", got)
	}
}

// A file that could not be read this time (another program holds it) is tried again; when it still cannot be read it is left as it is and Record says so, instead of building a new
// file from the notes alone and losing the other houses.
func TestPaidAFileThatCannotBeReadIsNeverRebuiltFromTheNotesAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	l := NewPaidLedger(path, `C:\X\game.db`)
	note := func(house string, paid float64) save.Note {
		b, _ := json.Marshal(paidNote{Player: 1, Term: 7, House: house, Faction: 2, Paid: paid})
		return save.Note{Kind: paidNoteKind, Data: b}
	}
	if err := l.Record([]save.Note{note("A", 3000)}); err != nil {
		t.Fatal(err)
	}
	oldOpen, oldPause := paidOpen, paidReadPause
	paidReadPause = 0
	defer func() { paidOpen, paidReadPause = oldOpen, oldPause }()
	fails := 1
	paidOpen = func(p string) (*os.File, error) {
		if fails > 0 {
			fails--
			return nil, &os.PathError{Op: "open", Path: p, Err: errors.New("sharing violation")}
		}
		return os.Open(p)
	}
	if err := l.Record([]save.Note{note("B", 1200)}); err != nil {
		t.Fatalf("a read that works on a later try is a success: %v", err)
	}
	paidOpen = oldOpen
	if l.recorded(1, 7, "A") != 3000 || l.recorded(1, 7, "B") != 1200 {
		t.Fatalf("both houses are recorded: %+v", readPaid(t, path))
	}
	before, _ := os.ReadFile(path)
	paidOpen = func(p string) (*os.File, error) {
		return nil, &os.PathError{Op: "open", Path: p, Err: errors.New("sharing violation")}
	}
	if err := l.Record([]save.Note{note("C", 600)}); err == nil {
		t.Fatal("a file that stays unreadable is an error, so the save keeps the note")
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatal("the file was rewritten from the notes alone")
	}
}

// The record is read once for a request and once for a board read, not once per house.
func TestPaidTheRecordIsReadOncePerRequestAndOncePerBoardRead(t *testing.T) {
	o, _ := paidOps(t, "")
	before := paidReads.Load()
	applyWith(t, o, doubled(), house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0}), house(176, Args{"was_mine": 0.0, "mine_to": 1200.0}))
	if n := paidReads.Load() - before; n != 1 {
		t.Fatalf("a request that raises two houses read the record %d times", n)
	}
	before = paidReads.Load()
	if _, err := o.Landsraad(); err != nil {
		t.Fatal(err)
	}
	if n := paidReads.Load() - before; n != 1 {
		t.Fatalf("a board read read the record %d times", n)
	}
}

func unreadable(failures *int) func(string) (*os.File, error) {
	return func(p string) (*os.File, error) {
		if failures != nil {
			if *failures <= 0 {
				return os.Open(p)
			}
			*failures--
		}
		return nil, &os.PathError{Op: "open", Path: p, Err: errors.New("sharing violation")}
	}
}

// N2: a record that cannot be read when a raise runs is not "nothing paid": the read is tried again, and then the raise is refused with nothing written. A read that works on the second
// try goes on. The board, which only shows, reads it as nothing paid.
func TestPaidARecordThatCannotBeReadRefusesTheRaiseAndNothingIsWritten(t *testing.T) {
	o, _ := paidOps(t, "")
	oldOpen, oldPause := paidOpen, paidReadPause
	paidReadPause = 0
	defer func() { paidOpen, paidReadPause = oldOpen, oldPause }()
	paidOpen = unreadable(nil)
	_, err := o.LandsraadApplyWith(doubled(), applyArgs(house(175, Args{"was_mine": 1300.0, "mine_to": 6000.0})))
	if !errors.Is(err, apierr.ErrWriteFailed) {
		t.Fatalf("a record that cannot be read refuses the raise: %v", err)
	}
	if strings.Contains(err.Error(), "sharing violation") || strings.Contains(err.Error(), os.TempDir()) {
		t.Fatalf("the answer names the file or the system's words: %v", err)
	}
	if got := rows(t, o, `select amount from landsraad_task_player_contributions where player_id=1 and task_id=175`); !strings.Contains(got, "1300") {
		t.Fatalf("something was written: %s", got)
	}
	if o.S.Dirty() {
		t.Fatal("an edit is queued")
	}
	if got := taskByID(t, o, 175)["my_paid"]; got != 0.0 {
		t.Fatalf("the board reads an unreadable record as nothing paid: %v", got)
	}
	one := 1
	paidOpen = unreadable(&one)
	m := fillKenolaDoubled(t, o, 1300, 6000)
	if i64(t, m, "missions") != 2 {
		t.Fatalf("a read that works on the second try goes on: %v", m)
	}
}

// N4: a note that comes again (a journal that outlived the write) merges only; what it asks to replace is not replaced, or the first part of a batch would lower what the whole batch
// recorded.
func TestPaidANoteThatComesAgainNeverLowersWhatIsRecorded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "p.json")
	l := NewPaidLedger(path, `C:\X\game.db`)
	note := func(paid float64, replace, again bool) save.Note {
		b, _ := json.Marshal(paidNote{Player: 1, Term: 7, House: "A", Faction: 2, Paid: paid, Replace: replace})
		return save.Note{Kind: paidNoteKind, Data: b, Again: again}
	}
	if err := l.Record([]save.Note{note(1200, true, false), note(6000, false, false)}); err != nil {
		t.Fatal(err)
	}
	if err := l.Record([]save.Note{note(1200, true, true)}); err != nil { // the journal holds only the first of the two
		t.Fatal(err)
	}
	if got := l.recorded(1, 7, "A"); got != 6000 {
		t.Fatalf("a prefix of the batch lowered the record: %v", got)
	}
	if err := l.Record([]save.Note{note(900, true, false)}); err != nil {
		t.Fatal(err)
	}
	if got := l.recorded(1, 7, "A"); got != 900 {
		t.Fatalf("a first delivery still replaces: %v", got)
	}
}

// N5: the boundary of "this term" is real: a write with no usable play clock stamps its rows with exactly the start of the term, and those rows stand.
func TestPaidARowWrittenExactlyAtTheStartOfTheTermStandsAndOneBeforeItDoesNot(t *testing.T) {
	o, _ := paidOps(t, "")
	fillKenolaDoubled(t, o, 1300, 6000)
	flush(t, o)
	check := func(stamp int, want float64) {
		t.Helper()
		setup(t, o, `update landsraad_house_rewards set last_updated=? where player_id=1 and house_name='DA_HouseKenola'`, stamp)
		got, err := standingTierSQL(o.S.One, 1, 175, "DA_HouseKenola", 100)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("rows stamped %d: the write's cap is %v, want %v", stamp, got, want)
		}
		if board := taskByID(t, o, 175)["my_paid"]; board != want {
			t.Fatalf("rows stamped %d: the board says %v, want %v", stamp, board, want)
		}
	}
	check(100, 6000)
	check(99, 0)
	// the tiers alone, with no swatch row: the first of the two queries has its own boundary
	check(100, 6000)
	setup(t, o, `delete from landsraad_house_rewards where player_id=1 and house_name='DA_HouseKenola' and template_id='Kenola_Placeables_Swatch'`)
	check(100, 4800)
	check(99, 0)
}

// The cap holds the record only when it is above the rows that stand: equal to them it is trusted whole, and a raise that pays does not replace it.
func TestPaidARecordEqualToWhatStandsIsTrustedAndNotMarkedUntrusted(t *testing.T) {
	rec := []paidNote{{Player: 1, Term: 7, House: "A", Faction: 2, Paid: 6000}}
	if pg := trustedPaid(rec, nil, 1, 7, "A", 6000); pg.ground != 6000 || pg.untrusted {
		t.Fatalf("equal: %v", pg)
	}
	if pg := trustedPaid(rec, nil, 1, 7, "A", 4800); pg.ground != 4800 || !pg.untrusted {
		t.Fatalf("above what stands: %v", pg)
	}
	if pg := trustedPaid(nil, nil, 1, 7, "A", 4800); pg.ground != 0 || pg.untrusted {
		t.Fatalf("no record: %v", pg)
	}
}

// N6: a read that fails is tried again once after a short pause, because the save's lock is held while it waits.
func TestPaidAFailingReadWaitsOnlyAShortWhile(t *testing.T) {
	if wait := time.Duration(paidReadTries-1) * paidReadPause; paidReadTries < 2 || wait > 60*time.Millisecond {
		t.Fatalf("%d tries, %v pause: %v of waiting under the save's lock", paidReadTries, paidReadPause, wait)
	}
}
