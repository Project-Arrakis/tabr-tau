package webguard

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func req(host string, hdr map[string]string) *http.Request {
	r := httptest.NewRequest("POST", "/api/x", nil)
	r.Host = host
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	return r
}

// The three policies in use and the strictest one, each over the same inputs. want is {strictest, no case folding but "none" allowed, both allowed}.
func TestSameOrigin(t *testing.T) {
	strict := OriginPolicy{}
	none := OriginPolicy{AllowFetchSiteNone: true}
	both := OriginPolicy{AllowFetchSiteNone: true, FoldHostCase: true}
	fold := OriginPolicy{FoldHostCase: true}
	for _, c := range []struct {
		name string
		host string
		hdr  map[string]string
		want [4]bool // strict, none, both, fold
	}{
		{"no headers (curl)", "127.0.0.1:8090", nil, [4]bool{true, true, true, true}},
		{"same origin", "127.0.0.1:8090", map[string]string{"Origin": "http://127.0.0.1:8090"}, [4]bool{true, true, true, true}},
		{"same origin with a path", "127.0.0.1:8090", map[string]string{"Origin": "http://127.0.0.1:8090/a?b#c"}, [4]bool{true, true, true, true}},
		{"scheme in capitals", "127.0.0.1:8090", map[string]string{"Origin": "HTTP://127.0.0.1:8090"}, [4]bool{true, true, true, true}},
		{"host case differs", "localhost:8090", map[string]string{"Origin": "http://LOCALHOST:8090"}, [4]bool{false, false, true, true}},
		{"another host", "127.0.0.1:8090", map[string]string{"Origin": "http://evil.example"}, [4]bool{false, false, false, false}},
		{"another port", "127.0.0.1:8090", map[string]string{"Origin": "http://127.0.0.1:9999"}, [4]bool{false, false, false, false}},
		{"port left off", "127.0.0.1:8090", map[string]string{"Origin": "http://127.0.0.1"}, [4]bool{false, false, false, false}},
		{"https", "127.0.0.1:8090", map[string]string{"Origin": "https://127.0.0.1:8090"}, [4]bool{false, false, false, false}},
		{"ftp", "127.0.0.1:8090", map[string]string{"Origin": "ftp://127.0.0.1:8090"}, [4]bool{false, false, false, false}},
		{"null", "127.0.0.1:8090", map[string]string{"Origin": "null"}, [4]bool{false, false, false, false}},
		{"no scheme", "127.0.0.1:8090", map[string]string{"Origin": "127.0.0.1:8090"}, [4]bool{false, false, false, false}},
		{"not an address", "127.0.0.1:8090", map[string]string{"Origin": "%zz"}, [4]bool{false, false, false, false}},
		{"fetch same-origin", "127.0.0.1:8090", map[string]string{"Sec-Fetch-Site": "same-origin"}, [4]bool{true, true, true, true}},
		{"fetch none", "127.0.0.1:8090", map[string]string{"Sec-Fetch-Site": "none"}, [4]bool{false, true, true, false}},
		{"fetch cross-site", "127.0.0.1:8090", map[string]string{"Sec-Fetch-Site": "cross-site"}, [4]bool{false, false, false, false}},
		{"fetch same-site", "127.0.0.1:8090", map[string]string{"Sec-Fetch-Site": "same-site"}, [4]bool{false, false, false, false}},
		{"fetch unknown", "127.0.0.1:8090", map[string]string{"Sec-Fetch-Site": "x"}, [4]bool{false, false, false, false}},
		{"fetch in capitals", "127.0.0.1:8090", map[string]string{"Sec-Fetch-Site": "SAME-ORIGIN"}, [4]bool{false, false, false, false}},
		{"fetch ok, origin not", "127.0.0.1:8090", map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "http://evil.example"}, [4]bool{false, false, false, false}},
		{"origin ok, fetch not", "127.0.0.1:8090", map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "http://127.0.0.1:8090"}, [4]bool{false, false, false, false}},
		{"none and a bad origin", "127.0.0.1:8090", map[string]string{"Sec-Fetch-Site": "none", "Origin": "http://evil.example"}, [4]bool{false, false, false, false}},
	} {
		for i, p := range []OriginPolicy{strict, none, both, fold} {
			if got := SameOrigin(req(c.host, c.hdr), p); got != c.want[i] {
				t.Errorf("%s, policy %+v: got %v, want %v", c.name, p, got, c.want[i])
			}
		}
	}
}

// No policy lets through what the strictest one refuses, except by what the field says: the other three only add "none" or case folding.
func TestNoPolicyIsLooserThanItsNamedOptions(t *testing.T) {
	for _, hdr := range []map[string]string{
		{"Origin": "https://127.0.0.1:8090"}, {"Origin": "http://evil.example"}, {"Sec-Fetch-Site": "cross-site"}, {"Sec-Fetch-Site": "same-site"},
		{"Origin": "null"}, {"Origin": "file://"}, {"Origin": ""},
	} {
		strict := SameOrigin(req("127.0.0.1:8090", hdr), OriginPolicy{})
		for _, p := range []OriginPolicy{{AllowFetchSiteNone: true}, {FoldHostCase: true}, {AllowFetchSiteNone: true, FoldHostCase: true}} {
			if !strict && SameOrigin(req("127.0.0.1:8090", hdr), p) {
				t.Errorf("%v: policy %+v accepts what the strictest refuses", hdr, p)
			}
		}
	}
}

func TestJSONRequest(t *testing.T) {
	for ct, want := range map[string]bool{
		"application/json": true, "application/json; charset=utf-8": true, "application/json;charset=UTF-8": true, "APPLICATION/JSON": true, "Application/Json": true,
		" application/json": true, "application/json ": true,
		"application/jsonx": false, "application/json-seq": false, "application/jsonp": false, "application/ld+json": false, "text/json": false,
		"application/x-www-form-urlencoded": false, "multipart/form-data; boundary=x": false, "text/plain": false, "text/plain; application/json": false,
		"": false, "json": false, "application": false, "application/": false, "/json": false, "application/json;;": false, "application/json; charset": false,
		"application/json, text/plain": false,
	} {
		r := httptest.NewRequest("POST", "/", nil)
		if ct != "" {
			r.Header.Set("Content-Type", ct)
		}
		if got := JSONRequest(r); got != want {
			t.Errorf("Content-Type %q: got %v, want %v", ct, got, want)
		}
	}
}
