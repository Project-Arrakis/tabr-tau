# Third-party notices

tabr-tau is built with the Go modules listed in `go.mod`; each keeps its own licence. The ones that ship inside the
Windows executable and need their notice kept:

| Module | Licence | Used for |
|---|---|---|
| `modernc.org/sqlite` and its `modernc.org/*` dependencies | BSD-3-Clause | the pure-Go SQLite engine that reads and writes the save |
| `github.com/jchv/go-webview2` | MIT, Copyright (c) 2020 John Chadwick | the native window (Microsoft Edge WebView2) |
| `github.com/jchv/go-winloader` | ISC, Copyright (c) 2021 John Chadwick | loads the WebView2 loader from memory |
| `golang.org/x/sys` | BSD-3-Clause, Copyright (c) The Go Authors | Windows system calls |

The full licence texts are in each module's repository and in the Go module cache (`go mod download -json`).
tabr-tau does not bundle the WebView2 runtime itself; Windows 11 and current Windows 10 include it.
