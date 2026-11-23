// Package layoutroll changes which Deep Desert layout the single-player game loads, by writing the game's own layout setting into the user's Game.ini (#1141).
//
// The layout is not in the save. The game picks it at launch from the line m_WorldLayoutsPerWorld=(... WorldLayoutIndexInSingleplayer=<n> ...) of its own DefaultGame.ini, and the
// same whole line in the user's Saved\Config\Windows\Game.ini overrides it. This package only edits that one number in that one line of that one file (or, when the line is not
// there yet, adds the whole line), after a backup, and never while the game is running. It does not read or write a save.
//
// ini.go holds the pure part: finding the line and changing the number. roll.go holds the part that touches files.
package layoutroll

import (
	_ "embed"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
)

const (
	// Section and Key are where the game's layout setting lives in an ini file.
	Section = "/Script/DuneSandbox.WorldLayoutSubsystem"
	Key     = "m_WorldLayoutsPerWorld"
	// MinIndex and MaxIndex are the layouts a roll may pick: 0 to 11, the ones measured to work. Indexes 15 to 17 are QA or special layouts and stay out; 12 to 14 were not measured.
	MinIndex = 0
	MaxIndex = 11
	// DefaultIndex is the layout the game's own DefaultGame.ini names (measured in the game): the layout in use when Game.ini holds no line of its own.
	DefaultIndex = 11
)

// lineTemplate is the whole line, as the game's DefaultGame.ini has it (the list of every layout's data and the index in the last place), with DefaultIndex as the number. It is
// written into Game.ini only when Game.ini has no such line yet. It was read from the game as measured on 2026-10-27; a game update that changes the list of layouts makes it stale,
// which is why a line that is already in Game.ini is only ever edited in its one number and never replaced.
//
//go:embed worldlayouts-line.txt
var lineTemplate string

// indexPattern finds the number in the value of the line. Spaces around the "=" are tolerated.
var indexPattern = regexp.MustCompile(`(WorldLayoutIndexInSingleplayer\s*=\s*)(-?[0-9]+)`)

// iniLine is one line of the file: its text without the line end, and the line end ("\r\n", "\n" or "" for a last line without one).
type iniLine struct{ text, eol string }

func splitLines(text string) []iniLine {
	var out []iniLine
	for len(text) > 0 {
		i := strings.IndexByte(text, '\n')
		if i < 0 {
			out = append(out, iniLine{text: text})
			break
		}
		line, eol := text[:i], "\n"
		if strings.HasSuffix(line, "\r") {
			line, eol = line[:len(line)-1], "\r\n"
		}
		out = append(out, iniLine{line, eol})
		text = text[i+1:]
	}
	return out
}

func joinLines(ls []iniLine) string {
	var b strings.Builder
	for _, l := range ls {
		b.WriteString(l.text)
		b.WriteString(l.eol)
	}
	return b.String()
}

// sectionName returns the name when the line is a section header ("[name]"), else ok is false.
func sectionName(line string) (string, bool) {
	t := strings.TrimSpace(line)
	if len(t) >= 2 && t[0] == '[' && t[len(t)-1] == ']' {
		return t[1 : len(t)-1], true
	}
	return "", false
}

// keyOf returns the key and the value of a "key=value" line; ok is false for a blank line, a comment, a header or a line with no "=".
func keyOf(line string) (key, value string, ok bool) {
	t := strings.TrimSpace(line)
	if t == "" || t[0] == ';' || t[0] == '#' || t[0] == '[' {
		return "", "", false
	}
	i := strings.IndexByte(t, '=')
	if i < 0 {
		return "", "", false
	}
	return strings.TrimSpace(t[:i]), t[i+1:], true
}

// found is where the layout line is in a parsed file.
type found struct {
	at      int // index of the layout line; -1 when there is none
	header  int // index of the first header of the section; -1 when the section is not there
	section bool
}

// find looks for the layout line inside the layout section (a section may be written more than once; the same key under another section is not this setting and is left alone).
// A key that is written more than once in the section, or with an ini operator in front of it (+, -, . or !), is a form this package does not know how to edit: an error, and nothing is changed.
func find(ls []iniLine) (found, error) {
	f := found{at: -1, header: -1}
	in := false
	for i, l := range ls {
		if name, ok := sectionName(l.text); ok {
			in = strings.EqualFold(name, Section)
			if in && f.header < 0 {
				f.header = i
			}
			continue
		}
		if !in {
			continue
		}
		k, _, ok := keyOf(l.text)
		if !ok {
			continue
		}
		bare := strings.TrimLeft(k, "+-.!")
		if !strings.EqualFold(bare, Key) {
			continue
		}
		if bare != k {
			return f, fmt.Errorf("the line for %s in Game.ini starts with an ini operator (%q); this form is not edited", Key, k[:len(k)-len(bare)])
		}
		if f.at >= 0 {
			return f, fmt.Errorf("Game.ini has %s more than once in [%s]; it is not clear which one the game uses, so nothing is changed", Key, Section)
		}
		f.at = i
	}
	f.section = f.header >= 0
	return f, nil
}

// CurrentIndex reads the layout number that Game.ini sets. present is false when the file has no line for it (the game then uses DefaultIndex). An error means the line is there
// but is not in a form that can be read or edited safely.
func CurrentIndex(text string) (idx int, present bool, err error) {
	ls := splitLines(text)
	f, err := find(ls)
	if err != nil {
		return 0, false, err
	}
	if f.at < 0 {
		return DefaultIndex, false, nil
	}
	_, v, _ := keyOf(ls[f.at].text)
	m := indexPattern.FindAllStringSubmatch(v, -1)
	if len(m) != 1 {
		return 0, true, fmt.Errorf("the %s line in Game.ini does not hold WorldLayoutIndexInSingleplayer exactly once; nothing is changed", Key)
	}
	n, err := strconv.Atoi(m[0][2])
	if err != nil {
		return 0, true, fmt.Errorf("the layout number in Game.ini is not a number this program reads: %v", err)
	}
	return n, true, nil
}

// SetIndex returns text with the layout number set to idx (0 to 11). Everything else in the text stays byte for byte as it was: when the line is there only the digits of its number
// change; when the line is missing the whole line is added under its section (the section is added at the end when it is missing too), with the file's own line ends.
func SetIndex(text string, idx int) (string, error) { return setIndex(text, idx, lineTemplate) }

func setIndex(text string, idx int, template string) (string, error) {
	if idx < MinIndex || idx > MaxIndex {
		return "", fmt.Errorf("layout %d is outside %d to %d", idx, MinIndex, MaxIndex)
	}
	ls := splitLines(text)
	f, err := find(ls)
	if err != nil {
		return "", err
	}
	if f.at >= 0 {
		key, v, _ := keyOf(ls[f.at].text)
		if m := indexPattern.FindAllStringSubmatchIndex(v, -1); len(m) != 1 {
			return "", fmt.Errorf("the %s line in Game.ini does not hold WorldLayoutIndexInSingleplayer exactly once; nothing is changed", key)
		}
		// replace the digits inside the original line text, so spacing and everything else stays as written
		line := ls[f.at].text
		eq := strings.IndexByte(line, '=')
		head, rest := line[:eq+1], line[eq+1:]
		loc := indexPattern.FindStringSubmatchIndex(rest)
		ls[f.at].text = head + rest[:loc[4]] + strconv.Itoa(idx) + rest[loc[5]:]
		return joinLines(ls), nil
	}
	line, err := templateLine(template, idx)
	if err != nil {
		return "", err
	}
	eol := "\r\n" // a file the game writes on Windows; a file that already has line ends keeps its own
	switch {
	case f.header >= 0 && ls[f.header].eol != "":
		eol = ls[f.header].eol
	case strings.Contains(text, "\r\n"):
		eol = "\r\n"
	case strings.Contains(text, "\n"):
		eol = "\n"
	}
	if f.header >= 0 {
		if ls[f.header].eol == "" {
			ls[f.header].eol = eol
		}
		out := append([]iniLine{}, ls[:f.header+1]...)
		out = append(out, iniLine{line, eol})
		out = append(out, ls[f.header+1:]...)
		return joinLines(out), nil
	}
	if n := len(ls); n > 0 && ls[n-1].eol == "" {
		ls[n-1].eol = eol
	}
	if len(ls) > 0 {
		ls = append(ls, iniLine{"", eol})
	}
	ls = append(ls, iniLine{"[" + Section + "]", eol}, iniLine{line, eol})
	return joinLines(ls), nil
}

// templateLine returns the template with its number set to idx. The template must be one line that starts with the key and holds the number exactly once.
func templateLine(template string, idx int) (string, error) {
	t := strings.TrimSpace(template)
	k, v, ok := keyOf(t)
	if !ok || k != Key || strings.ContainsAny(t, "\r\n") || len(indexPattern.FindAllStringIndex(v, -1)) != 1 {
		return "", fmt.Errorf("the built-in layout line is not usable (a bug); nothing is changed")
	}
	loc := indexPattern.FindStringSubmatchIndex(t)
	return t[:loc[4]] + strconv.Itoa(idx) + t[loc[5]:], nil
}

// Pick chooses the layout of a roll: a number from 0 to 11 that is never current. intn is the random source (intn(n) is in [0, n)); a test passes its own.
func Pick(current int, intn func(n int) int) int {
	var cand []int
	for i := MinIndex; i <= MaxIndex; i++ {
		if i != current {
			cand = append(cand, i)
		}
	}
	p := intn(len(cand))
	if p < 0 || p >= len(cand) { // a broken source never gives a number outside the list
		p = 0
	}
	return cand[p]
}

// randomIntn is the random source of a real roll.
func randomIntn(n int) int { return rand.IntN(n) }
