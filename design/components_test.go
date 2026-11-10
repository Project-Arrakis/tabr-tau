package design

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Checks of the component stylesheets (design review register, checks 1, 6, 7, 8 and 10; public issue 574).

// look's stylesheets in link order, parsed
func lookRules(t *testing.T, look string) []rule {
	t.Helper()
	var all []rule
	files, _ := LookFiles(look)
	for _, f := range files[1:] { // the tokens file is read separately
		all = append(all, readCSS(t, f)...)
	}
	return all
}

func items(sel string) []string {
	var out []string
	for _, s := range splitTop(sel, ',') {
		out = append(out, strings.TrimSpace(s))
	}
	return out
}

func has(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// properties that an app sets on a part, or that a rule sets on itself, and that therefore need no token
var appSet = map[string]bool{
	"--v": true, "--w": true, "--x": true, "--lvl-from": true, "--attn": true, "--edge": true, "--cut": true, "--fill": true, "--rail-w": true,
	"--mo-pop": true, "--mo-shift": true, "--mo-low": true, "--mo-delay": true,
}

var useVarRE = regexp.MustCompile(`var\(\s*(--[a-z0-9-]+)\s*(,)?`)

// Every var() in a look's components names a token its tokens file declares, or a property the part sets itself or an app sets, or has a fallback.
func TestEveryVarResolves(t *testing.T) {
	for _, l := range looks {
		p := loadPalette(t, l, "dark")
		for _, r := range lookRules(t, l) {
			for _, d := range r.decls {
				for _, m := range useVarRE.FindAllStringSubmatch(d.value, -1) {
					name := m[1]
					if _, ok := p.raw[name]; ok || appSet[name] || m[2] == "," {
						continue
					}
					// a property declared by some rule of the stylesheets (--edge on a part, --cut)
					declared := false
					for _, r2 := range lookRules(t, l) {
						for _, d2 := range r2.decls {
							if d2.name == name {
								declared = true
							}
						}
					}
					if !declared {
						t.Errorf("%s: %s uses var(%s), which %s does not declare", r.file, r.selector, name, tokenFile(l))
					}
				}
			}
		}
	}
}

// No component styles an app's class (x-) and the stylesheets hold no id selector.
func TestComponentsStyleTheirOwnClassesOnly(t *testing.T) {
	for _, f := range []string{"components-fremen.css", "components-awakening.css", "components-sietch.css", "components.css", "motion.css"} {
		for _, r := range readCSS(t, f) {
			if strings.HasPrefix(r.at, "@keyframes") {
				continue
			}
			if regexp.MustCompile(`\.x-`).MatchString(r.selector) {
				t.Errorf("%s: %s names a class of an app (x-)", f, r.selector)
			}
			if regexp.MustCompile(`(^|[^&\w-])#[a-zA-Z]`).MatchString(r.selector) {
				t.Errorf("%s: %s is an id selector", f, r.selector)
			}
		}
	}
}

// ---- text size (check 8) ----

var fontShortRE = regexp.MustCompile(`^(?:(?:italic|normal|bold|\d{3})\s+)*(\d+(?:\.\d+)?)px`)

// No text is under 11 px in the components or in the kitchen sink's layout.
func TestNoTextUnder11px(t *testing.T) {
	files := []string{"components-fremen.css", "components-awakening.css", "components-sietch.css", "components.css", "motion.css"}
	for _, f := range files {
		for _, r := range readCSS(t, f) {
			for _, d := range r.decls {
				size := ""
				switch d.name {
				case "font-size":
					size = d.value
				case "font":
					if m := fontShortRE.FindStringSubmatch(d.value); m != nil {
						size = m[1] + "px"
					}
				}
				if strings.HasSuffix(size, "px") {
					if v, _ := strconv.ParseFloat(strings.TrimSuffix(size, "px"), 64); v < 11 {
						t.Errorf("%s: %s sets text at %s, the smallest text is 11 px", f, r.selector, size)
					}
				}
			}
		}
	}
}

// ---- targets (check 8) ----

// Every part a person can press is at least 24 px in each direction; the pointer's size is --hit (32 px) and a touch screen's 44 px.
func TestTargetSizes(t *testing.T) {
	min := map[string]string{ // selector -> the declaration that gives its height
		".d-btn": "min-height", ".d-btn.sm": "min-height", ".d-seg button": "min-height", ".d-input, .d-select": "min-height", ".d-nav a, .d-nav button": "min-height",
		".d-range": "height", ".d-row": "min-height", ".d-alert": "min-height", ".d-more > summary": "min-height",
	}
	for _, l := range looks {
		p := loadPalette(t, l, "dark")
		rs := lookRules(t, l)
		for sel, prop := range min {
			found := false
			for _, r := range rs {
				if r.at != "" || r.selector != sel {
					continue
				}
				if v, ok := r.get(prop); ok {
					found = true
					if px, ok := p.px(t, v); !ok || px < 24 {
						t.Errorf("%s: %s has %s %s, the least a person can press is 24 px", l, sel, prop, v)
					}
				}
			}
			if !found {
				t.Errorf("%s: no %s on %s", l, prop, sel)
			}
		}
	}
	// the switch is 30 x 16 px, so a pseudo-element reaches its target past the track
	var toggleBefore, coarse string
	for _, r := range readCSS(t, "components.css") {
		if r.at == "" && r.selector == ".d-toggle::before" {
			toggleBefore, _ = r.get("inset")
		}
		if strings.HasPrefix(r.at, "@media (pointer: coarse)") && r.selector == ".d-toggle::before" {
			coarse, _ = r.get("inset")
		}
		if strings.HasPrefix(r.at, "@media (pointer: coarse)") && r.selector == ":root" {
			if v, _ := r.get("--hit"); v != "44px" {
				t.Errorf("under pointer: coarse --hit is %q, want 44px", v)
			}
		}
	}
	reach := func(inset string) (w, h float64) {
		f := strings.Fields(inset)
		if len(f) != 2 {
			t.Fatalf("inset %q is not two lengths", inset)
		}
		v, _ := strconv.ParseFloat(strings.TrimSuffix(f[0], "px"), 64)
		hx, _ := strconv.ParseFloat(strings.TrimSuffix(f[1], "px"), 64)
		return 30 - 2*hx, 16 - 2*v
	}
	if w, h := reach(toggleBefore); w < 24 || h < 24 {
		t.Errorf("a switch's target is %vx%v px, under 24", w, h)
	}
	if w, h := reach(coarse); w < 44 || h < 44 {
		t.Errorf("a switch's target under pointer: coarse is %vx%v px, under 44", w, h)
	}
}

// ---- focus (check 6) ----

var containerClasses = map[string]bool{"d-panel": true, "d-card": true, "d-pop": true, "d-modal": true, "d-seg": true}

// A part with a clip-path (a chamfer) clips its own outline and its children's. Every selector of the Awakening look that has one must be named by a rule
// that removes the clip while the focus ring shows (the part itself focused, or a part that holds focusable things while one of them is), with a higher
// specificity than the clip's own rule; the looks without a chamfer have no clip-path at all.
func TestFocusRingIsNeverClipped(t *testing.T) {
	for _, l := range looks {
		rs := lookRules(t, l)
		var cuts []rule
		var unclips []rule
		for _, r := range rs {
			if r.at != "" {
				continue
			}
			if v, ok := r.get("clip-path"); ok {
				if v == "none" {
					unclips = append(unclips, r)
				} else {
					cuts = append(cuts, r)
				}
			}
		}
		if l != "awakening" {
			if len(cuts) != 0 {
				t.Errorf("%s: has a clip-path (%s); only the Awakening look chamfers", l, cuts[0].selector)
			}
			continue
		}
		if len(cuts) < 2 {
			t.Fatalf("awakening: expected the cut rules, found %d", len(cuts))
		}
		for _, c := range cuts {
			for _, sel := range items(c.selector) {
				class := lastClass(sel)
				if class == "d-tip" {
					continue // a tooltip is neither focused nor holds focus
				}
				self, holds := false, !containerClasses[class]
				for _, u := range unclips {
					for _, us := range items(u.selector) {
						if !strings.Contains(us, "."+class+",") && !strings.Contains(us, "."+class+")") && !strings.HasSuffix(us, "."+class) && !strings.Contains(us, "."+class+":") {
							continue
						}
						if specificity(us).less(specificity(sel)) || specificity(us) == specificity(sel) {
							continue
						}
						if strings.Contains(us, ":has(:focus-visible)") {
							holds = true
						}
						if strings.HasSuffix(us, ":focus-visible") {
							self = true
						}
					}
				}
				if !self {
					t.Errorf("awakening: %s has a clip-path and no :focus-visible rule of higher specificity removes it", sel)
				}
				if !holds {
					t.Errorf("awakening: %s holds focusable parts and no :has(:focus-visible) rule of higher specificity removes its clip-path", sel)
				}
			}
		}
		// the generic ring is defined, and nothing switches an outline off
		ring := false
		for _, r := range rs {
			if r.selector == ":focus-visible" {
				if v, _ := r.get("outline"); strings.Contains(v, "var(--c-focus)") {
					ring = true
				}
			}
			for _, d := range r.decls {
				if d.name == "outline" && (d.value == "none" || d.value == "0") {
					t.Errorf("%s: %s switches the outline off", r.file, r.selector)
				}
			}
		}
		if !ring {
			t.Error("awakening: no :focus-visible rule draws the ring in --c-focus")
		}
	}
	for _, l := range []string{"fremen", "sietch"} {
		ring := false
		for _, r := range lookRules(t, l) {
			if r.selector == ":focus-visible" {
				if v, _ := r.get("outline"); strings.Contains(v, "var(--c-focus)") {
					ring = true
				}
			}
		}
		if !ring {
			t.Errorf("%s: no :focus-visible rule draws the ring in --c-focus", l)
		}
	}
}

var classRE = regexp.MustCompile(`\.([a-zA-Z][\w-]*)`)

// lastClass is the d- class of the last compound of a selector: ".d-tt > .d-tip" gives d-tip, "button.d-cut-tab" gives d-cut-tab.
func lastClass(sel string) string {
	parts := regexp.MustCompile(`\s*[>+~]\s*|\s+`).Split(sel, -1)
	last := parts[len(parts)-1]
	for i := len(parts) - 1; i >= 0 && last == ""; i-- {
		last = parts[i]
	}
	if m := classRE.FindStringSubmatch(last); m != nil {
		return m[1]
	}
	return ""
}

func TestSpecificityHelper(t *testing.T) {
	for sel, want := range map[string]spec{
		".d-panel":                                     {0, 1, 0},
		"button.d-cut-tab:focus-visible":               {0, 2, 1},
		":is(.d-a, .d-b):focus-visible":                {0, 2, 0},
		":is(.d-a, .d-b):has(:focus-visible)":          {0, 2, 0},
		".d-tt > .d-tip":                               {0, 2, 0},
		`html .d-tt.dismissed > .d-tip`:                {0, 3, 1},
		`html:not([data-motion-src="user"]) .d-x`:      {0, 2, 1},
		":where(.a .b) .c":                             {0, 1, 0},
		".d-panel::before":                             {0, 1, 1},
		`:root[data-theme="light"]`:                    {0, 2, 0},
		`:is(.a):has(.d-tt:is(:hover, :focus-within))`: {0, 3, 0},
	} {
		if got := specificity(sel); got != want {
			t.Errorf("specificity(%q) = %v, want %v", sel, got, want)
		}
	}
}

// ---- motion (check 7) ----

var timeRE = regexp.MustCompile(`(^|[\s,(])(-?\d*\.?\d+)(ms|s)\b`)

func toMS(v string) (float64, bool) {
	m := regexp.MustCompile(`^(-?\d*\.?\d+)(ms|s)$`).FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return 0, false
	}
	f, _ := strconv.ParseFloat(m[1], 64)
	if m[2] == "s" {
		f *= 1000
	}
	return f, true
}

type anim struct {
	name     string
	looping  bool
	duration string // the var(--mo-x) or literal time
}

// parseAnimation reads one animation shorthand (one comma-separated part).
func parseAnimation(part string) anim {
	a := anim{}
	for _, tok := range splitTop(strings.TrimSpace(part), ' ') {
		tok = strings.TrimSpace(tok)
		switch {
		case tok == "":
		case tok == "infinite":
			a.looping = true
		case regexp.MustCompile(`^\d+$`).MatchString(tok):
			if n, _ := strconv.Atoi(tok); n > 1 {
				a.looping = true
			}
		case strings.HasPrefix(tok, "var(--mo-") || regexp.MustCompile(`^-?\d*\.?\d+m?s$`).MatchString(tok):
			if a.duration == "" {
				a.duration = tok
			}
		case strings.HasPrefix(tok, "d-") || strings.HasPrefix(tok, "rm-"):
			a.name = tok
		}
	}
	return a
}

// Every animation has a reduced-motion answer, no loop is faster than 333 ms, and every duration is a token the Motion setting can reach.
func TestMotion(t *testing.T) {
	motion := readCSS(t, "motion.css")
	zeroed := func(sel string, tokens ...string) bool { // is each token 0.01ms in the block for sel
		for _, tok := range tokens {
			found := false
			for _, r := range motion {
				if strings.Contains(r.selector, sel) {
					if v, ok := r.get(tok); ok {
						if ms, ok := toMS(v); ok && ms <= 1 {
							found = true
						}
					}
				}
			}
			if !found {
				return false
			}
		}
		return true
	}
	stopped := map[string]bool{} // "prefix|selector" -> animation none (or not looping) in motion.css
	for _, r := range motion {
		v, ok := r.get("animation")
		if !ok {
			continue
		}
		if v == "none" || !parseAnimation(strings.Split(v, ",")[0]).looping {
			for _, s := range items(r.selector) {
				stopped[s] = true
			}
		}
	}
	prefixes := []string{`html[data-motion="calm"] `, `html[data-motion="off"] `, `html:not([data-motion-src="user"]) `}

	keyframes := map[string]bool{}
	for _, f := range []string{"components-fremen.css", "components-awakening.css", "components-sietch.css", "motion.css"} {
		for _, r := range readCSS(t, f) {
			if strings.HasPrefix(r.at, "@keyframes ") {
				keyframes[strings.TrimPrefix(r.at, "@keyframes ")] = true
			}
		}
	}
	used := map[string]bool{}
	for _, l := range looks {
		p := loadPalette(t, l, "dark")
		files, _ := LookFiles(l)
		for _, f := range files[1:] {
			for _, r := range readCSS(t, f) {
				if r.at != "" && !strings.HasPrefix(r.at, "@media") {
					continue
				}
				for _, d := range r.decls {
					if d.name != "animation" && d.name != "transition" {
						continue
					}
					if d.value == "none" {
						continue
					}
					for _, part := range splitTop(d.value, ',') {
						// a literal duration (other than 0s) would not follow the Motion setting
						for _, m := range timeRE.FindAllStringSubmatch(strings.ReplaceAll(part, "var(", " "), -1) {
							if v, _ := strconv.ParseFloat(m[2], 64); v != 0 && !strings.Contains(part, "var(--mo-") {
								t.Errorf("%s: %s: %s has the literal time %s%s; use a --mo- token", f, r.selector, d.name, m[2], m[3])
							}
						}
						if d.name != "animation" {
							continue
						}
						a := parseAnimation(part)
						if a.name == "" {
							continue
						}
						used[a.name] = true
						if !keyframes[a.name] {
							t.Errorf("%s: %s uses @keyframes %s, which does not exist", f, r.selector, a.name)
						}
						if !a.looping {
							continue
						}
						// a loop: slow enough, and stopped in Calm, in Off and by the system setting
						dur := a.duration
						if m := varRE.FindStringSubmatch(dur); m != nil {
							dur = p.value(t, m[1])
						}
						ms, ok := toMS(dur)
						if !ok || ms < 333 {
							t.Errorf("%s: %s: the loop %s runs every %v ms, under 333 ms", f, r.selector, a.name, ms)
						}
						for _, sel := range items(r.selector) {
							for _, pre := range prefixes {
								if stopped[pre+sel] {
									continue
								}
								// a short run of a few cycles whose duration the setting zeroes is also stopped
								tokens := []string{}
								if m := varRE.FindStringSubmatch(a.duration); m != nil {
									tokens = append(tokens, m[1])
								}
								if len(tokens) == 1 && zeroed(strings.TrimSpace(pre), tokens...) {
									continue
								}
								t.Errorf("%s: the loop %s on %s has no rule that stops it under %q", f, a.name, sel, strings.TrimSpace(pre))
							}
						}
					}
				}
			}
		}
	}
	for _, f := range []string{"components-fremen.css", "components-awakening.css", "components-sietch.css", "components.css", "motion.css"} {
		for _, r := range readCSS(t, f) {
			if v, ok := r.get("animation"); ok && v != "none" {
				for _, part := range splitTop(v, ',') {
					if a := parseAnimation(part); a.name != "" && !keyframes[a.name] {
						t.Errorf("%s: %s uses @keyframes %s, which does not exist", f, r.selector, a.name)
					}
				}
			}
		}
	}
	// the three settings and the system default exist
	need := []string{`html[data-motion="calm"]`, `html[data-motion="off"]`, `html:not([data-motion-src="user"]):not([data-motion="off"])`, `html[data-motion="off"] *`, `html[data-doc-hidden] *`}
	for _, n := range need {
		found := false
		for _, r := range motion {
			if has(items(r.selector), n) {
				found = true
			}
		}
		if !found {
			t.Errorf("motion.css has no rule for %s", n)
		}
	}
	for _, r := range motion {
		if r.selector == `html[data-motion="off"] *` {
			if v, _ := r.get("animation"); !strings.HasPrefix(v, "none") {
				t.Error("Off does not stop animations")
			}
			if v, _ := r.get("transition"); !strings.HasPrefix(v, "none") {
				t.Error("Off does not stop transitions")
			}
		}
	}
	if len(used) < 8 {
		t.Errorf("read only %d animations from the looks", len(used))
	}
	// the periods of the loops, in every look, are the slow ones
	for _, l := range looks {
		p := loadPalette(t, l, "dark")
		for _, n := range []string{"--mo-breathe", "--mo-spin", "--mo-shimmer", "--mo-low", "--mo-drift", "--mo-ping-player", "--mo-ping-giant"} {
			ms, ok := toMS(p.value(t, n))
			if !ok || ms < 333 {
				t.Errorf("%s: %s = %q, a loop is never faster than 333 ms", l, n, p.raw[n])
			}
		}
	}
}

// ---- headings (register A10) ----

var headingSelectors = []string{".d-word", ".d-word small", ".d-head", ".d-pop h4", ".d-modal h3", ".d-table th", ".d-empty h3", ".d-display"}

// Dune Rise is named by --t-heading and by the @font-face only; the body, the monospace face and the display stack never start with it, and the one rule
// that applies it lists fixed headings (no number, button, field or name).
func TestTheHeadingFaceIsForHeadingsOnly(t *testing.T) {
	for _, l := range looks {
		for _, r := range readCSS(t, tokenFile(l)) {
			for _, d := range r.decls {
				if strings.Contains(d.value, "Dune Rise") && d.name != "--t-heading" && d.name != "font-family" {
					t.Errorf("%s: %s sets %s to %q; only --t-heading names the face", tokenFile(l), r.selector, d.name, d.value)
				}
				if strings.Contains(d.value, "Dune Rise") && d.name == "font-family" && r.selector != "@font-face" {
					t.Errorf("%s: %s uses the face outside @font-face", tokenFile(l), r.selector)
				}
			}
		}
		p := loadPalette(t, l, "dark")
		if v := p.raw["--t-heading"]; !strings.HasPrefix(v, `"Dune Rise", var(--t-display)`) {
			t.Errorf("%s: --t-heading is %q", l, v)
		}
		applied := 0
		for _, r := range lookRules(t, l) {
			if v, ok := r.get("font-family"); ok && strings.Contains(v, "--t-heading") {
				applied++
				var got []string
				m := regexp.MustCompile(`:is\((.*)\)\s*$`).FindStringSubmatch(r.selector)
				if m == nil || !strings.HasPrefix(r.selector, `html:not([data-heading-font="system"]) :is(`) {
					t.Errorf("%s: the heading rule is %q", l, r.selector)
					continue
				}
				for _, s := range splitTop(m[1], ',') {
					got = append(got, strings.TrimSpace(s))
				}
				sort.Strings(got)
				want := append([]string{}, headingSelectors...)
				sort.Strings(want)
				if strings.Join(got, "|") != strings.Join(want, "|") {
					t.Errorf("%s: the heading rule names %v, want %v", l, got, want)
				}
				if v, _ := r.get("text-transform"); v != "uppercase" {
					t.Errorf("%s: headings in Dune Rise are set in capitals", l)
				}
			}
			for _, d := range r.decls {
				if d.name == "font-family" && strings.Contains(d.value, "Dune") && !strings.Contains(d.value, "--t-heading") {
					t.Errorf("%s: %s names the face directly", r.file, r.selector)
				}
				if (d.name == "font" || d.name == "font-family") && strings.Contains(d.value, "--t-heading") && !strings.Contains(r.selector, "data-heading-font") && !strings.Contains(r.selector, ".d-display") {
					t.Errorf("%s: %s uses --t-heading outside the heading rule", r.file, r.selector)
				}
			}
		}
		if applied != 1 {
			t.Errorf("%s: %d rules apply the heading face, want exactly 1", l, applied)
		}
	}
}

// ---- forced colours (check 10) ----

var forcedStates = []string{
	".d-toggle:checked", ".d-toggle:indeterminate", `.d-seg button[aria-pressed="true"]`, `.d-nav [aria-current="page"]`, ".d-pill.live", ".d-pill.paused",
	".d-pill.offline::before", ".d-cert.live", ".d-cert.saved", ".d-cert.possible", ".d-meter.low .d-val", ".d-meter.crit .d-val", ".d-btn.primary", ".d-btn.danger",
	".d-btn.quiet", ".d-alert.bad", ".d-banner.bad", ".d-toast.warn", ".d-toast.bad", ".d-card.pinned", ".d-row.off .d-name", ".d-btn:hover:not(:disabled)", ".d-btn:disabled",
}

// A forced-colours block replaces gradients, shadows and fills with real borders, and every state a fill shows also shows in a border, a text colour or a mark.
func TestForcedColoursBlock(t *testing.T) {
	var block []rule
	for _, r := range readCSS(t, "components.css") {
		if r.at == "@media (forced-colors: active)" {
			block = append(block, r)
		}
	}
	if len(block) < 20 {
		t.Fatalf("components.css has %d rules in @media (forced-colors: active)", len(block))
	}
	visible := map[string]bool{"border": true, "border-width": true, "border-style": true, "border-color": true, "border-left-width": true, "border-left-style": true, "border-bottom": true,
		"outline": true, "color": true, "text-decoration": true, "font-weight": true, "background-color": true}
	for _, s := range forcedStates {
		ok := false
		for _, r := range block {
			if has(items(r.selector), s) {
				for _, d := range r.decls {
					if visible[d.name] {
						ok = true
					}
				}
			}
		}
		if !ok {
			t.Errorf("forced colours: the state %s has no border, text colour or decoration of its own in the block", s)
		}
	}
	// the cut and gradient parts get a real border; a switch's on state does not depend on a background colour alone
	for _, r := range block {
		if strings.Contains(r.selector, ".d-panel") && strings.Contains(r.selector, ".d-btn") {
			for _, want := range []string{"border", "clip-path", "background-image"} {
				if _, ok := r.get(want); !ok {
					t.Errorf("forced colours: the part rule lacks %s", want)
				}
			}
			if v, _ := r.get("border"); !strings.Contains(v, "CanvasText") {
				t.Errorf("forced colours: parts get a border in a system colour, got %q", v)
			}
		}
		if has(items(r.selector), ".d-toggle:checked") {
			if _, ok := r.get("border-color"); !ok {
				t.Error("forced colours: a switch that is on changes its border as well as its fill")
			}
		}
		for _, d := range r.decls {
			if d.name == "outline" && (d.value == "none" || d.value == "0") {
				t.Errorf("forced colours: %s removes the outline", r.selector)
			}
			// only system colours in the block
			for _, bad := range regexp.MustCompile(`#[0-9a-fA-F]{3,6}\b|rgba?\(|var\(--c-`).FindAllString(d.value, -1) {
				if d.name != "background" || bad != "rgba(" { // the modal scrim may stay translucent
					t.Errorf("forced colours: %s sets %s to %q, which is not a system colour", r.selector, d.name, d.value)
				}
			}
		}
	}
	// the contrast preference exists
	found := false
	for _, r := range readCSS(t, "components.css") {
		if r.at == "@media (prefers-contrast: more)" {
			found = true
		}
	}
	if !found {
		t.Error("no @media (prefers-contrast: more) block")
	}
}

// ---- tooltips (register A2) ----

func TestTooltipsCanBeReached(t *testing.T) {
	for _, l := range looks {
		for _, r := range lookRules(t, l) {
			if r.file == "motion.css" {
				continue
			}
			if strings.Contains(r.selector, ".d-tip") {
				if v, ok := r.get("pointer-events"); ok && v == "none" && r.file != "components.css" {
					// the look sets none; components.css (later, same specificity) must set auto
					auto := false
					for _, r2 := range readCSS(t, "components.css") {
						if r2.selector == ".d-tt > .d-tip" {
							if v2, _ := r2.get("pointer-events"); v2 == "auto" {
								auto = true
							}
						}
					}
					if !auto {
						t.Errorf("%s: %s has pointer-events: none and components.css does not restore it", r.file, r.selector)
					}
				}
			}
		}
	}
	css := readFile(t, "assets/components.css")
	for _, must := range []string{".d-tt > .d-tip { pointer-events: auto; }", "dismissed", "aria-describedby"} {
		if !strings.Contains(css, must) {
			t.Errorf("components.css lacks %q", must)
		}
	}
}
