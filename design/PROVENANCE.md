# Where the contents of this module come from

Every file in this module is listed here with where it comes from. The rule for what may enter the module:

- code, stylesheets and drawings written for this project;
- the two third-party works named in [NOTICE](NOTICE), each with its licence text next to it or in the notice: the typeface Dune Rise (SIL Open Font License 1.1) and the MIT-licensed palette values;
- nothing else: no game artwork, no text or names copied from the game, no logo, no icon or picture from another project, no image file of any kind except the module's own SVG drawings, and no code from another project. The "feel" of the Dune look is a reference for the choice of colours and nothing is copied from it.

| Path | What it is | Source | Added |
|---|---|---|---|
| `assets/tokens.css` | The design tokens of version 0.1.0: colour, type, spacing, shape, elevation and motion as CSS custom properties, for a dark and a light theme. Unchanged in version 0.2.0 | Written for this project. The colour values are the palette the Tabr programs already show (a warm near-black and sand, a spice orange, a cool blue for information); each text colour was checked against WCAG AA (4.5:1) and the control boundary against 3:1, and a test repeats the check | #553 |
| `assets/tokens-fremen.css`, `assets/tokens-awakening.css`, `assets/tokens-sietch.css` | The tokens of the three looks, each in a dark theme (the default) and a light one called Dayside; the names of version 0.1.0 are kept so that a file can replace `tokens.css` | Written for this project. The grounds and the Dayside colours of the Fremen look follow the MIT-licensed themes named in NOTICE (copyright line and licence text there); every other value is this module's own. Each text colour, rule and focus colour is checked against WCAG AA for every look and theme by a test | #572 #573 #574 |
| `assets/components-fremen.css`, `assets/components-awakening.css`, `assets/components-sietch.css` | The components (`d-` classes) as each look draws them | Written for this project | #574 |
| `assets/components.css` | What the looks share: touch sizes, tooltips that can be reached, the tile, the rail and its drawer, the contrast preference and the forced-colours block | Written for this project | #574 |
| `assets/motion.css` | The Motion setting (`html[data-motion]` full, calm, off), the reduced-motion default and the keyframes | Written for this project | #574 |
| `assets/icons.svg` | The icon sprite: 24 px grid, one colour, a symbol per icon, and the outlines of the drop, the can and the shield of the toasts | Drawn for this project; no outside artwork | #573 |
| `assets/grain.svg` | The tile of the Awakening look's surface grain (a faint noise, 180 px, seamless) | Drawn for this project | #572 |
| `assets/fonts/DuneRise.ttf` | The Dune Rise typeface, used for headings, unmodified | Third party: Fontswan, SIL Open Font License 1.1. Downloaded from the dafont.com page of the font (the author's own page is https://fontswan.com); file 22,964 bytes, sha256 `cb9c956d14b4e743658904484d2e7e7bd6621d16faaafe7d0e47c022f70a99f9`; the zip it came in has sha256 `ba33b46ff46000eef81676b88c8c0994eedd3a5de2545ebca20d7fe4ca02b615`. Download date: 2026-11-09. The file is added to the module by the commit that prepares the release; until then `SHA256SUMS` and the test that reads it are in place and the release workflow refuses a tag without the file | #573 |
| `assets/fonts/DuneRise-OFL.txt`, `assets/fonts/SHA256SUMS` | The licence text of the font as it came with it, and the checksum a test compares the font with | The licence text is the font author's, unchanged (line endings only); the checksum file is written for this project | #573 |
| `kitchen-sink/index.html`, `kitchen-sink/kitchen-sink.css`, `kitchen-sink/kitchen-sink.js` | One of every token and component, for looking and for the tests; not embedded in the Go package | Written for this project | #574 |
| `design.go`, `doc.go` | The Go API (`Assets`, `Tokens`, `Handler`, `Looks`, `LookFiles`, `Version`) and the package description | Written for this project | #553, #572 |
| `*_test.go` | Tests of the API, of the stylesheets (names, themes, contrast of every look, focus, motion, sizes, forced colours), of the kitchen sink, of the files (types, font checksum, no outside reference, public safety) and of the module's boundary (standard library only, nothing internal) | Written for this project; `boundary_test.go` follows `kit/boundary_test.go` | #553, #572, #573, #574 |
| `VERSION`, `CHANGELOG.md`, `go.mod` | The version, its history, the module path and the Go version. No requirement | Written for this project | #553 |
| `LICENSE`, `NOTICE` | The MIT licence, the same text as the repository's, and the notice | Copy of the repository's `LICENSE`; `NOTICE` follows `kit/NOTICE` | #553, #573 |

When a file is added or a value changes, this table gets a row in the same pull request, with the issue and the source.
