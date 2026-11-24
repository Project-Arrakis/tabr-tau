package ops

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// An acceptance run on copies of real saves (#1174): the vehicles of the player that stand in the Deep Desert are taken out of a real save, and only the rows meant change. It is for the
// person who has such saves and it skips unless TABR_VEHICLES_REAL_DIR names a folder that holds them, one folder for each save, each with a game.db in it. The saves are never read from
// the repository and nothing of them is kept here; each is copied to a temporary folder first and only the copy is opened. The folders it looks for are matched by name:
//
//	*scout-standing*  a save in which the player's Scout stood in the world, with other vehicles in the Deep Desert
//	*scout-stored*    the save the game wrote after it put that Scout in the vehicle backup (state 2 and one backup row)
//	*before-bring*    a save with vehicles in the Deep Desert, before Bring vehicle moved one
func TestVehiclesOutOfTheDeepDesertOnCopiesOfRealSaves(t *testing.T) {
	dir := os.Getenv("TABR_VEHICLES_REAL_DIR")
	if dir == "" {
		t.Skip("no folder of real saves named (TABR_VEHICLES_REAL_DIR)")
	}
	ran := 0
	for _, pat := range []string{"*scout-standing*", "*scout-stored*", "*before-bring*", "*after-second-bring*"} {
		found, _ := filepath.Glob(filepath.Join(dir, pat, "game.db"))
		for _, src := range found {
			ran++
			name := filepath.Base(filepath.Dir(src))
			t.Run(name, func(t *testing.T) {
				cp := filepath.Join(t.TempDir(), "game.db")
				f, err := os.Open(src)
				if err != nil {
					t.Skipf("not readable here: %v", err)
				}
				g, err := os.Create(cp)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.Copy(g, f); err != nil {
					t.Fatal(err)
				}
				f.Close()
				g.Close()
				s, err := save.Open(cp)
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()
				o := &Ops{S: s}
				h0, err := o.DeepDesertHolds()
				if err != nil {
					t.Fatal(err)
				}
				if h0.Character != nil {
					t.Skipf("the character is in the Deep Desert in this save: nothing can be placed")
				}
				before := snapshot(t, s)
				ps, err := o.PlanDeepDesertVehicles()
				if err != nil {
					t.Fatalf("plan: %v", err)
				}
				t.Logf("%d vehicle(s) in the Deep Desert", len(ps))
				for _, l := range VehicleMoveLines(ps) {
					t.Log("  " + l)
				}
				if len(ps) != len(h0.Vehicles) {
					t.Fatalf("the guard finds %d vehicle(s), the plan %d", len(h0.Vehicles), len(ps))
				}
				if got := changedRows(before, snapshot(t, s)); len(got) != 0 {
					t.Fatalf("the plan changed rows: %v", got)
				}
				if len(ps) == 0 {
					return
				}
				done, err := o.MoveDeepDesertVehicles()
				if err != nil || len(done) != len(ps) {
					t.Fatalf("move: %v", err)
				}
				// only actors rows and (at most) one backup row changed, and the guard finds nothing
				stored := 0
				for _, v := range done {
					if v.Action == "store" {
						stored++
					}
				}
				got := changedRows(before, snapshot(t, s))
				for _, r := range got {
					if !strings.HasPrefix(r[2:], "actors ") && !strings.HasPrefix(r[2:], "backup_vehicles ") {
						t.Errorf("a row outside actors and backup_vehicles changed: %s", r)
					}
				}
				if c := changeCounts(got); c["+ actors"] != len(done) || c["- actors"] != len(done) || c["+ backup_vehicles"] != stored || c["- backup_vehicles"] != 0 {
					t.Errorf("rows changed: %v", c)
				}
				h, err := o.DeepDesertHolds()
				if err != nil || len(h.Vehicles) != 0 { // a base of the player may still stand there; the vehicles must not
					t.Errorf("the guard still finds a vehicle: %v %+v", err, h)
				}
				if err := o.VerifyVehicleMoves(done); err != nil {
					t.Error(err)
				}
			})
		}
	}
	if ran == 0 {
		t.Skip("none of the named saves is in the folder")
	}
}
