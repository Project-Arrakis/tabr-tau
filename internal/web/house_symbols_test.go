package web

import (
	"io/fs"
	"net/http"
	"strings"
	"testing"
)

// The Landsraad tab's house symbols (#645): the 25 embedded pictures are served with their picture type from a fixed list, small (the page shows them at 32 px),
// and an unknown name, a path out of the folder and a made-up extension never come back as a picture.
func TestHouseSymbolsAreServedFromAFixedList(t *testing.T) {
	entries, err := fs.ReadDir(static, "static/houses")
	if err != nil || len(entries) != 25 {
		t.Fatalf("embedded house symbols: %d %v", len(entries), err)
	}
	s := newTestServer(t)
	get := func(path string) (int, string, int) {
		w := do(t, s, "GET", "http://127.0.0.1"+path, loop, nil, nil)
		return w.Code, w.Header().Get("Content-Type"), w.Body.Len()
	}
	total := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".webp") || e.Name() != strings.ToLower(e.Name()) {
			t.Fatalf("%s: a symbol is a lower-case .webp file", e.Name())
		}
		code, ct, n := get("/houses/" + e.Name())
		if code != http.StatusOK || ct != "image/webp" {
			t.Fatalf("%s: %d %q %d bytes", e.Name(), code, ct, n)
		}
		embedded, err := static.ReadFile("static/houses/" + e.Name())
		if err != nil || n != len(embedded) || !strings.HasPrefix(string(embedded), "RIFF") {
			t.Fatalf("%s: served %d bytes, embedded %d (%v)", e.Name(), n, len(embedded), err)
		}
		// 64 by 64 pixels (VP8X: canvas width and height minus one at bytes 24 to 29): twice the 32 px the page draws, sharp on a high-density screen
		if len(embedded) < 30 || string(embedded[12:16]) != "VP8X" || int(embedded[24])|int(embedded[25])<<8|int(embedded[26])<<16 != 63 || int(embedded[27])|int(embedded[28])<<8|int(embedded[29])<<16 != 63 {
			t.Errorf("%s is not a 64 by 64 picture", e.Name())
		}
		if n > 4<<10 {
			t.Errorf("%s is %d bytes: the symbols are meant to stay small", e.Name(), n)
		}
		total += n
	}
	if total > 64<<10 {
		t.Errorf("the 25 symbols are %d bytes together; keep them under 64 KiB", total)
	}
	for _, bad := range []string{"/houses/nope.webp", "/houses/../app.js", "/houses/%2e%2e/app.js", "/houses/", "/houses", "/houses/alexin.png", "/houses/ALEXIN.webp"} {
		if code, ct, _ := get(bad); code == http.StatusOK && strings.HasPrefix(ct, "image/") {
			t.Errorf("%s must not be served as a picture: %d %q", bad, code, ct)
		}
	}
}
