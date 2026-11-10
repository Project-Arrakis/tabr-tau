package design

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The token files of the three looks: names, the two themes, the names of version 0.1.0, and contrast (design review register, check 1).

var lookNameRE = regexp.MustCompile(`^--(c|t|s|r|e|mo|m|k|hit|aura|vignette|weave|grain)(-[a-z0-9]+)*$`)

// blocks returns the plain :root blocks, the light media block and the forced light block of a look's token file.
func tokenRules(t *testing.T, look string) (dark []rule, lightMedia, lightForced *rule) {
	t.Helper()
	for _, r := range readCSS(t, tokenFile(look)) {
		r := r
		switch {
		case r.at == "" && r.selector == ":root":
			dark = append(dark, r)
		case r.at == "@media (prefers-color-scheme: light)" && r.selector == `:root:not([data-theme="dark"])`:
			lightMedia = &r
		case r.at == "" && r.selector == `:root[data-theme="light"]`:
			lightForced = &r
		case r.selector == "@font-face" || r.selector == `html[data-heading-font="system"]`:
			// the heading face, checked in files_test.go
		default:
			t.Errorf("%s: unexpected rule %q in %q", tokenFile(look), r.selector, r.at)
		}
	}
	if len(dark) == 0 || lightMedia == nil || lightForced == nil {
		t.Fatalf("%s must hold a :root block, the light media block and the forced light block", tokenFile(look))
	}
	return
}

func declMap(rs ...rule) map[string]string {
	m := map[string]string{}
	for _, r := range rs {
		for _, d := range r.decls {
			m[d.name] = d.value
		}
	}
	return m
}

func TestEveryLookHasItsFiles(t *testing.T) {
	if got := strings.Join(Looks(), ","); got != "fremen,awakening,sietch" {
		t.Errorf("Looks() = %s", got)
	}
	if DefaultLook != "fremen" {
		t.Errorf("the default look is %s, want fremen", DefaultLook)
	}
	for _, l := range Looks() {
		files, ok := LookFiles(l)
		if !ok || len(files) != 4 {
			t.Fatalf("LookFiles(%s) = %v, %v", l, files, ok)
		}
		for _, f := range files {
			if b, err := Assets.Open(f); err != nil {
				t.Errorf("%s: %v", f, err)
			} else {
				b.Close()
			}
		}
		if files[0] != "tokens-"+l+".css" || files[1] != "components-"+l+".css" || files[2] != "components.css" || files[3] != "motion.css" {
			t.Errorf("link order of %s: %v", l, files)
		}
	}
	if f, ok := LookFiles("../tokens"); ok || f != nil {
		t.Error("LookFiles accepted a string that is not a look")
	}
}

// Every custom property of a look has a known prefix, the light themes change colours and shadows only, and the two light blocks agree.
func TestLooksDeclareOnlyKnownTokens(t *testing.T) {
	for _, l := range looks {
		dark, lm, lf := tokenRules(t, l)
		d := declMap(dark...)
		for name := range d {
			if name != "color-scheme" && !lookNameRE.MatchString(name) {
				t.Errorf("%s declares %q: not a token name", l, name)
			}
		}
		light, forced := declMap(*lm), declMap(*lf)
		if len(light) != len(forced) {
			t.Errorf("%s: the light blocks declare %d and %d properties", l, len(light), len(forced))
		}
		for k, v := range light {
			if forced[k] != v {
				t.Errorf("%s: %s is %q in the system light block and %q in the forced one", l, k, v, forced[k])
			}
			if _, ok := d[k]; !ok {
				t.Errorf("%s: %s is in the light theme only", l, k)
			}
			if !strings.HasPrefix(k, "--c-") && !strings.HasPrefix(k, "--e-") && k != "--vignette" && k != "--weave" && k != "color-scheme" {
				t.Errorf("%s: the light theme changes %s (colour, shadow and texture only)", l, k)
			}
		}
	}
}

// A program written against version 0.1.0 can link a look's tokens instead of tokens.css: every name of that version is defined.
func TestLooksKeepTheNamesOfVersion010(t *testing.T) {
	var old []string
	for _, r := range readCSS(t, "tokens.css") {
		if r.at == "" && r.selector == ":root" {
			for _, d := range r.decls {
				if d.name != "color-scheme" {
					old = append(old, d.name)
				}
			}
		}
	}
	if len(old) < 30 {
		t.Fatalf("read only %d names from tokens.css", len(old))
	}
	for _, l := range looks {
		dark, _, _ := tokenRules(t, l)
		d := declMap(dark...)
		for _, n := range old {
			if _, ok := d[n]; !ok {
				t.Errorf("%s does not define %s, a name of version 0.1.0", l, n)
			}
		}
	}
}

// tokens.css is the file of version 0.1.0 and has not changed: its values are what the programs that adopted it show.
func TestVersion010TokensAreUnchanged(t *testing.T) {
	css := readFile(t, "assets/tokens.css")
	for _, must := range []string{"version 0.1.0", "--c-ground: #100d0b;", "--c-spice: #e8873a;", "--c-spice: #a8501a;", "--mo-slow: 320ms;", "--r-4: 8px;"} {
		if !strings.Contains(css, must) {
			t.Errorf("tokens.css lacks %q", must)
		}
	}
}

var surfaces = []string{"--c-ground", "--c-panel", "--c-raised", "--c-sunken"}

// Text tokens are 4.5:1 on all four surfaces; the boundary of a control, the divider ink and the focus ring are 3:1 on them. Every look, both themes.
func TestContrastOfEveryLookAndTheme(t *testing.T) {
	forEachPalette(t, func(t *testing.T, p palette) {
		for _, text := range []string{"--c-text", "--c-dim", "--c-gold", "--c-spice", "--c-sky", "--c-ok", "--c-warn", "--c-bad"} {
			for _, s := range surfaces {
				if r := contrast(p.color(t, text), p.color(t, s)); r < 4.5 {
					t.Errorf("%s on %s is %.2f:1, under 4.5:1", text, s, r)
				}
			}
		}
		for _, line := range []string{"--c-rule", "--c-focus", "--c-bronze"} {
			if _, ok := p.raw[line]; !ok && line == "--c-bronze" {
				continue // the Sietch look draws no bronze
			}
			for _, s := range surfaces {
				// the bronze divider is drawn on panels and raised parts only; the rule and the focus ring also on the ground and the sunken field
				if line == "--c-bronze" && (s == "--c-ground" || s == "--c-sunken") {
					continue
				}
				if r := contrast(p.color(t, line), p.color(t, s)); r < 3 {
					t.Errorf("%s on %s is %.2f:1, under 3:1", line, s, r)
				}
			}
		}
		// labels on a fill
		for _, pair := range [][2]string{{"--c-on-spice", "--c-spice"}, {"--c-on-switch", "--c-switch"}} {
			if r := contrast(p.color(t, pair[0]), p.color(t, pair[1])); r < 4.5 {
				t.Errorf("%s on %s is %.2f:1, under 4.5:1", pair[0], pair[1], r)
			}
		}
		if _, ok := p.raw["--c-on-blue"]; ok {
			if r := contrast(p.color(t, "--c-on-blue"), p.color(t, "--c-blue")); r < 4.5 {
				t.Errorf("--c-on-blue on --c-blue is %.2f:1, under 4.5:1", r)
			}
		}
		// the on state of a switch is told by its border (--c-spice, checked above as text), its knob and the knob's place; the knob is 3:1 on the fill
		if r := contrast(p.color(t, "--c-on-switch"), p.color(t, "--c-switch")); r < 3 {
			t.Errorf("the knob (--c-on-switch) on the on track is %.2f:1, under 3:1", r)
		}
		if r := contrast(p.color(t, "--c-dim"), p.color(t, "--c-sunken")); r < 3 {
			t.Errorf("the knob of a switch that is off (--c-dim) on the off track is %.2f:1, under 3:1", r)
		}
	})
}

// Derived colours: the tints, the hover mixes and the fills that components.css computes with color-mix, checked the way the browser mixes them.
func TestContrastOfDerivedStates(t *testing.T) {
	forEachPalette(t, func(t *testing.T, p palette) {
		c := func(n string) rgba { return p.color(t, n) }
		white := rgba{255, 255, 255, 1}
		check := func(what string, fg, bg rgba, min float64) {
			t.Helper()
			if r := contrast(fg, bg); r < min {
				t.Errorf("%s is %.2f:1, under %.1f:1", what, r, min)
			}
		}
		hoverPct := cssMixPct(t, "components-"+p.look+".css", ".d-alert:hover", "--c-warn")
		// banners and alerts: a tint of their colour over the panel; the title is --c-text, the detail line --c-dim, the icon (not text) 3:1
		for _, tint := range []struct {
			name string
			col  string
			pct  float64
		}{{"warn 12%", "--c-warn", 12}, {"warn hover " + itoa(hoverPct) + "%", "--c-warn", hoverPct}, {"bad 12%", "--c-bad", 12}, {"bad 14%", "--c-bad", 14}, {"sky 10%", "--c-sky", 10}} {
			bg := mix(c(tint.col), tint.pct, c("--c-panel"))
			check("--c-text on the "+tint.name+" tint", c("--c-text"), bg, 4.5)
			check("--c-dim on the "+tint.name+" tint", c("--c-dim"), bg, 4.5)
			check(tint.col+" icon on its "+tint.name+" tint", c(tint.col), bg, 3)
		}
		// the accent tints of a hovered or selected part, over raised and over the panel (the bar of the nav); at most 20 %
		for _, pct := range []float64{8, 9, 10, 12, 16, 20} {
			for _, base := range []string{"--c-raised", "--c-panel"} {
				bg := mix(c("--c-spice"), pct, c(base))
				check("--c-text on spice "+itoa(pct)+"% over "+base, c("--c-text"), bg, 4.5)
				if pct <= 12 {
					check("--c-dim on spice "+itoa(pct)+"% over "+base, c("--c-dim"), bg, 4.5)
				}
			}
		}
		// the primary button: the label on the fill, and on the fill when hovered (mixed with white)
		fill, label := "--c-spice", "--c-on-spice"
		if p.look == "fremen" {
			fill, label = "--c-blue", "--c-on-blue"
		}
		mixPct := cssMixPct(t, "components-"+p.look+".css", ".d-btn.primary:hover:not(:disabled)", fill)
		check(label+" on "+fill, c(label), c(fill), 4.5)
		check(label+" on "+fill+" hovered", c(label), mix(c(fill), mixPct, white), 4.5)
		// the icon box of a tile, a row that has just changed, the "!" of a critical meter
		check("--c-spice on the tile icon box", c("--c-spice"), mix(c("--c-spice"), 16, c("--c-raised")), 3)
		flash := mix(c("--c-spice"), cssMixPct(t, "motion.css", "from", "--c-spice"), c("--c-sunken"))
		check("--c-text on the flash of a changed row", c("--c-text"), flash, 4.5)
		check("--c-dim on the flash of a changed row", c("--c-dim"), flash, 4.5)
		check("the ! on a critical meter (--c-panel on --c-bad)", c("--c-panel"), c("--c-bad"), 4.5)
		// the pulse of the Live pill and the certainty rings are not text; their ring colours must hold 3:1 on the panel
		check("the Saved ring (--c-text) on the panel", c("--c-text"), c("--c-panel"), 3)
		check("the Possible ring (--c-dim) on the panel", c("--c-dim"), c("--c-panel"), 3)
	})
}

func itoa(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// Type, spacing and motion are the same in both themes and rise; no text token is under 11 px.
func TestScalesOfEveryLook(t *testing.T) {
	for _, l := range looks {
		p := loadPalette(t, l, "dark")
		for _, scale := range [][]string{
			{"--t-xs", "--t-sm", "--t-md", "--t-base", "--t-lg", "--t-xl", "--t-2xl"},
			{"--s-1", "--s-2", "--s-3", "--s-4", "--s-5", "--s-6"},
		} {
			prev := -1.0
			for _, n := range scale {
				v, ok := p.px(t, "var("+n+")")
				if !ok || v <= prev {
					if n == "--t-2xl" && !ok {
						continue // only the looks name a display size
					}
					t.Errorf("%s: %s = %q does not rise above the one before it", l, n, p.raw[n])
				}
				prev = v
			}
		}
		if v, _ := p.px(t, "var(--t-xs)"); v < 11 {
			t.Errorf("%s: --t-xs is %vpx, the smallest text is 11 px", l, v)
		}
		if v, _ := p.px(t, "var(--hit)"); v < 32 {
			t.Errorf("%s: --hit is %vpx, at least 32 px", l, v)
		}
		if v, _ := p.px(t, "var(--hit-dense)"); v < 24 {
			t.Errorf("%s: --hit-dense is %vpx, at least 24 px", l, v)
		}
	}
}

var mixRE = regexp.MustCompile(`color-mix\(in srgb,\s*var\((--[a-z-]+)\)\s+([\d.]+)%`)

// cssMixPct returns the percentage of the first color-mix of the given token in the first rule with this selector of a stylesheet, so that the
// contrast test reads the number the stylesheet really uses.
func cssMixPct(t *testing.T, file, selector, token string) float64 {
	t.Helper()
	for _, r := range readCSS(t, file) {
		if r.selector != selector {
			continue
		}
		for _, d := range r.decls {
			for _, m := range mixRE.FindAllStringSubmatch(d.value, -1) {
				if m[1] == token {
					f, _ := strconv.ParseFloat(m[2], 64)
					return f
				}
			}
		}
	}
	t.Fatalf("%s: no color-mix of %s in a rule %q", file, token, selector)
	return 0
}
