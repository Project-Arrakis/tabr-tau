package layoutroll

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/basefile/consolefixture"
	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The Deep Desert guard, on made-up saves built from the real schema (no real save, game or Game.ini is touched). The player is character 1 (controller 1, body 2, player state 3,
// all on HaggaBasin); a world adds the base 601 (the player's, standing) and the base 801 (the player's, on HaggaBasin).

const (
	baseInDD    = ddWorld + ddStanding + "\nupdate actors set map='DeepDesert' where id in (601,602,701,702);"
	baseOnHagga = ddWorld + ddStanding
)

// vehicleSQL is one vehicle actor with its permission record. owner 0 gives it no permission record (the world's own vehicles have none).
func vehicleSQL(id int, mp string, state, owner int) string {
	s := "insert into actors(id,class,map,location_x,location_y,location_z,state) values (" + strconv.Itoa(id) + ",'/Game/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','" + mp + "',4,4,0," + strconv.Itoa(state) + ");\n" +
		"insert into vehicles(id) values (" + strconv.Itoa(id) + ");\n"
	if owner != 0 {
		s += "insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (" + strconv.Itoa(id) + ",'d',2,3,0);\n" +
			"insert into permission_actor_rank(permission_actor_id,player_id,rank) values (" + strconv.Itoa(id) + "," + strconv.Itoa(owner) + ",1);\n"
	}
	return s
}

func sha(t *testing.T, p string) [32]byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(b)
}

func checkpoints(t *testing.T, savePath string) []string {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(filepath.Dir(savePath), "tabr-tau-backups", "*"+checkpointTag+".db"))
	return m
}

const checkpointTag = "-layoutroll"

func readState(t *testing.T, p string) state {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var s state
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestTheDeepDesertGuardTable(t *testing.T) {
	cases := []struct {
		name    string
		sql     string
		refuse  bool
		bases   int // base files expected
		mention string
	}{
		{"nothing of the player is in the Deep Desert", baseOnHagga, false, 0, ""},
		{"a standing base of the player in the Deep Desert", baseInDD, true, 1, "base 601"},
		{"a base in the Deep Desert owned by someone else", baseInDD + "\ndelete from permission_actor_rank where permission_actor_id=601; insert into permission_actor_rank(permission_actor_id,player_id,rank) values (601,5,1),(601,1,2);", false, 0, ""},
		{"a base in the Deep Desert the player is only a member of", baseInDD + "\nupdate permission_actor_rank set rank=2 where permission_actor_id=601 and player_id=1;", false, 0, ""},
		{"a base the game's tool picked up", baseInDD + "\nupdate actors set state=5 where id=601;", false, 0, ""},
		{"the controller in the Deep Desert", "update actors set map='DeepDesert' where id=1;", true, 0, "the character"},
		{"only the body in the Deep Desert", "update actors set map='DeepDesert' where id=2;", true, 0, "the character"},
		{"only the player state in the Deep Desert", "update actors set map='DeepDesert' where id=3;", true, 0, "the character"},
		{"the travel map of the Deep Desert", "update actors set map='DeepDesert_1_Travel' where id=1;", true, 0, "the character"},
		{"the character's map is not recorded", "update actors set map='' where id=1;", true, 0, "does not say which map"},
		// a placed vehicle of the player in the Deep Desert is no longer a refusal: the run takes it out (deepdesert_vehicles_test.go)
		{"a stored vehicle of the player in the Deep Desert is not in the world", vehicleSQL(311, "DeepDesert", 4, 1), false, 0, ""},
		{"a vehicle in the backup (state 2) in the Deep Desert is not in the world", vehicleSQL(317, "DeepDesert", 2, 1) + "insert into backup_vehicles(character_id,vehicle_id,customization_id) values (1,317,'x');", false, 0, ""},
		{"a vehicle of the player on another map", vehicleSQL(312, "HaggaBasin", 0, 1), false, 0, ""},
		{"a vehicle of someone else in the Deep Desert", vehicleSQL(313, "DeepDesert", 0, 5), false, 0, ""},
		{"a world vehicle (no owner) in the Deep Desert", vehicleSQL(314, "DeepDesert", 0, 0), false, 0, ""},
		{"a vehicle of the player whose map is not recorded", vehicleSQL(315, "", 0, 1), true, 0, "no map recorded"},
		{"a base and a vehicle of the player in the Deep Desert: the vehicle is left alone", baseInDD + "\n" + vehicleSQL(318, "DeepDesert", 0, 1), true, 1, "vehicle 318"},
		{"a base, the character and a vehicle together", baseInDD + "\nupdate actors set map='DeepDesert' where id=2;\n" + vehicleSQL(316, "DeepDesert", 0, 1), true, 1, "vehicle 316"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, iniWith("11"))
			e.save = testsave.PlayerWithSQL(t, c.sql).Path
			saveBefore := sha(t, e.save)
			r, err := Roll(e.opts())
			if sha(t, e.save) != saveBefore {
				t.Fatal("the save file was written")
			}
			if len(checkpoints(t, e.save)) != 1 {
				t.Fatalf("a checkpoint of the save is taken before the look: %v", checkpoints(t, e.save))
			}
			stt := readState(t, e.state)
			if c.refuse {
				if err == nil || !strings.Contains(err.Error(), "not changed") || r.Wrote {
					t.Fatalf("it did not refuse: %v %+v", err, r)
				}
				if e.ini() != iniWith("11") || len(e.backups()) != 0 {
					t.Fatal("Game.ini was touched")
				}
				if !strings.Contains(strings.Join(r.Lines, "\n"), c.mention) {
					t.Fatalf("the reason does not say %q:\n%s", c.mention, strings.Join(r.Lines, "\n"))
				}
				if stt.LastCheck == nil || len(stt.LastCheck.Blockers) == 0 || stt.LastRun == nil || !strings.HasPrefix(stt.LastRun.Outcome, "not changed") || stt.LastRoll != nil {
					t.Fatalf("the reason is not recorded: %+v", stt)
				}
				if len(stt.LastCheck.ExportFiles) != c.bases {
					t.Fatalf("exported %v, want %d file(s)", stt.LastCheck.ExportFiles, c.bases)
				}
				for _, f := range stt.LastCheck.ExportFiles {
					if _, err := os.Stat(filepath.Join(stt.LastCheck.ExportFolder, f)); err != nil {
						t.Fatalf("the exported file is not there: %v", err)
					}
				}
				if c.bases == 0 && stt.LastCheck.ExportFolder != "" {
					t.Fatal("a folder was named although no base stands there")
				}
				return
			}
			if err != nil || !r.Wrote {
				t.Fatalf("it should roll: %v\n%s", err, strings.Join(r.Lines, "\n"))
			}
			if stt.LastCheck == nil || len(stt.LastCheck.Blockers) != 0 || stt.LastCheck.Checkpoint == "" || stt.LastRoll == nil {
				t.Fatalf("state: %+v", stt)
			}
			if got, _ := filepath.Glob(filepath.Join(filepath.Dir(e.state), exportFolderPrefix+"*")); len(got) != 0 {
				t.Fatalf("a folder of base files was made: %v", got)
			}
		})
	}
}

// The base file of a refused run is the console's own file of that base, read back by the editor's reader, in a dated folder next to the state file, and the run names both.
func TestAStandingBaseIsExportedBeforeTheRefusal(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, baseInDD).Path
	r, err := Roll(e.opts())
	if err == nil {
		t.Fatal("it rolled")
	}
	stt := readState(t, e.state)
	c := stt.LastCheck
	if c == nil || len(c.ExportFiles) != 1 || !strings.HasPrefix(filepath.Base(c.ExportFolder), exportFolderPrefix) || filepath.Dir(c.ExportFolder) != filepath.Dir(e.state) {
		t.Fatalf("%+v", c)
	}
	if !strings.HasPrefix(c.ExportFiles[0], "tabr-tau-console-base-601-") {
		t.Fatalf("file name: %s", c.ExportFiles[0])
	}
	b, err2 := os.ReadFile(filepath.Join(c.ExportFolder, c.ExportFiles[0]))
	if err2 != nil || !strings.Contains(string(b), `"dune-base-backup"`) {
		t.Fatalf("the file: %v", err2)
	}
	out := strings.Join(r.Lines, "\n")
	if !strings.Contains(out, c.ExportFiles[0]) || !strings.Contains(out, "Checkpoint of the save:") || !strings.Contains(out, "What to do:") {
		t.Fatalf("the run does not say what it did:\n%s", out)
	}
	if c.Checkpoint == "" || len(c.CheckpointSHA256) != 64 {
		t.Fatalf("the checkpoint is not recorded: %+v", c)
	}
}

// --set and --force do not skip the guard.
func TestSetAndForceDoNotSkipTheGuard(t *testing.T) {
	for _, mod := range []func(o *Options){func(o *Options) { o.Set = ptr(4) }, func(o *Options) { o.Force = true }} {
		e := newEnv(t, iniWith("11"))
		e.save = testsave.PlayerWithSQL(t, "update actors set map='DeepDesert' where id=1;").Path
		o := e.opts()
		mod(&o)
		if _, err := Roll(o); err == nil || e.ini() != iniWith("11") {
			t.Fatalf("it rolled: %v", err)
		}
	}
}

// A dry run reads, says what it would find and what it would do, and writes nothing: no checkpoint, no base file, no state file, nothing in Game.ini.
func TestADryRunShowsTheFindingsAndWritesNothing(t *testing.T) {
	for _, c := range []struct {
		sql, want string
	}{
		{baseInDD, "would write a console base backup file of each of the 1 base(s)"},
		{"update actors set map='DeepDesert' where id=1;", "it would refuse for the reasons above"},
		{baseOnHagga, "would first take a checkpoint"},
	} {
		e := newEnv(t, iniWith("11"))
		e.save = testsave.PlayerWithSQL(t, c.sql).Path
		saveBefore := sha(t, e.save)
		o := e.opts()
		o.DryRun = true
		r, err := Roll(o)
		if err != nil {
			t.Fatalf("a dry run is not a failure: %v", err)
		}
		if out := strings.Join(r.Lines, "\n"); !strings.Contains(out, c.want) || !strings.Contains(out, "would first take a checkpoint") {
			t.Fatalf("missing %q:\n%s", c.want, out)
		}
		if sha(t, e.save) != saveBefore || len(checkpoints(t, e.save)) != 0 {
			t.Fatal("the dry run wrote to the save or its backup folder")
		}
		if _, err := os.Stat(e.state); err == nil {
			t.Fatal("the dry run wrote the state file")
		}
		if got, _ := filepath.Glob(filepath.Join(filepath.Dir(e.state), exportFolderPrefix+"*")); len(got) != 0 {
			t.Fatal("the dry run exported a base")
		}
		if e.ini() != iniWith("11") || r.Wrote {
			t.Fatal("Game.ini changed")
		}
	}
}

// With the game running nothing is looked at: no checkpoint, no read.
func TestARunningGameStopsBeforeTheSaveIsTouched(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.running = true
	opened := false
	defer func(f func(string) (*save.Save, error)) { openSave = f }(openSave)
	openSave = func(p string) (*save.Save, error) { opened = true; return save.Open(p) }
	if _, err := Roll(e.opts()); err == nil || opened || len(checkpoints(t, e.save)) != 0 {
		t.Fatalf("%v opened=%v", err, opened)
	}
}

func TestACheckpointThatFailsStopsTheRunBeforeTheRead(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	defer func(f func(*save.Save) (save.ManualBackup, error), g func(*save.Save) (ops.DeepDesertHolds, error)) {
		takeCheckpoint, inspectSave = f, g
	}(takeCheckpoint, inspectSave)
	takeCheckpoint = func(*save.Save) (save.ManualBackup, error) { return save.ManualBackup{}, errors.New("disk full") }
	inspectSave = func(*save.Save) (ops.DeepDesertHolds, error) {
		t.Fatal("the save was read after a failed checkpoint")
		return ops.DeepDesertHolds{}, nil
	}
	_, err := Roll(e.opts())
	if err == nil || !strings.Contains(err.Error(), "checkpoint") || !strings.Contains(err.Error(), "disk full") || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	if stt := readState(t, e.state); stt.LastCheck == nil || !strings.Contains(stt.LastCheck.Blockers[0], "disk full") {
		t.Fatalf("%+v", stt)
	}
}

// A base that cannot be exported stops the run and says so; the files already written stay and are recorded.
func TestAnExportThatFailsStopsTheRun(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, baseInDD).Path
	old := exportBaseFile
	defer func() { exportBaseFile = old }()
	exportBaseFile = func(*save.Save, int64, time.Time, string) (*ops.BaseExport, error) {
		return nil, errors.New("a storage holds a Solido")
	}
	_, err := Roll(e.opts())
	if err == nil || !strings.Contains(err.Error(), "stopped") || !strings.Contains(err.Error(), "Solido") || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	stt := readState(t, e.state)
	if stt.LastRun == nil || !strings.Contains(stt.LastRun.Outcome, "could not be exported") || stt.LastCheck == nil || len(stt.LastCheck.ExportFiles) != 0 {
		t.Fatalf("%+v", stt)
	}
}

// A file that does not read back (the editor's reader refuses it) is a failed export.
func TestAnExportedFileThatDoesNotReadBackStopsTheRun(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, baseInDD).Path
	old := exportBaseFile
	defer func() { exportBaseFile = old }()
	exportBaseFile = func(*save.Save, int64, time.Time, string) (*ops.BaseExport, error) {
		return &ops.BaseExport{Data: []byte(`{"not":"a base file"}`), FileName: "x.json"}, nil
	}
	_, err := Roll(e.opts())
	if err == nil || !strings.Contains(err.Error(), "reader") {
		t.Fatalf("%v", err)
	}
	// and a good file from the fixture passes the same check
	exportBaseFile = func(*save.Save, int64, time.Time, string) (*ops.BaseExport, error) {
		return &ops.BaseExport{Data: consolefixture.File(nil), FileName: "y.json"}, nil
	}
	e2 := newEnv(t, iniWith("11"))
	e2.save = e.save
	if _, err := Roll(e2.opts()); err == nil || strings.Contains(err.Error(), "reader") {
		t.Fatalf("the refusal for the base itself stands: %v", err)
	}
	if c := readState(t, e2.state).LastCheck; c == nil || len(c.ExportFiles) != 1 || c.ExportFiles[0] != "y.json" {
		t.Fatalf("%+v", c)
	}
}

func TestNoSaveToCheckIsARefusalNotAGuess(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	o := e.opts()
	o.SavePath, o.SaveProblem = "", "2 saves were found, so it is not clear which one to check"
	_, err := Roll(o)
	if err == nil || !strings.Contains(err.Error(), "2 saves were found") || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	// a file that is not there: the lock can be made beside it, and the read refuses
	o.SavePath, o.SaveProblem = filepath.Join(t.TempDir(), "game.db"), ""
	if _, err := Roll(o); err == nil || !strings.Contains(err.Error(), "could not be read") || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	// a folder that is not there: no lock file can be made, so the run is refused before it reads anything (#1209)
	o.SavePath = filepath.Join(t.TempDir(), "missing", "game.db")
	if _, err := Roll(o); err == nil || !strings.Contains(err.Error(), "no lock could be taken on the save") || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
}

func TestASaveWithNoPlayerCannotBeCleared(t *testing.T) {
	e := newEnv(t, iniWith("11"))
	e.save = testsave.PlayerWithSQL(t, "delete from player_state;").Path
	_, err := Roll(e.opts())
	if err == nil || e.ini() != iniWith("11") {
		t.Fatalf("%v", err)
	}
	if c := readState(t, e.state).LastCheck; c == nil || !strings.Contains(strings.Join(c.Blockers, ";"), "no player") {
		t.Fatalf("%+v", c)
	}
}
