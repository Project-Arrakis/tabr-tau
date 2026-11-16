package save

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shortErr is what the page, the warning of a write and the log show of a failed file operation: the file names, never the folders (they hold the name of the person's account).

func TestShortErrCutsEveryFolderOfRealAndWrappedFileErrors(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Some User", "AppData")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	secret := []string{"Some User", "AppData", filepath.Dir(dir)}
	noPath := func(name string, err error, want ...string) {
		t.Helper()
		got := shortErr(err)
		for _, s := range secret {
			if strings.Contains(got, s) {
				t.Fatalf("%s: %q still holds %q", name, got, s)
			}
		}
		for _, w := range want {
			if !strings.Contains(got, w) {
				t.Fatalf("%s: %q lost %q", name, got, w)
			}
		}
	}
	// a failed rename is a *os.LinkError and names two paths
	rerr := os.Rename(filepath.Join(dir, "paid.tmp"), filepath.Join(dir, "paid.json"))
	var le *os.LinkError
	if !errors.As(rerr, &le) {
		t.Fatalf("not a link error: %T %v", rerr, rerr)
	}
	noPath("link error", rerr, "paid.tmp", "paid.json")
	noPath("wrapped link error", fmt.Errorf("the record could not be written: %w", rerr), "the record could not be written", "paid.json")
	// a path error
	_, oerr := os.Open(filepath.Join(dir, "missing.json"))
	noPath("path error", oerr, "missing.json")
	noPath("wrapped path error", fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", oerr)), "outer", "inner", "missing.json")
	// an error of another type that quotes paths in its text, in the forms of the systems the editor runs on
	noPath("text with a drive path", errors.New(`cannot write C:\Users\Some User\AppData\Local\tabr\paid.json: busy`), "paid.json", "busy")
	noPath("text with a verbatim path", errors.New(`open \\?\C:\Users\Some User\AppData\paid.json: busy`), "paid.json")
	noPath("text with a network path", errors.New(`open \\nas\share\Some User\AppData\paid.json: busy`), "paid.json")
	noPath("text with a unix path", errors.New("open /home/Some User/AppData/paid.json: busy"), "paid.json")
	noPath("text with two paths", errors.New(`rename C:\Users\Some User\AppData\a.tmp C:\Users\Some User\AppData\b.json: denied`), "a.tmp", "b.json", "denied")
	if got := shortErr(errors.New("no folders in this one")); got != "no folders in this one" {
		t.Fatalf("a message with no path is not changed: %q", got)
	}
}
