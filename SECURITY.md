# Security policy

tabr-tau is a local, single-user editor for a Dune: Awakening single-player save, for Windows. It has no server, no account and no
network service beyond a web page it serves to this computer only. Saves and game config files are treated as untrusted input, because
people share them.

## Reporting a vulnerability

Please report privately first.

1. Open the **Security** tab of this repository and choose **Report a vulnerability** (GitHub private vulnerability reporting).
2. If that choice is not offered, open an ordinary issue that says only "I have a security report" (no details, no file, no screenshot) and
   the maintainer will arrange a private way to receive it.

Please do not attach a real save, `Game.ini`, snapshot or log to any report or issue: they carry platform and account identifiers. A
description, or a redacted diff from `tabr-tau diff` (identifiers are hidden by default), is enough.

This is a spare-time project. Reports are read and answered on a best-effort basis; there is no service-level promise. A fix for a confirmed
problem is released as soon as it is ready and the report is credited unless you ask otherwise.

## What is in scope

- The editor listening beyond this computer, or answering a request from another computer or another web page.
- A web page, or a crafted save or config file, that makes the editor run code, write somewhere other than the save, its backups, the game
  config folder and its own settings, or write the save without the review step and the game-running check.
- A crafted save that crashes the editor or exhausts its memory.
- A way to read the session token, or to learn account identifiers the editor hides.

Out of scope: the game itself, Microsoft's WebView2 runtime and Edge, and anything that needs the attacker to already run code as you on
your computer (they can edit your save directly).

## How it is built to resist these (summary)

- The editor binds to loopback only, checks the Host header and the peer address, and refuses a request from another site (fetch metadata and
  Origin). The session is an HttpOnly, SameSite=Strict cookie, handed over once through a link that works one time for ten minutes.
- All HTML is built by an escaping template function; the page allows only its own scripts and styles.
- A save is only written from the review pane, after a game-running check, a changed-on-disk check, an integrity check and a backup, and is
  read back and compared afterwards.
- Saves are opened from a private copy, and the container is checked (size, declared size, integrity) before anything is trusted.

## Known limits that are documented, not hidden

- **The one-time link.** When the editor opens in the browser (`--web`) it hands the default browser a link that carries a single-use token
  in its query string. It is valid for ten minutes and for one use, and the editor redirects away from it at once, but it passes through
  the browser's history and the command line of the process that opens it. The native window (the Windows default) does not use the browser.
- **The session cookie is a bearer token.** Browsers send a cookie for `127.0.0.1` to every port of that address, so another program on
  this computer that you browse to on another port receives the header, and a local program that is given the cookie can replay it.
- **WebView2 and Edge traffic.** The native window is Microsoft's WebView2 runtime, which is part of Edge. The editor itself makes no
  outbound connection, but the runtime may contact Microsoft for updates and, depending on your Windows and Edge settings, send
  diagnostic data. That traffic is outside this program's control; manage it with the WebView2 and Edge policies of your Windows install,
  or use `--web` with a browser you have already configured. The profile is kept under `%LOCALAPPDATA%\tabr-tau\webview2`, never next to the exe.
- **The WebView2 loader is loaded from memory.** `github.com/jchv/go-webview2` embeds `WebView2Loader.dll` and loads it with
  `go-winloader`, a Go PE loader, instead of the Windows loader. Windows therefore does not check that DLL's Authenticode signature, and
  application-control policies (WDAC, AppLocker) do not see it as a separate file. A test (`internal/gui/loaderhash_test.go`) records the
  SHA-256 of each embedded DLL, so a dependency update that changes one fails the build until the change is reviewed. If your policy
  forbids unsigned code loaded this way, do not use the native window; run with `--web`.
- **No published binaries.** There is no release or signing process yet: build from source (see the README) and review what you run. If
  binaries are ever published they will come with a SHA-256 checksum and the workflow that built them.
