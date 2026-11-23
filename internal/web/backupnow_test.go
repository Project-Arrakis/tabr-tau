package web

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/config"
	"github.com/Project-Arrakis/tabr-tau/internal/save"
	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// Back up now (#1168): the route takes a backup of the save as it is, answers with its name, size and checksum, lists it as a manual backup, works while the game may be running,
// and writes nothing to the save.
func TestBackUpNowRouteTakesAVerifiedManualBackupAndLeavesTheSaveAlone(t *testing.T) {
	p := testsave.Player(t)
	s := New(p, config.Dir{Path: t.TempDir()})
	before, err := os.ReadFile(p.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer save.ActiveSessionForTests()()
	w := postEdit(t, s, "/api/save/backup-now", `{}`)
	m := decodeBody(t, w)
	if w.Code != 200 || m["ok"] != true || m["outcome"] != nil {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	name, _ := m["name"].(string)
	if !strings.Contains(name, "-manual") || m["size"] != float64(len(before)) || len(m["sha256"].(string)) != 64 {
		t.Fatalf("answer: %s", w.Body.String())
	}
	list := postEditGet(t, s, "/api/save/backups")
	if !strings.Contains(list, `"manual":true`) || !strings.Contains(list, name) {
		t.Fatalf("the list does not show it as manual: %s", list)
	}
	after, _ := os.ReadFile(p.Path)
	if !bytes.Equal(before, after) || s.Save.Dirty() {
		t.Fatal("the route wrote to the save")
	}
}
