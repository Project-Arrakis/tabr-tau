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

func commitPost(t *testing.T, s *Server, body string) (int, string) {
	t.Helper()
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Content-Type": "application/json", "Origin": "http://127.0.0.1:8090"}
	w := do(t, s, "POST", "http://127.0.0.1/api/save/commit", loop, h, []byte(body))
	return w.Code, w.Body.String()
}

func reviewToken(t *testing.T, s *Server) string {
	t.Helper()
	_, body := reviewGet(t, s, "", nil)
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil || out.Token == "" {
		t.Fatalf("no token in review: %v\n%s", err, body)
	}
	return out.Token
}

// Saving is only reachable from the review: no token, a wrong token, and a token for edits that have since changed
// are all refused, and nothing is written.
func TestCommitRequiresTheReviewedToken(t *testing.T) {
	s := reviewServer(t)
	tok := reviewToken(t, s)
	if again := reviewToken(t, s); again != tok {
		t.Fatalf("the token must be stable for unchanged edits: %s vs %s", tok, again)
	}
	for name, body := range map[string]string{"missing": `{}`, "empty": `{"reviewed":""}`, "wrong": `{"reviewed":"0000"}`, "force is ignored": `{"force":true}`} {
		if code, out := commitPost(t, s, body); code == 200 || !strings.Contains(out, "review") {
			t.Errorf("%s: want a refusal mentioning the review, got %d %s", name, code, out)
		}
	}
	if len(s.Save.Ops()) != 2 {
		t.Fatal("a refused save must leave the pending edits alone")
	}
	// an edit made after the review invalidates the token
	if _, err := s.Save.Mutate("one more edit", func(m *save.Mut) error { _, err := m.Exec(`update items set stack_size=6 where id=10`); return err }); err != nil {
		t.Fatal(err)
	}
	if code, out := commitPost(t, s, `{"reviewed":"`+tok+`"}`); code == 200 || !strings.Contains(out, "different from the ones you reviewed") || !strings.Contains(out, `"code":"review_changed"`) {
		t.Fatalf("a stale token must be refused, got %d %s", code, out)
	}
}

// The token is the same for the redacted and unredacted views, and it changes when the edits change.
func TestReviewTokenTracksEdits(t *testing.T) {
	s := reviewServer(t)
	a := reviewToken(t, s)
	_, raw := reviewGet(t, s, "?unredacted=1", nil)
	if !strings.Contains(raw, a) {
		t.Error("the unredacted view must carry the same token")
	}
	if _, err := s.Save.Mutate("another", func(m *save.Mut) error { _, err := m.Exec(`update items set stack_size=7 where id=10`); return err }); err != nil {
		t.Fatal(err)
	}
	if b := reviewToken(t, s); b == a {
		t.Error("the token must change when the edits change")
	}
}

// The reviewed token is accepted and the save is written.
func TestCommitWithTheReviewedTokenSaves(t *testing.T) {
	s := reviewServer(t)
	tok := reviewToken(t, s)
	code, out := commitPost(t, s, `{"reviewed":"`+tok+`"}`)
	if code != 200 || !strings.Contains(out, `"saved":true`) {
		t.Fatalf("want a saved commit, got %d %s", code, out)
	}
	if len(s.Save.Ops()) != 0 {
		t.Error("the pending edits must be cleared after a save")
	}
}

// The poll endpoint is cheap and complete: it carries everything the header and the save pane need.
func TestSaveStateEndpoint(t *testing.T) {
	s := reviewServer(t)
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}
	w := do(t, s, "GET", "http://127.0.0.1/api/save/state", loop, h, nil)
	if w.Code != 200 {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var st struct {
		Path        string   `json:"path"`
		Dirty       bool     `json:"dirty"`
		Pending     []string `json:"pending"`
		GameRunning bool     `json:"gameRunning"`
		ReadOnly    string   `json:"readOnly"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil || !st.Dirty || len(st.Pending) != 2 || st.Path == "" {
		t.Fatalf("unexpected state: %v %+v", err, st)
	}
	if strings.Contains(w.Body.String(), `"tables"`) {
		t.Error("the poll must not scan the tables")
	}
}
