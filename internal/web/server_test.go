package web

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/ops"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/settings"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	plain := filepath.Join(dir, "p.sqlite")
	db, err := sql.Open("sqlite", plain)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`create table things(id integer primary key, name text); insert into things values (1,'a'),(2,'b');`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	raw, _ := os.ReadFile(plain)
	blob, _ := save.Encode(raw)
	p := filepath.Join(dir, "game.db")
	os.WriteFile(p, blob, 0o644)
	s, err := save.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return New(s, config.Dir{Path: t.TempDir()})
}

// do sends one request through the real handler. remote is the peer address the server would see.
func do(t *testing.T, s *Server, method, url, remote string, hdr map[string]string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, url, rd)
	req.RemoteAddr = remote
	for k, v := range hdr {
		if k == "Host" {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

const loop = "127.0.0.1:50000"

func TestHostHeader(t *testing.T) {
	s := newTestServer(t)
	for _, c := range []struct {
		host string
		ok   bool
	}{
		{"127.0.0.1:8090", true}, {"localhost:8090", true}, {"[::1]:8090", true}, {"127.0.0.1", true}, {"localhost", true},
		{"evil.com", false}, {"127.0.0.1.evil.com", false}, {"localhost.evil.com", false}, {"evil.com:8090", false},
		{"", false}, {"0.0.0.0:8090", false}, {"192.168.1.5:8090", false},
	} {
		w := do(t, s, "GET", "http://x/", loop, map[string]string{"Host": c.host, "Cookie": "tabr_session=" + s.token}, nil)
		if (w.Code == 200) != c.ok {
			t.Errorf("Host %q: got %d, want ok=%v", c.host, w.Code, c.ok)
		}
	}
}

// NET-1: a LAN peer sending Host: localhost must not reach the UI or get the token.
func TestRemotePeerRefusedAndNeverSeesToken(t *testing.T) {
	s := newTestServer(t)
	for _, remote := range []string{"192.168.1.5:5555", "10.0.0.7:1", "[2001:db8::1]:5555", "203.0.113.9:80"} {
		w := do(t, s, "GET", "http://localhost/", remote, map[string]string{"Host": "localhost:8090"}, nil)
		if w.Code != http.StatusForbidden {
			t.Errorf("remote %s: got %d, want 403", remote, w.Code)
		}
		if strings.Contains(w.Body.String(), s.token) {
			t.Errorf("remote %s: response leaked the token", remote)
		}
	}
	for _, remote := range []string{"127.0.0.1:1234", "[::1]:1234"} {
		if w := do(t, s, "GET", "http://localhost/", remote, map[string]string{"Host": "localhost:8090", "Cookie": "tabr_session=" + s.token}, nil); w.Code != 200 {
			t.Errorf("loopback %s: got %d, want 200", remote, w.Code)
		}
	}
}

func TestTokenRequired(t *testing.T) {
	s := newTestServer(t)
	for _, tok := range []string{"", "wrong", s.token[:len(s.token)-1], s.token + "x", strings.ToUpper(s.token)} {
		h := map[string]string{"Host": "127.0.0.1:8090"}
		if tok != "" {
			h["Cookie"] = "tabr_session=" + tok
		}
		if w := do(t, s, "GET", "http://127.0.0.1/api/save/backups", loop, h, nil); w.Code != 403 {
			t.Errorf("session %q: got %d, want 403", tok, w.Code)
		}
	}
	if w := do(t, s, "GET", "http://127.0.0.1/api/save/backups", loop, map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}, nil); w.Code != 200 {
		t.Errorf("valid session: got %d", w.Code)
	}
}

func TestPostRequiresJSONContentType(t *testing.T) {
	s := newTestServer(t)
	base := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
	cases := []struct {
		ct   string
		want int
	}{
		{"application/json", 200}, {"application/json; charset=utf-8", 200}, {"APPLICATION/JSON", 200},
		{"text/plain", 415}, {"application/x-www-form-urlencoded", 415}, {"multipart/form-data; boundary=x", 415}, {"", 415},
	}
	for _, c := range cases {
		h := map[string]string{}
		for k, v := range base {
			h[k] = v
		}
		if c.ct != "" {
			h["Content-Type"] = c.ct
		}
		w := do(t, s, "POST", "http://127.0.0.1/api/save/discard", loop, h, []byte(`{}`))
		if w.Code != c.want {
			t.Errorf("Content-Type %q: got %d, want %d", c.ct, w.Code, c.want)
		}
	}
}

func TestOriginAndFetchSite(t *testing.T) {
	s := newTestServer(t)
	post := func(extra map[string]string) int {
		h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Content-Type": "application/json"}
		for k, v := range extra {
			h[k] = v
		}
		return do(t, s, "POST", "http://127.0.0.1/api/save/discard", loop, h, []byte(`{}`)).Code
	}
	for name, c := range map[string]struct {
		hdr  map[string]string
		want int
	}{
		"no origin (non-browser client)": {nil, 200},
		"same origin":                    {map[string]string{"Origin": "http://127.0.0.1:8090"}, 200},
		"same origin localhost host":     {map[string]string{"Origin": "http://127.0.0.1:8090", "Sec-Fetch-Site": "same-origin"}, 200},
		"cross origin":                   {map[string]string{"Origin": "http://evil.com"}, 403},
		"other port":                     {map[string]string{"Origin": "http://127.0.0.1:9999"}, 403},
		"null origin":                    {map[string]string{"Origin": "null"}, 403},
		"cross-site fetch metadata":      {map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		"same-site fetch metadata":       {map[string]string{"Sec-Fetch-Site": "same-site"}, 403},
	} {
		if got := post(c.hdr); got != c.want {
			t.Errorf("%s: got %d, want %d", name, got, c.want)
		}
	}
}

func TestSecurityHeadersOnEveryResponseClass(t *testing.T) {
	s := newTestServer(t)
	tok := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
	responses := map[string]*httptest.ResponseRecorder{
		"index":     do(t, s, "GET", "http://127.0.0.1/", loop, map[string]string{"Host": "127.0.0.1:8090"}, nil),
		"api ok":    do(t, s, "GET", "http://127.0.0.1/api/save/backups", loop, tok, nil),
		"api 403":   do(t, s, "GET", "http://127.0.0.1/api/save/backups", loop, map[string]string{"Host": "127.0.0.1:8090"}, nil),
		"bad host":  do(t, s, "GET", "http://127.0.0.1/", loop, map[string]string{"Host": "evil.com"}, nil),
		"remote":    do(t, s, "GET", "http://127.0.0.1/", "192.168.1.5:1", map[string]string{"Host": "127.0.0.1:8090"}, nil),
		"not found": do(t, s, "GET", "http://127.0.0.1/nope", loop, map[string]string{"Host": "127.0.0.1:8090"}, nil),
	}
	want := map[string]string{
		"X-Frame-Options":              "DENY",
		"X-Content-Type-Options":       "nosniff",
		"Referrer-Policy":              "no-referrer",
		"Cross-Origin-Resource-Policy": "same-origin",
		"Cache-Control":                "no-store",
	}
	for name, w := range responses {
		for k, v := range want {
			if got := w.Header().Get(k); got != v {
				t.Errorf("%s: header %s = %q, want %q", name, k, got, v)
			}
		}
		if csp := w.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
			t.Errorf("%s: CSP %q must contain frame-ancestors 'none'", name, csp)
		}
	}
}

func TestNoCORSAndPreflightRefused(t *testing.T) {
	s := newTestServer(t)
	w := do(t, s, "OPTIONS", "http://127.0.0.1/api/db/exec", loop, map[string]string{
		"Host": "127.0.0.1:8090", "Origin": "http://evil.com", "Access-Control-Request-Method": "POST", "Access-Control-Request-Headers": "x-tabr-token,content-type"}, nil)
	if w.Code == 200 || w.Code == 204 {
		t.Errorf("preflight got %d; must not succeed", w.Code)
	}
	for k := range w.Header() {
		if strings.HasPrefix(k, "Access-Control-") {
			t.Errorf("CORS header %s must never be sent", k)
		}
	}
}

func TestMethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
	if w := do(t, s, "GET", "http://127.0.0.1/api/db/exec", loop, h, nil); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET on a mutating route: got %d, want 405", w.Code)
	}
}

func TestOversizeBodyRejected(t *testing.T) {
	s := newTestServer(t)
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Content-Type": "application/json"}
	big := []byte(`{"sql":"` + strings.Repeat("a", 5<<20) + `"}`)
	if w := do(t, s, "POST", "http://127.0.0.1/api/db/sql", loop, h, big); w.Code == 200 {
		t.Errorf("5 MiB body accepted")
	}
}

func TestSafeFilename(t *testing.T) {
	for in, want := range map[string]string{
		"things": "things", "a.b-c_d": "a.b-c_d", `a"b`: "ab", "a\r\nX-Evil: 1": "aX-Evil1", "../../etc/passwd": "....etcpasswd", "": "export", "é🙂": "export",
	} {
		if got := safeFilename(in); got != want {
			t.Errorf("safeFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExportDispositionIsSafe(t *testing.T) {
	s := newTestServer(t)
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
	w := do(t, s, "GET", "http://127.0.0.1/api/db/export?name=things&format=csv", loop, h, nil)
	if w.Code != 200 {
		t.Fatalf("export got %d: %s", w.Code, w.Body.String())
	}
	if d := w.Header().Get("Content-Disposition"); d != `attachment; filename="things.csv"` {
		t.Errorf("Content-Disposition = %q", d)
	}
}

func TestCheckListenAddrPolicy(t *testing.T) {
	ok := []string{"127.0.0.1:8090", "localhost:8090", "[::1]:8090", "127.0.0.1:0", "127.5.5.5:1"}
	bad := []string{":8090", "0.0.0.0:8090", "[::]:8090", "192.168.1.5:8090", "10.0.0.1:1", "example.com:80", "8090", ""}
	for _, a := range ok {
		if err := CheckListenAddr(a); err != nil {
			t.Errorf("%q should be allowed: %v", a, err)
		}
	}
	for _, a := range bad {
		if err := CheckListenAddr(a); err == nil {
			t.Errorf("%q must be refused: the editor only listens on this computer", a)
		}
	}
}

func TestListenFallsBackOnlyForDefaultAddr(t *testing.T) {
	hold, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer hold.Close()
	busy := hold.Addr().String()
	if ln, err := Listen(busy, false); err != nil {
		t.Errorf("default address busy: want fallback, got %v", err)
	} else {
		if ln.Addr().String() == busy {
			t.Errorf("fallback reused the busy address")
		}
		if !ln.Addr().(*net.TCPAddr).IP.IsLoopback() {
			t.Errorf("fallback must stay on loopback, got %s", ln.Addr())
		}
		ln.Close()
	}
	if ln, err := Listen(busy, true); err == nil {
		ln.Close()
		t.Errorf("explicit address busy: must fail, not silently move")
	}
}

func TestHTTPServerHasTimeouts(t *testing.T) {
	srv := NewHTTPServer(http.NewServeMux())
	if srv.ReadHeaderTimeout == 0 || srv.ReadTimeout == 0 || srv.WriteTimeout == 0 || srv.IdleTimeout == 0 || srv.MaxHeaderBytes == 0 {
		t.Errorf("missing limits: %+v", srv)
	}
}

func TestCatalogRouteListsItems(t *testing.T) {
	s := New(testsave.Empty(t), config.Dir{Path: t.TempDir()})
	cookie := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
	w := do(t, s, "GET", "http://127.0.0.1/api/catalog/items", loop, cookie, nil)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var items []struct{ ID, Name, Category string }
	if err := json.Unmarshal(w.Body.Bytes(), &items); err != nil || len(items) < 2000 {
		t.Fatalf("%v, %d items", err, len(items))
	}
}

const cisternSQL = `
insert into actors(id,class,map) values (901,'c','HaggaBasin');
insert into fgl_entities(entity_id,components) values (-901, jsonb('{"FWaterStorageComponent":[0,{"m_WaterStored":0}]}'));
insert into actor_fgl_entities(actor_id,entity_id,slot_name) values (901,-901,'a');
insert into placeables(id,health,building_type) values (901,1,'WaterCistern_Placeable');`

func TestSettingsEndpointsAndOpenTimeRefill(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, cisternSQL), config.Dir{Path: t.TempDir()})
	s.SettingsPath = filepath.Join(t.TempDir(), "tabr-tau", "settings.json")
	cookie := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
	call := func(method, route, body string) *httptest.ResponseRecorder {
		hdr := map[string]string{"Host": cookie["Host"], "Cookie": cookie["Cookie"]}
		var b []byte
		if body != "" {
			hdr["Content-Type"] = "application/json"
			b = []byte(body)
		}
		return do(t, s, method, "http://127.0.0.1"+route, loop, hdr, b)
	}

	// default: off, nothing happens on open
	if w := call("GET", "/api/settings", ""); !strings.Contains(w.Body.String(), `"autoRefillOnOpen":false`) {
		t.Fatalf("%s", w.Body.String())
	}
	s.RunStartupTasks()
	if len(s.Save.Pending()) != 0 || s.takeStartupNote() != "" {
		t.Fatal("with the setting off the editor must queue nothing")
	}

	// a non-boolean is refused and changes nothing
	if w := call("POST", "/api/settings", `{"autoRefillOnOpen":"yes"}`); w.Code == 200 {
		t.Fatal("a non-boolean must be refused")
	}
	if w := call("POST", "/api/settings", `{"autoRefillOnOpen":true}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if !settings.Load(s.SettingsPath).AutoRefillOnOpen {
		t.Fatal("the setting must be written to disk")
	}

	// on: opening refills and saves straight away, with a backup, and tells the UI once
	s.RunStartupTasks()
	if len(s.Save.Pending()) != 0 {
		t.Fatalf("the refill must be written, not left pending: %d pending", len(s.Save.Pending()))
	}
	w := call("GET", "/api/startup", "")
	if !strings.Contains(w.Body.String(), "saved 1 Water Cistern") {
		t.Fatalf("%s", w.Body.String())
	}
	if w := call("GET", "/api/startup", ""); !strings.Contains(w.Body.String(), `"note":""`) {
		t.Fatalf("the note must be shown only once: %s", w.Body.String())
	}
	if w := call("GET", "/api/settings", ""); !strings.Contains(w.Body.String(), "Automatic refill saved 1 Water Cistern") || !strings.Contains(w.Body.String(), "backed up as") {
		t.Fatalf("the last automatic save must stay visible with its backup name: %s", w.Body.String())
	}
	if backups, _ := filepath.Glob(filepath.Join(filepath.Dir(s.Save.Path), "tabr-tau-backups", "*.db")); len(backups) != 1 {
		t.Fatalf("expected one backup of the previous file, found %v", backups)
	}
	reopened, err := save.Open(s.Save.Path)
	if err != nil {
		t.Fatal(err)
	}
	row, _ := reopened.One(`select json_extract(components,'$.FWaterStorageComponent[1].m_WaterStored') w from fgl_entities where entity_id=-901`)
	reopened.Close()
	if row["w"].(int64) != 5000 {
		t.Fatalf("the cistern in the written file is %v, want 5000", row["w"])
	}

	// opening again with everything full does nothing and writes nothing
	s.RunStartupTasks()
	if len(s.Save.Pending()) != 0 || !strings.Contains(s.takeStartupNote(), "already full") {
		t.Fatal("a second open must find nothing to do")
	}
}

// If the write is refused (here: the file changed on disk after it was loaded), the edits stay pending and the note says so.
func TestOpenTimeRefillThatCannotSaveLeavesTheEditsPending(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, cisternSQL), config.Dir{Path: t.TempDir()})
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	if err := settings.Save(s.SettingsPath, settings.Settings{AutoRefillOnOpen: true}); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(s.Save.Path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte{0})
	f.Close()
	s.RunStartupTasks()
	note, warn := s.takeStartup()
	if !strings.Contains(note, "could not save it") || !strings.Contains(note, "waiting under Review & save") || !warn || len(s.Save.Pending()) != 1 {
		t.Fatalf("note %q, warn %v, pending %d", note, warn, len(s.Save.Pending()))
	}
}

// The status poll tells the page when the game has rewritten the file since the editor opened it.
func TestStateReportsAFileChangedOnDisk(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, cisternSQL), config.Dir{Path: t.TempDir()})
	state := func() map[string]any {
		w := do(t, s, "GET", "http://127.0.0.1/api/save/state", loop, map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}, nil)
		var m map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
			t.Fatalf("%v: %s", err, w.Body.String())
		}
		return m
	}
	if state()["changedOnDisk"] != false {
		t.Fatalf("a freshly opened save has not changed: %v", state())
	}
	f, err := os.OpenFile(s.Save.Path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.Write([]byte{0})
	f.Close()
	if state()["changedOnDisk"] != true {
		t.Fatalf("a rewritten file must be reported: %v", state())
	}
}

// The open-time refill is the one write that skips the review pane, so its gates are pinned (QA-1 of the 2026-10-15 audit): during a
// single-player session it writes nothing and leaves nothing pending, and it never folds someone else's unsaved edits into a save.
func TestOpenTimeRefillWritesNothingDuringASinglePlayerSession(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, cisternSQL), config.Dir{Path: t.TempDir()})
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	if err := settings.Save(s.SettingsPath, settings.Settings{AutoRefillOnOpen: true}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(s.Save.Path)
	defer save.ActiveSessionForTests()()
	s.RunStartupTasks()
	note, warn := s.takeStartup()
	if !strings.Contains(note, "skipped") || !strings.Contains(note, "single-player") || !warn {
		t.Fatalf("note %q, warn %v", note, warn)
	}
	after, _ := os.ReadFile(s.Save.Path)
	if !bytes.Equal(before, after) {
		t.Fatal("the save file changed during a single-player session")
	}
	if len(s.Save.Pending()) != 0 || s.Save.Dirty() {
		t.Fatalf("nothing may be prepared either: %d pending", len(s.Save.Pending()))
	}
	if backups, _ := filepath.Glob(filepath.Join(filepath.Dir(s.Save.Path), "tabr-tau-backups", "*.db")); len(backups) != 0 {
		t.Fatalf("a refused refill made a backup: %v", backups)
	}
}

func TestOpenTimeRefillLeavesSomeoneElsesPendingEditsAlone(t *testing.T) {
	s := New(testsave.PlayerWithSQL(t, cisternSQL), config.Dir{Path: t.TempDir()})
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	if err := settings.Save(s.SettingsPath, settings.Settings{AutoRefillOnOpen: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Ops.AddSolari(ops.Args{"amount": 10.0}); err != nil { // an unreviewed edit already waiting
		t.Fatal(err)
	}
	pending := len(s.Save.Pending())
	if pending == 0 {
		t.Fatal("the setup edit did not queue")
	}
	before, _ := os.ReadFile(s.Save.Path)
	s.RunStartupTasks()
	note, warn := s.takeStartup()
	if !strings.Contains(note, "unsaved edits") || !warn {
		t.Fatalf("note %q, warn %v", note, warn)
	}
	after, _ := os.ReadFile(s.Save.Path)
	if !bytes.Equal(before, after) || len(s.Save.Pending()) != pending {
		t.Fatalf("the refill must not write or add to the edits already waiting (pending %d, was %d)", len(s.Save.Pending()), pending)
	}
}

const twoVehiclesSQL = `
insert into actors(id,class,map) values (211,'/Game/BP_Sandbike_CHOAM.BP_Sandbike_CHOAM_C','HaggaBasin'),(212,'/Game/BP_Buggy_CHOAM.BP_Buggy_CHOAM_C','HaggaBasin');
insert into vehicles(id) values (211),(212);
insert into permission_actor(actor_id,actor_name,actor_type,access_level,is_child) values (211,'a',2,3,0),(212,'b',2,3,0);
insert into permission_actor_rank(permission_actor_id,player_id,rank) values (211,1,1),(212,1,1);
insert into vehicle_modules(id,vehicle_id,template_id,stats) values
  (11,211,'Hull_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":100.0,"DecayedMaxDurability":250.0}]}')),
  (12,212,'Hull_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":100.0,"DecayedMaxDurability":250.0}]}')),
  (13,212,'Gen_0',jsonb('{"FVehicleModuleDurabilityStats":[[],{"CurrentDurability":240.0,"DecayedMaxDurability":250.0}]}'));`

// The repair route must pass vehicle_id and threshold on (QA-2 of the 2026-10-15 audit: ignoring either stayed green) and refuse bad ones.
func TestRepairRouteReadsTheVehicleAndTheThreshold(t *testing.T) {
	newOne := func() (*Server, func(body string) *httptest.ResponseRecorder, func(id int) float64) {
		s := New(testsave.PlayerWithSQL(t, twoVehiclesSQL), config.Dir{Path: t.TempDir()})
		call := func(body string) *httptest.ResponseRecorder {
			return do(t, s, "POST", "http://127.0.0.1/api/vehicles/repair", loop,
				map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Content-Type": "application/json"}, []byte(body))
		}
		cur := func(id int) float64 {
			row, err := s.Save.One(`select json_extract(stats,'$.FVehicleModuleDurabilityStats[1].CurrentDurability') c from vehicle_modules where id=?`, id)
			if err != nil {
				t.Fatal(err)
			}
			return row["c"].(float64)
		}
		return s, call, cur
	}
	_, call, cur := newOne()
	if w := call(`{"vehicle_id":211}`); w.Code != 200 || cur(11) != 250 || cur(12) != 100 || cur(13) != 240 {
		t.Fatalf("vehicle_id must pick one vehicle: %d %s; %v %v %v", w.Code, w.Body.String(), cur(11), cur(12), cur(13))
	}
	_, call, cur = newOne()
	if w := call(`{"threshold":50}`); w.Code != 200 || cur(11) != 250 || cur(12) != 250 || cur(13) != 240 {
		t.Fatalf("threshold 50 must leave the 96 %% module: %d; %v %v %v", w.Code, cur(11), cur(12), cur(13))
	}
	_, call, cur = newOne()
	if w := call(`{}`); w.Code != 200 || cur(13) != 250 {
		t.Fatalf("no arguments repairs everything: %d; %v", w.Code, cur(13))
	}
	_, call, cur = newOne()
	for _, bad := range []string{`{"threshold":0}`, `{"threshold":101}`, `{"threshold":"x"}`, `{"vehicle_id":"x"}`, `{"vehicle_id":999}`} {
		if w := call(bad); w.Code == 200 {
			t.Errorf("%s must be refused", bad)
		}
	}
	if cur(11) != 100 || cur(12) != 100 || cur(13) != 240 {
		t.Fatal("refused requests must change nothing")
	}
}

// The automatic refresh decides from a status read that can be seconds old, so it asks for the discard only if nothing is unsaved by the time the server gets it.
func TestDiscardIfCleanKeepsAnEditMadeSinceTheStatusWasRead(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Content-Type": "application/json"}
	testsave.Exec(t, s.Save, `update actors set location_x = location_x + 1 where id = 2`)
	if !s.Save.Dirty() {
		t.Fatal("the edit should leave the save dirty")
	}
	w := do(t, s, "POST", "http://127.0.0.1/api/save/discard", loop, h, []byte(`{"ifClean":true}`))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"kept":true`) || !s.Save.Dirty() {
		t.Fatalf("an unsaved edit must survive an ifClean discard: %d %s dirty=%v", w.Code, w.Body.String(), s.Save.Dirty())
	}
	// a plain discard (the Discard button, Reload now) still throws the edit away
	w = do(t, s, "POST", "http://127.0.0.1/api/save/discard", loop, h, []byte(`{}`))
	if w.Code != 200 || s.Save.Dirty() {
		t.Fatalf("a plain discard drops the edits: %d dirty=%v", w.Code, s.Save.Dirty())
	}
	// and with nothing unsaved the ifClean discard reads the file again
	w = do(t, s, "POST", "http://127.0.0.1/api/save/discard", loop, h, []byte(`{"ifClean":true}`))
	if w.Code != 200 || strings.Contains(w.Body.String(), `"kept"`) {
		t.Fatalf("a clean save is reloaded: %d %s", w.Code, w.Body.String())
	}
}
