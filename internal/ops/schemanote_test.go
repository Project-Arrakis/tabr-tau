package ops

import (
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// DBA-16 of the 2026-10-22 audit (#257): a table defined differently from the game's schema the editor was built against is told to the person at open, in the state the
// page polls. It is a warning: the save stays writable.

func TestASaveWithTheGamesTableDefinitionsHasNoSchemaNote(t *testing.T) {
	o := &Ops{S: testsave.Player(t)}
	if n := o.S.SchemaNote(); n != "" {
		t.Fatalf("the real schema needs no note: %q", n)
	}
	st, err := o.State()
	if err != nil {
		t.Fatal(err)
	}
	if st.(map[string]any)["schemaNote"] != "" {
		t.Fatalf("state: %v", st)
	}
}

func TestATableDefinedDifferentlyIsWarnedAboutAndTheSaveStaysWritable(t *testing.T) {
	// what a game update that adds a column does to the stored text of its table, plus a table the editor does not know
	o := &Ops{S: testsave.PlayerWithSQL(t, `alter table items add column added_by_a_patch integer; create table from_a_newer_game(a integer);`)}
	n := o.S.SchemaNote()
	for _, want := range []string{"defined differently: items", "not known to this editor: from_a_newer_game", "keep editing"} {
		if !strings.Contains(n, want) {
			t.Errorf("the note says %q: %q", want, n)
		}
	}
	if why := o.S.WriteBlocked(); why != "" {
		t.Fatalf("a changed table definition must not make the save read-only: %q", why)
	}
	st, _ := o.State()
	if st.(map[string]any)["schemaNote"] != n {
		t.Fatalf("the state carries the note: %v", st)
	}
	if _, err := o.ExecSQL("update items set stack_size=101 where id=10"); err != nil {
		t.Fatalf("editing continues: %v", err)
	}
}

func TestAMissingTableIsNamedInTheNote(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithSQL(t, `pragma foreign_keys=off; drop table landsraad_decree_rotation;`)}
	if n := o.S.SchemaNote(); !strings.Contains(n, "missing: landsraad_decree_rotation") {
		t.Fatalf("%q", n)
	}
}
