package design

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// block is one rule of the stylesheet: the at-rule it sits in ("" at the top), its selector and its declarations.
type block struct {
	at, selector string
	decls        map[string]string
}

var commentRE = regexp.MustCompile(`(?s)/\*.*?\*/`)

// parse reads the stylesheet's rules. The file is small and plain (rules, and one @media with rules inside), so this is a brace reader, not a CSS parser.
func parse(t *testing.T, css string) []block {
	t.Helper()
	css = commentRE.ReplaceAllString(css, "")
	var out []block
	var walk func(s, at string)
	walk = func(s, at string) {
		for {
			s = strings.TrimSpace(s)
			if s == "" {
				return
			}
			open := strings.Index(s, "{")
			if open < 0 {
				t.Fatalf("stray text outside a rule: %q", s)
			}
			sel := strings.TrimSpace(s[:open])
			depth, end := 1, -1
			for i := open + 1; i < len(s) && end < 0; i++ {
				switch s[i] {
				case '{':
					depth++
				case '}':
					depth--
					if depth == 0 {
						end = i
					}
				}
			}
			if end < 0 {
				t.Fatalf("unclosed block after %q", sel)
			}
			body := s[open+1 : end]
			s = s[end+1:]
			if strings.HasPrefix(sel, "@") {
				walk(body, sel)
				continue
			}
			b := block{at: at, selector: sel, decls: map[string]string{}}
			for _, d := range strings.Split(body, ";") {
				d = strings.TrimSpace(d)
				if d == "" {
					continue
				}
				name, val, ok := strings.Cut(d, ":")
				if !ok {
					t.Fatalf("declaration without a colon: %q", d)
				}
				name = strings.TrimSpace(name)
				if _, dup := b.decls[name]; dup {
					t.Errorf("%s: %s is declared twice", sel, name)
				}
				b.decls[name] = strings.TrimSpace(val)
			}
			out = append(out, b)
		}
	}
	walk(css, "")
	return out
}

func tokenBlocks(t *testing.T) (dark, lightMedia, lightForced block) {
	t.Helper()
	n := 0
	for _, b := range parse(t, string(Tokens())) {
		n++
		switch {
		case b.at == "" && b.selector == ":root":
			dark = b
		case b.at == "@media (prefers-color-scheme: light)" && b.selector == `:root:not([data-theme="dark"])`:
			lightMedia = b
		case b.at == "" && b.selector == `:root[data-theme="light"]`:
			lightForced = b
		default:
			t.Errorf("unexpected rule %q in %q", b.selector, b.at)
		}
	}
	if n != 3 || dark.decls == nil || lightMedia.decls == nil || lightForced.decls == nil {
		t.Fatalf("tokens.css must hold exactly the dark block, the light media block and the forced light block (%d rules)", n)
	}
	return
}

var nameRE = regexp.MustCompile(`^--(c|t|s|r|e|mo)-[a-z0-9]+$`)

// Every custom property has a prefix that says what it is, and nothing but custom properties and color-scheme is declared.
func TestOnlyPrefixedCustomPropertiesAreDeclared(t *testing.T) {
	dark, lm, lf := tokenBlocks(t)
	for _, b := range []block{dark, lm, lf} {
		for name := range b.decls {
			if name != "color-scheme" && !nameRE.MatchString(name) {
				t.Errorf("%s declares %q: a token is --c-, --t-, --s-, --r-, --e- or --mo- followed by lower-case letters and digits", b.selector, name)
			}
		}
	}
}

// The two light blocks are the same theme: one for the system setting and one for the forced choice.
func TestTheTwoLightBlocksAreTheSame(t *testing.T) {
	_, lm, lf := tokenBlocks(t)
	if len(lm.decls) != len(lf.decls) {
		t.Errorf("the light blocks declare %d and %d properties", len(lm.decls), len(lf.decls))
	}
	for k, v := range lm.decls {
		if lf.decls[k] != v {
			t.Errorf("%s is %q in the system light block and %q in the forced one", k, v, lf.decls[k])
		}
	}
}

// The light theme changes colours (and the shadow, which is a colour); it never adds a name the dark theme lacks, and it never changes
// type, space, shape or motion, so a layout is the same in both themes.
func TestTheLightThemeOnlyChangesColours(t *testing.T) {
	dark, lm, _ := tokenBlocks(t)
	for k := range lm.decls {
		if _, ok := dark.decls[k]; !ok {
			t.Errorf("%s is in the light theme only", k)
		}
		if !strings.HasPrefix(k, "--c-") && k != "--e-2" && k != "color-scheme" {
			t.Errorf("the light theme changes %s", k)
		}
	}
	for k := range dark.decls {
		if strings.HasPrefix(k, "--c-") {
			if _, ok := lm.decls[k]; !ok {
				t.Errorf("the light theme lacks %s", k)
			}
		}
	}
}

func rgb(t *testing.T, hex string) [3]float64 {
	t.Helper()
	if !regexp.MustCompile(`^#[0-9a-f]{6}$`).MatchString(hex) {
		t.Fatalf("%q is not a #rrggbb colour", hex)
	}
	n, _ := strconv.ParseUint(hex[1:], 16, 32)
	return [3]float64{float64(n >> 16), float64((n >> 8) & 255), float64(n & 255)}
}

func luminance(c [3]float64) float64 {
	f := func(v float64) float64 {
		v /= 255
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*f(c[0]) + 0.7152*f(c[1]) + 0.0722*f(c[2])
}

func ratio(a, b [3]float64) float64 {
	x, y := luminance(a), luminance(b)
	return (math.Max(x, y) + 0.05) / (math.Min(x, y) + 0.05)
}

// Text colours are 4.5:1 or better on the three surfaces and the control boundary is 3:1 or better, in both themes (WCAG 2.1 AA).
func TestContrastInBothThemes(t *testing.T) {
	dark, lm, _ := tokenBlocks(t)
	themes := map[string]map[string]string{"dark": dark.decls, "light": {}}
	for k, v := range dark.decls {
		themes["light"][k] = v
	}
	for k, v := range lm.decls {
		themes["light"][k] = v
	}
	surfaces := []string{"--c-ground", "--c-panel", "--c-raised"}
	for name, th := range themes {
		for _, text := range []string{"--c-text", "--c-dim", "--c-spice", "--c-sky", "--c-ok", "--c-warn", "--c-bad"} {
			for _, s := range surfaces {
				if r := ratio(rgb(t, th[text]), rgb(t, th[s])); r < 4.5 {
					t.Errorf("%s: %s on %s is %.2f:1, under 4.5:1", name, text, s, r)
				}
			}
		}
		for _, s := range surfaces {
			if r := ratio(rgb(t, th["--c-rule"]), rgb(t, th[s])); r < 3 {
				t.Errorf("%s: --c-rule on %s is %.2f:1, under 3:1", name, s, r)
			}
		}
	}
}

// Each scale is in one unit and rises.
func TestScalesRise(t *testing.T) {
	dark, _, _ := tokenBlocks(t)
	for _, scale := range []struct {
		unit  string
		names []string
	}{
		{"px", []string{"--t-xs", "--t-sm", "--t-md", "--t-base", "--t-lg", "--t-xl"}},
		{"px", []string{"--s-1", "--s-2", "--s-3", "--s-4", "--s-5", "--s-6"}},
		{"px", []string{"--r-1", "--r-2", "--r-3", "--r-4", "--r-pill"}},
		{"ms", []string{"--mo-fast", "--mo-base", "--mo-slow"}},
	} {
		prev := -1.0
		for _, n := range scale.names {
			v, ok := dark.decls[n]
			if !ok || !strings.HasSuffix(v, scale.unit) {
				t.Errorf("%s is %q, want a number of %s", n, v, scale.unit)
				continue
			}
			f, err := strconv.ParseFloat(strings.TrimSuffix(v, scale.unit), 64)
			if err != nil || f <= prev {
				t.Errorf("%s = %q does not rise above the one before it", n, v)
			}
			prev = f
		}
	}
}

// The file asks for nothing from outside itself: no import, no url(), no address.
func TestNoExternalReference(t *testing.T) {
	css := commentRE.ReplaceAllString(string(Tokens()), "")
	for _, bad := range []string{"@import", "url(", "http:", "https:", "//", "expression("} {
		if strings.Contains(css, bad) {
			t.Errorf("tokens.css contains %q", bad)
		}
	}
}

// Every var() in the file names a token the file declares.
func TestVarsResolve(t *testing.T) {
	dark, _, _ := tokenBlocks(t)
	for _, m := range regexp.MustCompile(`var\(\s*(--[a-z0-9-]+)`).FindAllStringSubmatch(string(Tokens()), -1) {
		if _, ok := dark.decls[m[1]]; !ok {
			t.Errorf("var(%s) is not declared", m[1])
		}
	}
}
