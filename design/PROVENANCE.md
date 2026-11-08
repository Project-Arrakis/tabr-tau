# Where the contents of this module come from

Every file in this module is listed here with where it comes from. The rule for what may enter the module:

- code and stylesheets written for this project;
- nothing else: no game artwork, no text or names copied from the game, no font file, no logo, no icon or picture from another project, and no code from another project. The "feel" of the Dune look is a reference for the choice of colours and nothing is copied from it.

| Path | What it is | Source | Added |
|---|---|---|---|
| `assets/tokens.css` | The design tokens: colour, type, spacing, shape, elevation and motion as CSS custom properties, for a dark and a light theme | Written for this project. The colour values are the palette the Tabr programs already show (a warm near-black and sand, a spice orange, a cool blue for information); each text colour was checked against WCAG AA (4.5:1) and the control boundary against 3:1, and a test repeats the check | #553 |
| `design.go`, `doc.go` | The Go API (`Assets`, `Tokens`, `Handler`, `Version`) and the package description | Written for this project | #553 |
| `design_test.go`, `tokens_test.go`, `boundary_test.go` | Tests of the API, of the stylesheet (names, themes, contrast, no outside reference) and of the module's boundary (standard library only, nothing internal) | Written for this project; `boundary_test.go` follows `kit/boundary_test.go` | #553 |
| `VERSION`, `CHANGELOG.md`, `go.mod` | The version, its history, the module path and the Go version. No requirement | Written for this project | #553 |
| `LICENSE`, `NOTICE` | The MIT licence, the same text as the repository's, and the notice | Copy of the repository's `LICENSE`; `NOTICE` follows `kit/NOTICE` | #553 |

When a file is added or a value changes, this table gets a row in the same pull request, with the issue and the source.
