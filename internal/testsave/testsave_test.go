package testsave

import (
	"os"
	"testing"
)

// The embedded DDL must be byte-identical to the evidence file, so the fixture can never drift from the
// documented real schema. Update both together (docs/evidence/single-player-schema.ddl.sql is the source).
func TestEmbeddedSchemaMatchesEvidenceFile(t *testing.T) {
	want, err := os.ReadFile("../../docs/evidence/single-player-schema.ddl.sql")
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != Schema() {
		t.Fatal("internal/save/schema.sql differs from docs/evidence/single-player-schema.ddl.sql; copy one over the other")
	}
}

func TestRealSchemaShape(t *testing.T) {
	names := TableNames(t)
	if len(names) != 94 {
		t.Fatalf("real schema has %d application tables, want 94 (the audited count)", len(names))
	}
	have := map[string]bool{}
	for _, n := range names {
		have[n] = true
	}
	for _, must := range []string{"items", "inventories", "actors", "player_state", "fgl_entities", "items_id_sequencer", "applied_patches"} {
		if !have[must] {
			t.Errorf("missing table %s", must)
		}
	}
}

func TestEveryTableAcceptsAGenericRow(t *testing.T) {
	s := Hostile(t) // fails the test itself if the real schema rejects a generic row
	for _, n := range TableNames(t) {
		r, err := s.One(`select count(*) c from "` + n + `"`)
		if err != nil || r["c"].(int64) != 1 {
			t.Errorf("table %s: want exactly 1 row, got %v (err %v)", n, r, err)
		}
	}
}

func TestPlayerFixtureIsConsistentAndIntact(t *testing.T) {
	s := Player(t)
	for _, q := range []string{`pragma integrity_check`} {
		r, _ := s.One(q)
		if r["integrity_check"] != "ok" {
			t.Fatalf("%s: %v", q, r)
		}
	}
	rows, err := s.Query(`pragma foreign_key_check`)
	if err != nil || len(rows) != 0 {
		t.Fatalf("foreign_key_check violations in the fixture: %v (err %v)", rows, err)
	}
	r, _ := s.One(`select count(*) c from items`)
	if r["c"].(int64) != 2 {
		t.Fatalf("want 2 items, got %v", r)
	}
}
