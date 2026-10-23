package save

import (
	"path/filepath"
	"testing"
	"time"
)

// A clock set back gives no backup an old time: the next one is later than every one in the folder.
func TestABackupTakenAfterTheClockWasSetBackIsStillTheNewest(t *testing.T) {
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	s, _ := Open(p)
	defer s.Close()
	future := fakeAutoAt(t, s, time.Now().Add(48*time.Hour))
	got := takeAuto(t, s)
	at, ok := s.backupInstant(filepath.Base(got))
	fat, _ := s.backupInstant(filepath.Base(future))
	if !ok || at <= fat {
		t.Fatalf("the new backup (%d) must be later than the one from the future (%d)", at, fat)
	}
}
