package design

import (
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// A small reader for the module's stylesheets. They are plain: rules, a few @media blocks with rules inside, @keyframes and @font-face. It is a
// brace reader, not a CSS parser, and it fails loudly on anything it does not understand.

// rule is one style rule: the at-rule it sits in ("" at the top, "@media (...)" or "@keyframes name"), its selector list and its declarations in order.
type rule struct {
	file, at, selector string
	decls              []decl
}

type decl struct{ name, value string }

func (r rule) get(name string) (string, bool) {
	v, ok := "", false
	for _, d := range r.decls {
		if d.name == name {
			v, ok = d.value, true
		}
	}
	return v, ok
}

var blockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)

// splitTop splits s at sep, ignoring separators inside parentheses, brackets and quotes.
func splitTop(s string, sep byte) []string {
	var out []string
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(' || c == '[':
			depth++
		case c == ')' || c == ']':
			depth--
		case c == sep && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

// readFile returns a module file from disk (the tests run in the module's directory).
func readFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.FromSlash(name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// readCSS reads the rules of a stylesheet of the module (a path inside assets).
func readCSS(t *testing.T, name string) []rule {
	t.Helper()
	css := blockComment.ReplaceAllString(readFile(t, "assets/"+name), "")
	var out []rule
	var walk func(s, at string)
	walk = func(s, at string) {
		for {
			s = strings.TrimSpace(s)
			if s == "" {
				return
			}
			open := strings.Index(s, "{")
			if open < 0 {
				t.Fatalf("%s: stray text outside a rule: %.60q", name, s)
			}
			sel := strings.TrimSpace(s[:open])
			depth, end := 1, -1
			var quote byte
			for i := open + 1; i < len(s) && end < 0; i++ {
				c := s[i]
				switch {
				case quote != 0:
					if c == quote {
						quote = 0
					}
				case c == '"' || c == '\'':
					quote = c
				case c == '{':
					depth++
				case c == '}':
					depth--
					if depth == 0 {
						end = i
					}
				}
			}
			if end < 0 {
				t.Fatalf("%s: unclosed block after %q", name, sel)
			}
			body := s[open+1 : end]
			s = s[end+1:]
			if strings.HasPrefix(sel, "@") && !strings.HasPrefix(sel, "@font-face") {
				walk(body, sel)
				continue
			}
			r := rule{file: name, at: at, selector: sel}
			for _, d := range splitTop(body, ';') {
				d = strings.TrimSpace(d)
				if d == "" {
					continue
				}
				n, v, ok := strings.Cut(d, ":")
				if !ok {
					t.Fatalf("%s: %s: declaration without a colon: %q", name, sel, d)
				}
				r.decls = append(r.decls, decl{strings.TrimSpace(n), strings.TrimSpace(v)})
			}
			out = append(out, r)
		}
	}
	walk(css, "")
	return out
}

// assetNames returns the paths inside assets with the given extension.
func assetNames(t *testing.T, ext string) []string {
	t.Helper()
	var out []string
	err := fs.WalkDir(Assets, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ext) {
			out = append(out, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// ---- specificity ----

type spec [3]int

func (a spec) less(b spec) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

func (a spec) add(b spec) spec { return spec{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }

func maxSpec(a, b spec) spec {
	if a.less(b) {
		return b
	}
	return a
}

func isIdent(c byte) bool {
	return c == '-' || c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// specificity of one complex selector (no top-level comma): ids, classes, attributes and pseudo-classes, elements and pseudo-elements.
// :is, :not and :has count as their most specific argument; :where counts as nothing.
func specificity(sel string) spec {
	var s spec
	i := 0
	ident := func() string {
		j := i
		for j < len(sel) && isIdent(sel[j]) {
			j++
		}
		w := sel[i:j]
		i = j
		return w
	}
	paren := func() string {
		if i >= len(sel) || sel[i] != '(' {
			return ""
		}
		depth, j := 0, i
		for ; j < len(sel); j++ {
			if sel[j] == '(' {
				depth++
			} else if sel[j] == ')' {
				depth--
				if depth == 0 {
					break
				}
			}
		}
		in := sel[i+1 : j]
		i = j + 1
		return in
	}
	for i < len(sel) {
		switch c := sel[i]; {
		case c == '#':
			i++
			ident()
			s[0]++
		case c == '.':
			i++
			ident()
			s[1]++
		case c == '[':
			j := strings.IndexByte(sel[i:], ']')
			if j < 0 {
				j = len(sel) - i - 1
			}
			i += j + 1
			s[1]++
		case c == ':':
			i++
			if i < len(sel) && sel[i] == ':' {
				i++
				ident()
				paren()
				s[2]++
				continue
			}
			name := ident()
			arg := paren()
			switch name {
			case "where":
			case "is", "not", "has", "matches":
				var m spec
				for _, a := range splitTop(arg, ',') {
					m = maxSpec(m, specificity(strings.TrimSpace(a)))
				}
				s = s.add(m)
			default:
				s[1]++
			}
		case isIdent(c):
			ident()
			s[2]++
		default:
			i++
		}
	}
	return s
}

// ---- colours ----

type rgba [4]float64 // r, g, b in 0..255, a in 0..1

var hexRE = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
var rgbaRE = regexp.MustCompile(`^rgba?\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)\s*(?:,\s*([\d.]+)\s*)?\)$`)

func parseColor(s string) (rgba, bool) {
	s = strings.TrimSpace(s)
	if m := hexRE.FindStringSubmatch(s); m != nil {
		h := m[1]
		if len(h) == 3 {
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		}
		n, _ := strconv.ParseUint(h, 16, 32)
		return rgba{float64(n >> 16), float64((n >> 8) & 255), float64(n & 255), 1}, true
	}
	if m := rgbaRE.FindStringSubmatch(s); m != nil {
		var c rgba
		for i := 0; i < 3; i++ {
			c[i], _ = strconv.ParseFloat(m[i+1], 64)
		}
		c[3] = 1
		if m[4] != "" {
			c[3], _ = strconv.ParseFloat(m[4], 64)
		}
		return c, true
	}
	if strings.EqualFold(s, "transparent") {
		return rgba{0, 0, 0, 0}, true
	}
	return rgba{}, false
}

// mix is color-mix(in srgb, a p%, b) for opaque a and b, as the browser computes it.
func mix(a rgba, p float64, b rgba) rgba {
	p /= 100
	return rgba{a[0]*p + b[0]*(1-p), a[1]*p + b[1]*(1-p), a[2]*p + b[2]*(1-p), 1}
}

func lum(c rgba) float64 {
	f := func(v float64) float64 {
		v /= 255
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*f(c[0]) + 0.7152*f(c[1]) + 0.0722*f(c[2])
}

func contrast(a, b rgba) float64 {
	x, y := lum(a), lum(b)
	return (math.Max(x, y) + 0.05) / (math.Min(x, y) + 0.05)
}

// ---- a look's resolved tokens ----

// palette holds the custom properties of a look in one theme, with var() references resolved.
type palette struct {
	look, theme string
	pal         string // the id of the palette laid over the look, or ""
	raw         map[string]string
}

func tokenFile(look string) string { return "tokens-" + look + ".css" }

// loadPalette merges the dark block (and any other plain :root block) of a look's token file and, for the light theme, the forced light block over it.
func loadPalette(t *testing.T, look, theme string) palette {
	t.Helper()
	p := palette{look: look, theme: theme, raw: map[string]string{}}
	for _, r := range readCSS(t, tokenFile(look)) {
		if r.at == "" && r.selector == ":root" {
			for _, d := range r.decls {
				p.raw[d.name] = d.value
			}
		}
	}
	if theme == "light" {
		n := 0
		for _, r := range readCSS(t, tokenFile(look)) {
			if r.at == "" && r.selector == `:root[data-theme="light"]` {
				n++
				for _, d := range r.decls {
					p.raw[d.name] = d.value
				}
			}
		}
		if n == 0 {
			t.Fatalf("%s has no forced light block", tokenFile(look))
		}
	}
	return p
}

var varRE = regexp.MustCompile(`^var\(\s*(--[a-z0-9-]+)\s*\)$`)

// value returns the raw value of a token with var() references followed.
func (p palette) value(t *testing.T, name string) string {
	t.Helper()
	v, ok := p.raw[name]
	for n := 0; ok && n < 8; n++ {
		m := varRE.FindStringSubmatch(v)
		if m == nil {
			return v
		}
		v, ok = p.raw[m[1]]
	}
	t.Fatalf("%s %s: %s is not defined", p.look, p.theme, name)
	return ""
}

func (p palette) color(t *testing.T, name string) rgba {
	t.Helper()
	c, ok := parseColor(p.value(t, name))
	if !ok {
		t.Fatalf("%s %s: %s = %q is not a colour", p.look, p.theme, name, p.value(t, name))
	}
	return c
}

var looks = []string{"fremen", "awakening", "sietch"}
var themes = []string{"dark", "light"}

// forEachPalette runs fn as a subtest for every look and theme, and then for every look with each of the palettes laid over it (in the palette's
// own theme).
func forEachPalette(t *testing.T, fn func(t *testing.T, p palette)) {
	t.Helper()
	for _, l := range looks {
		for _, th := range themes {
			p := loadPalette(t, l, th)
			t.Run(l+"/"+th, func(t *testing.T) { fn(t, p) })
		}
	}
	for _, l := range looks {
		for _, pal := range Palettes() {
			p := loadPaletteOver(t, l, pal)
			t.Run(l+"/"+pal.ID, func(t *testing.T) { fn(t, p) })
		}
	}
}

// loadPaletteOver is the look in the palette's theme with the palette's tokens laid over it, as the browser resolves them: the palette's
// selectors are more specific than the look's, so every token the palette declares wins.
func loadPaletteOver(t *testing.T, look string, pal Palette) palette {
	t.Helper()
	p := loadPalette(t, look, pal.Theme)
	p.pal = pal.ID
	forced := `:root[data-palette="` + pal.ID + `"][data-theme="` + pal.Theme + `"]`
	n := 0
	for _, r := range readCSS(t, PalettesFile) {
		if r.at == "" && r.selector == forced {
			n++
			for _, d := range r.decls {
				p.raw[d.name] = d.value
			}
		}
	}
	if n != 1 {
		t.Fatalf("%s has %d blocks %s, want 1", PalettesFile, n, forced)
	}
	return p
}

// px reads a value such as "11px" or "var(--hit)" (resolved through the palette) as a number of pixels.
func (p palette) px(t *testing.T, v string) (float64, bool) {
	v = strings.TrimSpace(v)
	if m := varRE.FindStringSubmatch(v); m != nil {
		v = p.value(t, m[1])
	}
	if !strings.HasSuffix(v, "px") {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.TrimSuffix(v, "px"), 64)
	return f, err == nil
}
