package save

import (
	"path/filepath"
	"strings"
	"sync"
)

// The game-running guard looks at this computer: its process list, and the game's log found next to the save. A save on a network share, on
// a removable drive or anywhere outside the game's own Saved\Cloud\PlayerClientStorage folder may be in use by a game on another computer, or
// by one whose log is not where the guard looks, and then the guard sees nothing and says "no game" (NET-3 of the 2026-10-15 audit). Such a
// save is not trusted: writes are off until the person confirms, in the page, that the game is closed everywhere that can open it.

// locationFor says why the save's location cannot be trusted ("" when it can). It is a variable so tests can stand in for a machine
// where it can.
var locationFor = locationProblem

// locationProblem is the check itself, for a Windows machine (like the process check, it does nothing elsewhere).
func locationProblem(savePath string) string {
	if goos != "windows" {
		return ""
	}
	p := savePath
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if real, err := filepath.EvalSymlinks(p); err == nil { // a link or junction to a share is the share
		p = real
	}
	if why := pathProblem(p); why != "" {
		return why
	}
	return driveProblem(p)
}

// pathProblem looks at the spelling of a Windows path only: a UNC path, or a place that is not
// ...\Saved\Cloud\PlayerClientStorage\<service>\<id>\<file>. It does not depend on the system it runs on.
func pathProblem(p string) string {
	s := strings.ReplaceAll(p, `\`, "/")
	if strings.HasPrefix(s, "//?/") { // \\?\C:\... is a local path; \\?\UNC\... is a share
		s = s[len("//?/"):]
		if len(s) >= 4 && strings.EqualFold(s[:4], "UNC/") {
			return "the save is on a network share"
		}
	} else if strings.HasPrefix(s, "//") {
		return "the save is on a network share"
	}
	var parts []string
	for _, e := range strings.Split(s, "/") {
		if e != "" {
			parts = append(parts, e)
		}
	}
	n := len(parts)
	if n < 6 || !strings.EqualFold(parts[n-6], "Saved") || !strings.EqualFold(parts[n-5], "Cloud") || !strings.EqualFold(parts[n-4], "PlayerClientStorage") {
		return `the save is not in the game's own Saved\Cloud\PlayerClientStorage folder`
	}
	return ""
}

// locationOK holds the saves whose location the person confirmed this run, by cleaned lower-case path.
var locationOK sync.Map

func locationKey(savePath string) string {
	if abs, err := filepath.Abs(savePath); err == nil {
		savePath = abs
	}
	return strings.ToLower(filepath.Clean(savePath))
}

// ConfirmLocation records that the person said the game is closed everywhere that can open this save, so a location the guard cannot
// vouch for no longer blocks writes. It lasts until the editor closes.
func ConfirmLocation(savePath string) { locationOK.Store(locationKey(savePath), true) }

func locationConfirmed(savePath string) bool {
	_, ok := locationOK.Load(locationKey(savePath))
	return ok
}
