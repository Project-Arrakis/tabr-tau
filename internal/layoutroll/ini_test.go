package layoutroll

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

const sec = "[" + Section + "]"

// lineWith is a short stand-in for the layout line (the real one is 15 KB).
func lineWith(n string) string {
	return Key + `=(("/Game/Maps/DD",(WorldLayoutsSettingsList=((Content="/Game/L00"),(Content="/Game/L01")),WorldLayoutIndexInSingleplayer=` + n + `)))`
}

func TestTheBuiltInLineIsUsable(t *testing.T) {
	for i := MinIndex; i <= MaxIndex; i++ {
		got, err := templateLine(lineTemplate, i)
		if err != nil {
			t.Fatal(err)
		}
		_, v, _ := keyOf(got)
		if m := indexPattern.FindAllStringSubmatch(v, -1); len(m) != 1 || m[0][2] != strconv.Itoa(i) {
			t.Fatalf("layout %d: the number is not set once: %v", i, m)
		}
		// nothing but the number differs from the template
		a := strings.Replace(got, "IndexInSingleplayer="+strconv.Itoa(i), "IndexInSingleplayer=N", 1)
		b := strings.Replace(strings.TrimSpace(lineTemplate), "IndexInSingleplayer="+strconv.Itoa(DefaultIndex), "IndexInSingleplayer=N", 1)
		if a != b {
			t.Fatalf("layout %d: the line differs from the template in more than the number", i)
		}
	}
	if !strings.HasPrefix(lineTemplate, Key+"=") || strings.ContainsAny(strings.TrimSpace(lineTemplate), "\r\n") {
		t.Fatal("the built-in line is not one line")
	}
	// the default in the template is DefaultIndex, and the layouts 0 to 11 are all in it
	if i, _, err := CurrentIndex(sec + "\n" + strings.TrimSpace(lineTemplate) + "\n"); err != nil || i != DefaultIndex {
		t.Fatalf("template reads as %d, %v", i, err)
	}
	for i := MinIndex; i <= MaxIndex; i++ {
		name := "DA_DeepDesert_1_Layout_" + fmt.Sprintf("%02d", i) + "."
		if !strings.Contains(lineTemplate, name) {
			t.Errorf("the built-in line has no data for layout %d", i)
		}
	}
}

func TestSetIndexWhenTheLineIsThere(t *testing.T) {
	for _, eol := range []string{"\n", "\r\n"} {
		in := "[Other]" + eol + "A=1" + eol + eol + sec + eol + "; a comment" + eol + lineWith("11") + eol + "Z=2" + eol + "[Tail]" + eol + "B=3" + eol
		got, err := SetIndex(in, 4)
		if err != nil {
			t.Fatal(err)
		}
		want := strings.Replace(in, "WorldLayoutIndexInSingleplayer=11", "WorldLayoutIndexInSingleplayer=4", 1)
		if got != want {
			t.Fatalf("eol %q: got\n%q\nwant\n%q", eol, got, want)
		}
		if i, present, err := CurrentIndex(got); err != nil || !present || i != 4 {
			t.Fatalf("read back %d %v %v", i, present, err)
		}
	}
}

func TestSetIndexKeepsOddSpacingAndNoFinalLineEnd(t *testing.T) {
	in := "  " + sec + "  \r\n   " + Key + "  =  (X=1, WorldLayoutIndexInSingleplayer =   7 ,Y=2)  "
	got, err := SetIndex(in, 10)
	if err != nil {
		t.Fatal(err)
	}
	want := "  " + sec + "  \r\n   " + Key + "  =  (X=1, WorldLayoutIndexInSingleplayer =   10 ,Y=2)  "
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if i, _, err := CurrentIndex(in); err != nil || i != 7 {
		t.Fatalf("current %d %v", i, err)
	}
}

func TestSetIndexAddsTheLineUnderItsSection(t *testing.T) {
	for _, eol := range []string{"\n", "\r\n"} {
		in := "[A]" + eol + "x=1" + eol + sec + eol + "other=2" + eol + "[B]" + eol + "y=3" + eol
		got, err := SetIndex(in, 3)
		if err != nil {
			t.Fatal(err)
		}
		want := "[A]" + eol + "x=1" + eol + sec + eol + lineOf(3) + eol + "other=2" + eol + "[B]" + eol + "y=3" + eol
		if got != want {
			t.Fatalf("eol %q: line is not added right under the header:\n%q", eol, got[:min(len(got), 200)])
		}
		if i, present, err := CurrentIndex(got); err != nil || !present || i != 3 {
			t.Fatalf("read back %d %v %v", i, present, err)
		}
	}
}

func lineOf(n int) string {
	l, _ := templateLine(lineTemplate, n)
	return l
}

func TestSetIndexAddsTheSectionAtTheEnd(t *testing.T) {
	// no final line end, CRLF file
	got, err := SetIndex("[A]\r\nx=1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := "[A]\r\nx=1\r\n\r\n" + sec + "\r\n" + lineOf(0) + "\r\n"; got != want {
		t.Fatalf("got %q", got[:min(len(got), 120)])
	}
	// LF file
	got, _ = SetIndex("[A]\nx=1\n", 5)
	if want := "[A]\nx=1\n\n" + sec + "\n" + lineOf(5) + "\n"; got != want {
		t.Fatalf("got %q", got[:min(len(got), 120)])
	}
	// empty file: Windows line ends, only the section and the line
	got, _ = SetIndex("", 2)
	if want := sec + "\r\n" + lineOf(2) + "\r\n"; got != want {
		t.Fatalf("got %q", got[:min(len(got), 120)])
	}
	// a header with no line end at the end of the file
	got, _ = SetIndex("[A]\nx=1\n"+sec, 6)
	if want := "[A]\nx=1\n" + sec + "\n" + lineOf(6) + "\n"; got != want {
		t.Fatalf("got %q", got[:min(len(got), 120)])
	}
}

func TestTheSameKeyUnderAnotherSectionIsNotThisSetting(t *testing.T) {
	other := "[/Script/Other.Thing]\n" + lineWith("2") + "\n"
	in := other + sec + "\n" + lineWith("9") + "\n"
	got, err := SetIndex(in, 1)
	if err != nil {
		t.Fatal(err)
	}
	want := other + sec + "\n" + lineWith("1") + "\n"
	if got != want {
		t.Fatal("the line under the other section changed or the wrong line was edited")
	}
	// only under another section: the section is added, the other line stays
	got, _ = SetIndex(other, 1)
	if !strings.HasPrefix(got, other) || !strings.Contains(got, sec+"\n"+lineOf(1)+"\n") {
		t.Fatal("the layout section was not added after the other one")
	}
	if i, present, _ := CurrentIndex(other); present || i != DefaultIndex {
		t.Fatalf("a line under another section counted: %d %v", i, present)
	}
}

func TestTheSectionWrittenTwiceHasTheLineEditedInTheOneThatHasIt(t *testing.T) {
	in := sec + "\nA=1\n[Mid]\nB=2\n" + sec + "\n" + lineWith("3") + "\n"
	got, err := SetIndex(in, 8)
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Replace(in, "=3)))", "=8)))", 1); got != want {
		t.Fatal("wrong edit")
	}
	// and a missing line is added under the first header
	in = sec + "\nA=1\n[Mid]\nB=2\n" + sec + "\nC=3\n"
	got, _ = SetIndex(in, 8)
	if want := sec + "\n" + lineOf(8) + "\nA=1\n[Mid]\nB=2\n" + sec + "\nC=3\n"; got != want {
		t.Fatal("not added under the first header")
	}
}

func TestSectionAndKeyAreMatchedWithoutRegardToCase(t *testing.T) {
	in := "[" + strings.ToLower(Section) + "]\n" + strings.ToUpper(Key[:2]) + Key[2:] + "=(WorldLayoutIndexInSingleplayer=4)\n"
	got, err := SetIndex(in, 5)
	if err != nil || !strings.Contains(got, "Singleplayer=5)") || strings.Count(got, "[") != 1 {
		t.Fatalf("%q %v", got, err)
	}
}

func TestFormsThatAreNotEditedAreRefused(t *testing.T) {
	for name, in := range map[string]string{
		"the key twice":               sec + "\n" + lineWith("1") + "\n" + lineWith("2") + "\n",
		"the key twice, two sections": sec + "\n" + lineWith("1") + "\n[X]\n" + sec + "\n" + lineWith("2") + "\n",
		"an ini operator":             sec + "\n+" + lineWith("1") + "\n",
		"no number":                   sec + "\n" + Key + "=(A=1)\n",
		"the number twice":            sec + "\n" + Key + "=(WorldLayoutIndexInSingleplayer=1,WorldLayoutIndexInSingleplayer=2)\n",
	} {
		if got, err := SetIndex(in, 3); err == nil || got != "" {
			t.Errorf("%s: no refusal (%q, %v)", name, got, err)
		}
		if _, _, err := CurrentIndex(in); err == nil {
			t.Errorf("%s: CurrentIndex gave no error", name)
		}
	}
	if _, err := SetIndex(sec+"\n", MaxIndex+1); err == nil {
		t.Error("12 was accepted")
	}
	if _, err := SetIndex(sec+"\n", -1); err == nil {
		t.Error("-1 was accepted")
	}
	if _, err := setIndex("", 1, "Other=(WorldLayoutIndexInSingleplayer=1)"); err == nil {
		t.Error("a template for another key was used")
	}
	if _, err := setIndex("", 1, Key+"=(A=1)"); err == nil {
		t.Error("a template with no number was used")
	}
}

func TestCurrentIndexReadsLayoutsOutsideTheRollRange(t *testing.T) {
	if i, present, err := CurrentIndex(sec + "\n" + lineWith("16") + "\n"); err != nil || !present || i != 16 {
		t.Fatalf("%d %v %v", i, present, err)
	}
	if i, present, err := CurrentIndex(""); err != nil || present || i != DefaultIndex {
		t.Fatalf("%d %v %v", i, present, err)
	}
}

func TestPickIsInRangeAndNeverTheCurrentLayout(t *testing.T) {
	for cur := -1; cur <= 17; cur++ {
		seen := map[int]bool{}
		for r := 0; r < 40; r++ {
			r := r
			got := Pick(cur, func(n int) int { return r % n })
			if got < MinIndex || got > MaxIndex || got == cur {
				t.Fatalf("current %d: picked %d", cur, got)
			}
			seen[got] = true
		}
		want := MaxIndex - MinIndex + 1
		if cur >= MinIndex && cur <= MaxIndex {
			want--
		}
		if len(seen) != want {
			t.Fatalf("current %d: %d different picks, want %d", cur, len(seen), want)
		}
	}
	// a source that misbehaves still gives a legal number
	for _, bad := range []int{-5, 99} {
		bad := bad
		if got := Pick(3, func(int) int { return bad }); got == 3 || got < MinIndex || got > MaxIndex {
			t.Fatalf("bad source %d gave %d", bad, got)
		}
	}
	// the real source
	for i := 0; i < 200; i++ {
		if got := Pick(5, Rand()); got == 5 || got < MinIndex || got > MaxIndex {
			t.Fatalf("real source gave %d", got)
		}
	}
}
