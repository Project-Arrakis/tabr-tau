package web

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/settings"
)

// ARCH-10: the two routes that write settings.json (the automatic-refill switch and Assign on the Specialization tab) must not
// undo each other, and a settings file that cannot be read is reported and left alone, not replaced.
func TestSettingsRoutesKeepEachOthersChanges(t *testing.T) {
	s := newTestServer(t)
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	hdr := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Content-Type": "application/json"}
	post := func(route, body string) int {
		return do(t, s, "POST", "http://127.0.0.1"+route, loop, hdr, []byte(body)).Code
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	codes := map[int]int{}
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			c := post("/api/settings", `{"autoRefillOnOpen":true}`)
			mu.Lock()
			codes[c]++
			mu.Unlock()
		}()
		go func() {
			defer wg.Done()
			c := post("/api/player/specs/assign", `{"track":"Combat","number":3}`)
			mu.Lock()
			codes[c]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	got, err := settings.LoadStrict(s.SettingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !got.AutoRefillOnOpen || got.TrackNumbers["Combat"] != 3 {
		t.Fatalf("one route undid the other: %+v (response codes %v)", got, codes)
	}
}

func TestDamagedSettingsFileIsReportedAndNotOverwritten(t *testing.T) {
	s := newTestServer(t)
	s.SettingsPath = filepath.Join(t.TempDir(), "settings.json")
	damaged := `{"autoRefillOnOpen": true, "trackNumbers": {"Combat": 3`
	if err := os.WriteFile(s.SettingsPath, []byte(damaged), 0o600); err != nil {
		t.Fatal(err)
	}
	hdr := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Content-Type": "application/json"}
	w := do(t, s, "GET", "http://127.0.0.1/api/settings", loop, hdr, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "damaged") {
		t.Fatalf("the damage must be reported to the page: %d %s", w.Code, w.Body.String())
	}
	for _, c := range []struct{ route, body string }{
		{"/api/settings", `{"autoRefillOnOpen":false}`},
		{"/api/player/specs/assign", `{"track":"Crafting","number":4}`},
	} {
		w := do(t, s, "POST", "http://127.0.0.1"+c.route, loop, hdr, []byte(c.body))
		if w.Code == 200 || !strings.Contains(w.Body.String(), "damaged") {
			t.Errorf("%s on a damaged file must be refused and say why: %d %s", c.route, w.Code, w.Body.String())
		}
	}
	if b, _ := os.ReadFile(s.SettingsPath); string(b) != damaged {
		t.Fatalf("the damaged file was replaced: %q", b)
	}
	// once the file is removed the settings work again
	os.Remove(s.SettingsPath)
	if w := do(t, s, "POST", "http://127.0.0.1/api/settings", loop, hdr, []byte(`{"autoRefillOnOpen":true}`)); w.Code != 200 {
		t.Fatalf("after the damaged file is gone: %d %s", w.Code, w.Body.String())
	}
}
