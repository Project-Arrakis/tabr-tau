# Changelog of the design module

Tags of this module are `design/vX.Y.Z`. The module is version 0.x: until v1.0.0 a name can change in a minor version, and the entry says so. Patch: a value changes and no name does. Minor: a token, a component or a file is added (or a name changes before v1). Major: a rename or a removal after v1. A tag is never moved; a bad tag is answered by a new tag and a `retract` line in `go.mod`.

Each tag needs an entry here whose heading is `## vX.Y.Z` followed by the date: the release workflow uses the entry as the release notes and fails without it.

## v0.1.0 (2026-11-08)

First version: the design tokens only. Refs #553.

- New file `assets/tokens.css`: CSS custom properties for colour (`--c-ground`, `--c-panel`, `--c-raised`, `--c-rule`, `--c-text`, `--c-dim`, `--c-spice`, `--c-sky`, `--c-ok`, `--c-warn`, `--c-bad`), type (`--t-body`, the sizes `--t-xs` to `--t-xl`), spacing (`--s-1` to `--s-6`, a 4 px grid), shape (`--r-1` to `--r-4`, `--r-pill`), elevation (`--e-1`, `--e-2`) and motion (`--mo-fast`, `--mo-base`, `--mo-slow`). The dark theme is the default; the light theme follows the system setting, or is forced with `data-theme="light"` (and the dark one with `data-theme="dark"`).
- The values are the ones the first program that adopts the module already shows, so adopting it changes nothing on screen. Nothing else is in this version: no components, icons or word list.
- Go API: `design.Assets` (the embedded files, rooted at the assets folder), `design.Tokens()` (the stylesheet), `design.Handler()` (a read-only handler for the files; a stylesheet is always `text/css`), `design.TokensFile` and `design.Version`.
- Tests: the module imports the standard library only; the stylesheet's text colours are 4.5:1 and its control boundary 3:1 on the three surfaces in both themes; the two light blocks are the same; every name has a prefix; the version, the `VERSION` file and this file agree.
