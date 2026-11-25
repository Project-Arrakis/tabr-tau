package layoutroll

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The vetted list (#1219): a random roll picks only from layouts the operator has checked in the game.

func (e *env) vettedFile() string { return filepath.Join(filepath.Dir(e.state), VettedFileName) }

func (e *env) writeVetted(text string) {
	e.t.Helper()
	if err := os.MkdirAll(filepath.Dir(e.state), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(e.vettedFile(), []byte(text), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

// vettedOpts is the options of a run that reads the settings file (or the built-in list), not the test default of "every layout".
func (e *env) vettedOpts() Options {
	o := e.opts()
	o.VettedFlag = nil
	return o
}

func strPtr(s string) *string { return &s }

// mustRefuse checks that a run was refused with the words in want, and that Game.ini and its backups were not touched.
func (e *env) mustRefuse(o Options, ini string, want ...string) error {
	e.t.Helper()
	r, err := Roll(o)
	if err == nil || r.Wrote {
		e.t.Fatalf("the run was not refused: %+v %v", r, err)
	}
	for _, w := range want {
		if !strings.Contains(err.Error(), w) {
			e.t.Fatalf("the refusal %q does not say %q", err, w)
		}
	}
	if !strings.HasPrefix(err.Error(), "not changed: ") {
		e.t.Fatalf("the refusal does not say that nothing was changed: %q", err)
	}
	if e.ini() != ini || len(e.backups()) != 0 {
		e.t.Fatal("a refused run touched Game.ini or made a backup")
	}
	return err
}

func TestParseVettedTakesCommasSpacesNewlinesAndComments(t *testing.T) {
	for text, want := range map[string]string{
		"9,11":                                  "9, 11",
		"9, 11":                                 "9, 11",
		"11 9":                                  "9, 11",
		"9\n11\n":                               "9, 11",
		"9\r\n11\r\n":                           "9, 11",
		"# checked in the game\n9, 11 # both\n": "9, 11",
		"# 8 failed\n 5\n#6\n":                  "5",
		"9,9,11,9":                              "9, 11",
		"\xef\xbb\xbf9, 11":                     "9, 11",
		"09, 011":                               "9, 11",
		"0 99":                                  "0, 99",
		",,9,,":                                 "9",
	} {
		got, err := ParseVetted(text)
		if err != nil || joinInts(got) != want {
			t.Errorf("%q: %v %v, want %s", text, got, err, want)
		}
	}
}

func TestParseVettedRefusesWhatItDoesNotUnderstand(t *testing.T) {
	for _, text := range []string{"", "   \n\n", "# only a comment\n", "nine", "9, x", "9 11a", "-1", "+9", "9.5", "100", "999", "0x9", "９", "9;11", "9-11"} {
		if got, err := ParseVetted(text); err == nil {
			t.Errorf("%q was accepted as %v", text, got)
		}
	}
	// a typo is named, so it is found
	if _, err := ParseVetted("9, 1l"); err == nil || !strings.Contains(err.Error(), `"1l"`) {
		t.Errorf("%v", err)
	}
}

func TestTheBuiltInLineListsEnoughLayoutsAndTheTestLineMatchesIt(t *testing.T) {
	if n := knownLayouts(""); n < MaxIndex+1 {
		t.Fatalf("the built-in line lists %d layouts", n)
	}
	if n := knownLayouts(iniWith("11")); n != testLayouts {
		t.Fatalf("the stand-in line lists %d", n)
	}
	if n := knownLayouts(baseIni + Key + "=()\r\n"); n != 0 {
		t.Fatalf("a line with no list counts %d", n)
	}
	if got := defaultVetted(); joinInts(got) != "9, 11" {
		t.Fatalf("the built-in list is %v", got)
	}
}

func TestWithNoFileTheBuiltInListIsUsedAndOnlyItIsPicked(t *testing.T) {
	for _, cur := range []int{0, 5, 8, 9, 10, 11} {
		for pick := 0; pick < 3; pick++ {
			e := newEnv(t, iniWith(strconv.Itoa(cur)))
			e.pick = pick
			r, err := Roll(e.vettedOpts())
			if cur == 9 || cur == 11 {
				want := 11
				if cur == 11 {
					want = 9
				}
				if err != nil || r.New != want {
					t.Fatalf("current %d pick %d: %+v %v", cur, pick, r, err)
				}
			} else if err != nil || (r.New != 9 && r.New != 11) {
				t.Fatalf("current %d pick %d: %+v %v", cur, pick, r, err)
			}
			text := strings.Join(r.Lines, "\n")
			if !strings.Contains(text, "Vetted layouts: 9, 11.") || !strings.Contains(text, "the built-in default (no file "+e.vettedFile()+" exists)") {
				t.Fatalf("the lines do not say the list and where it came from:\n%s", text)
			}
		}
	}
}

func TestTheFileListIsUsedAndAnUnvettedLayoutIsNeverPicked(t *testing.T) {
	for _, cur := range []int{0, 8, 9, 11} {
		for pick := 0; pick < 4; pick++ {
			e := newEnv(t, iniWith(strconv.Itoa(cur)))
			e.writeVetted("# checked in the game\n5, 9\n11   # also fine\n")
			e.pick = pick
			r, err := Roll(e.vettedOpts())
			if err != nil || r.New == cur || (r.New != 5 && r.New != 9 && r.New != 11) {
				t.Fatalf("current %d pick %d: %+v %v", cur, pick, r, err)
			}
			text := strings.Join(r.Lines, "\n")
			if !strings.Contains(text, "Vetted layouts: 5, 9, 11. Source: the file "+e.vettedFile()) {
				t.Fatalf("the lines do not name the file:\n%s", text)
			}
			var cand []string
			for _, n := range []int{5, 9, 11} {
				if n != cur {
					cand = append(cand, strconv.Itoa(n))
				}
			}
			want := "A random pick may choose from: " + strings.Join(cand, ", ") + " (never layout " + strconv.Itoa(cur) + ", which is set now)."
			if !strings.Contains(text, want) {
				t.Fatalf("the candidates are not the list without the current layout (want %q):\n%s", want, text)
			}
		}
	}
}

func TestEveryCandidateIsReachableAndNothingElse(t *testing.T) {
	e := newEnv(t, iniWith("2"))
	e.writeVetted("4 7 9")
	seen := map[int]bool{}
	for pick := 0; pick < 3; pick++ {
		e.pick = pick
		o := e.vettedOpts()
		o.Force = true // the weekly rule is not what this test is about
		r, err := Roll(o)
		if err != nil {
			t.Fatal(err)
		}
		seen[r.New] = true
		e.write(iniWith("2")) // back to layout 2 so the draws see the same candidates
	}
	if len(seen) != 3 || !seen[4] || !seen[7] || !seen[9] {
		t.Fatalf("picks: %v", seen)
	}
}

func TestAnEmptyOrUnusableFileRefusesAndNamesTheFile(t *testing.T) {
	for name, text := range map[string]string{
		"empty":         "",
		"blank":         "\n  \n",
		"only comments": "# nothing checked yet\n# 9, 11\n",
		"a word":        "nine",
		"a typo":        "9, 1l",
		"too big":       "9, 100",
		"negative":      "-1",
		"UTF-16":        "\xff\xfe9\x00",
	} {
		old := iniWith("3")
		e := newEnv(t, old)
		e.writeVetted(text)
		o := e.vettedOpts()
		err := e.mustRefuse(o, old, e.vettedFile(), VettedFileName)
		if !strings.Contains(err.Error(), "delete the file") && !strings.Contains(err.Error(), "Fix or delete") {
			t.Errorf("%s: the refusal does not say what to do: %v", name, err)
		}
		// a dry run refuses in the same way and writes nothing
		o.DryRun = true
		if _, err := Roll(o); err == nil {
			t.Errorf("%s: a dry run was not refused", name)
		}
		if e.ini() != old || len(e.backups()) != 0 {
			t.Errorf("%s: Game.ini or its backups changed", name)
		}
	}
}

func TestAFileThatCannotBeReadRefuses(t *testing.T) {
	old := iniWith("3")
	e := newEnv(t, old)
	if err := os.MkdirAll(e.vettedFile(), 0o755); err != nil { // a folder where the file should be
		t.Fatal(err)
	}
	e.mustRefuse(e.vettedOpts(), old, e.vettedFile(), "cannot be used")

	e2 := newEnv(t, old)
	e2.writeVetted(strings.Repeat("9\n", maxVettedFile)) // larger than the limit
	e2.mustRefuse(e2.vettedOpts(), old, e2.vettedFile(), "larger than")
}

func TestANumberThatTheGameIniDoesNotListIsRefused(t *testing.T) {
	old := iniWith("3")
	e := newEnv(t, old)
	e.writeVetted("9, 40")
	e.mustRefuse(e.vettedOpts(), old, "layout 40", "lists only "+strconv.Itoa(testLayouts)+" layouts")

	// a Game.ini whose own line lists fewer layouts: 9 is not one of them
	short := baseIni + Key + `=(("/Game/Maps/DD",(WorldLayoutsSettingsList=((Content="/Game/L00"),(Content="/Game/L01"),(Content="/Game/L02")),WorldLayoutIndexInSingleplayer=1)))` + "\r\n"
	e2 := newEnv(t, short)
	e2.writeVetted("2, 9")
	e2.mustRefuse(e2.vettedOpts(), short, "layout 9", "lists only 3 layouts")
	e2.writeVetted("0, 2")
	if r, err := Roll(e2.vettedOpts()); err != nil || (r.New != 0 && r.New != 2) {
		t.Fatalf("%+v %v", r, err)
	}

	// listed by the game but never written by the roll
	e3 := newEnv(t, old)
	e3.writeVetted("9, 12")
	e3.mustRefuse(e3.vettedOpts(), old, "layout 12", "only writes layouts 0 to 11")
}

func TestTheVettedFlagOverridesTheFileForOneRun(t *testing.T) {
	old := iniWith("3")
	e := newEnv(t, old)
	e.writeVetted("5") // the file would give 5
	o := e.opts()
	o.VettedFlag = strPtr("9,11")
	for pick := 0; pick < 2; pick++ {
		e.write(old)
		e.pick = pick
		o.Force = true
		r, err := Roll(o)
		if err != nil || (r.New != 9 && r.New != 11) {
			t.Fatalf("%+v %v", r, err)
		}
		if text := strings.Join(r.Lines, "\n"); !strings.Contains(text, "Source: the --vetted flag.") || !strings.Contains(text, "Vetted layouts: 9, 11.") {
			t.Fatalf("the lines do not say the flag was used:\n%s", text)
		}
	}
	// the file itself is not changed by a run that used the flag
	if b, _ := os.ReadFile(e.vettedFile()); string(b) != "5" {
		t.Fatal("the settings file was changed")
	}
}

func TestABadVettedFlagIsRefusedEvenWithAGoodFile(t *testing.T) {
	for _, f := range []string{"", "  ", "x", "9,,y", "9,100", "9,40", "12"} {
		old := iniWith("3")
		e := newEnv(t, old)
		e.writeVetted("9, 11")
		o := e.opts()
		o.VettedFlag = strPtr(f)
		e.mustRefuse(o, old, "--vetted")
	}
}

func TestNothingToRollToWhenTheListHoldsOnlyTheCurrentLayout(t *testing.T) {
	for _, tc := range []struct{ file, flag string }{{file: "9"}, {flag: "9"}} {
		old := iniWith("9")
		e := newEnv(t, old)
		o := e.vettedOpts()
		if tc.file != "" {
			e.writeVetted(tc.file)
		} else {
			e.writeVetted("9, 11") // the flag wins over this
			o.VettedFlag = strPtr(tc.flag)
		}
		e.mustRefuse(o, old, "no other vetted layout", "sets layout 9", VettedFileName)
	}
}

func TestSetIsNotHeldBackByTheVettedList(t *testing.T) {
	for _, text := range []string{"", "nonsense", "9"} {
		old := iniWith("9")
		e := newEnv(t, old)
		e.writeVetted(text)
		o := e.vettedOpts()
		o.Set = ptr(8)
		if r, err := Roll(o); err != nil || r.New != 8 || !r.Wrote {
			t.Fatalf("file %q: %+v %v", text, r, err)
		}
	}
}

func TestADryRunSaysTheListItsSourceAndTheCandidatesAndWritesNothing(t *testing.T) {
	old := iniWith("9")
	e := newEnv(t, old)
	e.writeVetted("5, 9, 11")
	o := e.vettedOpts()
	o.DryRun = true
	r, err := Roll(o)
	if err != nil || r.Wrote {
		t.Fatalf("%+v %v", r, err)
	}
	text := strings.Join(r.Lines, "\n")
	for _, w := range []string{"Vetted layouts: 5, 9, 11. Source: the file " + e.vettedFile(), "A random pick may choose from: 5, 11 (never layout 9, which is set now).", "Dry run: would change Game.ini from layout 9 to layout"} {
		if !strings.Contains(text, w) {
			t.Fatalf("the dry run does not say %q:\n%s", w, text)
		}
	}
	if e.ini() != old || len(e.backups()) != 0 {
		t.Fatal("a dry run wrote Game.ini")
	}
	if _, err := os.Stat(e.state); err == nil {
		t.Fatal("a dry run wrote the state file")
	}
}

func TestARealRunRecordsTheListItsSourceAndTheCandidates(t *testing.T) {
	e := newEnv(t, iniWith("9"))
	e.pick = 0
	r, err := Roll(e.vettedOpts()) // no file: the built-in list
	if err != nil || r.New != 11 {
		t.Fatalf("%+v %v", r, err)
	}
	st := StateLinesForTest(t, e.state)
	for _, w := range []string{"rolled layout 9 -\\u003e 11", "vetted layouts 9, 11, source: the built-in default (no file", "chosen from 11"} {
		if !strings.Contains(st, w) {
			t.Fatalf("the record does not say %q:\n%s", w, st)
		}
	}
	// a chosen layout records no list
	e2 := newEnv(t, iniWith("9"))
	o := e2.vettedOpts()
	o.Set = ptr(4)
	if _, err := Roll(o); err != nil {
		t.Fatal(err)
	}
	if st := StateLinesForTest(t, e2.state); strings.Contains(st, "vetted") {
		t.Fatalf("a chosen layout recorded a list:\n%s", st)
	}
}

func TestARefusalOfTheListIsRecordedInTheStateFileUnderTheLock(t *testing.T) {
	old := iniWith("3")
	e := newEnv(t, old)
	e.writeVetted("")
	e.mustRefuse(e.vettedOpts(), old, VettedFileName)
	st := StateLinesForTest(t, e.state)
	if !strings.Contains(st, "not changed: the vetted layouts file") || strings.Contains(st, `"lastRoll"`) {
		t.Fatalf("the refusal is not recorded as a run that changed nothing:\n%s", st)
	}
}

func TestARefusalWithoutASaveGoesToTheLog(t *testing.T) {
	old := iniWith("3")
	e := newEnv(t, old)
	e.writeVetted("")
	o := e.vettedOpts()
	o.SavePath = "" // no lock is held, so the note goes to the log
	e.mustRefuse(o, old, VettedFileName)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(e.state), LogName))
	if err != nil || !strings.Contains(string(b), "layout roll refused: the vetted layouts file") {
		t.Fatalf("%q %v", b, err)
	}
}
