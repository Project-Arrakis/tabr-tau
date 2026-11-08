# design

The shared look of the Tabr programs, kept in the repository of the save editor [tabr-tau](https://github.com/Project-Arrakis/tabr-tau) and published as a separate Go module so that other programs can use it:

```
go get github.com/Project-Arrakis/tabr-tau/design@latest
```

**Unstable until v1.** The module is version 0.x. A name can change in a minor version; the [CHANGELOG](CHANGELOG.md) says when. Pin a version.

## What is in it

Version 0.1.0 holds the design tokens only: `assets/tokens.css`, CSS custom properties for colour, type, spacing, shape, elevation and motion, with a dark theme (the default) and a light one. Components, icons and a word list may come later; each is a minor version.

| Prefix | Group | Examples |
|---|---|---|
| `--c-` | colour | `--c-ground`, `--c-panel`, `--c-raised`, `--c-rule`, `--c-text`, `--c-dim`, `--c-spice`, `--c-sky`, `--c-ok`, `--c-warn`, `--c-bad` |
| `--t-` | type | `--t-body`, `--t-xs` (11 px) to `--t-xl` (18 px) |
| `--s-` | spacing | `--s-1` (4 px) to `--s-6` (32 px) |
| `--r-` | shape | `--r-1` (2 px) to `--r-4` (8 px), `--r-pill` |
| `--e-` | elevation | `--e-1` (top highlight of a card), `--e-2` (shadow of a floating part) |
| `--mo-` | motion | `--mo-fast` (120 ms), `--mo-base`, `--mo-slow` |

The light theme follows the system setting. Put `data-theme="light"` or `data-theme="dark"` on the root element to force one.

## Using it

The files are embedded in the Go program that imports the module, so there is no network at run time and a program always has exactly the version it was built with. Serve them from the program's own origin, next to its own files (a content security policy of `style-src 'self'` keeps working), and link the stylesheet before your own:

```go
mux.Handle("/design/", http.StripPrefix("/design/", design.Handler()))
```

```html
<link rel="stylesheet" href="/design/tokens.css">
```

```css
.panel { background: var(--c-panel); color: var(--c-text); border: 1px solid var(--c-rule); border-radius: var(--r-4); padding: var(--s-3) var(--s-4); }
```

`design.Tokens()` returns the stylesheet as bytes; `design.Assets` is an `fs.FS` rooted at the assets folder; `design.Version` is the version. A program keeps its own names as aliases while it moves over (`--bg: var(--c-ground)`), so that nothing it already styles has to change at once.

## Rules of this module

- **Standard library only.** No third-party dependency, and never an `internal` package of the editor. A test fails if either changes.
- **Values live here and nowhere else.** A program adds layout (grids, widths, order), not colours, fonts, shadows or radii.
- **Contents are listed in [PROVENANCE.md](PROVENANCE.md).** Written for this project; no game artwork, no font, no logo.
- **Tags.** `design/vX.Y.Z`, made by the repository owner from `main` after the checks pass. The tag workflow tests the module again and creates a GitHub release from the matching CHANGELOG entry.
- **Licence.** MIT, as the repository ([LICENSE](LICENSE), [NOTICE](NOTICE)). Unofficial: not affiliated with or endorsed by Funcom.

## Working on it

```
cd design
go vet ./...
go test -race -count=1 ./...
```

To try a change in another program before it is tagged, put a `go.work` file beside the two checkouts (`use ./your-program ./tabr-tau/design`). Keep `go.work` and `go.work.sum` out of git (they are in `.gitignore`).

## Making a tag

Only the repository owner tags. From an up-to-date `main` with the checks green, with `design.Version` and `VERSION` set to the new version and an entry `## vX.Y.Z` in [CHANGELOG.md](CHANGELOG.md):

```
git tag design/vX.Y.Z && git push origin design/vX.Y.Z
```

The `design release` workflow then checks that the tag is on `main`, that it equals `VERSION`, that the changelog has an entry, and that the tests pass; creates the release; and asks the Go module proxy for the version so that it is available to `go get`.
