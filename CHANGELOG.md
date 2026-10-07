# Changelog

All notable changes to this project are documented here. Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versioning: [Semantic Versioning](https://semver.org/), tags `v0.x` until the first audited release.

## [Unreleased]

### Added
- Documentation: single-player runtime analysis, dune-docker parity table, architecture and implementation plan (v0.2),
  Layer 1 audit findings register, single-player file reference, live-test protocol, schema-only evidence, redacted
  client-log evidence (`docs/`).
- Tools: `tools/snapshot.ps1`, `tools/send-snapshot.ps1` (hash-verified scp), `tools/snapdiff.py` (redacted diff),
  `tools/snapsummary.py` (redacted fingerprint).
- Governance: CI (gofmt, go vet, staticcheck, race tests, govulncheck, PSScriptAnalyzer, shellcheck, commit-message
  policy, shared security scan), `.gitleaks.toml` with identifier rules, PR and issue templates, `CODEOWNERS`, Dependabot.

### Added (tests)
- `internal/testsave`: builds test saves from the REAL game schema (94 tables, STRICT, CHECKs, foreign keys, the game's trigger): empty, filled, hostile (every TEXT column holds markup/quote/CSV/SQL breakout text) and a consistent one-player world that mirrors the game's controller/pawn/player-state linkage. A drift test keeps the embedded DDL identical to `docs/evidence/single-player-schema.ddl.sql` (F-02 / #3).
- Contract test: every read path in `ops` runs against a populated real-schema save and any SQL error, even one the code swallows, fails the test (new `save.SetSQLErrorHook`, test-only). A self-test proves the hook sees errors.
- Give-item, full-backpack and Solari tests on the real schema, each ending with `integrity_check`, `foreign_key_check` and the sequencer invariant.

### Changed
- `tools/snapshot.ps1` lists accounts and accepts `-SteamId` when several exist.

### Fixed
- `tools/send-snapshot.ps1` parse error (`"$dir:"` read as a drive-qualified variable).

### Security
- SQL consoles (F-03 / SEC-3, SEC-4, SEC-9, SEC-10, ARCH-7, PR #36): the read console runs on a separate read-only connection with a 1000-row cap and a 5 s timeout; statements are tokenised and vetted (read: one SELECT; write: INSERT/UPDATE/DELETE/REPLACE only, no PRAGMA/ATTACH/DDL/transactions/`load_extension`); a save with unknown or modified triggers, views or virtual tables opens read-only with the reason; table listing never splices save-controlled names into SQL; CSV export neutralises formula cells.
- Durable Commit/Restore (F-07 / #8): backups get unique names, mode 0600, are fsynced and read back to verify; the new save is written to a unique exclusive temp file, fsynced, renamed with retry/backoff, and read back; `force` now skips only the game-running check, never the changed-on-disk check; Restore goes through the same pipeline (no in-place truncation), checks integrity, refuses a backup with patches the current save lacks, and refuses path components in the name; symlinked saves are refused; stale temp files are cleaned at open; a reload problem after a successful write is a warning, not a failed save.
- UI hardening (F-03 / #4, F-04 / #5): the page no longer embeds the API token. A one-time boot link (10 minutes, single use) sets an `HttpOnly; SameSite=Strict` session cookie and redirects; anyone else gets a static locked page. The UI script and CSS moved to separate files so the CSP can be `default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors 'none'; form-action 'none'` with no `unsafe-inline` and no `unsafe-eval`. All HTML is now built with an escaping `html` tagged template and written only through `setHTML`, which refuses anything else; the 12 inline `style=` attributes became classes; event dispatch tables reject prototype keys.
  Evidence: the old UI was not exploitable against the real schema (every text value went through `esc()`, and the unescaped sites sit on numeric columns of STRICT tables, which reject text) so the audit's original rating of SEC-2 overstated it; it was a latent weakness. With every number in every API response replaced by an attack string (a schema that does not enforce types), the OLD UI injected elements on 5 of 7 tabs and the NEW UI passes all 41 browser-engine tests, including every view under type confusion.
- Web server hardening (F-04, #5): the editor refuses any peer that is not on this computer (a LAN peer sending `Host: localhost` used to get HTTP 200 and the API token; measured by the new test against the old code); `--addr` must be loopback, and a non-loopback bind needs `--allow-remote` plus typing `I UNDERSTAND`; busy default port falls back to a random loopback port; security headers on every response (`X-Frame-Options: DENY`, `frame-ancestors 'none'`, `nosniff`, `Referrer-Policy: no-referrer`, CORP); `/api/` requires `Content-Type: application/json` on state-changing requests and rejects cross-site `Origin`/`Sec-Fetch-Site`; constant-time token comparison; the server fails closed if the random source fails; read/write/idle timeouts; export filenames are sanitized. Not yet done from F-04: the token is still embedded in the page (bootstrap cookie + CSP script restrictions arrive with the UI step).
- Save decoder is bounded (F-03 / SEC-1, #4): the declared size is checked against a 256 MiB cap before decompression and the stream is read through a limit, so a ~65 KB hostile file no longer allocates ~165 MB. Tests: bomb with a lying header, over-cap header, over-cap file, truncation, size mismatch both ways, wrong flag, non-SQLite payload, round trip at real size, fuzz seeds.
- Go toolchain raised to 1.26.6 (six reachable standard-library vulnerabilities found by govulncheck: GO-2026-5037, 5039, 5856, 5972, 6089, 6090).
- Findings from the Layer 1 audit are tracked in issue #24. **None of the application-code findings (F-03, F-04,
  F-05..F-08) are fixed yet**; they are scheduled as phases S0 and S1 in the plan. Until then, do not run tabr-tau
  with `--addr` set to anything other than a loopback address, and do not open saves from untrusted sources.

## [0.0.0] - 2026-09-30
### Added
- Initial single-player save and config editor (Go, local web UI): player, bases, vehicles, exchange, Landsraad, config
  and database tabs; backup before every write.
