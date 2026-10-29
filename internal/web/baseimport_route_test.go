package web

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/basefile"
	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The import routes (#475, phase 2): a preview that is the import rolled back, and the import as an ordinary edit.

// importServer is a save with one character and a data folder for the queue, the target of an import.
func importServer(t *testing.T, world string) *Server {
	t.Helper()
	s := New(testsave.PlayerWithSQL(t, world), config.Dir{Path: t.TempDir()})
	s.KeepPendingEdits(t.TempDir())
	return s
}

// exportedBase is the base file of backupWorld, made by the export route.
func exportedBase(t *testing.T) []byte {
	t.Helper()
	// the positions a real pick-up has (backupWorld leaves them empty, which an import refuses)
	pos := `update actors set location_x=1,location_y=2,location_z=3,rotation_x=0,rotation_y=0,rotation_z=0,rotation_w=1;
update building_instances set location_x=1,location_y=2,location_z=3,rotation_x=0,rotation_y=0,rotation_z=0,rotation_w=1;`
	w := kitGet(t, backupServer(t, backupWorld+pos), "/api/bases/backups/export?backup=1")
	if w.Code != 200 {
		t.Fatalf("export: %d %s", w.Code, w.Body.String())
	}
	return w.Body.Bytes()
}

func importBody(t *testing.T, text []byte, hash string) string {
	t.Helper()
	m := map[string]any{"text": string(text)}
	if hash != "" {
		m["fileHash"] = hash
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func backupCount(t *testing.T, path string) int64 {
	t.Helper()
	chk, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	r, err := chk.One(`select count(*) n from base_backups`)
	if err != nil || r == nil {
		t.Fatal(err)
	}
	return r["n"].(int64)
}

func TestPreviewChangesNothingAndTheImportIsWrittenOnceAfterABackup(t *testing.T) {
	s := importServer(t, "")
	data := exportedBase(t)
	before, _ := os.ReadFile(s.Save.Path)

	w := postEdit(t, s, "/api/bases/backups/import-preview", importBody(t, data, ""))
	pv := decodeBody(t, w)
	if w.Code != 200 || pv["dryRun"] != true || pv["outcome"] != nil || pv["fileHash"] != basefile.Hash(data) {
		t.Fatalf("preview: %d %s", w.Code, w.Body.String())
	}
	sum := pv["summary"].(map[string]any)
	if sum["actors"] != float64(2) || sum["pieces"] != float64(1) || sum["placeables"] != float64(1) {
		t.Fatalf("summary: %v", sum)
	}
	if e := pv["ids"].(map[string]any)["entities"].(map[string]any); e["first"] == nil || e["count"] != float64(1) {
		t.Fatalf("entity ids are text: %v", e)
	}
	untouched(t, s, before)

	w = postEdit(t, s, "/api/bases/backups/import", importBody(t, data, pv["fileHash"].(string)))
	m := decodeBody(t, w)
	if w.Code != 200 || m["dryRun"] != false || m["outcome"] != "written" {
		t.Fatalf("import: %d %s", w.Code, w.Body.String())
	}
	if n := backupCount(t, s.Save.Path); n != 1 {
		t.Fatalf("backups in the file: %d", n)
	}
	// the game's file did not hold the base before, and a backup copy of it was made first
	if len(s.Save.Backups()) != 1 {
		t.Fatalf("one backup copy of the save: %d", len(s.Save.Backups()))
	}
	// and it is listed
	l := kitGet(t, s, "/api/bases/backups")
	var list []map[string]any
	if err := json.Unmarshal(l.Body.Bytes(), &list); err != nil || len(list) != 1 || list[0]["pieces"] != float64(1) || list[0]["picked_up"] != float64(2) {
		t.Fatalf("list: %s", l.Body.String())
	}
	// the same file again is a second backup (not idempotent, said in the page)
	w = postEdit(t, s, "/api/bases/backups/import", importBody(t, data, basefile.Hash(data)))
	if w.Code != 200 || backupCount(t, s.Save.Path) != 2 {
		t.Fatalf("second import: %d %s", w.Code, w.Body.String())
	}
}

func TestImportWhileAGameProcessExistsIsQueuedAsOneEditAndWritesNothing(t *testing.T) {
	s := importServer(t, "")
	data := exportedBase(t)
	before, _ := os.ReadFile(s.Save.Path)
	restore := save.ActiveSessionForTests()
	defer restore()
	w := postEdit(t, s, "/api/bases/backups/import", importBody(t, data, basefile.Hash(data)))
	m := decodeBody(t, w)
	if w.Code != 200 || m["outcome"] != "queued" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	after, _ := os.ReadFile(s.Save.Path)
	if string(before) != string(after) || len(s.Save.Backups()) != 0 {
		t.Fatal("the save was written while the game was running")
	}
	if p := s.Save.Pending(); len(p) != 1 || !strings.HasPrefix(p[0], `Import base backup "My base"`) {
		t.Fatalf("queued as one edit: %v", p)
	}
	// a preview while the game runs is still only a preview
	pw := postEdit(t, s, "/api/bases/backups/import-preview", importBody(t, data, ""))
	if pw.Code != 200 || decodeBody(t, pw)["outcome"] != nil || len(s.Save.Pending()) != 1 {
		t.Fatalf("preview while queued: %d %s", pw.Code, pw.Body.String())
	}
}

func TestImportRefusesWhatWasNotPreviewedAndWhatIsNotABaseFile(t *testing.T) {
	s := importServer(t, "")
	data := exportedBase(t)
	before, _ := os.ReadFile(s.Save.Path)
	other := append(append([]byte{}, data...), ' ') // a different file: the hash differs
	for _, c := range []struct {
		route, body string
		code        int
		want        string
	}{
		{"/api/bases/backups/import", importBody(t, data, ""), 422, "fileHash is missing"},
		{"/api/bases/backups/import", importBody(t, other, basefile.Hash(data)), 409, "not the file that was previewed"},
		{"/api/bases/backups/import", `{"text":5,"fileHash":"x"}`, 422, "content of a base file"},
		{"/api/bases/backups/import", `{}`, 422, "content of a base file"},
		{"/api/bases/backups/import-preview", `{}`, 422, "content of a base file"},
		{"/api/bases/backups/import-preview", `{"text":"hello"}`, 422, "cannot be imported"},
		{"/api/bases/backups/import-preview", `{"text":"{\"format\":\"tabr-tau-kits\",\"version\":1}"}`, 422, "cannot be imported"},
		{"/api/bases/backups/import", importBody(t, []byte("hello"), basefile.Hash([]byte("hello"))), 422, "Nothing was written"},
		{"/api/bases/backups/import", `not json`, 400, "invalid JSON"},
	} {
		w := postEdit(t, s, c.route, c.body)
		if m := decodeBody(t, w); w.Code != c.code || !strings.Contains(m["error"].(string), c.want) {
			t.Errorf("%s %.40s: %d %s", c.route, c.body, w.Code, w.Body.String())
		}
	}
	untouched(t, s, before)
}

func TestImportRoutesTakeAFileLargerThanOtherRoutesButNotWithoutLimit(t *testing.T) {
	s := importServer(t, "")
	// a text of 5 MiB passes the body limit (other routes stop at 4 MiB) and is then refused as a file, not as a request
	big := strings.Repeat("a", 5<<20)
	w := postEdit(t, s, "/api/bases/backups/import-preview", `{"text":"`+big+`"}`)
	if m := decodeBody(t, w); w.Code != 422 || !strings.Contains(m["error"].(string), "cannot be imported") {
		t.Fatalf("5 MiB: %d %.200s", w.Code, w.Body.String())
	}
	// a text over the file limit is refused for its size
	w = postEdit(t, s, "/api/bases/backups/import-preview", `{"text":"`+strings.Repeat("a", basefile.MaxFileBytes+1)+`"}`)
	if m := decodeBody(t, w); w.Code != 422 || !strings.Contains(m["error"].(string), "larger than 16 MiB") {
		t.Fatalf("over the file limit: %d %.200s", w.Code, w.Body.String())
	}
	// and a body over the request limit is not read
	w = postEdit(t, s, "/api/bases/backups/import-preview", `{"text":"`+strings.Repeat("a", baseImportBodyMax)+`"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("over the request limit: %d %.200s", w.Code, w.Body.String())
	}
	// other routes keep their 4 MiB
	w = postEdit(t, s, "/api/kits/import", `{"text":"`+big+`"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("kit import: %d", w.Code)
	}
}

func TestImportRoutesNeedTheSessionAndJSONLikeEveryOtherWriteRoute(t *testing.T) {
	s := importServer(t, "")
	for _, route := range []string{"/api/bases/backups/import", "/api/bases/backups/import-preview"} {
		if w := do(t, s, "POST", "http://127.0.0.1"+route, loop, map[string]string{"Host": "127.0.0.1:8090", "Content-Type": "application/json"}, []byte(`{}`)); w.Code != 403 {
			t.Errorf("%s without the session: %d", route, w.Code)
		}
		h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Content-Type": "text/plain"}
		if w := do(t, s, "POST", "http://127.0.0.1"+route, loop, h, []byte(`{}`)); w.Code != http.StatusUnsupportedMediaType {
			t.Errorf("%s as text/plain: %d", route, w.Code)
		}
		h = map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Content-Type": "application/json", "Origin": "http://evil.example"}
		if w := do(t, s, "POST", "http://127.0.0.1"+route, loop, h, []byte(`{}`)); w.Code != 403 {
			t.Errorf("%s from another origin: %d", route, w.Code)
		}
		if w := do(t, s, "GET", "http://127.0.0.1"+route, loop, map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}, nil); w.Code == 200 {
			t.Errorf("%s answers a GET", route)
		}
	}
}

// The hostile save of the UI fixtures (every text column holds markup and long text) takes an import and a preview without a panic, and the answer carries the text as data.
func TestImportIntoTheHostileSaveOfTheFixtures(t *testing.T) {
	sv := testsave.Hostile(t)
	addHostileStorage(t, sv)
	addHostileCharacter(t, sv)
	s := New(sv, config.Dir{Path: t.TempDir()})
	data := exportedBase(t)
	for _, route := range []string{"/api/bases/backups/import-preview", "/api/bases/backups/import"} {
		body := importBody(t, data, basefile.Hash(data))
		w := postEdit(t, s, route, body)
		var m map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
			t.Fatalf("%s: not JSON (%d): %.200s", route, w.Code, w.Body.String())
		}
		if w.Code == http.StatusInternalServerError {
			t.Fatalf("%s: %d %s", route, w.Code, w.Body.String())
		}
	}
	// the list of backups of the hostile save still answers
	if w := kitGet(t, s, "/api/bases/backups"); w.Code != 200 {
		t.Fatalf("list after: %d %.200s", w.Code, w.Body.String())
	}
}
