// Package settings keeps the editor's own preferences (not the game's) in a small JSON file in the user's config
// folder, for example %APPDATA%\tabr-tau\settings.json on Windows. A missing file means defaults. An empty or damaged one means the copy
// from before the last change (settings.json.bak) when that is good; without one it is reported, read as defaults, and never written
// over without a copy (settings.json.damaged-<time>): a bad settings file must never stop the editor or turn a feature on.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Project-Arrakis/tabr-tau/internal/kits"
)

// Settings is everything the editor remembers between runs.
type Settings struct {
	// AutoRefillOnOpen is the master switch of the automatic pass: when the editor opens it runs the jobs below that are
	// switched on (base water, base power, repairs, fills) as one edit and writes it straight away, like any other edit
	// (it is skipped while a single-player session is running). Off by default. It applies to whichever save the editor opens.
	AutoRefillOnOpen bool `json:"autoRefillOnOpen"`
	// One switch per automatic job, used only while AutoRefillOnOpen is on. A key that is missing from the file (a file from an
	// older version, or a job added since) is that job's default, which the job list in internal/ops gives (all on); that is why
	// they are pointers: absent is not the same as false. Read them with JobOn and change them with SetJob.
	AutoRefillWater    *bool `json:"autoRefillWater,omitempty"`
	AutoRefillPower    *bool `json:"autoRefillPower,omitempty"`
	AutoRepairVehicles *bool `json:"autoRepairVehicles,omitempty"`
	AutoRepairGear     *bool `json:"autoRepairGear,omitempty"`
	AutoFillContainers *bool `json:"autoFillContainers,omitempty"`
	AutoFillHydration  *bool `json:"autoFillHydration,omitempty"`
	AutoFillBloodSacks *bool `json:"autoFillBloodSacks,omitempty"`
	AutoRefuelVehicles *bool `json:"autoRefuelVehicles,omitempty"`
	// TrackNumbers is which number the game uses for each specialization track in specialization_tracks (the save stores a bare number
	// and does not say which track it is). Learned from the character's own rows with Assign on the Specialization tab.
	TrackNumbers map[string]int64 `json:"trackNumbers,omitempty"`
	// Kits are the care packages (#417): named kits of items, XP and skill points that are kept here, not in a save, so they are still there when a new
	// save is started. internal/kits has the model and the checks; this file only holds them. They are written and read with Update and Load like every other
	// setting, and a list that does not pass kits.CheckStored is never written (the file would outgrow maxFileBytes and every setting with it).
	Kits []kits.Kit `json:"kits,omitempty"`
}

// maxFileBytes is the largest settings file that is read. The kits may take up to kits.MaxStoredBytes of it.
const maxFileBytes = 512 << 10

// JobKeys are the settings keys of the automatic jobs, in the order the page lists them. internal/ops has the jobs themselves and a test
// holds the two lists together.
var JobKeys = []string{"autoRefillWater", "autoRefillPower", "autoRepairVehicles", "autoRepairGear", "autoFillContainers", "autoFillHydration", "autoFillBloodSacks", "autoRefuelVehicles"}

func (s *Settings) jobField(key string) **bool {
	switch key {
	case "autoRefillWater":
		return &s.AutoRefillWater
	case "autoRefillPower":
		return &s.AutoRefillPower
	case "autoRepairVehicles":
		return &s.AutoRepairVehicles
	case "autoRepairGear":
		return &s.AutoRepairGear
	case "autoFillContainers":
		return &s.AutoFillContainers
	case "autoFillHydration":
		return &s.AutoFillHydration
	case "autoFillBloodSacks":
		return &s.AutoFillBloodSacks
	case "autoRefuelVehicles":
		return &s.AutoRefuelVehicles
	}
	return nil
}

// JobOn says whether the automatic job with this settings key is switched on: what the file says, or def when it says nothing. An
// unknown key is off.
func (s Settings) JobOn(key string, def bool) bool {
	f := s.jobField(key)
	if f == nil {
		return false
	}
	if *f == nil {
		return def
	}
	return **f
}

// SetJob switches the automatic job with this settings key on or off and says whether the key is one of JobKeys.
func (s *Settings) SetJob(key string, on bool) bool {
	f := s.jobField(key)
	if f == nil {
		return false
	}
	*f = &on
	return true
}

// Path is the settings file. It is empty when the OS has no user config folder.
func Path() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, "tabr-tau", "settings.json")
}

// Load reads the file. Any problem (missing, unreadable, not JSON, and no usable backup copy) gives the defaults.
func Load(path string) Settings {
	s, _, err := LoadNote(path)
	if err != nil {
		return Settings{}
	}
	return s
}

// LoadStrict reads the file like Load, but tells a damaged file from an absent one: a missing file is the defaults with no error; one
// that exists and cannot be read, or is empty, too large or not JSON, is an error unless the backup copy (see BackupPath) is good, so
// a caller that is about to write can refuse instead of replacing what it could not read (the other settings in it, the kits among
// them, would be lost).
func LoadStrict(path string) (Settings, error) {
	s, _, err := LoadNote(path)
	return s, err
}

// LoadNote is LoadStrict plus a note for the person: it is empty unless the file was damaged and the settings come from the backup copy,
// when it says so. The damaged file is not touched by a read; the next change keeps it as settings.json.damaged-<time>.
func LoadNote(path string) (Settings, string, error) {
	writeMu.Lock() // a read must not land in the middle of a write: on Windows the file is briefly unreadable while it is replaced
	defer writeMu.Unlock()
	return loadNote(path)
}

func loadStrict(path string) (Settings, error) {
	s, _, err := loadNote(path)
	return s, err
}

func loadNote(path string) (Settings, string, error) {
	if path == "" {
		return Settings{}, "", nil
	}
	f := readSettings(path)
	if f.state != stDamaged {
		return f.s, "", f.err
	}
	// An empty or cut-off file is what a crash right after a replace leaves behind. The copy of the file before the last change is the
	// way back; without a good one the damage is reported and nothing is written over it.
	if b := readSettings(BackupPath(path)); b.state == stOK {
		return b.s, f.err.Error() + "; the copy from before the last change (" + filepath.Base(BackupPath(path)) + ") is used instead", nil
	}
	return Settings{}, "", fmt.Errorf("%w; there is no good copy (%s) to fall back on: delete the file to start again with the defaults", f.err, filepath.Base(BackupPath(path)))
}

// writeMu serialises Update: two requests that change different settings at the same moment must not each read the old file and
// then overwrite each other's change.
var writeMu sync.Mutex

// Update changes the settings under a lock: it reads the file, applies fn and writes the result, so concurrent updates add up instead
// of the last one winning. A damaged file is reported and left alone (nothing is written) rather than replaced by the defaults plus
// the one change. It returns the settings as written. If fn returns an error nothing is written.
func Update(path string, fn func(*Settings) error) (Settings, error) {
	writeMu.Lock()
	defer writeMu.Unlock()
	s, err := loadStrict(path)
	if err != nil {
		return Settings{}, err
	}
	if err := fn(&s); err != nil {
		return Settings{}, err
	}
	if err := save(path, s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

// Save writes the file atomically and durably (see writeFileAtomic), mode 0600, after keeping the file as it is now as the backup copy
// (or, if it is damaged, as settings.json.damaged-<time>). It does not use what is in the file; to change one setting and keep the
// others use Update.
func Save(path string, s Settings) error {
	writeMu.Lock()
	defer writeMu.Unlock()
	return save(path, s)
}

func save(path string, s Settings) error {
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
	if len(b) > maxFileBytes { // a file the next read would refuse is never written
		return fmt.Errorf("the settings would take %d KiB; the file is read only up to %d KiB: delete or shorten a kit", len(b)>>10, maxFileBytes>>10)
	}
	sweepTemps(filepath.Dir(path))
	// What is in the file now decides what happens to it before it is replaced (the new file is still written to a temp file first, so
	// until the last rename the old one is untouched):
	//   good    -> kept as the backup copy (one generation), so a replaced file that turns out empty or cut off has a way back
	//   damaged -> kept as settings.json.damaged-<time>, never replaced without a trace; the backup copy is not touched
	//   missing -> the backup copy goes too (the file was removed on purpose; its copy must not come back later)
	cur := readSettings(path)
	switch cur.state {
	case stUnreadable:
		return cur.err
	case stOK:
		if err := writeFileAtomic(BackupPath(path), cur.raw); err != nil {
			return fmt.Errorf("the previous settings could not be kept as %s, so nothing was changed: %w", filepath.Base(BackupPath(path)), err)
		}
	case stDamaged:
		if err := writeFileAtomic(damagedName(path), cur.raw); err != nil {
			return fmt.Errorf("the damaged settings file could not be kept aside, so nothing was changed: %w", err)
		}
	}
	if err := writeFileAtomic(path, append(b, '\n')); err != nil {
		return err
	}
	if cur.state == stMissing {
		os.Remove(BackupPath(path))
	}
	return nil
}
