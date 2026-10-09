package web

import (
	"io/fs"
	"net/http"
	"strings"
	"testing"
)

// Every embedded Live Map icon is served with its picture type, and nothing else under /maps/icons/ is: an unknown name, a path out
// of the folder and a made-up extension never come back as a picture.
func TestLiveMapIconsAreServedFromAFixedList(t *testing.T) {
	entries, err := fs.ReadDir(static, "static/maps/icons")
	if err != nil || len(entries) < 20 {
		t.Fatalf("embedded icons: %d %v", len(entries), err)
	}
	s := newTestServer(t)
	get := func(path string) (int, string, int) {
		w := do(t, s, "GET", "http://127.0.0.1"+path, loop, nil, nil)
		return w.Code, w.Header().Get("Content-Type"), w.Body.Len()
	}
	for _, e := range entries {
		want := "image/png"
		if strings.HasSuffix(e.Name(), ".webp") {
			want = "image/webp"
		}
		code, ct, n := get("/maps/icons/" + e.Name())
		if code != http.StatusOK || ct != want || n < 100 {
			t.Fatalf("%s: %d %q %d bytes", e.Name(), code, ct, n)
		}
	}
	for _, bad := range []string{"/maps/icons/nope.webp", "/maps/icons/../app.js", "/maps/icons/%2e%2e/app.js", "/maps/icons/", "/maps/icons", "/maps/icons/x.svg"} {
		if code, ct, _ := get(bad); code == http.StatusOK && strings.HasPrefix(ct, "image/") {
			t.Errorf("%s must not be served as a picture: %d %q", bad, code, ct)
		}
	}
}
