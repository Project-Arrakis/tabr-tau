package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// A panic in a handler (say, a NULL where the code expected text) answers a JSON 500 instead of resetting the connection, and the
// answer says nothing about what panicked (SEC-5 of the 2026-10-15 audit).
func TestAPanicInAHandlerIsAJSONError(t *testing.T) {
	s := newTestServer(t)
	s.mux.HandleFunc("GET /api/boom", func(http.ResponseWriter, *http.Request) {
		var m map[string]any
		_ = m["kind"].(string) // the unchecked assertion on a NULL column
	})
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
	w := do(t, s, "GET", "http://127.0.0.1:8090/api/boom", loop, h, nil)
	if w.Code != http.StatusInternalServerError || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("%d %q %s", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil || !strings.Contains(m["error"].(string), "internal error") {
		t.Fatalf("%v %s", err, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "interface conversion") || strings.Contains(w.Body.String(), ".go") {
		t.Fatalf("the answer must not carry the panic: %s", w.Body.String())
	}
	if w.Header().Get("X-Frame-Options") == "" {
		t.Fatal("the security headers are on the error too")
	}
	// the same request still reaches a working route afterwards
	if w := do(t, s, "GET", "http://127.0.0.1:8090/api/save/state", loop, h, nil); w.Code != 200 {
		t.Fatalf("the server must keep serving: %d", w.Code)
	}
}

// The panic of an aborted response (the client went away) is passed on, as net/http intends.
func TestAbortedResponsesAreNotSwallowed(t *testing.T) {
	defer func() {
		if rec := recover(); rec != http.ErrAbortHandler {
			t.Fatalf("http.ErrAbortHandler must propagate, got %v", rec)
		}
	}()
	recoverPanic(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
}

// An error answered to the page names a file, not the folders (and the Windows user name) above it (SEC-10).
func TestErrorsDoNotCarryAbsolutePathsOrTheHomeFolder(t *testing.T) {
	err := fmt.Errorf("could not save: %w", &fs.PathError{Op: "open", Path: `C:\Users\someone\AppData\Local\Dune\Saved\game.db`, Err: errors.New("access denied")})
	got := publicError(err)
	if strings.Contains(got, "someone") || strings.Contains(got, `AppData`) || !strings.Contains(got, "game.db") || !strings.Contains(got, "access denied") {
		t.Fatalf("path error: %q", got)
	}
	link := &os.LinkError{Op: "rename", Old: `/home/someone/x/game.db.tmp`, New: `/home/someone/x/game.db`, Err: errors.New("busy")}
	if got := publicError(errors.Join(errors.New("first"), link)); strings.Contains(got, "someone") || !strings.Contains(got, "game.db.tmp") {
		t.Fatalf("link error inside a joined error: %q", got)
	}
	// a path that is only text in a message (not an OS error) is scrubbed by the home folder
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if got := publicError(fmt.Errorf("config folder not found: %s", home+"/Saved/Config")); strings.Contains(got, home) || !strings.Contains(got, "~") {
		t.Fatalf("home folder: %q", got)
	}
	// a root path has no file name to keep: the message is not stripped of its separators
	if got := publicError(&fs.PathError{Op: "open", Path: "/", Err: errors.New("is a directory")}); !strings.Contains(got, "/") {
		t.Fatalf("root path: %q", got)
	}
	if got := publicError(errors.New("item not found")); got != "item not found" {
		t.Fatalf("an ordinary message is unchanged: %q", got)
	}
	// and respond uses it
	w := httptest.NewRecorder()
	respond(w, nil, err)
	if w.Code != http.StatusBadRequest || strings.Contains(w.Body.String(), "someone") {
		t.Fatalf("respond: %d %s", w.Code, w.Body.String())
	}
}
