package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// These tests pin, input by input, what the request guard in Handler accepts and refuses. They were written against the guard as it was
// typed in this package, before its mechanisms moved to kit/webguard (#561), and have to pass unchanged after the move: the move may
// not make any check looser. A case that looks odd (a mixed-case Host is refused, Sec-Fetch-Site "none" passes) is the policy of this
// program and is pinned on purpose.

// guardCall sends one request through the whole handler with a valid session and returns the answer.
func guardCall(t *testing.T, s *Server, method, remote string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
	for k, v := range hdr {
		if v == "-" { // "-" removes a default header
			delete(h, k)
			continue
		}
		h[k] = v
	}
	path := "http://127.0.0.1/api/save/backups"
	var body []byte
	if method == "POST" {
		path = "http://127.0.0.1/api/save/discard"
		body = []byte(`{}`)
	}
	return do(t, s, method, path, remote, h, body)
}

func TestGuardPinHostHeader(t *testing.T) {
	s := newTestServer(t)
	for host, want := range map[string]int{
		"127.0.0.1": 200, "127.0.0.1:8090": 200, "127.0.0.1:1": 200, "localhost": 200, "localhost:8090": 200,
		"[::1]": 200, "[::1]:8090": 200, "::1": 200,
		"LocalHost:8090": 403, "LOCALHOST": 403, "127.0.0.2:8090": 403, "127.5.6.7": 403, "0.0.0.0:8090": 403, "[::]:8090": 403,
		"": 403, "evil.com": 403, "localhost.evil.com": 403, "127.0.0.1.evil.com:8090": 403, "127.0.0.1@evil.com": 403, " 127.0.0.1": 403,
		"[::ffff:127.0.0.1]:8090": 403, "[0:0:0:0:0:0:0:1]:8090": 403,
	} {
		if got := guardCall(t, s, "GET", loop, map[string]string{"Host": host}).Code; got != want {
			t.Errorf("Host %q: got %d, want %d", host, got, want)
		}
	}
}

func TestGuardPinPeerAddress(t *testing.T) {
	s := newTestServer(t)
	for remote, want := range map[string]int{
		"127.0.0.1:5": 200, "127.9.9.9:5": 200, "[::1]:5": 200,
		"": 403, "127.0.0.1": 403, "::1": 403, "localhost:5": 403, "[::]:5": 403, "0.0.0.0:5": 403, "192.168.0.4:5": 403,
		"[2001:db8::1]:5": 403, "[::ffff:127.0.0.1]:5": 200, "[::ffff:192.168.0.4]:5": 403, "127.0.0.1:": 200, "999.0.0.1:5": 403,
	} {
		if got := guardCall(t, s, "GET", remote, nil).Code; got != want {
			t.Errorf("peer %q: got %d, want %d", remote, got, want)
		}
	}
}

func TestGuardPinOriginAndFetchMetadata(t *testing.T) {
	s := newTestServer(t)
	type c struct {
		name string
		hdr  map[string]string
		want int
	}
	for _, m := range []string{"POST", "GET"} { // the same test guards a read and a write
		for _, k := range []c{
			{"no Origin", nil, 200},
			{"same origin", map[string]string{"Origin": "http://127.0.0.1:8090"}, 200},
			{"same origin, with a path", map[string]string{"Origin": "http://127.0.0.1:8090/x/y?z#w"}, 200},
			{"same origin, scheme in capitals", map[string]string{"Origin": "HTTP://127.0.0.1:8090"}, 200},
			{"user info is not part of the host", map[string]string{"Origin": "http://evil@127.0.0.1:8090"}, 200},
			{"another host", map[string]string{"Origin": "http://evil.example"}, 403},
			{"another host, same port", map[string]string{"Origin": "http://evil.example:8090"}, 403},
			{"another port", map[string]string{"Origin": "http://127.0.0.1:9999"}, 403},
			{"no port", map[string]string{"Origin": "http://127.0.0.1"}, 403},
			{"another loopback name", map[string]string{"Origin": "http://localhost:8090"}, 403},
			{"https", map[string]string{"Origin": "https://127.0.0.1:8090"}, 403},
			{"another scheme", map[string]string{"Origin": "ftp://127.0.0.1:8090"}, 403},
			{"a file page", map[string]string{"Origin": "file://"}, 403},
			{"null", map[string]string{"Origin": "null"}, 403},
			{"not an address", map[string]string{"Origin": "%zz"}, 403},
			{"no scheme", map[string]string{"Origin": "127.0.0.1:8090"}, 403},
			{"same origin is the only value that passes", map[string]string{"Sec-Fetch-Site": "same-origin"}, 200},
			{"address bar or bookmark", map[string]string{"Sec-Fetch-Site": "none"}, 200},
			{"cross-site", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
			{"same-site", map[string]string{"Sec-Fetch-Site": "same-site"}, 403},
			{"unknown value", map[string]string{"Sec-Fetch-Site": "anything"}, 403},
			{"value in capitals", map[string]string{"Sec-Fetch-Site": "SAME-ORIGIN"}, 403},
			{"same origin, Fetch Metadata says cross-site", map[string]string{"Origin": "http://127.0.0.1:8090", "Sec-Fetch-Site": "cross-site"}, 403},
			{"Fetch Metadata says same origin, Origin says another", map[string]string{"Origin": "http://evil.example", "Sec-Fetch-Site": "same-origin"}, 403},
		} {
			hdr := map[string]string{"Content-Type": "application/json"}
			for key, v := range k.hdr {
				hdr[key] = v
			}
			if got := guardCall(t, s, m, loop, hdr).Code; got != k.want {
				t.Errorf("%s, %s: got %d, want %d", m, k.name, got, k.want)
			}
		}
	}
	// the Origin host follows the Host header, with the case folded
	for _, k := range []struct {
		host, origin string
		want         int
	}{
		{"localhost:8090", "http://localhost:8090", 200},
		{"localhost:8090", "http://LOCALHOST:8090", 200},
		{"localhost:8090", "http://127.0.0.1:8090", 403},
		{"[::1]:8090", "http://[::1]:8090", 200},
		{"::1", "http://[::1]", 403},
	} {
		got := guardCall(t, s, "POST", loop, map[string]string{"Host": k.host, "Origin": k.origin, "Content-Type": "application/json"}).Code
		if got != k.want {
			t.Errorf("Host %q, Origin %q: got %d, want %d", k.host, k.origin, got, k.want)
		}
	}
}

func TestGuardPinContentTypeOfAWrite(t *testing.T) {
	s := newTestServer(t)
	for ct, want := range map[string]int{
		"application/json": 200, "application/json; charset=utf-8": 200, "application/json;charset=UTF-8": 200, "APPLICATION/JSON": 200, "Application/Json": 200,
		" application/json": 200, "application/json ": 200,
		"application/jsonx": 415, "application/json-seq": 415, "application/jsonp": 415, "application/ld+json": 415, "text/json": 415,
		"application/x-www-form-urlencoded": 415, "multipart/form-data; boundary=x": 415, "text/plain": 415, "text/plain; application/json": 415,
		"": 415, "json": 415, "application": 415, "application/": 415, "/json": 415, "application/json;;": 415, "application/json; charset": 415,
		"application/json, text/plain": 415,
	} {
		hdr := map[string]string{}
		if ct != "" {
			hdr["Content-Type"] = ct
		} else {
			hdr["Content-Type"] = "-"
		}
		if got := guardCall(t, s, "POST", loop, hdr).Code; got != want {
			t.Errorf("Content-Type %q: got %d, want %d", ct, got, want)
		}
	}
	// a read needs no content type; any other method that is not a read does
	for _, m := range []string{"GET", "HEAD"} {
		if got := guardCall(t, s, m, loop, map[string]string{"Content-Type": "text/plain"}).Code; got != 200 {
			t.Errorf("%s with text/plain: got %d, want 200", m, got)
		}
	}
	for _, m := range []string{"PUT", "DELETE", "PATCH"} {
		if got := guardCall(t, s, m, loop, map[string]string{"Content-Type": "text/plain"}).Code; got != http.StatusUnsupportedMediaType {
			t.Errorf("%s with text/plain: got %d, want 415", m, got)
		}
	}
}

func TestGuardPinResponseHeadersOnEveryAnswer(t *testing.T) {
	s := newTestServer(t)
	const csp = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; font-src 'self'; " +
		"frame-ancestors 'none'; base-uri 'none'; object-src 'none'; form-action 'none'"
	want := map[string]string{
		"Cache-Control":                "no-store",
		"X-Frame-Options":              "DENY",
		"X-Content-Type-Options":       "nosniff",
		"Referrer-Policy":              "no-referrer",
		"Cross-Origin-Resource-Policy": "same-origin",
		"Content-Security-Policy":      csp,
	}
	answers := map[string]*httptest.ResponseRecorder{
		"ok":                 guardCall(t, s, "GET", loop, nil),
		"page":               do(t, s, "GET", "http://127.0.0.1/", loop, map[string]string{"Host": "127.0.0.1:8090"}, nil),
		"not found":          do(t, s, "GET", "http://127.0.0.1/nope", loop, map[string]string{"Host": "127.0.0.1:8090"}, nil),
		"bad host":           guardCall(t, s, "GET", loop, map[string]string{"Host": "evil.example"}),
		"bad peer":           guardCall(t, s, "GET", "192.168.0.4:1", nil),
		"no session":         guardCall(t, s, "GET", loop, map[string]string{"Cookie": "-"}),
		"cross-origin":       guardCall(t, s, "POST", loop, map[string]string{"Content-Type": "application/json", "Origin": "http://evil.example"}),
		"wrong content type": guardCall(t, s, "POST", loop, map[string]string{"Content-Type": "text/plain"}),
	}
	wantCode := map[string]int{"ok": 200, "page": 401, "not found": 404, "bad host": 403, "bad peer": 403, "no session": 403, "cross-origin": 403, "wrong content type": 415}
	for name, w := range answers {
		if w.Code != wantCode[name] {
			t.Errorf("%s: got %d, want %d", name, w.Code, wantCode[name])
		}
		for k, v := range want {
			if got := w.Header().Get(k); got != v {
				t.Errorf("%s: header %s = %q, want %q", name, k, got, v)
			}
		}
	}
}
