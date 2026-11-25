package ops

import (
	"strings"
	"testing"
)

// The automatic blood-sack job says that its fill is not proven in the game (#1238).
func TestBloodJobSaysUnchecked(t *testing.T) {
	for _, j := range AutoJobs() {
		if j.ID == "fill-blood" {
			if !strings.Contains(j.Desc, "Not yet checked in the game.") {
				t.Fatalf("the blood job does not say it is unchecked: %q", j.Desc)
			}
			return
		}
	}
	t.Fatal("no fill-blood job")
}
