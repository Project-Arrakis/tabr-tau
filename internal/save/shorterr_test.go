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

// The folders and the account name that must never survive; every folder name below holds one of these or is one of them.
var shortErrSecrets = []string{"Some User", "AppData", "Profiles", "srv", "nas", "share"}

func shortErrCheck(t *testing.T, name string, err error, keep ...string) {
	t.Helper()
	got := shortErr(err)
	for _, s := range shortErrSecrets {
		if strings.Contains(got, s) {
			t.Fatalf("%s: %q still holds %q", name, got, s)
		}
	}
	for _, k := range keep {
		if !strings.Contains(got, k) {
			t.Fatalf("%s: %q lost %q", name, got, k)
		}
	}
}

// The error texts are built from parts, in the forms of both systems, so that the test means the same on every system the editor is built on and no profile path is written out in it.
func TestShortErrCutsTheFoldersOfEveryFormAndKeepsBothFileNames(t *testing.T) {
	win := func(parts ...string) string { return strings.Join(parts, `\`) }
	nix := func(parts ...string) string { return "/" + strings.Join(parts, "/") }
	fwdNet := func(parts ...string) string { return "//" + strings.Join(parts, "/") }
	drive := func(file string) string { return win("C:", "Profiles", "Some User", "AppData", file) }
	unc := func(file string) string { return win(`\\nas`, "share", "Some User", "AppData", file) }
	verbatimUNC := func(file string) string { return win(`\\?`, "UNC", "nas", "share", "Some User", "AppData", file) }
	unix := func(file string) string { return nix("srv", "Some User", "AppData", file) }
	fwd := func(file string) string { return fwdNet("nas", "share", "Some User", "AppData", file) }
	for _, c := range []struct {
		name string
		text string
		keep []string
	}{
		{"drive, one path", "cannot write " + drive("a.json") + ": busy", []string{"a.json", "busy"}},
		{"drive, two paths", "rename " + drive("a.tmp") + " " + drive("b.json") + ": denied", []string{"a.tmp", "b.json", "denied"}},
		{"verbatim drive", `open \\?\` + drive("a.json") + ": busy", []string{"a.json"}},
		{"network, one path", "open " + unc("a.json") + ": busy", []string{"a.json"}},
		{"network, two paths", "rename " + unc("a.tmp") + " " + unc("b.json") + ": denied", []string{"a.tmp", "b.json", "denied"}},
		{"verbatim network", "rename " + verbatimUNC("a.tmp") + " " + verbatimUNC("b.json") + ": denied", []string{"a.tmp", "b.json", "denied"}},
		{"network with forward slashes", "rename " + fwd("a.tmp") + " " + fwd("b.json") + ": denied", []string{"a.tmp", "b.json", "denied"}},
		{"unix, one path", "open " + unix("a.json") + ": no such file or directory", []string{"a.json", "no such file or directory"}},
		{"unix, two paths with a space in the folder", "rename " + unix("a.tmp") + " " + unix("b.json") + ": no such file or directory", []string{"a.tmp", "b.json", "no such file or directory"}},
		{"unix, quoted", `open "` + unix("a.json") + `": busy`, []string{`"a.json"`, "busy"}},
		{"a windows path and a unix path", "rename " + drive("a.tmp") + " " + unix("b.json") + ": denied", []string{"a.tmp", "b.json", "denied"}},
	} {
		shortErrCheck(t, c.name, errors.New(c.text), c.keep...)
		shortErrCheck(t, c.name+" (wrapped)", fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", errors.New(c.text))), append([]string{"outer", "inner"}, c.keep...)...)
	}
}

// What is not a path of a file is left as it is: a URL, and prose with a slash between blanks.
func TestShortErrLeavesUrlsAndProseAlone(t *testing.T) {
	for _, s := range []string{
		"https://example.com/a/b",
		"fetch https://example.com/a/b failed",
		"ftp://host/dir/file: refused",
		"5 / 10 items / done",
		"read 3/4 of the rows",
		"no folders in this one",
	} {
		if got := shortErr(errors.New(s)); got != s {
			t.Fatalf("%q was changed to %q", s, got)
		}
	}
}

// The real errors of the system the test runs on: a failed rename is a *os.LinkError and names two paths, in a folder whose name holds a space.
func TestShortErrCutsTheFoldersOfRealFileErrors(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Some User", "AppData")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	secrets := append([]string{filepath.Dir(dir)}, "Some User", "AppData")
	check := func(name string, err error, keep ...string) {
		t.Helper()
		got := shortErr(err)
		for _, s := range secrets {
			if strings.Contains(got, s) {
				t.Fatalf("%s: %q still holds %q", name, got, s)
			}
		}
		for _, k := range keep {
			if !strings.Contains(got, k) {
				t.Fatalf("%s: %q lost %q", name, got, k)
			}
		}
	}
	rerr := os.Rename(filepath.Join(dir, "paid.tmp"), filepath.Join(dir, "paid.json"))
	var le *os.LinkError
	if !errors.As(rerr, &le) {
		t.Fatalf("not a link error: %T %v", rerr, rerr)
	}
	check("link error", rerr, "paid.tmp", "paid.json")
	check("wrapped link error", fmt.Errorf("the record could not be written: %w", rerr), "the record could not be written", "paid.tmp", "paid.json")
	_, oerr := os.Open(filepath.Join(dir, "missing.json"))
	check("path error", oerr, "missing.json")
	check("wrapped path error", fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", oerr)), "outer", "inner", "missing.json")
}
