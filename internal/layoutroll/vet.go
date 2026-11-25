package layoutroll

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Project-Arrakis/tabr-tau/kit/durable"
)

// Vetting (#1222). A layout is checked in the game one at a time: `layout-roll vet N` sets it in Game.ini, the person reads the Sandworm Threat in the game, and `layout-roll vet N pass|fail`
// records the result and moves N into or out of the vetted list (vetted.go) that a random roll picks from. Nothing here opens or writes the save: the save is only named, to take its lock
// (the same refusal as the roll, while the editor or another roll holds it).
const (
	// VettingFileName is the record of results beside the vetted list: one line per result, newest last, for example
	//   2026-11-24 22:41 layout 8 fail worm threat 100% on rock
	// The roll never reads it; it is for the person (and `vet list`).
	VettingFileName = "layout-roll-vetting.txt"
	// maxVetNote is the most characters of a note that are kept.
	maxVetNote = 200
	// maxVettingFile is the largest results file that is read (about 3,800 lines of the longest kind).
	maxVettingFile = 1 << 20
	// vetTimeFormat is the date and time of a line of the results file (the clock of the machine).
	vetTimeFormat = "2006-01-02 15:04"
	// vettedHeader starts a vetted file that this command creates.
	vettedHeader = "# Layouts checked in the game. The roll picks only from these (commas, spaces or new lines; # starts a comment). `tabr-tau layout-roll vet N pass` and `vet N fail` edit this file."
)

// vettingPath is the results file: beside the vetted list. Empty when no settings place is known.
func vettingPath(statePath string) string {
	if statePath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(statePath), VettingFileName)
}

// vetResultLine is one line of the results file.
var vetResultLine = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}) layout (\d+) (pass|fail)(?: (.*))?$`)

// vetResult is the last result of one layout.
type vetResult struct {
	at   string
	pass bool
	note string
}

// cleanNote makes a note fit on one line: every line end, tab and other control character becomes a space, runs of spaces become one, and only the first maxVetNote characters stay.
func cleanNote(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == ' ' || r == ' ' || !utf8.ValidRune(r) || r == utf8.RuneError {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > maxVetNote {
		s = strings.TrimSpace(string([]rune(s)[:maxVetNote]))
	}
	return s
}

// parseVetting reads the results file: the last line of each layout wins (newest last). Lines that are not a result (blank lines, # comments, anything else) are counted in skipped.
func parseVetting(text string) (last map[int]vetResult, skipped int) {
	last = map[int]vetResult{}
	text = strings.TrimPrefix(text, "\xef\xbb\xbf")
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimRight(l, "\r")
		if t := strings.TrimSpace(l); t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		m := vetResultLine.FindStringSubmatch(l)
		if m == nil {
			skipped++
			continue
		}
		n, err := strconv.Atoi(m[2])
		if err != nil {
			skipped++
			continue
		}
		last[n] = vetResult{at: m[1], pass: m[3] == "pass", note: m[4]}
	}
	return last, skipped
}

// vettedListInUse reads the vetted list a roll would use from the settings file (or the built-in list). An error (text says what to do) when the file cannot be used; a file of comments only is
// an empty list here (a roll refuses it, but this command may fill it again). text and exists are what is on disk, for an edit.
func vettedListInUse(statePath string) (list []int, text string, exists bool, source string, err error) {
	path := vettedPath(statePath)
	if path == "" {
		return defaultVetted(), "", false, fmt.Sprintf(vettedFromDefault, VettedFileName), nil
	}
	text, exists, rerr := readVettedFile(path)
	if rerr != nil {
		return nil, "", true, "", fmt.Errorf("the vetted layouts file %s cannot be used: %v. Fix or delete it (without it the built-in list %s is used)", path, rerr, joinInts(defaultVetted()))
	}
	if !exists {
		return defaultVetted(), "", false, fmt.Sprintf(vettedFromDefault, path), nil
	}
	list, perr := ParseVetted(text)
	if perr != nil && !errors.Is(perr, errNoVettedNumber) {
		return nil, "", true, "", fmt.Errorf("the vetted layouts file %s cannot be used: %v. Fix or delete it (without it the built-in list %s is used)", path, perr, joinInts(defaultVetted()))
	}
	return list, text, true, vettedFromFile + path, nil
}

func hasInt(l []int, n int) bool {
	for _, v := range l {
		if v == n {
			return true
		}
	}
	return false
}

// ---- editing the text of the vetted file, keeping what the person wrote around the numbers ----

var digitRun = regexp.MustCompile(`[0-9]+`)

// codePart splits a line at the "#" that starts a comment.
func codePart(line string) (code, comment string) {
	if i := strings.IndexByte(line, '#'); i >= 0 {
		return line[:i], line[i:]
	}
	return line, ""
}

func eolOf(text string) string {
	if strings.Contains(text, "\n") && !strings.Contains(text, "\r\n") {
		return "\n"
	}
	return "\r\n"
}

// newVettedText is a vetted file made from a list: the header and the numbers.
func newVettedText(list []int) string {
	l := append([]int{}, list...)
	sort.Ints(l)
	return vettedHeader + "\r\n" + joinInts(l) + "\r\n"
}

// addVettedText returns the text with n added: on the last line that holds numbers (before its comment), else on a new last line. Comments, spacing and line ends stay as they are. The text
// must already be a valid vetted list (or hold comments only).
func addVettedText(text string, n int) string {
	bom := ""
	if strings.HasPrefix(text, "\xef\xbb\xbf") {
		bom, text = "\xef\xbb\xbf", strings.TrimPrefix(text, "\xef\xbb\xbf")
	}
	eol := eolOf(text)
	ls := splitLines(text)
	at, end := -1, 0
	for i, l := range ls {
		code, _ := codePart(l.text)
		if m := digitRun.FindAllStringIndex(code, -1); len(m) > 0 {
			at, end = i, m[len(m)-1][1]
		}
	}
	if at >= 0 {
		t := ls[at].text
		ls[at].text = t[:end] + ", " + strconv.Itoa(n) + t[end:]
		return bom + joinLines(ls)
	}
	if k := len(ls); k > 0 && ls[k-1].eol == "" {
		ls[k-1].eol = eol
	}
	ls = append(ls, iniLine{strconv.Itoa(n), eol})
	return bom + joinLines(ls)
}

// removeVettedText returns the text with every n taken out of the numbers (a number inside a comment is not a number and is left). A line that held only n is dropped; a line with other numbers
// keeps them, written with ", " between; a line with a comment keeps the comment. A text left with nothing but comments gets a comment line that says the list is empty.
func removeVettedText(text string, n int) string {
	bom := ""
	if strings.HasPrefix(text, "\xef\xbb\xbf") {
		bom, text = "\xef\xbb\xbf", strings.TrimPrefix(text, "\xef\xbb\xbf")
	}
	eol := eolOf(text)
	ls := splitLines(text)
	var out []iniLine
	for _, l := range ls {
		code, comment := codePart(l.text)
		runs := digitRun.FindAllString(code, -1)
		var keep []string
		hit := false
		for _, r := range runs {
			if v, err := strconv.Atoi(r); err == nil && v == n {
				hit = true
				continue
			}
			keep = append(keep, r)
		}
		if !hit {
			out = append(out, l)
			continue
		}
		lead := code[:len(code)-len(strings.TrimLeft(code, " \t"))]
		gap := code[len(strings.TrimRight(code, " \t")):]
		switch {
		case len(keep) > 0 && comment != "":
			if gap == "" {
				gap = " "
			}
			l.text = lead + strings.Join(keep, ", ") + gap + comment
		case len(keep) > 0:
			l.text = lead + strings.Join(keep, ", ")
		case comment != "":
			l.text = lead + comment
		default:
			continue // the line held only the number
		}
		out = append(out, l)
	}
	res := joinLines(out)
	if _, err := ParseVetted(res); errors.Is(err, errNoVettedNumber) {
		if res != "" && !strings.HasSuffix(res, "\n") {
			res += eol
		}
		res += "# (no layout is vetted now: `tabr-tau layout-roll vet N pass` adds one)" + eol
	}
	return bom + res
}

// ---- the three commands ----

func vetReady(o Options, needGame bool) error {
	if o.Now == nil || (needGame && o.GameRunning == nil) {
		return errors.New("layout-roll vet: not set up (a bug)")
	}
	return nil
}

// VetRecord records the result of vetting layout n: a line in the results file and, for a pass, n in the vetted list, for a fail, n out of it (the record stays). Game.ini and the save are not
// touched, and the game may be running (the person has just looked at the layout in it); a pass of a layout that Game.ini does not list, or above 11, is refused. Both files are read and
// checked before either is written, and each is replaced whole (a new file beside it, flushed, then renamed over). The list is written first, since it is what changes the roll; when the
// record cannot be written after that, the error says so.
func VetRecord(o Options, n int, pass bool, note string) (Result, error) {
	var r Result
	if err := vetReady(o, false); err != nil {
		return r, err
	}
	word := "fail"
	if pass {
		word = "pass"
	}
	switch {
	case pass && (n < MinIndex || n > MaxIndex):
		return r, fmt.Errorf("not recorded: layout %d cannot be vetted as a pass: a roll writes only layouts %d to %d", n, MinIndex, MaxIndex)
	case n < 0 || n > maxVettedNumber:
		return r, fmt.Errorf("not recorded: %d is not a layout number (use a whole number from 0 to %d)", n, maxVettedNumber)
	}
	listPath, resPath := vettedPath(o.StatePath), vettingPath(o.StatePath)
	if listPath == "" {
		return r, errors.New("not recorded: no settings folder is known (the folder of the editor's settings could not be found)")
	}
	if pass {
		if err := plainFolder(o.ConfigDir); err != nil {
			return r, fmt.Errorf("not recorded: %v (Game.ini must be readable to check that layout %d exists)", err, n)
		}
		ini, _, _, err := readIni(filepath.Join(o.ConfigDir, FileName))
		if err != nil {
			return r, fmt.Errorf("not recorded: Game.ini cannot be read (%v), so it cannot be checked that layout %d exists", err, n)
		}
		if known := knownLayouts(ini); n >= known {
			return r, fmt.Errorf("not recorded: Game.ini lists only %d layouts (0 to %d), so layout %d cannot be vetted as a pass", known, known-1, n)
		}
	}
	list, text, exists, source, err := vettedListInUse(o.StatePath)
	if err != nil {
		return r, fmt.Errorf("not recorded: %v", err)
	}
	resText, _, err := readVettingFile(resPath)
	if err != nil {
		return r, fmt.Errorf("not recorded: the results file %s cannot be used: %v. Fix or delete it", resPath, err)
	}

	// the new list
	var newText string
	var newList []int
	switch {
	case pass && hasInt(list, n):
		newList = list
	case pass && !exists:
		newList = append(append([]int{}, list...), n)
		newText = newVettedText(newList)
	case pass:
		newText = addVettedText(text, n)
	case hasInt(list, n) && !exists:
		for _, v := range list {
			if v != n {
				newList = append(newList, v)
			}
		}
		newText = newVettedText(newList)
	case hasInt(list, n):
		newText = removeVettedText(text, n)
	default:
		newList = list
	}
	if newText != "" {
		got, perr := ParseVetted(newText)
		if perr != nil && !errors.Is(perr, errNoVettedNumber) {
			return r, fmt.Errorf("not recorded: the edited vetted list does not read back (a bug): %v", perr)
		}
		newList = got
		want := hasInt(newList, n)
		if want != pass {
			return r, fmt.Errorf("not recorded: the edited vetted list does not hold layout %d as expected (a bug)", n)
		}
	}
	line := fmt.Sprintf("%s layout %d %s", o.Now().Format(vetTimeFormat), n, word)
	if note = cleanNote(note); note != "" {
		line += " " + note
	}

	if err := os.MkdirAll(filepath.Dir(listPath), 0o700); err != nil {
		return r, fmt.Errorf("not recorded: the settings folder cannot be made (%v)", err)
	}
	if newText != "" {
		if err := writeVetFile(listPath, text, exists, newText); err != nil {
			return r, fmt.Errorf("not recorded: the vetted layouts file %s was not changed (%v)", listPath, err)
		}
		if exists {
			verb := "is taken out, it is"
			if pass {
				verb = "is added, it is"
			}
			r.say("Vetted list: layout %d %s now %s (the file %s; the comments in it are kept).", n, verb, listOrNone(newList), listPath)
		} else {
			r.say("Vetted list: the file %s did not exist and is made from the built-in list %s with the change: %s.", listPath, joinInts(defaultVetted()), listOrNone(newList))
		}
	} else if pass {
		r.say("Vetted list: layout %d is in it already (%s): %s; the file is not changed.", n, source, joinInts(list))
	} else {
		r.say("Vetted list: layout %d was not in it (%s): %s; the file is not changed.", n, source, listOrNone(list))
	}

	next := resText
	if next != "" && !strings.HasSuffix(next, "\n") {
		next += eolOf(next)
	}
	next += line + eolOf(resText)
	if err := writeVetFile(resPath, resText, resText != "" || fileExists(resPath), next); err != nil {
		changed := ""
		if newText != "" {
			changed = " The vetted list WAS changed (see above); write the result line by hand if you want it kept."
		}
		return r, fmt.Errorf("the result could not be recorded in %s (%v).%s", resPath, err, changed)
	}
	r.say("Recorded in %s: %s", resPath, line)
	r.say("Game.ini and the save were not touched.")
	switch {
	case pass:
		r.say("A random roll may pick layout %d now (never while it is the layout set in Game.ini).", n)
	case len(newList) == 0:
		r.say("WARNING: the vetted list is empty now, so a random roll refuses until a layout is vetted as a pass (or until you give --vetted).")
	default:
		r.say("A random roll no longer picks layout %d; the record of the failure stays in the results file.", n)
	}
	return r, nil
}

func listOrNone(l []int) string {
	if len(l) == 0 {
		return "empty"
	}
	return joinInts(l)
}

func fileExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// readVettingFile reads the results file (a missing one is empty).
func readVettingFile(path string) (text string, exists bool, err error) {
	return readPlainFile(path, maxVettingFile)
}

// writeVetFile writes a small settings file whole and atomically (a new file beside it, flushed, renamed over it). It checks first that the file on disk is still what was read (before,
// existed), and afterwards that it reads back as written. The mode of an existing file is kept.
func writeVetFile(path, before string, existed bool, text string) error {
	cur, curExists, err := readPlainFile(path, maxVettingFile)
	if err != nil {
		return err
	}
	if cur != before || curExists != existed {
		return errors.New("the file changed while this ran; run it again")
	}
	mode := fs.FileMode(0o644)
	if st, err := os.Lstat(path); err == nil {
		mode = st.Mode().Perm()
	}
	if err := durable.WriteFile(path, []byte(text), mode, durable.Defaults()); err != nil {
		return err
	}
	back, _, err := readPlainFile(path, maxVettingFile)
	if err != nil {
		return fmt.Errorf("written, but it cannot be read back: %v", err)
	}
	if back != text {
		return errors.New("written, but it reads back as other text (another program changed it?)")
	}
	return nil
}

// VetList says, for each layout 0 to 11, its last result and whether a roll may pick it (it is in the vetted list now). It only reads.
func VetList(o Options) (Result, error) {
	var r Result
	list, _, _, source, err := vettedListInUse(o.StatePath)
	if err != nil {
		return r, err
	}
	var last map[int]vetResult
	skipped := 0
	if resPath := vettingPath(o.StatePath); resPath != "" {
		text, _, rerr := readVettingFile(resPath)
		if rerr != nil {
			return r, fmt.Errorf("the results file %s cannot be used: %v. Fix or delete it", resPath, rerr)
		}
		last, skipped = parseVetting(text)
	} else {
		last = map[int]vetResult{}
	}
	cur, curKnown, curSet := 0, false, false
	if plainFolder(o.ConfigDir) == nil {
		if ini, _, _, err := readIni(filepath.Join(o.ConfigDir, FileName)); err == nil {
			if c, present, err := CurrentIndex(ini); err == nil {
				cur, curKnown, curSet = c, true, present
			}
		}
	}
	r.say("Vetted list in use: %s (%s).", listOrNone(list), source)
	switch {
	case curKnown && curSet:
		r.say("Game.ini sets layout %d now (a roll never picks the layout that is set now).", cur)
	case curKnown:
		r.say("Game.ini has no layout line, so the game uses its own default, layout %d (a roll never picks the layout that is set now).", cur)
	default:
		r.say("Game.ini could not be read, so the layout set now is not shown.")
	}
	r.say("%-7s %-24s %s", "Layout", "Last result", "Roll may pick it")
	for n := MinIndex; n <= MaxIndex; n++ {
		res := "no result yet"
		note := ""
		if v, ok := last[n]; ok {
			w := "fail"
			if v.pass {
				w = "pass"
			}
			res = w + " " + v.at
			if v.note != "" {
				note = "  note: " + v.note
			}
		}
		may := "no"
		switch {
		case hasInt(list, n) && curKnown && n == cur:
			may = "yes (but not while it is set now)"
		case hasInt(list, n):
			may = "yes"
		}
		r.say("%-7d %-24s %s%s", n, res, may, note)
	}
	var extra []int
	for _, v := range list {
		if v > MaxIndex {
			extra = append(extra, v)
		}
	}
	if len(extra) > 0 {
		r.say("The list also holds %s, which a roll refuses (a roll writes only layouts %d to %d); take %s out of it.", joinInts(extra), MinIndex, MaxIndex, joinInts(extra))
	}
	if skipped > 0 {
		r.say("%d line(s) of the results file were not understood and are ignored.", skipped)
	}
	return r, nil
}

// VetSet sets layout n in Game.ini so that it can be checked in the game, with the checks and the write of the roll: the game must not be running (asked before and again right before the
// write), the save must not be held by the editor or another roll (when a save is known), Game.ini is backed up first, must still be what was read when it is written, and is read back. It
// never opens the save, never writes the state file and does not look for bases or vehicles in the Deep Desert (the roll does).
func VetSet(o Options, n int) (Result, error) {
	var r Result
	if err := vetReady(o, true); err != nil {
		return r, err
	}
	if n < MinIndex || n > MaxIndex {
		return r, fmt.Errorf("not changed: vet takes a layout from %d to %d, not %d", MinIndex, MaxIndex, n)
	}
	if err := plainFolder(o.ConfigDir); err != nil {
		return r, fmt.Errorf("not changed: %v", err)
	}
	path := filepath.Join(o.ConfigDir, FileName)
	old, existed, mode, err := readIni(path)
	if err != nil {
		return r, fmt.Errorf("not changed: Game.ini cannot be read (%v)", err)
	}
	cur, present, err := CurrentIndex(old)
	if err != nil {
		return r, fmt.Errorf("not changed: %v", err)
	}
	if known := knownLayouts(old); n >= known {
		return r, fmt.Errorf("not changed: Game.ini lists only %d layouts (0 to %d), so layout %d does not exist", known, known-1, n)
	}
	text, err := SetIndex(old, n)
	if err != nil {
		return r, fmt.Errorf("not changed: %v", err)
	}
	if got, _, err := CurrentIndex(text); err != nil || got != n {
		return r, fmt.Errorf("the edited text does not read back as layout %d (a bug); nothing is changed", n)
	}
	// the save lock, as the roll takes it: the editor or another roll holding the save refuses the run. Only the lock is taken; the save is not opened.
	if o.SavePath != "" {
		release, err := lockSave(lockedSavePath(o.SavePath))
		if err != nil {
			return r, fmt.Errorf("not changed: %s", lockRefusal(err))
		}
		defer release()
	} else {
		why := o.SaveProblem
		if why == "" {
			why = "no save was named"
		}
		r.say("Note: %s, so it was not checked whether the editor has a save open. Game.ini does not depend on the save.", why)
	}
	notRun := func(again bool) error {
		running, why := o.GameRunning()
		if !running {
			return nil
		}
		extra := ""
		if again {
			extra = " (it started while this ran)"
		}
		return fmt.Errorf("not changed: %s%s. Game.ini is rewritten by the game when it exits, so it is left alone while the game runs. Close the game and run this again", why, extra)
	}
	if err := notRun(false); err != nil {
		return r, err
	}
	r.Old, r.New = cur, n
	if present && cur == n {
		r.say("Game.ini already sets layout %d; nothing was written and no backup was made.", n)
		vetNextSteps(&r, n)
		return r, nil
	}
	backup := ""
	if existed {
		if backup, err = writeBackup(o.ConfigDir, old, o.Now(), mode); err != nil {
			return r, fmt.Errorf("not changed: the backup could not be made (%v)", err)
		}
		r.say("Backup: %s", filepath.Base(backup))
		if k := pruneBackups(o.ConfigDir); k > 0 {
			r.say("Removed %d older backup(s); the newest %d are kept.", k, KeepBackups)
		}
	}
	if err := notRun(true); err != nil {
		return r, err
	}
	again, _, _, err := readIni(path)
	if err != nil || again != old {
		return r, errors.New("not changed: Game.ini changed while this ran; run it again")
	}
	if mode == 0 {
		mode = 0o644
	}
	if err := durable.WriteFile(path, []byte(text), mode, writeOptions()); err != nil {
		return r, fmt.Errorf("the file Game.ini could not be written (%v); it was left as it was%s", err, backupNote(backup))
	}
	back, _, _, err := readIni(path)
	if err != nil {
		return r, fmt.Errorf("the file Game.ini was written but could not be read back to check it (%v); look at it before the game starts%s", err, backupNote(backup))
	}
	if got, _, err := CurrentIndex(back); back != text || err != nil || got != n {
		return r, fmt.Errorf("the file Game.ini was written but does not read back as layout %d; look at it before the game starts%s", n, backupNote(backup))
	}
	r.Wrote = true
	r.say("Changed Game.ini from layout %d to layout %d (%s) and read it back to check it.", cur, n, describe(present, existed))
	vetNextSteps(&r, n)
	return r, nil
}

// vetNextSteps says in plain words what to do after Game.ini sets the layout.
func vetNextSteps(r *Result, n int) {
	r.say("Next:")
	r.say("  1. Start the game and load in (the game was closed, so it reads Game.ini now).")
	r.say("  2. With the shield on, read the Sandworm Threat on a rock island and again inside a lab. It should be 0%% in both.")
	r.say("  3. Check that the trainer's rock matches the ground you stand on.")
	r.say("  4. Record what you saw: `tabr-tau layout-roll vet %d pass`, or `tabr-tau layout-roll vet %d fail <what you saw>`.", n, n)
	r.say("A random roll picks layout %d only after it is recorded as a pass. This command did not open or change the save, and it does not look for a base, a vehicle or your character in the Deep Desert: they stand on different terrain on another layout (a roll checks for them).", n)
}
