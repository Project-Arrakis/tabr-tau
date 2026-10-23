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
