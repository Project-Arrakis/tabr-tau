package web

import (
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/design"
)

// The editor's colours come from the design module (#570): app.css names its own colour variables (--bg, --fg, --mut and so on) and each one is an alias of a module token, so
// the colours of the page are the module's, and the module's look files are what the contrast tests read. The helpers here read a look's token file the way the cascade does and
// resolve the editor's aliases through it; the tests of this file check every look the module has, in the dark theme and in Dayside.

var (
	reLookComment = regexp.MustCompile(`(?s)/\*.*?\*/`)
	reLookDecl    = regexp.MustCompile(`(--[a-z0-9-]+)\s*:\s*([^;]+);`)
	reLookVar     = regexp.MustCompile(`^var\((--[a-z0-9-]+)\)$`)
	reLookHex     = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	reRootDark    = regexp.MustCompile(`(?m)^:root \{([^}]*)\}`)
	reRootLight   = regexp.MustCompile(`(?m)^:root\[data-theme="light"\] \{([^}]*)\}`)
)

// lookTokens returns the colour tokens of a look as plain #rrggbb colours: the dark theme (the :root blocks), and Dayside (the same, with the light block on top). A value that is
// another token (var(--x)) is followed; a value that is not a plain colour (a gradient, an rgba) is left out.
func lookTokens(t *testing.T, look string) (dark, light map[string]string) {
	t.Helper()
	b, err := fs.ReadFile(design.Assets, "tokens-"+look+".css")
	if err != nil {
		t.Fatal(err)
	}
	css := reLookComment.ReplaceAllString(strings.ReplaceAll(string(b), "\r", ""), "")
	rawDark, rawLight := map[string]string{}, map[string]string{}
	for _, m := range reRootDark.FindAllStringSubmatch(css, -1) {
		for _, d := range reLookDecl.FindAllStringSubmatch(m[1], -1) {
			rawDark[d[1]] = strings.TrimSpace(d[2])
		}
	}
	for k, v := range rawDark {
		rawLight[k] = v
	}
	for _, m := range reRootLight.FindAllStringSubmatch(css, -1) {
		for _, d := range reLookDecl.FindAllStringSubmatch(m[1], -1) {
			rawLight[d[1]] = strings.TrimSpace(d[2])
		}
	}
	return resolveColours(rawDark), resolveColours(rawLight)
}

func resolveColours(raw map[string]string) map[string]string {
	out := map[string]string{}
	var one func(name string, depth int) string
	one = func(name string, depth int) string {
		v := strings.TrimSpace(raw[name])
		if depth > 8 || v == "" {
			return ""
		}
		if m := reLookVar.FindStringSubmatch(v); m != nil {
			return one(m[1], depth+1)
		}
		if !reLookHex.MatchString(v) {
			return ""
		}
		if len(v) == 4 {
			v = "#" + strings.Repeat(string(v[1]), 2) + strings.Repeat(string(v[2]), 2) + strings.Repeat(string(v[3]), 2)
		}
		return strings.ToLower(v)
	}
	for k := range raw {
		if c := one(k, 0); c != "" {
			out[k] = c
		}
	}
	return out
}

// editorAliases reads the first :root block of app.css: the editor's own colour names and the module token each one stands for.
func editorAliases(t *testing.T) map[string]string {
	t.Helper()
	b, err := os.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^:root\{([^}]*)\}`).FindStringSubmatch(strings.ReplaceAll(string(b), "\r", ""))
	if m == nil {
		t.Fatal("app.css has no :root block with the page's colour names")
	}
	out := map[string]string{}
	for _, d := range reLookDecl.FindAllStringSubmatch(m[1]+";", -1) {
		out[d[1]] = strings.TrimSpace(d[2])
	}
	return out
}

// editorTheme is a look's theme as the editor sees it: every module token, and every name of app.css that is an alias of one, as plain colours.
func editorTheme(t *testing.T, tokens map[string]string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for k, v := range tokens {
		out[k] = v
	}
	for name, val := range editorAliases(t) {
		if m := reLookVar.FindStringSubmatch(val); m != nil {
			if c := tokens[m[1]]; c != "" {
				out[name] = c
			}
		}
	}
	return out
}

// forEachLookTheme runs f for every look of the module in the dark theme and in Dayside, with the editor's colour names resolved for it.
func forEachLookTheme(t *testing.T, f func(t *testing.T, theme map[string]string)) {
	t.Helper()
	for _, look := range design.Looks() {
		dark, light := lookTokens(t, look)
		for name, tokens := range map[string]map[string]string{"dark": dark, "Dayside": light} {
			theme := editorTheme(t, tokens)
			t.Run(look+"/"+name, func(t *testing.T) { f(t, theme) })
		}
	}
}

// Every colour name of the editor's stylesheet is an alias of a module token that every look defines, in both themes: a look that lacked one would leave a rule with no colour.
func TestEveryEditorColourNameIsAnAliasOfATokenOfEveryLook(t *testing.T) {
	aliases := editorAliases(t)
	var names []string
	for k, v := range aliases {
		if strings.HasPrefix(v, "var(--c-") {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	for _, must := range []string{"--bg", "--panel", "--panel2", "--in", "--line", "--line-strong", "--fg", "--mut", "--mut-tint", "--acc", "--acc2", "--ok", "--bad", "--warn", "--focus"} {
		if aliases[must] == "" || !strings.HasPrefix(aliases[must], "var(--c-") {
			t.Errorf("%s is not an alias of a module colour token in app.css (it is %q)", must, aliases[must])
		}
	}
	forEachLookTheme(t, func(t *testing.T, theme map[string]string) {
		for _, n := range names {
			if theme[n] == "" {
				t.Errorf("%s (%s) is not a plain colour in this look", n, aliases[n])
			}
		}
	})
}

// Text reaches 4.5:1 and a rule or the focus ring 3:1 on the surfaces the editor draws them on, in every look and in both themes (the module checks its own pairs; this checks the ones
// the editor's names stand for).
func TestEditorTextRulesAndFocusReachTheirRatiosInEveryLookAndTheme(t *testing.T) {
	forEachLookTheme(t, func(t *testing.T, theme map[string]string) {
		for _, text := range []string{"--fg", "--mut", "--mut-tint", "--ok", "--bad", "--warn", "--acc", "--acc2", "--c-sky"} {
			for _, bg := range []string{"--bg", "--panel", "--panel2", "--in"} {
				if theme[text] == "" || theme[bg] == "" {
					t.Fatalf("%s or %s is not defined", text, bg)
				}
				if c := contrast(theme[text], theme[bg]); c < 4.5 {
					t.Errorf("%s (%s) on %s (%s) is %.2f:1; text needs 4.5:1", text, theme[text], bg, theme[bg], c)
				}
			}
		}
		for _, line := range []string{"--line-strong", "--focus"} {
			for _, bg := range []string{"--bg", "--panel", "--panel2", "--in"} {
				if c := contrast(theme[line], theme[bg]); c < 3 {
					t.Errorf("%s (%s) on %s (%s) is %.2f:1; a control's edge and the focus ring need 3:1", line, theme[line], bg, theme[bg], c)
				}
			}
		}
		// the label of a filled button is the module's pair (a primary button is drawn by the module, never by this stylesheet)
		for _, pair := range [][2]string{{"--c-on-switch", "--c-switch"}, {"--c-on-spice", "--c-spice"}} {
			if c := contrast(theme[pair[0]], theme[pair[1]]); c < 4.5 {
				t.Errorf("%s on %s is %.2f:1; the label of a filled button needs 4.5:1", pair[0], pair[1], c)
			}
		}
	})
}

// The page's own tinted bands (a warning, an information line, a pill) are drawn from the stylesheet's color-mix rules: body text and muted text reach 4.5:1 on each tint, in every look
// and theme.
func TestEditorBannerTintsKeepTheirTextReadableInEveryLookAndTheme(t *testing.T) {
	b, err := os.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	css := strings.ReplaceAll(string(b), "\r", "")
	re := regexp.MustCompile(`([^{}]*)\{[^{}]*background:color-mix\(in srgb,var\((--[a-z0-9-]+)\) (\d+)%,var\((--[a-z0-9-]+)\)\)`)
	tints := re.FindAllStringSubmatch(css, -1)
	if len(tints) < 4 {
		t.Fatalf("expected the tinted band backgrounds in the stylesheet, found %d", len(tints))
	}
	forEachLookTheme(t, func(t *testing.T, theme map[string]string) {
		for _, m := range tints {
			p := float64(atoiOr0(m[3])) / 100
			bg := mixHex(theme[m[2]], theme[m[4]], p)
			for _, text := range []string{"--fg", "--mut-tint", "--warn"} {
				if !strings.Contains(m[1], "span.warn") && text == "--warn" {
					continue // the warning colour is text only on the pill (span.warn), which carries a word
				}
				if c := contrast(theme[text], bg); c < 4.5 {
					t.Errorf("%s (%s) on the band %q (%s) is %.2f:1; text needs 4.5:1", text, theme[text], strings.TrimSpace(m[1]), bg, c)
				}
			}
		}
	})
}

func atoiOr0(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// Every page the editor serves links the four files of the default look, in the module's order, before app.css; the old palettes are gone from app.css (a hex colour in it is a
// map or marker colour, which never changes with a look).
func TestPagesLinkTheDefaultLookBeforeAppCSS(t *testing.T) {
	files, ok := design.LookFiles(design.DefaultLook)
	if !ok || len(files) != 4 {
		t.Fatalf("LookFiles(%q) = %v, %v", design.DefaultLook, files, ok)
	}
	for _, page := range []string{"static/index.html", "static/locked.html"} {
		b, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		s := strings.ReplaceAll(string(b), "\r", "")
		at := -1
		for _, f := range files {
			i := strings.Index(s, `<link rel="stylesheet" href="/design/`+f+`">`)
			if i < 0 || i < at {
				t.Errorf("%s: /design/%s at %d (after %d): the look's files must be linked in the module's order", page, f, i, at)
			}
			at = i
		}
		if app := strings.Index(s, `<link rel="stylesheet" href="/app.css">`); app < at {
			t.Errorf("%s: app.css at %d comes before the look's last file at %d", page, app, at)
		}
		if strings.Contains(s, "/design/tokens.css") {
			t.Errorf("%s links tokens.css beside the look's own tokens", page)
		}
	}
}
