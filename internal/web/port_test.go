package web

import (
	"net"
	"net/http"
	"strings"
	"testing"
)

// Browsers share cookies between the ports of one host. Once the listener is known the session cookie is named after the port, so two
// editors on one machine keep their own, and a request is served only when its Host header names this listener (NET-1, NET-2, SEC-7 of
// the 2026-10-15 audit).
func boundServer(t *testing.T, addr string) *Server {
	t.Helper()
	s := newTestServer(t)
	s.BindAddr(addr)
	return s
}

func TestBoundServerNamesTheSessionCookieAfterThePort(t *testing.T) {
	a, b := boundServer(t, "127.0.0.1:8090"), boundServer(t, "127.0.0.1:54321")
	if a.sessionCookie() != "tabr_session_8090" || b.sessionCookie() != "tabr_session_54321" {
		t.Fatalf("%q %q", a.sessionCookie(), b.sessionCookie())
	}
	w := do(t, a, "GET", "http://127.0.0.1:8090/?boot="+a.NewBootToken(), loop, map[string]string{"Host": "127.0.0.1:8090"}, nil)
	var named string
	for _, k := range w.Result().Cookies() {
		named += k.Name + " "
	}
	if strings.TrimSpace(named) != "tabr_session_8090" {
		t.Fatalf("the boot exchange must set the per-port cookie only: %q", named)
	}
	// one editor's cookie does not open the other
	h := map[string]string{"Host": "127.0.0.1:54321", "Cookie": "tabr_session_8090=" + a.token + "; tabr_session=" + a.token}
	if w := do(t, b, "GET", "http://127.0.0.1:54321/api/save/backups", loop, h, nil); w.Code != http.StatusForbidden {
		t.Fatalf("another editor's cookie must not sign in here: %d", w.Code)
	}
	h = map[string]string{"Host": "127.0.0.1:54321", "Cookie": "tabr_session_54321=" + b.token}
	if w := do(t, b, "GET", "http://127.0.0.1:54321/api/save/backups", loop, h, nil); w.Code != http.StatusOK {
		t.Fatalf("its own cookie signs in: %d", w.Code)
	}
}

func TestBoundServerOnlyServesItsOwnHost(t *testing.T) {
	s := boundServer(t, "127.0.0.1:8090")
	cookie := "tabr_session_8090=" + s.token
	for host, want := range map[string]int{
		"127.0.0.1:8090": http.StatusOK, "localhost:8090": http.StatusOK, "LOCALHOST:8090": http.StatusOK,
		"127.0.0.1:9999": http.StatusForbidden, "127.0.0.1": http.StatusForbidden, "localhost": http.StatusForbidden,
		"[::1]:8090": http.StatusForbidden, "evil.example:8090": http.StatusForbidden, "": http.StatusForbidden,
	} {
		w := do(t, s, "GET", "http://x/api/save/backups", loop, map[string]string{"Host": host, "Cookie": cookie}, nil)
		if w.Code != want {
			t.Errorf("Host %q: got %d, want %d", host, w.Code, want)
		}
	}
	v6 := boundServer(t, "[::1]:8090")
	if w := do(t, v6, "GET", "http://x/api/save/backups", loop, map[string]string{"Host": "[::1]:8090", "Cookie": "tabr_session_8090=" + v6.token}, nil); w.Code != http.StatusOK {
		t.Errorf("an IPv6 listener serves its own address: %d", w.Code)
	}
}

// Another program on the same host can plant a cookie of the same name; a browser then sends both. The real one still opens the editor,
// and the shadow one alone does not.
func TestAShadowCookieDoesNotHideTheRealOne(t *testing.T) {
	s := boundServer(t, "127.0.0.1:8090")
	get := func(cookie string) int {
		return do(t, s, "GET", "http://x/api/save/backups", loop, map[string]string{"Host": "127.0.0.1:8090", "Cookie": cookie}, nil).Code
	}
	if c := get("tabr_session_8090=planted; tabr_session_8090=" + s.token); c != http.StatusOK {
		t.Errorf("shadow first: %d", c)
	}
	if c := get("tabr_session_8090=" + s.token + "; tabr_session_8090=planted"); c != http.StatusOK {
		t.Errorf("shadow last: %d", c)
	}
	if c := get("tabr_session_8090=planted; tabr_session_8090=also-wrong"); c != http.StatusForbidden {
		t.Errorf("only shadows: %d", c)
	}
}

func TestPortChangeNoteSaysSoOnlyWhenThePortMoved(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, got, _ := net.SplitHostPort(ln.Addr().String())
	if n := PortChangeNote("127.0.0.1:"+got, ln); n != "" {
		t.Fatalf("the port asked for is the port used: %q", n)
	}
	if n := PortChangeNote("127.0.0.1:0", ln); n != "" {
		t.Fatalf("port 0 asks for any port: %q", n)
	}
	n := PortChangeNote("127.0.0.1:8090", ln)
	if !strings.Contains(n, "8090") || !strings.Contains(n, got) || !strings.Contains(n, "already in use") {
		t.Fatalf("the note must name both ports and say why: %q", n)
	}
	if PortChangeNote("not an address", ln) != "" {
		t.Fatal("a malformed request address is not a port change")
	}
}

func TestStartupNotesAreShownOnceAndJoined(t *testing.T) {
	s := newTestServer(t)
	s.NoteStartup("Port moved.", true)
	s.NoteStartup("Second.", false)
	note, warn := s.takeStartup()
	if note != "Port moved. Second." || !warn {
		t.Fatalf("%q %v", note, warn)
	}
	if note, _ := s.takeStartup(); note != "" {
		t.Fatalf("shown once: %q", note)
	}
}
