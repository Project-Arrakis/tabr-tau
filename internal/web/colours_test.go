package web

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/design"
)

// The Colours group of the Theme menu (#587, refs #570). The palettes are the design module's: palettes.css is served from /design/ and linked by both pages after the look's files, and the
// table of palettes in look-boot.js (the ids, labels, groups and themes the menu shows) is the module's own list, checked here line by line.

// jsLine is a table entry of look-boot.js as it is written there.
func jsLine(p design.Palette) string {
	q := "'"
	if strings.Contains(p.Name, "'") {
		q = `"`
	}
	return "{ id: '" + p.ID + "', name: " + q + p.Name + q + ", group: '" + p.Group + "', theme: '" + p.Theme + "' },"
}

func TestThePaletteTableOfTheBootScriptIsTheModulesOwn(t *testing.T) {
	b, err := os.ReadFile("static/look-boot.js")
	if err != nil {
		t.Fatal(err)
	}
	src := strings.ReplaceAll(string(b), "\r\n", "\n")
	start := strings.Index(src, "const PALETTES = Object.freeze([\n")
	end := strings.Index(src, "  ]);\n")
	if start < 0 || end < start {
		t.Fatal("look-boot.js has no PALETTES table")
	}
	var rows []string
	for _, l := range strings.Split(src[start:end], "\n")[1:] {
		if l = strings.TrimSpace(l); l != "" {
			rows = append(rows, l)
		}
	}
	want := design.Palettes()
	if len(want) != 18 || want[0].Name != "Arrakis Incandescent" {
		t.Fatalf("the module lists %d palettes, the first %q", len(want), want[0].Name)
	}
	if len(rows) != len(want) {
		t.Fatalf("look-boot.js lists %d palettes, design.Palettes() %d", len(rows), len(want))
	}
	for i, p := range want {
		if rows[i] != jsLine(p) {
			t.Errorf("palette %d of look-boot.js is\n  %s\nwant\n  %s", i, rows[i], jsLine(p))
		}
		if files, ok := design.PaletteFiles(p.ID); !ok || len(files) != 1 {
			t.Errorf("design.PaletteFiles(%q) = %v, %v", p.ID, files, ok)
		}
	}
	// the key is the editor's own, and the file name is not made up in the script
	if !strings.Contains(src, "palette: 'tauPalette'") {
		t.Error("look-boot.js lacks the key tauPalette")
	}
	if strings.Contains(src, "'/design/"+design.PalettesFile) {
		t.Error("look-boot.js names the palettes stylesheet: the pages link it, a stored value never builds a file name")
	}
}

func TestThePalettesStylesheetIsServedAndLinkedAfterTheLookFilesOnBothPages(t *testing.T) {
	s := newTestServer(t)
	w := do(t, s, "GET", "http://127.0.0.1/design/"+design.PalettesFile, loop, map[string]string{"Host": "127.0.0.1:8090"}, nil)
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("/design/%s: %d, %q", design.PalettesFile, w.Code, w.Header().Get("Content-Type"))
	}
	body := w.Body.String()
	for _, p := range design.Palettes() {
		if !strings.Contains(body, `:root[data-palette="`+p.ID+`"][data-theme="`+p.Theme+`"]`) {
			t.Errorf("%s has no block for %s in the theme %s", design.PalettesFile, p.ID, p.Theme)
		}
	}
	if want, err := fs.ReadFile(design.Assets, design.PalettesFile); err != nil || body != string(want) {
		t.Errorf("/design/%s is not the module's file as it is", design.PalettesFile)
	}
	files, _ := design.LookFiles(design.DefaultLook)
	for _, page := range []string{"static/index.html", "static/locked.html"} {
		b, err := os.ReadFile(page)
		if err != nil {
			t.Fatal(err)
		}
		s := strings.ReplaceAll(string(b), "\r\n", "\n")
		head := s[:strings.Index(s, "</head>")]
		link := `<link rel="stylesheet" href="/design/` + design.PalettesFile + `">`
		at := strings.Index(head, link)
		if at < 0 || strings.Count(head, link) != 1 {
			t.Fatalf("%s: the head must link %s once", page, link)
		}
		for _, f := range files {
			if i := strings.Index(head, `href="/design/`+f+`"`); i < 0 || at < i {
				t.Errorf("%s: %s must come after the look's %s", page, design.PalettesFile, f)
			}
		}
		if app := strings.Index(head, `href="/app.css"`); app < at {
			t.Errorf("%s: app.css must come after %s", page, design.PalettesFile)
		}
		if boot := strings.Index(head, `<script src="/look-boot.js"></script>`); boot < at {
			t.Errorf("%s: look-boot.js must come after the stylesheets", page)
		}
	}
}
