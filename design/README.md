# design

The shared look of the Tabr programs, kept in the repository of the save editor [tabr-tau](https://github.com/Project-Arrakis/tabr-tau) and published as a separate Go module so that other programs can use it:

```
go get github.com/Project-Arrakis/tabr-tau/design@latest
```

**Unstable until v1.** The module is version 0.x. A name can change in a minor version; the [CHANGELOG](CHANGELOG.md) says when. Pin a version.

## What is in it

| Files (in `assets/`) | What |
|---|---|
| `tokens.css` | The tokens of version 0.1.0, unchanged: colour, type, spacing, shape, elevation and motion, dark and light. A program written against 0.1.0 keeps its look. |
| `tokens-fremen.css`, `tokens-awakening.css`, `tokens-sietch.css` | The tokens of the three looks, each dark (the default) and light (called Dayside). **Fremen is the default look.** Every name of 0.1.0 is defined here too. |
| `components-fremen.css`, `components-awakening.css`, `components-sietch.css` | The components, as each look draws them. |
| `components.css` | What the looks share: touch sizes, reachable tooltips, tile, rail and drawer, the more-contrast rule and the forced-colours block. |
| `motion.css` | The Motion setting and every keyframe. |
| `icons.svg` | The icon sprite. |
| `grain.svg` | The tile of the Awakening look's surface grain. |
| `fonts/DuneRise.ttf`, `fonts/DuneRise-OFL.txt`, `fonts/SHA256SUMS` | The heading typeface (unmodified), its licence and its checksum. |

`kitchen-sink/` (not embedded in the Go package) is a page that shows every token and component of every look; serve the `design` folder with any static file server on a loopback address and open `kitchen-sink/index.html`.

| Prefix | Group | Examples |
|---|---|---|
| `--c-` | colour | `--c-ground`, `--c-panel`, `--c-raised`, `--c-sunken`, `--c-rule`, `--c-hair`, `--c-text`, `--c-dim`, `--c-gold`, `--c-spice`, `--c-sky`, `--c-ok`, `--c-warn`, `--c-bad`, `--c-focus` |
| `--t-` | type | `--t-body`, `--t-display`, `--t-heading`, `--t-mono`, `--t-xs` (11 px) to `--t-2xl` |
| `--s-` | spacing | `--s-1` (4 px) to `--s-6` (32 px) |
| `--r-` | shape | `--r-1` to `--r-4`, `--r-pill` (0 in the Awakening look, which cuts its corners instead) |
| `--e-` | elevation | `--e-1`, `--e-2`, `--e-3` |
| `--mo-` | motion | `--mo-fast`, `--mo-base`, `--mo-slow`, `--mo-fill`, `--mo-breathe` and the other periods |
| `--m-` | the colours of the map's glyphs, the same in every look and theme | `--m-player`, `--m-worm`, `--m-live` |

The light theme (Dayside) follows the system setting. Put `data-theme="light"` or `data-theme="dark"` on the root element to force one.

## Using a look

The files are embedded in the Go program that imports the module, so there is no network at run time and a program always has exactly the version it was built with. Serve them from the program's own origin, next to its own files, and link the four files of a look before your own stylesheet:

```go
mux.Handle("/design/", http.StripPrefix("/design/", design.Handler()))
files, _ := design.LookFiles(design.DefaultLook) // tokens-fremen.css, components-fremen.css, components.css, motion.css
```

```html
<link rel="stylesheet" href="/design/tokens-fremen.css">
<link rel="stylesheet" href="/design/components-fremen.css">
<link rel="stylesheet" href="/design/components.css">
<link rel="stylesheet" href="/design/motion.css">
```

A program that lets its user switch looks swaps the first two hrefs for the names `design.LookFiles` gives, taken from a fixed table. A stored or typed string is never used to build a file name, a selector or markup; read it against the list `design.Looks()` and fall back to the default.

`design.Tokens()` returns `tokens.css` as bytes; `design.Assets` is an `fs.FS` rooted at the assets folder; `design.Version` is the version. A program keeps its own names as aliases while it moves over (`--bg: var(--c-ground)`), so that nothing it already styles has to change at once.

### What the program's content security policy needs

Everything is served from the program's own origin. `default-src 'none'` with `style-src 'self'`, `img-src 'self'`, `script-src 'self'` and `font-src 'self'` is enough. Nothing in the module needs `data:`, `blob:`, an inline style, an inline script or `unsafe-*`.

- The icon sprite is a file: `<svg class="d-i"><use href="/design/icons.svg#i-drop"/></svg>`. Whether a sprite file loads in the Windows WebView2 window under the policy above has not been checked in that window yet; a program that finds it blocked builds the same `<symbol>` elements in a script (`createElementNS`) and uses `<use href="#i-drop">`.
- The heading font needs `font-src 'self'`. Without it the headings fall back to the system face.
- No inline `style=` attribute and no `<style>` element in anything built from the module: layout goes in classes that start with `x-`, and a value that changes at run time (a meter's fill) is set with `element.style.setProperty('--v', '40%')`.
- Build markup with `createElement` and `textContent`, never with a string of HTML. The module's own scripts are checked for this.

### Settings a program stores

The module does not store anything. A program that remembers a look, a theme, the heading face or the Motion choice keeps a fixed list of values, validates what it reads against the list, uses the default for anything else, wraps every read and write in `try/catch`, and writes it on the root element before the first paint with an external script in the head (an inline script is blocked by the policy above).

| Attribute on `<html>` | Values | Meaning |
|---|---|---|
| `data-theme` | `dark`, `light` | forces a theme; absent follows the system |
| `data-heading-font` | `dune-rise`, `system` | the heading face; absent is `dune-rise` |
| `data-motion` | `full`, `calm`, `off` | the Motion setting |
| `data-motion-src` | `user` | set when the person chose a Motion value; without it the system setting `prefers-reduced-motion` makes Calm the default |
| `data-doc-hidden` | present or absent | set while the document is hidden; pauses every running animation |

### Parts

- **Tooltip.** `<span class="d-tt"><button aria-describedby="tip1">...</button><span class="d-tip" role="tooltip" id="tip1">Text</span></span>`. The tip can be reached with the pointer, shows on hover and on focus, and the program adds `dismissed` to `d-tt` on Esc and removes it when the pointer or focus leaves. A tip inside a cut panel is not cut off while it is open. For a tip that must escape a scroll container, render it in a layer at the top of the document.
- **Folding card.** The head is a button inside a heading: `<h2 class="d-heading"><button class="d-head" aria-expanded="true" aria-controls="id">...</button></h2>`.
- **Switch.** Put the switch in a row or a label so that the whole row is the target; the switch also reaches 36 x 28 px by itself (44 x 44 px for touch).
- **Rail and drawer.** `d-rail` with `d-rail-in`, `shut` when folded. A shut rail is hidden from the keyboard and from assistive technology. Under 760 px wide the rail lies over the page.
- **Toast.** One live region for all toasts (`aria-live="polite"` on `d-toasts`), no role on a toast, an error toast stays until dismissed, and its text is also kept where the person can read it again.
- **Dialog.** A `<dialog class="d-modal">` opened with `showModal()`; the default answer is the Cancel button.
- **Motion.** A program marks its own looping decorations with `data-loop`; Calm and Off stop them. No loop is faster than 333 ms.
- **Headings.** Dune Rise has capitals-height letters and lacks `%`, the middle dot, dashes, arrows and accented letters. It is used on fixed heading strings (the list is in the components file), never on numbers, units or names that come from a save, the game or a user.

### Limits

- The stylesheets use `:has()`, `:is()` and `color-mix()`: Chromium 111 or newer (the WebView2 runtime of Windows 11 and current Windows 10 has them). A browser older than that shows the parts without their tints and without the focus rules of the cut parts.
- Print is not supported: these are live views.
- The words are English; the capitals of headings are made by CSS, so another language can leave them off.
- The minimum window the looks were laid out for is 420 px wide; at a browser zoom that makes the window narrower than 320 CSS pixels the pages scroll sideways.

## Rules of this module

- **Standard library only.** No third-party dependency, and never an `internal` package of the editor. A test fails if either changes.
- **Values live here and nowhere else.** A program adds layout (grids, widths, order), not colours, fonts, shadows or radii.
- **Contents are listed in [PROVENANCE.md](PROVENANCE.md).** Written for this project, except the typeface Dune Rise (SIL Open Font License 1.1) and some palette values (MIT); both notices are in [NOTICE](NOTICE). No game artwork, no logo, and no image file but the module's own SVG drawings.
- **Public.** No private context in any file: a test reads every file for it.
- **Tags.** `design/vX.Y.Z`, made by the repository owner from `main` after the checks pass. The tag workflow tests the module again, requires the font file and its checksum, and creates a GitHub release from the matching CHANGELOG entry.
- **Licence.** MIT, as the repository ([LICENSE](LICENSE), [NOTICE](NOTICE)); the typeface keeps its own licence. Unofficial: not affiliated with or endorsed by Funcom.

## Working on it

```
cd design
go vet ./...
go test -race -count=1 ./...
```

To try a change in another program before it is tagged, put a `go.work` file beside the two checkouts (`use ./your-program ./tabr-tau/design`). Keep `go.work` and `go.work.sum` out of git (they are in `.gitignore`).

## Making a tag

Only the repository owner tags. From an up-to-date `main` with the checks green, with `design.Version` and `VERSION` set to the new version, an entry `## vX.Y.Z` in [CHANGELOG.md](CHANGELOG.md), and (while the font file is not yet in the module) `assets/fonts/DuneRise.ttf` added unmodified, whose sha256 equals `assets/fonts/SHA256SUMS`:

```
DESIGN_RELEASE=1 go test -count=1 ./...   # in design/: this run fails when the font file is missing
git tag design/vX.Y.Z && git push origin design/vX.Y.Z
```

The `design release` workflow then checks that the tag is on `main`, that it equals `VERSION`, that the changelog has an entry, that the font file is present and matches, and that the tests pass; creates the release; and asks the Go module proxy for the version so that it is available to `go get`.
