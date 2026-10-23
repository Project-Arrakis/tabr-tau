package save

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// takeAuto is what the automatic write does to take and prune a backup.
func takeAuto(t *testing.T, s *Save) string {
	t.Helper()
	b, _, err := s.backupNamed(autoTag)
	if err != nil {
		t.Fatal(err)
	}
	if s.firstBackup == "" {
		s.firstBackup = b
	}
	s.pruneBackups(b)
	return b
}

// The wall clock of the computer ran backwards (a daylight-saving change, a new zone): the backups taken after it carry an earlier local time. The pruning must still remove the
// oldest by when they were taken, not the ones whose local time reads earliest.
func TestPruningFollowsWhenBackupsWereTakenNotTheLocalClock(t *testing.T) {
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	oldLocal := time.Local
	t.Cleanup(func() { time.Local = oldLocal })
	var first, later []string
	time.Local = time.FixedZone("clock before", 0)
	for i := 0; i < 5; i++ {
		first = append(first, takeAuto(t, s))
	}
	time.Local = time.FixedZone("clock after", -5*3600) // the wall clock reads five hours earlier from here on
	for i := 0; i < keepBackups; i++ {
		later = append(later, takeAuto(t, s))
	}
	if !exists(first[0]) {
		t.Fatal("the oldest backup is always kept")
	}
	for _, f := range first[1:] {
		if exists(f) {
			t.Fatalf("an old backup beyond the limit was kept while a newer one was removed: %s", filepath.Base(f))
		}
	}
	for _, l := range later {
		if !exists(l) {
			t.Fatalf("a newer backup was removed: %s", filepath.Base(l))
		}
	}
}

// The names sort as the times do: nanoseconds are padded, the time is UTC, and the list is newest first.
func TestBackupNamesArePaddedAndTheListIsNewestFirst(t *testing.T) {
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	var names []string
	for i := 0; i < 4; i++ {
		names = append(names, filepath.Base(takeAuto(t, s)))
	}
	re := regexp.MustCompile(`^game-\d{8}-\d{6}Z-\d{9}-auto\.db$`)
	for _, n := range names {
		if !re.MatchString(n) {
			t.Fatalf("name %q is not <name>-<UTC time>Z-<9 digits>-auto.db", n)
		}
	}
	list := s.Backups()
	for i := range names {
		if got := list[i]["name"].(string); got != names[len(names)-1-i] {
			t.Fatalf("newest first: position %d is %s, want %s", i, got, names[len(names)-1-i])
		}
	}
}

// fakeAutoAt writes a file named as an earlier version named the automatic backup taken at the given time (local time, no Z, unpadded nanoseconds).
func fakeAutoAt(t *testing.T, s *Save, at time.Time) string {
	t.Helper()
	ext := filepath.Ext(s.Path)
	base := strings.TrimSuffix(filepath.Base(s.Path), ext)
	name := filepath.Join(s.backupDir(), fmt.Sprintf("%s-%s-%d%s%s", base, at.Format("20060102-150405"), 7, autoTag, ext))
	if err := os.MkdirAll(s.backupDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return name
}
