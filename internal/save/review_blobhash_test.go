package save

import (
	"strings"
	"testing"
)

// a 37-byte blob of zeros, and the same with byte 30 changed
var (
	blobDDL = `create table bt(a integer primary key, b blob, n integer);
insert into bt values (1, x'` + strings.Repeat("00", 37) + `', 0);`
	blobTailChanged = `update bt set b=x'` + strings.Repeat("00", 29) + "ff" + strings.Repeat("00", 7) + `' where a=1;`
)

// The rows an edit read are hashed with the whole blob, not only the 24 bytes the display shows: the game changing a byte after the 24th must stop the edit being put back.
func TestAnEditThatReadALongBlobIsNotPutBackWhenTheGameChangedItsTail(t *testing.T) {
	p := makeSave(t, blobDDL)
	dir := t.TempDir()
	s1 := openJournaled(t, p, dir)
	if _, err := s1.Mutate("count by the blob", func(m *Mut) error {
		r, err := m.One(`select b, n from bt where a=1`)
		if err != nil || r == nil {
			return err
		}
		_, err = m.Exec(`update bt set n=? where a=1`, r["n"].(int64)+1)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	s1.Close()
	putNewerFile(t, p, blobDDL+blobTailChanged)
	s2 := openJournaled(t, p, dir)
	r, err := s2.RestoreJournal()
	if err != nil || r == nil || r.Applied != 0 || len(r.Conflicts) != 1 {
		t.Fatalf("the edit read a blob that has changed: it is not put back: %+v %v", r, err)
	}
}

// An edit that read a short blob that the game did not change is still put back (the hash of a blob of 24 bytes or fewer is what it was before).
func TestAnEditThatReadALongBlobIsPutBackWhenItIsTheSame(t *testing.T) {
	p := makeSave(t, blobDDL)
	dir := t.TempDir()
	s1 := openJournaled(t, p, dir)
	if _, err := s1.Mutate("touch", func(m *Mut) error {
		if _, err := m.One(`select b from bt where a=1`); err != nil {
			return err
		}
		_, err := m.Exec(`update bt set n=5 where a=1`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	s1.Close()
	s2 := openJournaled(t, p, dir)
	if r, err := s2.RestoreJournal(); err != nil || r == nil || r.Applied != 1 || len(r.Conflicts) != 0 {
		t.Fatalf("the blob is the same: %+v %v", r, err)
	}
}
