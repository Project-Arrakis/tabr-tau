package save

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// makeSave writes a container holding the real game schema plus ddl (a toy table "t") and returns its path.
// The real schema is needed because a save with a different set of triggers is deliberately read-only.
func makeSave(t *testing.T, ddl string) string {
	t.Helper()
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain.sqlite")
	db, err := sql.Open("sqlite", plain)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(KnownSchema + ddl); err != nil {
		t.Fatal(err)
	}
	db.Close()
	raw, _ := os.ReadFile(plain)
	blob, err := Encode(raw)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "game.db")
	if err := os.WriteFile(p, blob, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCodecRoundTripAndHeader(t *testing.T) {
	p := makeSave(t, `create table t(a integer); insert into t values (1),(2);`)
	blob, _ := os.ReadFile(p)
	if blob[0] != 1 || blob[1] != 0 || blob[2] != 0 || blob[3] != 0 {
		t.Fatalf("flag bytes wrong: % x", blob[:4])
	}
	raw, err := Decode(blob)
	if err != nil {
		t.Fatal(err)
	}
	size := uint32(blob[4]) | uint32(blob[5])<<8 | uint32(blob[6])<<16 | uint32(blob[7])<<24
	if int(size) != len(raw) {
		t.Fatalf("size header %d != payload %d", size, len(raw))
	}
	again, _ := Encode(raw)
	if back, _ := Decode(again); !bytes.Equal(back, raw) {
		t.Fatal("round trip mismatch")
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	for _, b := range [][]byte{nil, []byte("short"), append([]byte{9, 0, 0, 0, 1, 0, 0, 0}, 0x78, 0x9c, 0, 0)} {
		if _, err := Decode(b); err == nil {
			t.Fatalf("expected error for %v", b)
		}
	}
	if _, err := Encode([]byte("not sqlite")); err == nil {
		t.Fatal("Encode must refuse non-SQLite payloads")
	}
}

func TestExecScriptIsAtomic(t *testing.T) {
	s, err := Open(makeSave(t, `create table t(a integer primary key); insert into t values (1);`))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.ExecScript(`insert into t values (2); insert into t values (1);`); err == nil {
		t.Fatal("expected constraint failure")
	}
	r, _ := s.One(`select count(*) c from t`)
	if r["c"].(int64) != 1 || s.Dirty() {
		t.Fatalf("failed script must roll back completely, got %v dirty=%v", r, s.Dirty())
	}
	n, err := s.ExecScript(`insert into t values (5); insert into t values (6);`)
	if err != nil || n != 2 || !s.Dirty() {
		t.Fatalf("n=%d err=%v dirty=%v", n, err, s.Dirty())
	}
}

func TestReadOnlyBlocksWrites(t *testing.T) {
	s, _ := Open(makeSave(t, `create table t(a integer); insert into t values (1);`))
	defer s.Close()
	if _, _, err := s.ReadOnlyTable(`delete from t`, 10); err == nil {
		t.Fatal("write must fail in read-only mode")
	}
	if _, err := s.ExecScript(`insert into t values (2)`); err != nil {
		t.Fatalf("writes must work again afterwards: %v", err)
	}
}

func TestCommitBacksUpAndPersists(t *testing.T) {
	p := makeSave(t, `create table t(a integer); insert into t values (1);`)
	orig, _ := os.ReadFile(p)
	s, _ := Open(p)
	defer s.Close()
	if r, _ := s.Commit(false); r["saved"] != false {
		t.Fatal("commit with no changes must be a no-op")
	}
	s.ExecScript(`insert into t values (2)`)
	res, err := s.Commit(false)
	if err != nil {
		t.Fatal(err)
	}
	bak, _ := os.ReadFile(res["backup"].(string))
	if !bytes.Equal(bak, orig) {
		t.Fatal("backup must equal the original file")
	}
	s2, _ := Open(p)
	defer s2.Close()
	r, _ := s2.One(`select count(*) c from t`)
	if r["c"].(int64) != 2 || s.Dirty() {
		t.Fatalf("saved file should hold 2 rows, got %v", r)
	}
	if len(s.Backups()) != 1 {
		t.Fatal("expected one listed backup")
	}
}

func TestCommitDetectsExternalChange(t *testing.T) {
	p := makeSave(t, `create table t(a integer);`)
	s, _ := Open(p)
	defer s.Close()
	s.ExecScript(`insert into t values (1)`)
	other := makeSave(t, `create table t(a integer); insert into t values (9);`)
	b, _ := os.ReadFile(other)
	os.WriteFile(p, b, 0o644) // the game autosaved in the meantime
	if _, err := s.Commit(false); err == nil {
		t.Fatal("commit must refuse when the file changed on disk")
	}
}
