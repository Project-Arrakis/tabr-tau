package buildinfo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveStamped(t *testing.T) {
	i := resolve("0.14.2", "317", "a1b2c3d", "2026-11-16", "ffffffffffffffffffffffffffffffffffffffff", "windows/amd64")
	if i.Version != "0.14.2" || i.Build != "317" || i.Commit != "a1b2c3d" || i.Date != "2026-11-16" || i.Channel != "stable" || i.Arch != "windows/amd64" || i.Dev {
		t.Fatalf("stamped release: %+v", i)
	}
	if got, want := i.Line(), "v0.14.2 (build 317, a1b2c3d)"; got != want {
		t.Errorf("Line() = %q, want %q", got, want)
	}
	if got := i.Label(); got != "v0.14.2" {
		t.Errorf("Label() = %q", got)
	}
}

func TestResolveLongCommitIsShortened(t *testing.T) {
	i := resolve("1.0.0", "5", "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678", "2026-01-02", "", "x")
	if i.Commit != "a1b2c3d" {
		t.Errorf("commit = %q", i.Commit)
	}
}

func TestResolveDevelopmentBuilds(t *testing.T) {
	for _, v := range []string{"0.14.2+5", "0.14.2+5.dirty", "0.14.2+dirty", "1.0.0-rc.1", "0.0.0+437"} {
		i := resolve(v, "322", "e4f5a6b", "2026-11-16", "", "x")
		if i.Version != v || !i.Dev {
			t.Errorf("%q: %+v, want a development build with the same version", v, i)
		}
		if i.Label() != "v"+v {
			t.Errorf("%q: label %q", v, i.Label())
		}
	}
}

func TestResolveUnstampedReadsTheRevisionGoRecorded(t *testing.T) {
	i := resolve("", "", "", "", "A1B2C3D4E5F60718293A4B5C6D7E8F9012345678", "x")
	if i.Version != "dev" || !i.Dev || i.Build != "" || i.Date != "" {
		t.Fatalf("unstamped: %+v", i)
	}
	if i.Commit != "a1b2c3d" {
		t.Errorf("commit = %q, want the first 7 characters of the recorded revision", i.Commit)
	}
	if got, want := i.Line(), "dev (a1b2c3d)"; got != want {
		t.Errorf("Line() = %q, want %q", got, want)
	}
	if got := resolve("", "", "", "", "", "x").Line(); got != "dev" {
		t.Errorf("nothing known: Line() = %q, want dev", got)
	}
	if got := resolve("", "", "", "", "", "x").Label(); got != "dev" {
		t.Errorf("Label() = %q, want dev", got)
	}
}

// A stamp that does not have the expected shape is never shown: not as markup, not as a path, not as an oversized string.
func TestResolveThrowsAwayWhatIsNotAVersion(t *testing.T) {
	bad := []string{"<b>1.0.0</b>", "1.0", "v1.0.0", "1.0.0 ", "1.0.0\n", "latest", "1.0.0+" + strings.Repeat("a", 80), "../../x", "1.0.0;rm"}
	for _, v := range bad {
		i := resolve(v, "x", "<script>", "yesterday", "not-hex", "x")
		if i.Version != "dev" || i.Build != "" || i.Commit != "" || i.Date != "" {
			t.Errorf("%q: %+v, want everything thrown away", v, i)
		}
	}
}

func TestIsDev(t *testing.T) {
	for v, want := range map[string]bool{"dev": true, "": true, "0.14.2": false, "0.14.2+5": true, "0.14.2+dirty": true, "1.0.0-rc.1": true, "10.20.30": false} {
		if got := IsDev(v); got != want {
			t.Errorf("IsDev(%q) = %v, want %v", v, got, want)
		}
	}
}

// Get on a plain test binary has no stamp: it must not panic and must say it is a development build.
func TestGetUnstampedTestBinary(t *testing.T) {
	i := Get()
	if i.Version != "dev" || !i.Dev || i.Channel != "stable" || i.Arch == "" {
		t.Errorf("Get() = %+v", i)
	}
}

func TestReadNotes(t *testing.T) {
	dir := t.TempDir()
	if text, cut := ReadNotes(dir); text != "" || cut {
		t.Errorf("no file: %q %v", text, cut)
	}
	if text, _ := ReadNotes(""); text != "" {
		t.Errorf("no folder: %q", text)
	}
	write := func(b []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, "notes.md"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write([]byte("## [0.1.0]\r\n\r\n- First release.\r\n"))
	if text, cut := ReadNotes(dir); text != "## [0.1.0]\n\n- First release." || cut {
		t.Errorf("plain notes: %q %v", text, cut)
	}
	// control characters and text-direction overrides are dropped, tabs and line breaks stay, markup stays as plain text
	write([]byte("a\x00b\x1b[31m\tc‮d\n<script>x</script>\xff"))
	text, _ := ReadNotes(dir)
	if strings.ContainsAny(text, "\x00\x1b‮") || !strings.Contains(text, "\tc") || !strings.Contains(text, "<script>x</script>") || !strings.Contains(text, "�") {
		t.Errorf("hostile notes: %q", text)
	}
	// longer than the limit: cut, and said so
	write([]byte(strings.Repeat("x", notesMax+500)))
	if text, cut := ReadNotes(dir); len(text) != notesMax || !cut {
		t.Errorf("long notes: %d bytes, cut=%v", len(text), cut)
	}
	// exactly the limit is whole
	write([]byte(strings.Repeat("y", notesMax)))
	if text, cut := ReadNotes(dir); len(text) != notesMax || cut {
		t.Errorf("limit-sized notes: %d bytes, cut=%v", len(text), cut)
	}
}

func TestReadNotesIgnoresAFolderOfThatName(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "notes.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if text, cut := ReadNotes(dir); text != "" || cut {
		t.Errorf("folder: %q %v", text, cut)
	}
}
