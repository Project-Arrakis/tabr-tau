package web

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

func getIndex(t *testing.T, s *Server, query string, cookie string) (code int, body string, hdr http.Header) {
	t.Helper()
	h := map[string]string{"Host": "127.0.0.1:8090"}
	if cookie != "" {
		h["Cookie"] = cookie
	}
	w := do(t, s, "GET", "http://127.0.0.1/"+query, loop, h, nil)
	return w.Code, w.Body.String(), w.Header()
}

func TestIndexIsLockedWithoutSession(t *testing.T) {
	s := newTestServer(t)
	code, body, _ := getIndex(t, s, "", "")
	if code != http.StatusUnauthorized {
		t.Fatalf("GET / without a session: got %d, want 401", code)
	}
	if strings.Contains(body, s.token) || strings.Contains(body, "__TOKEN__") {
		t.Fatal("the locked page must not contain the session token")
	}
	if !strings.Contains(body, "terminal") {
		t.Errorf("locked page should tell the user to use the link printed in the terminal; got %q", body)
	}
}

func TestBootTokenFlowSetsHardenedCookie(t *testing.T) {
	s := newTestServer(t)
	boot := s.NewBootToken()
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(boot) {
		t.Fatalf("boot token should be 128-bit hex, got %q", boot)
	}
	w := do(t, s, "GET", "http://127.0.0.1/?boot="+boot, loop, map[string]string{"Host": "127.0.0.1:8090"}, nil)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
		t.Fatalf("boot exchange: got %d Location=%q, want 303 to /", w.Code, w.Header().Get("Location"))
	}
	var c *http.Cookie
	for _, k := range w.Result().Cookies() {
		if k.Name == "tabr_session" {
			c = k
		}
	}
	if c == nil {
		t.Fatal("no tabr_session cookie set")
	}
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.Value != s.token {
		t.Errorf("cookie must be HttpOnly, SameSite=Strict, Path=/ and carry the session token: %+v", c)
	}
	if c.MaxAge != 0 || !c.Expires.IsZero() {
		t.Errorf("cookie must be a browser-session cookie (no persistent expiry): %+v", c)
	}
	if strings.Contains(w.Body.String(), s.token) {
		t.Error("the redirect body must not contain the token")
	}
}

func TestBootTokenIsSingleUse(t *testing.T) {
	s := newTestServer(t)
	boot := s.NewBootToken()
	if code, _, _ := getIndex(t, s, "?boot="+boot, ""); code != http.StatusSeeOther {
		t.Fatalf("first use: got %d", code)
	}
	if code, _, _ := getIndex(t, s, "?boot="+boot, ""); code != http.StatusUnauthorized {
		t.Fatalf("replay: got %d, want 401", code)
	}
}

func TestBootTokenExpires(t *testing.T) {
	s := newTestServer(t)
	now := time.Now()
	s.now = func() time.Time { return now }
	boot := s.NewBootToken()
	now = now.Add(bootTTL + time.Second)
	if code, _, _ := getIndex(t, s, "?boot="+boot, ""); code != http.StatusUnauthorized {
		t.Fatalf("expired boot token: got %d, want 401", code)
	}
}

func TestWrongOrMalformedBootTokensRefused(t *testing.T) {
	s := newTestServer(t)
	s.NewBootToken()
	for _, q := range []string{"?boot=", "?boot=nope", "?boot=" + strings.Repeat("0", 32), "?boot=" + s.token, "?boot[]=x"} {
		if code, _, _ := getIndex(t, s, q, ""); code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", q, code)
		}
	}
}

func TestSessionTokenIsNeverInThePage(t *testing.T) {
	s := newTestServer(t)
	code, body, _ := getIndex(t, s, "", "tabr_session="+s.token)
	if code != http.StatusOK {
		t.Fatalf("with a valid session: got %d", code)
	}
	if strings.Contains(body, s.token) || strings.Contains(body, "__TOKEN__") || strings.Contains(strings.ToLower(body), "token") {
		t.Fatal("the page must not embed or mention the session token (F-04)")
	}
}

func TestAPIAcceptsOnlyTheSessionCookie(t *testing.T) {
	s := newTestServer(t)
	get := func(h map[string]string) int {
		h["Host"] = "127.0.0.1:8090"
		return do(t, s, "GET", "http://127.0.0.1/api/save/backups", loop, h, nil).Code
	}
	if c := get(map[string]string{"Cookie": "tabr_session=" + s.token}); c != 200 {
		t.Errorf("valid cookie: %d", c)
	}
	if c := get(map[string]string{"X-Tabr-Token": s.token}); c != 403 {
		t.Errorf("the old header token must no longer authenticate: %d", c)
	}
	for _, cookie := range []string{"", "tabr_session=", "tabr_session=wrong", "tabr_session=" + s.token + "x", "tabr_session=" + s.token[:31], "other=" + s.token} {
		h := map[string]string{}
		if cookie != "" {
			h["Cookie"] = cookie
		}
		if c := get(h); c != 403 {
			t.Errorf("cookie %q: got %d, want 403", cookie, c)
		}
	}
}

func TestStaticAssets(t *testing.T) {
	s := newTestServer(t)
	for path, ct := range map[string]string{"/app.js": "text/javascript", "/html.js": "text/javascript", "/app.css": "text/css"} {
		w := do(t, s, "GET", "http://127.0.0.1"+path, loop, map[string]string{"Host": "127.0.0.1:8090"}, nil)
		if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), ct) || w.Body.Len() == 0 {
			t.Errorf("%s: code %d type %q len %d", path, w.Code, w.Header().Get("Content-Type"), w.Body.Len())
		}
		if strings.Contains(w.Body.String(), s.token) {
			t.Errorf("%s contains the token", path)
		}
	}
	for _, path := range []string{"/index.html", "/static/index.html", "/static/app.js", "/locked.html", "/../go.mod", "/app.js/", "/favicon.ico"} {
		if w := do(t, s, "GET", "http://127.0.0.1"+path, loop, map[string]string{"Host": "127.0.0.1:8090"}, nil); w.Code == 200 {
			t.Errorf("%s must not be served", path)
		}
	}
	if w := do(t, s, "GET", "http://127.0.0.1/app.js", "192.168.1.5:1", map[string]string{"Host": "127.0.0.1:8090"}, nil); w.Code != 403 {
		t.Errorf("assets are still subject to the loopback-peer rule: %d", w.Code)
	}
}

func TestContentSecurityPolicyForbidsInlineAndEval(t *testing.T) {
	s := newTestServer(t)
	_, _, h := getIndex(t, s, "", "tabr_session="+s.token)
	csp := h.Get("Content-Security-Policy")
	for _, must := range []string{"default-src 'none'", "script-src 'self'", "style-src 'self'", "connect-src 'self'", "frame-ancestors 'none'", "base-uri 'none'", "object-src 'none'", "form-action 'none'"} {
		if !strings.Contains(csp, must) {
			t.Errorf("CSP missing %q: %s", must, csp)
		}
	}
	for _, mustNot := range []string{"unsafe-inline", "unsafe-eval", "data:", "*", "http:", "https:"} {
		if strings.Contains(csp, mustNot) {
			t.Errorf("CSP must not contain %q: %s", mustNot, csp)
		}
	}
}

// The UI files are checked as text: no inline code, no dynamic code execution, no raw HTML sinks outside the one
// helper, and no attribute position that would escape a quoted value. The hostile-data browser test (web-tests)
// covers behavior; these catch the regressions that are cheap to see in source.
func TestUIFilesContainNoInlineCodeOrRawSinks(t *testing.T) {
	read := func(n string) string {
		b, err := static.ReadFile("static/" + n)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	index, app, htmlJS, locked := read("index.html"), read("app.js"), read("html.js"), read("locked.html")

	for name, doc := range map[string]string{"index.html": index, "locked.html": locked} {
		for _, m := range regexp.MustCompile(`(?is)<script\b[^>]*>`).FindAllString(doc, -1) {
			if !strings.Contains(m, " src=") {
				t.Errorf("%s has an inline <script>: %s", name, m)
			}
		}
		if regexp.MustCompile(`(?i)\son[a-z]+\s*=`).MatchString(doc) {
			t.Errorf("%s has an inline event handler attribute", name)
		}
		if regexp.MustCompile(`(?i)\sstyle\s*=|<style\b`).MatchString(doc) {
			t.Errorf("%s has inline style", name)
		}
		if strings.Contains(strings.ToLower(doc), "javascript:") {
			t.Errorf("%s contains a javascript: URL", name)
		}
	}
	for name, src := range map[string]string{"app.js": app, "html.js": htmlJS} {
		for _, bad := range []string{"eval(", "new Function", "document.write", "insertAdjacentHTML", "outerHTML", "javascript:", "setAttribute('on", "setAttribute(\"on"} {
			if strings.Contains(src, bad) {
				t.Errorf("%s contains %q", name, bad)
			}
		}
	}
	// the only place allowed to assign innerHTML is the setHTML sink itself
	sink := regexp.MustCompile(`\.innerHTML\s*=`)
	if n := len(sink.FindAllString(app, -1)); n != 0 {
		t.Errorf("app.js must not assign innerHTML directly (found %d): use setHTML(el, html`...`)", n)
	}
	if n := len(sink.FindAllString(htmlJS, -1)); n != 1 {
		t.Errorf("html.js must assign innerHTML in exactly one place (setHTML), found %d", n)
	}
	// whitespace + attribute name + =${ is an UNQUOTED attribute value: attribute injection. (?name=${x} inside a
	// quoted URL is fine and does not match.)
	if m := regexp.MustCompile("\\s[\\w:-]+=\\$\\{").FindString(app); m != "" {
		t.Errorf("app.js has an unquoted attribute interpolation near %q; always quote attribute values", m)
	}
	if regexp.MustCompile(`style\s*=\s*"`).MatchString(app) {
		t.Errorf("app.js must not emit inline style attributes (CSP style-src 'self')")
	}
	if strings.Contains(app, "X-Tabr-Token") || strings.Contains(app, "TOKEN") {
		t.Errorf("app.js must not know about a token: the session is an HttpOnly cookie")
	}
}

func TestBootURL(t *testing.T) {
	s := newTestServer(t)
	u := s.BootURL("http://127.0.0.1:8090")
	if !regexp.MustCompile(`^http://127\.0\.0\.1:8090/\?boot=[0-9a-f]{32}$`).MatchString(u) {
		t.Fatalf("BootURL = %q", u)
	}
	if strings.Contains(u, s.token) {
		t.Fatal("BootURL must contain the one-time boot token, never the session token")
	}
}
