package save

import (
	"context"
	"path/filepath"
	"testing"
)

// The process check starts tasklist by its full path in the system folder, so a tasklist.exe found earlier in PATH cannot answer for it (SEC-8).
func TestTasklistIsStartedByItsFullPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SystemRoot", root)
	cmd := tasklistCommand(context.Background())
	if want := filepath.Join(root, "System32", "tasklist.exe"); cmd.Path != want || cmd.Args[0] != want {
		t.Fatalf("tasklist must be started by %q, got Path %q Args[0] %q", want, cmd.Path, cmd.Args[0])
	}
	if cmd.Args[1] != "/FI" || cmd.Args[len(cmd.Args)-1] != "CSV" {
		t.Fatalf("arguments changed: %v", cmd.Args)
	}
}
