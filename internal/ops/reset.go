package ops

import (
	"github.com/Project-Arrakis/tabr-tau/internal/apierr"
)

// The reset buttons (Skills, Level and XP, Journey) share one shape. The page shows the number of things a reset would change and the
// person types a confirmation that names the area and that number; the request carries the number (expected). The reset reads its
// own state inside the edit and refuses with a conflict when the number is not the one the person confirmed, so a save that changed in
// between (the game saved, another edit was made) cannot be reset on the strength of a stale page. When nothing would change the
// reset writes nothing and says so.

// maxResetCount bounds the count a request may carry; nothing in a save comes near it.
const maxResetCount = 10_000_000

// expectedCount reads the count the person confirmed. It is required: a reset without it is a bad argument, never "reset whatever is there".
func expectedCount(a Args) (int64, error) {
	if _, ok := a["expected"]; !ok {
		return 0, apierr.New(apierr.ErrBadArgument, "expected is required: it is the number the page showed and the person confirmed")
	}
	return a.IntRange("expected", 0, maxResetCount)
}

// resetMismatch is the refusal when the state inside the edit is not what the person confirmed.
func resetMismatch(what string, now, confirmed int64) error {
	return apierr.Newf(apierr.ErrConflict, "%s to reset is now %d, not the %d you confirmed, so nothing was changed: reload the page and look again", what, now, confirmed)
}

// resetAnswer is the answer of every reset: how many things changed, and whether there was nothing to do.
func resetAnswer(changed int64, extra map[string]any) map[string]any {
	out := map[string]any{"ok": true, "changed": changed, "noop": changed == 0}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
