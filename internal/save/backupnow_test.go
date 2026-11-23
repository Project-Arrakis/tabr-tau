package save

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const backupNowDDL = `create table t(a integer); insert into t values (1);`

func openBackupNow(t *testing.T) *Save {
	t.Helper()
	s, err := Open(makeSave(t, backupNowDDL))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func manualNames(s *Save) []string {
	var out []string
	for _, b := range s.Backups() {
		if b["manual"] == true {
			out = append(out, b["name"].(string))
		}
	}
	return out
}

// Back up now copies the save file as it is into the backup folder, with a name that carries the time and the manual tag, and says the size and the SHA-256 it verified. The save
// file itself is not written: its bytes and its time are the same afterwards, and no edit is pending.
func TestBackupNowCopiesTheSaveVerifiesItAndWritesNothingToTheSave(t *testing.T) {
	s := openBackupNow(t)
	before, _ := os.ReadFile(s.Path)
	stBefore, _ := os.Stat(s.Path)
	got, err := s.BackupNow()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(got.Path) != s.backupDir() || filepath.Base(got.Path) != got.Name {
		t.Fatalf("the backup is not in the editor's backup folder: %+v", got)
	}
	if !strings.HasPrefix(got.Name, "game-") || !strings.HasSuffix(got.Name, manualTag+".db") {
		t.Fatalf("name: %s", got.Name)
	}
	if _, _, ok := s.parseBackupName(got.Name); !ok {
		t.Fatalf("the name has no time the list can sort by: %s", got.Name)
	}
	copyBytes, err := os.ReadFile(got.Path)
	if err != nil || !bytes.Equal(copyBytes, before) {
		t.Fatalf("the copy is not the file: %v", err)
	}
	sum := sha256.Sum256(before)
	if got.Size != int64(len(before)) || got.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("size and checksum: %+v", got)
	}
	if time.Since(got.At) > time.Minute {
		t.Fatalf("time: %v", got.At)
	}
	after, _ := os.ReadFile(s.Path)
	stAfter, _ := os.Stat(s.Path)
	if !bytes.Equal(before, after) || !stAfter.ModTime().Equal(stBefore.ModTime()) || s.Dirty() || len(s.Ops()) != 0 {
		t.Fatal("Back up now changed the save or left something pending")
	}
	if m := manualNames(s); len(m) != 1 || m[0] != got.Name {
		t.Fatalf("the list does not mark it as manual: %v", s.Backups())
	}
}

// It works while the game is running: the copy is of what the game last wrote.
func TestBackupNowWorksWhileTheGameIsRunning(t *testing.T) {
	s := openBackupNow(t)
	defer ActiveSessionForTests()()
	if blocked, _ := s.gate(); !blocked {
		t.Fatal("the test needs the game to look like it is running")
	}
	if _, err := s.BackupNow(); err != nil {
		t.Fatalf("a running game must not stop a backup: %v", err)
	}
}

// A copy that is not of the file as it stands is refused and nothing is kept: the game wrote to the file between the copy and the second read.
func TestBackupNowRefusesWhenTheFileChangedUnderTheCopy(t *testing.T) {
	s := openBackupNow(t)
	old := afterManualCopy
	t.Cleanup(func() { afterManualCopy = old })
	afterManualCopy = func() { os.WriteFile(s.Path, []byte("the game wrote here"), 0o644) }
	if _, err := s.BackupNow(); err == nil {
		t.Fatal("a copy of a file that changed under it must be refused")
	}
	if ents, _ := os.ReadDir(s.backupDir()); len(ents) != 0 {
		t.Fatalf("a refused backup left a file: %v", ents)
	}
	if len(s.Backups()) != 0 {
		t.Fatalf("listed: %v", s.Backups())
	}
}

// A save file that is not there is refused and no file is left in the backup folder.
func TestBackupNowRefusesAMissingSaveAndLeavesNothing(t *testing.T) {
	s := openBackupNow(t)
	if err := os.Remove(s.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BackupNow(); err == nil {
		t.Fatal("no save file, no backup")
	}
	if ents, _ := os.ReadDir(s.backupDir()); len(ents) != 0 {
		t.Fatalf("left a file: %v", ents)
	}
}

// Two backups in the same instant get two names; they are never overwritten.
func TestTwoManualBackupsAreTwoFiles(t *testing.T) {
	s := openBackupNow(t)
	a, err := s.BackupNow()
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.BackupNow()
	if err != nil || a.Name == b.Name || len(manualNames(s)) != 2 {
		t.Fatalf("%v %v %v", err, a.Name, b.Name)
	}
	if got := s.Backups()[0]["name"]; got != b.Name {
		t.Fatalf("the newest is listed first: %v", got)
	}
}

// The pruning of the automatic backups neither removes a manual one nor counts it: with more than the limit of automatic backups taken after it, it is still there and the
// automatic ones are kept to the usual number.
func TestPruningNeitherRemovesNorCountsAManualBackup(t *testing.T) {
	s := openBackupNow(t)
	first := takeAuto(t, s)
	m, err := s.BackupNow()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < keepBackups+5; i++ {
		takeAuto(t, s)
	}
	if _, err := os.Stat(m.Path); err != nil {
		t.Fatalf("the pruning removed the manual backup: %v", err)
	}
	if _, err := os.Stat(first); err != nil {
		t.Fatalf("the oldest automatic backup is kept as before: %v", err)
	}
	auto := 0
	for _, b := range s.Backups() {
		if _, isAuto, ok := s.parseBackupName(b["name"].(string)); ok && isAuto {
			auto++
		}
	}
	// the newest keepBackups, the oldest and the first of the session (the same file here): the manual backup takes no place among them
	if auto != keepBackups+1 {
		t.Fatalf("%d automatic backups kept, want %d", auto, keepBackups+1)
	}
}

// Restore treats a manual backup like any other: it is put back over the save after the current file is backed up first.
func TestRestoreTakesAManualBackup(t *testing.T) {
	s := openBackupNow(t)
	m, err := s.BackupNow()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Exec(`update t set a=2`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitForTests(); err != nil {
		t.Fatal(err)
	}
	if err := s.Restore(m.Name); err != nil {
		t.Fatal(err)
	}
	r, err := s.One(`select a from t`)
	if err != nil || r["a"].(int64) != 1 {
		t.Fatalf("restored: %v %v", r, err)
	}
}
