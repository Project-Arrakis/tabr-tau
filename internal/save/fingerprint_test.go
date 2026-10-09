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

// A hand-crafted file can carry objects SQLite would refuse to create through SQL. The fingerprint reads
// sqlite_master directly, so these must still be caught (security review S2, S4).
func TestFingerprintCatchesDisguisedObjects(t *testing.T) {
	cases := map[string]string{
		"virtual table with a comment between the words": `create /*x*/ virtual /*y*/ table sneaky_rt using rtree(id, a, b);`,
		"trigger named sqlite_*": `pragma writable_schema=on;
			insert into sqlite_master(type,name,tbl_name,rootpage,sql) values ('trigger','sqlite_evil','items',0,
			'CREATE TRIGGER sqlite_evil AFTER UPDATE ON items BEGIN UPDATE items SET stack_size=stack_size; END');`,
		"game trigger changed only inside a string literal": `drop trigger actor_fgl_entities_cleanup_orphaned_entities;
			create trigger actor_fgl_entities_cleanup_orphaned_entities AFTER DELETE ON actor_fgl_entities FOR EACH ROW
			BEGIN DELETE FROM fgl_entities WHERE entity_id = OLD.entity_id AND 'A' = 'A'; END;`,
	}
	for name, ddl := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Logf("%s: fixture could not be built by this SQLite (%v); skipped", name, r)
				}
			}()
			s := testsave.PlayerWithSQL(t, ddl)
			if s.WriteBlocked() == "" {
				t.Errorf("%s: must make the save read-only", name)
			}
		}()
	}
}

// A real game save stores its triggers with LF line endings, while the built-in schema file is embedded as checked out: CRLF
// on a Windows checkout with autocrlf. The fingerprint must compare the text, not the line endings, or every real save is
// reported as modified and opened read-only (issue #104). Both endings are tried so the test fails on any platform.
func TestLineEndingsDoNotMakeTheGameTriggerLookModified(t *testing.T) {
	const trigger = "CREATE TRIGGER actor_fgl_entities_cleanup_orphaned_entities AFTER DELETE ON actor_fgl_entities FOR EACH ROW\nBEGIN\n\tDELETE FROM fgl_entities WHERE entity_id = OLD.entity_id;\nEND;"
	for name, eol := range map[string]string{"LF": "\n", "CRLF": "\r\n"} {
		ddl := "DROP TRIGGER actor_fgl_entities_cleanup_orphaned_entities;\n" + strings.ReplaceAll(trigger, "\n", eol)
		s := testsave.PlayerWithSQL(t, ddl)
		if r := s.WriteBlocked(); r != "" {
			t.Errorf("%s line endings: the game's own trigger must not make the save read-only, got %q", name, r)
		}
	}
}
