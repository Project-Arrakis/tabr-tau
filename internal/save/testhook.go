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
	oldOS, oldRun, oldLoc := goos, tasklistOutput, locationFor
	goos = "windows"
	locationFor = func(string) string { return "" } // a test save lives in a temp folder
	tasklistOutput = func(context.Context) ([]byte, error) {
		return []byte("INFO: No tasks are running which match the specified criteria."), nil
	}
	resetGameCache()
	return func() {
		goos, tasklistOutput, locationFor = oldOS, oldRun, oldLoc
		resetGameCache()
	}
}

func resetGameCache() {
	gameCache.Lock()
	gameCache.at, gameCache.names, gameCache.err = time.Time{}, nil, ""
	gameCache.Unlock()
}

// ActiveSessionForTests makes the editor believe a Dune process is running with a single-player session active, so a test in another
// package can check that a write path refuses. Test code calls it; production code never does (TestTestHooksAreNotCalledByProductionCode
// fails if it does). It returns a function that puts the real check back.
func ActiveSessionForTests() (restore func()) {
	oldL, oldS := listProcs, statusFor
	listProcs = func(bool) ([]string, string) { return []string{"DuneSandbox-Win64-Shipping.exe"}, "" }
	statusFor = func(string) (spStatus, error) { return spStatus{Active: true}, nil }
	return func() { listProcs, statusFor = oldL, oldS }
}
