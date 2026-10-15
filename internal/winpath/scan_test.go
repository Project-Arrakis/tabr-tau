package winpath

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// No shipped code starts a program by a bare name (which is searched for in PATH): the programs the editor starts are system programs and
// are named by full path through System32 (SEC-8 of the 2026-10-15 audit).
func TestNoProgramIsStartedByABareName(t *testing.T) {
	bare := regexp.MustCompile(`exec\.Command(Context)?\((ctx, )?"[^"/\\]*"`)
	root := filepath.Join("..", "..")
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
		checked++
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if m := bare.Find(b); m != nil {
			rel, _ := filepath.Rel(root, path)
			t.Errorf("%s starts a program by a bare name: %s", rel, m)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 20 {
		t.Fatalf("only %d files were checked: the walk is not seeing the tree", checked)
	}
}
