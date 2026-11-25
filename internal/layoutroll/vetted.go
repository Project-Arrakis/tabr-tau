package layoutroll

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The vetted list (#1219). A random roll picks only from layouts the operator has checked in the game, one at a time (one layout was seen to give a Sandworm Threat on rock and in labs
// that the others did not), and never the layout that is set now. The list is a plain text file beside layout-roll.json, so it can be extended without a new build.
const (
	// VettedFileName is the settings file: layout numbers separated by commas, spaces or new lines; a "#" starts a comment that runs to the end of the line.
	VettedFileName = "layout-roll-vetted.txt"
	// maxVettedNumber is the largest number a vetted list may hold (a typo such as 911 is refused as a typo, not as an unknown layout).
	maxVettedNumber = 99
	// maxVettedFile is the largest vetted file that is read; a real one is a line or two.
	maxVettedFile = 16 << 10
)

// defaultVetted is the list used when the settings file does not exist.
func defaultVetted() []int { return []int{9, 11} }

// errNoVettedNumber is what ParseVetted says of a text with no layout number in it (a file of comments only, which the vet command may fill again).
var errNoVettedNumber = errors.New("there is no layout number in it")

// Where the list in use came from.
const (
	vettedFromFlag    = "the --vetted flag"
	vettedFromFile    = "the file "
	vettedFromDefault = "the built-in default (no file %s exists)"
)

// vettedChoice is the list in use for one random roll and the layouts the pick may choose from.
type vettedChoice struct {
	list       []int  // the vetted list, sorted, without repeats
	candidates []int  // the list without the current layout
	from       string // where the list came from, in words
}

// vettedPath is where the settings file is: beside the state file (layout-roll.json). Empty when no state file place is known.
func vettedPath(statePath string) string {
	if statePath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(statePath), VettedFileName)
}

// ParseVetted reads the text of a vetted list: integers from 0 to 99 separated by commas, spaces or new lines, with "#" comments. Anything else in it is an error that names it, so a typo
// never turns into a smaller list. The numbers come back sorted without repeats; a text with no number in it is an error.
func ParseVetted(text string) ([]int, error) {
	text = strings.TrimPrefix(text, "\xef\xbb\xbf") // a byte order mark, which Notepad may add
	var out []int
	seen := map[int]bool{}
	for _, line := range strings.Split(text, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		for _, tok := range strings.Fields(strings.ReplaceAll(line, ",", " ")) {
			n, err := strconv.Atoi(tok)
			if err != nil || strings.TrimLeft(tok, "0123456789") != "" {
				return nil, fmt.Errorf("%q is not a layout number (use whole numbers from 0 to %d, separated by commas, spaces or new lines)", tok, maxVettedNumber)
			}
			if n > maxVettedNumber {
				return nil, fmt.Errorf("%d is not a layout number (use whole numbers from 0 to %d)", n, maxVettedNumber)
			}
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	if len(out) == 0 {
		return nil, errNoVettedNumber
	}
	sort.Ints(out)
	return out, nil
}

// layoutEntry finds the start of one layout in the list of the line: the game indexes the layouts by their order in that list.
var layoutEntry = regexp.MustCompile(`\(Content\s*=`)

// knownLayouts is how many layouts the layout line lists: the line of Game.ini when it has one, else the built-in line (which is what the game's own list is when Game.ini has none).
func knownLayouts(iniText string) int {
	ls := splitLines(iniText)
	if f, err := find(ls); err == nil && f.at >= 0 {
		_, v, _ := keyOf(ls[f.at].text)
		return len(layoutEntry.FindAllStringIndex(v, -1))
	}
	return len(layoutEntry.FindAllStringIndex(lineTemplate, -1))
}

// readVettedFile reads the settings file. exists is false when there is none (the default list is then used). A file that cannot be used safely is an error.
func readVettedFile(path string) (text string, exists bool, err error) {
	return readPlainFile(path, maxVettedFile)
}

// readPlainFile reads a small plain 8-bit text file (the vetted list, the vetting results): a missing file is exists false; a link, a folder, a file over max bytes and text that is not 8-bit text are errors.
func readPlainFile(path string, max int64) (text string, exists bool, err error) {
	st, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", true, err
	}
	if !st.Mode().IsRegular() {
		return "", true, errors.New("it is not a plain file (links are refused)")
	}
	if st.Size() > max {
		return "", true, fmt.Errorf("it is larger than %d bytes", max)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", true, err
	}
	if bytes.HasPrefix(b, []byte{0xFF, 0xFE}) || bytes.HasPrefix(b, []byte{0xFE, 0xFF}) || bytes.IndexByte(b, 0) >= 0 {
		return "", true, errors.New("it is not 8-bit text (save it as plain text, UTF-8)")
	}
	return string(b), true, nil
}

// chooseVetted works out the list for one random roll: the --vetted flag, else the settings file, else the built-in list. It refuses (an error whose text says what to do) when the file
// exists but cannot be read or holds no valid number, when a number is not a layout that Game.ini lists or that the roll writes, and when the list holds nothing but the current layout.
// Nothing is written.
func chooseVetted(o Options, iniText string, cur int) (vettedChoice, error) {
	var v vettedChoice
	var list []int
	var err error
	switch path := vettedPath(o.StatePath); {
	case o.VettedFlag != nil:
		v.from = vettedFromFlag
		if list, err = ParseVetted(*o.VettedFlag); err != nil {
			return v, fmt.Errorf("the list given with --vetted cannot be used: %v. Give layout numbers, for example --vetted 9,11", err)
		}
	case path == "":
		v.from = fmt.Sprintf(vettedFromDefault, VettedFileName)
		list = defaultVetted()
	default:
		text, exists, rerr := readVettedFile(path)
		switch {
		case rerr != nil:
			return v, fmt.Errorf("the vetted layouts file %s cannot be used: %v. Fix or delete it (without it the built-in list %s is used), or pass --vetted 9,11 for one run", path, rerr, joinInts(defaultVetted()))
		case !exists:
			v.from = fmt.Sprintf(vettedFromDefault, path)
			list = defaultVetted()
		default:
			v.from = vettedFromFile + path
			if list, err = ParseVetted(text); err != nil {
				return v, fmt.Errorf("the vetted layouts file %s cannot be used: %v. Put the layouts you have checked in the game in it, for example 9, 11 (commas, spaces or new lines; # starts a comment), or delete the file to use the built-in list %s", path, err, joinInts(defaultVetted()))
			}
		}
	}
	known := knownLayouts(iniText)
	for _, n := range list {
		switch {
		case n >= known:
			return v, fmt.Errorf("layout %d is in the vetted list (%s) but Game.ini lists only %d layouts (0 to %d); take %d out of the list", n, v.from, known, known-1, n)
		case n < MinIndex || n > MaxIndex:
			return v, fmt.Errorf("layout %d is in the vetted list (%s) but a roll only writes layouts %d to %d; take %d out of the list", n, v.from, MinIndex, MaxIndex, n)
		}
	}
	v.list = list
	for _, n := range list {
		if n != cur {
			v.candidates = append(v.candidates, n)
		}
	}
	if len(v.candidates) == 0 {
		return v, fmt.Errorf("the vetted list (%s) is %s and Game.ini sets layout %d now, so there is no other vetted layout to roll to. Check another layout in the game and add its number to %s (or pass --vetted)", v.from, joinInts(list), cur, VettedFileName)
	}
	return v, nil
}

func joinInts(l []int) string {
	s := make([]string, len(l))
	for i, n := range l {
		s[i] = strconv.Itoa(n)
	}
	return strings.Join(s, ", ")
}

// lines is what a run says about the list: the list, where it came from, and what the pick may choose from.
func (v vettedChoice) lines(cur int) []string {
	return []string{
		fmt.Sprintf("Vetted layouts: %s. Source: %s.", joinInts(v.list), v.from),
		fmt.Sprintf("A random pick may choose from: %s (never layout %d, which is set now).", joinInts(v.candidates), cur),
	}
}

// note is the same in one phrase, for the record of the run.
func (v vettedChoice) note() string {
	return fmt.Sprintf("vetted layouts %s, source: %s; chosen from %s", joinInts(v.list), v.from, joinInts(v.candidates))
}
