package web

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/design"
)

func luminance(hex string) float64 {
	hex = strings.TrimPrefix(hex, "#")
	ch := func(i int) float64 {
		v, _ := strconv.ParseUint(hex[i:i+2], 16, 8)
		c := float64(v) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(0) + 0.7152*ch(2) + 0.0722*ch(4)
}

func contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// UX-41: text drawn in the muted and the "ok" colours must reach 4.5:1 on every surface of the light theme (the muted colour on the beige tiles was 4.08:1). The colours are the
// design module's Dayside, for every look it has (#570).
func TestLightThemeTextColoursReachFourPointFive(t *testing.T) {
	for _, look := range design.Looks() {
		_, light := lookTokens(t, look)
		tok := editorTheme(t, light)
		for _, text := range []string{"--mut", "--ok", "--bad", "--acc2"} {
			for _, bg := range []string{"--bg", "--panel", "--panel2"} {
				if tok[text] == "" || tok[bg] == "" {
					t.Fatalf("%s: %s or %s is not defined in Dayside", look, text, bg)
				}
				if c := contrast(tok[text], tok[bg]); c < 4.5 {
					t.Errorf("%s: %s (%s) on %s (%s) is %.2f:1; text needs 4.5:1", look, text, tok[text], bg, tok[bg], c)
				}
			}
		}
	}
}

// #250: buttons, inputs and selects are at least 32 px tall, so a finger or a pointer can hit them.
func TestControlsHaveAThirtyTwoPixelMinimum(t *testing.T) {
	b, err := os.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	css := strings.ReplaceAll(string(b), "\r", "")
	if !regexp.MustCompile(`button,input[^{]*select\{min-height:32px\}`).MatchString(css) {
		t.Error("buttons, inputs and selects need min-height:32px")
	}
}

// Register 2026-10-15, UX-09: white text on the red Delete button was 3.53:1 on the dark theme. Since #570 a filled button is the module's (d-btn, primary or danger), which draws its own
// fill and label: every button of the page carries d-btn, the stylesheet gives none a fill of its own, and the module's label and fill pairs reach 4.5:1 in every look and theme
// (TestEditorTextRulesAndFocusReachTheirRatiosInEveryLookAndTheme).
func TestFilledButtonsAreTheModulesAndNeverHaveAFillOfTheirOwn(t *testing.T) {
	b, err := os.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`button\.b[^{]*\{[^}]*(background|color):`).MatchString(string(b)) {
		t.Error("app.css still draws a button.b fill or label: that is the module's d-btn")
	}
	for _, f := range []string{"static/app.js", "static/livemap.js"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range regexp.MustCompile(`class="b[ "][^"]*"`).FindAllString(string(src), -1) {
			if !strings.Contains(m, "d-btn") {
				t.Errorf("%s: %s is a button without the module's d-btn", f, m)
			}
		}
	}
}
