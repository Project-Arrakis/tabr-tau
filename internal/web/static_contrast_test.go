package web

import (
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func rgbOf(hex string) [3]float64 {
	var c [3]float64
	for i := 0; i < 3; i++ {
		v, _ := strconv.ParseUint(hex[1+2*i:3+2*i], 16, 8)
		c[i] = float64(v)
	}
	return c
}

// mixHex is color-mix(in srgb, a p%, b): the channels are mixed as they are written.
func mixHex(a, b string, p float64) string {
	x, y := rgbOf(a), rgbOf(b)
	out := "#"
	for i := 0; i < 3; i++ {
		out += pad2(int64(math.Round(x[i]*p + y[i]*(1-p))))
	}
	return out
}

func pad2(v int64) string {
	s := strconv.FormatInt(v, 16)
	if len(s) < 2 {
		s = "0" + s
	}
	return s
}

// surfaces are the backgrounds text is drawn on: the page, the panels and the tinted banners (warning and info), whose tint is read from the stylesheet.
func surfaces(t *testing.T, css string, theme map[string]string) map[string]string {
	t.Helper()
	out := map[string]string{"--bg": theme["--bg"], "--panel": theme["--panel"], "--panel2": theme["--panel2"]}
	re := regexp.MustCompile(`([^{}]*)\{[^{}]*background:color-mix\(in srgb,var\((--[a-z0-9-]+)\) (\d+)%,var\((--[a-z0-9-]+)\)\)`)
	n := 0
	for _, m := range re.FindAllStringSubmatch(css, -1) {
		p, _ := strconv.ParseFloat(m[3], 64)
		out[strings.TrimSpace(m[1])+" "+m[3]+"% of "+m[2]] = mixHex(theme[m[2]], theme[m[4]], p/100)
		n++
	}
	if n < 4 {
		t.Fatalf("expected the tinted banner backgrounds in the stylesheet, found %d", n)
	}
	return out
}

// Review 2026-10-23 B2: text colours reach 4.5:1 in BOTH themes on every surface they are drawn on, the tinted banners included (the muted colour on a warning or info banner was
// 3.84 to 4.18:1 in the dark theme, and the light accent on the beige tiles 4.37:1). The colours are the design module's, for every look it has (#570).
func TestTextColoursReachFourPointFiveInBothThemesOnEverySurface(t *testing.T) {
	b, err := os.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	css := strings.ReplaceAll(string(b), "\r", "")
	forEachLookTheme(t, func(t *testing.T, theme map[string]string) {
		surf := surfaces(t, css, theme)
		names := make([]string, 0, len(surf))
		for k := range surf {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, text := range []string{"--fg", "--mut", "--ok", "--bad", "--acc", "--acc2"} {
			for _, s := range names {
				if strings.HasPrefix(s, "--") { // the page and the panels: every text colour
					if c := contrast(theme[text], surf[s]); c < 4.5 {
						t.Errorf("%s (%s) on %s (%s) is %.2f:1; text needs 4.5:1", text, theme[text], s, surf[s], c)
					}
				}
			}
		}
		// on a tinted banner: the body text and the muted text, which the stylesheet gives its own colour there (--mut-tint)
		for _, s := range names {
			if strings.HasPrefix(s, "--") {
				continue
			}
			for _, text := range []string{"--fg", "--mut-tint"} {
				if theme[text] == "" {
					t.Fatalf("%s is not defined", text)
				}
				if c := contrast(theme[text], surf[s]); c < 4.5 {
					t.Errorf("%s (%s) on the banner %q (%s) is %.2f:1; text needs 4.5:1", text, theme[text], s, surf[s], c)
				}
			}
		}
	})
	if !regexp.MustCompile(`\.warn \.mut[^{]*\{color:var\(--mut-tint\)\}`).MatchString(css) {
		t.Error("the muted text on a banner must use --mut-tint")
	}
}
