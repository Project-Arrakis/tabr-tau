package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func aboutHeaders(s *Server) map[string]string {
	return map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
}

func getAbout(t *testing.T, s *Server, query string) map[string]any {
	t.Helper()
	w := do(t, s, "GET", "http://127.0.0.1/api/about"+query, loop, aboutHeaders(s), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/about%s: %d %s", query, w.Code, w.Body.String())
	}
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("not JSON: %v: %s", err, w.Body.String())
	}
	return m
}

func TestAboutAnswersTheVersionOfThisBuild(t *testing.T) {
	s := newTestServer(t)
	m := getAbout(t, s, "")
	// a test binary has no stamp: it is a development build
	for k, want := range map[string]any{"version": "dev", "label": "dev", "channel": "stable", "dev": true} {
		if m[k] != want {
			t.Errorf("%s = %v, want %v", k, m[k], want)
		}
	}
	for _, k := range []string{"line", "build", "commit", "date", "arch"} {
		if _, ok := m[k]; !ok {
			t.Errorf("answer has no %q", k)
		}
	}
	if _, has := m["notes"]; has {
		t.Error("the notes come only with notes=1")
	}
}

func TestAboutNotesAreReadFromTheEditorsFolderOnly(t *testing.T) {
	s := newTestServer(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("## [0.1.0]\n- First <b>release</b>.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// no folder set: no notes
	if m := getAbout(t, s, "?notes=1"); m["notes"] != "" {
		t.Errorf("no folder: notes = %q", m["notes"])
	}
	s.NotesDir = dir
	m := getAbout(t, s, "?notes=1")
	if m["notes"] != "## [0.1.0]\n- First <b>release</b>." || m["notesFile"] != "notes.md" || m["notesTruncated"] != false {
		t.Errorf("notes answer: %v", m)
	}
	// the query chooses nothing but whether the notes come: a path or another value is not followed
	for _, q := range []string{"?notes=../../etc", "?notes=" + dir, "?file=notes.md", "?notes=2"} {
		if m := getAbout(t, s, q); m["notes"] != nil {
			t.Errorf("%s: notes were returned", q)
		}
	}
}

func TestAboutIsReadOnlyAndGuarded(t *testing.T) {
	s := newTestServer(t)
	h := aboutHeaders(s)
	h["Content-Type"] = "application/json"
	if w := do(t, s, "POST", "http://127.0.0.1/api/about", loop, h, []byte(`{}`)); w.Code != http.StatusMethodNotAllowed && w.Code != http.StatusNotFound {
		t.Errorf("POST /api/about: %d", w.Code)
	}
	// no session: refused like every other route, and the version is not in the answer
	w := do(t, s, "GET", "http://127.0.0.1/api/about", loop, map[string]string{"Host": "127.0.0.1:8090"}, nil)
	if w.Code == http.StatusOK || strings.Contains(w.Body.String(), "version") {
		t.Errorf("without a session: %d %s", w.Code, w.Body.String())
	}
}
