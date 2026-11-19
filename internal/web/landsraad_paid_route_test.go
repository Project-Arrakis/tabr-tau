package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The paid-ground record at the route level (#834, #838).

// When a record that kept failing has dropped notes of written edits, a raise through the route is refused as a write failure (HTTP 500, code write_failed) with nothing written and
// nothing queued. The refusal happens inside the edit, so the answer is the one a failed write gets, and the page shows it as the refusal it is.
func TestARaiseRouteIsRefusedAsAWriteFailureWhenAFailingRecordDroppedNotes(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld), config.Dir{Path: t.TempDir()})
	s.KeepPendingEdits(t.TempDir())
	s.Save.SetNoteSink(func([]save.Note) error { return errors.New("the record file is held by another program") }) // the sink keeps failing
	for i := 0; i < 1001; i++ {                                                                                     // one more than the save keeps for a sink that fails
		if _, err := s.Save.Mutate("note", func(m *save.Mut) error {
			if _, err := m.Exec(`insert into player_virtual_currency_balances(player_controller_id, currency_id, balance) values (1, 0, 1)
				on conflict(player_controller_id, currency_id) do update set balance=balance+1`); err != nil {
				return err
			}
			return m.Note("landsraad.paid", map[string]any{"player": 1, "term": 7, "house": "DA_HouseOther", "faction": 2, "paid": 100})
		}); err != nil {
			t.Fatal(err)
		}
	}
	if f := s.Save.Flush(); !f.Saved {
		t.Fatalf("the edits were not written: %+v", f)
	}
	if !s.Save.NotesDropped() {
		t.Fatal("the save does not say that notes were dropped")
	}
	solari, scrip, xp := grantTotals(t, s.Save.Path)
	w := postEdit(t, s, "/api/landsraad/apply", `{"term_id":7,"houses":[{"task_id":175,"was_mine":1300,"mine_to":6000}]}`)
	m := decodeBody(t, w)
	if w.Code != 500 || m["code"] != "write_failed" || m["outcome"] != nil {
		t.Fatalf("a raise after dropped notes: %d %s", w.Code, w.Body.String())
	}
	msg, _ := m["error"].(string)
	if !strings.Contains(msg, "refused") || !strings.Contains(msg, "nothing was written") || strings.Contains(msg, os.TempDir()) {
		t.Fatalf("the refusal says what happened and names no folder: %q", msg)
	}
	if s.Save.Dirty() {
		t.Fatal("a refused raise left an edit queued")
	}
	if solari2, scrip2, xp2 := grantTotals(t, s.Save.Path); solari2 != solari || scrip2 != scrip || xp2 != xp {
		t.Fatalf("a refused raise wrote: %d %d %d, was %d %d %d", solari2, scrip2, xp2, solari, scrip, xp)
	}
	if got := landsraadMineAmounts(t, s.Save.Path); got[1] != 1300 {
		t.Fatalf("the contribution changed: %v", got)
	}
}

// A record file that is not usable is kept aside by the next write, and the state the page polls names the copy (without its folder), so the person is told (#838).
func TestAMovedAsideRecordIsNamedInTheState(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, landsraadChosenWorld), config.Dir{Path: t.TempDir()})
	dir := t.TempDir()
	s.KeepPendingEdits(dir)
	state := func() map[string]any {
		var m map[string]any
		if err := json.Unmarshal([]byte(postEditGet(t, s, "/api/save/state")), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	if got := state()["recordNotice"]; got != "" {
		t.Fatalf("a notice before anything happened: %v", got)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "landsraad-paid-*.json"))
	if len(files) != 0 {
		t.Fatalf("the record exists before a raise: %v", files)
	}
	h := sha256.Sum256([]byte(strings.ToLower(s.Save.Path))) // the name KeepPendingEdits gives the record
	path := filepath.Join(dir, ops.PaidLedgerName(hex.EncodeToString(h[:6])))
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if w := postEdit(t, s, "/api/landsraad/apply", `{"term_id":7,"houses":[{"task_id":175,"was_mine":1300,"mine_to":6000}]}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	notice, _ := state()["recordNotice"].(string)
	aside, _ := filepath.Glob(path + ".damaged-*")
	if len(aside) != 1 || !strings.Contains(notice, filepath.Base(aside[0])) || strings.Contains(notice, dir) {
		t.Fatalf("the notice names the copy by its name: %q, copies %v", notice, aside)
	}
}
