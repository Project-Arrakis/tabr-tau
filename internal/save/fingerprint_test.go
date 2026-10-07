package save_test

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The schema fingerprint (audit SEC-4): a save carrying database objects the editor does not know could run hidden
// logic (triggers, views, virtual tables) whenever an ordinary edit touches a table, and the editor's diff would never
// show it. Such saves stay readable but are made read-only, with a reason the UI can show.

func TestKnownRealSchemaIsWritable(t *testing.T) {
	for name, s := range map[string]interface{ WriteBlocked() string }{"empty": testsave.Empty(t), "player": testsave.Player(t), "filled": testsave.Filled(t)} {
		if r := s.WriteBlocked(); r != "" {
			t.Errorf("%s: a save with the real game schema must be writable, got %q", name, r)
		}
	}
}

func TestUnknownDatabaseObjectsMakeTheSaveReadOnly(t *testing.T) {
	cases := map[string]struct{ sql, reasonHas string }{
		"extra trigger": {`create trigger evil after update on items begin update items set stack_size=stack_size; end;`, "evil"},
		"view":          {`create view sneaky as select * from items;`, "sneaky"},
		"virtual table": {`create virtual table sneaky_rt using rtree(id, a, b);`, "sneaky_rt"},
		"modified game trigger": {`drop trigger actor_fgl_entities_cleanup_orphaned_entities;
			create trigger actor_fgl_entities_cleanup_orphaned_entities after delete on actor_fgl_entities begin delete from items; end;`, "actor_fgl_entities_cleanup_orphaned_entities"},
		"game trigger removed": {`drop trigger actor_fgl_entities_cleanup_orphaned_entities;`, "actor_fgl_entities_cleanup_orphaned_entities"},
	}
	for name, c := range cases {
		s := testsave.PlayerWithSQL(t, c.sql)
		r := s.WriteBlocked()
		if r == "" {
			t.Errorf("%s: save must be read-only", name)
			continue
		}
		if !strings.Contains(r, c.reasonHas) {
			t.Errorf("%s: reason %q should name %q", name, r, c.reasonHas)
		}
		// every write path refuses, with the reason
		if _, err := s.Exec(`update items set stack_size=1`); err == nil || !strings.Contains(err.Error(), "read-only") {
			t.Errorf("%s: Exec must refuse with a read-only message, got %v", name, err)
		}
		if _, err := s.ExecScript(`update items set stack_size=1`); err == nil {
			t.Errorf("%s: ExecScript must refuse", name)
		}
		ran := false
		if _, err := s.Mutate("x", func(m *save.Mut) error { ran = true; return nil }); err == nil || ran {
			t.Errorf("%s: Mutate must refuse without running its function (err=%v ran=%v)", name, err, ran)
		}
		// reads still work
		if rows, err := s.Query(`select count(*) c from items`); err != nil || len(rows) != 1 {
			t.Errorf("%s: reads must keep working: %v %v", name, rows, err)
		}
		if _, err := s.Commit(false); err != nil && !strings.Contains(err.Error(), "read-only") {
			// nothing dirty: Commit reports no changes; if it errors it must be for the right reason
			t.Errorf("%s: unexpected commit error %v", name, err)
		}
	}
}

func TestDiscardRecomputesTheFingerprint(t *testing.T) {
	s := testsave.Player(t)
	if s.WriteBlocked() != "" {
		t.Fatal("fixture should be writable")
	}
	if err := s.Discard(); err != nil {
		t.Fatal(err)
	}
	if s.WriteBlocked() != "" {
		t.Fatal("still writable after discard")
	}
}
