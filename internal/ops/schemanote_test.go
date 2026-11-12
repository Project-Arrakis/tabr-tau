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

// #588: the game changed the layout of its save in an update of early November 2026 (markers.map_name_id, the map_names table, two Landsraad reward tables). The editor knows both
// layouts: a save of either has no note, and a table that is neither layout's is still named.
func TestASaveOfEitherLayoutHasNoSchemaNote(t *testing.T) {
	for _, l := range testsave.Layouts {
		t.Run(l.String(), func(t *testing.T) {
			o := &Ops{S: testsave.PlayerWithSQLIn(t, l, "")}
			if n := o.S.SchemaNote(); n != "" {
				t.Fatalf("the game's own schema needs no note: %q", n)
			}
			st, err := o.State()
			if err != nil {
				t.Fatal(err)
			}
			if st.(map[string]any)["schemaNote"] != "" {
				t.Fatalf("state: %v", st)
			}
		})
	}
}

func TestATableNoLayoutHasIsNamedInTheNoteOfEitherLayout(t *testing.T) {
	for _, l := range testsave.Layouts {
		t.Run(l.String(), func(t *testing.T) {
			o := &Ops{S: testsave.PlayerWithSQLIn(t, l, `alter table items add column added_by_a_patch integer; create table from_a_newer_game(a integer);`)}
			n := o.S.SchemaNote()
			for _, want := range []string{"defined differently: items;", "not known to this editor: from_a_newer_game)", "keep editing"} {
				if !strings.Contains(n, want) {
					t.Errorf("the note says %q: %q", want, n)
				}
			}
			// the note is about what differs from the layout the save has, not from the other one
			for _, not := range []string{"markers", "map_names", "landsraad_one_time", "missing"} {
				if strings.Contains(n, not) {
					t.Errorf("the note mentions %q, which is as the game's layout has it: %q", not, n)
				}
			}
			if why := o.S.WriteBlocked(); why != "" {
				t.Fatalf("a changed table definition must not make the save read-only: %q", why)
			}
		})
	}
}

// A save of the newer layout that lost one of the newer tables is a save with a missing table: it is named, and it is not read as the older layout (which would call it three
// tables too few... and map_names unknown).
func TestAMissingTableOfTheNewerLayoutIsNamed(t *testing.T) {
	o := &Ops{S: testsave.PlayerWithNewerSQL(t, `drop table landsraad_one_time_house_rewards;`)}
	n := o.S.SchemaNote()
	if !strings.Contains(n, "missing: landsraad_one_time_house_rewards") {
		t.Fatalf("%q", n)
	}
	if strings.Contains(n, "map_names") || strings.Contains(n, "defined differently") {
		t.Fatalf("only the missing table is named: %q", n)
	}
}
