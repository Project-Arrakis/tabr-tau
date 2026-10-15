package winpath

import (
	"path/filepath"
	"testing"
)

func TestSystem32IsUnderTheSystemRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SystemRoot", root)
	t.Setenv("windir", "ignored")
	if got, want := System32("tasklist.exe"), filepath.Join(root, "System32", "tasklist.exe"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	t.Setenv("SystemRoot", "")
	other := t.TempDir()
	t.Setenv("windir", other)
	if got, want := System32("rundll32.exe"), filepath.Join(other, "System32", "rundll32.exe"); got != want {
		t.Fatalf("windir is the second choice: got %q, want %q", got, want)
	}
	t.Setenv("windir", "")
	if got, want := System32("x.exe"), filepath.Join(`C:\Windows`, "System32", "x.exe"); got != want {
		t.Fatalf("the last resort is C:\\Windows: got %q, want %q", got, want)
	}
}
