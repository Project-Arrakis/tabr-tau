package design

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// assets/palettes.css is generated from the table below by the rules in this file, and a test compares the committed file with what the rules
// make. To change a colour or a rule, change it here and run
//
//	go test -run TestPalettesCSSIsGenerated -update-palettes
//
// The table holds the thirteen colours of each palette as they come from their source (the grounds, lines, text, accent and status colours of
// the fan-made editor themes under the MIT Licence that NOTICE names; alpha channels removed, nothing else changed). The rules then move a
// colour toward white (a dark palette) or toward black (a light palette), in steps of 4 %, only as far as the checks of this module need, and
// the generated file says in a comment which colours of a palette were moved.

var updatePalettes = flag.Bool("update-palettes", false, "write assets/palettes.css from the table in palettes_gen_test.go")

type paletteSource struct {
	id, name, group, theme                               string
	ground, panel, raised, hair, rule, text, dim, accent string
	ok, warn, bad, info, secondary                       string
}

var paletteSources = []paletteSource{
	{id: "warm-charcoal", name: "Arrakis Incandescent", group: "Planet", theme: "dark", ground: "#151313", panel: "#1c1b1a", raised: "#282726", hair: "#343331", rule: "#878580", text: "#cecdc3", dim: "#878580", accent: "#5a96bc", ok: "#7fb069", warn: "#e8b04b", bad: "#c15849", info: "#5a96bc", secondary: "#e2ad4a"},
	{id: "umber-sand", name: "Sandworm", group: "Planet", theme: "dark", ground: "#1a1611", panel: "#221e17", raised: "#342d22", hair: "#5a4a36", rule: "#7a6548", text: "#e8dcc4", dim: "#9e8b6f", accent: "#5ca7e8", ok: "#8ca674", warn: "#e8a85c", bad: "#d67c5c", info: "#6cb4ee", secondary: "#e8b88c"},
	{id: "deep-teal", name: "Kaitain", group: "Planet", theme: "dark", ground: "#0f191c", panel: "#021015", raised: "#081e26", hair: "#1b3743", rule: "#5e8c80", text: "#a8bcc9", dim: "#8299ac", accent: "#3f9aca", ok: "#4a9a4a", warn: "#ccad31", bad: "#cc5555", info: "#55a2c2", secondary: "#d4a52a"},
	{id: "storm-blue", name: "Caladan Storm", group: "Planet", theme: "dark", ground: "#0d1117", panel: "#161b22", raised: "#272e38", hair: "#484f5b", rule: "#6e7681", text: "#e6edf3", dim: "#8b949e", accent: "#58a6ff", ok: "#7ee787", warn: "#ffa657", bad: "#ff7b72", info: "#79c0ff", secondary: "#ff9b54"},
	{id: "ember-red", name: "Salusa Secundus", group: "Planet", theme: "dark", ground: "#0f0d0d", panel: "#1a1616", raised: "#2e2828", hair: "#524848", rule: "#726565", text: "#e8d6d6", dim: "#9e8585", accent: "#ff6b6b", ok: "#c78860", warn: "#ffa500", bad: "#ff3838", info: "#ff8c42", secondary: "#ffb86c"},
	{id: "circuit-indigo", name: "Ix Technology", group: "Planet", theme: "dark", ground: "#0a0c0f", panel: "#121517", raised: "#242930", hair: "#4a5361", rule: "#6b7585", text: "#e8ecf0", dim: "#9099a8", accent: "#6272a4", ok: "#50fa7b", warn: "#f1b644", bad: "#ff5555", info: "#6272a4", secondary: "#ffb86c"},
	{id: "mono-grey", name: "Giedi Prime", group: "House & faction", theme: "dark", ground: "#0a0a0a", panel: "#121212", raised: "#252525", hair: "#2a2a2a", rule: "#7c7c7c", text: "#d4d4d4", dim: "#8c8c8c", accent: "#969696", ok: "#6a8e6a", warn: "#9e8a6a", bad: "#9e6a6a", info: "#6a7e9e", secondary: "#e3bdad"},
	{id: "cold-slate", name: "Harkonnen", group: "House & faction", theme: "dark", ground: "#090a0c", panel: "#1a1b21", raised: "#282931", hair: "#46474f", rule: "#565760", text: "#d4d7de", dim: "#b8bcc8", accent: "#6b8ebd", ok: "#8fb5a3", warn: "#c4c7a0", bad: "#c77374", info: "#8fa9d0", secondary: "#d2d895"},
	{id: "navy-blue", name: "House Atreides", group: "House & faction", theme: "dark", ground: "#11161c", panel: "#191f27", raised: "#28333d", hair: "#3f4d5c", rule: "#5a6b7e", text: "#e4edf7", dim: "#8b9cae", accent: "#3b82f6", ok: "#10b981", warn: "#f59e0b", bad: "#ef4444", info: "#3b82f6", secondary: "#22d3ee"},
	{id: "violet-iron", name: "Sardaukar Imperial", group: "House & faction", theme: "dark", ground: "#0c0c0e", panel: "#141418", raised: "#26262e", hair: "#4a4a58", rule: "#6a6a7c", text: "#e8e8f0", dim: "#9090a8", accent: "#9d6fcc", ok: "#6b9e6b", warn: "#ffaa40", bad: "#ff3838", info: "#8b72c7", secondary: "#ffd700"},
	{id: "ink-blue", name: "Bene Gesserit", group: "House & faction", theme: "dark", ground: "#08090d", panel: "#101318", raised: "#20252e", hair: "#404854", rule: "#606878", text: "#e0e4e8", dim: "#8890a0", accent: "#2c5f8d", ok: "#6b8c5f", warn: "#c9934e", bad: "#a65353", info: "#4a7c9e", secondary: "#c9934e"},
	{id: "desert-dusk", name: "Muad'Dib", group: "Character", theme: "dark", ground: "#0f0e0c", panel: "#181614", raised: "#2a2822", hair: "#4e4a40", rule: "#6e6858", text: "#e8e2d0", dim: "#988e78", accent: "#4a9eff", ok: "#7ca668", warn: "#e8a85c", bad: "#d67e60", info: "#5ca8e8", secondary: "#d4a85c"},
	{id: "sand-linen", name: "Fremen Sietch", group: "Character", theme: "light", ground: "#faf8f3", panel: "#f5f2ea", raised: "#e8e0d0", hair: "#b8a890", rule: "#8b7355", text: "#2c2416", dim: "#6b5d48", accent: "#1e6b8c", ok: "#4a7c59", warn: "#b45309", bad: "#c2410c", info: "#2a7f9e", secondary: "#8b6239"},
	{id: "void-violet", name: "Guild Navigator", group: "Guild & commerce", theme: "dark", ground: "#050308", panel: "#0d0a14", raised: "#1e1a26", hair: "#3f3848", rule: "#5e5468", text: "#f0e8ff", dim: "#9888b0", accent: "#4ca8ff", ok: "#69db7c", warn: "#ffa94d", bad: "#ff6b6b", info: "#5ca7ff", secondary: "#c8884c"},
	{id: "steel-night", name: "Spacing Guild", group: "Guild & commerce", theme: "dark", ground: "#0a0b0d", panel: "#12141a", raised: "#24272e", hair: "#484e58", rule: "#68707c", text: "#e8ecf4", dim: "#909aa8", accent: "#4a90e2", ok: "#6bc46d", warn: "#ffaa40", bad: "#e85855", info: "#5ca7ff", secondary: "#ff9940"},
	{id: "deep-ocean", name: "Water of Life", group: "Mystical & mental", theme: "dark", ground: "#0a0e14", panel: "#111621", raised: "#202938", hair: "#3f4e66", rule: "#5a6b88", text: "#e8f0ff", dim: "#8a9bb8", accent: "#5eb3ff", ok: "#48dbfb", warn: "#feca57", bad: "#ff6b9d", info: "#54a0ff", secondary: "#ffd700"},
	{id: "paper-grey", name: "Mentat", group: "Mystical & mental", theme: "light", ground: "#fafafa", panel: "#f4f4f4", raised: "#e8e8e8", hair: "#c0c0c0", rule: "#909090", text: "#1a1a1a", dim: "#606060", accent: "#0550ae", ok: "#1a7f37", warn: "#9a6700", bad: "#a40e26", info: "#0969da", secondary: "#cf222e"},
	{id: "copper-cyan", name: "Spice Vision", group: "Mystical & mental", theme: "dark", ground: "#140f0a", panel: "#1c1611", raised: "#2e251f", hair: "#524438", rule: "#6e5a48", text: "#ffedc6", dim: "#a08968", accent: "#00bfff", ok: "#26de81", warn: "#ffa502", bad: "#ff4757", info: "#00bfff", secondary: "#ffd700"},
}

// ---- colour arithmetic on whole-number channels, as a browser holds a hex colour ----

func palClamp(v float64) float64 {
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return float64(int(v + 0.5))
}

func palHex(c rgba) string {
	return fmt.Sprintf("#%02x%02x%02x", int(palClamp(c[0])), int(palClamp(c[1])), int(palClamp(c[2])))
}

func palRGB(t *testing.T, s string) rgba {
	t.Helper()
	c, ok := parseColor(s)
	if !ok {
		t.Fatalf("not a colour: %q", s)
	}
	return c
}

// palShift moves a colour by t (0..1) of the way toward another, and rounds to whole channels.
func palShift(a, to rgba, t float64) rgba {
	return rgba{palClamp(a[0] + (to[0]-a[0])*t), palClamp(a[1] + (to[1]-a[1])*t), palClamp(a[2] + (to[2]-a[2])*t), 1}
}

var (
	palWhite = rgba{255, 255, 255, 1}
	palBlack = rgba{0, 0, 0, 1}
	palInk   = rgba{16, 16, 16, 1} // the dark label on a fill: #101010
)

// palUntil moves c toward `to` in steps of 4 % until ok holds, or until it is `to` itself.
func palUntil(c, to rgba, ok func(rgba) bool) rgba {
	v := palShift(c, c, 0)
	for step := 1; !ok(v) && step <= 25; step++ {
		v = palShift(c, to, float64(step)*0.04)
	}
	return v
}

// palMin is the least contrast of c over the given colours.
func palMin(c rgba, over ...rgba) float64 {
	m := 99.0
	for _, o := range over {
		if r := contrast(c, o); r < m {
			m = r
		}
	}
	return m
}

// palLabel is the label on a fill: white or near-black, whichever reads better, and pure black where near-black is not enough.
func palLabel(fill rgba) rgba {
	best := palWhite
	if contrast(palInk, fill) > contrast(palWhite, fill) {
		best = palInk
	}
	if contrast(palBlack, fill) > contrast(best, fill) && contrast(best, fill) < 4.58 {
		best = palBlack
	}
	return best
}

func palDistance(a, b rgba) float64 {
	return (a[0]-b[0])*(a[0]-b[0]) + (a[1]-b[1])*(a[1]-b[1]) + (a[2]-b[2])*(a[2]-b[2])
}

func palRGBA(c rgba, a string) string {
	return fmt.Sprintf("rgba(%d, %d, %d, %s)", int(c[0]), int(c[1]), int(c[2]), a)
}

// ---- what the stylesheets of the looks say, read once ----

type palRules struct {
	warnHover float64 // the strongest hover tint of an alert (percent of its colour)
	flash     float64 // the flash of a changed row (percent of the accent)
	fillHover float64 // the least percent of a primary button's fill left when it is hovered (the rest is white)
}

func readPalRules(t *testing.T) palRules {
	t.Helper()
	r := palRules{fillHover: 100}
	for _, l := range looks {
		file := "components-" + l + ".css"
		if v := cssMixPct(t, file, ".d-alert:hover", "--c-warn"); v > r.warnHover {
			r.warnHover = v
		}
		fill := "--c-spice"
		if l == "fremen" {
			fill = "--c-blue"
		}
		if v := cssMixPct(t, file, ".d-btn.primary:hover:not(:disabled)", fill); v < r.fillHover {
			r.fillHover = v
		}
	}
	r.flash = cssMixPct(t, "motion.css", "from", "--c-spice")
	return r
}

// ---- the rules ----

type paletteTokens struct {
	decls [][2]string // in the order they are written
	moved []string
}

func (p *paletteTokens) set(name, value string) { p.decls = append(p.decls, [2]string{name, value}) }

// palDerive makes the tokens of a palette. The order of the steps matters: the colours that are text come first; then the accent (which is
// also a fill, with a label); then the text, which must hold on every tint the components compute from those colours; then the lines, the
// fills of the primary button and of the switch, and last the strength of the accent tints.
func palDerive(t *testing.T, s paletteSource, rules palRules) paletteTokens {
	t.Helper()
	dark := s.theme == "dark"
	toward, away := palWhite, palBlack
	if !dark {
		toward, away = palBlack, palWhite
	}
	var out paletteTokens
	note := func(name string, from, to rgba) {
		if palHex(from) != palHex(to) {
			out.moved = append(out.moved, name)
		}
	}
	ground, panel, raised := palRGB(t, s.ground), palRGB(t, s.panel), palRGB(t, s.raised)
	surf := []rgba{ground, panel, raised, ground} // ground, panel, raised and sunken (a palette's sunken is its ground)

	// colours that are text on the surfaces: 4.6:1 (the check is 4.5:1)
	role := func(name, hex string) rgba {
		c := palRGB(t, hex)
		v := palUntil(c, toward, func(v rgba) bool { return palMin(v, surf...) >= 4.6 })
		note(name, c, v)
		return v
	}
	gold, sky := role("gold", s.secondary), role("sky", s.info)
	okc, warn, bad := role("ok", s.ok), role("warn", s.warn), role("bad", s.bad)

	// the accent: text, border and focus on the surfaces, and the fill of the primary button in two looks, where its label must hold, also hovered
	accentRaw := palRGB(t, s.accent)
	labelHolds := func(v rgba) bool {
		l := palLabel(v)
		return contrast(l, v) >= 4.58 && contrast(l, mix(v, rules.fillHover, palWhite)) >= 4.58
	}
	spice := palUntil(accentRaw, toward, func(v rgba) bool { return palMin(v, surf...) >= 4.6 && labelHolds(v) })
	note("accent", accentRaw, spice)

	// text and quiet text: 5.1:1 on the surfaces (the rule of the reviewed mock) and 4.6:1 on every tint of a status colour or of the accent that a component draws
	accentTints := func(pcts []float64, boost float64) []rgba {
		var o []rgba
		for _, p := range pcts {
			for _, base := range []rgba{raised, panel} {
				o = append(o, mix(spice, p*boost, base))
			}
		}
		return o
	}
	statusTints := []rgba{mix(warn, 12, panel), mix(warn, rules.warnHover, panel), mix(bad, 12, panel), mix(bad, 14, panel), mix(sky, 10, panel), mix(spice, rules.flash, ground)}
	holds := func(v rgba, pcts []float64) bool {
		return palMin(v, surf...) >= 5.1 && palMin(v, statusTints...) >= 4.6 && palMin(v, accentTints(pcts, 1)...) >= 4.6
	}
	textRaw, dimRaw := palRGB(t, s.text), palRGB(t, s.dim)
	text := palUntil(textRaw, toward, func(v rgba) bool { return holds(v, []float64{8, 9, 10, 12, 16, 20}) })
	dim := palUntil(dimRaw, toward, func(v rgba) bool { return holds(v, []float64{8, 9, 10, 12}) })
	note("text", textRaw, text)
	note("dim", dimRaw, dim)

	// a fill whose label holds (and holds hovered): the accent itself when it does, else the nearest colour toward white or toward black that does
	fill := func(raw rgba, extra func(rgba) bool) rgba {
		ok := func(v rgba) bool { return labelHolds(v) && extra(v) }
		if ok(palShift(raw, raw, 0)) {
			return palShift(raw, raw, 0)
		}
		a, b := palUntil(raw, toward, ok), palUntil(raw, away, ok)
		if ok(b) && (!ok(a) || palDistance(raw, b) < palDistance(raw, a)) {
			return b
		}
		return a
	}
	blue := fill(accentRaw, func(rgba) bool { return true })
	note("fill", accentRaw, blue)
	// the switch that is on: 3:1 from the off track (the ground) and from the panel it sits on
	sw := fill(accentRaw, func(v rgba) bool { return palMin(v, ground, panel) >= 3.05 })
	note("switch", accentRaw, sw)

	// the boundary of a control and the divider ink: 3:1 on the four surfaces
	ruleRaw := palRGB(t, s.rule)
	rule := palUntil(ruleRaw, toward, func(v rgba) bool { return palMin(v, surf...) >= 3.05 })
	note("rule", ruleRaw, rule)

	// the strength of the accent tints (a selected segment, the current tab, a hovered row): the largest of 2 to 1 that keeps the text 4.6:1 on the strongest tint
	boost := 1.0
	for _, b := range []float64{2, 1.75, 1.5, 1.25, 1} {
		if palMin(text, accentTints([]float64{8, 9, 10, 12, 16, 20}, b)...) >= 4.6 && palMin(dim, accentTints([]float64{8, 9, 10, 12}, b)...) >= 4.6 {
			boost = b
			break
		}
	}

	hair := palRGB(t, s.hair)
	set := out.set
	scrim := ".5"
	if dark {
		scrim = ".78"
	}
	set("--c-ground", palHex(ground))
	set("--c-panel", palHex(panel))
	set("--c-raised", palHex(raised))
	set("--c-sunken", palHex(ground))
	set("--c-scrim", palRGBA(ground, scrim))
	set("--c-hair", palHex(hair))
	set("--c-rule", palHex(rule))
	set("--c-bronze", palHex(rule))
	set("--c-stitch", palHex(hair))
	set("--c-text", palHex(text))
	set("--c-dim", palHex(dim))
	set("--c-spice", palHex(spice))
	set("--c-on-spice", palHex(palLabel(spice)))
	set("--c-blue", palHex(blue))
	set("--c-on-blue", palHex(palLabel(blue)))
	set("--c-focus", palHex(spice))
	set("--c-gold", palHex(gold))
	set("--c-rust", palHex(warn))
	set("--c-sky", palHex(sky))
	set("--c-ok", palHex(okc))
	set("--c-warn", palHex(warn))
	set("--c-bad", palHex(bad))
	set("--c-glow", palRGBA(accentRaw, ".28"))
	set("--c-dust-1", palHex(ground))
	set("--c-dust-2", palHex(panel))
	set("--c-dust-3", palHex(raised))
	set("--c-switch", palHex(sw))
	set("--c-on-switch", palHex(palLabel(sw)))
	set("--k-boost", strconv.FormatFloat(boost, 'f', -1, 64))
	set("--aura", fmt.Sprintf("%d, %d, %d", int(accentRaw[0]), int(accentRaw[1]), int(accentRaw[2])))
	set("accent-color", "var(--c-spice)")
	set("scrollbar-color", "var(--c-rule) var(--c-ground)")
	return out
}

// ---- the stylesheet ----

const palettesHeader = `/* Tabr design palettes, version 0.3.0: eighteen sets of colours that recolour a look. Link this file after the files of a look and set html[data-palette="ID"] (the ids are design.Palettes).
   A palette changes colour tokens only: no type, space, shape, motion, texture or component. It applies in its own theme: a dark palette in the dark theme, a light one in Dayside (the light theme),
   and not at all in the other; removing the attribute gives the look's own colours back. The selectors are more specific than the looks' own, so the order of the files does not matter.
   The values are the colours of the fan-made editor themes under the MIT Licence that NOTICE names (alpha removed), moved toward white (dark palettes) or toward black (Dayside palettes) in steps of 4 %
   only where a check of this module needed it: every text colour 4.5:1 on the four surfaces and on every tint a component draws, the boundary of a control and the focus ring 3:1. The comment of each
   palette names the colours that were moved. This file is generated (palettes_gen_test.go holds the table and the rules, and a test compares the file with what they make).
   Names: --c-spice is the accent as text, border and focus light; --c-blue the accent as the fill of a primary button in the Fremen look; --c-switch the fill of a switch that is on; --k-boost the strength of
   the accent tints. accent-color and scrollbar-color are inherited, so a native checkbox, the range and every scrollbar follow. */
`

func palBlock(indent string, tk paletteTokens) string {
	var b strings.Builder
	for _, d := range tk.decls {
		fmt.Fprintf(&b, "%s%s: %s;\n", indent, d[0], d[1])
	}
	return b.String()
}

func palCSS(t *testing.T) string {
	t.Helper()
	rules := readPalRules(t)
	var b strings.Builder
	b.WriteString(palettesHeader)
	for _, s := range paletteSources {
		tk := palDerive(t, s, rules)
		moved := "none"
		if len(tk.moved) > 0 {
			moved = strings.Join(tk.moved, ", ")
		}
		fmt.Fprintf(&b, "\n/* %s (%s), %s. Moved to pass the checks: %s. */\n", s.name, s.group, s.theme, moved)
		sel := `:root[data-palette="` + s.id + `"]`
		media, other := "@media not (prefers-color-scheme: light)", `:not([data-theme="light"])`
		if s.theme == "light" {
			media, other = "@media (prefers-color-scheme: light)", `:not([data-theme="dark"])`
		}
		fmt.Fprintf(&b, "%s[data-theme=%q] {\n%s}\n", sel, s.theme, palBlock("  ", tk))
		fmt.Fprintf(&b, "%s {\n  %s%s {\n%s  }\n}\n", media, sel, other, palBlock("    ", tk))
	}
	return b.String()
}

func TestPalettesCSSIsGenerated(t *testing.T) {
	want := palCSS(t)
	if *updatePalettes {
		if err := os.WriteFile("assets/palettes.css", []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got := readFile(t, "assets/palettes.css")
	if got == want {
		return
	}
	gl, wl := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(gl) && i < len(wl); i++ {
		if gl[i] != wl[i] {
			t.Fatalf("assets/palettes.css differs from what palettes_gen_test.go makes, first at line %d:\n have %s\n want %s\nrun: go test -run TestPalettesCSSIsGenerated -update-palettes", i+1, gl[i], wl[i])
		}
	}
	t.Fatalf("assets/palettes.css has %d lines, the generator makes %d; run: go test -run TestPalettesCSSIsGenerated -update-palettes", len(gl), len(wl))
}
