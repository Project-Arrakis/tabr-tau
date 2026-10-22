package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The editor's settings live in the user's config folder; these tests must neither read nor write the developer's.
func TestMain(m *testing.M) {
	_, cleanup := testsave.IsolateUserConfig()
	code := m.Run()
	cleanup()
	os.Exit(code)
}

const cisternSQL = `
insert into actors(id,class,map) values (901,'c','HaggaBasin');
insert into fgl_entities(entity_id,components) values (-901, jsonb('{"FWaterStorageComponent":[0,{"m_WaterStored":0}]}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (901,-901,'a');
insert into placeables(id,health,building_type) values (901,1,'WaterCistern_Placeable');`

// QA-8: starting the editor runs the start-up tasks the settings ask for. With the automatic refill on, the empty cistern is
// full in the written file when newWebServer returns.
func TestNewWebServerRunsTheStartupTasks(t *testing.T) {
	if p := settings.Path(); p == "" {
		t.Fatal("no settings path in the isolated config folder")
	}
	if err := settings.Save(settings.Path(), settings.Settings{AutoRefillOnOpen: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(settings.Path()) })
	sv := testsave.PlayerWithSQL(t, cisternSQL)
	ws := newWebServer(sv, t.TempDir(), t.TempDir())
	if ws == nil || ws.Handler() == nil {
		t.Fatal("no server")
	}
	if len(sv.Pending()) != 0 {
		t.Fatalf("the refill must be written, not left pending: %v", sv.Pending())
	}
	if backups, _ := filepath.Glob(filepath.Join(filepath.Dir(sv.Path), "tabr-tau-backups", "*.db")); len(backups) != 1 {
		t.Fatalf("expected one backup of the previous file, found %v", backups)
	}
	reopened, err := save.Open(sv.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	row, _ := reopened.One(`select json_extract(components,'$.FWaterStorageComponent[1].m_WaterStored') w from fgl_entities where entity_id=-901`)
	if w, _ := row["w"].(int64); w != 5000 {
		t.Fatalf("the cistern in the written file is %v, want 5000", row["w"])
	}
}

// main itself cannot run in a test (it opens windows and listens), so check by its syntax that it builds the server through
// newWebServer, which runs the start-up tasks, and does not call web.New directly.
func TestMainBuildsTheServerThroughNewWebServer(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var viaHelper, direct bool
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "main" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			switch fun := call.Fun.(type) {
			case *ast.Ident:
				if fun.Name == "newWebServer" {
					viaHelper = true
				}
			case *ast.SelectorExpr:
				if x, ok := fun.X.(*ast.Ident); ok && x.Name == "web" && fun.Sel.Name == "New" {
					direct = true
				}
			}
			return true
		})
	}
	if !viaHelper || direct {
		t.Fatalf("main must build the server with newWebServer (found %v) and not with web.New (found %v)", viaHelper, direct)
	}
}
