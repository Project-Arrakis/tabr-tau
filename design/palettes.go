package design

// The palettes of the module: sixty sets of colours, each a recolouring of whatever look is linked. A palette changes colour tokens and
// nothing else (no type, space, shape, motion, texture or component); it is chosen by html[data-palette="ID"] and applies in its own theme only.
// All sixty are in one stylesheet, assets/palettes.css, which a program links after the look's files.

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
	{ID: "dye-olive-sage", Name: "House Atreides dyes", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-iron-crimson", Name: "House Harkonnen dyes", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-alexin", Name: "House Alexin", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-argosaz", Name: "House Argosaz", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-dyvetz", Name: "House Dyvetz", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-ecaz", Name: "House Ecaz", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-hagal", Name: "House Hagal", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-hurata", Name: "House Hurata", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-imota", Name: "House Imota", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-kenola", Name: "House Kenola", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-lindaren", Name: "House Lindaren", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-maros", Name: "House Maros", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-mikarrol", Name: "House Mikarrol", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-moritani", Name: "House Moritani", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-mutelli", Name: "House Mutelli", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-novebruns", Name: "House Novebruns", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-richese", Name: "House Richese", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-sor", Name: "House Sor", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-spinnette", Name: "House Spinnette", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-taligari", Name: "House Taligari", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-thorvald", Name: "House Thorvald", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-tseida", Name: "House Tseida", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-varota", Name: "House Varota", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-vernius", Name: "House Vernius", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-wallach", Name: "House Wallach", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-wayku", Name: "House Wayku", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-wydras", Name: "House Wydras", Group: "House dyes", Theme: ThemeDark},
	{ID: "dye-sunset-dye", Name: "Sunset Dye", Group: "Outfit dyes", Theme: ThemeDark},
	{ID: "dye-dusk-rider", Name: "Dusk Rider", Group: "Outfit dyes", Theme: ThemeDark},
	{ID: "dye-blue-dasher", Name: "Blue Dasher", Group: "Outfit dyes", Theme: ThemeDark},
	{ID: "dye-maula", Name: "Maula", Group: "Outfit dyes", Theme: ThemeDark},
	{ID: "dye-choam", Name: "CHOAM", Group: "Outfit dyes", Theme: ThemeDark},
	{ID: "dye-vermillius-gap", Name: "Vermillius Gap", Group: "Outfit dyes", Theme: ThemeDark},
	{ID: "dye-smuggler", Name: "Smuggler", Group: "Outfit dyes", Theme: ThemeDark},
	{ID: "dye-water-shipper-utility", Name: "Water Shipper Utility", Group: "Outfit dyes", Theme: ThemeDark},
	{ID: "dye-scriveners", Name: "Scriveners'", Group: "Outfit dyes", Theme: ThemeDark},
	{ID: "dye-sand-rust", Name: "Fremen Sietch dyes", Group: "Outfit dyes", Theme: ThemeDark},
	{ID: "dye-ixian-garment", Name: "Ixian Garment", Group: "Outfit dyes", Theme: ThemeDark},
	{ID: "dye-resplendent-seraph", Name: "Resplendent Seraph", Group: "Outfit dyes", Theme: ThemeDark},
	{ID: "dye-palatial-servitors", Name: "Palatial Servitors", Group: "Outfit dyes", Theme: ThemeDark},
	{ID: "dye-ash-brass", Name: "Sardaukar Tactical", Group: "Outfit dyes", Theme: ThemeDark},
	{ID: "dye-hot-zone", Name: "Hot Zone", Group: "Outfit dyes", Theme: ThemeDark},
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
