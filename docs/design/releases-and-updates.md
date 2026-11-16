# Build numbers, releases and the update check (design note)

Status: design note, 2026-11-16. Nothing in this note is built. Issue: #806. Related: #850 (the Settings gear, which takes the
settings listed in section 9), #577 (packaging the editor for other people: this note settles its questions 2, 3 and 5; the rest of
#577 stays open), #558 (release workflow fixes).

The operator's go of 2026-11-16 asked for build numbers, releases and an update function that finds a new release and installs it.
The decisions proposed on the issue are kept. This note works out how, finds what they touch, and ends with the issues to file and the
few questions that are the operator's to answer.

## 0. Summary and what was found

Decisions kept as proposed: SemVer plus a build counter and the commit, with the git tag as the one source of truth; a version label in
the header that opens an About and Releases panel; a poll (not a push) on start and about every 6 hours; notify only, install only after
a click; SHA-256 and a signature checked against a public key inside the app; the old exe kept for one-click rollback; a skip switch and
an off switch; nothing about the user or the save is ever sent; a release workflow that runs only after the required checks.

No decision is reversed. Four findings about the present repository matter to the build (F1 to F4), and three refinements are added (R1 to R3):

| # | Finding or refinement | Consequence |
|---|---|---|
| F1 | README ("no telemetry, no update check"), SECURITY.md and a test (`internal/save/netguard_test.go`, register 2026-09-30 F-22) all state that the editor makes no outbound connection, and the test fails the build if shipped code uses an HTTP client. | The update check is the first and only exception. The three places change together, in the slice that adds the check, and the test is narrowed to one package and one file (section 4.8), not switched off. |
| F2 | `var version = "dev"` exists in `cmd/tabr-tau/main.go` and nothing ever sets it (the build flags carry no `-X`). Base files the editor writes say the editor was `dev`. | Slice 1 fixes this first. It needs no network. |
| F3 | The build flags live in the maintainer's local build script, which is not in this repository, and CI cross-compiles with its own copy of the flags. | The stamping logic goes into one script in this repository that the local script, CI and the release workflow all call. Otherwise three places can drift (section 1.4). |
| F4 | The repository already publishes `design/vX.Y.Z` and `kit/vX.Y.Z` releases, created with `--latest=false`. | The editor's own tags are `vX.Y.Z` (no prefix), which cannot collide with those. The poll reads a fixed asset of the latest release, so it must stay true that only editor releases are ever "latest" (section 3.4). |
| R1 | A signed manifest alone is not enough: an attacker who replays an old signed pointer can hold users on an old, vulnerable version. | The pointer (`latest.json`) is signed too, carries a serial number, and the app refuses anything not newer than what it already runs or has seen (section 8). A replay can delay an update, never move anyone to a worse one. |
| R2 | If the signing key is a CI secret, anyone who can change the workflow or the runner can sign. | Recommendation: the signing key is never in GitHub. CI builds a draft release; the operator signs it on his own machine and publishes (section 6). Then a stolen repository account or runner cannot produce a release the app accepts. |
| R3 | #577 asks for the update check to be opt-in; the issue here says "can be turned off". | Recommendation: the app asks once, before any connection is made, and stores the answer (section 4.7). This is question 2 in section 12. |

## 1. Numbering

### 1.1 The form

- Version: SemVer `MAJOR.MINOR.PATCH`, `0.x` until the first audited stable release, as the CHANGELOG already says. A release tag is
  `vMAJOR.MINOR.PATCH` (an optional pre-release part such as `-rc.1` is allowed; a pre-release is never offered by the update check).
- Build counter: the number of commits that lead to the released commit (`git rev-list --count <commit>`). It rises with every commit on
  `main`, needs no file and no stored counter, and anyone can recompute it from the tag. It is also the serial number the update check
  compares (section 8).
- Commit: the first 7 characters of the commit id.
- Shown as `v0.14.2 (build 317, a1b2c3d)`. A build that is not exactly a tag shows the nearest tag plus build metadata,
  `v0.14.2+5 (build 322, e4f5a6b)`, where `5` is the number of commits since the tag. SemVer ignores build metadata when it compares, so a
  development build compares equal to its tag and is older than the next release. A build from a dirty tree adds `.dirty` and is refused
  by the release workflow.
- A development build (anything with `+N`, `.dirty` or the word `dev`) shows the label and the About panel and may show the update dot,
  but its Update button is replaced by the sentence "this is a development build" (section 5.5).

### 1.2 One source of truth

The git tag. Nothing else holds the version: no `VERSION` file for the editor, no number typed into the code, no CHANGELOG heading that is
trusted. The release workflow checks that the CHANGELOG has a `## [X.Y.Z]` section for the tag (the same check `design-release-notes.sh`
and `kit-release-notes.sh` already do for the module tags) but the CHANGELOG follows the tag, not the other way round.

### 1.3 What is stamped

A new package `internal/buildinfo` with four string variables, set by the linker and read-only afterwards:

| Variable | Example | From |
|---|---|---|
| `Version` | `0.14.2` or `0.14.2+5` | `git describe --tags --long --match 'v[0-9]*'` |
| `Build` | `317` | `git rev-list --count HEAD` |
| `Commit` | `a1b2c3d` | `git rev-parse --short=7 HEAD` |
| `Date` | `2026-11-16` | the commit date (`git show -s --format=%cs HEAD`), not the wall clock, so that a rebuild of the same commit gives the same bytes |

When the variables are empty (a plain `go build`, `go run`, a test) the label shows `dev` and the commit that Go itself records in the
binary (`debug.ReadBuildInfo`, key `vcs.revision`) when it is there. The old `var version` in `cmd/tabr-tau/main.go` is removed and its two
readers (the `usage` text and `ws.EditorVersion`) read `buildinfo.Version`. `tabr-tau --version` prints the one line of section 1.1.

### 1.4 Where the stamp goes

Today the flags exist in two places: the local build script (`GOOS=windows CGO_ENABLED=0`, `go build -trimpath -ldflags "-H=windowsgui -s -w"`
of `./cmd/tabr-tau`, built from a clean worktree at `origin/main`) and the cross-compile step of the `go` job in `ci.yml` (the same flags,
output discarded). Neither has an `-X`.

Plan: one script in this repository, `scripts/buildinfo.ps1` (PowerShell, because the maintainer's builds run on Windows; the release
workflow runs it with PowerShell on the runner and the existing `powershell` check lints it), that prints the ldflags string:

```
-H=windowsgui -s -w -X <module>/internal/buildinfo.Version=... -X <module>/internal/buildinfo.Build=... -X <module>/internal/buildinfo.Commit=... -X <module>/internal/buildinfo.Date=...
```

and the three callers use it: the local build script replaces its literal `-H=windowsgui -s -w` with the script's output; the release
workflow builds with it; the CI cross-compile step builds with it too (so a broken stamp fails a pull request, not a release). The flags
`-trimpath`, `CGO_ENABLED=0` and `GOOS=windows` stay where they are. The release build also passes `-buildvcs=false` so the bytes do not
depend on the checkout. A test builds the program with a fake stamp and runs `--version` to prove the variable names still match the
package (a renamed variable would otherwise stamp nothing, silently).

Aim, to be proven in the release slice and not promised before: building the same tag with the same Go version gives the same SHA-256, so
that anyone can check a release against the source.

## 2. The header label and the About and Releases panel

### 2.1 The label

A small text button after the title in the `d-bar` header, on every tab: `v0.14.2`. It uses the existing header and pill styles, takes a
dot (a small filled circle with the visible text "update available" for assistive technology, not colour alone) when the check has
found a newer release that is not skipped, and opens the panel. A development build shows `v0.14.2+5`.

### 2.2 The panel

A dialog from the design module's own dialog, not a new tab. It holds:

1. The running version line of section 1.1, the build date, the channel (`stable`), and the Windows architecture.
2. The update state in words: "Checked 14:03, you have the newest release", "Version 0.15.0 is available", "Update check is off",
   "No connection (last check 12:03)". A **Check now** button.
3. When a newer release exists: its version, date, size, its short notes (section 3.2), and the buttons **Update now**, **Skip this
   version** and **Later** (close).
4. **Roll back to v0.14.1** when the previous exe is on disk (section 5.4).
5. The release list: the last ten releases from the signed `latest.json` (version, date, the highlights each one carries). Offline it shows
   the last copy that passed the signature check, labelled with its date.
6. **Copy link** buttons for a release page and for the CHANGELOG. The editor does not navigate away from its own page (the navigation
   guard blocks it), so links are copied, not followed.
7. A line that says what the check sends: nothing but a request for a public file (section 4.6), and the switch to turn it off.

Every string that came from the network (notes, highlights) is shown as text with `textContent`, never as HTML, and is limited in length
before it is shown.

### 2.3 Sources

The panel reads only what the Go side has already verified (section 4). The page never fetches anything: the content security policy has
`connect-src 'self'` and stays so.

## 3. Release layout

### 3.1 The pattern

A stable channel; one folder per version holding the release notes, the exe, SHA-256 checksums and a signature; and one small manifest
that the poll reads. This is a pattern seen in other desktop applications and it fits GitHub Releases without change.

Logical layout (what the app and the tools think in):

```
stable/
  latest.json              the pointer the poll reads, signed
  latest.json.minisig
  v0.14.2/
    notes.md               the release notes (the CHANGELOG section, rendered by a script)
    tabr-tau-0.14.2-windows-amd64.exe
    SHA256SUMS             sha256sum format: one line per file in this folder except itself and the signatures
    SHA256SUMS.minisig     the signature of SHA256SUMS
    manifest.json          machine-readable: version, build, commit, date, file names, sizes
```

Physical layout on GitHub: the tag `v0.14.2` is the folder and the assets of its release are the files, with the same names. `latest.json`
and its signature are assets of every editor release; the poll reads the one of the release GitHub calls "latest". There is one channel,
`stable`. Pre-releases (tag with a `-` part) exist for the maintainer's tests and are never "latest", so the poll never sees them. A second
channel is not built; the field `channel` exists in the files so that adding one later does not change the format.

### 3.2 What each file holds

`latest.json` (small, a few kilobytes, signed):

```
{ "schema": 1, "product": "tabr-tau", "channel": "stable",
  "version": "0.14.2", "build": 317, "commit": "a1b2c3d", "date": "2026-11-16",
  "issued": "2026-11-16T20:00:00Z",
  "manifest": "manifest.json", "withdrawn": ["0.14.0"],
  "releases": [ { "version": "0.14.2", "date": "2026-11-16", "build": 317, "highlights": ["...", "..."] }, ... up to 10 ] }
```

`manifest.json` (the same facts about this one release plus, for each file, its name, size and SHA-256). It is listed in `SHA256SUMS`, so
its bytes are covered by that signature. File names in both documents match `^[A-Za-z0-9._-]{1,100}$` and are never paths or URLs: the
download URL is built by the app from the fixed repository address, the tag and the name.

`SHA256SUMS` is the plain format that `sha256sum --check` reads, so a person can check a download without our tools. `notes.md` is covered
by it as well.

Signatures use the format of section 6.

### 3.3 The serial number

`build` is the serial number the app compares. The version and the build must both be greater than the running ones for an offer to be
made (section 8, downgrade).

### 3.4 Keeping "latest" the editor's

The poll's address is `https://github.com/Project-Arrakis/tabr-tau/releases/latest/download/latest.json`, a fixed address in the code.
GitHub answers it with a redirect to the asset of the release marked "latest". Today only the editor can be that release, because the
`design` and `kit` workflows pass `--latest=false`. This is made a rule, not a habit: the editor's release workflow passes `--latest`
explicitly, a test reads the two module workflows and fails if either one lacks `--latest=false`, and the app checks `product` and
`channel` in the signed file, so a stray release is ignored even if it were marked "latest".

The API address (`/repos/.../releases/latest`) is not used: it is a bigger document that we do not control the shape of, and it counts
against the unauthenticated API rate limit. The asset address is a plain file read and the file is ours and signed.

## 4. The update flow

### 4.1 Poll, not push

Decided on the issue and kept. A push needs a server that holds a connection or an address for each user, sends data about the user and
adds infrastructure. A poll is a plain read of a public file. There is no operating-system notification and no background service: the
check runs only while the editor is open.

### 4.2 When it runs

- Once shortly after start (after the page is up, a random 10 to 60 seconds later, so a start is never slowed and a crowd of starts at one
  minute does not arrive together).
- Then about every 6 hours while the editor stays open: 6 hours plus a random 0 to 30 minutes, drawn again each time.
- On **Check now**, immediately (at most once a minute, so a held button is not a loop).
- Never when the setting is off or has not been answered (section 4.7).

### 4.3 The request

One `GET` of the fixed address of section 3.4 with:

- `If-None-Match` set to the `ETag` of the last answer, kept in the update state file (section 4.9). An unchanged file answers 304 with no
  body, so a check costs one small request. The redirect keeps the header; the first slice of the check verifies by test against the real
  address that a 304 does come back, and if it does not the check still works because the file is a few kilobytes.
- A fixed `User-Agent: tabr-tau-update-check` with no version, no operating-system string, no machine or user value, no cookie, no
  `Referer`. The check cannot tell the other side who the user is. What GitHub sees anyway is the IP address and the time of the request,
  and the README says so.
- A 10 second timeout, a body limit of 64 KB for `latest.json` and 4 KB for the signature, HTTPS only, redirects only to `https` and only
  to the hosts in a short list in the code (`github.com` and the host GitHub serves release assets from), at most 3 redirects.

### 4.4 Rate limits and failures

- 429 or 403 with `Retry-After` or `X-RateLimit-Reset`: wait at least that long; otherwise back off 30 minutes, 1 hour, 2, 4, 8, capped at
  24 hours, reset on the first good answer. The next scheduled check is never earlier than the wait.
- No network, DNS failure, timeout, 5xx: fail quietly. No toast, no dialog, no log line above debug level. The panel says "No connection
  (last check <time>)" when the user opens it. The next try follows the same backoff.
- A body that is not valid JSON, is too large, fails the signature, names another product or channel, or has a lower serial number than
  the last accepted one: ignored as if there were no answer, with one line in the panel ("The release list could not be verified") and
  one debug log line. It is never shown as an update.

### 4.5 When something newer exists

The Go side verifies `latest.json` (section 6), checks that `version` and `build` are both greater than the running ones and that the
version is not the skipped one, and stores the result. The page, which polls the editor's own `/api/update/state` like it polls the other
state, then shows a toast ("Version 0.15.0 is available") once per version, and the dot on the label. A toast is shown at most once for
a version, and not again after Later; the dot stays until the version is installed or skipped.

### 4.6 What is sent, and what is never sent

Sent: the one request above. Never sent: the version of the editor, anything about the save, the game, the machine, the account, the
settings, which tab is open, whether the user updated. There is no identifier of any kind, no "install id", no counter. The download for
an update is another plain `GET` of a public file with the same headers. The app has no way to report back, because no code posts
anything.

### 4.7 The off switch and the first question

The setting `updateCheck` has three states: unanswered, on, off. In the unanswered state no connection is made. The first start of a
version that has the check shows one question in the About panel and as a quiet banner: "Check GitHub for new releases? The check
reads one public file and sends nothing about you or your save. You can change this in Settings." with **Yes** and **No**; the
answer is stored. A user who never answers is never checked. (Whether it should instead start "on" is question 2 of section 12.)

**Skip this version** stores that version; a later version is offered again. A rollback (section 5.4) skips the version rolled back from.

### 4.8 Where the code lives and how the "no outbound connection" test changes

All network code is in one new package, `internal/update`, in one file that builds the HTTP client (`client.go`). The rest of the
package (parsing, signature, hash, version compare, state) is pure and has no network import. The test `TestShippedCodeMakesNoOutbound
Connections` keeps forbidding the HTTP client, dialers and resolvers everywhere, with one new allowed entry: `internal/update/client.go`.
A second test pins that file: the only hosts in it are constants from one list, every request is a `GET`, the client has a timeout, and
the package has no function that takes a URL from a caller. README line "makes no outbound network connections (no telemetry, no update
check)" and the matching SECURITY.md paragraph change to: "the only outbound connection is the update check: one read of a public file
from this project's releases, off with one switch, sending nothing about you; no telemetry". SECURITY.md's "No published binaries"
paragraph is replaced by the release process (section 6 and 7).

The routes the page uses (`GET /api/update/state`, and `POST` for check, apply, skip, rollback and the setting) take no URL, no path and no
free text: they carry a version string that is compared with what the Go side already holds, and nothing else. The `POST` routes are
behind the same loopback and Fetch Metadata guards as the write routes, and the guard-pin test lists them, so another web page or program
cannot make the editor update itself.

### 4.9 State kept

`settings.json` (roaming profile folder, as today) gets `updateCheck` and `updateSkipped`, because they are the user's choices. The state
that is only about this machine goes in a new `%LOCALAPPDATA%\tabr-tau\update-state.json`: the last `ETag`, the time of the last check
and of the last good answer, the highest serial number seen, the last verified `latest.json` for the release list, and, for rollback, the
SHA-256 and version of the previous exe. The file is written with the existing atomic write, holds no identifier and no secret, and a
missing or damaged file means "start fresh": it never blocks the editor.

## 5. Applying the update

### 5.1 Steps

The user clicks **Update now** in the panel. Then, in the Go side:

1. Refuse if this is a development build, if a write is in flight, or if this is not the only instance. Warn (and require a second click)
   if the queue of waiting edits is not empty: the queue is kept on disk and is put back at the next start, but the edits are not written
   while the editor is closed.
2. Fetch `latest.json` again and verify it again. If it now names another version, show that version and ask again. (This closes the
   gap between the check and the click, section 8.)
3. Download `manifest.json`, `SHA256SUMS`, `SHA256SUMS.minisig` and verify the signature. Check that `manifest.json` and the exe appear in
   `SHA256SUMS` with the hashes the manifest gives, and that version, build and commit agree between `latest.json`, the manifest and the
   tag in the download address.
4. Download the exe into the folder of the running exe as `<name>.new`, hashing the stream as it is written, with the size capped at the
   size in the signed manifest (a longer body stops the download). Stop and delete on any mismatch.
5. Close the file, reopen it with sharing that denies writing, hash it again from that handle and compare with the signed value. The new
   file is never started to check it.
6. Swap (section 5.3), then start the new exe and exit.

Nothing in steps 1 to 5 changes the running installation. A failure at any of them leaves the installed exe exactly as it was, deletes
the staging file, and shows one sentence with the reason.

### 5.2 The problem

On Windows a running exe cannot be overwritten or deleted, so a program cannot simply write its successor over itself. A running exe can
be renamed. Two methods follow from that.

### 5.3 Two methods compared, and the choice

| | A. Helper process | B. Rename and restart |
|---|---|---|
| How | The running app starts a helper (a second exe, or the new exe in an `--apply-update` mode) and exits; the helper waits for the old process to end, moves the files, starts the app. | The running app renames itself to `<name>.prev.exe`, renames `<name>.new` to `<name>.exe`, starts the new exe and exits. |
| Extra program that runs with the user's rights | Yes, and it runs between the verification and the swap, which is the moment that must not be attackable. A second exe is a second thing to sign, ship and trust. | No. |
| Rollback | Needs its own code, run the same way. | The same two renames run the other way; the `.prev.exe` is already the rollback copy. |
| When the folder is not writable (a protected folder) | Fails too, or asks to elevate. | Fails before anything is changed; the app offers to save the new exe to a folder the user picks and tells them to run it. It never asks for elevation. |
| What a crash can leave | A helper that waits forever or half-moved files. | A very short window between the two renames in which `<name>.exe` does not exist (section 5.3.1). |
| Matches today | No. | Yes: this is the `.prev.exe` pattern the maintainer's own swap already uses. |

Recommendation: **B, rename and restart.** It needs no second process, it is the same mechanism for update and rollback, and the part
that can fail is short, local and recoverable. Its one weak point, the window between the renames, is reduced and tested as follows.

#### 5.3.1 The swap in detail

On the same volume, in the folder of the exe (all names derived from the exe's own name, because users rename it):

1. If `<name>.prev.exe` exists, delete it (it is the one before the previous; only one rollback copy is kept).
2. Rename `<name>.exe` to `<name>.prev.exe`.
3. Rename `<name>.new` to `<name>.exe`. If this fails, rename `<name>.prev.exe` back at once and report the failure; nothing changed.
4. Record in the update state file: the version and SHA-256 of the previous exe and of the new one.
5. Start `<name>.exe` with the same arguments and the same working directory, in a visible window (never hidden), with
   `--updated-from=<old pid>`; then exit the old process.

The new process waits up to 15 seconds for the old process to end and for the listen address and the single-instance guard to be free,
then starts normally. If it cannot, it says so and does not start a second server.

If the process dies between steps 2 and 3, the folder holds `<name>.prev.exe` and `<name>.new`, both complete. The editor cannot start
itself then, so the README holds one two-line recovery ("rename `<name>.new` to `<name>.exe`"), and the update state file would hold the
intent. The window is microseconds; the test plan (section 10) kills the process at every step in a harness to prove that the folder
is always recoverable by that one rename.

Whether `ReplaceFileW` (which does the two renames as one call and keeps a backup) may be used on a running exe is to be settled by a
test on Windows 10 and 11 in the first slice of the apply work, before any release code depends on either. If it works it replaces steps
2 and 3 and shrinks the window to nothing; if not, the renames above stay.

The exe is written by the app itself, so it carries no "downloaded from the internet" mark and Windows does not ask the user to confirm
it. That is the intended trade: the app has verified it against the project's key, which is a stronger check than that mark.

### 5.4 Rollback

**Roll back** in the panel (shown when `<name>.prev.exe` exists and its SHA-256 matches the one in the update state file) swaps the two
files with the same renames, through a third temporary name so that the newer exe is kept and the user can roll forward again without a
download. No network is used and nothing is downgraded by a server: the old exe is a file that was verified when it was installed. After
a rollback the version rolled back from becomes the skipped version, so the dot does not return at once. A damaged `.prev.exe` (hash
does not match) is not offered.

### 5.5 What is not done

- No silent update, no update at start, no update because the app was idle. Every apply is a click.
- No update of a development build.
- No change to the save, the settings or the game files. The update replaces one exe.
- No elevation and no installer.

## 6. Signing

### 6.1 Options

| Option | What it gives | What it costs or risks |
|---|---|---|
| **ed25519 signature on the checksum file and the pointer** (minisign layout) | The app can verify, offline, that a file was approved by whoever holds the private key. Standard library only (`crypto/ed25519`), no new dependency (the module has no `x/crypto`). A person can check a download with the free `minisign` tool. | The project holds the key; the app embeds the public part. |
| **Authenticode** certificate | Windows shows a publisher name, and SmartScreen reputation builds up over time. The updater could also check the downloaded exe with the operating system's trust check. | A certificate costs money every year (a standard certificate or a monthly cloud-signing service; current terms to be checked), the key must be in a hardware token or a signing service, and it does not by itself tell the running editor which publisher to trust unless the app also pins the certificate. |
| Both | Strongest. | Both costs. |

Recommendation: **ed25519 now, Authenticode later and in addition.** The signature is what the updater trusts, because it is the one that
the app can check by itself with a key it already holds. Authenticode helps the people who download the first exe and see a SmartScreen
warning; it is a separate decision (question 1 of section 12) and does not change this design.

### 6.2 What is signed

Two files per release, both small texts: `SHA256SUMS` (which covers the exe, the manifest and the notes) and `latest.json`. The exe itself
is not signed, it is covered by its hash in the signed file; this keeps the signed object small and makes "SHA-256 and signature" one
chain: signature, then hash.

Format: the minisign file layout in its legacy (non-prehashed) mode, so that the verification is `ed25519.Verify` over the file bytes
plus a short parser, with no hash dependency outside the standard library, and the files can still be checked with the free tool. The
signed "trusted comment" carries `product=tabr-tau version=0.14.2 build=317`, which the app compares with the document (a signature
for one release cannot be glued onto another). The exact byte layout is fixed in the build slice from the format's specification and a
test vector produced by the tool itself.

### 6.3 Key custody

- The private key is generated on the operator's own machine, protected by a passphrase, and is **never in GitHub**: not as a secret, not
  in a runner, not in a file in the repository.
- A second key pair (the "next key") is generated at the same time and kept offline (for example on removable media in a safe place),
  encrypted, and never used for a release unless the first is lost or stolen.
- The app embeds both public keys from its first release (`internal/update/keys.go`, two 32-byte values, a short list, never fetched).
  A release is valid if signed by either.
- The fingerprints of both public keys are written in SECURITY.md, so a person can compare the key inside the exe with the project's
  own page. Changing the keys is a change to that file and to `keys.go` in one pull request, and a test compares the two.
- Who may sign: the operator only. The signing tool (`tools/releasesign`, standard library only, or the `minisign` tool) refuses to sign
  files it has not first shown the operator (version, commit, the hashes, whether the commit is on `main` and its checks are green).

### 6.4 Rotation

Planned rotation (every year or on a change of custody): a release signed with the current key adds a new "next" key to `keys.go`; the
following release is signed with the former "next" key; the oldest key is dropped from `keys.go` two releases later. Because the app
always carries the next key, a rotation never needs users to install anything by hand.

### 6.5 What a compromised release or a compromised key means

**A bad release signed by the real key** (a faulty build, a malicious change that passed review): users who updated have it.
Response: (1) do not move or delete the tag (the repository's rule for the module tags); (2) publish a fixed release at once with the bad
version in `withdrawn` in the signed `latest.json`; the app, on its next check, tells a user who runs a withdrawn version to update, in
words ("This version was withdrawn"), still with a click; (3) mark the bad release as a pre-release in GitHub so that it can no longer
be "latest" and the poll stops offering it; (4) a security advisory on the repository. The signed `withdrawn` list is the reason the
pointer is signed too.

**A stolen or lost private key**: whoever has it can sign a release that the app accepts, until users hold an app that does not trust it.
Response: (1) stop publishing; (2) the "next key" is used to sign a release whose `keys.go` drops the stolen key and whose `latest.json`
lists the stolen key's id under `revoked_keys`; an app that already carries the next key and sees that signed list refuses everything
signed by the revoked key from then on; (3) the repository advisory and SECURITY.md say which key id is revoked and give the new
fingerprints; (4) users whose app never saw the revocation are the risk that remains: the stolen key can still serve them a
validly-signed bad release until they install the clean one, so the advisory asks everyone to download the clean exe by hand and check
it against the checksum shown on the repository page. This is why the key is on the operator's machine under a passphrase and not in CI,
and why the app updates only on a click. Nothing in this design makes a stolen key harmless; it makes it rare, detectable and recoverable.

**A compromised GitHub account or repository** (the update host): the attacker can replace assets but cannot sign them. The app rejects
them (nothing is installed), and the effect is that updates stop. That is the property recommended in R2; it is lost if the key is moved
into CI.

## 7. The release workflow

A new workflow `release.yml`, a change to CI control, so its pull request gets a **high** code review before merge and is covered by
`CODEOWNERS` for `/.github/` as it is today. Its shape follows the `design` and `kit` release workflows that already exist.

Trigger: a push of a tag `v[0-9]*.[0-9]*.[0-9]*` (which cannot match `design/v...` or `kit/v...`). Only the owner can create such
a tag: a tag ruleset for `v*` restricts creation to the owner (a repository setting, done by hand).

Jobs, with permissions `contents: read` unless stated:

1. **verify**: the tag matches the form; the tagged commit is on `main` (the same `merge-base --is-ancestor` check); the required checks
   of that commit are all green (a read of the check runs of the commit for the names the `main` rule requires; any other state fails);
   `CHANGELOG.md` has a section for the version; the tree is clean; the version has no existing release.
2. **build** (needs verify): builds with the pinned Go from `go.mod`, with the flags of `scripts/buildinfo.ps1` (section 1.4), writes
   `manifest.json`, `SHA256SUMS`, the notes (a script that extracts the CHANGELOG section, like the two module scripts), and
   `latest.json` (unsigned at this point), and uploads them as one workflow artifact. It builds twice and compares the two exes (the
   check of the reproducibility aim).
3. **draft** (needs build; `contents: write`, only this job): creates a **draft** release with those files. A draft is invisible to
   everyone and is never "latest".
4. The operator signs on his own machine (`tools/releasesign v0.14.2`): it downloads the draft's files, shows what it will sign,
   signs `SHA256SUMS` and `latest.json`, uploads the two signatures, and publishes the draft with `--latest`. A tag from another person,
   or a draft no one signs, never reaches users.
5. **verify-published** (a second small workflow on the `published` event; `contents: write` only for the one step that demotes a
   release): downloads what is public, verifies both signatures with the public keys of `keys.go`, checks every hash, and runs the exe's
   `--version` against the tag. If anything fails it marks the release as a pre-release (so the poll no longer offers it) and fails
   loudly.

Rules that hold for all of it, from the existing `public-text` and CI documents: actions pinned to a commit id and only first-party
actions; no `pull_request_target`; no `${{ }}` expression that carries text from an event into a shell step; secrets: none are needed
(the signing key is not one). The workflow's text and the release notes are public text and pass the public-text check; a release is
created from the CHANGELOG section, which a pull request already reviewed.

A GitHub build provenance attestation can be added to the build job later (first-party action). The app does not rely on it: it cannot
check one offline, and the signature already provides the property the app needs.

## 8. Threat model

Scope: an attacker who is not already running code as the user on the machine (such a program can replace the exe or the save directly
and nothing here defends against that).

| Threat | How it would work | Mitigation | Residual |
|---|---|---|---|
| Downgrade | Serve an older, validly signed release (with a known flaw) as if it were new. | The offer needs `version` and `build` both greater than the running ones, and the serial is greater than the highest ever seen (kept in the state file). The only downgrade is the user's own rollback to a local, previously verified file. | None for a network attacker. |
| Replay or freeze | Keep serving an old signed `latest.json` so users never learn of a new release. | The panel shows the date of the last good answer, and "last checked" turns into a visible warning after 30 days without one. | A blocked or replayed pointer delays an update; it cannot install anything. Accepted. |
| Malicious manifest | A crafted `latest.json` or `manifest.json`: path or URL in a file name, huge sizes, deep JSON, text meant to be run, HTML in the notes. | The signature is checked before any field is used. File names match a strict pattern and are only joined to a fixed address by the app. Sizes are capped in the code and the stream is cut at the signed size. JSON is read with a size limit and unknown fields are ignored. Notes are text only, shown with `textContent`, cut to a length. | A signed document is trusted as far as its signer. |
| MITM | Alter the download on the network. | HTTPS to a short list of hosts with the system roots, no downgrade to HTTP, redirects only inside the list, then the signature and the hash. A MITM that defeats TLS still cannot sign. | Can block (denial). |
| Key theft | The private key leaves the operator's machine. | Key never in GitHub, passphrase, offline next key, revocation list, section 6.5. | A thief with the key and a network position can serve a bad release to users until it is revoked. |
| Update server compromise | The repository, a release or the asset host is replaced. | Assets are untrusted until verified; without the key the attacker can only stop updates. | Denial. Lost if the key moves into CI. |
| Compromised workflow or build | A change to the workflow or a dependency alters the exe in CI. | CI only drafts; the operator signs what he sees (hashes, commit); the release is built twice and compared; actions pinned; the workflow is a high-review change; the published release is verified again. | A malicious change in the source that passes review is signed. That is the review's job, not the updater's. |
| TOCTOU between check and apply | The pointer or the downloaded file changes after it was checked. | The pointer is fetched and verified again at the click. The hash is computed on the stream as written and again from the final file just before the swap, from a handle that denies writing. The exe is never started to verify it. | A same-user program that wins a microsecond race on the folder could swap the file; such a program could write the exe directly. Out of scope above. |
| Page or program triggers an update | A web page or another local program calls the apply route. | Loopback only, Fetch Metadata and Host guards, the apply route is a `POST` that the guard-pin test lists, and the page asks for a click on a visible button with the version shown. | A local program as the user, out of scope. |
| Rollback copy tampered | The `.prev.exe` is replaced. | Its hash is checked against the state file before it is offered or used. | A same-user program could change both; out of scope. |
| Privacy leak | The check tells a third party something about the user. | Section 4.6: a plain request of a public file, a fixed `User-Agent` with no version, no identifiers; off by default until the user answers. | The host sees the IP address and the time. Said in the README. |
| Denial of service on the host | Many users poll at once. | Jitter, 6 hours, ETag, `Retry-After`, a 24 hour cap on backoff. | |

## 9. What the Settings gear needs (for #850)

An **Updates** section, with these and nothing else from this feature:

1. **Check for updates**: Yes, No (the state of section 4.7; shows "Not answered" until it is).
2. **Check now**, with the time and result of the last check.
3. **Skipped version**: shows the version, with **Stop skipping**.
4. **Channel**: shows `Stable`; the only choice. It exists so the field is in place; no second channel is built.
5. **Previous version**: shows the version kept for rollback and a **Roll back** button, or "none".
6. A line stating what the check sends (section 4.6), kept in the section, not hidden in a tooltip.

Where stored: `updateCheck` and `updateSkipped` in `settings.json`; everything else in `update-state.json` (section 4.9). Until the gear
exists, the switch and the question live in the About panel, so the feature does not wait for #850. Not in Settings: the 6 hour interval,
the host list, the key, the download folder (a fixed behaviour, not a preference).

## 10. Test plan

Unit tests, pure Go, run in the existing `go` and `windows` jobs:

- Version and serial: comparison of releases, pre-releases, build metadata; a development build; downgrade, equal and skipped cases.
- Document parsing: valid files; unknown fields; wrong `product`, `channel` or schema; names with `..`, slashes, drive letters, control
  characters, over-long names; sizes that are negative, huge or wrong; duplicate keys; deep nesting; a body at and over the limit.
- Signature: a good signature; a flipped byte in the document; in the signature; in the trusted comment; a signature of another release
  glued on; the wrong key; the next key; a revoked key; a truncated file; a test vector made by the `minisign` tool.
- Hash: the exe, a truncated body, a longer body, a stream that stops, a size that differs from the signed one.
- State: damaged, missing, future-schema state files; the serial number never goes down.

Poll tests against a local TLS test server (the test build replaces the host list and the key through a build tag that the release build
cannot carry; a CI step greps the release flags for the tag):

- 200 then 304 with `ETag`; 429 and 403 with `Retry-After` and with `X-RateLimit-Reset`; 5xx; a timeout; a redirect to a host outside the
  list; a redirect to `http`; a body over the limit; a connection reset; offline (no error shown).
- Jitter and schedule on a fake clock: first check within its window, the next in 6 hours plus 0 to 30 minutes, no check while off or
  unanswered, no more than one **Check now** a minute.
- Nothing is sent but the request: the test server records every header and asserts the `User-Agent` and the absence of any other
  identifying header or cookie.
- The netguard test: still fails for any other file that uses a client; passes for `client.go`; a second test for the pins of 4.8.

Apply tests:

- A temp folder with a fake exe and an injectable rename: every failure point of steps 1 to 6 and every crash point of 5.3.1, with the
  folder recoverable by the single rename each time; a hash mismatch leaves the installed exe byte for byte as it was; a read-only
  folder; a folder with the `.prev.exe` already there.
- Windows only (the `windows` job): start a copy of a test binary that sleeps, rename it while it runs, put another in its place, start
  that, confirm the first keeps running and exits cleanly. Run on Windows 10 and 11 images, and once with the folder inside a
  synchronised user folder and with Defender real-time scanning on, by hand. The same test decides the `ReplaceFileW` question.
- Rollback: swap, swap back, hash check, a damaged `.prev.exe`.

Web and page tests:

- The routes: guard pin (cross-origin `POST` refused, no URL or path parameter accepted), `GET` state shape.
- The label, the dot, the dialog, the toast: the existing static accessibility, contrast and text tests extended; text from the network
  rendered as text (a notes string with markup stays inert); a screen-reader sentence for the dot.

Release tests:

- A dry run mode of the workflow on a test tag builds and verifies but publishes nothing.
- `tools/releasesign` run against a fixture draft; the verify-published job run against a good and a deliberately broken release.
- Manual, once per major step, recorded in `docs/live-test-protocol.md`: a real update from release N to N+1 on a clean user profile, a
  rollback, an update with the game running, an update with edits queued, an offline start, the check turned off.

## 11. Phased build plan

Issues to file once the note is accepted (each with `cat:` and `p` labels, a test written first, and CI green before review).
Slices 4 and 5 and 3 are security-sensitive and get a **high** code review.

1. **Version stamp, header label and About panel (no network).** `internal/buildinfo`, `scripts/buildinfo.ps1`, the local build script
   and the CI cross-compile call it, `--version`, `ws.EditorVersion` fixed, label and a panel that shows the version, date, commit and
   channel only, the test that the stamp reaches the exe. The smallest safe slice: nothing leaves the machine, nothing is replaced.
2. **The self-replace spike.** A test-only change (no product code) that settles section 5.3: rename a running exe on Windows 10 and 11,
   `ReplaceFileW` or not, the behaviour with a synchronised folder and Defender. The result is written into this note.
3. **Release workflow, signing tool and keys.** `release.yml`, the verify-published workflow, `tools/releasesign`, `internal/update`'s
   pure part (parse, verify, version compare) and `keys.go`, the key ceremony, SECURITY.md (release process, fingerprints), the README
   download and check instructions, the tag ruleset, the first release `v0.1.0` marked pre-release to prove the path. No update check yet.
4. **The check (notify only).** `internal/update/client.go`, the schedule, state file, the three-state setting and the question, the
   dot and the toast, the Updates section in the About panel, the narrowed netguard test, the README and SECURITY.md privacy text, and the
   guard-pin test for the routes.
5. **Apply and rollback.** Staging, double verification, the swap of section 5.3, restart, rollback, skip, the crash harness and the live
   tests. Behind a "development build refuses" rule until slice 3's first real release exists.
6. **Settings gear section** (belongs to #850): moves the three controls into the gear. Authenticode (slice 7) is separate.
7. **Authenticode signing** (only if the operator buys a certificate or service): signing step in the release path, the SmartScreen
   guidance, optional `WinVerifyTrust` check in the updater.

Order: 1, then 2 and 3 in parallel, then 4, then 5. A release made after slice 3 is downloaded by hand; slices 4 and 5 are what turn it
into an update.

## 12. Open questions for the operator

Only what is really the operator's to decide:

1. **Authenticode.** Buy a code-signing certificate or use a cloud signing service, now, later or never? The design works without it
   (the ed25519 signature is what the updater trusts); it only affects the SmartScreen warning on a first download by hand and costs money
   every year.
2. **Default of the check.** Recommended: ask once, before any connection, and never check until the answer is Yes (section 4.7). It matches
   #577's "opt-in" and the README's present promise. The alternative is "on until turned off", which is what the issue literally says.
3. **Who signs.** Recommended: the operator, on his own machine, with the key never in GitHub (section 6.3), which costs one command per
   release. The alternative is a key in a protected CI environment, which saves that command and gives up the property that a stolen
   repository account cannot ship a release.
4. **First version number.** `v0.1.0` for the first release (the CHANGELOG says `v0.x` until the first audited release), or another
   starting point.

Decided in this note, not asked: the poll, the six hours, the toast and dot, the skip and off switches, rename-and-restart, ed25519 now,
one channel.
