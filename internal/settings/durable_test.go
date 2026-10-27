package settings

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/kits"
)

// #442: settings.json holds the care package kits, so replacing it must survive a crash (flushed temp file, rename, folder sync), keep
// one generation of the previous file, and never read an empty or cut-off file as "nothing to lose".

// withKit writes settings with one kit, a track number and a switch, in two steps so the backup copy exists.
func withKit(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "settings.json")
	if _, err := Update(p, func(s *Settings) error {
		var err error
		s.Kits, err = kits.Put(s.Kits, "", aKit("Starter"))
		s.TrackNumbers = map[string]int64{"Combat": 3}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(p, func(s *Settings) error { s.AutoRefillOnOpen = true; return nil }); err != nil {
		t.Fatal(err)
	}
	return p
}

func kitSurvived(t *testing.T, s Settings) {
	t.Helper()
	if len(s.Kits) != 1 || s.Kits[0].Name != "Starter" || s.Kits[0].Items[0].Quantity != 100 || s.Kits[0].XP != 500 || s.TrackNumbers["Combat"] != 3 {
		t.Fatalf("the kit or the track numbers did not survive: %+v", s)
	}
}

func hooksBack(t *testing.T) {
	t.Helper()
	sf, sd, br, nw := syncFile, syncDir, beforeRename, now
	t.Cleanup(func() { syncFile, syncDir, beforeRename, now = sf, sd, br, nw })
}

func glob(t *testing.T, p, pattern string) []string {
	t.Helper()
	m, _ := filepath.Glob(filepath.Join(filepath.Dir(p), pattern))
	return m
}

func TestTheNewFileIsFlushedBeforeTheRenameAndTheFolderAfterIt(t *testing.T) {
	hooksBack(t)
	var steps []string
	syncFile = func(f *os.File) error { steps = append(steps, "sync"); return f.Sync() }
	beforeRename = func(dst string) { steps = append(steps, "rename:"+filepath.Base(dst)) }
	syncDir = func(string) { steps = append(steps, "dir") }
	p := filepath.Join(t.TempDir(), "settings.json")
	if err := Save(p, Settings{AutoRefillOnOpen: true}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(steps, ","); got != "sync,rename:settings.json,dir" {
		t.Fatalf("want flush, rename, folder sync; got %s", got)
	}
	steps = nil
	if err := Save(p, Settings{}); err != nil { // the second write also keeps the first as the backup, with the same steps
		t.Fatal(err)
	}
	if got := strings.Join(steps, ","); got != "sync,rename:settings.json.bak,dir,sync,rename:settings.json,dir" {
		t.Fatalf("want the backup written the same way, then the file; got %s", got)
	}
}

func TestACrashBetweenTheTempFileAndTheRenameLeavesTheOldFileWhole(t *testing.T) {
	hooksBack(t)
	p := withKit(t)
	before, _ := os.ReadFile(p)
	crash := errors.New("power cut")
	beforeRename = func(dst string) {
		if dst == p { // the temp file of the new settings is written and flushed; the process dies before the rename
			panic(crash)
		}
	}
	func() {
		defer func() {
			if r := recover(); r != crash {
				t.Fatalf("want the simulated crash, got %v", r)
			}
		}()
		Update(p, func(s *Settings) error { s.AutoRefillOnOpen = false; s.Kits = nil; return nil })
	}()
	beforeRename = func(string) {}
	after, _ := os.ReadFile(p)
	if string(after) != string(before) {
		t.Fatalf("the file changed although the rename never happened:\n%s", after)
	}
	got, note, err := LoadNote(p)
	if err != nil || note != "" || !got.AutoRefillOnOpen {
		t.Fatalf("the old file reads as it was: %v %q %+v", err, note, got)
	}
	kitSurvived(t, got)
	tmps := glob(t, p, "settings-*.tmp")
	if len(tmps) != 1 {
		t.Fatalf("the crash leaves its temp file behind: %v", tmps)
	}
	// the next run: a temp file of a minute ago is swept by the next write, and the write works
	old := time.Now().Add(-time.Hour)
	os.Chtimes(tmps[0], old, old)
	if _, err := Update(p, func(s *Settings) error { s.AutoRefillOnOpen = false; return nil }); err != nil {
		t.Fatal(err)
	}
	if left := glob(t, p, "settings-*.tmp"); len(left) != 0 {
		t.Fatalf("the stale temp file was not swept: %v", left)
	}
	got, _ = LoadStrict(p)
	if got.AutoRefillOnOpen {
		t.Fatal("the change after the crash did not land")
	}
	kitSurvived(t, got)
}

func TestARecentTempFileIsLeftAloneInCaseAnotherEditorIsWriting(t *testing.T) {
	p := withKit(t)
	tmp := filepath.Join(filepath.Dir(p), "settings-123.tmp")
	os.WriteFile(tmp, []byte("{"), 0o600)
	if _, err := Update(p, func(s *Settings) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tmp); err != nil {
		t.Fatalf("a temp file from a moment ago was removed: %v", err)
	}
}

func TestAFailedFlushWritesNothingAndLeavesNoTempFile(t *testing.T) {
	hooksBack(t)
	p := withKit(t)
	before, _ := os.ReadFile(p)
	bak, _ := os.ReadFile(BackupPath(p))
	syncFile = func(f *os.File) error { return errors.New("disk full") }
	if _, err := Update(p, func(s *Settings) error { s.AutoRefillOnOpen = false; return nil }); err == nil {
		t.Fatal("a file that could not be flushed must not be reported as saved")
	}
	syncFile = func(f *os.File) error { return f.Sync() }
	if after, _ := os.ReadFile(p); string(after) != string(before) {
		t.Fatal("the file changed")
	}
	if after, _ := os.ReadFile(BackupPath(p)); string(after) != string(bak) {
		t.Fatal("the backup changed")
	}
	if left := glob(t, p, "settings-*.tmp"); len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

func TestOneGenerationOfTheOldFileIsKept(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	steps := []Settings{{TrackNumbers: map[string]int64{"a": 1}}, {TrackNumbers: map[string]int64{"a": 2}}, {TrackNumbers: map[string]int64{"a": 3}}}
	for _, s := range steps {
		if err := Save(p, s); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(BackupPath(p))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"a": 2`) {
		t.Fatalf("the backup is the file before the last change, got %s", b)
	}
	if m := glob(t, p, "settings.json.*"); len(m) != 1 {
		t.Fatalf("only one backup copy: %v", m)
	}
	if info, err := os.Stat(BackupPath(p)); err != nil || info.Mode().Perm()&0o077 != 0 && !isWindows() {
		t.Fatalf("the backup is as private as the file: %v %v", info, err)
	}
}

func isWindows() bool { return os.PathSeparator == '\\' }

func TestAnEmptyFileIsDamagedAndTheBackupHasTheKits(t *testing.T) {
	p := withKit(t)
	if err := os.WriteFile(p, nil, 0o600); err != nil { // what a crash right after a replace can leave
		t.Fatal(err)
	}
	got, note, err := LoadNote(p)
	if err != nil {
		t.Fatalf("the backup is good, so the file reads: %v", err)
	}
	if !strings.Contains(note, "empty") || !strings.Contains(note, "settings.json.bak") {
		t.Fatalf("the note must say what happened and where the settings came from: %q", note)
	}
	kitSurvived(t, got)
	if s, err := LoadStrict(p); err != nil || len(s.Kits) != 1 {
		t.Fatalf("LoadStrict reads the same: %v %+v", err, s)
	}
	if len(Load(p).Kits) != 1 {
		t.Fatal("Load reads the same")
	}
}

func TestACutOffFileIsDamagedAndTheBackupHasTheKits(t *testing.T) {
	p := withKit(t)
	whole, _ := os.ReadFile(p)
	if err := os.WriteFile(p, whole[:len(whole)/2], 0o600); err != nil {
		t.Fatal(err)
	}
	got, note, err := LoadNote(p)
	if err != nil || note == "" {
		t.Fatalf("want the backup and a note, got %v %q", err, note)
	}
	kitSurvived(t, got)
}

func TestTheNextChangeAfterARecoveryKeepsTheDamagedCopyAndStartsFromTheBackup(t *testing.T) {
	p := withKit(t)
	bak, _ := os.ReadFile(BackupPath(p))
	cut := []byte(`{"autoRefillOnOpen": tr`)
	os.WriteFile(p, cut, 0o600)
	next, err := Update(p, func(s *Settings) error { s.AutoRefillOnOpen = true; s.SetJob("autoRepairGear", false); return nil })
	if err != nil {
		t.Fatalf("the backup is good, so a change goes through: %v", err)
	}
	kitSurvived(t, next)
	got, note, err := LoadNote(p)
	if err != nil || note != "" {
		t.Fatalf("the file is whole again: %v %q", err, note)
	}
	kitSurvived(t, got)
	if got.JobOn("autoRepairGear", true) {
		t.Fatal("the change did not land")
	}
	kept := glob(t, p, "settings.json.damaged-*")
	if len(kept) != 1 {
		t.Fatalf("the damaged file must be kept beside it: %v", kept)
	}
	if b, _ := os.ReadFile(kept[0]); string(b) != string(cut) {
		t.Fatalf("the kept copy is not the damaged file: %q", b)
	}
	if b, _ := os.ReadFile(BackupPath(p)); string(b) != string(bak) {
		t.Fatal("a damaged file must never become the backup")
	}
	// and the change after that is a normal one: the file before it becomes the backup
	whole, _ := os.ReadFile(p)
	if _, err := Update(p, func(s *Settings) error { s.AutoRefillOnOpen = false; return nil }); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(BackupPath(p)); string(b) != string(whole) {
		t.Fatal("the backup is the file before the last change")
	}
	if len(glob(t, p, "settings.json.damaged-*")) != 1 {
		t.Fatal("no further damaged copies")
	}
}

func TestDamagedWithNoGoodBackupIsReportedAndNeverOverwrittenWithoutACopy(t *testing.T) {
	for name, content := range map[string]string{"empty": "", "spaces": " \n", "cut": `{"kits": [{"name": "Sta`, "null": "null"} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "settings.json")
			os.WriteFile(p, []byte(content), 0o600)
			if _, _, err := LoadNote(p); err == nil || !strings.Contains(err.Error(), "damaged") || !strings.Contains(err.Error(), "settings.json") || strings.Contains(err.Error(), filepath.Dir(p)) {
				t.Fatalf("want the damage reported by file name, got %v", err)
			}
			if s := Load(p); s.AutoRefillOnOpen || len(s.Kits) != 0 {
				t.Fatal("reading a damaged file gives the defaults")
			}
			called := false
			if _, err := Update(p, func(s *Settings) error { called = true; return nil }); err == nil || called {
				t.Fatalf("a change must be refused, not attempted: %v %v", err, called)
			}
			if b, _ := os.ReadFile(p); string(b) != content {
				t.Fatal("the damaged file was changed")
			}
			// a direct Save does not read the file as settings, but it must not make the old content vanish
			if err := Save(p, Settings{AutoRefillOnOpen: true}); err != nil {
				t.Fatal(err)
			}
			kept := glob(t, p, "settings.json.damaged-*")
			if len(kept) != 1 {
				t.Fatalf("the damaged file must be kept: %v", kept)
			}
			if b, _ := os.ReadFile(kept[0]); string(b) != content {
				t.Fatalf("the kept copy differs: %q", b)
			}
			if _, err := os.Stat(BackupPath(p)); err == nil {
				t.Fatal("a damaged file must never become the backup")
			}
		})
	}
}

func TestADamagedBackupIsNoWayBack(t *testing.T) {
	p := withKit(t)
	os.WriteFile(BackupPath(p), nil, 0o600)
	os.WriteFile(p, nil, 0o600)
	if _, err := LoadStrict(p); err == nil {
		t.Fatal("both copies are empty: the damage is reported")
	}
	if _, err := Update(p, func(s *Settings) error { return nil }); err == nil {
		t.Fatal("and nothing is written")
	}
}

func TestAFileRemovedOnPurposeDoesNotComeBackFromItsBackup(t *testing.T) {
	p := withKit(t)
	os.Remove(p)
	if s, note, err := LoadNote(p); err != nil || note != "" || len(s.Kits) != 0 {
		t.Fatalf("a missing file is the defaults: %v %q %+v", err, note, s)
	}
	if _, err := Update(p, func(s *Settings) error { s.AutoRefillOnOpen = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(BackupPath(p)); err == nil {
		t.Fatal("the old backup must go with the file that was removed")
	}
	os.WriteFile(p, nil, 0o600)
	if _, err := LoadStrict(p); err == nil {
		t.Fatal("an empty file is damaged, and the removed file's kits are not behind it")
	}
}

func TestDamagedCopiesDoNotOverwriteEachOther(t *testing.T) {
	hooksBack(t)
	fixed := time.Date(2026, 10, 26, 15, 30, 12, 0, time.UTC)
	now = func() time.Time { return fixed }
	p := filepath.Join(t.TempDir(), "settings.json")
	for i, junk := range []string{"first", "second"} {
		os.WriteFile(p, []byte(junk), 0o600)
		if err := Save(p, Settings{}); err != nil {
			t.Fatalf("%d: %v", i, err)
		}
	}
	kept := glob(t, p, "settings.json.damaged-*")
	if len(kept) != 2 {
		t.Fatalf("two damaged files in the same second, two copies: %v", kept)
	}
	if filepath.Base(kept[0]) != "settings.json.damaged-20261026-153012" {
		t.Fatalf("the name carries the time: %v", kept)
	}
}
