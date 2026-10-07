package web

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// steamLike builds a synthetic 17-digit id at run time so no id-shaped literal is committed.
func steamLike() string { return "7656119" + "8000000001" }

func reviewServer(t *testing.T) *Server {
	t.Helper()
	s := testsave.Player(t)
	if _, err := s.Mutate("rename the account", func(m *save.Mut) error {
		_, err := m.Exec(`update accounts set funcom_id='FUNCOM-CHANGED-` + steamLike() + `' where id=1`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Mutate("edit item 10 of account "+steamLike(), func(m *save.Mut) error {
		_, err := m.Exec(`update items set stack_size=5 where id=10`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return New(s, config.Dir{Path: t.TempDir()})
}

func reviewGet(t *testing.T, s *Server, query string, hdr map[string]string) (int, string) {
	t.Helper()
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
	for k, v := range hdr {
		h[k] = v
	}
	w := do(t, s, "GET", "http://127.0.0.1/api/save/review"+query, loop, h, nil)
	return w.Code, w.Body.String()
}

func TestReviewEndpointShapeAndRedaction(t *testing.T) {
	s := reviewServer(t)
	code, body := reviewGet(t, s, "", nil)
	if code != 200 {
		t.Fatalf("status %d: %s", code, body)
	}
	var out struct {
		Dirty bool `json:"dirty"`
		Ops   []struct {
			Desc string `json:"desc"`
			Rows int64  `json:"rows"`
		} `json:"ops"`
		Diff struct {
			Redacted bool `json:"redacted"`
			Tables   []struct {
				Name        string `json:"name"`
				NumModified int    `json:"numModified"`
			} `json:"tables"`
		} `json:"diff"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, body)
	}
	if !out.Dirty || len(out.Ops) != 2 || !out.Diff.Redacted || len(out.Diff.Tables) != 2 {
		t.Fatalf("unexpected shape: %+v\n%s", out, body)
	}
	for _, leak := range []string{"FUNCOM-CHANGED", steamLike(), "FUNCOM-TEST"} {
		if strings.Contains(body, leak) {
			t.Errorf("default review leaks %q:\n%s", leak, body)
		}
	}
	// the opt-in shows values
	if _, raw := reviewGet(t, s, "?unredacted=1", nil); !strings.Contains(raw, "FUNCOM-CHANGED") {
		t.Error("unredacted=1 must show values")
	}
}

func TestReviewEndpointLimitIsBoundedAndGated(t *testing.T) {
	s := reviewServer(t)
	for _, q := range []string{"?limit=-7", "?limit=0", "?limit=999999999", "?limit=abc"} {
		if code, body := reviewGet(t, s, q, nil); code != 200 || !strings.Contains(body, `"tables"`) {
			t.Errorf("%s: %d %s", q, code, body)
		}
	}
	// no session, a foreign Host, a cross-site fetch: all refused like every other API route
	w := do(t, s, "GET", "http://127.0.0.1/api/save/review", loop, map[string]string{"Host": "127.0.0.1:8090"}, nil)
	if w.Code != 403 {
		t.Errorf("no cookie: %d", w.Code)
	}
	if code, _ := reviewGet(t, s, "", map[string]string{"Host": "evil.com"}); code == 200 {
		t.Error("foreign Host must be refused")
	}
	if code, _ := reviewGet(t, s, "?unredacted=1", map[string]string{"Sec-Fetch-Site": "cross-site"}); code == 200 {
		t.Error("a cross-site request must not get the unredacted review")
	}
}
