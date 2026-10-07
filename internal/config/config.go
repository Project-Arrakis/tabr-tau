// Package config edits the game's Unreal .ini files (Saved/Config/Windows).
//
// Edits are line based so comments, key order, repeated keys (Paths=...) and
// line endings survive untouched. Every write is preceded by a backup.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// bomStr is the UTF-8 byte order mark.
var bomStr = string([]byte{0xEF, 0xBB, 0xBF})

var nameRe = regexp.MustCompile(`^[A-Za-z0-9_.\-]+\.ini$`)

// Entry is one key=value line.
type Entry struct {
	Line    int    `json:"line"` // 0-based line index
	Section string `json:"section"`
	Key     string `json:"key"`
	Value   string `json:"value"`
}

// Dir manages a config directory.
type Dir struct{ Path string }

// DefaultDir is %LOCALAPPDATA%\DuneSandbox\Saved\Config\Windows.
func DefaultDir() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "DuneSandbox", "Saved", "Config", "Windows")
}

func (d Dir) file(name string) (string, error) {
	if !nameRe.MatchString(name) {
		return "", errors.New("invalid config file name")
	}
	return filepath.Join(d.Path, name), nil
}

// List returns the .ini files, non-empty ones first.
func (d Dir) List() ([]map[string]any, error) {
	ents, err := os.ReadDir(d.Path)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, e := range ents {
		if e.IsDir() || !nameRe.MatchString(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, map[string]any{"name": e.Name(), "size": info.Size(), "empty": strings.TrimSpace(readOr(filepath.Join(d.Path, e.Name()))) == "", "modified": info.ModTime().Format(time.RFC3339)})
	}
	sort.SliceStable(out, func(i, j int) bool {
		ei, ej := out[i]["empty"].(bool), out[j]["empty"].(bool)
		if ei != ej {
			return !ei
		}
		return strings.ToLower(out[i]["name"].(string)) < strings.ToLower(out[j]["name"].(string))
	})
	return out, nil
}

func readOr(p string) string { b, _ := os.ReadFile(p); return string(b) }

// Expected lists the config files the app relies on and the sections the game
// keeps in each of them.
var Expected = []struct {
	Name     string
	Purpose  string
	Sections []string
}{
	{"ServerCustomSettings.ini", "Single-player difficulty, rates and multipliers", []string{"/Script/DuneSandbox.UserServerCustomSettings"}},
	{"Game.ini", "Audio, video, account and UI settings", []string{"Settings.Audio", "Settings.Video"}},
	{"GameUserSettings.ini", "Resolution, scalability and window settings", []string{"/Script/Engine.GameUserSettings"}},
	{"Engine.ini", "Engine paths and console variables", []string{"Core.System"}},
	{"Input.ini", "Input and aim settings", []string{"Settings.Input"}},
}

// Validate reports whether the config folder and each expected file exist,
// are readable, and contain the expected sections.
func (d Dir) Validate() map[string]any {
	res := map[string]any{"dir": d.Path, "dirExists": false, "ok": true}
	st, err := os.Stat(d.Path)
	files := []map[string]any{}
	if err != nil || !st.IsDir() {
		res["ok"] = false
		res["error"] = "config folder not found: " + d.Path
		for _, e := range Expected {
			files = append(files, map[string]any{"name": e.Name, "purpose": e.Purpose, "exists": false, "missingSections": e.Sections})
		}
		res["files"] = files
		return res
	}
	res["dirExists"] = true
	for _, e := range Expected {
		f := map[string]any{"name": e.Name, "purpose": e.Purpose, "exists": false, "missingSections": []string{}}
		b, err := os.ReadFile(filepath.Join(d.Path, e.Name))
		switch {
		case err != nil && os.IsNotExist(err):
			res["ok"] = false
			f["missingSections"] = e.Sections
		case err != nil:
			res["ok"] = false
			f["exists"], f["error"] = true, err.Error()
		default:
			f["exists"], f["size"] = true, len(b)
			have := map[string]bool{}
			for _, en := range parseDoc(string(b)).sections() {
				have[en] = true
			}
			missing := []string{}
			for _, s := range e.Sections {
				if !have[s] {
					missing = append(missing, s)
				}
			}
			f["missingSections"] = missing
			if len(missing) > 0 {
				res["ok"] = false
			}
		}
		files = append(files, f)
	}
	res["files"] = files
	return res
}

func (d doc) sections() []string {
	var out []string
	for _, l := range d.lines {
		if t := strings.TrimSpace(l); strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			out = append(out, t[1:len(t)-1])
		}
	}
	return out
}

// Create makes a missing expected file, or adds its missing section headers.
func (d Dir) Create(name string) error {
	for _, e := range Expected {
		if e.Name != name {
			continue
		}
		if st, err := os.Stat(d.Path); err != nil || !st.IsDir() {
			return errors.New("config folder not found: " + d.Path)
		}
		dc := doc{eol: "\r\n"}
		if b, err := os.ReadFile(filepath.Join(d.Path, name)); err == nil {
			dc = parseDoc(string(b))
		}
		have := map[string]bool{}
		for _, s := range dc.sections() {
			have[s] = true
		}
		changed := false
		for _, s := range e.Sections {
			if !have[s] {
				if len(dc.lines) > 0 && strings.TrimSpace(dc.lines[len(dc.lines)-1]) != "" {
					dc.lines = append(dc.lines, "")
				}
				dc.lines = append(dc.lines, "["+s+"]")
				changed = true
			}
		}
		if !changed {
			return errors.New("nothing to create")
		}
		return d.write(name, dc.String())
	}
	return errors.New("not an expected config file")
}

type doc struct {
	lines []string
	eol   string
	bom   bool
}

func parseDoc(text string) doc {
	d := doc{eol: "\n"}
	if strings.HasPrefix(text, bomStr) {
		d.bom, text = true, strings.TrimPrefix(text, bomStr)
	}
	if strings.Contains(text, "\r\n") {
		d.eol = "\r\n"
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		d.lines = nil
	} else {
		d.lines = strings.Split(text, "\n")
	}
	return d
}

func (d doc) String() string {
	s := strings.Join(d.lines, d.eol)
	if len(d.lines) > 0 {
		s += d.eol
	}
	if d.bom {
		s = bomStr + s
	}
	return s
}

func (d doc) entries() []Entry {
	var out []Entry
	section := ""
	for i, l := range d.lines {
		t := strings.TrimSpace(l)
		switch {
		case t == "" || strings.HasPrefix(t, ";") || strings.HasPrefix(t, "#"):
		case strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]"):
			section = t[1 : len(t)-1]
		default:
			if k, v, ok := strings.Cut(l, "="); ok {
				out = append(out, Entry{Line: i, Section: section, Key: strings.TrimSpace(k), Value: strings.TrimSpace(v)})
			}
		}
	}
	return out
}

// Read returns the raw text and parsed entries.
func (d Dir) Read(name string) (map[string]any, error) {
	p, err := d.file(name)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	dc := parseDoc(string(b))
	return map[string]any{"name": name, "text": string(b), "entries": nonNil(dc.entries()), "gameRunning": save.GameRunning()}, nil
}

func nonNil(e []Entry) []Entry {
	if e == nil {
		return []Entry{}
	}
	return e
}

func cleanValue(v string) (string, error) {
	if strings.ContainsAny(v, "\r\n") {
		return "", errors.New("value cannot contain line breaks")
	}
	return strings.TrimSpace(v), nil
}

func (d Dir) backupDir() string { return filepath.Join(d.Path, "tabr-tau-backups") }

func (d Dir) write(name, text string) error {
	if save.GameRunningNow() {
		return fmt.Errorf("%s is running and rewrites its config on exit; close the game first", save.GameProcess)
	}
	p, err := d.file(name)
	if err != nil {
		return err
	}
	if old, err := os.ReadFile(p); err == nil {
		if err := os.MkdirAll(d.backupDir(), 0o755); err != nil {
			return err
		}
		stem := strings.TrimSuffix(name, ".ini")
		if err := os.WriteFile(filepath.Join(d.backupDir(), stem+"-"+time.Now().Format("20060102-150405.000")+".ini"), old, 0o644); err != nil {
			return err
		}
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(text), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (d Dir) load(name string) (doc, error) {
	p, err := d.file(name)
	if err != nil {
		return doc{}, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return doc{}, err
	}
	return parseDoc(string(b)), nil
}

// Set changes an existing entry (identified by line + key so a stale view cannot
// clobber a different line), or adds section/key when line < 0.
func (d Dir) Set(name, section, key, value string, line int) error {
	value, err := cleanValue(value)
	if err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" || strings.ContainsAny(key, "=\r\n[]") {
		return errors.New("invalid key")
	}
	if strings.ContainsAny(section, "\r\n[]") {
		return errors.New("invalid section")
	}
	dc, err := d.load(name)
	if err != nil {
		return err
	}
	if line >= 0 {
		if line >= len(dc.lines) {
			return errors.New("file changed; reload")
		}
		k, _, ok := strings.Cut(dc.lines[line], "=")
		if !ok || strings.TrimSpace(k) != key {
			return errors.New("file changed; reload")
		}
		dc.lines[line] = key + "=" + value
		return d.write(name, dc.String())
	}
	newLine := key + "=" + value
	// append to the end of the named section, creating it if needed
	secStart, secEnd := -1, len(dc.lines)
	for i, l := range dc.lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			if secStart >= 0 {
				secEnd = i
				break
			}
			if t[1:len(t)-1] == section {
				secStart = i
			}
		}
	}
	if secStart < 0 {
		if len(dc.lines) > 0 && strings.TrimSpace(dc.lines[len(dc.lines)-1]) != "" {
			dc.lines = append(dc.lines, "")
		}
		dc.lines = append(dc.lines, "["+section+"]", newLine)
	} else {
		ins := secEnd
		for ins > secStart+1 && strings.TrimSpace(dc.lines[ins-1]) == "" {
			ins--
		}
		dc.lines = append(dc.lines[:ins], append([]string{newLine}, dc.lines[ins:]...)...)
	}
	return d.write(name, dc.String())
}

// Delete removes one entry line (verified against key).
func (d Dir) Delete(name, key string, line int) error {
	dc, err := d.load(name)
	if err != nil {
		return err
	}
	if line < 0 || line >= len(dc.lines) {
		return errors.New("file changed; reload")
	}
	if k, _, ok := strings.Cut(dc.lines[line], "="); !ok || strings.TrimSpace(k) != key {
		return errors.New("file changed; reload")
	}
	dc.lines = append(dc.lines[:line], dc.lines[line+1:]...)
	return d.write(name, dc.String())
}

// WriteRaw replaces the whole file.
func (d Dir) WriteRaw(name, text string) error { return d.write(name, text) }

// Backups lists backups for one file, newest first.
func (d Dir) Backups(name string) []map[string]any {
	stem := strings.TrimSuffix(name, ".ini") + "-"
	ents, _ := os.ReadDir(d.backupDir())
	out := []map[string]any{}
	for i := len(ents) - 1; i >= 0; i-- {
		if strings.HasPrefix(ents[i].Name(), stem) {
			info, _ := ents[i].Info()
			out = append(out, map[string]any{"name": ents[i].Name(), "size": info.Size()})
		}
	}
	return out
}

// Restore puts a backup back in place (the current file is backed up first).
func (d Dir) Restore(name, backup string) error {
	b, err := os.ReadFile(filepath.Join(d.backupDir(), filepath.Base(backup)))
	if err != nil {
		return err
	}
	if !strings.HasPrefix(filepath.Base(backup), strings.TrimSuffix(name, ".ini")+"-") {
		return errors.New("backup does not belong to this file")
	}
	return d.write(name, string(b))
}
