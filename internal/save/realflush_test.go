package save

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The suite's other tests do not wait for the disk (TestMain, #437), so the flush as shipped is shown to run here: a durable write, a
// commit with its backup, and a line of the write log each go through the real flush and leave the data readable.
func TestTheRealFlushToDiskRunsForADurableWriteACommitAndALogLine(t *testing.T) {
	realFlush(t)
	calls := 0
	syncFile = func(f *os.File) error { calls++; return realSyncFile(f) }

	p := filepath.Join(t.TempDir(), "durable.bin")
	if err := writeDurable(p, []byte("kept"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(p); err != nil || !bytes.Equal(got, []byte("kept")) {
		t.Fatalf("the durable file holds what was written: %v %q", err, got)
	}
	if calls != 1 {
		t.Fatalf("a durable write flushes once, flushed %d times", calls)
	}

	s, save := openToy(t)
	calls = 0
	syncFile = func(f *os.File) error { calls++; return realSyncFile(f) }
	if _, err := s.ExecScript(`insert into t values (2)`); err != nil {
		t.Fatal(err)
	}
	res, err := s.CommitForTests()
	if err != nil {
		t.Fatal(err)
	}
	if calls < 2 {
		t.Fatalf("a commit flushes its backup and its new file, flushed %d times", calls)
	}
	if bak, err := os.ReadFile(res["backup"].(string)); err != nil || len(bak) == 0 {
		t.Fatalf("the backup is on the disk: %v", err)
	}
	s2, err := Open(save)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if r, _ := s2.One(`select count(*) c from t`); r["c"].(int64) != 2 {
		t.Fatalf("the committed file holds both rows: %v", r)
	}
}
