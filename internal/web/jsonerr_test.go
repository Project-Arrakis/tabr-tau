package web

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A value JSON cannot carry used to be sent as 200 with an empty body (SEC-1 of the 2026-10-15 audit). It is now a JSON 500 that says so.
func TestWriteJSONAnswersAnErrorWhenTheValueCannotBeEncoded(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]any{"x": math.Inf(1)})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, body %q", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content type %q", ct)
	}
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil || !strings.Contains(m["error"].(string), "could not encode") {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	w = httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]any{"x": 1.5})
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"x":1.5}` {
		t.Fatalf("an ordinary value is written as before: %d %q", w.Code, w.Body.String())
	}
}
