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
  config folder and its own settings, or write the save without the game-running check.
- A crafted save that crashes the editor or exhausts its memory.
- A way to read the session token, or to learn account identifiers the editor hides.

Out of scope: the game itself, Microsoft's WebView2 runtime and Edge, and anything that needs the attacker to already run code as you on
your computer (they can edit your save directly).

## How it is built to resist these (summary)

- The editor binds to loopback only, checks the Host header and the peer address, and refuses a request from another site (fetch metadata and
  Origin). The session is an HttpOnly, SameSite=Strict cookie, handed over once through a link that works one time for ten minutes.
- All HTML is built by an escaping template function; the page allows only its own scripts and styles.
- Every edit is written to the save as it is made. Before each write the editor checks that no single-player session is using the file (and
  that the game check itself worked), that the file is unchanged since it was read, and that the working copy passes an integrity check; it
  takes a backup first (one backup covers a burst of edits made within two minutes; the newest 60 automatic backups are kept, and always the oldest
  and the first one of the session; backups made by a person or an earlier version are never removed) and reads the written file back and compares it afterwards (a file that cannot be read back is looked at again before anything is written a second time; a window of milliseconds remains between the last check of the file and the rename, which no rename can close, and a game save inside it is caught by the read-back, which leaves the game's file alone; a journal that could not be kept up to date is reported at the next start and not put back). There is no free-space check ahead of a write: a disk without room fails the backup or the temp file before the file is replaced and changes nothing, and the message names that write. Each edit is also checked
  for dangling references when it is made. When the file cannot be written (a single-player session is running, the location is not
  confirmed, the game check failed) the edits wait in a queue that is kept in a journal and listed on the page, and are written by
  themselves once the file can be written. If the game saved meanwhile, the queue is put back onto its newer file only when every edit still
  applies; if the game saved changes that conflict with it, none of it is written and the person is told.
- The optional automatic pass when the editor opens (master switch off by default, one switch per job) is one edit like any other: the same
  guards, one backup, one write-log line. It is skipped while a single-player session is running, writes nothing when nothing needs doing,
  and a failing job rolls the whole pass back. It acts only on what the character owns or carries.
- There is no review step before a write (the review pane was removed in #238). What stands in its place is the set of guards above at every
  write, the backups (Database > Save backups restores one), the queue with its list and its Discard control, and the confirmations that
  remain for destructive actions. The audit register records this replacement.
- Whether the game is running is read from the system's process list in memory (the Windows process snapshot); the editor starts no other program
  to find out, so a program placed earlier in the search path cannot answer for it and there is no child process to fail. A check that fails is tried
  again a few times before it blocks anything. If it still cannot be made, the write is refused and the edit waits in the queue, with the reason
  shown; the editor checks again by itself, and **Check again** and **Write now** run the check at once. A write never relies on an older answer.
- Every write is recorded: after the file has been written the editor appends one line to `write-log.jsonl` in its own folder
  (`%APPDATA%\tabr-tau`): when, how many edits, what they were in words, and the name of the backup taken before (a restore of a backup is
  recorded the same way). The words are the sentences the page lists as changes, so they can carry game ids and amounts ("Give 500 x ... to
  container 12", "moved vehicle 201 next to player"); the optional pass when the editor opens is one line, "automatic pass: 1 Water Cistern;
  11 vehicle parts repaired". They do not carry a statement typed into the SQL console (only that one was run), the
  primary-key value of a row edited in the Database view, a number of fifteen digits or more, a long hexadecimal id or GUID, or a file path.
  The log is capped: at 256 KB it becomes `write-log.1.jsonl` and a new file starts, so there are never more than two. It is created for the
  owner (mode 0600), but Windows does not apply that mode: the file has the permissions of your profile folder, normally readable by you and by
  administrators only. It is a record to read, not a way back (the backup is), and the editor never reads it.
- Saves are opened from a private copy, and the container is checked (size, declared size, integrity) before anything is trusted.

## Accepted risk: a write on every accepted request

Since the review step was removed, an edit request that the editor accepts is written to the game's file before the answer is sent (or queued
while the file cannot be written). There is no second, human step between the request and the write. This is accepted, on these grounds:

- The only client is the editor's own page. A request must carry the session cookie (HttpOnly, SameSite=Strict, valid for this run of the
  editor only), come from the loopback address with a Host header the editor expects, and pass the fetch-metadata and Origin checks, so another
  web page, another site or another computer cannot make one.
- A program that already runs as you, and could therefore replay the cookie, can edit the save file directly; that is out of scope above.
- What limits the damage of a request that is wrong or unwanted is the same set of guards at every write (no single-player session, an unchanged
  file, an integrity check, a read-back), a backup before each burst of edits that Database > Save backups restores, the queue with its list
  and Discard control, a typed word before a raw SQL write or a restore, a typed item count before a container is cleared, a fill that only raises two values of a blood sack and leaves the others alone, a refuel that raises one fuel number of each of your own vehicles and never lowers one, a fill of all liquids that is one edit and is put back as a whole if any part fails, a question before the bulk actions, and the write log above.

This is to be looked at again if the editor ever listens beyond this computer, serves more than one person, or offers an interface to other programs.

## Known limits that are documented, not hidden

- **The one-time link.** When the editor opens in the browser (`--web`) it hands the default browser a link that carries a single-use token
  in its query string. It is valid for ten minutes and for one use, and the editor redirects away from it at once, but it passes through
  the browser's history and the command line of the process that opens it. The native window (the Windows default) does not use the browser.
- **The session cookie is a bearer token.** Browsers send a cookie for `127.0.0.1` to every port of that address (RFC 6265 does not scope a
  cookie by port; the cookie's name carries the port only so two editors do not overwrite each other's), so another program on this
  computer that you browse to on another port receives the header, and a local program that is given the cookie can replay it. In the native
  window the editor is the only site the window visits, so no other listener sees the cookie; with `--web`, use a browser profile of its own
  for the editor, and close other local web servers you browse to while it runs. The cookie is valid for this run of the editor only.
- **The queue of waiting edits is a plain file.** While edits wait (the game is in a single-player session) they are kept in
  `pending-<id>.json` in `%APPDATA%\tabr-tau` so they survive a restart. It holds what each edit ran, with the values of the save they used, as
  plain text. It is created with mode 0600 (which Windows does not apply: the file has the permissions of your profile folder) and removed
  when the edits are written or discarded, but `%APPDATA%` is the roaming profile folder, which some organisations copy to a server. When the edits are put back (at start, or onto a newer save) the recorded
  statements are run as they were recorded, without the SQL console's checks: they were checked or made by the editor when the edit was first
  made, and only a program running as you can plant a file there, which is out of scope above. A queue that cannot be kept says so on the page.
- **The decoded copy of the save.** To edit, the editor writes a decoded (plain SQLite) copy of the save, which holds the account and
  platform identifiers, to a folder of its own in your temp folder (`tabr-tau-work`), named by the process that made it. It is removed when
  the editor closes, including on Ctrl+C in a terminal; a copy that a crash or a kill left behind is removed at the next start, once its
  process is gone. Anyone who can read your temp folder as you can read it while the editor runs.
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
