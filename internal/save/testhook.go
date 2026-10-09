package save

import (
	"context"
	"time"
)

// NoGameRunningForTests makes the process check report that no game is running, whatever the machine is doing, so a test does not
// pass or fail depending on whether the developer has Dune open (issue #102). Test binaries call it (internal/testsave does, in
// init); production code never does. It returns a function that puts the real check back. A test that needs a running game
// replaces tasklistOutput itself, as the tests in this package do.
func NoGameRunningForTests() (restore func()) {
	oldOS, oldRun := goos, tasklistOutput
	goos = "windows"
	tasklistOutput = func(context.Context) ([]byte, error) {
		return []byte("INFO: No tasks are running which match the specified criteria."), nil
	}
	resetGameCache()
	return func() {
		goos, tasklistOutput = oldOS, oldRun
		resetGameCache()
	}
}

func resetGameCache() {
	gameCache.Lock()
	gameCache.at, gameCache.names, gameCache.err = time.Time{}, nil, ""
	gameCache.Unlock()
}
