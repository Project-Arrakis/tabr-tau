package save

import (
	"strings"
	"testing"
	"time"
)

// #1128: the reason of a wait is short ("the game saved 21 seconds ago; they are written in about 39 seconds, or when the game closes"); the technical account moves to Detail,
// which the page shows under Details. The waits themselves are the same as before (the table in TestGameStateDecisions holds the times).
func TestWaitReasonsAreShortAndTheDetailIsKept(t *testing.T) {
	oldList, oldStatus, oldNow, oldHeld, oldTime := listProcs, statusFor, nowFunc, fileHeld, fileTimeFor
	t.Cleanup(func() {
		listProcs, statusFor, nowFunc, fileHeld, fileTimeFor = oldList, oldStatus, oldNow, oldHeld, oldTime
	})
	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return now }
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	listProcs = func(bool) ([]string, string) { return []string{"DuneSandbox-Win64-Shipping.exe"}, "" }
	fileHeld = func(string) (bool, error) { return false, nil }
	fileTimeFor = func(string) (time.Time, bool, error) { return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), false, nil }
	cases := []struct {
		name       string
		status     spStatus
		reason     string // the whole Reason
		detailHas  string // a technical word that stays, in Detail
		reasonLack string // a technical word that must not be in the Reason
	}{
		{"the grace after a clean exit", spStatus{EndedAt: ago(24 * time.Second), LastSaveAt: ago(21 * time.Second)},
			"the game saved 21 seconds ago; they are written in about 39 seconds, or when the game closes", "the last write after the single-player session ended 24 seconds ago", "database"},
		{"the flat quiet time", spStatus{EndedAt: ago(100 * time.Second), LastSaveAt: ago(78 * time.Second), WritesAfterEnd: 5},
			"the game saved 78 seconds ago; they are written in about 102 seconds, or when the game closes", "must be quiet for 3 minutes", "every 20 seconds"},
		{"the fallback with no write after the end", spStatus{EndedAt: ago(4 * time.Minute)},
			"you left the world 4 minutes ago; they are written in about 2 minutes, or when the game closes", "does not show the game's last write", "log"},
		{"a session is active", spStatus{Active: true},
			"a single-player session is active; they are written when this clears, or when the game closes", "rewrites the save about every 20 seconds", "every 20 seconds"},
	}
	for _, c := range cases {
		statusFor = func(string) (spStatus, error) { return c.status, nil }
		g := GameStateFor("x", true)
		if !g.Blocked || g.Reason != c.reason {
			t.Errorf("%s: reason %q, want %q", c.name, g.Reason, c.reason)
		}
		if !strings.Contains(g.Detail, c.detailHas) {
			t.Errorf("%s: the detail %q lost %q", c.name, g.Detail, c.detailHas)
		}
		if strings.Contains(g.Reason, c.reasonLack) {
			t.Errorf("%s: the reason %q still carries %q", c.name, g.Reason, c.reasonLack)
		}
		if len(g.Reason) >= len(g.Detail) {
			t.Errorf("%s: the reason (%d bytes) must be shorter than the detail (%d)", c.name, len(g.Reason), len(g.Detail))
		}
	}
}

// WaitFact is what the automatic pass says it waits for: the reason without its sentence about the queue, in both shapes.
func TestWaitFactTakesTheQueueSentenceOff(t *testing.T) {
	for in, want := range map[string]string{
		"the game saved 21 seconds ago; they are written in about 39 seconds, or when the game closes": "the game saved 21 seconds ago",
		"a single-player session is active" + QueuedUntilClears:                                        "a single-player session is active",
		"a reason that is neither": "a reason that is neither",
	} {
		if got := WaitFact(in); got != want {
			t.Errorf("WaitFact(%q) = %q, want %q", in, got, want)
		}
	}
}
