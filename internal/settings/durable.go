package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

// Hooks so tests can inject faults (a crash between the temp file and the rename, a failing sync). Production code never changes them.
var (
	syncFile     = func(f *os.File) error { return f.Sync() }
	syncDir      = syncDirectory
	beforeRename = func(dst string) {}
	now          = time.Now
)

// BackupPath is the copy of the settings file as it was before the last change. One generation is kept.
func BackupPath(path string) string { return path + ".bak" }

type fileState int

const (
	stMissing    fileState = iota // no file: the defaults, nothing to lose
	stOK                          // read and parsed
	stDamaged                     // there, readable, but empty, too large or not a settings object
	stUnreadable                  // there, but it could not be read (permissions, a folder in its place)
)

type fileRead struct {
	state fileState
	s     Settings
	raw   []byte
	err   error
}

// readSettings reads and checks one settings file. An empty file is damaged, not "nothing to lose": the editor never writes one, so
// it is what a crash in the middle of a replace leaves behind, and the kits that were in it are gone.
func readSettings(path string) fileRead {
	base := filepath.Base(path)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return fileRead{state: stMissing}
	}
	if err != nil {
		return fileRead{state: stUnreadable, err: fmt.Errorf("the settings file %s cannot be read: %w", base, err)}
	}
	dmg := func(why string) fileRead {
		return fileRead{state: stDamaged, raw: b, err: fmt.Errorf("the settings file %s is damaged (%s)", base, why)}
	}
	if len(b) > maxFileBytes {
		return dmg(fmt.Sprintf("it is larger than %d KiB", maxFileBytes>>10))
	}
	t := bytes.TrimSpace(b)
	if len(t) == 0 {
		return dmg("it is empty")
	}
	if bytes.Equal(t, []byte("null")) {
		return dmg("it holds no settings")
	}
	var s Settings
	if err := json.Unmarshal(b, &s); err != nil {
		return dmg(err.Error())
	}
	return fileRead{state: stOK, s: s, raw: b}
}

// writeFileAtomic replaces path with data so that a crash at any moment leaves either the old file or the new one, never a part of
// either: the data goes to a temp file in the same folder, is flushed to stable storage (Sync) and closed, the temp file is renamed
// over the target (with short retries: on Windows a virus scanner or the indexer can hold the target for a moment) and the folder is
// synced where the system allows it, so the rename itself survives a power loss. The same steps as the save file (internal/save).
func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "settings-*.tmp") // mode 0600
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := syncFile(tmp); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	beforeRename(path)
	for i := 0; i < 8; i++ {
		if err = os.Rename(name, path); err == nil {
			syncDir(dir)
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	os.Remove(name)
	return err
}

// syncDirectory flushes the folder entry of a rename on systems that can (Linux, macOS). Windows cannot open a folder for a flush from
// Go, and NTFS journals the rename itself; elsewhere a failure here is not a reason to fail a write that has already landed.
func syncDirectory(dir string) {
	if runtime.GOOS == "windows" {
		return
	}
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	d.Sync()
	d.Close()
}

// sweepTemps removes temp files that a crash left in the settings folder. A file younger than a minute may belong to a write that is
// still going on in another copy of the editor, so it is left.
func sweepTemps(dir string) {
	m, _ := filepath.Glob(filepath.Join(dir, "settings-*.tmp"))
	for _, f := range m {
		if st, err := os.Stat(f); err == nil && now().Sub(st.ModTime()) > time.Minute {
			os.Remove(f)
		}
	}
}

// damagedName is a free name for the damaged file: settings.json.damaged-20261026-153012, then -2, -3.
func damagedName(path string) string {
	base := path + ".damaged-" + now().Format("20060102-150405")
	name := base
	for i := 2; i < 1000; i++ {
		if _, err := os.Lstat(name); errors.Is(err, os.ErrNotExist) {
			break
		}
		name = base + "-" + strconv.Itoa(i)
	}
	return name
}
