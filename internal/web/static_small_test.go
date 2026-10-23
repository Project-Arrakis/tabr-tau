package web

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// UX-37 / #266: under 720 px the header and the banner stop being sticky (200 px of an 800 px screen at 390 px), and the sub-tab rows scroll sideways
// instead of growing. A browser engine is needed to measure the result; this test keeps the rules from being dropped.
func TestSmallScreensUnstickTheHeaderAndTheBanner(t *testing.T) {
	b, err := os.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	css := strings.ReplaceAll(string(b), "\r", "")
	m := regexp.MustCompile(`@media \(max-width:720px\)\{([\s\S]*?)\n\}`).FindStringSubmatch(css)
	if m == nil {
		t.Fatal("no @media (max-width:720px) block")
	}
	for _, want := range []string{"header{position:static", "#gamewarn.warn,#gamewarn.info{position:static", ".sub{flex-wrap:nowrap;overflow-x:auto"} {
		if !strings.Contains(m[1], want) {
			t.Errorf("the small-screen rules lack %q", want)
		}
	}
	// the small-screen block comes last so that it beats the rules it overrides at equal specificity
	if i := strings.Index(css, "@media (max-width:720px)"); i < strings.Index(css, "#gamewarn.warn,#gamewarn.info{border-radius") {
		t.Error("the small-screen block must come after the banner rules it overrides")
	}
}
