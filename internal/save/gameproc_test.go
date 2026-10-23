package save

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseTasklist(t *testing.T) {
	cases := []struct {
		name, out string
		want      []string
	}{
		{"csv cut to 25", `"DuneSandbox-Win64-Shippin","4242","Console","1","1,234,567 K"`, []string{"DuneSandbox-Win64-Shippin"}},
		{"full name", `"DuneSandbox-Win64-Shipping.exe","4242","Console","1","9 K"`, []string{"DuneSandbox-Win64-Shipping.exe"}},
		{"a different dune process", `"DuneAwakening.exe","1","Console","1","9 K"`, []string{"DuneAwakening.exe"}},
		{"mixed case", `"dunesandbox.exe","1"`, []string{"dunesandbox.exe"}},
		{"two processes, one repeated", "\"DuneA.exe\",\"1\"\r\n\"DuneB.exe\",\"2\"\r\n\"DuneA.exe\",\"3\"\r\n", []string{"DuneA.exe", "DuneB.exe"}},
		{"no match notice", "INFO: No tasks are running which match the specified criteria.", nil},
		{"other process", `"notepad.exe","1"`, nil},
		{"our own program is not the game", `"tabr-tau-window.exe","1"`, nil},
		{"empty", "", nil},
	}
	for _, c := range cases {
		if got := parseTasklist(c.out); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

// A check that cannot run must block writes and say why, not pass as "game closed". The failure is injected (on every
// OS), so this does not depend on whether the real tasklist works.
func TestFailedGameCheckBlocks(t *testing.T) {
	oldOS, oldRun := goos, tasklistOutput
	goos = "windows"
	tasklistOutput = func(context.Context) ([]byte, error) { return nil, errors.New("exec: tasklist: not found") }
	gameCache.Lock()
	gameCache.at, gameCache.names, gameCache.err = time.Time{}, nil, ""
	gameCache.Unlock()
	defer func() {
		goos, tasklistOutput = oldOS, oldRun
		gameCache.Lock()
		gameCache.at, gameCache.names, gameCache.err = time.Time{}, nil, ""
		gameCache.Unlock()
	}()
	if !GameRunningNow() || !GameRunning() {
		t.Fatal("a failed check must count as the game running")
	}
	if m := RunningMessage(); !strings.Contains(m, "could not check") || !strings.Contains(m, "tasklist") {
		t.Fatalf("message does not say the check failed: %q", m)
	}
}

// A write asks the machine again every time (fresh); only the status poll may use the answer of the last three seconds. This pins the direction of the flag (ARCH-4 of the
// 2026-10-22 audit read it as inverted; it was fixed in #242).
func TestAWriteGateAsksTheProcessListAgainAndTheStatusPollMayUseTheCache(t *testing.T) {
	defer NoGameRunningForTests()()
	calls := 0
	tasklistOutput = func(context.Context) ([]byte, error) {
		calls++
		return []byte("INFO: No tasks are running which match the specified criteria."), nil
	}
	resetGameCache()
	p := filepath.Join(t.TempDir(), "game.db")
	GameStateFor(p, true)
	GameStateFor(p, true)
	GameStateFor(p, true)
	if calls != 3 {
		t.Fatalf("each write check asks the process list itself, asked %d times in 3", calls)
	}
	resetGameCache()
	calls = 0
	GameStateFor(p, false)
	GameStateFor(p, false)
	GameStateFor(p, false)
	if calls != 1 {
		t.Fatalf("the status poll reuses the answer of the last three seconds, asked %d times in 3", calls)
	}
	if blocked, _ := SaveBlocked(p); blocked || calls != 2 {
		t.Fatalf("SaveBlocked (the write gate) is never answered from the cache: blocked=%v calls=%d", blocked, calls)
	}
}
