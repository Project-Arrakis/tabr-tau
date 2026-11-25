package layoutroll

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Vetting (#1222). Every test works in temp folders: no real Game.ini, settings folder, save or process list is touched.

func (e *env) vettingFile() string { return filepath.Join(filepath.Dir(e.state), VettingFileName) }

func (e *env) readFile(p string) string {
	e.t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		e.t.Fatal(err)
	}
	return string(b)
}

func (e *env) exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func (e *env) saveHash() [32]byte {
	e.t.Helper()
	p := e.save
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		p = filepath.Join(p, "game.db")
	}
	return sha256.Sum256([]byte(e.readFile(p)))
}

// settingsFiles is every file in the settings folder (where the vetted list, the results and the state file live), by name.
func (e *env) settingsFiles() map[string]string {
	m := map[string]string{}
	ents, _ := os.ReadDir(filepath.Dir(e.state))
	for _, x := range ents {
		b, _ := os.ReadFile(filepath.Join(filepath.Dir(e.state), x.Name()))
		m[x.Name()] = string(b)
	}
	return m
}

// ---- vet N: set the layout in Game.ini ----

func TestVetSetChangesOnlyTheNumberAfterABackupAndNeverTouchesTheSave(t *testing.T) {
	old := iniWith("11")
	e := newEnv(t, old)
	saveBefore := e.saveHash()
	r, err := VetSet(e.opts(), 8)
	if err != nil || !r.Wrote || r.Old != 11 || r.New != 8 {
		t.Fatalf("%+v %v", r, err)
	}
	if got := e.ini(); got != iniWith("8") {
		t.Fatalf("Game.ini is not the old text with the number changed:\n%s", got)
	}
	bk := e.backups()
	if len(bk) != 1 || e.readFile(filepath.Join(e.dir, bk[0])) != old {
		t.Fatalf("backups %v", bk)
	}
	// plain words: what to do next
	mustContain(t, lines(r), "Changed Game.ini from layout 11 to layout 8", "Sandworm Threat", "shield on", "rock island", "inside a lab", "0%", "trainer's rock matches the ground", "vet 8 pass", "vet 8 fail", "did not open or change the save")
	// no state file, no vetted file, no results file; the save is the same bytes
	if len(e.settingsFiles()) != 0 {
		t.Fatalf("vet wrote in the settings folder: %v", e.settingsFiles())
	}
	if e.saveHash() != saveBefore {
		t.Fatal("the save changed")
	}
}

func TestVetSetRefusesWhileTheGameRunsAndWritesNothing(t *testing.T) {
	old := iniWith("11")
	e := newEnv(t, old)
	e.running = true
	r, err := VetSet(e.opts(), 8)
	if err == nil || r.Wrote {
		t.Fatalf("%+v %v", r, err)
	}
	mustContain(t, err.Error(), "not changed", "Dune.exe is running", "Close the game")
	if e.ini() != old || len(e.backups()) != 0 || len(e.settingsFiles()) != 0 {
		t.Fatal("a refused run wrote something")
	}
}

func TestVetSetStopsWhenTheGameStartsDuringTheRun(t *testing.T) {
	old := iniWith("11")
	e := newEnv(t, old)
	o := e.opts()
	calls := 0
	o.GameRunning = func() (bool, string) { calls++; return calls >= 2, "Dune.exe is running" }
	_, err := VetSet(o, 8)
	if err == nil {
		t.Fatal("not refused")
	}
	mustContain(t, err.Error(), "not changed", "it started while this ran")
	if e.ini() != old {
		t.Fatal("Game.ini was written")
	}
}

func TestVetSetDoesNotWriteWhenGameIniChangedDuringTheRun(t *testing.T) {
	old := iniWith("11")
	e := newEnv(t, old)
	o := e.opts()
	calls := 0
	o.GameRunning = func() (bool, string) {
		calls++
		if calls == 2 { // between the backup and the write another program changes the file
			e.write(iniWith("4"))
		}
		return false, ""
	}
	_, err := VetSet(o, 8)
	if err == nil || !strings.Contains(err.Error(), "changed while this ran") {
		t.Fatalf("%v", err)
	}
	if e.ini() != iniWith("4") {
		t.Fatal("the other program's change was overwritten")
	}
}

func TestVetSetRefusesWhileTheSaveIsHeldAndTakesTheSaveLockOtherwise(t *testing.T) {
	old := iniWith("11")
	e := newEnv(t, old)
	took, gave, paths := countLock(t)
	if _, err := VetSet(e.opts(), 8); err != nil {
		t.Fatal(err)
	}
	if *took != 1 || *gave != 1 || len(*paths) != 1 || filepath.Base((*paths)[0]) != "game.db" {
		t.Fatalf("lock taken %d, given back %d, on %v", *took, *gave, *paths)
	}

	e2 := newEnv(t, old)
	swapVar(t, &lockSave, func(string) (func(), error) { return nil, errors.New("held by another window") })
	r, err := VetSet(e2.opts(), 8)
	if err == nil || r.Wrote {
		t.Fatalf("%+v %v", r, err)
	}
	mustContain(t, err.Error(), "not changed", "the save is in use", "did not wait")
	if e2.ini() != old || len(e2.backups()) != 0 || len(e2.settingsFiles()) != 0 {
		t.Fatal("a refused run wrote something")
	}
}

func TestVetSetWithNoSaveKnownSaysSoAndStillWorks(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	took, _, _ := countLock(t)
	o := e.opts()
	o.SavePath, o.SaveProblem = "", "no save (game.db) was found on this machine"
	r, err := VetSet(o, 3)
	if err != nil || !r.Wrote {
		t.Fatalf("%+v %v", r, err)
	}
	mustContain(t, lines(r), "no save (game.db) was found", "was not checked whether the editor has a save open")
	if *took != 0 {
		t.Fatal("a lock was taken on no save")
	}
}

func TestVetSetRefusesBadNumbersAndUnreadableGameIni(t *testing.T) {
	old := iniWith("11")
	e := newEnv(t, old)
	for _, n := range []int{-1, 12, 15, 99} {
		if _, err := VetSet(e.opts(), n); err == nil || !strings.Contains(err.Error(), "from 0 to 11") {
			t.Errorf("layout %d: %v", n, err)
		}
	}
	// a layout the Game.ini does not list
	short := baseIni + Key + `=(("/Game/Maps/DD",(WorldLayoutsSettingsList=((Content="/Game/L00"),(Content="/Game/L01"),(Content="/Game/L02")),WorldLayoutIndexInSingleplayer=1)))` + "\r\n"
	es := newEnv(t, short)
	if _, err := VetSet(es.opts(), 5); err == nil || !strings.Contains(err.Error(), "lists only 3 layouts") {
		t.Fatalf("%v", err)
	}
	if es.ini() != short {
		t.Fatal("changed")
	}
	// Game.ini that is not 8-bit text, or a line the roll does not edit
	eu := newEnv(t, "")
	if err := os.WriteFile(filepath.Join(eu.dir, FileName), []byte("\xff\xfe[\x00A\x00]\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VetSet(eu.opts(), 3); err == nil || !strings.Contains(err.Error(), "Game.ini cannot be read") {
		t.Fatalf("%v", err)
	}
	dup := iniWith("11") + "[" + Section + "]\r\n" + lineWith("2") + "\r\n"
	ed := newEnv(t, dup)
	if _, err := VetSet(ed.opts(), 3); err == nil || !strings.Contains(err.Error(), "more than once") {
		t.Fatalf("%v", err)
	}
	if ed.ini() != dup || len(ed.backups()) != 0 {
		t.Fatal("changed")
	}
	// a config folder that does not exist
	e2 := newEnv(t, old)
	o := e2.opts()
	o.ConfigDir = filepath.Join(e2.dir, "nope")
	if _, err := VetSet(o, 3); err == nil || !strings.Contains(err.Error(), "not changed") {
		t.Fatalf("%v", err)
	}
}

func TestVetSetWhenGameIniAlreadySetsItWritesNothing(t *testing.T) {
	old := iniWith("8")
	e := newEnv(t, old)
	r, err := VetSet(e.opts(), 8)
	if err != nil || r.Wrote {
		t.Fatalf("%+v %v", r, err)
	}
	mustContain(t, lines(r), "already sets layout 8", "nothing was written", "Sandworm Threat")
	if e.ini() != old || len(e.backups()) != 0 {
		t.Fatal("written")
	}
}

func TestVetSetAddsTheLineWhenGameIniHasNone(t *testing.T) {
	e := newEnv(t, "[A]\r\nx=1\r\n")
	r, err := VetSet(e.opts(), 6)
	if err != nil || !r.Wrote {
		t.Fatalf("%+v %v", r, err)
	}
	if got, present, err := CurrentIndex(e.ini()); err != nil || !present || got != 6 {
		t.Fatalf("%d %v %v", got, present, err)
	}
	if !strings.HasPrefix(e.ini(), "[A]\r\nx=1\r\n") || len(e.backups()) != 1 {
		t.Fatalf("%q", e.ini())
	}
}

func TestVetSetKeepsOnlyTenBackups(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	for i := 0; i < 13; i++ {
		e.now = e.now.Add(1000 * 1000 * 1000)
		if _, err := VetSet(e.opts(), i%2); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(e.backups()); n != KeepBackups {
		t.Fatalf("%d backups", n)
	}
}

// ---- vet N pass|fail: the record and the vetted list ----

func TestVetPassWithNoFileMakesTheListFromTheBuiltInOnePlusN(t *testing.T) {
	old := iniWith("11")
	e := newEnv(t, old)
	r, err := VetRecord(e.opts(), 8, true, "threat 0% on rock and in the lab")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, lines(r), "did not exist", "built-in list 9, 11", "8, 9, 11", "layout 8 pass threat 0% on rock and in the lab", "Game.ini and the save were not touched")
	list, err := ParseVetted(e.readFile(e.vettedFile()))
	if err != nil || joinInts(list) != "8, 9, 11" {
		t.Fatalf("%v %v", list, err)
	}
	if got := e.readFile(e.vettingFile()); got != "2026-11-24 05:00 layout 8 pass threat 0% on rock and in the lab\r\n" {
		t.Fatalf("%q", got)
	}
	if e.ini() != old || len(e.backups()) != 0 || e.exists(e.state) {
		t.Fatal("Game.ini or the state file was touched")
	}
}

func TestVetPassKeepsTheCommentsAndTheSpacingOfTheFile(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.writeVetted("# checked in the game\r\n9, 11   # the good ones\r\n# 5   (not checked in a lab yet)\r\n")
	if _, err := VetRecord(e.opts(), 10, true, ""); err != nil {
		t.Fatal(err)
	}
	want := "# checked in the game\r\n9, 11, 10   # the good ones\r\n# 5   (not checked in a lab yet)\r\n"
	if got := e.readFile(e.vettedFile()); got != want {
		t.Fatalf("%q", got)
	}
	// a layout that is in a comment is not in the list, and a pass adds it
	if _, err := VetRecord(e.opts(), 5, true, ""); err != nil {
		t.Fatal(err)
	}
	want = "# checked in the game\r\n9, 11, 10, 5   # the good ones\r\n# 5   (not checked in a lab yet)\r\n"
	if got := e.readFile(e.vettedFile()); got != want {
		t.Fatalf("%q", got)
	}
	if list, _ := ParseVetted(e.readFile(e.vettedFile())); joinInts(list) != "5, 9, 10, 11" {
		t.Fatal(list)
	}
}

func TestVetPassOfALayoutAlreadyInTheListLeavesTheFileAlone(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.writeVetted("9 11 # mine")
	r, err := VetRecord(e.opts(), 9, true, "again")
	if err != nil {
		t.Fatal(err)
	}
	if e.readFile(e.vettedFile()) != "9 11 # mine" {
		t.Fatal("the list file was rewritten")
	}
	mustContain(t, lines(r), "is in it already", "the file is not changed")
	if !strings.Contains(e.readFile(e.vettingFile()), "layout 9 pass again") {
		t.Fatal("the result was not recorded")
	}
}

func TestVetFailTakesTheLayoutOutKeepsTheCommentsAndTheRecord(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.writeVetted("# checked\n9, 11 # good ones\n10\n# 8 failed\n")
	r, err := VetRecord(e.opts(), 9, false, "worm threat 100% on rock")
	if err != nil {
		t.Fatal(err)
	}
	if got := e.readFile(e.vettedFile()); got != "# checked\n11 # good ones\n10\n# 8 failed\n" {
		t.Fatalf("%q", got)
	}
	mustContain(t, lines(r), "taken out", "no longer picks layout 9", "the record of the failure stays")
	// a line that held only the number is dropped
	if _, err := VetRecord(e.opts(), 10, false, ""); err != nil {
		t.Fatal(err)
	}
	if got := e.readFile(e.vettedFile()); got != "# checked\n11 # good ones\n# 8 failed\n" {
		t.Fatalf("%q", got)
	}
	if got := e.readFile(e.vettingFile()); got != "2026-11-24 05:00 layout 9 fail worm threat 100% on rock\r\n2026-11-24 05:00 layout 10 fail\r\n" {
		t.Fatalf("%q", got)
	}
}

func TestVetFailWithNoFileMakesTheFileFromTheBuiltInListWithoutN(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	if _, err := VetRecord(e.opts(), 9, false, "bad"); err != nil {
		t.Fatal(err)
	}
	list, err := ParseVetted(e.readFile(e.vettedFile()))
	if err != nil || joinInts(list) != "11" {
		t.Fatalf("%v %v", list, err)
	}
	// a fail of a layout that is in no list creates no list file, but is recorded
	e2 := newEnv(t, iniWith("11"))
	r, err := VetRecord(e2.opts(), 4, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if e2.exists(e2.vettedFile()) {
		t.Fatal("a list file was made")
	}
	mustContain(t, lines(r), "was not in it")
	if !strings.Contains(e2.readFile(e2.vettingFile()), "layout 4 fail") {
		t.Fatal("not recorded")
	}
}

func TestVetFailOfTheLastLayoutWarnsAndALaterPassWorks(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.writeVetted("# mine\r\n9\r\n")
	r, err := VetRecord(e.opts(), 9, false, "")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, lines(r), "WARNING", "list is empty")
	if _, err := ParseVetted(e.readFile(e.vettedFile())); !errors.Is(err, errNoVettedNumber) {
		t.Fatalf("%v", err)
	}
	if !strings.HasPrefix(e.readFile(e.vettedFile()), "# mine\r\n") {
		t.Fatal("the comment was lost")
	}
	// a roll refuses an empty list and says so; a pass puts a layout back
	if _, err := Roll(e.vettedOpts()); err == nil || !strings.Contains(err.Error(), "no layout number") {
		t.Fatalf("%v", err)
	}
	if _, err := VetRecord(e.opts(), 10, true, ""); err != nil {
		t.Fatal(err)
	}
	list, err := ParseVetted(e.readFile(e.vettedFile()))
	if err != nil || joinInts(list) != "10" {
		t.Fatalf("%v %v", list, err)
	}
	if !strings.HasPrefix(e.readFile(e.vettedFile()), "# mine\r\n") {
		t.Fatal("the comment was lost")
	}
}

func TestVetPassIsRefusedAbove11AndForALayoutGameIniDoesNotList(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	if _, err := VetRecord(e.opts(), 12, true, ""); err == nil || !strings.Contains(err.Error(), "only layouts 0 to 11") {
		t.Fatalf("%v", err)
	}
	short := baseIni + Key + `=(("/Game/Maps/DD",(WorldLayoutsSettingsList=((Content="/Game/L00"),(Content="/Game/L01"),(Content="/Game/L02")),WorldLayoutIndexInSingleplayer=1)))` + "\r\n"
	es := newEnv(t, short)
	if _, err := VetRecord(es.opts(), 5, true, ""); err == nil || !strings.Contains(err.Error(), "lists only 3 layouts") {
		t.Fatalf("%v", err)
	}
	for _, x := range []*env{e, es} {
		if len(x.settingsFiles()) != 0 {
			t.Fatalf("a refused pass wrote: %v", x.settingsFiles())
		}
	}
	// a pass is also refused while Game.ini cannot be read; a fail does not need it
	eu := newEnv(t, "")
	if err := os.WriteFile(filepath.Join(eu.dir, FileName), []byte("\xff\xfe[\x00A\x00]\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VetRecord(eu.opts(), 3, true, ""); err == nil || !strings.Contains(err.Error(), "Game.ini cannot be read") {
		t.Fatalf("%v", err)
	}
	if len(eu.settingsFiles()) != 0 {
		t.Fatal("written")
	}
	if _, err := VetRecord(eu.opts(), 3, false, ""); err != nil {
		t.Fatal(err)
	}
	// a number that is not a layout number at all
	if _, err := VetRecord(e.opts(), 100, false, ""); err == nil {
		t.Fatal("100 accepted")
	}
	if _, err := VetRecord(e.opts(), -1, false, ""); err == nil {
		t.Fatal("-1 accepted")
	}
}

func TestVetFailOfANumberAbove11TakesAStrayEntryOut(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.writeVetted("9, 15")
	if _, err := VetRecord(e.opts(), 15, false, "typo"); err != nil {
		t.Fatal(err)
	}
	if got := e.readFile(e.vettedFile()); got != "9" {
		t.Fatalf("%q", got)
	}
}

func TestVetRecordNeedsNoClosedGameAndNeverTouchesGameIni(t *testing.T) {
	old := iniWith("11")
	e := newEnv(t, old)
	e.running = true // the person has just looked at the layout in the game
	if _, err := VetRecord(e.opts(), 8, true, ""); err != nil {
		t.Fatal(err)
	}
	if e.ini() != old || len(e.backups()) != 0 {
		t.Fatal("Game.ini was touched")
	}
	if e.checks != 0 {
		t.Fatalf("the process list was asked %d times", e.checks)
	}
}

func TestVetNoteIsOneLineOf200CharactersAtMost(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	note := "worm\nthreat\r\n100%\ton rock and\x00 more " + strings.Repeat("é", 300)
	if _, err := VetRecord(e.opts(), 8, false, note); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimRight(e.readFile(e.vettingFile()), "\r\n")
	if strings.ContainsAny(got, "\r\n\t\x00 ") || strings.Count(e.readFile(e.vettingFile()), "\n") != 1 {
		t.Fatalf("%q", got)
	}
	prefix := "2026-11-24 05:00 layout 8 fail "
	n := strings.TrimPrefix(got, prefix)
	if !strings.HasPrefix(n, "worm threat 100% on rock and more é") || len([]rune(n)) != maxVetNote {
		t.Fatalf("%d runes: %q", len([]rune(n)), n)
	}
	if cleanNote("  \n \t ") != "" {
		t.Fatal("blank note")
	}
}

func TestVetResultsAreAppendedNewestLastInTheFilesOwnLineEnds(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	if err := os.MkdirAll(filepath.Dir(e.state), 0o755); err != nil {
		t.Fatal(err)
	}
	// an older file without a last line end, with unix line ends
	if err := os.WriteFile(e.vettingFile(), []byte("2026-11-20 10:00 layout 9 pass\n2026-11-21 10:00 layout 8 fail rock"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VetRecord(e.opts(), 8, true, "second look"); err != nil {
		t.Fatal(err)
	}
	want := "2026-11-20 10:00 layout 9 pass\n2026-11-21 10:00 layout 8 fail rock\n2026-11-24 05:00 layout 8 pass second look\n"
	if got := e.readFile(e.vettingFile()); got != want {
		t.Fatalf("%q", got)
	}
}

func TestVetKeepsAByteOrderMarkAndNoTempFilesAreLeftBehind(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.writeVetted("\xef\xbb\xbf# mine\r\n9, 11\r\n")
	if _, err := VetRecord(e.opts(), 10, true, ""); err != nil {
		t.Fatal(err)
	}
	if got := e.readFile(e.vettedFile()); got != "\xef\xbb\xbf# mine\r\n9, 11, 10\r\n" {
		t.Fatalf("%q", got)
	}
	if _, err := VetRecord(e.opts(), 9, false, ""); err != nil {
		t.Fatal(err)
	}
	if got := e.readFile(e.vettedFile()); got != "\xef\xbb\xbf# mine\r\n11, 10\r\n" {
		t.Fatalf("%q", got)
	}
	for name := range e.settingsFiles() {
		if name != VettedFileName && name != VettingFileName {
			t.Errorf("a leftover file: %s", name)
		}
	}
}

func TestVetRecordWritesNothingWhenAFileCannotBeUsed(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	// a list file with something that is not a number: refused, and no result line is written
	e.writeVetted("9, eleven")
	if _, err := VetRecord(e.opts(), 8, true, ""); err == nil || !strings.Contains(err.Error(), "cannot be used") {
		t.Fatalf("%v", err)
	}
	if e.readFile(e.vettedFile()) != "9, eleven" || e.exists(e.vettingFile()) {
		t.Fatal("written")
	}
	// a results file that is a folder: refused before the list is changed
	e2 := newEnv(t, iniWith("11"))
	e2.writeVetted("9, 11")
	if err := os.Mkdir(e2.vettingFile(), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := VetRecord(e2.opts(), 8, true, ""); err == nil || !strings.Contains(err.Error(), "results file") {
		t.Fatalf("%v", err)
	}
	if e2.readFile(e2.vettedFile()) != "9, 11" {
		t.Fatal("the list was changed although the record could not be written")
	}
	// no settings folder is known
	e3 := newEnv(t, iniWith("11"))
	o := e3.opts()
	o.StatePath = ""
	if _, err := VetRecord(o, 8, true, ""); err == nil || !strings.Contains(err.Error(), "no settings folder") {
		t.Fatalf("%v", err)
	}
}

func TestReplaceVetFileRefusesWhenTheFileChangedSinceItWasRead(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "x.txt")
	if err := os.WriteFile(p, []byte("9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeVetFile(p, "9, 11\n", true, "9, 11, 10\n"); err == nil || !strings.Contains(err.Error(), "changed while this ran") {
		t.Fatalf("%v", err)
	}
	if err := writeVetFile(p, "", false, "x\n"); err == nil {
		t.Fatal("a file that appeared was overwritten")
	}
	if b, _ := os.ReadFile(p); string(b) != "9\n" {
		t.Fatal("changed")
	}
	if err := writeVetFile(p, "9\n", true, "9, 10\n"); err != nil {
		t.Fatal(err)
	}
}

func TestAPassedLayoutIsPickedByTheRollAndAFailedOneIsNot(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	if _, err := VetRecord(e.opts(), 10, true, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := VetRecord(e.opts(), 9, false, ""); err != nil {
		t.Fatal(err)
	}
	// the list is 10, 11; Game.ini sets 11 now, so the only layout a roll may pick is 10
	for pick := 0; pick < 3; pick++ {
		e.pick = pick
		o := e.vettedOpts()
		o.Force = true
		o.DryRun = true
		r, err := Roll(o)
		if err != nil || r.New != 10 {
			t.Fatalf("%+v %v", r, err)
		}
	}
}

// ---- the editing of the text ----

func TestAddAndRemoveVettedTextEdgeCases(t *testing.T) {
	cases := []struct {
		name, in string
		n        int
		add      string
	}{
		{"one number, no line end", "9", 10, "9, 10"},
		{"unix line ends", "9, 11\n", 10, "9, 11, 10\n"},
		{"comment only, then a number", "# nothing yet\r\n", 10, "# nothing yet\r\n10\r\n"},
		{"number in a comment is not a number", "9 # 10 is not good\r\n", 10, "9, 10 # 10 is not good\r\n"},
		{"a leading zero is the same number", "09, 11\n", 10, "09, 11, 10\n"},
		{"a number twice", "9, 11, 9\n", 10, "9, 11, 9, 10\n"},
		{"numbers on several lines", "9\n11\n", 10, "9\n11, 10\n"},
		{"trailing comma", "9, 11,\n", 10, "9, 11, 10,\n"},
		{"indented", "  9 11  # x\n", 10, "  9 11, 10  # x\n"},
	}
	for _, c := range cases {
		if got := addVettedText(c.in, c.n); got != c.add {
			t.Errorf("%s: add: %q, want %q", c.name, got, c.add)
		}
		// remove 10 from the added text gives back something that lists the same layouts as the input
		back := removeVettedText(addVettedText(c.in, c.n), c.n)
		l1, e1 := ParseVetted(c.in)
		l2, e2 := ParseVetted(back)
		if (e1 == nil) != (e2 == nil) && !errors.Is(e1, errNoVettedNumber) || joinInts(l1) != joinInts(l2) {
			t.Errorf("%s: add then remove: %q -> %q", c.name, c.in, back)
		}
	}
	if got := removeVettedText("9, 11, 9\n", 9); got != "11\n" {
		t.Errorf("%q", got)
	}
	if got := removeVettedText("9, 11 # a\n", 11); got != "9 # a\n" {
		t.Errorf("%q", got)
	}
	if got := removeVettedText("9 # a\n", 9); got != "# a\n# (no layout is vetted now: `tabr-tau layout-roll vet N pass` adds one)\n" {
		t.Errorf("%q", got)
	}
	if got := removeVettedText("9\n11\n", 9); got != "11\n" {
		t.Errorf("%q", got)
	}
	if got := removeVettedText("9", 9); !strings.HasPrefix(got, "# (no layout is vetted now") {
		t.Errorf("%q", got)
	}
}

// ---- vet list ----

func TestVetListShowsEveryLayoutItsLastResultAndWhetherARollMayPickIt(t *testing.T) {
	e := newEnv(t, iniWith("9"))
	e.writeVetted("9, 11, 15 # mine")
	if err := os.WriteFile(e.vettingFile(), []byte("2026-11-20 10:00 layout 8 pass looked fine\r\n# a comment\r\nnot a result\r\n2026-11-21 11:30 layout 9 pass\r\n2026-11-24 22:41 layout 8 fail worm threat 100% on rock\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := e.settingsFiles()
	r, err := VetList(e.opts())
	if err != nil {
		t.Fatal(err)
	}
	out := lines(r)
	mustContain(t, out, "Vetted list in use: 9, 11, 15", "layout-roll-vetted.txt", "Game.ini sets layout 9 now")
	row := func(n string) string {
		for _, l := range r.Lines {
			if strings.HasPrefix(l, n+" ") {
				return l
			}
		}
		t.Fatalf("no row for layout %s in:\n%s", n, out)
		return ""
	}
	rows := 0
	for _, l := range r.Lines {
		if f := strings.Fields(l); len(f) > 0 && len(f) > 1 && f[1] != "line(s)" && strings.Trim(f[0], "0123456789") == "" {
			rows++
		}
	}
	if rows != 12 {
		t.Fatalf("%d rows:\n%s", rows, out)
	}
	mustContain(t, row("8"), "fail 2026-11-24 22:41", "no", "note: worm threat 100% on rock")
	mustContain(t, row("9"), "pass 2026-11-21 11:30", "yes (but not while it is set now)")
	mustContain(t, row("11"), "no result yet", "yes")
	mustContain(t, row("0"), "no result yet")
	if strings.Contains(row("0"), "yes") || strings.Contains(row("8"), "yes") || strings.Contains(row("10"), "yes") {
		t.Fatal("a layout that is not vetted is shown as pickable")
	}
	mustContain(t, out, "also holds 15", "1 line(s) of the results file were not understood")
	// reading only
	if after := e.settingsFiles(); len(after) != len(before) || after[VettedFileName] != before[VettedFileName] || after[VettingFileName] != before[VettingFileName] {
		t.Fatal("vet list wrote")
	}
	if len(e.backups()) != 0 || e.checks != 0 {
		t.Fatal("vet list touched Game.ini or asked for the game")
	}
}

func TestVetListWithNoFilesUsesTheBuiltInListAndRefusesUnusableFiles(t *testing.T) {
	e := newEnv(t, iniWith("3"))
	r, err := VetList(e.opts())
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, lines(r), "Vetted list in use: 9, 11", "built-in default", "Game.ini sets layout 3 now")
	if len(e.settingsFiles()) != 0 {
		t.Fatal("vet list made a file")
	}
	// no Game.ini at all: the game default is said
	em := newEnv(t, "")
	r, err = VetList(em.opts())
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, lines(r), "has no layout line", "layout 11")
	eu := newEnv(t, "")
	if err := os.WriteFile(filepath.Join(eu.dir, FileName), []byte("\xff\xfe[\x00A\x00]\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err = VetList(eu.opts())
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, lines(r), "Game.ini could not be read", "no result yet")
	// a list file that cannot be used, or a results file that cannot be used
	e.writeVetted("9, nine")
	if _, err := VetList(e.opts()); err == nil || !strings.Contains(err.Error(), "cannot be used") {
		t.Fatalf("%v", err)
	}
	e.writeVetted("# comments only\n")
	if r, err := VetList(e.opts()); err != nil || !strings.Contains(lines(r), "Vetted list in use: empty") {
		t.Fatalf("%v\n%s", err, lines(r))
	}
	if err := os.Mkdir(e.vettingFile(), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := VetList(e.opts()); err == nil || !strings.Contains(err.Error(), "results file") {
		t.Fatalf("%v", err)
	}
}

func TestVetSetupErrorsAreBugsNotPanics(t *testing.T) {
	if _, err := VetSet(Options{}, 3); err == nil {
		t.Fatal("no error")
	}
	if _, err := VetRecord(Options{}, 3, true, ""); err == nil {
		t.Fatal("no error")
	}
}
