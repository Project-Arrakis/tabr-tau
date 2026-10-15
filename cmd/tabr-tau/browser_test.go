package main

import (
	"path/filepath"
	"testing"
)

// The browser is opened through rundll32 started by its full path in the system folder, not found through PATH (SEC-8 of the 2026-10-15 audit).
func TestBrowserIsOpenedThroughTheFullRundll32Path(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SystemRoot", root)
	cmd := browserCommand("http://127.0.0.1:8090/?boot=x")
	want := filepath.Join(root, "System32", "rundll32.exe")
	if cmd.Path != want || cmd.Args[0] != want {
		t.Fatalf("rundll32 must be started by %q, got Path %q Args[0] %q", want, cmd.Path, cmd.Args[0])
	}
	if len(cmd.Args) != 3 || cmd.Args[1] != "url.dll,FileProtocolHandler" || cmd.Args[2] != "http://127.0.0.1:8090/?boot=x" {
		t.Fatalf("arguments: %v", cmd.Args)
	}
}
