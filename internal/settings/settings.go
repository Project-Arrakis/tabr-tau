// Package settings keeps the editor's own preferences (not the game's) in a small JSON file in the user's config
// folder, for example %APPDATA%\tabr-tau\settings.json on Windows. A missing or damaged file means defaults: a bad
// settings file must never stop the editor or turn a feature on.
package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Settings is everything the editor remembers between runs.
type Settings struct {
	// AutoRefillOnOpen refills base water and generators when the editor opens and saves them straight away (no review
	// step; an explicit, opt-in exception), unless a single-player session is running. Off by default. It applies to
	// whichever save the editor opens.
	AutoRefillOnOpen bool `json:"autoRefillOnOpen"`
	// TrackNumbers is which number the game uses for each specialization track in specialization_tracks (the save stores a bare number
	// and does not say which track it is). Learned from the character's own rows with Assign on the Specialization tab.
	TrackNumbers map[string]int64 `json:"trackNumbers,omitempty"`
}

// Path is the settings file. It is empty when the OS has no user config folder.
func Path() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, "tabr-tau", "settings.json")
}

// Load reads the file. Any problem (missing, unreadable, not JSON) gives the defaults.
func Load(path string) Settings {
	var s Settings
	if path == "" {
		return s
	}
	b, err := os.ReadFile(path)
	if err != nil || len(b) > 64<<10 {
		return Settings{}
	}
	if json.Unmarshal(b, &s) != nil {
		return Settings{}
	}
	return s
}

// Save writes the file atomically (temp file in the same folder, then rename), mode 0600.
func Save(path string, s Settings) error {
	if path == "" {
		return errors.New("this system has no user config folder to keep settings in")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "settings-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
