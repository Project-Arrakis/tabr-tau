package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/kits"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// The care package routes (#417) keep kits in the settings file. They never touch the save: no edit, no backup, no queue, and the same answer while the game runs.

const starterKit = `{"name":"Starter","items":[{"item":"SpicedFuelCell","quantity":100},{"item":"Combat_Heavy_Unique_Reinforced_Helmet_06","quantity":1,"grade":3,"augments":["T6_Augment_Armor10"],"augmentGrade":4}],"xp":5000,"skillPoints":3}`

func kitServer(t *testing.T) *Server {
	t.Helper()
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	return s
}

func kitGet(t *testing.T, s *Server, route string) *httptest.ResponseRecorder {
	t.Helper()
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
	return do(t, s, "GET", "http://127.0.0.1"+route, loop, h, nil)
}

func kitNames(t *testing.T, m map[string]any) []string {
	t.Helper()
	var out []string
	for _, k := range m["kits"].([]any) {
		out = append(out, k.(map[string]any)["name"].(string))
	}
	return out
}

func untouched(t *testing.T, s *Server, before []byte) {
	t.Helper()
	after, _ := os.ReadFile(s.Save.Path)
	if s.Save.Dirty() || string(before) != string(after) || len(s.Save.Backups()) != 0 {
		t.Fatal("a kit route touched the save")
	}
}

func TestKitsAreSavedListedRenamedDuplicatedAndDeleted(t *testing.T) {
	s := kitServer(t)
	before, _ := os.ReadFile(s.Save.Path)
	w := kitGet(t, s, "/api/kits")
	if m := decodeBody(t, w); w.Code != 200 || len(m["kits"].([]any)) != 0 || m["problem"] != "" || m["limits"].(map[string]any)["items"] != float64(50) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	w = postEdit(t, s, "/api/kits/save", `{"kit":`+starterKit+`}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["saved"] != "Starter" || m["outcome"] != nil || strings.Join(kitNames(t, m), ",") != "Starter" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	k := m["kits"].([]any)[0].(map[string]any)
	if k["xp"] != float64(5000) || k["skillPoints"] != float64(3) || k["problem"] != "" || len(k["items"].([]any)) != 2 {
		t.Fatalf("%v", k)
	}
	// it is in the settings file, beside what was there
	got, err := settings.LoadStrict(s.SettingsPath)
	if err != nil || len(got.Kits) != 1 || got.Kits[0].Items[1].AugmentGrade != 4 {
		t.Fatalf("%v %+v", err, got)
	}
	// replaced and renamed
	w = postEdit(t, s, "/api/kits/save", `{"was":"starter","kit":{"name":"First kit","items":[{"item":"SpicedFuelCell","quantity":7}]}}`)
	if m = decodeBody(t, w); w.Code != 200 || strings.Join(kitNames(t, m), ",") != "First kit" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	w = postEdit(t, s, "/api/kits/duplicate", `{"name":"First kit"}`)
	if m = decodeBody(t, w); w.Code != 200 || m["saved"] != "First kit (copy)" || strings.Join(kitNames(t, m), ",") != "First kit,First kit (copy)" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	w = postEdit(t, s, "/api/kits/delete", `{"name":"first kit"}`)
	if m = decodeBody(t, w); w.Code != 200 || strings.Join(kitNames(t, m), ",") != "First kit (copy)" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	untouched(t, s, before)
}

func TestARefusedKitIsA422AndChangesNothing(t *testing.T) {
	s := kitServer(t)
	postEdit(t, s, "/api/kits/save", `{"kit":`+starterKit+`}`)
	file, _ := os.ReadFile(s.SettingsPath)
	for _, c := range []struct{ route, body, words string }{
		{"/api/kits/save", `{"kit":{"name":"X","items":[{"item":"NoSuchThing","quantity":1}]}}`, "catalog"},
		{"/api/kits/save", `{"kit":{"name":"X","items":[{"item":"SpicedFuelCell","quantity":0}]}}`, "quantity"},
		{"/api/kits/save", `{"kit":{"name":"X","items":[{"item":"SpicedFuelCell","quantity":1.5}]}}`, "kit:"},
		{"/api/kits/save", `{"kit":{"name":"X","xp":1,"path":"C:\\Windows"}}`, "unknown field"},
		{"/api/kits/save", `{"kit":{"name":"../X","xp":1}}`, "name"},
		{"/api/kits/save", `{"kit":{"name":"starter","xp":1}}`, "exists already"},
		{"/api/kits/save", `{"was":"Missing","kit":{"name":"X","xp":1}}`, "no kit called"},
		{"/api/kits/save", `{"kit":{"name":"X"}}`, "at least one"},
		{"/api/kits/save", `{"kit":"x"}`, "must be an object"},
		{"/api/kits/save", `{}`, "must be an object"},
		{"/api/kits/delete", `{"name":"Missing"}`, "no kit called"},
		{"/api/kits/duplicate", `{"name":"Missing"}`, "no kit called"},
		{"/api/kits/import", `{}`, "content of a kit file"},
		{"/api/kits/import", `{"text":5}`, "content of a kit file"},
	} {
		w := postEdit(t, s, c.route, c.body)
		m := decodeBody(t, w)
		if w.Code != http.StatusUnprocessableEntity || m["code"] != "bad_argument" || !strings.Contains(m["error"].(string), c.words) {
			t.Errorf("%s %s: %d %s", c.route, c.body, w.Code, w.Body.String())
		}
	}
	if after, _ := os.ReadFile(s.SettingsPath); string(after) != string(file) {
		t.Fatal("a refused request changed the settings file")
	}
}

func TestExportAndImportRoundTripAndAnImportNeverReplacesAKit(t *testing.T) {
	s := kitServer(t)
	postEdit(t, s, "/api/kits/save", `{"kit":`+starterKit+`}`)
	postEdit(t, s, "/api/kits/save", `{"kit":{"name":"Levels","xp":900}}`)
	w := kitGet(t, s, "/api/kits/export?name=Starter")
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var f struct {
		Format string
		Kits   []kits.Kit
	}
	if err := json.Unmarshal(w.Body.Bytes(), &f); err != nil || f.Format != "tabr-tau-kits" || len(f.Kits) != 1 || f.Kits[0].Name != "Starter" {
		t.Fatalf("%v %s", err, w.Body.String())
	}
	if all := decodeBody(t, kitGet(t, s, "/api/kits/export")); len(all["kits"].([]any)) != 2 {
		t.Fatalf("all kits are exported when none is named: %v", all)
	}
	if w := kitGet(t, s, "/api/kits/export?name="+url.QueryEscape("Nope")); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	// the same file imported again: the names are taken, so the kits come in under other names and the originals stay
	text, _ := json.Marshal(map[string]any{"text": w0(t, s)})
	w = postEdit(t, s, "/api/kits/import", string(text))
	m := decodeBody(t, w)
	if w.Code != 200 || m["imported"] != float64(2) || m["renamed"] != float64(2) || strings.Join(kitNames(t, m), ",") != "Starter,Levels,Starter (imported),Levels (imported)" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

// w0 is the file of every kit, as text.
func w0(t *testing.T, s *Server) string {
	t.Helper()
	cur, err := settings.LoadStrict(s.SettingsPath)
	if err != nil {
		t.Fatal(err)
	}
	b, err := kits.Export(cur.Kits, nil)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestABadKitFileChangesNothing(t *testing.T) {
	s := kitServer(t)
	postEdit(t, s, "/api/kits/save", `{"kit":`+starterKit+`}`)
	file, _ := os.ReadFile(s.SettingsPath)
	good := `{"format":"tabr-tau-kits","version":1,"kits":[{"name":"New","xp":1},`
	for name, text := range map[string]string{
		"not json":          "hello",
		"unknown item":      good + `{"name":"Bad","items":[{"item":"Nope","quantity":1}]}]}`,
		"a path":            `{"format":"tabr-tau-kits","version":1,"kits":[{"name":"New","xp":1}],"path":"C:\\x"}`,
		"another format":    `{"format":"x","version":1,"kits":[{"name":"New","xp":1}]}`,
		"oversized":         `{"format":"tabr-tau-kits","version":1,"kits":[{"name":"New","xp":1}]}` + strings.Repeat(" ", kits.MaxFileBytes),
		"a bad quantity":    good + `{"name":"Bad","items":[{"item":"SpicedFuelCell","quantity":-4}]}]}`,
		"too many in a kit": good + `{"name":"Bad","items":[` + strings.Repeat(`{"item":"SpicedFuelCell","quantity":1},`, 50) + `{"item":"SpicedFuelCell","quantity":1}]}]}`,
	} {
		body, _ := json.Marshal(map[string]any{"text": text})
		w := postEdit(t, s, "/api/kits/import", string(body))
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s", name, w.Code, w.Body.String())
		}
	}
	if after, _ := os.ReadFile(s.SettingsPath); string(after) != string(file) {
		t.Fatal("a refused file changed the settings file")
	}
}

func TestKitRoutesWorkWhileTheGameRunsAndWriteNothingToTheSave(t *testing.T) {
	restore := save.ActiveSessionForTests()
	defer restore()
	s := kitServer(t)
	before, _ := os.ReadFile(s.Save.Path)
	w := postEdit(t, s, "/api/kits/save", `{"kit":`+starterKit+`}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["outcome"] != nil {
		t.Fatalf("a kit is a setting, not a queued edit: %d %s", w.Code, w.Body.String())
	}
	if w = postEdit(t, s, "/api/kits/duplicate", `{"name":"Starter"}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	untouched(t, s, before)
	// with an edit already queued, a kit route still adds no outcome and does not try to write it
	if w := postEdit(t, s, "/api/player/solari", `{"amount":5}`); decodeBody(t, w)["outcome"] != "queued" {
		t.Fatalf("%s", w.Body.String())
	}
	if m := decodeBody(t, postEdit(t, s, "/api/kits/delete", `{"name":"Starter (copy)"}`)); m["outcome"] != nil || m["ok"] != true {
		t.Fatalf("%v", m)
	}
}

func TestKitsInADamagedSettingsFileAreReportedAndTheFileIsLeftAlone(t *testing.T) {
	s := kitServer(t)
	damaged := `{"autoRefillOnOpen": true, "kits": [{"name": "Starter"`
	os.WriteFile(s.SettingsPath, []byte(damaged), 0o600)
	if m := decodeBody(t, kitGet(t, s, "/api/kits")); !strings.Contains(m["problem"].(string), "damaged") {
		t.Fatalf("%v", m)
	}
	if w := postEdit(t, s, "/api/kits/save", `{"kit":`+starterKit+`}`); w.Code == 200 || !strings.Contains(w.Body.String(), "damaged") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if b, _ := os.ReadFile(s.SettingsPath); string(b) != damaged {
		t.Fatalf("the damaged file was replaced: %q", b)
	}
}

func TestAKitEditedByHandIntoSomethingInvalidIsListedWithItsProblem(t *testing.T) {
	s := kitServer(t)
	os.WriteFile(s.SettingsPath, []byte(`{"kits":[{"name":"Hand made","items":[{"item":"NoSuchThing","quantity":1}]},{"name":"Fine","xp":5}]}`), 0o600)
	m := decodeBody(t, kitGet(t, s, "/api/kits"))
	ks := m["kits"].([]any)
	if len(ks) != 2 || !strings.Contains(ks[0].(map[string]any)["problem"].(string), "catalog") || ks[1].(map[string]any)["problem"] != "" {
		t.Fatalf("%v", m)
	}
}
