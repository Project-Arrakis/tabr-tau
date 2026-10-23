package save

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The decoded copies of a save are plain SQLite files and hold everything in it, account and platform ids included. They are made in a folder of their own in the temp
// folder, tabr-tau-work, created for the owner alone, and every copy is named with the id of the process that made it (p1234-tabr-tau-....sqlite). A copy is removed when its
// session ends; one that a crash or a kill left behind is found by its process id the next time an editor starts and removed if that process is gone (SweepWorkCopies).
// Copies that older versions left loose in the temp folder are removed when they are a day old or more (a copy that is open is not removable on Windows, so a live one stays).

const workDirName = "tabr-tau-work"

var copyOfProcess = regexp.MustCompile(`^p(\d+)-`)

// workDir is the folder of the decoded copies, made if it is not there.
func workDir() string {
	d := filepath.Join(os.TempDir(), workDirName)
	if err := os.MkdirAll(d, 0o700); err == nil {
		os.Chmod(d, 0o700) // an earlier, looser folder is made private too (a no-op on Windows, where the temp folder is the account's own)
	}
	return d
}

// tempCopy makes a new empty file for a decoded copy, named with this process's id. pattern is as for os.CreateTemp.
func tempCopy(pattern string) (*os.File, error) {
	return os.CreateTemp(workDir(), "p"+strconv.Itoa(os.Getpid())+"-"+pattern)
}

// legacyCopyAge is how old a decoded copy loose in the temp folder must be to be removed.
const legacyCopyAge = 24 * time.Hour

// SweepWorkCopies removes the decoded copies that sessions which have ended left behind. It is called once at start, by the editor that holds the save's lock. It returns how
// many files it removed.
func SweepWorkCopies() int {
	removed := 0
	dir := filepath.Join(os.TempDir(), workDirName)
	if ents, err := os.ReadDir(dir); err == nil {
		for _, e := range ents {
			m := copyOfProcess.FindStringSubmatch(e.Name())
			if e.IsDir() || m == nil {
				continue
			}
			pid, _ := strconv.Atoi(m[1])
			if pid == os.Getpid() || processAlive(pid) {
				continue
			}
			if os.Remove(filepath.Join(dir, e.Name())) == nil {
				removed++
			}
		}
	}
	if ents, err := os.ReadDir(os.TempDir()); err == nil {
		for _, e := range ents {
			n := e.Name()
			if e.IsDir() || !strings.HasPrefix(n, "tabr-tau-") || !strings.HasSuffix(n, ".sqlite") {
				continue
			}
			if info, err := e.Info(); err != nil || time.Since(info.ModTime()) < legacyCopyAge {
				continue
			}
			if os.Remove(filepath.Join(os.TempDir(), n)) == nil {
				removed++
			}
		}
	}
	return removed
}
