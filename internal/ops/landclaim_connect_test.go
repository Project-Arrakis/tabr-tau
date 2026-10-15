package ops

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// A claim of the totem's own cell plus a corridor that goes out along x and bends back in: (1,0) (2,0) (3,0) (3,1) (3,2) (2,2). Cell (2,2)
// is only linked to the totem through (3,2), (3,1) and (3,0).
const corridorFixture = `
insert into actors(id,class,map) values (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin');
insert into totems(id,landclaim_vertical_level,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z,landclaim_original_global_yaw_rotation) values (601,0,0.0,0.0,0.0,0.0);
insert into landclaim_segments(totem_id,grid_location_x,grid_location_y) values (601,1,0),(601,2,0),(601,3,0),(601,3,1),(601,3,2),(601,2,2);`

// Audit L2 DBA-5 (#172): shrinking by rings kept a cell whose only link to the totem ran through a removed cell. The game needs the claim
// connected edge to edge, so the request is refused and nothing is removed.
func TestShrinkLandClaimRefusesToStrandACell(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, corridorFixture)}
	_, err := o.ShrinkLandClaim(Args{"totem_id": 601.0, "rings": 2.0})
	if err == nil || !strings.Contains(err.Error(), "cut 1 cells") || !strings.Contains(err.Error(), "2, 2") {
		t.Fatalf("the cell at 2, 2 would be cut off and the request must say so: %v", err)
	}
	if cellCount(t, o) != 6 || o.S.Dirty() {
		t.Fatal("a refused request must remove nothing")
	}
	// every cell is inside 3 rings: nothing to remove
	if _, err := o.ShrinkLandClaim(Args{"totem_id": 601.0, "rings": 3.0}); err == nil {
		t.Fatal("every cell is inside 3 rings: there is nothing to remove")
	}
	// keeping only the first ring leaves (1,0) and the totem, which is connected
	if r, err := o.ShrinkLandClaim(Args{"totem_id": 601.0, "rings": 1.0}); err != nil || r.(map[string]any)["removed"].(int) != 5 || r.(map[string]any)["remaining"].(int) != 2 {
		t.Fatalf("%v %v", r, err)
	}
	assertDBSound(t, o)
}

// A second route to the totem keeps a cell connected, and a cell that was already cut off before the request does not block it.
func TestShrinkLandClaimAllowsWhatStaysConnected(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, corridorFixture+`insert into landclaim_segments(totem_id,grid_location_x,grid_location_y) values (601,2,1);`)}
	// (2,2) now also touches (2,1), which touches (2,0): removing the cells at x=3 cuts nothing
	if r, err := o.ShrinkLandClaim(Args{"totem_id": 601.0, "rings": 2.0}); err != nil || r.(map[string]any)["removed"].(int) != 3 {
		t.Fatalf("%v %v", r, err)
	}
	o2 := &Ops{S: testsave.PlayerWithSQL(t, `
insert into actors(id,class,map) values (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin');
insert into totems(id,landclaim_original_global_location_x,landclaim_original_global_location_y,landclaim_original_global_location_z,landclaim_original_global_yaw_rotation) values (601,0.0,0.0,0.0,0.0);
insert into landclaim_segments(totem_id,grid_location_x,grid_location_y) values (601,1,0),(601,2,2),(601,4,0);`)}
	// (2,2) is not connected to anything before the request; that is not the request's doing
	if r, err := o2.ShrinkLandClaim(Args{"totem_id": 601.0, "rings": 2.0}); err != nil || r.(map[string]any)["removed"].(int) != 1 || r.(map[string]any)["remaining"].(int) != 3 {
		t.Fatalf("%v %v", r, err)
	}
}

// ExpandLandClaim and ApplyLandClaim write through one path: the same cells and level through either give the same claim.
func TestExpandAndApplyLandClaimAgree(t *testing.T) {
	viaExpand := &Ops{S: testsave.PlayerWithSQL(t, claimFixture)}
	if _, err := viaExpand.ExpandLandClaim(Args{"totem_id": 601.0, "rings": 2.0, "level": 3.0}); err != nil {
		t.Fatal(err)
	}
	var square []any
	for y := -2; y <= 2; y++ {
		for x := -2; x <= 2; x++ {
			if (x == 0 && y == 0) || (x == -1 && (y == 0 || y == -1)) { // the totem's own cell and the two stored ones
				continue
			}
			square = append(square, map[string]any{"x": float64(x), "y": float64(y)})
		}
	}
	viaApply := &Ops{S: testsave.PlayerWithSQL(t, claimFixture)}
	if _, err := viaApply.ApplyLandClaim(Args{"totem_id": 601.0, "cells": square, "level": 3.0}); err != nil {
		t.Fatal(err)
	}
	dump := func(o *Ops) string {
		rows, err := o.S.Query(`select grid_location_x x, grid_location_y y from landclaim_segments order by 1, 2`)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprint(rows, mustOne(t, o, `select landclaim_vertical_level l from totems where id=601`)["l"])
	}
	if a, b := dump(viaExpand), dump(viaApply); a != b {
		t.Fatalf("the two edits disagree:\n expand: %s\n apply:  %s", a, b)
	}
}

// Audit L2 ARCH-8 (#172): a new cell must be connected to the totem's own cell, not just to some stored cell. A stored cell that is cut off
// from the totem does not make the cells beside it acceptable.
func TestApplyLandClaimRefusesACellThatOnlyTouchesACutOffCell(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, claimFixture+`insert into landclaim_segments(totem_id,grid_location_x,grid_location_y) values (601,5,5);`)}
	if _, err := o.ApplyLandClaim(Args{"totem_id": 601.0, "cells": cells([2]float64{5, 6})}); err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("5, 6 touches only the cut-off cell 5, 5: %v", err)
	}
	if cellCount(t, o) != 3 {
		t.Fatal("a refused request must not write")
	}
	// a chain that does reach the totem through the new cells is fine
	if _, err := o.ApplyLandClaim(Args{"totem_id": 601.0, "cells": cells([2]float64{1, 0}, [2]float64{2, 0})}); err != nil {
		t.Fatal(err)
	}
}

// Overlapping requests for the same growth (a double click) add the cells once: the others find it done and are refused, none half applied.
func TestOverlappingLandClaimExpandsAddTheCellsOnce(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, claimFixture)}
	var okCount, failCount int
	var mu sync.Mutex
	var start, done sync.WaitGroup
	gate := make(chan struct{})
	for i := 0; i < 8; i++ {
		start.Add(1)
		done.Add(1)
		go func() {
			defer done.Done()
			start.Done()
			<-gate
			_, err := o.ExpandLandClaim(Args{"totem_id": 601.0, "rings": 2.0})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failCount++
			} else {
				okCount++
			}
		}()
	}
	start.Wait()
	close(gate)
	done.Wait()
	if okCount != 1 || failCount != 7 || cellCount(t, o) != 24 {
		t.Fatalf("one expand succeeds, the rest find it done: %d ok, %d refused, %d stored", okCount, failCount, cellCount(t, o))
	}
	assertDBSound(t, o)
}
