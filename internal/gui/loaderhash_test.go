package gui

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The Windows exe embeds WebView2Loader.dll files that go-winloader loads from memory, bypassing the operating system's signature and
// application-control checks. go.sum already pins the module as a whole; this makes a change to the DLLs themselves a visible event: a
// dependency bump that alters one fails here until the new hash is reviewed and written down (CLD-6 of the 2026-10-15 audit).
func TestEmbeddedWebView2LoadersAreTheReviewedOnes(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/jchv/go-webview2").Output()
	if err != nil {
		t.Skipf("cannot locate the go-webview2 module: %v", err)
	}
	dir := strings.TrimSpace(string(out))

	want := map[string]string{}
	f, err := os.Open("webview2-loader.sha256")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		h, name, ok := strings.Cut(line, "  ")
		if !ok || len(h) != 64 {
			t.Fatalf("malformed line in webview2-loader.sha256: %q", line)
		}
		want[name] = h
	}

	got := map[string]string{}
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".dll") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		rel, _ := filepath.Rel(dir, path)
		got[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatalf("no DLL found under %s: the walk is not seeing the module", dir)
	}

	var names []string
	for n := range got {
		names = append(names, n)
	}
	for n := range want {
		if _, in := got[n]; !in {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		switch {
		case want[n] == "":
			t.Errorf("%s is embedded but not in webview2-loader.sha256 (sha256 %s): review it, then add the line", n, got[n])
		case got[n] == "":
			t.Errorf("%s is listed in webview2-loader.sha256 but the module no longer has it", n)
		case want[n] != got[n]:
			t.Errorf("%s changed: reviewed %s, now %s. Compare it with Microsoft's WebView2 SDK of the same version, then update webview2-loader.sha256", n, want[n], got[n])
		}
	}
}
