package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The move-reserve verb (#1191) works on a scratch save in a temp folder only: no real save, game process or config is touched.
const moveReserveCLIWorld = `
insert into inventories(id,actor_id,inventory_type,max_item_count,max_item_volume) values (12,2,33,-1,0.0),(7001,701,4,3,1000.0),(7002,702,4,3,1000.0);
insert into actors(id,class,map,location_x,location_y,location_z,rotation_x,rotation_y,rotation_z,rotation_w,state) values
  (601,'/Game/BP_Totem.BP_Totem_C','HaggaBasin',5000,6000,7,0,0,0,1,0),
  (701,'/Game/BP_Box.BP_Box_C','HaggaBasin',5001,6000,7,0,0,0,1,0),(702,'/Game/BP_Box.BP_Box_C','HaggaBasin',5002,6000,7,0,0,0,1,0);
insert into fgl_entities(entity_id,components) values (5764607523034234881, jsonb('{}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (601,5764607523034234881,'Actor');
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child,edited_by_player_id) values (601,'Home',3,3,0,0),(701,'DD-1',1,3,1,0),(702,'DD-2',1,3,1,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (601,1,1);
insert into placeables(id,owner_entity_id,building_type,has_buildable_support) values (601,5764607523034234881,'Totem_Placeable',1),
  (701,5764607523034234881,'Developer_StorageContainer_Placeable',1),(702,5764607523034234881,'Developer_StorageContainer_Placeable',1);
insert into items(id,inventory_id,stack_size,position_index,template_id,is_new,acquisition_time,stats,quality_level) values
  (9001,12,30,0,'Stone',0,1790000001,'{}',0),(9002,12,20,1,'Stone',0,1790000002,'{}',0),(9003,12,10,2,'ScrapMetal',0,1790000003,'{}',0),(9004,7001,5,0,'Stone',0,1790000004,'{}',0);
insert into totems(id) values (601);`

func moveEnv(running bool) moveReserveEnv {
	return moveReserveEnv{
		GameRunning: func() (bool, string) {
			if running {
				return true, "Dune.exe is running"
			}
			return false, ""
		},
		Discover: func() []string { return nil },
		Flush:    func(s *save.Save) save.Flush { return s.Flush() },
	}
}

func moveSavePath(t *testing.T) string {
	t.Helper()
	return testsave.PlayerWithSQL(t, moveReserveCLIWorld).Path
}

func reserveCount(t *testing.T, path string) (reserve, boxA, boxB int64) {
	t.Helper()
	s, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.One(`select (select count(*) from items where inventory_id=12) r, (select count(*) from items where inventory_id=7001) a, (select count(*) from items where inventory_id=7002) b`)
	if err != nil {
		t.Fatal(err)
	}
	return r["r"].(int64), r["a"].(int64), r["b"].(int64)
}

func TestMoveReserveDryRunWritesNothingAndSaysWhatItWouldDo(t *testing.T) {
	p := moveSavePath(t)
	before, _ := os.ReadFile(p)
	var out bytes.Buffer
	if err := runMoveReserve([]string{"--dry-run"}, p, &out, moveEnv(false)); err != nil { // no --boxes: the boxes DD-1, DD-2 ... are found
		t.Fatal(err)
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatal("a dry run wrote the save")
	}
	for _, w := range []string{"Dry run: nothing is written", "Would move: 3 item stacks (60 items)", `Box "DD-1"`, "would go in"} {
		if !strings.Contains(out.String(), w) {
			t.Errorf("the output lacks %q:\n%s", w, out.String())
		}
	}
	// a dry run is allowed while the game runs: it only reads
	if err := runMoveReserve([]string{"--boxes", "DD-1,DD-2", "--dry-run"}, p, &out, moveEnv(true)); err != nil {
		t.Fatalf("a dry run reads only: %v", err)
	}
}

func TestMoveReserveWritesAndReadsBack(t *testing.T) {
	p := moveSavePath(t)
	var out bytes.Buffer
	if err := runMoveReserve([]string{"--boxes", " DD-1 , DD-2 "}, p, &out, moveEnv(false)); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if r, a, b := reserveCount(t, p); r != 0 || a != 3 || b != 1 {
		t.Fatalf("Reserve %d, Box A %d, Box B %d", r, a, b)
	}
	for _, w := range []string{"Backup copy of the save:", "Written, and read back from the written file: the Reserve is empty.", "Moved: 3 item stacks"} {
		if !strings.Contains(out.String(), w) {
			t.Errorf("the output lacks %q:\n%s", w, out.String())
		}
	}
	// the backup copies are in the save's backup folder
	if ents, _ := os.ReadDir(filepath.Join(filepath.Dir(p), "tabr-tau-backups")); len(ents) == 0 {
		t.Fatal("no backup copy in the backup folder")
	}
	// the second run has nothing to do and says so
	out.Reset()
	if err := runMoveReserve([]string{"--boxes", "DD-1,DD-2"}, p, &out, moveEnv(false)); err != nil || !strings.Contains(out.String(), "The Reserve is empty. Nothing was moved") {
		t.Fatalf("%v\n%s", err, out.String())
	}
}

func TestMoveReserveRefusesWhileTheGameRunsAndOnBadBoxes(t *testing.T) {
	p := moveSavePath(t)
	before, _ := os.ReadFile(p)
	var out bytes.Buffer
	if err := runMoveReserve([]string{"--boxes", "DD-1,DD-2"}, p, &out, moveEnv(true)); err == nil || !strings.Contains(err.Error(), "Dune.exe is running") || !strings.Contains(err.Error(), "not changed") {
		t.Fatalf("%v", err)
	}
	if err := runMoveReserve([]string{"--boxes", "DD-1,dd-2"}, p, &out, moveEnv(false)); err == nil || !strings.Contains(err.Error(), `named "dd-2"`) {
		t.Fatalf("%v", err)
	}
	if err := runMoveReserve([]string{"--boxes", "DD-1,DD-1"}, p, &out, moveEnv(false)); err == nil || !strings.Contains(err.Error(), "named twice") {
		t.Fatalf("%v", err)
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatal("a refusal wrote the save")
	}
	if r, _, _ := reserveCount(t, p); r != 3 {
		t.Fatalf("the Reserve changed: %d", r)
	}
}

// The edit was worked out but the save cannot be written: the edit is thrown away, the run says "not changed", and the save is as it was.
func TestMoveReserveWhenTheWriteFailsNothingIsLeftQueued(t *testing.T) {
	p := moveSavePath(t)
	before, _ := os.ReadFile(p)
	env := moveEnv(false)
	var seen *save.Save
	env.Flush = func(s *save.Save) save.Flush {
		seen = s
		if !s.Dirty() {
			t.Error("the edit was not made before the write")
		}
		return save.Flush{Saved: false, Why: "the file is in use"}
	}
	var out bytes.Buffer
	err := runMoveReserve([]string{"--boxes", "DD-1,DD-2"}, p, &out, env)
	if err == nil || !strings.Contains(err.Error(), "not changed") || !strings.Contains(err.Error(), "could not be written (the file is in use)") {
		t.Fatalf("%v", err)
	}
	if seen == nil || seen.Dirty() {
		t.Fatal("the edit must be discarded, not left queued")
	}
	after, _ := os.ReadFile(p)
	if !bytes.Equal(before, after) {
		t.Fatal("the save was written")
	}
	if r, _, _ := reserveCount(t, p); r != 3 {
		t.Fatalf("the Reserve changed: %d", r)
	}
}

func TestMoveReserveChecksItsArguments(t *testing.T) {
	var out bytes.Buffer
	for _, args := range [][]string{{"--boxes", ""}, {"--boxes", "A,B,C,D,E,F,G"}, {"--boxes", "A,B", "extra"}, {"--bogus"}} {
		if err := runMoveReserve(args, "x.db", &out, moveEnv(false)); err == nil || !strings.Contains(err.Error(), "usage: move-reserve") {
			t.Errorf("%v: %v", args, err)
		}
	}
	// no save to name: the run does not guess
	if err := runMoveReserve([]string{"--boxes", "A,B"}, "", &out, moveEnv(false)); err == nil || !strings.Contains(err.Error(), "no save (game.db) was found") {
		t.Fatalf("%v", err)
	}
}

// the verb is wired into the command list: a bad argument reaches its own usage text.
func TestMoveReserveIsInTheCommandList(t *testing.T) {
	if err := runCommand([]string{"move-reserve", "extra"}, t.TempDir(), ""); err == nil || !strings.Contains(err.Error(), "usage: move-reserve") {
		t.Fatalf("%v", err)
	}
}
