# Where the contents of this module come from

Every file in this module is listed here with where it comes from. The rule for what may enter the module:

- code written for this project, and tables of the game's identifiers and numbers that come from a public source or from the installed game's own content files read without modifying them;
- nothing else: no game artwork, no text or names copied from the game's art or audio, no save, log or snapshot of a person's machine, and no code from another project.

Test data in this module is made by the tests (random bytes after the SQLite header, and containers built in the test), never read from a real save.

| Path | What it is | Source | Added |
|---|---|---|---|
| `doc.go`, `boundary_test.go` | The module's description, and the tests that it imports the standard library only and nothing internal | Written for this project | #547 |
| `savefile/codec.go` | `Decode`, `Encode` and `ReadCapped`: the container around a single-player save (a little-endian flag of 1, a little-endian uncompressed size and a zlib stream whose payload is a SQLite 3 database) | Written for the editor and moved here from `internal/save/codec.go` unchanged except for the package name and the exported names. The container layout is the one the game writes and the editor has read since its first version (`docs/single-player-files.md`); no game file or game code was copied | #547 (moved), earlier as `internal/save/codec.go` |
| `savefile/codec_test.go`, `savefile/readcap_test.go` | Tests of the container: round trip at a real size, a stream that expands far beyond its header, a header over the limit, truncation, a wrong size in either direction, a wrong flag, a payload that is not SQLite, a fuzz target; and the capped read | Written for the editor and moved here; the payloads are made by the tests | #547 (moved) |
| `go.mod` | The module path and the Go version. No requirement | Written for this project | #547 |
| `LICENSE` | The MIT licence, the same text as the repository's | Copy of the repository's `LICENSE` | #547 |

When a file is added or a table is refreshed, this table gets a row in the same pull request, with the issue, the source and (for data) the game build the values were read from.
