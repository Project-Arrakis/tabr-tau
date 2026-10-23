package web

import (
	"os"
	"strings"
	"testing"
)

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile("../../" + rel)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r", "")
}

// Review 2026-10-23 B4: text that describes something the editor no longer has or no longer does.
func TestNoStaleTextAboutSaveButtonsTabNamesOrTokens(t *testing.T) {
	if s := repoFile(t, "cmd/tabr-tau/main.go"); strings.Contains(s, "pressed Save") {
		t.Error("the crash box mentions a Save button; edits are written as they are made")
	}
	readme := repoFile(t, "README.md")
	if strings.Contains(readme, "Live Map") {
		t.Error("README names the Live Map; the tab is called Saved map")
	}
	if strings.Contains(readme, "per-run token on every API call") {
		t.Error("README calls the page guard a token on every call; it is a session cookie")
	}
	if inv := repoFile(t, "docs/design/feature-inventory.md"); !strings.Contains(inv, "/api/save/review") || !strings.Contains(inv, "Historical") {
		t.Error("the feature inventory still lists routes that are gone and must say that part is historical")
	}
}

// The [Unreleased] part of the changelog has each entry once, and one entry to a line.
func TestChangelogUnreleasedHasNoDuplicateOrRunTogetherEntries(t *testing.T) {
	cl := repoFile(t, "CHANGELOG.md")
	i := strings.Index(cl, "## [Unreleased]")
	j := strings.Index(cl, "## [0.0.0]")
	if i < 0 || j < i {
		t.Fatal("the changelog layout changed; this test looks for [Unreleased] up to [0.0.0]")
	}
	seen := map[string]int{}
	for n, line := range strings.Split(cl[i:j], "\n") {
		if strings.Contains(line, ".- **") {
			t.Errorf("line %d of [Unreleased] runs two entries together", n+1)
		}
		if strings.HasPrefix(line, "- ") {
			if prev, dup := seen[line]; dup {
				t.Errorf("an entry is in [Unreleased] twice (lines %d and %d): %.80s", prev, n+1, line)
			}
			seen[line] = n + 1
		}
	}
}
