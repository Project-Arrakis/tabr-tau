package web

import (
	"net/http"
	"testing"
)

// The edit boxes on single building pieces and vehicles were removed in #155; the routes behind them went too. A route nothing uses
// is a write the page cannot explain: it updated every building sharing an instance id (DBA-4 of the 2026-10-15 audit).
func TestRoutesForRemovedEditBoxesAreGone(t *testing.T) {
	s := newTestServer(t)
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Content-Type": "application/json", "Origin": "http://127.0.0.1:8090"}
	for _, p := range []string{"/api/bases/health", "/api/vehicles/durability"} {
		w := do(t, s, "POST", "http://127.0.0.1:8090"+p, loop, h, []byte(`{"id":1,"health":1,"vehicle_id":1,"chassis_durability":1}`))
		if w.Code != http.StatusNotFound && w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s must not be a route any more: %d %s", p, w.Code, w.Body.String())
		}
	}
	if s.Save.Dirty() {
		t.Fatal("nothing may have been written")
	}
	// the routes that stay still answer
	if w := do(t, s, "POST", "http://127.0.0.1:8090/api/items/update", loop, h, []byte(`{"id":1}`)); w.Code == http.StatusNotFound {
		t.Errorf("/api/items/update is still a route: %d", w.Code)
	}
}
