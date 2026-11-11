package design

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The palettes: the Go list, the stylesheet's structure and how it cascades. The contrast of every palette on every look is checked by the
// contrast tests of looks_test.go, which run once more for each palette (forEachPalette); the file itself is checked against its generator in
// palettes_gen_test.go.

var paletteIDRE = regexp.MustCompile(`^[a-z]+(-[a-z]+)+$`)

// the tokens a palette sets, in every block of it
var paletteNames = []string{
	"--c-ground", "--c-panel", "--c-raised", "--c-sunken", "--c-scrim", "--c-hair", "--c-rule", "--c-bronze", "--c-stitch", "--c-text", "--c-dim",
	"--c-spice", "--c-on-spice", "--c-blue", "--c-on-blue", "--c-focus", "--c-gold", "--c-rust", "--c-sky", "--c-ok", "--c-warn", "--c-bad",
	"--c-glow", "--c-dust-1", "--c-dust-2", "--c-dust-3", "--c-switch", "--c-on-switch", "--k-boost", "--aura", "accent-color", "scrollbar-color",
}

func TestPalettesListAndAPI(t *testing.T) {
	ps := Palettes()
	if len(ps) != 18 {
		t.Fatalf("%d palettes, want 18", len(ps))
	}
	if ps[0].ID != "warm-charcoal" || ps[0].Name != "Arrakis Incandescent" {
		t.Errorf("the first palette is %+v, want Arrakis Incandescent", ps[0])
	}
	seen, names := map[string]bool{}, map[string]bool{}
	var light []string
	for _, p := range ps {
		if !paletteIDRE.MatchString(p.ID) {
			t.Errorf("id %q: lower-case words joined by hyphens", p.ID)
		}
		if seen[p.ID] || names[p.Name] {
			t.Errorf("%q is listed twice", p.ID)
		}
		seen[p.ID], names[p.Name] = true, true
		if p.Name == "" || p.Group == "" {
			t.Errorf("%q has no name or no group", p.ID)
		}
		if p.Theme != ThemeDark && p.Theme != ThemeLight {
			t.Errorf("%q has the theme %q", p.ID, p.Theme)
		}
		if p.Theme == ThemeLight {
			light = append(light, p.Name)
		}
		// the id names colours: a franchise word belongs in the label only
		if regexp.MustCompile(`(?i)atreides|harkonnen|dune|arrakis|fremen|sietch|mentat|sardaukar|gesserit|guild|sandworm|muad|kaitain|caladan|salusa|ix-|giedi|spice|navigator|water-of-life`).MatchString(p.ID) {
			t.Errorf("id %q holds a word from the franchise", p.ID)
		}
		if p.ID == NoPalette {
			t.Errorf("%q is the id that means no palette", p.ID)
		}
	}
	if strings.Join(light, ",") != "Fremen Sietch,Mentat" {
		t.Errorf("the light palettes are %v, want Fremen Sietch and Mentat only", light)
	}
	// the groups are listed one after the other, so that a menu can print a heading whenever the group changes
	done, prev := map[string]bool{}, ""
	for _, p := range ps {
		if p.Group != prev {
			if done[p.Group] {
				t.Errorf("group %q is split in the list", p.Group)
			}
			done[p.Group], prev = true, p.Group
		}
	}
	// Palettes is a copy
	ps[0].ID = "changed"
	if Palettes()[0].ID == "changed" {
		t.Error("Palettes returned the module's own slice")
	}
	// PaletteFiles: a file for each palette, none for NoPalette, and a refusal for any other string (a stored value never names a file)
	for _, p := range Palettes() {
		if f, ok := PaletteFiles(p.ID); !ok || len(f) != 1 || f[0] != PalettesFile {
			t.Errorf("PaletteFiles(%q) = %v, %v", p.ID, f, ok)
		}
	}
	if f, ok := PaletteFiles(NoPalette); !ok || f != nil {
		t.Errorf("PaletteFiles(NoPalette) = %v, %v", f, ok)
	}
	for _, bad := range []string{"", "../tokens", "warm-charcoal.css", "WARM-CHARCOAL", "dune-arrakis-inc", "palettes.css", " warm-charcoal"} {
		if f, ok := PaletteFiles(bad); ok || f != nil {
			t.Errorf("PaletteFiles(%q) = %v, %v: not a palette", bad, f, ok)
		}
	}
	if _, err := Assets.Open(PalettesFile); err != nil {
		t.Errorf("%s is not in Assets: %v", PalettesFile, err)
	}
}

// the table of the generator, the list of the API and the stylesheet name the same palettes in the same order
func TestPalettesAgreeWithTheirSource(t *testing.T) {
	ps := Palettes()
	if len(paletteSources) != len(ps) {
		t.Fatalf("%d palettes in the table of palettes_gen_test.go and %d in the list", len(paletteSources), len(ps))
	}
	for i, p := range ps {
		s := paletteSources[i]
		if p.ID != s.id || p.Name != s.name || p.Group != s.group || p.Theme != s.theme {
			t.Errorf("palette %d is %+v in palettes.go and %+v in the table", i, p, s)
		}
	}
}

// Every palette has exactly the blocks it should: one for the forced theme and one for the system setting, each declaring the same names with
// the same values and nothing else; a palette declares no block for the theme that is not its own.
func TestPalettesHaveTheirTwoBlocksAndNothingElse(t *testing.T) {
	type pair struct{ forced, media *rule }
	got := map[string]*pair{}
	idRE := regexp.MustCompile(`^:root\[data-palette="([a-z-]+)"\]`)
	rs := readCSS(t, PalettesFile)
	for i := range rs {
		r := rs[i]
		m := idRE.FindStringSubmatch(r.selector)
		if m == nil {
			t.Errorf("unexpected rule %q in %q", r.selector, r.at)
			continue
		}
		pal := got[m[1]]
		if pal == nil {
			pal = &pair{}
			got[m[1]] = pal
		}
		switch {
		case r.at == "" && (r.selector == m[0]+`[data-theme="dark"]` || r.selector == m[0]+`[data-theme="light"]`):
			if pal.forced != nil {
				t.Errorf("%s has two forced blocks", m[1])
			}
			pal.forced = &rs[i]
		case r.at == "@media not (prefers-color-scheme: light)" && r.selector == m[0]+`:not([data-theme="light"])`,
			r.at == "@media (prefers-color-scheme: light)" && r.selector == m[0]+`:not([data-theme="dark"])`:
			if pal.media != nil {
				t.Errorf("%s has two system blocks", m[1])
			}
			pal.media = &rs[i]
		default:
			t.Errorf("unexpected rule %q in %q", r.selector, r.at)
		}
	}
	for _, p := range Palettes() {
		pal := got[p.ID]
		if pal == nil || pal.forced == nil || pal.media == nil {
			t.Errorf("%s needs a forced block and a system block", p.ID)
			continue
		}
		if !strings.HasSuffix(pal.forced.selector, `[data-theme="`+p.Theme+`"]`) {
			t.Errorf("%s: the forced block is for %s", p.ID, pal.forced.selector)
		}
		if p.Theme == ThemeDark && pal.media.at != "@media not (prefers-color-scheme: light)" || p.Theme == ThemeLight && pal.media.at != "@media (prefers-color-scheme: light)" {
			t.Errorf("%s (%s): the system block sits in %q", p.ID, p.Theme, pal.media.at)
		}
		a, b := declMap(*pal.forced), declMap(*pal.media)
		if len(a) != len(paletteNames) || len(b) != len(paletteNames) {
			t.Errorf("%s declares %d and %d properties, want %d", p.ID, len(a), len(b), len(paletteNames))
		}
		for _, n := range paletteNames {
			if v, ok := a[n]; !ok || v != b[n] {
				t.Errorf("%s: %s is %q in the forced block and %q in the system block", p.ID, n, v, b[n])
			}
		}
	}
	if len(got) != len(Palettes()) {
		t.Errorf("%s holds %d palettes, the list has %d", PalettesFile, len(got), len(Palettes()))
	}
}

// A palette changes colours only, and only the tokens the looks define, so a palette can never move a layout.
func TestPalettesChangeColourTokensOnly(t *testing.T) {
	look := map[string]bool{}
	for _, l := range looks {
		dark, _, _ := tokenRules(t, l)
		for n := range declMap(dark...) {
			look[n] = true
		}
	}
	for _, n := range paletteNames {
		if n == "accent-color" || n == "scrollbar-color" {
			continue
		}
		if !look[n] {
			t.Errorf("%s is not a token of the looks", n)
		}
		if !strings.HasPrefix(n, "--c-") && n != "--k-boost" && n != "--aura" {
			t.Errorf("%s is not a colour token", n)
		}
	}
	for _, r := range readCSS(t, PalettesFile) {
		for _, d := range r.decls {
			if strings.HasPrefix(d.name, "--") && !lookNameRE.MatchString(d.name) {
				t.Errorf("%s declares %q", r.selector, d.name)
			}
			if strings.Contains(d.value, "url(") || strings.Contains(d.value, "@import") {
				t.Errorf("%s: %s names a file", r.selector, d.name)
			}
		}
	}
}

// A palette must win over the look whatever the order of the files, in the two ways the look's own light theme is switched on.
func TestPalettesAreMoreSpecificThanTheLooks(t *testing.T) {
	var top spec
	for _, l := range looks {
		for _, r := range readCSS(t, tokenFile(l)) {
			if strings.HasPrefix(r.selector, ":root") {
				top = maxSpec(top, specificity(r.selector))
			}
		}
	}
	for _, r := range readCSS(t, PalettesFile) {
		if s := specificity(r.selector); !top.less(s) {
			t.Errorf("%s has the specificity %v, not above the looks' %v", r.selector, s, top)
		}
	}
}

// The kitchen sink offers every palette and links the file.
func TestEveryPaletteIsInTheKitchenSink(t *testing.T) {
	page := readFile(t, "kitchen-sink/index.html")
	js := readFile(t, "kitchen-sink/kitchen-sink.js")
	var ids []string
	for _, p := range Palettes() {
		ids = append(ids, p.ID)
		if !strings.Contains(page, `value="`+p.ID+`"`) || !strings.Contains(page, ">"+p.Name+"<") {
			t.Errorf("the kitchen sink does not offer %s (%s)", p.Name, p.ID)
		}
		if !strings.Contains(js, "'"+p.ID+"'") {
			t.Errorf("kitchen-sink.js does not know %s", p.ID)
		}
	}
	if !strings.Contains(page, `href="../assets/`+PalettesFile+`"`) {
		t.Error("the kitchen sink does not link " + PalettesFile)
	}
	if !strings.Contains(js, "data-palette") {
		t.Error("kitchen-sink.js never sets data-palette")
	}
	sort.Strings(ids)
	for i := 1; i < len(ids); i++ {
		if ids[i] == ids[i-1] {
			t.Errorf("id %s twice", ids[i])
		}
	}
}
