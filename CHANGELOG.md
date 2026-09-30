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

### Changed
- `tools/snapshot.ps1` lists accounts and accepts `-SteamId` when several exist.

### Fixed
- `tools/send-snapshot.ps1` parse error (`"$dir:"` read as a drive-qualified variable).

### Security
- Save decoder is bounded (F-03 / SEC-1, #4): the declared size is checked against a 256 MiB cap before decompression and the stream is read through a limit, so a ~65 KB hostile file no longer allocates ~165 MB. Tests: bomb with a lying header, over-cap header, over-cap file, truncation, size mismatch both ways, wrong flag, non-SQLite payload, round trip at real size, fuzz seeds.
- Go toolchain raised to 1.26.6 (six reachable standard-library vulnerabilities found by govulncheck: GO-2026-5037, 5039, 5856, 5972, 6089, 6090).
- Findings from the Layer 1 audit are tracked in issue #24. **None of the application-code findings (F-03, F-04,
  F-05..F-08) are fixed yet**; they are scheduled as phases S0 and S1 in the plan. Until then, do not run tabr-tau
  with `--addr` set to anything other than a loopback address, and do not open saves from untrusted sources.

## [0.0.0] - 2026-09-30
### Added
- Initial single-player save and config editor (Go, local web UI): player, bases, vehicles, exchange, Landsraad, config
  and database tabs; backup before every write.
