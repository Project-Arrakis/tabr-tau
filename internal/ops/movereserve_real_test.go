package ops

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// An acceptance run on a real save (#1191). It skips unless the variables name one. It copies the save, moves the Reserve into the two boxes with this editor, and checks that no table
// but items changed in its row counts, that only the place of the Reserve's rows changed, and that a second move has nothing to do. The save is only read; the copy is made in a folder
// of the person's choosing and nothing of it is kept in the repository.
//
//	TABR_MOVERES_SAVE   path of the save (it is only read)
//	TABR_MOVERES_BOXES  the two box names, separated by a pipe
//	TABR_MOVERES_OUT    folder for the copy (must not exist yet)
func TestMoveReserveOnARealSave(t *testing.T) {
	src, boxes, out := os.Getenv("TABR_MOVERES_SAVE"), os.Getenv("TABR_MOVERES_BOXES"), os.Getenv("TABR_MOVERES_OUT")
	if src == "" || boxes == "" || out == "" {
		t.Skip("no real save named (TABR_MOVERES_SAVE, TABR_MOVERES_BOXES, TABR_MOVERES_OUT)")
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatal("the output folder exists; name one that does not")
	}
	if err := os.MkdirAll(out, 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(src)
	if err != nil {
		t.Skipf("the save is not readable here: %v", err)
	}
	copyPath := filepath.Join(out, "game.db")
	g, err := os.Create(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(g, f); err != nil {
		t.Fatal(err)
	}
	f.Close()
	g.Close()
	_ = testsave.HostileText
	s, err := save.Open(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	o := &Ops{S: s}
	names := strings.Split(boxes, "|")

	counts := func() map[string]int64 {
		tables, err := s.Query(`select name from sqlite_master where type='table' and name not like 'sqlite_%' order by name`)
		if err != nil {
			t.Fatal(err)
		}
		c := map[string]int64{}
		for _, r := range tables {
			n, err := s.One(`select count(*) n from "` + r["name"].(string) + `"`)
			if err != nil {
				t.Fatal(err)
			}
			c[r["name"].(string)] = n["n"].(int64)
		}
		return c
	}
	before := counts()
	otherBefore := columnsOtherThanPlace(t, o)
	reserve, _ := s.One(`select count(*) n, coalesce(sum(stack_size),0) u from items where inventory_id=(select id from inventories where actor_id=(select player_pawn_id from player_state limit 1) and inventory_type=33)`)
	t.Logf("the Reserve before: %v", reserve)

	pv, err := o.PreviewMoveReserve(names, MoveReserveOptions{})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	p := pv.(map[string]any)
	for _, l := range p["summary"].([]string) {
		t.Log(l)
	}
	if fmt.Sprint(counts()) != fmt.Sprint(before) || len(s.Ops()) != 0 {
		t.Fatal("the preview changed the save")
	}
	r, err := o.MoveReserve(names, p["fingerprint"].(string), MoveReserveOptions{})
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	t.Logf("answer: %v", r.(map[string]any)["summary"])
	after := counts()
	for tbl, n := range before {
		if after[tbl] != n {
			t.Errorf("table %s: %d rows before, %d after", tbl, n, after[tbl])
		}
	}
	if columnsOtherThanPlace(t, o) != otherBefore {
		t.Fatal("a column other than the place of a row changed")
	}
	if n, _ := s.One(`select count(*) n from items where inventory_id=(select id from inventories where actor_id=(select player_pawn_id from player_state limit 1) and inventory_type=33)`); n["n"] != int64(0) {
		t.Fatalf("the Reserve is not empty: %v", n)
	}
	if _, err := s.Query(`pragma foreign_key_check`); err != nil {
		t.Fatal(err)
	}
	pv2, err := o.PreviewMoveReserve(names, MoveReserveOptions{})
	if err != nil || pv2.(map[string]any)["nothingToDo"] != true {
		t.Fatalf("a second move has nothing to do: %v %v", pv2, err)
	}
}
