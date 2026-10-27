package save

import (
	"errors"
	"runtime"
	"strings"
	"sync"
	"time"
)

// gamePrefix is the start of every Dune client process name. The exact image name was never verified on a real
// machine, so the check matches the prefix instead of one guessed name; a false match only blocks a save, and the
// message names the process so a wrong match is visible.
const gamePrefix = "dune"

// matchGameImages returns the process names, from a list of every running process image, that look like the Dune client
// (the same prefix rule as before, case-insensitive, each name once, in the order found).
func matchGameImages(images []string) []string {
	var names []string
	seen := map[string]bool{}
	for _, img := range images {
		n := strings.TrimSpace(img)
		if strings.HasPrefix(strings.ToLower(n), gamePrefix) && !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	return names
}

var gameCache struct {
	sync.Mutex
	at        time.Time // when the answer below was taken
	names     []string
	err       string
	goodAt    time.Time // when the last check that worked was taken (zero: none yet)
	goodNames []string
}

const (
	// gameCacheTTL is how long the status poll reuses an answer. The poll runs every few seconds and only draws the banner; a write never reads the cache.
	gameCacheTTL = 15 * time.Second
	// gameErrTTL is how long a failed answer is reused: short, so the banner clears at the next poll once the check works again.
	gameErrTTL = 2 * time.Second
	// gameLastGoodFor is how long the status poll keeps showing the last answer that worked when a later check fails, so one failure does not flip the banner.
	// A check that gates a write never uses it.
	gameLastGoodFor = 30 * time.Second
	// checkAttempts and checkRetryDelay: a check that fails is made again after a short pause before it blocks anything (#366).
	checkAttempts   = 3
	checkRetryDelay = 250 * time.Millisecond
	// checkTimeout bounds one attempt; the process list is read in memory and answers in milliseconds.
	checkTimeout = 3 * time.Second
)

// GameProcesses lists the running Dune client processes (Windows only). It is for display and polling: the answer
// may be up to gameCacheTTL old. Anything that decides whether to write must use GameProcessesNow.
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

// goos, listImages and sleepFunc are variables so a test can stand in for the machine. listImages returns the image name of every running process; it reads the
// system's process snapshot in memory (gameproc_windows.go) and starts no program.
var (
	goos       = runtime.GOOS
	listImages = processImages
	sleepFunc  = time.Sleep
)

// listWithTimeout is one attempt. The snapshot cannot hang in practice; the bound keeps the lock from being held for ever if it ever did.
func listWithTimeout() ([]string, error) {
	type res struct {
		names []string
		err   error
	}
	ch := make(chan res, 1)
	go func() {
		n, err := listImages()
		ch <- res{n, err}
	}()
	select {
	case r := <-ch:
		return r.names, r.err
	case <-time.After(checkTimeout):
		return nil, errors.New("the process list did not answer in time")
	}
}

// checkProcesses reads the process list, trying again after a short pause when it fails, and returns the Dune processes among them. Only a check that failed on every
// attempt returns an error (the last one's reason).
func checkProcesses() ([]string, error) {
	var err error
	for i := 0; i < checkAttempts; i++ {
		if i > 0 {
			sleepFunc(checkRetryDelay)
		}
		var images []string
		if images, err = listWithTimeout(); err == nil {
			return matchGameImages(images), nil
		}
	}
	return nil, err
}

// gameProcesses reads the process snapshot. A check that failed is not final: it is repeated (checkAttempts tries). With useCache (the status poll) an answer younger than
// gameCacheTTL is reused, and when a check fails right after one that worked the old answer stays on show for gameLastGoodFor so that one failure does not flip the
// banner. Without it (anything that gates a write) the machine is asked now, and a check that fails on every try is an error: fail closed.
func gameProcesses(useCache bool) []string {
	if goos != "windows" {
		return nil
	}
	gameCache.Lock()
	defer gameCache.Unlock()
	if useCache && !gameCache.at.IsZero() {
		ttl := gameCacheTTL
		if gameCache.err != "" {
			ttl = gameErrTTL
		}
		if time.Since(gameCache.at) < ttl {
			return gameCache.names
		}
	}
	names, err := checkProcesses()
	switch {
	case err == nil:
		gameCache.names, gameCache.err = names, ""
		gameCache.goodNames, gameCache.goodAt = names, time.Now()
	case useCache && !gameCache.goodAt.IsZero() && time.Since(gameCache.goodAt) < gameLastGoodFor:
		// keep showing the last good answer; do not store the failure, so the next poll tries again
		return gameCache.goodNames
	default:
		gameCache.names, gameCache.err = nil, err.Error()
	}
	gameCache.at = time.Now()
	return gameCache.names
}

// GameCheck is the answer to "Check again": what a check made right now found, in a sentence the page can show.
type GameCheck struct {
	Running    bool     `json:"running"`    // a Dune process was found
	Failed     bool     `json:"failed"`     // the process list could not be read (after the retries)
	Blocked    bool     `json:"blocked"`    // an edit would queue instead of being written
	Processes  []string `json:"processes"`  // the Dune processes found
	CheckError string   `json:"checkError"` // why the check failed, when it did
	Message    string   `json:"message"`
}

// CheckGameNow makes the game check at once, never from the cache, and says what it found. It is what Write now and Check again call; the answer is also what the
// status poll shows next (a failed check is replaced by the next one that works).
func CheckGameNow(savePath string) GameCheck {
	g := GameStateFor(savePath, true)
	c := GameCheck{Running: len(g.Processes) > 0, Blocked: g.Blocked, Processes: g.Processes, CheckError: g.CheckError}
	if c.Processes == nil {
		c.Processes = []string{}
	}
	switch {
	case g.CheckError != "" && !c.Running:
		c.Failed = true
		c.Message = "the check failed again: " + g.CheckError
	case c.Running && g.Blocked:
		c.Message = "the game is open and edits will queue: " + g.Reason
	case c.Running:
		c.Message = "the game is open at its menu, no world is loaded and no program has the save file open: edits are written at once"
	case g.Blocked:
		c.Message = "the game is not running, but " + g.Reason + ": edits will queue"
	default:
		c.Message = "the game is not running: edits are written at once"
	}
	return c
}
