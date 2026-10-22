package web

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// steamLike builds a synthetic 17-digit id at run time so no id-shaped literal is committed.
func steamLike() string { return "7656119" + "8000000001" }

func editedServer(t *testing.T) *Server {
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

// The poll endpoint is cheap and complete: it carries everything the header and the save pane need.
func TestSaveStateEndpoint(t *testing.T) {
	s := editedServer(t)
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

func postEdit(t *testing.T, s *Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	h := map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token, "Content-Type": "application/json"}
	return do(t, s, "POST", "http://127.0.0.1"+path, loop, h, []byte(body))
}

func solariOf(t *testing.T, path string) int64 {
	t.Helper()
	chk, err := save.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer chk.Close()
	r, err := chk.One(`select stack_size n from items where template_id='SolarisCoin'`)
	if err != nil || r == nil {
		t.Fatal(err)
	}
	return r["n"].(int64)
}

// There is no step where a person saves: an edit is in the game's file when the request returns.
func TestAnEditIsWrittenToTheGamesFileAsItIsMade(t *testing.T) {
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	before := solariOf(t, s.Save.Path)
	if w := postEdit(t, s, "/api/player/solari", `{"amount":250}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if s.Save.Dirty() || len(s.Save.Pending()) != 0 {
		t.Fatal("the edit is written, not left pending")
	}
	if got := solariOf(t, s.Save.Path); got != before+250 {
		t.Fatalf("the file holds %d, want %d", got, before+250)
	}
	w := do(t, s, "GET", "http://127.0.0.1/api/save/state", loop, map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}, nil)
	var st struct {
		Dirty     bool `json:"dirty"`
		LastWrite *struct {
			Saved  bool   `json:"saved"`
			Queued int    `json:"queued"`
			Backup string `json:"backup"`
		} `json:"lastWrite"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil || st.Dirty || st.LastWrite == nil || !st.LastWrite.Saved || st.LastWrite.Queued != 0 || st.LastWrite.Backup == "" {
		t.Fatalf("the state says what was written: %v %s", err, w.Body.String())
	}
}

// While the game is running the edit is queued, the state says why, and it is written by itself when the file can be.
func TestAnEditWaitsForTheGameAndIsWrittenWhenItCloses(t *testing.T) {
	restore := save.ActiveSessionForTests()
	s := New(testsave.Player(t), config.Dir{Path: t.TempDir()})
	before := solariOf(t, s.Save.Path)
	if w := postEdit(t, s, "/api/player/solari", `{"amount":250}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if !s.Save.Dirty() || solariOf(t, s.Save.Path) != before {
		t.Fatal("the edit waits and the file is untouched")
	}
	w := do(t, s, "GET", "http://127.0.0.1/api/save/state", loop, map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}, nil)
	var st struct {
		Pending   []string `json:"pending"`
		LastWrite *struct {
			Saved  bool   `json:"saved"`
			Queued int    `json:"queued"`
			Why    string `json:"why"`
		} `json:"lastWrite"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil || len(st.Pending) != 1 || st.LastWrite == nil || st.LastWrite.Saved || st.LastWrite.Queued != 1 || !strings.Contains(st.LastWrite.Why, "single-player") {
		t.Fatalf("the state says the edit is queued and why: %v %s", err, w.Body.String())
	}
	restore() // the game was closed
	stop := make(chan struct{})
	defer close(stop)
	go s.RunAutosave(stop)
	deadline := time.Now().Add(10 * time.Second)
	for s.Save.Dirty() && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if s.Save.Dirty() || solariOf(t, s.Save.Path) != before+250 {
		t.Fatalf("the waiting edit is written once the file can be (dirty=%v)", s.Save.Dirty())
	}
	if _, err := os.Stat(s.Save.Path); err != nil {
		t.Fatal(err)
	}
}
