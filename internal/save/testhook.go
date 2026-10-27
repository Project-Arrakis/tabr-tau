package save

import "time"

// NoGameRunningForTests makes the process check report that no game is running, whatever the machine is doing, so a test does not
// pass or fail depending on whether the developer has Dune open (issue #102). Test binaries call it (internal/testsave does, in
// init); production code never does. It returns a function that puts the real check back. A test that needs a running game
// replaces listImages itself, as the tests in this package do.
func NoGameRunningForTests() (restore func()) {
	oldOS, oldRun, oldLoc := goos, listImages, locationFor
	goos = "windows"
	locationFor = func(string) string { return "" } // a test save lives in a temp folder
	listImages = func() ([]string, error) { return []string{"explorer.exe"}, nil }
	resetGameCache()
	return func() {
		goos, listImages, locationFor = oldOS, oldRun, oldLoc
		resetGameCache()
	}
}

func resetGameCache() {
	gameCache.Lock()
	gameCache.at, gameCache.names, gameCache.err = time.Time{}, nil, ""
	gameCache.goodAt, gameCache.goodNames = time.Time{}, nil
	gameCache.Unlock()
}

// CommitForTests writes the pending edits to the file the way Flush does, but takes a backup of its own every time and does not wait for the quick-backup interval. The game-running
// and unchanged-file checks still apply. A test uses it to put an edit into the file as if it were the game's own save; shipped code writes through Flush, and the guard test
// fails if it calls this.
func (s *Save) CommitForTests() (map[string]any, error) { return s.commit(false) }

// UntrustedLocationForTests makes the editor believe the save is on a network share, so a test in another package can check the confirm-location
// route and the banner. Test code calls it; production code never does (the guard test fails if it does). It returns a function that puts
// the real check back.
func UntrustedLocationForTests() (restore func()) {
	old := locationFor
	locationFor = func(string) string { return "the save is on a network share" }
	return func() { locationFor = old }
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

// GameAtMenuForTests makes the editor believe a Dune process is running while the game sits at its menu: the log shows a session that ended an hour ago and the game's last write
// of the database just after it, and no program has the save file open. An edit made by hand is written then; the automatic pass at launch is not (#449). Test code calls it;
// production code never does (the guard test fails if it does). It returns a function that puts the real check back.
func GameAtMenuForTests() (restore func()) {
	oldL, oldS, oldH := listProcs, statusFor, fileHeld
	listProcs = func(bool) ([]string, string) { return []string{"DuneSandbox-Win64-Shipping.exe"}, "" }
	statusFor = func(string) (spStatus, error) {
		return spStatus{EndedAt: time.Now().Add(-time.Hour), LastSaveAt: time.Now().Add(-time.Hour + 5*time.Second)}, nil
	}
	fileHeld = func(string) (bool, error) { return false, nil }
	return func() { listProcs, statusFor, fileHeld = oldL, oldS, oldH }
}

// GameJustLeftWorldForTests makes the editor believe a Dune process is running and the player left the world 23 seconds ago, the game's last write of the database 20 seconds
// ago (the #419 case: the game is open at the character-selection screen). Edits stay queued. Test code calls it; production code never does (the guard test fails if it does).
// It returns a function that puts the real check back.
func GameJustLeftWorldForTests() (restore func()) {
	oldL, oldS, oldH := listProcs, statusFor, fileHeld
	listProcs = func(bool) ([]string, string) { return []string{"DuneSandbox-Win64-Shipping.exe"}, "" }
	statusFor = func(string) (spStatus, error) {
		return spStatus{EndedAt: time.Now().Add(-23 * time.Second), LastSaveAt: time.Now().Add(-20 * time.Second)}, nil
	}
	fileHeld = func(string) (bool, error) { return false, nil }
	return func() { listProcs, statusFor, fileHeld = oldL, oldS, oldH }
}

// FailedGameCheckForTests makes the process check fail with this reason, so a test in another package can check what the page is told when the game check cannot be made
// (and that the next check that works clears it). Test code calls it; production code never does (the guard test fails if it does). It returns a function that puts the
// real check back.
func FailedGameCheckForTests(reason string) (restore func()) {
	old := listProcs
	listProcs = func(bool) ([]string, string) { return nil, reason }
	return func() { listProcs = old }
}

// DeadlinesForTests sets the time every bounded read may take (the read-only console, Table, Query and One, and the soundness check of a decoded save) to d, so a test can show the
// deadline is applied without waiting for the real one: a negative d is a deadline that has already passed. Test code calls it; production code never does (the guard test fails if
// it does). It returns a function that puts the real values back.
func DeadlinesForTests(d time.Duration) (restore func()) {
	oldR, oldI := ReadTimeout, inspectTimeout
	ReadTimeout, inspectTimeout = d, d
	return func() { ReadTimeout, inspectTimeout = oldR, oldI }
}
