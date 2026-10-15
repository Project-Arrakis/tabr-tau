package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// A page whose session is gone is told so in a JSON sentence the page can show. A plain-text 403 made the page fall back to a generic
// message, and "open the link printed in the terminal" is wrong for the editor's own window, which has no terminal (UX-10 of the 2026-10-15
// audit).
func TestASignedOutPageGetsAJSONSentenceThatFitsBothWaysOfRunning(t *testing.T) {
	s := newTestServer(t)
	w := do(t, s, "GET", "http://127.0.0.1:8090/api/save/state", loop, map[string]string{"Host": "127.0.0.1:8090"}, nil)
	if w.Code != http.StatusForbidden || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("%d %q %s", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
	var m map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("%v: %s", err, w.Body.String())
	}
	if m["code"] != "not_signed_in" || !strings.Contains(m["error"], "no longer signed in") || !strings.Contains(m["error"], "own window") || !strings.Contains(m["error"], "terminal") {
		t.Fatalf("the sentence must cover the window and the browser: %v", m)
	}
	// a wrong cookie is the same answer, and a request that is refused for another reason is not mistaken for it
	bad := do(t, s, "GET", "http://127.0.0.1:8090/api/save/state", loop, map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=wrong"}, nil)
	if bad.Code != http.StatusForbidden || !strings.Contains(bad.Body.String(), "not_signed_in") {
		t.Fatalf("%d %s", bad.Code, bad.Body.String())
	}
	cross := do(t, s, "GET", "http://127.0.0.1:8090/api/save/state", loop, map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Sec-Fetch-Site": "cross-site"}, nil)
	if cross.Code != http.StatusForbidden || strings.Contains(cross.Body.String(), "not_signed_in") {
		t.Fatalf("a cross-site refusal is not a sign-in problem: %d %s", cross.Code, cross.Body.String())
	}
}
