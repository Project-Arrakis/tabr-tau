package design

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// Version is the version of this module. It equals the contents of the VERSION file and the newest heading of CHANGELOG.md (a test checks
// both), and on a tag build the tag design/vX.Y.Z (the release workflow checks that).
const Version = "v0.1.0"

// TokensFile is the path of the stylesheet with the tokens inside Assets.
const TokensFile = "tokens.css"

//go:embed assets
var embedded embed.FS

// Assets holds the embedded files, rooted at the module's assets folder: Assets has "tokens.css", not "assets/tokens.css". It is
// read-only.
var Assets fs.FS = mustSub(embedded, "assets")

func mustSub(f fs.FS, dir string) fs.FS {
	s, err := fs.Sub(f, dir)
	if err != nil {
		panic("design: " + err.Error()) // the folder is embedded by the line above; a missing folder fails every build
	}
	return s
}

// Tokens returns the tokens stylesheet. The slice is a copy that the caller may change.
func Tokens() []byte {
	b, err := fs.ReadFile(Assets, TokensFile)
	if err != nil {
		panic("design: " + err.Error()) // the file is embedded; a test checks it exists
	}
	return b
}

// Handler serves the files of Assets, read-only: GET and HEAD only, no folder listing, and a stylesheet is always text/css whatever the
// host's own table of file types says (on Windows that table is the registry's). Mount it under a prefix of the program's own, for
// example http.StripPrefix("/design/", design.Handler()). It sets no cache header: the program decides, and a program that embeds a
// version of this module can serve its files with whatever caching its own files get.
func Handler() http.Handler {
	files := http.FileServerFS(Assets)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "read-only", http.StatusMethodNotAllowed)
			return
		}
		p := r.URL.Path
		if p == "" || strings.HasSuffix(p, "/") {
			http.NotFound(w, r) // no folder listings
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if strings.HasSuffix(p, ".css") {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		}
		files.ServeHTTP(w, r)
	})
}
