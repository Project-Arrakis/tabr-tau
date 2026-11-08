# Changelog of the kit module

Tags of this module are `kit/vX.Y.Z`. The module is version 0.x: until v1.0.0 an exported name can change in a minor version, and the entry says so. A data change after a game patch is a patch version when only values change and a minor version when a row's meaning or a field changes. A tag is never moved; a bad tag is answered by a new tag and a `retract` line in `go.mod`.

Each tag needs an entry here whose heading is `## vX.Y.Z` followed by the date: the release workflow uses the entry as the release notes and fails without it.

## v0.1.0 (2026-11-08)

First version. Refs #547.

- New package `savefile`: `Decode`, `Encode` and `ReadCapped` for the container around a single-player save, and the constant `Limit` (256 MiB), moved from the editor's `internal/save` without a change of behaviour. `ReadCapped` is the editor's `readCapped`, now exported: it refuses a file over `Limit` from its size before any byte is read.
- The module has no third-party dependency; a test checks it.
