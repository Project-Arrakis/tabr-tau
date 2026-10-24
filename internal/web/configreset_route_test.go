package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

const resetRouteINI = "[/Script/DuneSandbox.UserServerCustomSettings]\r\nDifficultyLevel=Custom\r\nGatheringAmount=4.000000\r\nCraftingCost=0.500000\r\nLootRespawnSpeed=1.000000\r\n"

func TestConfigResetRoutes(t *testing.T) {
	cfg := t.TempDir()
	ini := filepath.Join(cfg, "ServerCustomSettings.ini")
	os.WriteFile(ini, []byte(resetRouteINI), 0o644)
	s := New(testsave.Player(t), config.Dir{Path: cfg})
	logDir := t.TempDir()
	s.KeepPendingEdits(logDir)

	// the file answer lists defaults and what a reset of all would change
	w := do(t, s, "GET", "http://127.0.0.1/api/config/file?name=ServerCustomSettings.ini", loop, map[string]string{"Host": "127.0.0.1:8090", "Cookie": "tabr_session=" + s.token}, nil)
	var f struct {
		Defaults []struct {
			Key     string `json:"key"`
			Known   bool   `json:"known"`
			Default string `json:"default"`
		} `json:"defaults"`
		ResetCounts struct{ Change, NoDefault int } `json:"resetCounts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &f); err != nil || len(f.Defaults) != 4 || f.ResetCounts.Change != 2 || f.ResetCounts.NoDefault != 1 {
		t.Fatalf("%v %s", err, w.Body.String())
	}

	// one setting
	w = postEdit(t, s, "/api/config/reset", `{"name":"ServerCustomSettings.ini","line":2,"key":"GatheringAmount"}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["ok"] != true || m["result"].(map[string]any)["changed"] != float64(1) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	// a stale page: the count it showed is no longer true
	w = postEdit(t, s, "/api/config/reset", `{"name":"ServerCustomSettings.ini","expect":2}`)
	if w.Code == 200 || !strings.Contains(w.Body.String(), "reload") {
		t.Fatalf("a stale count must be refused: %d %s", w.Code, w.Body.String())
	}
	// all
	w = postEdit(t, s, "/api/config/reset", `{"name":"ServerCustomSettings.ini","expect":1}`)
	if m = decodeBody(t, w); w.Code != 200 || m["result"].(map[string]any)["changed"] != float64(1) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if got, _ := os.ReadFile(ini); string(got) != strings.NewReplacer("GatheringAmount=4.000000", "GatheringAmount=1.000000", "CraftingCost=0.500000", "CraftingCost=1.000000").Replace(resetRouteINI) {
		t.Fatalf("file: %q", got)
	}
	// a key with no verified default is refused
	w = postEdit(t, s, "/api/config/reset", `{"name":"ServerCustomSettings.ini","line":1,"key":"DifficultyLevel"}`)
	if w.Code == 200 || !strings.Contains(w.Body.String(), "default unknown") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	// the write log has one line for each of the two resets
	b, err := os.ReadFile(filepath.Join(logDir, "write-log.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "reset 1 setting(s) in ServerCustomSettings.ini to the game's default") {
		t.Fatalf("write log: %q", b)
	}
	if strings.Contains(string(b), "4.0") || strings.Contains(string(b), "0.5") {
		t.Error("the log holds no values")
	}
}
