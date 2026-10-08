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
	// AutoRefillOnOpen queues the base water and generator refills as pending edits when the editor opens, if no
	// single-player session is running. Off by default; the edits still need Review & save.
	AutoRefillOnOpen bool `json:"autoRefillOnOpen"`
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
