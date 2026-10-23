package web

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
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

// UX-41: text drawn in the muted and the "ok" colours must reach 4.5:1 on every surface of the light theme (the muted colour on the beige tiles was 4.08:1).
func TestLightThemeTextColoursReachFourPointFive(t *testing.T) {
	b, err := os.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	css := strings.ReplaceAll(string(b), "\r", "")
	m := regexp.MustCompile(`@media \(prefers-color-scheme:light\)\{:root\{([^}]*)\}\}`).FindStringSubmatch(css)
	if m == nil {
		t.Fatal("the light theme block is not where this test looks for it")
	}
	tok := map[string]string{}
	for _, d := range regexp.MustCompile(`(--[a-z0-9-]+):(#[0-9a-fA-F]{6})`).FindAllStringSubmatch(m[1], -1) {
		tok[d[1]] = d[2]
	}
	for _, text := range []string{"--mut", "--ok", "--bad", "--acc2"} {
		for _, bg := range []string{"--bg", "--panel", "--panel2"} {
			if tok[text] == "" || tok[bg] == "" {
				t.Fatalf("%s or %s is not defined in the light theme", text, bg)
			}
			if c := contrast(tok[text], tok[bg]); c < 4.5 {
				t.Errorf("%s (%s) on %s (%s) is %.2f:1; text needs 4.5:1", text, tok[text], bg, tok[bg], c)
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

// Register 2026-10-15, UX-09: white text on the red Delete button was 3.53:1 on the dark theme. The label of a filled
// button (accent, and the red one) must reach 4.5:1 on its own fill in both themes.
func TestFilledButtonLabelsReachFourPointFiveInBothThemes(t *testing.T) {
	b, err := os.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	css := strings.ReplaceAll(string(b), "\r", "")
	tokens := func(re string) map[string]string {
		out := map[string]string{}
		for _, m := range regexp.MustCompile(re).FindAllStringSubmatch(css, -1) {
			for _, d := range regexp.MustCompile(`(--[a-z0-9-]+):(#[0-9a-fA-F]{3,6})`).FindAllStringSubmatch(m[1], -1) {
				out[d[1]] = d[2]
			}
		}
		return out
	}
	dark := tokens(`(?m)^:root\{([^}]*)\}`)
	light := map[string]string{}
	for k, v := range dark {
		light[k] = v
	}
	for k, v := range tokens(`@media \(prefers-color-scheme:light\)\{:root\{([^}]*)\}\}`) {
		light[k] = v
	}
	resolve := func(theme map[string]string, v string) string {
		v = strings.TrimSpace(v)
		if m := regexp.MustCompile(`^var\((--[a-z0-9-]+)\)$`).FindStringSubmatch(v); m != nil {
			v = theme[m[1]]
		}
		if len(v) == 4 && v[0] == '#' { // #rgb
			v = "#" + strings.Repeat(string(v[1]), 2) + strings.Repeat(string(v[2]), 2) + strings.Repeat(string(v[3]), 2)
		}
		return v
	}
	rules := []struct{ name, selector, fill string }{
		{"the accent button", `button\.b\{color:([^;}]*)`, "--acc"},
		{"the red button", `button\.b\.bad\{color:([^;}]*)`, "--bad"},
	}
	for themeName, theme := range map[string]map[string]string{"dark": dark, "light": light} {
		for _, r := range rules {
			ms := regexp.MustCompile(r.selector).FindAllStringSubmatch(css, -1)
			if len(ms) == 0 {
				t.Fatalf("%s: no rule found for %s", themeName, r.name)
			}
			label := resolve(theme, ms[len(ms)-1][1])
			fill := resolve(theme, theme[r.fill])
			if len(label) != 7 || fill == "" {
				t.Fatalf("%s: %s label %q or fill %q is not a plain colour", themeName, r.name, label, fill)
			}
			if c := contrast(label, fill); c < 4.5 {
				t.Errorf("%s theme: the label of %s (%s on %s) is %.2f:1; text needs 4.5:1", themeName, r.name, label, fill, c)
			}
		}
	}
}
