package ops

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// A dry run of Recycle base on a picked-up base of a real save (#1207). It is for the person who has one, and it skips unless the variables name it. It opens the save (the editor
// works on its own copy in memory and a preview is rolled back), asks the question the page asks, and prints what the recycle would delete, move and give back. It writes nothing to
// the file and keeps nothing of it.
//
//	TABR_PICKEDUP_SAVE    path of the save (it is only read; name a copy, never the live save)
//	TABR_PICKEDUP_TOTEM   the totem of a picked-up base of the character's
func TestPreviewOfAPickedUpBaseOnARealSave(t *testing.T) {
	src, tid := os.Getenv("TABR_PICKEDUP_SAVE"), os.Getenv("TABR_PICKEDUP_TOTEM")
	if src == "" || tid == "" {
		t.Skip("no real save named (TABR_PICKEDUP_SAVE, TABR_PICKEDUP_TOTEM)")
	}
	totem, err := strconv.ParseInt(tid, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	_ = testsave.HostileText // the package's init makes the process check report no game
	s, err := save.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	o := &Ops{S: s}
	for _, c := range []struct {
		name string
		opt  RecycleOptions
	}{
		{"as the page asks (the Reserve must be empty)", RecycleOptions{}},
		{"with the Reserve rule off (what the plan holds)", RecycleOptions{AllowNonEmptyReserve: true}},
		{"with a Solido, Reserve rule off", RecycleOptions{Solido: true, AllowNonEmptyReserve: true}},
	} {
		pv, err := o.PreviewBaseRecycleWith(totem, c.opt)
		fmt.Printf("== %s\n", c.name)
		if err != nil {
			fmt.Printf("   refused: %v\n", err)
			continue
		}
		p := pv.(map[string]any)
		fmt.Printf("   pickedUp=%v backup=%v map=%v costPercent=%v pieces=%v placeables(without totem)=%v holograms=%v devStorage=%v actors=%v\n", p["pickedUp"], p["backup"], p["map"], p["costPercent"], p["pieces"], p["placeables"], p["holograms"], p["devStorage"], p["actors"])
		fmt.Printf("   item stacks moved to the Reserve=%v (items %v), refund stacks=%v, water dropped=%v in %v devices, respawn points=%v, marker removed=%v\n", p["itemStacksMoved"], p["itemsMoved"], p["refundStacks"], p["waterDropped"], p["waterDevices"], p["respawnPoints"], p["markerRemoved"])
		fmt.Printf("   deleted rows by table: %v\n", p["deleted"])
		for _, l := range p["refund"].([]RefundLine) {
			fmt.Printf("   refund %s: %d (%d stacks)\n", l.Item, l.Count, l.Stacks)
		}
		if sol, ok := p["solido"].(map[string]any); ok {
			fmt.Printf("   solido: %v\n", sol)
		}
		for _, w := range p["warnings"].([]string) {
			fmt.Printf("   warning: %s\n", w)
		}
	}
	if n := len(s.Ops()); n != 0 {
		t.Fatalf("a preview queued %d edits", n)
	}
}
