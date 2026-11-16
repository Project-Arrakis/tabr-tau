package save

import (
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/kit/sessionlog"
)

// The pin of the editor's own tracker (spTracker.feed: wide starts and writes, ends only at the start of a line's text, #612) over the same seeded corpus as the pin of the kit's
// tracker in spsession_pin_test.go. The two differ on purpose, only where an end marker stands inside a line of some other category.
const spEditorPinHashWant = "b63aa2d5ea8bdf0aa8b684efe1a4cad41838077e81687a1dde95ef6cf4d6b109"

func TestPinEditorTrackerOverASeededCorpus(t *testing.T) {
	var tr spTracker
	got := spPinHash(func(l string) (bool, time.Time, time.Time) {
		tr.feed(l)
		return tr.SPActive, tr.EndedAt, tr.LastSaveAt
	})
	if got != spEditorPinHashWant {
		t.Fatalf("the editor's tracker answers differently for some line of the corpus: hash %s, pinned %s", got, spEditorPinHashWant)
	}
}

// The editor is never less cautious than the wide reading of the kit: whenever the wide tracker says a session is active, so does the editor's. Its ends are a subset of the wide
// ends and its starts are the same, so a line can only keep the editor active longer, never end a session the wide reading kept open (#612, D2-3).
func TestEditorTrackerIsNeverLessActiveThanTheWideReading(t *testing.T) {
	var wide sessionlog.Tracker
	var ed spTracker
	moreActive := 0
	for i, l := range spPinCorpus() {
		wide.Feed(spPolicy, l, nil)
		ed.feed(l)
		if wide.SPActive && !ed.SPActive {
			t.Fatalf("line %d %q: the wide reading says a session is active and the editor's tracker says it is not", i, l)
		}
		if ed.SPActive && !wide.SPActive {
			moreActive++
		}
	}
	if moreActive == 0 {
		t.Fatal("the corpus never separates the two readings, so this test checks nothing")
	}
}
