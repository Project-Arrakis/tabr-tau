package ops

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// An acceptance run on a real save (#1176). It is for the person who has one, and it skips unless the variables name it. It copies the save, recycles a standing base in the
// copy with this editor, and checks the result against the pattern of the game's own recycle (the before and after pair of 2026-11-23): rows go from exactly the tables the game
// deleted from (plus the three a standing base still has rows in), no table that the game left alone changes, no base backup row is written, every stored item row is in the
// Reserve unchanged, and the refund is there as new rows. The copy is left where TABR_RECYCLE_OUT says, so that `tabr-tau diff` can compare it with the save it was made from.
// The save is never read from the repository and nothing of it is kept here.
//
//	TABR_RECYCLE_BEFORE     path of the save (it is only read)
//	TABR_RECYCLE_TOTEM      the totem of a standing base of the character's
//	TABR_RECYCLE_OUT        path of the copy to write (must not exist)
//	TABR_RECYCLE_ITEM_ROWS  optional: the number of item rows stored in the base, when it is known from elsewhere
func TestRecycleOnARealSaveMatchesThePatternOfTheGamesOwnRecycle(t *testing.T) {
	src, out, tid := os.Getenv("TABR_RECYCLE_BEFORE"), os.Getenv("TABR_RECYCLE_OUT"), os.Getenv("TABR_RECYCLE_TOTEM")
	if src == "" || out == "" || tid == "" {
		t.Skip("no real save named (TABR_RECYCLE_BEFORE, TABR_RECYCLE_TOTEM, TABR_RECYCLE_OUT)")
	}
	totem, err := strconv.ParseInt(tid, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("the output file exists; name one that does not")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(src)
	if err != nil {
		t.Skipf("the save is not readable here: %v", err)
	}
	g, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(g, f); err != nil {
		t.Fatal(err)
	}
	f.Close()
	g.Close()
	_ = testsave.HostileText // the package's init makes the process check report no game
	s, err := save.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	o := &Ops{S: s}

	counts := func() map[string]int64 {
		tables, err := s.Query(`select name from sqlite_master where type='table' and name not like 'sqlite_%' order by name`)
		if err != nil {
			t.Fatal(err)
		}
		c := map[string]int64{}
		for _, r := range tables {
			name := r["name"].(string)
			n, err := s.One(`select count(*) n from "` + name + `"`)
			if err != nil {
				t.Fatal(err)
			}
			c[name] = n["n"].(int64)
		}
		return c
	}
	before := counts()
	itemsBefore, _ := s.One(`select count(*) n, coalesce(sum(stack_size),0) u from items`)

	pv, err := o.PreviewBaseRecycle(totem)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	p := pv.(map[string]any)
	if fmt.Sprint(counts()) != fmt.Sprint(before) {
		t.Fatal("the preview changed the save")
	}
	if want := os.Getenv("TABR_RECYCLE_ITEM_ROWS"); want != "" && fmt.Sprint(p["itemStacksMoved"]) != want {
		t.Fatalf("the base stores %v item rows, and %s were expected", p["itemStacksMoved"], want)
	}
	// the stored rows, as they are before
	inv, _ := s.Query(`select id from inventories where actor_id=? or actor_id in (select id from placeables where owner_entity_id=(select entity_id from actor_fgl_entities where actor_id=? and slot_name='Actor'))`, totem, totem)
	ids := make([]string, 0, len(inv))
	for _, r := range inv {
		ids = append(ids, fmt.Sprint(r["id"]))
	}
	const cols = `id, stack_size, template_id, is_new, acquisition_time, stats, quality_level, volume_override`
	stored, _ := s.Query(`select ` + cols + ` from items where inventory_id in (` + strings.Join(append(ids, "-1"), ",") + `) order by id`)

	res, err := o.RecycleBase(totem, p["fingerprint"].(string))
	if err != nil {
		t.Fatalf("recycle: %v", err)
	}
	r := res.(map[string]any)
	after := counts()

	// (a) rows go from the tables the game's own recycle deleted from, and from no other
	gamePattern := map[string]bool{"actor_fgl_entities": true, "actor_inventories": true, "actors": true, "building_instances": true, "buildings": true, "fgl_entities": true, "inventories": true,
		"permission_actor": true, "placeables": true, "totem_fuel_update_times": true, "totems": true}
	standingOnly := map[string]bool{"permission_actor_rank": true, "landclaim_segments": true, "player_markers": true} // the measured base was already picked up: these rows had gone with the pick-up
	deleted := r["deleted"].(map[string]int64)
	var changed []string
	for name, n := range before {
		d := n - after[name]
		switch {
		case name == "items":
			if -d != r["refundStacks"].(int64) {
				t.Errorf("items: %d rows more, and the refund is %v stacks", -d, r["refundStacks"])
			}
		case name == "items_id_sequencer":
		case d == 0:
			if deleted[name] != 0 {
				t.Errorf("%s: the answer says %d rows were deleted and the table did not shrink", name, deleted[name])
			}
		case d != deleted[name] || !(gamePattern[name] || standingOnly[name]):
			t.Errorf("%s: %d rows fewer (the answer says %d): not part of the pattern", name, d, deleted[name])
		default:
			changed = append(changed, fmt.Sprintf("%s -%d", name, d))
		}
	}
	for name := range gamePattern {
		if before[name]-after[name] <= 0 {
			t.Errorf("%s: the game's recycle deletes rows there and this edit deleted none", name)
		}
	}
	if before["base_backups"] != after["base_backups"] || before["base_backup_linked_actors"] != after["base_backup_linked_actors"] {
		t.Error("a base backup row was written or deleted")
	}
	if before["vehicles"] != after["vehicles"] || before["vehicle_modules"] != after["vehicle_modules"] {
		t.Error("a vehicle was touched")
	}
	sort.Strings(changed)

	// (c) every stored row is in the Reserve, unchanged but for the inventory and the position; none is lost
	storedIDs := make([]string, 0, len(stored))
	for _, row := range stored {
		storedIDs = append(storedIDs, fmt.Sprint(row["id"]))
	}
	now, _ := s.Query(`select `+cols+` from items where inventory_id=? and id in (`+strings.Join(append(storedIDs, "-1"), ",")+`) order by id`, r["reserveInventory"])
	if len(now) != len(stored) || fmt.Sprint(now) != fmt.Sprint(stored) || int64(len(stored)) != r["itemStacksMoved"].(int64) {
		t.Errorf("the stored rows did not all arrive unchanged: %d before, %d in the Reserve, the answer says %v", len(stored), len(now), r["itemStacksMoved"])
	}
	itemsAfter, _ := s.One(`select count(*) n, coalesce(sum(stack_size),0) u from items`)
	var refundUnits int64
	var refund []string
	for _, l := range r["refund"].([]RefundLine) {
		refundUnits += l.Count
		refund = append(refund, fmt.Sprintf("%s %d", l.Item, l.Count))
	}
	if itemsAfter["u"].(int64)-itemsBefore["u"].(int64) != refundUnits {
		t.Errorf("the items of the save grew by %d and the refund is %d", itemsAfter["u"].(int64)-itemsBefore["u"].(int64), refundUnits)
	}

	t.Logf("base %d %q on %v: %v pieces, %v placeables (%v holograms), %v actors", totem, r["name"], r["map"], r["pieces"], r["placeables"], r["holograms"], r["actors"])
	t.Logf("rows deleted: %s", strings.Join(changed, ", "))
	t.Logf("moved to the Reserve: %v item rows, %v items; water dropped: %v in %v devices", r["itemStacksMoved"], r["itemsMoved"], r["waterDropped"], r["waterDevices"])
	t.Logf("refund (%v%% of the table cost, %v stacks): %s", r["costPercent"], r["refundStacks"], strings.Join(refund, ", "))
	for _, w := range r["summary"].([]string) {
		t.Log(w)
	}
	t.Logf("warnings: %v", r["warnings"])
	if _, err := s.CommitForTests(); err != nil {
		t.Fatal(err)
	}
}
