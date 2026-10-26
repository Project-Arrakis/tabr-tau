// Package settings keeps the editor's own preferences (not the game's) in a small JSON file in the user's config
// folder, for example %APPDATA%\tabr-tau\settings.json on Windows. A missing or damaged file means defaults: a bad
// settings file must never stop the editor or turn a feature on.
package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

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

// Load reads the file. Any problem (missing, unreadable, not JSON) gives the defaults.
func Load(path string) Settings {
	s, err := LoadStrict(path)
	if err != nil {
		return Settings{}
	}
	return s
}

// LoadStrict reads the file like Load, but tells a damaged file from an absent one: a missing file is the defaults with no error; one
// that exists and cannot be read, is too large or is not JSON is an error, so a caller that is about to write can refuse instead of
// replacing what it could not read (the other settings in it would be lost).
func LoadStrict(path string) (Settings, error) {
	writeMu.Lock() // a read must not land in the middle of a write: on Windows the file is briefly unreadable while it is replaced
	defer writeMu.Unlock()
	return loadStrict(path)
}

func loadStrict(path string) (Settings, error) {
	var s Settings
	if path == "" {
		return s, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("the settings file %s cannot be read: %w", filepath.Base(path), err)
	}
	if len(b) > maxFileBytes {
		return Settings{}, fmt.Errorf("the settings file %s is larger than %d KiB", filepath.Base(path), maxFileBytes>>10)
	}
	if len(bytes.TrimSpace(b)) == 0 { // an empty file holds nothing to lose
		return s, nil
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return Settings{}, fmt.Errorf("the settings file %s is damaged (%v)", filepath.Base(path), err)
	}
	return s, nil
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

// Save writes the file atomically (temp file in the same folder, then rename), mode 0600. It does not look at what is in the file now;
// to change one setting and keep the others use Update.
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
	// On Windows a virus scanner or the search indexer can hold the file open for a moment right after it was written, and the rename
	// over it is then refused; a few short retries get past that.
	for i := 0; i < 8; i++ {
		if err = os.Rename(name, path); err == nil {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	os.Remove(name)
	return err
}
