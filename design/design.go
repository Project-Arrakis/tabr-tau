package design

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Version is the version of this module. It equals the contents of the VERSION file and the newest heading of CHANGELOG.md (a test checks
// both), and on a tag build the tag design/vX.Y.Z (the release workflow checks that).
const Version = "v0.2.0"

// TokensFile is the path of the stylesheet with the tokens of version 0.1.0 inside Assets. It is unchanged since that version, so a program
// written against it keeps its look. The three looks have their own files (see Looks and LookFiles).
const TokensFile = "tokens.css"

// The looks of the module, by id. Fremen is the default; the others are chosen by a program's own setting.
const (
	Fremen    = "fremen"
	Awakening = "awakening"
	Sietch    = "sietch"

	// DefaultLook is the look a program starts with when nothing is chosen.
	DefaultLook = Fremen
)

// Looks lists the ids of the looks, the default first.
func Looks() []string { return []string{Fremen, Awakening, Sietch} }

// LookFiles returns the stylesheets of a look in the order they must be linked, as paths inside Assets: the tokens of the look, the components
// of the look, the components every look shares, and the motion rules. ok is false for an id that is not one of Looks. A program that lets
// its user switch looks takes the file names from here and never from a stored value, so a stored string cannot name a file.
func LookFiles(look string) (files []string, ok bool) {
	switch look {
	case Fremen, Awakening, Sietch:
		return []string{"tokens-" + look + ".css", "components-" + look + ".css", "components.css", "motion.css"}, true
	}
	return nil, false
}

// Other files of the module, as paths inside Assets.
const (
	IconsFile   = "icons.svg" // the icon sprite, one <symbol id="i-NAME"> per icon
	GrainFile   = "grain.svg" // the tile of the Awakening look's surface grain
	FontFile    = "fonts/DuneRise.ttf"
	FontLicence = "fonts/DuneRise-OFL.txt"
)

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

// Tokens returns the tokens stylesheet of version 0.1.0 (tokens.css). The slice is a copy that the caller may change.
func Tokens() []byte {
	b, err := fs.ReadFile(Assets, TokensFile)
	if err != nil {
		panic("design: " + err.Error()) // the file is embedded; a test checks it exists
	}
	return b
}

// types is the type that Handler sends for each extension the module carries. It does not use the host's own table: on Windows that table
// is the registry's, and with nosniff a stylesheet, a sprite or a font served with another type is not used. A file with another extension
// is sent as application/octet-stream.
var types = map[string]string{
	".css": "text/css; charset=utf-8",
	".svg": "image/svg+xml",
	".ttf": "font/ttf",
	".txt": "text/plain; charset=utf-8",
}

// contentType returns the type of a file by its name; SHA256SUMS and other files without an extension are plain text.
func contentType(p string) string {
	base := path.Base(p)
	if t, ok := types[strings.ToLower(path.Ext(base))]; ok {
		return t
	}
	if base == "SHA256SUMS" {
		return "text/plain; charset=utf-8"
	}
	return "application/octet-stream"
}

// Handler serves the files of Assets, read-only: GET and HEAD only, no folder listing, and every file with the type of its extension (see
// types) whatever the host's own table says. Mount it under a prefix of the program's own, for example
// http.StripPrefix("/design/", design.Handler()). It sets no cache header: the program decides, and a program that embeds a version of this
// module can serve its files with whatever caching its own files get.
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
		w.Header().Set("Content-Type", contentType(p))
		files.ServeHTTP(w, r)
	})
}
