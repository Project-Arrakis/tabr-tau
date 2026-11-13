// Package hygiene holds a test over the whole public repository: nothing tracked in it may name the maintainers' private tooling.
package hygiene

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// privateTokens are names that must not appear in any text file of this repository (audit 2026-11-13, D11-1). They are kept as
// SHA-256 digests of the lower-case token, so this file does not name what it keeps out. A token is a run of letters, digits,
// hyphens and underscores, so a file name with an extension or a possessive is caught by its stem.
var privateTokens = map[string]string{
	"f3d5bd2b82bcb38fd930d9baeb8ca4438470c53b3e31b70f947c85fff36f5e72": "a private program's name",
	"9d0b5d30f807e1081a0a75a3e5fa0b29ee086315ad3d42e963a4782b8ef2abfe": "a private program's name",
	"bb5865066104da838c54688c4450dd4069e65b065048f59e5a31fab81e228180": "a private program's name",
	"463bbf36edbb88cc27a7138ee858b33da2874e02f1038581a1ce7698d069eb4f": "a file of a private program",
	"603804cfd7a43745f9dc3053241ba70591aa5784d455fad1c40233dcb191e8e2": "a way of reading the live game's memory",
}

var tokenRE = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9_-]*`)

var textExts = map[string]bool{
	".md": true, ".go": true, ".js": true, ".mjs": true, ".html": true, ".css": true, ".yml": true, ".yaml": true, ".json": true,
	".txt": true, ".sh": true, ".ps1": true, ".toml": true, ".mod": true, ".sum": true, ".sql": true, ".tsv": true, ".svg": true,
	".py": true, ".ini": true, ".cfg": true, ".gitignore": true, ".gitattributes": true,
}

// leak returns the first private token in s, or "".
func leak(s string) (token, what string) {
	for _, w := range tokenRE.FindAllString(s, -1) {
		sum := sha256.Sum256([]byte(strings.ToLower(w)))
		if what, ok := privateTokens[hex.EncodeToString(sum[:])]; ok {
			return w, what
		}
	}
	return "", ""
}

func TestLeakFindsWhatItKeepsOut(t *testing.T) {
	// the digests are of words this file does not spell, so the check is proved against a word built from parts
	name := strings.Join([]string{"tabr", "trainer"}, "-")
	for _, s := range []string{name, "see " + name + ".exe", "`" + name + "`'s repo", strings.ToUpper(name)} {
		if tok, _ := leak(s); tok == "" {
			t.Errorf("leak(%q) found nothing", s)
		}
	}
	for _, s := range []string{"", "a tabr-tau editor", "the NPC trainers of the map"} {
		if tok, _ := leak(s); tok != "" {
			t.Errorf("leak(%q) = %q, want nothing", s, tok)
		}
	}
}

// No tracked text file of the repository (docs, code and comments, web files, workflows, issue templates, changelogs) names a private tool.
func TestNoPrivateToolIsNamedInThePublicRepository(t *testing.T) {
	root := filepath.Join("..", "..")
	checked := 0
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if !textExts[ext] && !textExts[strings.ToLower(d.Name())] {
			return nil
		}
		if info, err := d.Info(); err != nil || info.Size() > 8<<20 {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		checked++
		rel, _ := filepath.Rel(root, p)
		if tok, what := leak(string(b)); tok != "" {
			t.Errorf("%s names %q (%s); the public repository never names a private tool", filepath.ToSlash(rel), tok, what)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 100 {
		t.Fatalf("only %d files were checked; the walk is wrong", checked)
	}
}
