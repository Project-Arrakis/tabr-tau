package web

import (
	"bytes"
	"io/fs"
	"path"
	"strings"
	"testing"
)

// QA-8: what the editor ships is what the server serves. Every embedded file is either served at its own path with its own
// bytes and the right type, or is one of the two pages that "/" serves; a file added to static/ without a route (the
// /livemap.js case) fails here, and so does a route that stops serving one.
func TestEveryEmbeddedAssetIsServedWithItsOwnBytes(t *testing.T) {
	s := newTestServer(t)
	hdr := map[string]string{"Host": "127.0.0.1:8090"}
	wantType := map[string]string{".js": "text/javascript", ".css": "text/css", ".png": "image/png", ".webp": "image/webp"}
	pages := map[string]bool{"static/index.html": true, "static/locked.html": true}
	served := 0
	err := fs.WalkDir(static, "static", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || pages[p] {
			return err
		}
		want, err := static.ReadFile(p)
		if err != nil {
			return err
		}
		url := strings.TrimPrefix(p, "static")
		ct, known := wantType[strings.ToLower(path.Ext(p))]
		if !known {
			t.Errorf("%s: no expected content type for this kind of file; add it to the test and to the server's list", p)
			return nil
		}
		w := do(t, s, "GET", "http://127.0.0.1"+url, loop, hdr, nil)
		if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), ct) || !bytes.Equal(w.Body.Bytes(), want) || len(want) == 0 {
			t.Errorf("%s: code %d, type %q, %d of %d bytes", url, w.Code, w.Header().Get("Content-Type"), w.Body.Len(), len(want))
		}
		served++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if served < 25 { // app.js, html.js, livemap.js, app.css, the two maps and the icons
		t.Fatalf("only %d embedded assets found", served)
	}
	for _, must := range []string{"/app.js", "/html.js", "/livemap.js", "/app.css", "/maps/hagga-basin.png", "/maps/deep-desert.png"} {
		if w := do(t, s, "GET", "http://127.0.0.1"+must, loop, hdr, nil); w.Code != 200 {
			t.Errorf("%s: %d; the page cannot start without it", must, w.Code)
		}
	}
	// the page itself: index.html for a session, locked.html for anyone else, and the page names every script it needs
	index, _ := static.ReadFile("static/index.html")
	locked, _ := static.ReadFile("static/locked.html")
	w := do(t, s, "GET", "http://127.0.0.1/", loop, map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}, nil)
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), index) {
		t.Errorf("/ with a session: %d, index.html served = %v", w.Code, bytes.Equal(w.Body.Bytes(), index))
	}
	w = do(t, s, "GET", "http://127.0.0.1/", loop, hdr, nil)
	if w.Code != 401 || !bytes.Equal(w.Body.Bytes(), locked) {
		t.Errorf("/ without a session: %d, locked.html served = %v", w.Code, bytes.Equal(w.Body.Bytes(), locked))
	}
	for _, src := range []string{"/html.js", "/livemap.js", "/app.js"} {
		if !strings.Contains(string(index), `src="`+src+`"`) {
			t.Errorf("index.html does not load %s", src)
		}
	}
}
