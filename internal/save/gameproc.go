package save

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
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

var gameCache struct {
	sync.Mutex
	at      time.Time
	running bool
}

const gameCacheTTL = 3 * time.Second

// GameRunning reports whether the Dune client process is alive (Windows only). It is for display and polling: the
// answer may be up to three seconds old. Anything that decides whether to write must use GameRunningNow.
func GameRunning() bool { return gameRunning(true) }

// GameRunningNow is GameRunning without the cache, for the checks that gate a write.
func GameRunningNow() bool { return gameRunning(false) }

// gameRunning asks tasklist. The check cannot hang: a stuck tasklist is abandoned after three seconds.
func gameRunning(useCache bool) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	gameCache.Lock()
	defer gameCache.Unlock()
	if useCache && !gameCache.at.IsZero() && time.Since(gameCache.at) < gameCacheTTL {
		return gameCache.running
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tasklist", "/FI", "IMAGENAME eq "+GameProcess, "/NH", "/FO", "CSV")
	hideWindow(cmd)
	out, err := cmd.Output()
	gameCache.running = err == nil && listedInTasklist(string(out))
	gameCache.at = time.Now()
	return gameCache.running
}
