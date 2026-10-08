package save

import (
	"context"
	"encoding/csv"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// gamePrefix is the start of every Dune client process name. The exact image name was never verified on a real
// machine, so the check matches the prefix instead of one guessed name; a false match only blocks a save, and the
// message names the process so a wrong match is visible.
const gamePrefix = "dune"

// parseTasklist returns the process names in `tasklist /FO CSV /NH` output that look like the Dune client. The
// "no tasks" notice is not CSV and never matches. tasklist cuts image names to 25 characters; that is fine for a prefix
// match and for display.
func parseTasklist(out string) []string {
	var names []string
	seen := map[string]bool{}
	r := csv.NewReader(strings.NewReader(out))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	recs, _ := r.ReadAll()
	for _, rec := range recs {
		if len(rec) == 0 {
			continue
		}
		n := strings.TrimSpace(rec[0])
		if strings.HasPrefix(strings.ToLower(n), gamePrefix) && !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	return names
}

var gameCache struct {
	sync.Mutex
	at    time.Time
	names []string
	err   string
}

const gameCacheTTL = 3 * time.Second

// GameProcesses lists the running Dune client processes (Windows only). It is for display and polling: the answer
// may be up to three seconds old. Anything that decides whether to write must use GameProcessesNow.
func GameProcesses() []string { return gameProcesses(true) }

// GameProcessesNow is GameProcesses without the cache, for the checks that gate a write.
func GameProcessesNow() []string { return gameProcesses(false) }

// GameCheckError is why the last check could not run ("" when it ran). A check that cannot run counts as the game
// running: a write must not go ahead on a guess.
func GameCheckError() string {
	GameProcesses()
	gameCache.Lock()
	defer gameCache.Unlock()
	return gameCache.err
}

// GameRunning reports whether a Dune client process is alive, or the check could not run (cached, for display).
func GameRunning() bool { return len(GameProcesses()) > 0 || GameCheckError() != "" }

// GameRunningNow reports it without the cache, for checks that gate a write.
func GameRunningNow() bool { return len(GameProcessesNow()) > 0 || GameCheckError() != "" }

// RunningMessage names what was found (or why the check failed), for the refusal shown when a write is blocked.
func RunningMessage() string {
	n := GameProcessesNow()
	if e := GameCheckError(); len(n) == 0 && e != "" {
		return "could not check whether the game is running (" + e + ")"
	}
	if len(n) == 0 {
		return "the game is running"
	}
	return strings.Join(n, ", ") + " is running"
}

// gameProcesses asks tasklist. The check cannot hang: a stuck tasklist is abandoned after three seconds.
func gameProcesses(useCache bool) []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	gameCache.Lock()
	defer gameCache.Unlock()
	if useCache && !gameCache.at.IsZero() && time.Since(gameCache.at) < gameCacheTTL {
		return gameCache.names
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tasklist", "/FI", "IMAGENAME eq "+gamePrefix+"*", "/NH", "/FO", "CSV")
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		gameCache.names, gameCache.err = nil, err.Error()
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			gameCache.err += ": " + strings.TrimSpace(string(ee.Stderr))
		}
	} else {
		gameCache.names, gameCache.err = parseTasklist(string(out)), ""
	}
	gameCache.at = time.Now()
	return gameCache.names
}
