package save

import (
	"os/exec"
	"runtime"
	"strings"
)

// tasklist cuts the image name in its table to 25 characters, so a 30-character name such as GameProcess never
// appears whole in its output. Matching on this prefix works with the cut and the uncut form alike.
const tasklistNameLen = 25

// listedInTasklist reports whether tasklist output names the game. The "no tasks" notice carries no process name,
// so it can never match.
func listedInTasklist(out string) bool {
	want := strings.ToLower(GameProcess)
	if len(want) > tasklistNameLen {
		want = want[:tasklistNameLen]
	}
	return strings.Contains(strings.ToLower(out), want)
}

// GameRunning reports whether the Dune client process is alive (Windows only).
func GameRunning() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+GameProcess, "/NH", "/FO", "CSV")
	hideWindow(cmd)
	out, err := cmd.Output()
	return err == nil && listedInTasklist(string(out))
}
