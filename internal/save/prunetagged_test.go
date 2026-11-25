package save

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A backup of the save "game.db" of one kind, written into the backup folder with the time in its name and as its file time. Temp folders only.
func putTagged(t *testing.T, s *Save, tag string, at time.Time) string {
	t.Helper()
	if err := os.MkdirAll(s.backupDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("game-%sZ-%09d%s.db", at.UTC().Format("20060102-150405"), at.UTC().Nanosecond(), tag)
	return putNamed(t, s, name, at)
}

func putNamed(t *testing.T, s *Save, name string, mod time.Time) string {
	t.Helper()
	if err := os.MkdirAll(s.backupDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(s.backupDir(), name)
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mod, mod); err != nil {
		t.Fatal(err)
	}
	return p
}

func tFileThere(p string) bool { _, err := os.Lstat(p); return err == nil }

// days old, as a time
func ago(d int) time.Time { return time.Now().Add(-time.Duration(d) * 24 * time.Hour) }

// Old backups of the kind beyond the newest 8 go; the newest 8 stay.
func TestPruneTaggedKeepsTheNewestEightAndRemovesTheOldOnes(t *testing.T) {
	for _, tag := range []string{manualTag, checkpointTag} {
		s := openBackupNow(t)
		var ps []string
		for i := 0; i < 12; i++ { // 10..21 days old: the first of the list is the newest
			ps = append(ps, putTagged(t, s, tag, ago(10+i)))
		}
		s.pruneTagged(tag, ps[0])
		for i, p := range ps {
			if want := i < keepTaggedBackups; tFileThere(p) != want {
				t.Fatalf("%s: backup %d (%d days old): kept=%v want %v", tag, i, 10+i, tFileThere(p), want)
			}
		}
	}
}

// Nothing younger than 7 days is removed, whatever the count, by its name or by its file time; the newest is never removed.
func TestPruneTaggedNeverRemovesAYoungOrTheNewestBackup(t *testing.T) {
	s := openBackupNow(t)
	var young []string
	for i := 0; i < 12; i++ { // 12 backups, all 1-6 days old
		young = append(young, putTagged(t, s, manualTag, ago(1).Add(-time.Duration(i)*time.Hour)))
	}
	oldName := putTagged(t, s, manualTag, ago(30))
	// old by its name, but its file was written a day ago: kept
	freshFile := putTagged(t, s, manualTag, ago(40))
	if err := os.Chtimes(freshFile, ago(1), ago(1)); err != nil {
		t.Fatal(err)
	}
	// young by its name, old by its file time: kept
	youngName := putTagged(t, s, manualTag, ago(2).Add(-time.Hour*30))
	if err := os.Chtimes(youngName, ago(60), ago(60)); err != nil {
		t.Fatal(err)
	}
	s.pruneTagged(manualTag, young[0])
	for _, p := range young {
		if !tFileThere(p) {
			t.Fatalf("a backup under 7 days old was removed: %s", p)
		}
	}
	if !tFileThere(freshFile) || !tFileThere(youngName) {
		t.Fatal("a backup with a young name or a young file time was removed")
	}
	if tFileThere(oldName) {
		t.Fatal("an old backup beyond the newest 8 was kept")
	}
}

// Even when everything is old, the newest is never removed (also with a keep count of one, by the rule that it is never among the removable).
func TestPruneTaggedKeepsTheNewestWhenAllAreOld(t *testing.T) {
	s := openBackupNow(t)
	var ps []string
	for i := 0; i < 10; i++ {
		ps = append(ps, putTagged(t, s, checkpointTag, ago(100+i)))
	}
	s.pruneTagged(checkpointTag, ps[0])
	if !tFileThere(ps[0]) {
		t.Fatal("the newest backup was removed")
	}
	if tFileThere(ps[9]) || tFileThere(ps[8]) {
		t.Fatal("the oldest were kept")
	}
}

// Only names that are exactly this save's backup name for the kind are removed: look-alikes (a prefix, a suffix, an extra tag, another save, a directory, a different
// extension) are kept, and so are the save, the automatic backups, the other kind and the game's own backup files.
func TestPruneTaggedOnlyRemovesTheExactNameOfTheKind(t *testing.T) {
	s := openBackupNow(t)
	for i := 0; i < 9; i++ { // 9 real ones: one is over the count and old
		putTagged(t, s, manualTag, ago(20+i))
	}
	oldest := putTagged(t, s, manualTag, ago(90))
	old := ago(200)
	stamp := old.UTC().Format("20060102-150405")
	ns := fmt.Sprintf("%09d", old.UTC().Nanosecond())
	keepNames := []string{
		"x-game-" + stamp + "Z-" + ns + "-manual.db",          // a prefix
		"game-" + stamp + "Z-" + ns + "-manual.db.old",        // a suffix
		"game-" + stamp + "Z-" + ns + "-manual-manual.db",     // a doubled tag
		"game-" + stamp + "Z-" + ns + "-layoutroll-manual.db", // two tags
		"game-" + stamp + "Z-" + ns + "-manual.dbx",           // another extension
		"game-" + stamp + "Z-" + ns + "-manual.bak",           // another extension
		"other-" + stamp + "Z-" + ns + "-manual.db",           // another save
		"game-" + stamp + "Z-" + ns + "-auto.db",              // an automatic backup
		"game-" + stamp + "Z-" + ns + "-layoutroll.db",        // the other kind
		"game-" + stamp + "Z-" + ns + ".db",                   // an untagged backup of an earlier version
		"game-" + stamp + "Z-" + ns + "-manual",               // no extension
		"game.db.bak",                                         // the game's own file
		"game.db",                                             // the save's name
		"game-" + stamp + "Z-abc-manual.db",                   // a stamp that is not one
		"game-" + stamp + "Z-" + ns + "-MANUAL.db",            // another case
		"game-" + stamp[:8] + "Z-" + ns + "-manual.db",        // a stamp cut short
	}
	var kept []string
	for _, n := range keepNames {
		kept = append(kept, putNamed(t, s, n, old))
	}
	dir := filepath.Join(s.backupDir(), "game-"+stamp+"Z-"+"000000001"+"-manual.db") // a directory with the exact name of a backup
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	s.pruneTagged(manualTag, "")
	if tFileThere(oldest) {
		t.Fatal("the old backup beyond the count was kept")
	}
	for _, p := range append(kept, dir) {
		if !tFileThere(p) {
			t.Fatalf("a file that is not this kind of backup was removed: %s", filepath.Base(p))
		}
	}
	if !tFileThere(s.Path) {
		t.Fatal("the save was removed")
	}
}

// The other kind and the automatic backups are not touched by the pruning of a kind, and a kind that is not ours is refused.
func TestPruneTaggedLeavesTheOtherKindsAlone(t *testing.T) {
	s := openBackupNow(t)
	var cps, autos []string
	for i := 0; i < 12; i++ {
		cps = append(cps, putTagged(t, s, checkpointTag, ago(20+i)))
		autos = append(autos, putTagged(t, s, autoTag, ago(20+i)))
	}
	s.pruneTagged(manualTag, "")
	s.pruneTagged(autoTag, "")
	s.pruneTagged("", "")
	for _, p := range append(cps, autos...) {
		if !tFileThere(p) {
			t.Fatalf("a backup of another kind was removed: %s", filepath.Base(p))
		}
	}
}

// A file that cannot be removed is logged and left, and the others are still removed; the backup that was just taken is not failed by it.
func TestPruneTaggedSurvivesAFileThatCannotBeRemoved(t *testing.T) {
	s := openBackupNow(t)
	var ps []string
	for i := 0; i < 12; i++ {
		ps = append(ps, putTagged(t, s, manualTag, ago(10+i)))
	}
	stuck := ps[9]
	old := removeBackupFile
	t.Cleanup(func() { removeBackupFile = old })
	removeBackupFile = func(p string) error {
		if p == stuck {
			return errors.New("the file is in use")
		}
		return os.Remove(p)
	}
	s.pruneTagged(manualTag, ps[0])
	if !tFileThere(stuck) {
		t.Fatal("the file that could not be removed is gone")
	}
	if tFileThere(ps[8]) || tFileThere(ps[10]) || tFileThere(ps[11]) {
		t.Fatal("the failure stopped the other removals")
	}
	// and through the real call: a backup is taken, verified and returned although a removal fails
	m, err := s.BackupNow()
	if err != nil {
		t.Fatalf("a failed removal failed the backup: %v", err)
	}
	if !tFileThere(m.Path) {
		t.Fatal("the new backup is not there")
	}
}

// BackupNow and BackupCheckpoint run the pruning after the copy is verified, and never remove the backup they just took.
func TestBackupNowAndCheckpointPruneTheirOwnKind(t *testing.T) {
	for _, tag := range []string{manualTag, checkpointTag} {
		s := openBackupNow(t)
		var ps []string
		for i := 0; i < 10; i++ {
			ps = append(ps, putTagged(t, s, tag, ago(10+i)))
		}
		auto := putTagged(t, s, autoTag, ago(40))
		var m ManualBackup
		var err error
		if tag == manualTag {
			m, err = s.BackupNow()
		} else {
			m, err = s.BackupCheckpoint()
		}
		if err != nil {
			t.Fatal(err)
		}
		if !tFileThere(m.Path) {
			t.Fatal("the backup just taken is gone")
		}
		if !tFileThere(ps[0]) || !tFileThere(ps[6]) || tFileThere(ps[8]) || tFileThere(ps[9]) {
			t.Fatalf("%s: the newest 8 (the new one and 7 of the old) are kept and the rest go", tag)
		}
		if !tFileThere(auto) || !tFileThere(s.Path) {
			t.Fatal("an automatic backup or the save was removed")
		}
	}
}

// A backup folder that is a link is not pruned through.
func TestPruneTaggedDoesNotFollowALinkedFolder(t *testing.T) {
	s := openBackupNow(t)
	target := t.TempDir()
	var ps []string
	for i := 0; i < 12; i++ {
		p := filepath.Join(target, fmt.Sprintf("game-%sZ-%09d%s.db", ago(30+i).UTC().Format("20060102-150405"), 0, manualTag))
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(p, ago(30+i), ago(30+i))
		ps = append(ps, p)
	}
	if err := os.Symlink(target, s.backupDir()); err != nil {
		t.Skipf("links cannot be made here: %v", err)
	}
	s.pruneTagged(manualTag, "")
	for _, p := range ps {
		if !tFileThere(p) {
			t.Fatal("a file was removed through a linked backup folder")
		}
	}
}
