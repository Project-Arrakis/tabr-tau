package save

import (
	"os"
	"strings"
	"testing"
)

// The standard place, a UNC path, a mapped or removable drive, and anywhere else the game does not keep its saves (NET-3 of the 2026-10-15
// audit). pathProblem reads the spelling only, so it is tested on every system.
func TestPathProblemRecognisesTheStandardLayoutAndRefusesTheRest(t *testing.T) {
	good := []string{
		`C:\Users\a\AppData\Local\DuneSandbox\Saved\Cloud\PlayerClientStorage\FLS_retail\12345\game.db`,
		`c:\users\a\appdata\local\dunesandbox\saved\cloud\playerclientstorage\fls_retail\12345\game.db`,
		`D:/Games/Dune/Saved/Cloud/PlayerClientStorage/FLS_retail/abc/game.db`,
		`\\?\C:\Users\a\Dune\Saved\Cloud\PlayerClientStorage\FLS_retail\12345\game_prepatch.db`,
	}
	for _, p := range good {
		if why := pathProblem(p); why != "" {
			t.Errorf("%s should be trusted: %s", p, why)
		}
	}
	bad := map[string]string{
		`\\nas\games\Saved\Cloud\PlayerClientStorage\FLS_retail\12345\game.db`:       "network share",
		`//nas/games/Saved/Cloud/PlayerClientStorage/FLS_retail/12345/game.db`:       "network share",
		`\\?\UNC\nas\games\Saved\Cloud\PlayerClientStorage\FLS_retail\12345\game.db`: "network share",
		`C:\Users\a\Desktop\game.db`: "not in the game's own",
		`C:\Users\a\AppData\Local\DuneSandbox\Saved\Cloud\PlayerClientStorage\FLS_retail\game.db`:              "not in the game's own",
		`C:\Dropbox\Saved\Cloud\Other\FLS_retail\12345\game.db`:                                                "not in the game's own",
		`C:\Users\a\AppData\Local\DuneSandbox\Saved\Cloud\PlayerClientStorage\FLS_retail\12345\autosave\x.bak`: "not in the game's own",
		`game.db`: "not in the game's own",
		``:        "not in the game's own",
	}
	for p, want := range bad {
		if why := pathProblem(p); !strings.Contains(why, want) {
			t.Errorf("%q: got %q, want it to say %q", p, why, want)
		}
	}
}

// A save the guard cannot vouch for is blocked even when no game is running here, and the person's confirmation lifts it.
func TestAnUntrustedLocationBlocksUntilConfirmed(t *testing.T) {
	oldOS, oldLoc := goos, locationFor
	goos, locationFor = "windows", defaultLocationFor
	t.Cleanup(func() { goos, locationFor = oldOS, oldLoc })
	fakeGame(t, nil, "", spStatus{}, nil) // no game on this computer

	p := t.TempDir() + string(os.PathSeparator) + "game.db" // a temp folder is not the game's folder
	g := GameStateFor(p, true)
	if !g.Blocked || !g.NeedsConfirm || g.Mode != "unknown" || !strings.Contains(g.Reason, "cannot tell whether the game is using it") {
		t.Fatalf("an untrusted location must block and ask: %+v", g)
	}
	if blocked, why := SaveBlocked(p); !blocked || !strings.Contains(why, "confirm") {
		t.Fatalf("a write is blocked too: %v %q", blocked, why)
	}
	ConfirmLocation(p)
	if g := GameStateFor(p, true); g.Blocked || g.NeedsConfirm {
		t.Fatalf("after the confirmation the location is accepted: %+v", g)
	}
	other := t.TempDir() + string(os.PathSeparator) + "game.db"
	if g := GameStateFor(other, true); !g.Blocked {
		t.Fatalf("the confirmation is for that save only: %+v", g)
	}
}

// A real reason to block (a single-player session here) is not hidden behind the location question.
func TestARunningSessionStillOutranksTheLocationQuestion(t *testing.T) {
	oldOS, oldLoc := goos, locationFor
	goos, locationFor = "windows", defaultLocationFor
	t.Cleanup(func() { goos, locationFor = oldOS, oldLoc })
	fakeGame(t, duneProc, "", spStatus{Active: true}, nil)
	g := GameStateFor(t.TempDir()+string(os.PathSeparator)+"game.db", true)
	if !g.Blocked || g.NeedsConfirm || g.Mode != "single_player" {
		t.Fatalf("%+v", g)
	}
}

// The check shipped in the program is the real one, not the stand-in the tests use.
func TestTheShippedLocationCheckIsOn(t *testing.T) {
	oldOS := goos
	goos = "windows"
	t.Cleanup(func() { goos = oldOS })
	if why := defaultLocationFor(`\\nas\share\Saved\Cloud\PlayerClientStorage\FLS_retail\1\game.db`); !strings.Contains(why, "network share") {
		t.Fatalf("the shipped check must refuse a share: %q", why)
	}
	goos = "linux"
	if why := defaultLocationFor(`\\nas\share\game.db`); why != "" {
		t.Fatalf("off Windows the guard does nothing, as before: %q", why)
	}
}
