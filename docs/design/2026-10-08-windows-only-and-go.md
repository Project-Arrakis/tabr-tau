# Decision: Windows only, written in Go

Date: 2026-10-08. Status: accepted (operator). Issue: #87.

## Context
The game, Dune: Awakening, runs only on Windows, and its single-player save lives on the same PC. tabr-tau edits that save
file and the game's local configs. Early development happened on a Linux host, which is why parts of the code are portable
and CI started Linux-only.

## Decision
1. **Supported platform: Windows 10/11, 64-bit.** Nothing else is supported or tested for users.
2. **Language: Go.** The app is one self-contained exe (pure Go, no C compiler, no runtime to install). The UI is a web page shown
   in WebView2, a component Windows already ships, so a native UI toolkit would add little. Rewriting the audited save
   pipeline, the item catalog and the test suite in another language would cost far more than it returns.
3. **The Windows CI job is the gate that decides a merge** (#85). The Linux job stays as a fast extra check while most of the
   code builds portably.
4. **Non-Windows fallbacks are removed over time**, not kept for show. The browser fallback (`--web`) stays on Windows for the
   case where WebView2 is missing.

## Revisit when
- a native look and feel, an installer, or auto-update becomes a requirement (C#/.NET would then be the stronger tool);
- Go's Windows API access becomes the bottleneck for a feature.

## Consequences
- Tests that exercise Windows-only code run on a real Windows runner instead of being skipped or cross-compiled only.
- Wording and docs say "Windows only"; no promise of macOS, Linux or Proton support.
