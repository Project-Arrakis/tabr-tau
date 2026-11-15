package design

import (
	"sort"
	"strconv"
)

// The forty-two dye palettes (#683). Each one is made from the four armour-dye colours that the community wiki publishes for one swatch of the
// game (https://awakening.wiki/Module:Swatches/data, listed on https://awakening.wiki/Swatches): colour values only, which are facts about how
// the game looks. No text and no code of the wiki is used. The colours below are the wiki's, in the wiki's order and its slot order, and the
// rules after the table make the thirteen colours of a palette from them. The contrast rules of palettes_gen_test.go then run on the result,
// like on every other palette, so a dye set that is too pale or too dark is moved until every check holds.

type dyeSet struct {
	id, name, group string
	dyes            [4]string
}

// Where a name would repeat one of the first eighteen palettes ("House Atreides", "Fremen Sietch") the label says "dyes", and where the wiki's
// name is a word the module keeps out of ids (Atreides, Harkonnen, Fremen, Sietch, Sardaukar) the id names the colours instead.
var dyeSets = []dyeSet{
	{"dye-olive-sage", "House Atreides dyes", "House dyes", [4]string{"#313f29", "#959595", "#849141", "#bcab95"}},
	{"dye-iron-crimson", "House Harkonnen dyes", "House dyes", [4]string{"#2b2b2b", "#3f3f3f", "#7c383d", "#2b2b2b"}},
	{"dye-alexin", "House Alexin", "House dyes", [4]string{"#593730", "#bcab95", "#d2a552", "#959595"}},
	{"dye-argosaz", "House Argosaz", "House dyes", [4]string{"#2b271d", "#896b36", "#c48642", "#896b36"}},
	{"dye-dyvetz", "House Dyvetz", "House dyes", [4]string{"#505522", "#3f3f3f", "#bc766c", "#5c707c"}},
	{"dye-ecaz", "House Ecaz", "House dyes", [4]string{"#364b2b", "#bcab95", "#d2a552", "#bcab95"}},
	{"dye-hagal", "House Hagal", "House dyes", [4]string{"#345259", "#6c5a57", "#004d59", "#2b2b2b"}},
	{"dye-hurata", "House Hurata", "House dyes", [4]string{"#534c43", "#2f3f26", "#a2aa50", "#2b2b2b"}},
	{"dye-imota", "House Imota", "House dyes", [4]string{"#591a19", "#591a19", "#7cb8c9", "#3f3f3f"}},
	{"dye-kenola", "House Kenola", "House dyes", [4]string{"#5c707c", "#706f7c", "#cccccc", "#959595"}},
	{"dye-lindaren", "House Lindaren", "House dyes", [4]string{"#303859", "#3f3f3f", "#303859", "#2b2b2b"}},
	{"dye-maros", "House Maros", "House dyes", [4]string{"#896b36", "#2b2b2b", "#2b2b2b", "#896b36"}},
	{"dye-mikarrol", "House Mikarrol", "House dyes", [4]string{"#959595", "#3f3f3f", "#bc6c91", "#6c5a57"}},
	{"dye-moritani", "House Moritani", "House dyes", [4]string{"#2b2b2b", "#3f3f3f", "#d2a552", "#a39964"}},
	{"dye-mutelli", "House Mutelli", "House dyes", [4]string{"#592626", "#3f3f3f", "#cccccc", "#d5a38d"}},
	{"dye-novebruns", "House Novebruns", "House dyes", [4]string{"#3f2632", "#2b2b2b", "#955c30", "#cccccc"}},
	{"dye-richese", "House Richese", "House dyes", [4]string{"#3f3f3f", "#959595", "#7cb8c9", "#d88746"}},
	{"dye-sor", "House Sor", "House dyes", [4]string{"#6c5a57", "#6c5a57", "#953030", "#592626"}},
	{"dye-spinnette", "House Spinnette", "House dyes", [4]string{"#4d2b0c", "#2b1604", "#cccccc", "#cccccc"}},
	{"dye-taligari", "House Taligari", "House dyes", [4]string{"#1d302d", "#bcab95", "#54958b", "#a39964"}},
	{"dye-thorvald", "House Thorvald", "House dyes", [4]string{"#c48642", "#d2a552", "#2b2b2b", "#4b4079"}},
	{"dye-tseida", "House Tseida", "House dyes", [4]string{"#7c5b20", "#6c5a57", "#d2a552", "#6c5a57"}},
	{"dye-varota", "House Varota", "House dyes", [4]string{"#4d2b0c", "#303859", "#303859", "#6c5a57"}},
	{"dye-vernius", "House Vernius", "House dyes", [4]string{"#45203a", "#955c30", "#5c507c", "#2b2b2b"}},
	{"dye-wallach", "House Wallach", "House dyes", [4]string{"#959595", "#593730", "#953030", "#303859"}},
	{"dye-wayku", "House Wayku", "House dyes", [4]string{"#29304d", "#959595", "#953030", "#cccccc"}},
	{"dye-wydras", "House Wydras", "House dyes", [4]string{"#546c5b", "#bcab95", "#507c50", "#2b2b2b"}},
	{"dye-sunset-dye", "Sunset Dye", "Outfit dyes", [4]string{"#2b2b2b", "#5b4523", "#bcab95", "#a0581c"}},
	{"dye-dusk-rider", "Dusk Rider", "Outfit dyes", [4]string{"#27383f", "#2b2b2b", "#434448", "#bc766c"}},
	{"dye-blue-dasher", "Blue Dasher", "Outfit dyes", [4]string{"#5c707c", "#502b31", "#2b2b2b", "#d2a552"}},
	{"dye-maula", "Maula", "Outfit dyes", [4]string{"#4d5d6c", "#847867", "#5c707c", "#2b2b2b"}},
	{"dye-choam", "CHOAM", "Outfit dyes", [4]string{"#bcac8f", "#bcab95", "#d88746", "#bc8700"}},
	{"dye-vermillius-gap", "Vermillius Gap", "Outfit dyes", [4]string{"#593123", "#4c0400", "#d2a552", "#959595"}},
	{"dye-smuggler", "Smuggler", "Outfit dyes", [4]string{"#36486c", "#08192b", "#bc863f", "#957130"}},
	{"dye-water-shipper-utility", "Water Shipper Utility", "Outfit dyes", [4]string{"#a09b94", "#19303f", "#897468", "#7c2200"}},
	{"dye-scriveners", "Scriveners'", "Outfit dyes", [4]string{"#3f1d0f", "#59403f", "#dab57e", "#d58316"}},
	{"dye-sand-rust", "Fremen Sietch dyes", "Outfit dyes", [4]string{"#b49d76", "#644d25", "#812a14", "#b4984f"}},
	{"dye-ixian-garment", "Ixian Garment", "Outfit dyes", [4]string{"#98897a", "#595959", "#90d0da", "#a08c74"}},
	{"dye-resplendent-seraph", "Resplendent Seraph", "Outfit dyes", [4]string{"#bcbcbc", "#2b2b2b", "#a0581c", "#a0581c"}},
	{"dye-palatial-servitors", "Palatial Servitors", "Outfit dyes", [4]string{"#4d6c68", "#0f1915", "#0f1915", "#86aa9f"}},
	{"dye-ash-brass", "Sardaukar Tactical", "Outfit dyes", [4]string{"#6c6c6c", "#595959", "#b5b5b5", "#b39c54"}},
	{"dye-hot-zone", "Hot Zone", "Outfit dyes", [4]string{"#a19964", "#40412f", "#bd6068", "#c5c85f"}},
}

// ---- the rules ----

func dyeRGB(hex string) rgba {
	n, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil || len(hex) != 7 {
		panic("dye colour " + hex)
	}
	return rgba{float64(n >> 16), float64((n >> 8) & 255), float64(n & 255), 1}
}

// dyeChroma is how far a colour is from grey: the spread of its channels.
func dyeChroma(c rgba) float64 {
	hi, lo := c[0], c[0]
	for _, v := range c[:3] {
		if v > hi {
			hi = v
		}
		if v < lo {
			lo = v
		}
	}
	return hi - lo
}

// dyeSource makes the palette of one dye set:
//   - the page (ground) is the darkest dye, darkened by 62 % (more, in steps of 4 %, until its luminance is 0.018 or less); panel and raised are lifts of the ground toward the next darker dye (lightened by
//     20 %), by 14 % and 26 %; the hairline is the raised surface moved 12 % toward white;
//   - the accent is the most vivid of the dyes that are light enough to be seen on the page (luminance 0.10 and up; the lightest dye when none is);
//     the second accent is the most vivid of the others that differ from it (the accent again when there is none);
//   - the text is a near-white with 8 % of the accent in it, and the quiet text is that colour 42 % of the way to the page;
//   - the status colours are the same fixed set for every dye palette, so that a status never changes its meaning with the palette.
//
// The generator's contrast rules (palDerive) move any of these colours until the checks of the module hold; none of that is decided here.
func dyeSource(d dyeSet) paletteSource {
	dyes := make([]rgba, len(d.dyes))
	for i, h := range d.dyes {
		dyes[i] = dyeRGB(h)
	}
	order := make([]int, len(dyes))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return lum(dyes[order[a]]) < lum(dyes[order[b]]) })
	darkest := dyes[order[0]]
	next := darkest
	for _, i := range order[1:] {
		if palHex(dyes[i]) != palHex(darkest) {
			next = dyes[i]
			break
		}
	}
	ground := palShift(darkest, palBlack, 0.62)
	for t := 0.66; lum(ground) > 0.018 && t <= 0.9; t += 0.04 { // a swatch with no dark dye still gets a dark page
		ground = palShift(darkest, palBlack, t)
	}
	target := palShift(next, palWhite, 0.2)
	panel := palShift(ground, target, 0.14)
	raised := palShift(ground, target, 0.26)
	hair := palShift(raised, palWhite, 0.12)

	// the accents
	var light []rgba
	for _, i := range order {
		if lum(dyes[i]) >= 0.10 {
			light = append(light, dyes[i])
		}
	}
	if len(light) == 0 {
		light = []rgba{dyes[order[len(order)-1]]}
	}
	vivid := func(from []rgba, not string) rgba {
		best, found := from[0], false
		for _, c := range from {
			if palHex(c) == not {
				continue
			}
			if !found || dyeChroma(c) > dyeChroma(best) {
				best, found = c, true
			}
		}
		return best
	}
	accent := vivid(light, "")
	var rest []rgba
	for _, c := range dyes {
		if palHex(c) != palHex(accent) && lum(c) >= 0.06 {
			rest = append(rest, c)
		}
	}
	second := accent
	if len(rest) > 0 {
		second = vivid(rest, palHex(accent))
	}
	text := palShift(rgba{236, 236, 236, 1}, accent, 0.08)
	dim := palShift(text, ground, 0.42)
	rule := palShift(raised, palWhite, 0.42)
	return paletteSource{
		id: d.id, name: d.name, group: d.group, theme: "dark", band: true,
		ground: palHex(ground), panel: palHex(panel), raised: palHex(raised), hair: palHex(hair), rule: palHex(rule),
		text: palHex(text), dim: palHex(dim), accent: palHex(accent),
		ok: "#74c476", warn: "#f0b84a", bad: "#e8665a", info: "#6aaee8", secondary: palHex(second),
	}
}

func dyeSources() []paletteSource {
	out := make([]paletteSource, 0, len(dyeSets))
	for _, d := range dyeSets {
		out = append(out, dyeSource(d))
	}
	return out
}
