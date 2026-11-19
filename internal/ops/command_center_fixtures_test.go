package ops

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Guards for the synthetic fixtures of docs/design/landsraad-command-center/fixtures (the source audit of the Landsraad and Specialization rework, M1) and the pure invariants the
// rework relies on: the twelve lines of the board, the specialization XP table, the track numbers, the House lines, the tier states and the request shapes the board sends. Nothing
// here is a new feature: every helper called already exists, and the fixtures are made up (no save, no account, no character).
//
// The Sysselraad evaluator itself is a later milestone. These tests do not decide a winner: they re-derive only the geometry facts (counts, the status of each of the twelve lines,
// which lines are complete or one House away) with a second, test-local implementation, so a fixture whose hand-written expectation is wrong fails here.

const ccFixtures = "../../docs/design/landsraad-command-center/fixtures/"

func ccRead(t *testing.T, name string, strict bool, into any) {
	t.Helper()
	b, err := os.ReadFile(ccFixtures + name)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	if strict {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(into); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if dec.More() {
		t.Fatalf("%s: more than one JSON value", name)
	}
}

// ---------------------------------------------------------------- the board and the twelve lines

type ccLine struct {
	ID      string `json:"id"`
	Indexes []int  `json:"indexes"`
}

// ccLines is the geometry of the 5 x 5 board: board_index = 5 * row + column; five rows, five columns and the two diagonals.
func ccLines() []ccLine {
	var out []ccLine
	for r := 0; r < 5; r++ {
		l := ccLine{ID: fmt.Sprintf("row%d", r)}
		for c := 0; c < 5; c++ {
			l.Indexes = append(l.Indexes, 5*r+c)
		}
		out = append(out, l)
	}
	for c := 0; c < 5; c++ {
		l := ccLine{ID: fmt.Sprintf("col%d", c)}
		for r := 0; r < 5; r++ {
			l.Indexes = append(l.Indexes, 5*r+c)
		}
		out = append(out, l)
	}
	main, anti := ccLine{ID: "diag-main"}, ccLine{ID: "diag-anti"}
	for i := 0; i < 5; i++ {
		main.Indexes = append(main.Indexes, 6*i)
		anti.Indexes = append(anti.Indexes, 4+4*i)
	}
	return append(out, main, anti)
}

// ccStatus is the status of one line for a board of owner letters (A, H, O open, ? unreadable).
func ccStatus(owners string, idx []int) string {
	var a, h, o, u int
	for _, i := range idx {
		switch owners[i] {
		case 'A':
			a++
		case 'H':
			h++
		case 'O':
			o++
		default:
			u++
		}
	}
	switch {
	case u > 0:
		return "unknown"
	case a > 0 && h > 0:
		return "blocked"
	case a == 5:
		return "complete-A"
	case h == 5:
		return "complete-H"
	case a == 4 && o == 1:
		return "one-away-A"
	case h == 4 && o == 1:
		return "one-away-H"
	case a > 0:
		return "open-A"
	case h > 0:
		return "open-H"
	}
	return "open-any"
}

// ccOwnerOf is the strict reading of a task row: completed with a Landsraad faction as winner is that faction's, not completed with no winner is open, anything else is not readable.
func ccOwnerOf(completed int, winner *int64) byte {
	switch {
	case completed == 1 && winner != nil && *winner == 1:
		return 'A'
	case completed == 1 && winner != nil && *winner == 2:
		return 'H'
	case completed == 0 && winner == nil:
		return 'O'
	}
	return '?'
}

type ccTask struct {
	BoardIndex int    `json:"board_index"`
	HouseName  string `json:"house_name"`
	Completed  int    `json:"completed"`
	Winning    *int64 `json:"winning_faction_id"`
	Sysselraad int    `json:"sysselraad"`
}

type ccCounts struct {
	A       int `json:"A"`
	H       int `json:"H"`
	O       int `json:"O"`
	Unknown int `json:"unknown"`
}

type ccBoardCase struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Owners string `json:"owners"`
	Source struct {
		Phase          string  `json:"phase"`
		CutoffVerified bool    `json:"cutoffVerified"`
		CutoffOwners   *string `json:"cutoffOwners"`
	} `json:"source"`
	Tasks    []ccTask `json:"tasks"`
	Expected struct {
		BoardValid    bool      `json:"boardValid"`
		Counts        *ccCounts `json:"counts"`
		LineStatus    []string  `json:"lineStatus"`
		CompleteLines []string  `json:"completeLines"`
		OneAwayLines  []struct {
			Line         string `json:"line"`
			Owner        string `json:"owner"`
			MissingIndex int    `json:"missing_index"`
		} `json:"oneAwayLines"`
		CountLeader      *string `json:"countLeader"`
		Margin           int     `json:"margin"`
		ThresholdReached bool    `json:"thresholdReached"`
		MajorityAtCutoff struct {
			Winner       *string `json:"winner"`
			Verified     bool    `json:"verified"`
			CutoffCounts *struct {
				A int `json:"A"`
				H int `json:"H"`
				O int `json:"O"`
			} `json:"cutoffCounts"`
			Reason string `json:"reason"`
		} `json:"majorityAtCutoff"`
		Verdict              string  `json:"verdict"`
		Winner               *string `json:"winner"`
		Reason               string  `json:"reason"`
		MajorityAlsoRecorded *bool   `json:"majorityAlsoRecorded"`
		InvalidBecause       string  `json:"invalidBecause"`
	} `json:"expected"`
}

type ccBoardFile struct {
	Meta  map[string]any `json:"_fixture"`
	Lines []ccLine       `json:"lines"`
	Cases []ccBoardCase  `json:"cases"`
}

func ccBoards(t *testing.T) ccBoardFile {
	t.Helper()
	var f ccBoardFile
	ccRead(t, "board-victory-cases.json", true, &f)
	return f
}

func ccCount(owners string) ccCounts {
	var c ccCounts
	for _, r := range owners {
		switch r {
		case 'A':
			c.A++
		case 'H':
			c.H++
		case 'O':
			c.O++
		default:
			c.Unknown++
		}
	}
	return c
}

func TestCommandCenterBoardHasTwelveFixedLines(t *testing.T) {
	f := ccBoards(t)
	want := ccLines()
	if len(want) != 12 {
		t.Fatalf("the geometry has %d lines, not 12", len(want))
	}
	if !reflect.DeepEqual(f.Lines, want) {
		t.Errorf("the fixture's lines are not the rows, columns and diagonals of a 5 x 5 board:\n got %v\nwant %v", f.Lines, want)
	}
	seen := map[string]bool{}
	times := make([]int, 25)
	for _, l := range want {
		if seen[l.ID] {
			t.Errorf("line %s twice", l.ID)
		}
		seen[l.ID] = true
		in := map[int]bool{}
		for _, i := range l.Indexes {
			if i < 0 || i > 24 || in[i] {
				t.Errorf("line %s: index %d is out of 0..24 or repeated", l.ID, i)
			}
			in[i] = true
			times[i]++
		}
		if len(in) != 5 {
			t.Errorf("line %s has %d distinct indexes, want 5", l.ID, len(in))
		}
	}
	// a cell is in its row and its column, and on a diagonal when it is on one: the centre (12) is on four lines, the other diagonal cells on three, every other cell on two
	for i, n := range times {
		r, c := i/5, i%5
		w := 2
		if r == c {
			w++
		}
		if r+c == 4 {
			w++
		}
		if n != w {
			t.Errorf("cell %d is on %d lines, want %d", i, n, w)
		}
	}
	if times[12] != 4 {
		t.Errorf("the centre cell is on %d lines, want 4", times[12])
	}
}

func TestCommandCenterBoardCasesAgreeWithTheGeometry(t *testing.T) {
	f := ccBoards(t)
	lines := ccLines()
	ids := map[string]bool{}
	completeSeen := map[string]bool{}
	var majorityBelow13, verifiedFalseLeader, invalids int
	for _, c := range f.Cases {
		if ids[c.ID] {
			t.Errorf("case %s twice", c.ID)
		}
		ids[c.ID] = true
		if len(c.Owners) != 25 || strings.Trim(c.Owners, "AHO?") != "" {
			t.Errorf("%s: owners %q is not 25 letters of A, H, O, ?", c.ID, c.Owners)
			continue
		}
		valid := len(c.Tasks) == 0 || ccValidBoard(c.Tasks)
		if len(c.Tasks) == 0 && strings.Contains(c.ID, "duplicate") {
			t.Errorf("%s: a case about a broken board must list its tasks", c.ID)
		}
		if c.Expected.BoardValid != valid {
			t.Errorf("%s: boardValid is %v, the tasks say %v", c.ID, c.Expected.BoardValid, valid)
		}
		if len(c.Tasks) == 25 && valid {
			// the letters are what the rows say, by board_index
			got := make([]byte, 25)
			for _, tk := range c.Tasks {
				got[tk.BoardIndex] = ccOwnerOf(tk.Completed, tk.Winning)
			}
			if string(got) != c.Owners {
				t.Errorf("%s: the rows read as %s, owners says %s", c.ID, got, c.Owners)
			}
		}
		e := c.Expected
		if !valid {
			invalids++
			if e.Counts != nil || e.Verdict != "unknown" || e.Winner != nil || e.InvalidBecause == "" {
				t.Errorf("%s: a board that is not valid has no counts, no winner and the verdict unknown, with its reason", c.ID)
			}
			for _, s := range e.LineStatus {
				if s != "unknown" {
					t.Errorf("%s: a line of a board that is not valid is %s, want unknown", c.ID, s)
				}
			}
			if len(e.CompleteLines) != 0 || len(e.OneAwayLines) != 0 {
				t.Errorf("%s: a board that is not valid claims lines", c.ID)
			}
			continue
		}
		cnt := ccCount(c.Owners)
		if e.Counts == nil || *e.Counts != cnt {
			t.Errorf("%s: counts %+v, the letters give %+v", c.ID, e.Counts, cnt)
		}
		if cnt.A+cnt.H+cnt.O+cnt.Unknown != 25 {
			t.Errorf("%s: the counts do not add up to 25", c.ID)
		}
		if len(e.LineStatus) != 12 {
			t.Fatalf("%s: %d line statuses, want 12", c.ID, len(e.LineStatus))
		}
		var complete []string
		type away struct {
			line, owner string
			missing     int
		}
		var oneAway []away
		for i, l := range lines {
			s := ccStatus(c.Owners, l.Indexes)
			if e.LineStatus[i] != s {
				t.Errorf("%s: line %s is %s, the letters give %s", c.ID, l.ID, e.LineStatus[i], s)
			}
			if strings.HasPrefix(s, "complete") {
				complete = append(complete, l.ID)
				completeSeen[l.ID] = true
			}
			if strings.HasPrefix(s, "one-away") {
				m := -1
				for _, ix := range l.Indexes {
					if c.Owners[ix] == 'O' {
						m = ix
					}
				}
				oneAway = append(oneAway, away{l.ID, s[len(s)-1:], m})
			}
		}
		if strings.Join(complete, ",") != strings.Join(e.CompleteLines, ",") {
			t.Errorf("%s: complete lines %v, the letters give %v", c.ID, e.CompleteLines, complete)
		}
		if len(oneAway) != len(e.OneAwayLines) {
			t.Errorf("%s: %d lines one House away, the letters give %d", c.ID, len(e.OneAwayLines), len(oneAway))
		} else {
			for i, x := range oneAway {
				g := e.OneAwayLines[i]
				if g.Line != x.line || g.Owner != x.owner || g.MissingIndex != x.missing {
					t.Errorf("%s: one-away %+v, the letters give %+v", c.ID, g, x)
				}
			}
		}
		var leader *string
		margin := 0
		if cnt.A != cnt.H {
			l := "A"
			if cnt.H > cnt.A {
				l = "H"
			}
			leader, margin = &l, cnt.A-cnt.H
			if margin < 0 {
				margin = -margin
			}
		}
		if (leader == nil) != (e.CountLeader == nil) || (leader != nil && *leader != *e.CountLeader) || margin != e.Margin {
			t.Errorf("%s: leader/margin %v/%d, the letters give %v/%d", c.ID, e.CountLeader, e.Margin, leader, margin)
		}
		best := cnt.A
		if cnt.H > best {
			best = cnt.H
		}
		if e.ThresholdReached != (leader != nil && best >= 13) {
			t.Errorf("%s: thresholdReached %v, the counts give %v", c.ID, e.ThresholdReached, leader != nil && best >= 13)
		}
		// the tug track has 25 seats: A fills from the left, H from the right, the 13th is the middle one, and the two never overlap
		if cnt.A+cnt.H > 25 {
			t.Errorf("%s: more claimed Houses than seats", c.ID)
		}

		// the cutoff evidence
		m := e.MajorityAtCutoff
		if m.Verified != c.Source.CutoffVerified {
			t.Errorf("%s: majorityAtCutoff.verified %v, source says %v", c.ID, m.Verified, c.Source.CutoffVerified)
		}
		if c.Source.CutoffVerified != (c.Source.CutoffOwners != nil) {
			t.Errorf("%s: a verified cutoff has a snapshot and an unverified one has none", c.ID)
		}
		if !m.Verified && m.Winner != nil {
			t.Errorf("%s: a majority without a verified cutoff", c.ID)
		}
		if m.Verified {
			k := ccCount(*c.Source.CutoffOwners)
			if m.CutoffCounts == nil || m.CutoffCounts.A != k.A || m.CutoffCounts.H != k.H || m.CutoffCounts.O != k.O {
				t.Errorf("%s: cutoff counts %+v, the snapshot gives %+v", c.ID, m.CutoffCounts, k)
			}
			var w *string
			switch {
			case k.A > k.H:
				x := "A"
				w = &x
			case k.H > k.A:
				x := "H"
				w = &x
			}
			if (w == nil) != (m.Winner == nil) || (w != nil && *w != *m.Winner) {
				t.Errorf("%s: cutoff winner %v, the snapshot gives %v", c.ID, m.Winner, w)
			}
			if w != nil && k.A+k.H > 0 && max(k.A, k.H) < 13 {
				majorityBelow13++
			}
		}

		// the verdict facts that must hold whatever the evaluator's rules are
		hasWinner := e.Winner != nil
		if hasWinner != (e.Verdict == "line_confirmed" || e.Verdict == "majority_confirmed") {
			t.Errorf("%s: verdict %s with winner %v", c.ID, e.Verdict, e.Winner)
		}
		if e.Verdict == "majority_confirmed" && !m.Verified {
			t.Errorf("%s: a majority confirmed without a verified cutoff", c.ID)
		}
		if e.Verdict == "line_confirmed" && len(e.CompleteLines) == 0 {
			t.Errorf("%s: a line confirmed with no complete line", c.ID)
		}
		if strings.HasPrefix(strings.Join(e.LineStatus, ","), "") && containsBoth(e.LineStatus, "complete-A", "complete-H") && e.Verdict != "conflict" {
			t.Errorf("%s: both factions hold a line and the verdict is %s", c.ID, e.Verdict)
		}
		if !m.Verified && leader != nil && len(e.CompleteLines) == 0 && cnt.Unknown == 0 {
			verifiedFalseLeader++
			if e.Verdict != "pending_vote" {
				t.Errorf("%s: a lead before a verified cutoff is pending_vote, not %s", c.ID, e.Verdict)
			}
		}
	}
	// what the set must cover (the milestone's list)
	for _, l := range lines {
		if !completeSeen[l.ID] {
			t.Errorf("no case holds %s as a complete line", l.ID)
		}
	}
	for _, id := range []string{"all-open-0-0-25", "atreides-12-harkonnen-4-open-9", "atreides-13-harkonnen-12-before-vote", "atreides-12-harkonnen-13-before-vote", "harkonnen-row2-low-total",
		"atreides-col3-low-total", "atreides-main-diagonal", "harkonnen-anti-diagonal", "both-factions-complete-lines", "duplicate-board-index", "missing-task-24-of-25",
		"no-term-tasks-of-all-terms", "unknown-owner-faction-3", "majority-8-6-at-verified-cutoff", "majority-8-6-cutoff-not-verified", "sysselraad-flag-set-no-evidence"} {
		if !ids[id] {
			t.Errorf("the case %s is missing", id)
		}
	}
	if majorityBelow13 == 0 || verifiedFalseLeader == 0 || invalids < 3 {
		t.Errorf("the set needs a verified majority below 13 (%d), an unverified lead (%d) and broken boards (%d)", majorityBelow13, verifiedFalseLeader, invalids)
	}
}

func containsBoth(list []string, a, b string) bool {
	var x, y bool
	for _, s := range list {
		x, y = x || s == a, y || s == b
	}
	return x && y
}

// ccValidBoard is the rule of a complete board: 25 tasks, board_index a whole 0..24 and each once, each House once.
func ccValidBoard(tasks []ccTask) bool {
	if len(tasks) != 25 {
		return false
	}
	idx, houses := map[int]bool{}, map[string]bool{}
	for _, tk := range tasks {
		if tk.BoardIndex < 0 || tk.BoardIndex > 24 || idx[tk.BoardIndex] {
			return false
		}
		idx[tk.BoardIndex] = true
		houses[tk.HouseName] = true
	}
	return len(houses) == 25 || (len(houses) == 24 && hasDuplicateHouses(tasks))
}

func hasDuplicateHouses(tasks []ccTask) bool {
	seen := map[string]bool{}
	for _, tk := range tasks {
		if seen[tk.HouseName] {
			return true
		}
		seen[tk.HouseName] = true
	}
	return false
}

// ccTug is the 25 seats of the voting track for a count of Atreides and of Harkonnen Houses: Atreides fills from the left, Harkonnen from the right.
func ccTug(a, h int) []byte {
	seats := make([]byte, 25)
	for i := range seats {
		seats[i] = '.'
	}
	for i := 0; i < a; i++ {
		seats[i] = 'A'
	}
	for i := 0; i < h; i++ {
		seats[24-i] = 'H'
	}
	return seats
}

func TestCommandCenterTugTrackHasTwentyFiveSeatsWithOneMiddleSeat(t *testing.T) {
	for a := 0; a <= 25; a++ {
		for h := 0; a+h <= 25; h++ {
			s := ccTug(a, h)
			if len(s) != 25 {
				t.Fatalf("%d seats", len(s))
			}
			var na, nh int
			for i, c := range s {
				switch c {
				case 'A':
					na++
					if i >= a {
						t.Fatalf("a=%d h=%d: A outside its run", a, h)
					}
				case 'H':
					nh++
					if i < 25-h {
						t.Fatalf("a=%d h=%d: H outside its run", a, h)
					}
				}
			}
			if na != a || nh != h {
				t.Fatalf("a=%d h=%d: counted %d and %d", a, h, na, nh)
			}
			// the 13th seat (index 12, the gold one) is Atreides' from 13 Houses and Harkonnen's from 13 Houses, never both, empty otherwise
			mid := s[12]
			switch {
			case a >= 13 && mid != 'A', h >= 13 && mid != 'H', a < 13 && h < 13 && mid != '.':
				t.Fatalf("a=%d h=%d: the middle seat is %c", a, h, mid)
			}
		}
	}
}

// ---------------------------------------------------------------- GET /api/landsraad, synthetic

type ccAPIReward struct {
	TaskID          int64   `json:"task_id"`
	Threshold       float64 `json:"threshold"`
	TemplateID      string  `json:"template_id"`
	Amount          float64 `json:"amount"`
	Reached         bool    `json:"reached"`
	RewardState     string  `json:"reward_state"`
	Delivered       bool    `json:"delivered"`
	Swatch          bool    `json:"swatch"`
	ShownTemplateID string  `json:"shown_template_id"`
	ShownAmount     float64 `json:"shown_amount"`
}

type ccAPITask struct {
	ID             int64   `json:"id"`
	BoardIndex     int     `json:"board_index"`
	HouseName      string  `json:"house_name"`
	GoalAmount     float64 `json:"goal_amount"`
	Completed      int     `json:"completed"`
	Winning        *int64  `json:"winning_faction_id"`
	Line           string  `json:"line"`
	MyContribution float64 `json:"my_contribution"`
	SwatchTemplate string  `json:"swatch_template_id"`
	Contributions  []struct {
		FactionID int64   `json:"faction_id"`
		Amount    float64 `json:"amount"`
	} `json:"contributions"`
	PlayerContributions []struct {
		PlayerID int64   `json:"player_id"`
		Amount   float64 `json:"amount"`
		Mine     bool    `json:"mine"`
	} `json:"player_contributions"`
	Rewards []ccAPIReward `json:"rewards"`
}

func ccKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ccWithout drops the keys that are not part of what the server sends: the fixture's own notes.
func ccWithout(keys []string, drop ...string) []string {
	var out []string
	for _, k := range keys {
		skip := false
		for _, d := range drop {
			skip = skip || k == d
		}
		if !skip {
			out = append(out, k)
		}
	}
	return out
}

func ccAsMap(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func ccList(t *testing.T, m map[string]any, key string) []map[string]any {
	t.Helper()
	raw, ok := m[key].([]any)
	if !ok || len(raw) == 0 {
		t.Fatalf("%s is not a non-empty list", key)
	}
	out := make([]map[string]any, len(raw))
	for i, x := range raw {
		out[i], ok = x.(map[string]any)
		if !ok {
			t.Fatalf("%s[%d] is not an object", key, i)
		}
	}
	return out
}

func TestCommandCenterLandsraadFixtureIsTheBoardTheServerAnswers(t *testing.T) {
	var raw map[string]any
	ccRead(t, "api-landsraad.synthetic.json", false, &raw)
	var typed struct {
		Tasks []ccAPITask `json:"tasks"`
	}
	ccRead(t, "api-landsraad.synthetic.json", false, &typed)
	boards := ccBoards(t)
	var base string
	for _, c := range boards.Cases {
		if c.ID == "atreides-12-harkonnen-4-open-9" {
			base = c.Owners
		}
	}
	if base == "" {
		t.Fatal("the case the response is built from is missing")
	}

	// 25 tasks, each position and each House once; the physical order is not the alphabetical one
	if len(typed.Tasks) != 25 {
		t.Fatalf("%d tasks, want 25", len(typed.Tasks))
	}
	owners := make([]byte, 25)
	pos, houses := map[int]bool{}, map[string]bool{}
	byName := []string{}
	for i, tk := range typed.Tasks {
		if tk.BoardIndex != i {
			t.Errorf("task %d is at board_index %d: the server orders by board_index", i, tk.BoardIndex)
		}
		if tk.BoardIndex < 0 || tk.BoardIndex > 24 || pos[tk.BoardIndex] {
			t.Errorf("board_index %d is out of range or repeated", tk.BoardIndex)
		}
		pos[tk.BoardIndex] = true
		if houses[tk.HouseName] || !strings.HasPrefix(tk.HouseName, "DA_House") {
			t.Errorf("House %q repeated or not a DA_House name", tk.HouseName)
		}
		houses[tk.HouseName] = true
		byName = append(byName, tk.HouseName)
		owners[tk.BoardIndex] = ccOwnerOf(tk.Completed, tk.Winning)
		if tk.Line == "" || tk.Line != lineOfHouse(tk.HouseName) {
			t.Errorf("%s: line %q, lineOfHouse says %q", tk.HouseName, tk.Line, lineOfHouse(tk.HouseName))
		}
	}
	if string(owners) != base {
		t.Errorf("the owners of the response are %s, the board case says %s", owners, base)
	}
	if sort.StringsAreSorted(byName) {
		t.Error("the fixture's board order is alphabetical, so it cannot show that the physical order differs from the list")
	}

	// tiers: the four states are all there, and every row is consistent with the contribution
	states := map[string]int{}
	var nulls int
	for _, tk := range typed.Tasks {
		if tk.Contributions == nil || tk.PlayerContributions == nil || tk.Rewards == nil {
			nulls++ // the server marshals an empty list of a task as null
		}
		for _, r := range tk.Rewards {
			states[r.RewardState]++
			if r.Reached != (tk.MyContribution >= r.Threshold) {
				t.Errorf("%s tier %v: reached %v with contribution %v", tk.HouseName, r.Threshold, r.Reached, tk.MyContribution)
			}
			if r.Reached == (r.RewardState == "not_reached") {
				t.Errorf("%s tier %v: reached %v but state %s", tk.HouseName, r.Threshold, r.Reached, r.RewardState)
			}
			if r.Delivered != (r.RewardState == "waiting" || r.RewardState == "delivered") {
				t.Errorf("%s tier %v: delivered %v with state %s", tk.HouseName, r.Threshold, r.Delivered, r.RewardState)
			}
			if r.TaskID != tk.ID {
				t.Errorf("%s: a reward row of task %d", tk.HouseName, r.TaskID)
			}
			if r.Swatch && r.ShownTemplateID != tk.SwatchTemplate {
				t.Errorf("%s: a swatch tier shows %s, not the swatch %s", tk.HouseName, r.ShownTemplateID, tk.SwatchTemplate)
			}
			if !r.Swatch && (r.ShownTemplateID != r.TemplateID || r.ShownAmount != r.Amount) {
				t.Errorf("%s: a tier that is not the swatch shows another item than its own", tk.HouseName)
			}
		}
	}
	for _, s := range []string{"not_reached", "waiting", "delivered", "no_row"} {
		if states[s] == 0 {
			t.Errorf("no reward row is %s", s)
		}
	}
	if nulls == 0 {
		t.Error("no task has a null list: the fixture would not exercise the null that Go marshals for an empty list")
	}

	// the shape: the keys of every object are the keys the real read gives (a drift in the server's answer fails here, before a design is built on a field that is gone)
	o := newerOps(t, `delete from landsraad_decree_rotation; delete from landsraad_decrees;
		insert into landsraad_decrees(id, decree_name, disabled, weight) values (1, 'SpecialVendorActive', 0, 1);
		insert into landsraad_decree_rotation(decree_id) values (1);
		update landsraad_decree_term set active_decree_id=1, elected_decree_id=1 where term_id=7;`)
	res, err := o.Landsraad()
	if err != nil {
		t.Fatal(err)
	}
	real := ccAsMap(t, res)
	if got, want := ccWithout(ccKeys(raw), "_fixture", "grant"), ccKeys(real); !reflect.DeepEqual(got, want) {
		t.Errorf("top-level keys %v, the server gives %v (grant is added by the route, internal/web/server.go)", got, want)
	}
	for _, k := range []string{"term", "character"} {
		fm, rm := raw[k].(map[string]any), real[k].(map[string]any)
		if !reflect.DeepEqual(ccKeys(fm), ccKeys(rm)) {
			t.Errorf("%s: keys %v, the server gives %v", k, ccKeys(fm), ccKeys(rm))
		}
	}
	for _, k := range []string{"decrees", "terms", "factions", "rotation", "earlier_terms"} {
		f0, r0 := ccList(t, raw, k)[0], ccList(t, real, k)[0]
		if !reflect.DeepEqual(ccKeys(f0), ccKeys(r0)) {
			t.Errorf("%s[0]: keys %v, the server gives %v", k, ccKeys(f0), ccKeys(r0))
		}
	}
	fh, rh := ccList(t, raw, "earlier_terms")[0]["houses"].([]any)[0].(map[string]any), ccList(t, real, "earlier_terms")[0]["houses"].([]any)[0].(map[string]any)
	if !reflect.DeepEqual(ccKeys(fh), ccKeys(rh)) {
		t.Errorf("earlier_terms houses: keys %v, the server gives %v", ccKeys(fh), ccKeys(rh))
	}
	var realWithSwatch map[string]any
	for _, tk := range ccList(t, real, "tasks") {
		if _, has := tk["swatch_template_id"]; has && tk["rewards"] != nil {
			realWithSwatch = tk
		}
	}
	if realWithSwatch == nil {
		t.Fatal("the test save has no task with a swatch")
	}
	ftasks := ccList(t, raw, "tasks")
	for _, ft := range ftasks {
		want := ccKeys(realWithSwatch)
		if _, has := ft["swatch_template_id"]; !has {
			want = ccWithout(want, "swatch_template_id")
		}
		if got := ccKeys(ft); !reflect.DeepEqual(got, want) {
			t.Errorf("task %v: keys %v, the server gives %v", ft["id"], got, want)
		}
	}
	freward := ftasks[0]["rewards"].([]any)[0].(map[string]any)
	rreward := realWithSwatch["rewards"].([]any)[0].(map[string]any)
	if !reflect.DeepEqual(ccKeys(freward), ccKeys(rreward)) {
		t.Errorf("reward row: keys %v, the server gives %v", ccKeys(freward), ccKeys(rreward))
	}
	fpc, rpc := ftasks[0]["player_contributions"], realWithSwatch["player_contributions"]
	if fpc != nil && rpc != nil && !reflect.DeepEqual(ccKeys(fpc.([]any)[0].(map[string]any)), ccKeys(rpc.([]any)[0].(map[string]any))) {
		t.Errorf("player_contributions row: keys differ from the server's")
	}
	for _, ft := range ftasks {
		loc := ft["location"].(map[string]any)
		if loc["known"] == false {
			if len(loc) != 1 {
				t.Errorf("task %v: an unknown place carries only known:false", ft["id"])
			}
			continue
		}
		allowed := map[string]bool{"known": true, "map": true, "area": true, "place": true, "map_key": true, "cell": true, "unconfirmed": true, "x": true, "y": true, "from_you_cm": true}
		for k := range loc {
			if !allowed[k] {
				t.Errorf("task %v: location key %s is not one the server sends", ft["id"], k)
			}
		}
	}
	g := raw["grant"].(map[string]any)
	if got, want := ccKeys(g), ccKeys(ccAsMap(t, mapOfGrant())); !reflect.DeepEqual(got, want) {
		t.Errorf("grant keys %v, the server's preview has %v plus room", got, want)
	}
}

// mapOfGrant is the preview the route sends (internal/web/server.go adds room): with the room key the fixture carries.
func mapOfGrant() map[string]any {
	p := DefaultGrantSettings().Preview()
	p["room"] = map[string]int64{}
	return p
}

// ---------------------------------------------------------------- tier states and the request shapes of the board

func TestCommandCenterTierStateClassifiesWaitingDeliveredAndNoRow(t *testing.T) {
	const termStart = 1000.0
	row := func(amount, updated float64) save.Row { return save.Row{"amount": amount, "last_updated": updated} }
	cases := []struct {
		name    string
		reached bool
		row     save.Row
		hasRow  bool
		want    string
	}{
		{"below the threshold, no row", false, nil, false, "not_reached"},
		{"below the threshold, a waiting row of this term is not an earned tier", false, row(5, 1500), true, "not_reached"},
		{"reached, no row: ambiguous, not collected and not waiting", true, nil, false, "no_row"},
		{"reached, a row of an earlier term only", true, row(0, 999), true, "no_row"},
		{"reached, a row written at the start of the term counts for the term", true, row(0, 1000), true, "delivered"},
		{"reached, an amount above 0 waits at the house representative", true, row(25, 1500), true, "waiting"},
		{"reached, amount 0 was collected", true, row(0, 1500), true, "delivered"},
	}
	for _, c := range cases {
		if got := tierState(c.reached, c.row, c.hasRow, termStart); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

func TestCommandCenterRequestShapesTheBoardSends(t *testing.T) {
	parse := func(body string) ([]applyItem, error) {
		var a Args
		if err := json.Unmarshal([]byte(body), &a); err != nil {
			t.Fatal(err)
		}
		return parseApplyItems(a)
	}
	one := func(body string) applyItem {
		t.Helper()
		items, err := parse(body)
		if err != nil || len(items) != 1 {
			t.Fatalf("%s: %v %v", body, items, err)
		}
		return items[0]
	}
	// what lbItem (internal/web/static/app.js) sends for each click, with the guards (was_*) the page always adds
	tier := one(`{"houses":[{"task_id":9001,"was_mine":1300,"was_done":false,"was_win":null,"was_events":0,"mine_to":2400}]}`)
	if tier.mineTo == nil || *tier.mineTo != 2400 || tier.lower || tier.completeFor != nil || tier.reopen || !tier.stateShown || tier.wasEvents == nil {
		t.Errorf("a tier click: %+v", tier)
	}
	removal := one(`{"houses":[{"task_id":9001,"was_mine":2400,"was_done":false,"was_win":null,"was_events":0,"mine_to":1200,"lower":true}]}`)
	if removal.mineTo == nil || *removal.mineTo != 1200 || !removal.lower {
		t.Errorf("a tier removal: %+v", removal)
	}
	remove0 := one(`{"houses":[{"task_id":9001,"was_mine":1200,"was_done":false,"was_win":null,"mine_to":0,"lower":true}]}`)
	if remove0.mineTo == nil || *remove0.mineTo != 0 || !remove0.lower || remove0.wasEvents != nil {
		t.Errorf("the removal of tier 1: %+v", remove0)
	}
	fillGoal := one(`{"houses":[{"task_id":9001,"was_mine":0,"was_done":false,"was_win":null,"was_events":0,"mine_to":6000,"complete_for":1}]}`)
	if fillGoal.mineTo == nil || fillGoal.completeFor == nil || *fillGoal.completeFor != 1 {
		t.Errorf("Fill faction goal: %+v", fillGoal)
	}
	complete := one(`{"houses":[{"task_id":9001,"was_mine":0,"was_done":false,"was_win":null,"complete_for":2}]}`)
	if complete.mineTo != nil || complete.completeFor == nil || *complete.completeFor != 2 {
		t.Errorf("Complete: %+v", complete)
	}
	reopen := one(`{"houses":[{"task_id":9001,"was_mine":0,"was_done":true,"was_win":2,"reopen":true}]}`)
	if !reopen.reopen || !reopen.wasDone || reopen.wasWin != int64(2) {
		t.Errorf("Reopen: %+v", reopen)
	}
	// a bulk request is several houses in one body; each is checked on its own and the whole is refused for any one that is not valid
	bulk, err := parse(`{"houses":[{"task_id":1,"mine_to":1200},{"task_id":2,"complete_for":1},{"task_id":3,"reopen":true}]}`)
	if err != nil || len(bulk) != 3 {
		t.Errorf("a bulk request: %v %v", bulk, err)
	}
	for name, body := range map[string]string{
		"a House twice":                 `{"houses":[{"task_id":1,"mine_to":1200},{"task_id":1,"mine_to":2400}]}`,
		"no change in a House":          `{"houses":[{"task_id":1}]}`,
		"complete and reopen":           `{"houses":[{"task_id":1,"complete_for":1,"reopen":true}]}`,
		"lower as text":                 `{"houses":[{"task_id":1,"mine_to":0,"lower":"true"}]}`,
		"a negative contribution":       `{"houses":[{"task_id":1,"mine_to":-1}]}`,
		"a fractional task id":          `{"houses":[{"task_id":1.5,"mine_to":1200}]}`,
		"a House that is not an object": `{"houses":[7]}`,
	} {
		if _, err := parse(body); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	many := make([]string, maxFillHouses+1)
	for i := range many {
		many[i] = fmt.Sprintf(`{"task_id":%d,"mine_to":1200}`, i+1)
	}
	if _, err := parse(`{"houses":[` + strings.Join(many, ",") + `]}`); err == nil {
		t.Errorf("a request of %d Houses was accepted; one request carries at most %d", len(many), maxFillHouses)
	}
}

// ---------------------------------------------------------------- the House lines and the art that names them

func TestCommandCenterHouseLinesAreFiveHousesOnEachOfTheFiveLines(t *testing.T) {
	if len(houseLine) != 25 {
		t.Fatalf("%d Houses have a line, want 25", len(houseLine))
	}
	per := map[string]int{}
	for h, l := range houseLine {
		per[l]++
		if h != strings.TrimSpace(h) || strings.HasPrefix(h, "DA_House") {
			t.Errorf("%q: the table is keyed by the name without the DA_House prefix", h)
		}
		if got := lineOfHouse("DA_House" + h); got != l {
			t.Errorf("lineOfHouse(DA_House%s) = %q, want %q", h, got, l)
		}
	}
	if len(per) != len(specializationLines) {
		t.Errorf("Houses are on %d lines, want %d", len(per), len(specializationLines))
	}
	for _, l := range specializationLines {
		if per[l] != 5 {
			t.Errorf("line %s has %d Houses, want 5", l, per[l])
		}
	}
	for _, unknown := range []string{"", "DA_House", "DA_HouseNobody", "Alexin-extra"} {
		if got := lineOfHouse(unknown); got != "" {
			t.Errorf("lineOfHouse(%q) = %q, want none", unknown, got)
		}
	}
}

func TestCommandCenterArtFilesNameTheHousesAndLinesTheEditorKnows(t *testing.T) {
	const art, runtime = "../../docs/art/landsraad/", "../web/static/houses/"
	// the save's spelling of one House differs from the picture's: the save and the lines table say Mikkarol, the game's own picture file says mikarrol
	alias := map[string]string{"mikkarol": "mikarrol"}
	for h := range houseLine {
		name := strings.ToLower(h)
		ref := name
		if a, ok := alias[name]; ok {
			ref = a
		}
		if _, err := os.Stat(art + "houses/" + ref + ".png"); err != nil {
			t.Errorf("House %s: no reference picture houses/%s.png: %v", h, ref, err)
		}
		if _, err := os.Stat(runtime + name + ".webp"); err != nil {
			t.Errorf("House %s: no served picture %s.webp: %v", h, name, err)
		}
	}
	for _, l := range specializationLines {
		for _, dir := range []string{"lanes", "banners"} {
			if _, err := os.Stat(art + dir + "/" + strings.ToLower(l) + ".png"); err != nil {
				t.Errorf("line %s: no %s picture: %v", l, dir, err)
			}
		}
	}
}

// ---------------------------------------------------------------- the XP table and the track numbers

func TestCommandCenterXpTableHasOneHundredOneLevelsAndNeverGuessesALevel(t *testing.T) {
	if len(specCumulativeXP) != 101 {
		t.Fatalf("%d thresholds, want 101 (level 0 to 100)", len(specCumulativeXP))
	}
	if specCumulativeXP[0] != 0 || specCumulativeXP[100] != 44182 || maxSpecXP != 44182 {
		t.Errorf("the table runs from %d to %d with maxSpecXP %d, want 0 to 44182", specCumulativeXP[0], specCumulativeXP[100], maxSpecXP)
	}
	for i := 1; i < len(specCumulativeXP); i++ {
		if specCumulativeXP[i] <= specCumulativeXP[i-1] {
			t.Errorf("level %d starts at %d, not above level %d at %d", i, specCumulativeXP[i], i-1, specCumulativeXP[i-1])
		}
	}
	for i, x := range specCumulativeXP {
		if got := specLevel(x); got != float64(i) {
			t.Errorf("specLevel(%d) = %v, want exactly %d", x, got, i)
		}
	}
	last := -1.0
	for xp := int64(0); xp <= maxSpecXP; xp++ {
		l := specLevel(xp)
		if l < last {
			t.Fatalf("level falls from %v to %v at %d XP", last, l, xp)
		}
		last = l
	}
	if l := specLevel(1); !(l > 0 && l < 1) {
		t.Errorf("1 XP is level %v, want a fraction of the first level", l)
	}
	if l := specLevel(maxSpecXP - 1); !(l > 99 && l < 100) {
		t.Errorf("one XP below the cap is level %v, want between 99 and 100", l)
	}
	if specLevel(-5) != 0 || specLevel(maxSpecXP+1000) != 100 {
		t.Errorf("a negative XP is level 0 and one above the cap is level 100")
	}
	// the cap is not a straight percentage: XP over cap is not the level
	if lin := float64(22091) / float64(maxSpecXP) * 100; math.Abs(specLevel(22091)-lin) < 1 {
		t.Errorf("the level of half the cap equals the linear guess %v: the table is not being used", lin)
	}
}

func TestCommandCenterTrackNumbersAreOneNumberPerTrack(t *testing.T) {
	if len(measuredTrackNumbers) != len(SpecTracks) {
		t.Fatalf("%d measured numbers for %d tracks", len(measuredTrackNumbers), len(SpecTracks))
	}
	seen := map[int64]string{}
	for _, tr := range SpecTracks {
		n, ok := measuredTrackNumbers[tr]
		if !ok {
			t.Errorf("no measured number for %s", tr)
		}
		if other, dup := seen[n]; dup {
			t.Errorf("%s and %s share the measured number %d", tr, other, n)
		}
		seen[n] = tr
	}
	// whatever one number a person assigns, the numbers the tab works with never put two tracks on one row, and the learned one is kept
	for _, tr := range SpecTracks {
		for n := int64(0); n <= 255; n++ {
			eff := EffectiveTrackNumbers(map[string]int64{tr: n})
			if eff[tr] != n {
				t.Fatalf("a learned %s = %d became %d", tr, n, eff[tr])
			}
			used := map[int64]string{}
			for other, num := range eff {
				if prev, dup := used[num]; dup {
					t.Fatalf("learned %s = %d: %s and %s both use %d", tr, n, prev, other, num)
				}
				used[num] = other
			}
		}
	}
	// a learned number on another track's measured number leaves that track without one (it is unresolved, never doubled)
	eff := EffectiveTrackNumbers(map[string]int64{"Crafting": 4})
	if _, has := eff["Combat"]; has || eff["Crafting"] != 4 || len(eff) != 4 {
		t.Errorf("Crafting learned as 4 gives %v: Combat (measured 4) must be left without a number", eff)
	}
	if _, err := CheckAssign(map[string]int64{"Combat": 4}, "Crafting", 4); err == nil {
		t.Error("assigning a number another track holds was accepted")
	}
	if _, err := CheckAssign(nil, "Crafting", 256); err == nil {
		t.Error("track number 256 was accepted")
	}
	if g := (GrantSettings{ContributionMultiplier: 1, SpecializationXPMultiplier: 1, TrackNumbers: map[string]int64{"Combat": 1}}); g.checkTracks() == nil {
		t.Error("a mission would write two lines to one track number and the settings were not refused")
	}
	if err := DefaultGrantSettings().checkTracks(); err != nil {
		t.Errorf("the stock settings are refused: %v", err)
	}
}

// ---------------------------------------------------------------- GET /api/player/specs, synthetic

type ccSpecCase struct {
	ID       string           `json:"id"`
	Title    string           `json:"title"`
	Learned  map[string]int64 `json:"learned"`
	SaveRows []struct {
		Number int64   `json:"number"`
		XP     int64   `json:"xp"`
		Level  float64 `json:"level"`
	} `json:"saveRows"`
	Response map[string]any `json:"response"`
	Expect   struct {
		Combat *struct {
			XpCap             bool `json:"xpCap"`
			KeystonesComplete bool `json:"keystonesComplete"`
			XpEditingEnabled  bool `json:"xpEditingEnabled"`
		} `json:"combat"`
		GlobalKeystonesEnabled *bool `json:"globalKeystonesEnabled"`
		ResetKnownOnly         *int  `json:"resetKnownOnly"`
		UnknownUnaffected      *bool `json:"unknownUnaffected"`
		KeystonesUnaffected    *bool `json:"keystonesUnaffected"`
		StoredAndDerivedDiffer *bool `json:"storedAndDerivedDiffer"`
		AutoCorrect            *bool `json:"autoCorrect"`
		LandsraadRoom          *int  `json:"landsraadRoom"`
	} `json:"expect"`
}

func TestCommandCenterSpecFixturesAreWhatSpecsAnswers(t *testing.T) {
	t.Parallel() // #767: state-free and slow, so it runs beside the others (its save, its record file and its ops are its own)
	var f struct {
		Cases []ccSpecCase `json:"cases"`
	}
	ccRead(t, "api-player-specs.cases.json", false, &f)
	if len(f.Cases) < 10 {
		t.Fatalf("%d cases", len(f.Cases))
	}
	var landsraad struct {
		Grant struct {
			Room map[string]int64 `json:"room"`
		} `json:"grant"`
	}
	ccRead(t, "api-landsraad.synthetic.json", false, &landsraad)
	for _, c := range f.Cases {
		t.Run(c.ID, func(t *testing.T) {
			o := &Ops{S: testsave.PlayerWithSQL(t, ccSpecSQL(t, c))}
			if _, err := o.S.Mutate("fixture rows", func(m *save.Mut) error {
				for _, r := range c.SaveRows {
					if err := upsertSpecInTx(m, 1, r.Number, r.XP, r.Level); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			got, err := o.Specs(EffectiveTrackNumbers(c.Learned))
			if err != nil {
				t.Fatal(err)
			}
			if gotJSON, want := ccAsMap(t, got), c.Response; !reflect.DeepEqual(gotJSON, want) {
				a, _ := json.Marshal(gotJSON)
				b, _ := json.Marshal(want)
				t.Fatalf("the fixture is not what Specs answers:\n got %s\nwant %s", a, b)
			}
			rows := ccList(t, c.Response, "rows")
			if len(rows) != 5 || c.Response["maxXp"] != float64(maxSpecXP) {
				t.Fatalf("%d rows, maxXp %v", len(rows), c.Response["maxXp"])
			}
			var combat map[string]any
			for i, r := range rows {
				if r["track"] != SpecTracks[i] {
					t.Errorf("row %d is %v, want %s", i, r["track"], SpecTracks[i])
				}
				xp, level := r["xp"].(float64), r["level"].(float64)
				// the stored level is the table's level unless the case writes a raw row on purpose
				if c.Expect.StoredAndDerivedDiffer == nil && level != specLevel(int64(xp)) {
					t.Errorf("%v: level %v for %v XP, the table says %v", r["track"], level, xp, specLevel(int64(xp)))
				}
				owned, total := r["keystoneOwned"].(float64), r["keystoneTotal"].(float64)
				if r["granted"] != (total > 0 && owned >= total) {
					t.Errorf("%v: granted %v with %v of %v keystones", r["track"], r["granted"], owned, total)
				}
				if r["track"] == "Combat" {
					combat = r
				}
			}
			if e := c.Expect.Combat; e != nil {
				xp := combat["xp"].(float64)
				if (xp >= float64(maxSpecXP)) != e.XpCap {
					t.Errorf("Combat at %v XP: xpCap expected %v", xp, e.XpCap)
				}
				if (combat["granted"] == true) != e.KeystonesComplete {
					t.Errorf("Combat keystones complete expected %v", e.KeystonesComplete)
				}
				if (combat["number"] != nil) != e.XpEditingEnabled {
					t.Errorf("Combat number %v: XP editing expected %v", combat["number"], e.XpEditingEnabled)
				}
			}
			if d := c.Expect.StoredAndDerivedDiffer; d != nil && *d {
				if combat["level"] == specLevel(int64(combat["xp"].(float64))) {
					t.Errorf("the case says the stored and the computed level differ, and they do not")
				}
			}
			if c.Expect.LandsraadRoom != nil {
				room := maxSpecXP - int64(combat["xp"].(float64))
				if int(room) != *c.Expect.LandsraadRoom || landsraad.Grant.Room["Combat"] != room {
					t.Errorf("room: %d from the XP, %d expected, %d in the Landsraad fixture", room, *c.Expect.LandsraadRoom, landsraad.Grant.Room["Combat"])
				}
			}
			if k := c.Expect.ResetKnownOnly; k != nil {
				if rc := c.Response["resetCount"].(float64); int(rc) != *k {
					t.Errorf("resetCount %v, expected %d", rc, *k)
				}
				if u, ok := c.Response["unassigned"].([]any); !ok || len(u) == 0 {
					t.Errorf("a reset-all case with no unassigned row")
				}
			}
		})
	}
}

// ccSpecSQL builds the keystone map and the purchases of a case: the first `owned` keystones of each track, named as the save names them (by track).
func ccSpecSQL(t *testing.T, c ccSpecCase) string {
	t.Helper()
	var b strings.Builder
	id := 0
	for _, r := range ccList(t, c.Response, "rows") {
		track := r["track"].(string)
		total, owned := int(r["keystoneTotal"].(float64)), int(r["keystoneOwned"].(float64))
		for i := 1; i <= total; i++ {
			id++
			fmt.Fprintf(&b, "insert into specialization_keystones_map(id,name) values (%d,'%s_%sKeystone_%02d');\n", id, track, track, i)
			if i <= owned {
				fmt.Fprintf(&b, "insert into purchased_specialization_keystones(player_id,keystone_id) values (1,%d);\n", id)
			}
		}
	}
	return b.String()
}
