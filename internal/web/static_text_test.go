package web

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Every colour the stylesheet uses must be defined: an undefined variable makes the rule invalid at computed time, so a warning banner is drawn with no background and no border
// (the round-2 skin lost --warn this way).
func TestEveryCSSVariableTheStylesheetUsesIsDefined(t *testing.T) {
	b, err := os.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(b)
	defined := map[string]bool{}
	for _, m := range regexp.MustCompile(`(--[a-z0-9-]+)\s*:`).FindAllStringSubmatch(css, -1) {
		defined[m[1]] = true
	}
	for _, m := range regexp.MustCompile(`var\((--[a-z0-9-]+)`).FindAllStringSubmatch(css, -1) {
		if !defined[m[1]] {
			t.Errorf("%s is used and never defined", m[1])
		}
	}
}

// The editor writes every edit as it is made (or queues it): nothing on the page may still talk about a Save step or a review.
func TestThePageHasNoWordsAboutAReviewOrSaveStep(t *testing.T) {
	for _, f := range []string{"static/app.js", "static/index.html"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{"Save to game", "Review &amp; save", "Review & save", "until you save", "not at Save", "review pane"} {
			if strings.Contains(string(b), bad) {
				t.Errorf("%s still says %q", f, bad)
			}
		}
	}
}
