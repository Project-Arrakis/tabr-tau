package design

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestTokensIsACopy(t *testing.T) {
	a := Tokens()
	if len(a) == 0 {
		t.Fatal("tokens.css is empty")
	}
	a[0] ^= 0xff
	if b := Tokens(); b[0] == a[0] {
		t.Error("Tokens returned the embedded bytes, not a copy")
	}
}

func TestAssetsAreRootedAtTheAssetsFolder(t *testing.T) {
	var names []string
	err := fs.WalkDir(Assets, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != TokensFile {
		t.Errorf("version 0.1.0 holds the tokens only; Assets has %v", names)
	}
}

func get(h http.Handler, method, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, target, nil))
	return w
}

func TestHandlerServesTheStylesheetReadOnly(t *testing.T) {
	h := http.StripPrefix("/design/", Handler())
	w := get(h, "GET", "/design/tokens.css")
	if w.Code != 200 || w.Body.String() != string(Tokens()) {
		t.Fatalf("GET tokens.css: %d, %d bytes", w.Code, w.Body.Len())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/css; charset=utf-8" {
		t.Errorf("content type %q", ct)
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("no nosniff")
	}
	if w := get(h, "HEAD", "/design/tokens.css"); w.Code != 200 || w.Body.Len() != 0 {
		t.Errorf("HEAD: %d, %d bytes", w.Code, w.Body.Len())
	}
	for _, m := range []string{"POST", "PUT", "DELETE", "PATCH"} {
		if w := get(h, m, "/design/tokens.css"); w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET, HEAD" {
			t.Errorf("%s: %d", m, w.Code)
		}
	}
	for _, p := range []string{"/design/", "/design/nothing.css", "/design/%2e%2e/go.mod", "/design/..%2fgo.mod", "/design/tokens.css/"} {
		if w := get(h, "GET", p); w.Code == 200 {
			t.Errorf("GET %s answered 200", p)
		}
	}
}

func TestHandlerAlsoWorksWithoutAPrefix(t *testing.T) {
	if w := get(Handler(), "GET", "/tokens.css"); w.Code != 200 {
		t.Errorf("GET /tokens.css: %d", w.Code)
	}
}

// Version, the VERSION file and the newest heading of the changelog say the same thing.
func TestVersionsAgree(t *testing.T) {
	b, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatal(err)
	}
	if v := strings.TrimSpace(string(b)); v != Version {
		t.Errorf("VERSION is %q, design.Version is %q", v, Version)
	}
	if !regexp.MustCompile(`^v[01]\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`).MatchString(Version) {
		t.Errorf("Version %q is not vX.Y.Z with X of 0 or 1", Version)
	}
	c, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^## (v\S+)`).FindStringSubmatch(string(c))
	if m == nil || m[1] != Version {
		t.Errorf("the newest heading of CHANGELOG.md is %v, want ## %s", m, Version)
	}
}
