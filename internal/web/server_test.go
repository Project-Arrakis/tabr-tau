package web

import (
	"bytes"
	"database/sql"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
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
		w := do(t, s, "GET", "http://x/", loop, map[string]string{"Host": c.host}, nil)
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
		if w := do(t, s, "GET", "http://localhost/", remote, map[string]string{"Host": "localhost:8090"}, nil); w.Code != 200 {
			t.Errorf("loopback %s: got %d, want 200", remote, w.Code)
		}
	}
}

func TestAllowRemoteStillNeedsToken(t *testing.T) {
	s := newTestServer(t)
	s.AllowRemote = true
	if w := do(t, s, "GET", "http://localhost/api/save/backups", "192.168.1.5:5555", map[string]string{"Host": "localhost:8090"}, nil); w.Code != 403 {
		t.Errorf("no token: got %d", w.Code)
	}
	if w := do(t, s, "GET", "http://localhost/api/save/backups", "192.168.1.5:5555", map[string]string{"Host": "localhost:8090", "X-Tabr-Token": s.token}, nil); w.Code != 200 {
		t.Errorf("with token and AllowRemote: got %d", w.Code)
	}
}

func TestTokenRequired(t *testing.T) {
	s := newTestServer(t)
	for _, tok := range []string{"", "wrong", s.token[:len(s.token)-1], s.token + "x", strings.ToUpper(s.token)} {
		h := map[string]string{"Host": "127.0.0.1:8090"}
		if tok != "" {
			h["X-Tabr-Token"] = tok
		}
		if w := do(t, s, "GET", "http://127.0.0.1/api/save/backups", loop, h, nil); w.Code != 403 {
			t.Errorf("token %q: got %d, want 403", tok, w.Code)
		}
	}
	if w := do(t, s, "GET", "http://127.0.0.1/api/save/backups", loop, map[string]string{"Host": "127.0.0.1:8090", "X-Tabr-Token": s.token}, nil); w.Code != 200 {
		t.Errorf("valid token: got %d", w.Code)
	}
}

func TestPostRequiresJSONContentType(t *testing.T) {
	s := newTestServer(t)
	base := map[string]string{"Host": "127.0.0.1:8090", "X-Tabr-Token": s.token}
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
		h := map[string]string{"Host": "127.0.0.1:8090", "X-Tabr-Token": s.token, "Content-Type": "application/json"}
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
	tok := map[string]string{"Host": "127.0.0.1:8090", "X-Tabr-Token": s.token}
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
	h := map[string]string{"Host": "127.0.0.1:8090", "X-Tabr-Token": s.token}
	if w := do(t, s, "GET", "http://127.0.0.1/api/db/exec", loop, h, nil); w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET on a mutating route: got %d, want 405", w.Code)
	}
}

func TestOversizeBodyRejected(t *testing.T) {
	s := newTestServer(t)
	h := map[string]string{"Host": "127.0.0.1:8090", "X-Tabr-Token": s.token, "Content-Type": "application/json"}
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
	h := map[string]string{"Host": "127.0.0.1:8090", "X-Tabr-Token": s.token}
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
		if err := CheckListenAddr(a, false); err != nil {
			t.Errorf("%q should be allowed: %v", a, err)
		}
	}
	for _, a := range bad {
		if err := CheckListenAddr(a, false); err == nil {
			t.Errorf("%q must be refused without the remote flag", a)
		}
	}
	if err := CheckListenAddr("0.0.0.0:8090", true); err != nil {
		t.Errorf("explicit remote flag should allow a routable bind: %v", err)
	}
	if err := CheckListenAddr("8090", true); err == nil {
		t.Errorf("a malformed address is refused even with the remote flag")
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
