package web

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/save"
)

// The page learns that a save's location is unconfirmed from the status poll, and answers through one route (NET-3 of the 2026-10-15 audit).
func TestStateReportsLocationAndTheConfirmRouteAnswers(t *testing.T) {
	s := newTestServer(t)
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Content-Type": "application/json", "Origin": "http://127.0.0.1:8090"}
	w := do(t, s, "GET", "http://127.0.0.1:8090/api/save/state", loop, h, nil)
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if v, has := m["locationUnconfirmed"]; !has || v != false {
		t.Fatalf("the state must carry locationUnconfirmed (false for a save the guard trusts): %v", m)
	}
	// a location the guard trusts has nothing to confirm: the route refuses and records nothing
	if w := do(t, s, "POST", "http://127.0.0.1:8090/api/save/confirm-location", loop, h, []byte(`{}`)); w.Code == http.StatusOK {
		t.Fatalf("confirm-location accepted a location that is not in question: %s", w.Body.String())
	}
	// one it is refusing can be confirmed, and then no longer blocks
	defer save.UntrustedLocationForTests()()
	if g := save.GameStateFor(s.Save.Path, true); !g.NeedsConfirm {
		t.Fatalf("the stand-in untrusted location must ask for confirmation: %+v", g)
	}
	if w := do(t, s, "POST", "http://127.0.0.1:8090/api/save/confirm-location", loop, h, []byte(`{}`)); w.Code != http.StatusOK {
		t.Fatalf("confirm-location: %d %s", w.Code, w.Body.String())
	}
	if g := save.GameStateFor(s.Save.Path, true); g.NeedsConfirm {
		t.Fatalf("after the confirmation the location must stop blocking: %+v", g)
	}
	// it is a POST route behind the session like the rest
	if w := do(t, s, "GET", "http://127.0.0.1:8090/api/save/confirm-location", loop, h, nil); w.Code == http.StatusOK {
		t.Fatalf("GET must not confirm anything: %d", w.Code)
	}
	noCookie := map[string]string{"Host": "127.0.0.1:8090", "Content-Type": "application/json"}
	if w := do(t, s, "POST", "http://127.0.0.1:8090/api/save/confirm-location", loop, noCookie, []byte(`{}`)); w.Code != http.StatusForbidden {
		t.Fatalf("without a session: %d", w.Code)
	}
}
