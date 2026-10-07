package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/internal/testsave"
)

// capture runs fn with stdout redirected and returns what it printed.
func capture(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := fn()
	w.Close()
	os.Stdout = old
	b, _ := io.ReadAll(r)
	return string(b), err
}

func writeSave(t *testing.T, edit string) string {
	t.Helper()
	s := testsave.Player(t)
	if edit != "" {
		testsave.Exec(t, s, edit)
	}
	if _, err := s.Commit(true); err != nil && edit != "" {
		t.Fatal(err)
	}
	return s.Path
}

func TestDiffCommand(t *testing.T) {
	a := writeSave(t, "")
	b := writeSave(t, `update items set stack_size=5, template_id='Item123456789012345678' where id=10`)
	out, err := capture(t, func() error { return runDiff([]string{a, b}) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## items") || !strings.Contains(out, "stack_size") {
		t.Fatalf("output lacks the items diff:\n%s", out)
	}
	if strings.Contains(out, "123456789012345678") {
		t.Fatalf("long ids must be redacted by default:\n%s", out)
	}
	// flags may follow the two paths, and --no-redact shows the value
	out, err = capture(t, func() error { return runDiff([]string{a, b, "--no-redact"}) })
	if err != nil || !strings.Contains(out, "123456789012345678") || !strings.Contains(out, "UNREDACTED") {
		t.Fatalf("no-redact output wrong (err=%v):\n%s", err, out)
	}
	// ignore flags from the Python tool are accepted
	out, err = capture(t, func() error { return runDiff([]string{a, b, "--ignore-tables", "items", "--float-eps", "1e-3"}) })
	if err != nil || strings.Contains(out, "## items") {
		t.Fatalf("--ignore-tables items must hide the items table (err=%v):\n%s", err, out)
	}
	if err := runDiff([]string{a}); err == nil {
		t.Fatal("one path must be a usage error")
	}
	// inputs are never modified
	before, _ := os.ReadFile(a)
	capture(t, func() error { return runDiff([]string{a, b}) })
	after, _ := os.ReadFile(a)
	if string(before) != string(after) {
		t.Fatal("diff modified its input")
	}
}
