package save

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The hooks that fake the game's state are exported from this package so tests elsewhere can use them. Nothing but test code may call
// them: a call from shipped code would switch the "is the game running" guard off (QA-15 of the 2026-10-15 audit).
func TestTestHooksAreNotCalledByProductionCode(t *testing.T) {
	root := filepath.Join("..", "..")
	allowed := map[string]bool{
		filepath.Join("internal", "save", "testhook.go"):            true, // defines them
		filepath.Join("internal", "testsave", "testsave.go"):        true, // the shared fixture package, imported by tests only
		filepath.Join("internal", "save", "testhook_guard_test.go"): true,
	}
	hooks := []string{"NoGameRunningForTests(", "ActiveSessionForTests(", "UntrustedLocationForTests("}
	checked := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".claude", "node_modules", "web-tests":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if allowed[rel] {
			return nil
		}
		checked++
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, h := range hooks {
			if strings.Contains(string(b), h) {
				t.Errorf("%s calls %s: test hooks must not be used by shipped code", rel, strings.TrimSuffix(h, "("))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 20 {
		t.Fatalf("only %d production files were checked: the walk is not seeing the tree", checked)
	}
}

// testsave is a non-test package, so check who imports it: only test files may.
func TestTestsaveIsImportedByTestsOnly(t *testing.T) {
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == ".claude" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.Contains(path, filepath.Join("internal", "testsave")) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), "tabr-tau/internal/testsave") {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s imports internal/testsave, which switches the game check off", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
