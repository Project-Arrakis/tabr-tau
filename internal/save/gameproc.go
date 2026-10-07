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

// GameRunning reports whether the Dune client process is alive (Windows only). The answer is cached for a few
// seconds (the UI polls it) and the check cannot hang: a stuck tasklist is abandoned after three seconds.
func GameRunning() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	gameCache.Lock()
	defer gameCache.Unlock()
	if !gameCache.at.IsZero() && time.Since(gameCache.at) < gameCacheTTL {
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
