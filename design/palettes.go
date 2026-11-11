package design

// The palettes of the module: eighteen sets of colours, each a recolouring of whatever look is linked. A palette changes colour tokens and
// nothing else (no type, space, shape, motion, texture or component); it is chosen by html[data-palette="ID"] and applies in its own theme only.
// All eighteen are in one stylesheet, assets/palettes.css, which a program links after the look's files.

// PalettesFile is the stylesheet of the palettes, as a path inside Assets.
const PalettesFile = "palettes.css"

// NoPalette is the id that means "no palette": the look's own colours. It is not in Palettes, and a program that stores a choice stores this
// string (or nothing) for it.
const NoPalette = "none"

// The two themes a palette belongs to. A palette applies in its own theme: a dark palette in the dark theme, a light palette in Dayside.
const (
	ThemeDark  = "dark"
	ThemeLight = "light"
)

// Palette describes one palette. The colours are in PalettesFile; nothing but this description is in Go.
type Palette struct {
	ID    string // the value of html[data-palette]; a descriptive name of the colours, lower case and hyphens
	Name  string // the label to show
	Group string // the heading to list it under
	Theme string // ThemeDark or ThemeLight: the only theme the palette applies in
}

var palettes = []Palette{
	{ID: "warm-charcoal", Name: "Arrakis Incandescent", Group: "Planet", Theme: ThemeDark},
	{ID: "umber-sand", Name: "Sandworm", Group: "Planet", Theme: ThemeDark},
	{ID: "deep-teal", Name: "Kaitain", Group: "Planet", Theme: ThemeDark},
	{ID: "storm-blue", Name: "Caladan Storm", Group: "Planet", Theme: ThemeDark},
	{ID: "ember-red", Name: "Salusa Secundus", Group: "Planet", Theme: ThemeDark},
	{ID: "circuit-indigo", Name: "Ix Technology", Group: "Planet", Theme: ThemeDark},
	{ID: "mono-grey", Name: "Giedi Prime", Group: "House & faction", Theme: ThemeDark},
	{ID: "cold-slate", Name: "Harkonnen", Group: "House & faction", Theme: ThemeDark},
	{ID: "navy-blue", Name: "House Atreides", Group: "House & faction", Theme: ThemeDark},
	{ID: "violet-iron", Name: "Sardaukar Imperial", Group: "House & faction", Theme: ThemeDark},
	{ID: "ink-blue", Name: "Bene Gesserit", Group: "House & faction", Theme: ThemeDark},
	{ID: "desert-dusk", Name: "Muad'Dib", Group: "Character", Theme: ThemeDark},
	{ID: "sand-linen", Name: "Fremen Sietch", Group: "Character", Theme: ThemeLight},
	{ID: "void-violet", Name: "Guild Navigator", Group: "Guild & commerce", Theme: ThemeDark},
	{ID: "steel-night", Name: "Spacing Guild", Group: "Guild & commerce", Theme: ThemeDark},
	{ID: "deep-ocean", Name: "Water of Life", Group: "Mystical & mental", Theme: ThemeDark},
	{ID: "paper-grey", Name: "Mentat", Group: "Mystical & mental", Theme: ThemeLight},
	{ID: "copper-cyan", Name: "Spice Vision", Group: "Mystical & mental", Theme: ThemeDark},
}

// Palettes lists the palettes in the order to show them (Arrakis Incandescent first, then by group). The slice is a copy that the caller may
// change.
func Palettes() []Palette { return append([]Palette(nil), palettes...) }

// PaletteFiles returns the stylesheets that a palette needs, as paths inside Assets, to be linked after the files of the look: PalettesFile for
// each id of Palettes, and none for NoPalette (the look's own colours need no file). ok is false for any other string. A program that lets its
// user choose a palette takes the file names from here and never from a stored value, so a stored string cannot name a file.
func PaletteFiles(id string) (files []string, ok bool) {
	if id == NoPalette {
		return nil, true
	}
	for _, p := range palettes {
		if p.ID == id {
			return []string{PalettesFile}, true
		}
	}
	return nil, false
}
