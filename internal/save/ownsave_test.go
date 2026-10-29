package save

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// gameTree makes a stand-in for %LOCALAPPDATA%\DuneSandbox\Saved\Cloud\PlayerClientStorage\FLS_retail\<id> in a temp folder, points LOCALAPPDATA at it, and returns the
// folder of the id and the LOCALAPPDATA folder.
func gameTree(t *testing.T) (idDir, local string) {
	t.Helper()
	local = t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	idDir = filepath.Join(local, "DuneSandbox", "Saved", "Cloud", "PlayerClientStorage", "FLS_retail", "76561190000000001")
	if err := os.MkdirAll(filepath.Join(idDir, "autosave"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"game.db", "game_prepatch.db", "game-1.db", filepath.Join("autosave", "autosave_1.db")} {
		if err := os.WriteFile(filepath.Join(idDir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return idDir, local
}

func put(t *testing.T, p string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// The game's own files are the ones under PlayerClientStorage, in every spelling; a file anywhere else is not (#498).
func TestWhichSavesAreTheGamesOwn(t *testing.T) {
	idDir, local := gameTree(t)
	sep := string(filepath.Separator)
	own := map[string]string{
		"game.db":     filepath.Join(idDir, "game.db"),
		"prepatch":    filepath.Join(idDir, "game_prepatch.db"),
		"game-1":      filepath.Join(idDir, "game-1.db"),
		"autosave":    filepath.Join(idDir, "autosave", "autosave_1.db"),
		"not created": filepath.Join(idDir, "game-2.db"),
		"other id":    filepath.Join(idDir, "..", "76561190000000002", "game.db"),
		"upper case":  filepath.Join(local, "DUNESANDBOX", "saved", "CLOUD", "playerclientstorage", "FLS_retail", "76561190000000001", "GAME.DB"),
		"dot dot":     local + sep + "DuneSandbox" + sep + "Elsewhere" + sep + ".." + sep + "Saved" + sep + "Cloud" + sep + "PlayerClientStorage" + sep + "FLS_retail" + sep + "1" + sep + "game.db",
		"forward":     strings.ReplaceAll(filepath.Join(idDir, "game.db"), "\\", "/"),
		"short name":  filepath.Join(local, "DUNESA~1", "Saved", "Cloud", "PlayerClientStorage", "FLS_retail", "1", "game.db"),
		"same layout": filepath.Join(t.TempDir(), "Saved", "Cloud", "PlayerClientStorage", "FLS_retail", "1", "game.db"),
		"empty":       "",
	}
	for name, p := range own {
		if !gamesOwnSave(p) {
			t.Errorf("%s: %q must count as the game's own", name, p)
		}
	}

	away := t.TempDir()
	notOwn := map[string]string{
		"a copy":                      put(t, filepath.Join(away, "reports", "import-test", "game.db")),
		"a copy that is not made yet": filepath.Join(away, "reports", "later", "game.db"),
		"game.db renamed outside":     put(t, filepath.Join(away, "game.db")),
		"sibling storage folder":      put(t, filepath.Join(local, "DuneSandbox", "Saved", "Cloud", "PlayerClientStorage2", "FLS_retail", "1", "game.db")),
		"sibling of the tree":         put(t, filepath.Join(local, "DuneSandbox", "Saved", "Cloud", "PlayerClientStorage-old", "FLS_retail", "1", "game.db")),
		"storage name, no layout":     put(t, filepath.Join(away, "PlayerClientStorage", "FLS_retail", "1", "game.db")),
		"next to the tree":            put(t, filepath.Join(local, "DuneSandbox", "Saved", "Cloud", "game.db")),
		"the config folder":           put(t, filepath.Join(local, "DuneSandbox", "Saved", "Config", "WindowsClient", "game.db")),
		"up and out":                  filepath.Join(idDir, "..", "..", "..", "..", "..", "copy", "game.db"),
	}
	for name, p := range notOwn {
		if gamesOwnSave(p) {
			t.Errorf("%s: %q must NOT count as the game's own", name, p)
		}
	}
}

// A path spelled with \\?\ is the same path; the prefix is compared away, whatever system the test runs on.
func TestExtendedPathPrefixIsTheSamePath(t *testing.T) {
	plain, odd := nameParts(`C:\Users\x\AppData\Local\DuneSandbox\Saved\Cloud\PlayerClientStorage\FLS_retail\1\game.db`)
	pre, odd2 := nameParts(`\\?\C:\Users\x\AppData\Local\DuneSandbox\Saved\Cloud\PlayerClientStorage\FLS_retail\1\game.db`)
	if odd || odd2 || strings.Join(plain, "/") != strings.Join(pre, "/") || !hasGameLayout(pre) {
		t.Fatalf("%v %v", plain, pre)
	}
	if _, odd := nameParts(`\\.\C:\x\game.db`); !odd {
		t.Fatal("a device path cannot be judged, so it is the game's own")
	}
	if _, odd := nameParts(`\\?\Volume{1234}\x\game.db`); !odd {
		t.Fatal("a volume path cannot be judged, so it is the game's own")
	}
	if p, _ := nameParts(`C:\a\b\..\..\c\.\d`); strings.Join(p, "/") != "c:/c/d" {
		t.Fatalf("%v", p)
	}
	if runtime.GOOS == "windows" {
		idDir, _ := gameTree(t)
		if !gamesOwnSave(`\\?\` + filepath.Join(idDir, "game.db")) {
			t.Fatal(`\\?\ spelling of the game's file`)
		}
	}
}

// A link into the tree is the tree, a hard link to the game's file is the game's file; a file in the tree is the game's wherever its link leads (fail safe), and a link between
// two places outside is outside.
func TestLinksAreFollowed(t *testing.T) {
	idDir, _ := gameTree(t)
	away := t.TempDir()
	if err := os.Symlink(idDir, filepath.Join(away, "link")); err != nil {
		t.Skip("symbolic links are not available here: ", err)
	}
	if !gamesOwnSave(filepath.Join(away, "link", "game.db")) {
		t.Error("a link to the game's folder is the game's folder")
	}
	if !gamesOwnSave(filepath.Join(away, "link", "never-made.db")) {
		t.Error("a name that does not exist yet in the game's folder is the game's")
	}
	out := put(t, filepath.Join(away, "real", "game.db"))
	if err := os.Symlink(filepath.Dir(out), filepath.Join(away, "link2")); err == nil {
		if gamesOwnSave(filepath.Join(away, "link2", "game.db")) {
			t.Error("a link between two places outside the tree is outside")
		}
	}
	if err := os.Symlink(out, filepath.Join(idDir, "pointer.db")); err == nil {
		if !gamesOwnSave(filepath.Join(idDir, "pointer.db")) {
			t.Error("a file in the tree is the game's, wherever its link leads")
		}
	}
	if err := os.Link(filepath.Join(idDir, "game.db"), filepath.Join(away, "hard.db")); err == nil {
		if !gamesOwnSave(filepath.Join(away, "hard.db")) {
			t.Error("a hard link to the game's file is the game's file")
		}
	}
}

// With no LOCALAPPDATA the tree cannot be told, so every save is the game's own.
func TestWithoutTheTreeEverySaveIsTheGamesOwn(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")
	if !gamesOwnSave(put(t, filepath.Join(t.TempDir(), "game.db"))) {
		t.Fatal("unsure means the game's own")
	}
}

// A save outside the game's folder is written at once while the game runs a single-player session; the game's own save in the same situation waits (#498).
func TestACopyIsWrittenAtOnceAndTheGamesOwnSaveWaits(t *testing.T) {
	defer NoGameRunningForTests()()
	defer RealOwnSaveCheckForTests()()
	defer ActiveSessionForTests()()
	idDir, _ := gameTree(t)

	copyPath := makeSave(t, `create table t(a integer); insert into t values (1);`)
	if IsGamesOwnSave(copyPath) {
		t.Fatalf("%s is not in the game's folder", copyPath)
	}
	if g := GameStateFor(copyPath, true); g.Blocked || g.Mode != "not_live" || len(g.Processes) != 0 {
		t.Fatalf("a copy is not held by the game: %+v", g)
	}
	s, err := Open(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	setA(t, s, 2)
	if f := s.Flush(); !f.Saved || f.Queued != 0 || f.Backup == "" || f.Why != "" {
		t.Fatalf("the copy takes the edit at once: %+v", f)
	}
	if s.Dirty() || fileA(t, copyPath) != 2 {
		t.Fatal("the copy's file has the edit")
	}
	if len(s.Backups()) == 0 {
		t.Fatal("the backup before the write is still made")
	}

	own := filepath.Join(idDir, "game.db")
	blob, _ := os.ReadFile(makeSave(t, `create table t(a integer); insert into t values (1);`))
	if err := os.WriteFile(own, blob, 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsGamesOwnSave(own) {
		t.Fatal("the tree's game.db is the game's own")
	}
	g, err := Open(own)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	setA(t, g, 2)
	f := g.Flush()
	if f.Saved || f.Queued != 1 || !strings.Contains(f.Why, "single-player session") {
		t.Fatalf("the game's own save waits for the game: %+v", f)
	}
	if !g.Dirty() || fileA(t, own) != 1 {
		t.Fatal("the game's file is as it was")
	}
}

// The checks that are not about the game stay for a copy: a network share or removable drive still needs the confirmation, and the question about the folder does not.
func TestACopyKeepsTheLocationConfirmationButNotTheFolderQuestion(t *testing.T) {
	defer NoGameRunningForTests()()
	defer RealOwnSaveCheckForTests()()
	p := makeSave(t, `create table t(a integer);`)

	locationFor = func(string) string { return notInGameFolderWhy }
	if g := GameStateFor(p, true); g.Blocked || g.NeedsConfirm {
		t.Fatalf("being outside the game's folder is what a copy is: %+v", g)
	}
	locationFor = func(string) string { return "the save is on a network share" }
	g := GameStateFor(p, true)
	if !g.Blocked || !g.NeedsConfirm || !strings.Contains(g.Reason, "network share") {
		t.Fatalf("a share still asks: %+v", g)
	}
	ConfirmLocation(p)
	if g := GameStateFor(p, true); g.Blocked {
		t.Fatalf("confirmed: %+v", g)
	}
}

// Check again says so for a copy.
func TestCheckGameNowForACopy(t *testing.T) {
	defer NoGameRunningForTests()()
	defer RealOwnSaveCheckForTests()()
	defer ActiveSessionForTests()()
	c := CheckGameNow(makeSave(t, `create table t(a integer);`))
	if c.Blocked || c.Running || !strings.Contains(c.Message, "not the game's save") || !strings.Contains(c.Message, "written at once") {
		t.Fatalf("%+v", c)
	}
}
