package web

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The Command Center's art and styles (milestone M2): the runtime copies of the game pictures, the manifest that describes them, the fixed list that serves them, and the rule that the
// preview gallery and its demo values never reach the editor. The helpers and the 25-seat geometry are tested in web-tests/lcc-*.test.mjs.

const lccManifestPath = "../../docs/design/landsraad-command-center/art-manifest.json"

type lccFile struct {
	Role   string `json:"role"`
	Key    string `json:"key"`
	File   string `json:"file"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Source string `json:"source"`
}

type lccManifest struct {
	HouseAlias map[string]string `json:"houseAlias"`
	DecreeArt  map[string]struct {
		File  string `json:"file"`
		Basis string `json:"basis"`
	} `json:"decreeArt"`
	DecreeNotRuntime []string  `json:"decreeNotRuntime"`
	DecreeMissing    []string  `json:"decreeMissing"`
	Files            []lccFile `json:"files"`
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func readLccManifest(t *testing.T) lccManifest {
	t.Helper()
	b, err := os.ReadFile(lccManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var m lccManifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("art-manifest.json: %v", err)
	}
	return m
}

// webpSize reads the canvas size of a WebP file (VP8X, VP8L and VP8 headers).
func webpSize(b []byte) (int, int, bool) {
	if len(b) < 30 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
		return 0, 0, false
	}
	switch string(b[12:16]) {
	case "VP8X":
		return 1 + int(b[24]) | int(b[25])<<8 | int(b[26])<<16, 1 + int(b[27]) | int(b[28])<<8 | int(b[29])<<16, true
	case "VP8L":
		v := binary.LittleEndian.Uint32(b[21:25])
		return int(v&0x3fff) + 1, int((v>>14)&0x3fff) + 1, true
	case "VP8 ":
		return int(binary.LittleEndian.Uint16(b[26:28]) & 0x3fff), int(binary.LittleEndian.Uint16(b[28:30]) & 0x3fff), true
	}
	return 0, 0, false
}

// Resource existence: every file the manifest names is embedded with the recorded bytes, hash and size; every embedded file of the art folder is in the manifest (no strays); the counts
// are the ones the design needs (25 Houses, 5 lanes, 5 banners, the decree pictures and the neutral one).
func TestLccArtManifestMatchesTheEmbeddedFiles(t *testing.T) {
	m := readLccManifest(t)
	want := map[string]lccFile{}
	counts := map[string]int{}
	for _, f := range m.Files {
		want[f.File] = f
		counts[f.Role]++
	}
	if counts["house"] != 25 || counts["lane"] != 5 || counts["banner"] != 5 || counts["decree"] != 9 || counts["decree-fallback"] != 1 || counts["faction-mark"] != 2 || counts["faction-medal"] != 2 || counts["standard-tip"] != 2 {
		t.Errorf("manifest counts %v: want 25 houses, 5 lanes, 5 banners, 9 decrees, 1 neutral picture, 2 faction crests, 2 faction medallions and 2 cloth standard feet", counts)
	}
	var embedded []string
	err := fs.WalkDir(static, "static/lcc/art", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		embedded = append(embedded, strings.TrimPrefix(p, "static/lcc/art/"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(embedded)
	for _, e := range embedded {
		if _, ok := want[e]; !ok {
			t.Errorf("%s is embedded but not in art-manifest.json", e)
		}
	}
	for file, f := range want {
		b, err := static.ReadFile("static/lcc/art/" + file)
		if err != nil {
			t.Errorf("%s is in the manifest but not embedded: %v", file, err)
			continue
		}
		sum := sha256.Sum256(b)
		if len(b) != f.Bytes || hex.EncodeToString(sum[:]) != f.SHA256 {
			t.Errorf("%s: %d bytes / hash %s, manifest says %d / %s", file, len(b), hex.EncodeToString(sum[:])[:12], f.Bytes, f.SHA256[:12])
		}
		if strings.HasSuffix(file, ".webp") {
			w, h, ok := webpSize(b)
			if !ok || w != f.Width || h != f.Height {
				t.Errorf("%s: picture size %dx%d (ok %v), manifest says %dx%d", file, w, h, ok, f.Width, f.Height)
			}
			wantSize := map[string]int{"house": 128, "lane": 256, "banner": 256, "decree": 128, "faction-mark": 256}[f.Role]
			if f.Role == "faction-medal" {
				// the round medallions keep the size of their reference pictures (100 and 104 px), so they are checked against the reference file, not a fixed size
				wantSize = 0
				if w != h || w < 64 || w > 128 {
					t.Errorf("%s: %dx%d, a faction medallion is a square between 64 and 128 px", file, w, h)
				}
			}
			if wantSize != 0 && (w != wantSize || h != wantSize) {
				t.Errorf("%s: %dx%d, a %s picture is %d x %d (the banners are 256 x 256, not the 512 the folder README says)", file, w, h, f.Role, wantSize, wantSize)
			}
			if len(b) > 40<<10 {
				t.Errorf("%s is %d bytes: runtime copies are meant to stay small", file, len(b))
			}
		}
		if f.Role == "standard-tip" {
			// the feet of the cloth standards are PNG files of the approved design, kept at their exact pixels: a PNG header with the size the manifest records
			if len(b) < 24 || string(b[1:4]) != "PNG" || int(binary.BigEndian.Uint32(b[16:20])) != f.Width || int(binary.BigEndian.Uint32(b[20:24])) != f.Height || f.Height != 111 || f.Width < 300 || f.Width > 330 {
				t.Errorf("%s is not a PNG of the recorded %dx%d size", file, f.Width, f.Height)
			}
			if len(b) > 64<<10 {
				t.Errorf("%s is %d bytes: keep each foot picture under 64 KiB", file, len(b))
			}
		}
		if !strings.HasSuffix(file, ".svg") && strings.HasPrefix(f.Source, "docs/art/landsraad/") {
			if _, err := os.Stat("../../" + f.Source); err != nil {
				t.Errorf("%s: its source picture %s is missing: %v", file, f.Source, err)
			}
		}
	}
	total := 0
	for _, f := range m.Files {
		total += f.Bytes
	}
	if total > 640<<10 {
		t.Errorf("the runtime art is %d bytes together; keep it under 640 KiB", total)
	}
}

// The House list is the 25 symbols the Landsraad tab already serves; the save spells one House Mikkarol and the reference picture Mikarrol, and that is an explicit alias.
func TestLccHouseArtIsTheSameTwentyFiveAsTheTabAndTheAliasIsExplicit(t *testing.T) {
	m := readLccManifest(t)
	tab, err := fs.ReadDir(static, "static/houses")
	if err != nil {
		t.Fatal(err)
	}
	var a, b []string
	for _, e := range tab {
		a = append(a, strings.TrimSuffix(e.Name(), ".webp"))
	}
	for _, f := range m.Files {
		if f.Role == "house" {
			b = append(b, f.Key)
		}
	}
	sort.Strings(a)
	sort.Strings(b)
	if strings.Join(a, ",") != strings.Join(b, ",") {
		t.Errorf("houses of the tab %v, houses of the Command Center %v", a, b)
	}
	if m.HouseAlias["mikarrol"] != "mikkarol" || len(m.HouseAlias) != 1 {
		t.Errorf("house alias table %v: want exactly mikarrol -> mikkarol", m.HouseAlias)
	}
	if _, err := os.Stat("../../docs/art/landsraad/houses/mikarrol.png"); err != nil {
		t.Errorf("the reference picture is spelled mikarrol: %v", err)
	}
	if _, err := os.Stat("../../docs/art/landsraad/houses/mikkarol.png"); err == nil {
		t.Errorf("the reference folder now has mikkarol.png: the alias note is out of date")
	}
}

// The decree pictures: only ids of the editor's own decree table are matched, the one the README names but the folder lacks is recorded as missing, and the neutral picture is a real file.
func TestLccDecreeArtIsMatchedToKnownIdsOnly(t *testing.T) {
	m := readLccManifest(t)
	raw, err := os.ReadFile("../ops/decree-names.json")
	if err != nil {
		t.Fatal(err)
	}
	var names struct {
		Decrees map[string]json.RawMessage `json:"decrees"`
	}
	if err := json.Unmarshal(raw, &names); err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, f := range m.Files {
		if f.Role == "decree" {
			have[f.Key] = true
		}
	}
	for id, d := range m.DecreeArt {
		if _, ok := names.Decrees[id]; !ok {
			t.Errorf("decree art is matched to %q, which is not in the editor's decree table", id)
		}
		if !have[d.File] {
			t.Errorf("decree %s is matched to %q, which is not embedded", id, d.File)
		}
		if d.Basis != "name" && d.Basis != "subject" {
			t.Errorf("decree %s: basis %q", id, d.Basis)
		}
	}
	if _, ok := m.DecreeArt["SpecialVendorActive"]; ok {
		t.Error("the plain SpecialVendorActive row has no picture of its own and must use the neutral one")
	}
	if len(m.DecreeMissing) != 1 || m.DecreeMissing[0] != "vendor-special" {
		t.Errorf("missing decree pictures %v", m.DecreeMissing)
	}
	if _, err := os.Stat("../../docs/art/landsraad/decrees/vendor-special.png"); err == nil {
		t.Error("docs/art/landsraad/decrees/vendor-special.png now exists: map it and update the manifest")
	}
	for _, f := range m.DecreeNotRuntime {
		if have[f] {
			t.Errorf("%s is listed as not used at run time but is embedded", f)
		}
	}
	svg, err := static.ReadFile("static/lcc/art/decrees/fallback.svg")
	if err != nil || !bytes.HasPrefix(svg, []byte("<svg")) || bytes.Contains(svg, []byte("<script")) || bytes.Contains(svg, []byte("href")) || bytes.Contains(svg, []byte("<text")) {
		t.Errorf("the neutral decree picture must be a plain svg with no script, link or text: %v", err)
	}
}

// The art is served from a fixed list: each file with its own type and bytes; no name, case, path or extension outside the list is a picture; the missing decree picture is not a 404 for the
// page (the helper never asks for it) and not a picture for a request either.
func TestLccArtIsServedFromAFixedListOnly(t *testing.T) {
	s := newTestServer(t)
	get := func(p string) (int, string, []byte) {
		w := do(t, s, "GET", "http://127.0.0.1"+p, loop, map[string]string{"Host": "127.0.0.1:8090"}, nil)
		return w.Code, w.Header().Get("Content-Type"), w.Body.Bytes()
	}
	m := readLccManifest(t)
	for _, f := range m.Files {
		code, ct, body := get("/lcc/art/" + f.File)
		wantCT := "image/webp"
		if strings.HasSuffix(f.File, ".svg") {
			wantCT = "image/svg+xml"
		}
		if strings.HasSuffix(f.File, ".png") {
			wantCT = "image/png"
		}
		want, _ := static.ReadFile("static/lcc/art/" + f.File)
		if code != http.StatusOK || ct != wantCT || !bytes.Equal(body, want) {
			t.Errorf("/lcc/art/%s: %d %q, %d of %d bytes", f.File, code, ct, len(body), len(want))
		}
	}
	for p, ct := range map[string]string{"/lcc/lcc.css": "text/css", "/lcc/lcc.js": "text/javascript", "/lcc/lcc-model.js": "text/javascript", "/lcc/lcc-board.css": "text/css", "/lcc/lcc-board.js": "text/javascript", "/lcc/lcc-spec.css": "text/css", "/lcc/lcc-spec.js": "text/javascript", "/lcc/lcc-term.css": "text/css", "/lcc/lcc-term.js": "text/javascript"} {
		if code, got, _ := get(p); code != http.StatusOK || !strings.HasPrefix(got, ct) {
			t.Errorf("%s: %d %q", p, code, got)
		}
	}
	for _, bad := range []string{
		"/lcc/art/houses/nope.webp", "/lcc/art/houses/ECAZ.webp", "/lcc/art/houses/Ecaz.webp", "/lcc/art/houses/mikarrol.webp", "/lcc/art/houses/ecaz.png", "/lcc/art/houses/ecaz.webp/", "/lcc/art/houses/",
		"/lcc/art/houses/../../../app.js", "/lcc/art/houses/%2e%2e/%2e%2e/app.js", "/lcc/art/houses/..%2f..%2fapp.js", "/lcc/art/houses/%2e%2e%2fapp.js", "/lcc/art/../lcc.js/../../app.js",
		"/lcc/art/decrees/vendor-special.webp", "/lcc/art/decrees/SpecialVendorActive.webp", "/lcc/art/lanes/Combat.webp", "/lcc/art/banners/combat.png", "/lcc/art/manifest.json", "/lcc/art/decrees/fallback.svg.bak",
		"/lcc/gallery.html", "/lcc/gallery/index.html", "/lcc/gallery.js", "/lcc/", "/lcc",
	} {
		if code, ct, _ := get(bad); code == http.StatusOK && (strings.HasPrefix(ct, "image/") || strings.HasPrefix(ct, "text/javascript") || strings.HasPrefix(ct, "text/html")) {
			t.Errorf("%s must not be served: %d %q", bad, code, ct)
		}
	}
}

// What the files may contain: nothing that reaches the network, writes the save or keeps data. The preview page is not part of the editor: it is not embedded, not routed and not linked from
// the page, and none of its demo values appear in the helpers, the styles or the page script.
func TestLccFilesReachNothingAndTheGalleryStaysOutOfTheEditor(t *testing.T) {
	js, css := read(t, "static/lcc/lcc.js"), read(t, "static/lcc/lcc.css")
	// the files of the board (M3) follow the same rules: they draw and compute, they never reach the network, keep data or write markup
	for name, src := range map[string]string{"lcc.js": js, "lcc.css": css, "lcc-model.js": read(t, "static/lcc/lcc-model.js"), "lcc-board.js": read(t, "static/lcc/lcc-board.js"), "lcc-board.css": read(t, "static/lcc/lcc-board.css"), "lcc-spec.js": read(t, "static/lcc/lcc-spec.js"), "lcc-spec.css": read(t, "static/lcc/lcc-spec.css"), "lcc-term.js": read(t, "static/lcc/lcc-term.js"), "lcc-term.css": read(t, "static/lcc/lcc-term.css")} {
		for _, bad := range []string{"fetch(", "XMLHttpRequest", "WebSocket", "sendBeacon", "EventSource", "/api/", "localStorage", "sessionStorage", "document.cookie", "eval(", "new Function", "innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "importScripts", "@import", "data:", "javascript:"} {
			if strings.Contains(src, bad) {
				t.Errorf("%s contains %q", name, bad)
			}
		}
		// the only address in the files is the svg namespace; any other scheme is a network reference
		for _, u := range regexp.MustCompile(`[a-z]+://[^\s'")]+`).FindAllString(src, -1) {
			if u != "http://www.w3.org/2000/svg" {
				t.Errorf("%s names the address %s", name, u)
			}
		}
	}
	if strings.Contains(css, "url(") || strings.Contains(read(t, "static/lcc/lcc-board.css"), "url(") || strings.Contains(read(t, "static/lcc/lcc-spec.css"), "url(") || strings.Contains(read(t, "static/lcc/lcc-term.css"), "url(") {
		t.Error("the Command Center styles load a picture by address: the art is an <img> from the fixed list, so the styles need no url()")
	}
	// no demo value in the helpers or the styles
	for _, demo := range []string{"Fixture", "CHOAM Refining Contract", "Verthovan", "LetoBaszion", "Plebs of the Square", "Warawatwan", "Salt Wound", "Spice Boys"} {
		if strings.Contains(js, demo) {
			t.Errorf("lcc.js holds the demo value %q", demo)
		}
		if strings.Contains(css, demo) {
			t.Errorf("lcc.css holds the demo value %q", demo)
		}
	}
	// the gallery: outside the embedded folder, outside the routes, outside the page
	err := fs.WalkDir(static, "static", func(p string, d fs.DirEntry, err error) error {
		if err == nil && strings.Contains(strings.ToLower(p), "gallery") {
			t.Errorf("%s: the preview gallery must not be embedded in the editor", p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	// The editor's page links the board's five files (index.html) and app.js talks to them only through the names they publish (LCC for the art and the plaque, LCCM for the reader of the
	// answer, LCCB for the board, LCCS for the Specializations view, LCCT for the Term and Decrees section) and the one object it hands them (LCC_HOST). It never builds a file name of the art, and no other file of the page mentions them.
	idx := read(t, "static/index.html")
	for _, want := range []string{`href="/lcc/lcc.css"`, `href="/lcc/lcc-board.css"`, `src="/lcc/lcc.js"`, `src="/lcc/lcc-model.js"`, `src="/lcc/lcc-board.js"`, `href="/lcc/lcc-spec.css"`, `src="/lcc/lcc-spec.js"`, `href="/lcc/lcc-term.css"`, `src="/lcc/lcc-term.js"`} {
		if !strings.Contains(idx, want) {
			t.Errorf("static/index.html does not link %s", want)
		}
	}
	for _, f := range []string{"static/index.html", "static/app.js", "static/html.js", "static/livemap.js", "static/theme-menu.js", "static/look-boot.js", "static/motion-boot.js", "static/app.css"} {
		src := read(t, f)
		if strings.Contains(src, "gallery") {
			t.Errorf("%s mentions the preview gallery: it is not part of the editor", f)
		}
		if f == "static/index.html" || f == "static/app.js" {
			continue
		}
		for _, bad := range []string{"LCC", "lcc/lcc"} {
			if strings.Contains(src, bad) {
				t.Errorf("%s mentions %q: only the page script and the page's own file list use the Command Center parts", f, bad)
			}
		}
	}
	app := read(t, "static/app.js")
	for _, bad := range []string{"LCC.", "lcc/art", "houseAsset", "gallery"} {
		if strings.Contains(app, bad) {
			t.Errorf("static/app.js reaches into the Command Center helpers with %q: it uses only LCCM, LCCB, LCCS, LCCT and LCC_HOST", bad)
		}
	}
	// a request for the gallery files gets no page
	s := newTestServer(t)
	for _, p := range []string{"/docs/design/landsraad-command-center/gallery/index.html", "/gallery/index.html", "/gallery", "/gallery.js"} {
		if w := do(t, s, "GET", "http://127.0.0.1"+p, loop, map[string]string{"Host": "127.0.0.1:8090"}, nil); w.Code == http.StatusOK {
			t.Errorf("%s answers 200", p)
		}
	}
}

// The preview page follows the same rules as the editor's page: no inline script, style or handler, no address that is not a file of this repository, and a content security policy that is
// not wider than the editor's (default-src none, everything else from this origin only).
func TestLccGalleryPageIsAFixtureAndSelfContained(t *testing.T) {
	dir := "../../docs/design/landsraad-command-center/gallery"
	idxB, err := os.ReadFile(dir + "/index.html")
	if err != nil {
		t.Fatal(err)
	}
	idx := string(idxB)
	if !strings.Contains(idx, "FIXTURE") || !strings.Contains(strings.ToLower(idx), "not part of the editor") {
		t.Error("the gallery page must say at the top that it is fixture data and not the editor")
	}
	csp := regexp.MustCompile(`http-equiv="Content-Security-Policy" content="([^"]+)"`).FindStringSubmatch(idx)
	if csp == nil {
		t.Fatal("the gallery page carries no content security policy")
	}
	for _, want := range []string{"default-src 'none'", "script-src 'self'", "style-src 'self'", "img-src 'self'", "font-src 'self'"} {
		if !strings.Contains(csp[1], want) {
			t.Errorf("gallery policy %q lacks %s", csp[1], want)
		}
	}
	for _, wide := range []string{"unsafe-", "http:", "https:", "data:", "blob:", "connect-src", "*"} {
		if strings.Contains(csp[1], wide) {
			t.Errorf("gallery policy %q is wider than the editor's (%s)", csp[1], wide)
		}
	}
	if srv := read(t, "server.go"); !strings.Contains(srv, "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self'; connect-src 'self'; font-src 'self'") {
		t.Error("the editor policy changed: update the gallery policy to match")
	}
	for _, bad := range []string{"<style", " style=", "<script>", "onclick=", "onsubmit=", "onload=", "onerror=", "javascript:"} {
		if strings.Contains(idx, bad) {
			t.Errorf("gallery index.html contains %q", bad)
		}
	}
	for _, m := range regexp.MustCompile(`(?:src|href)="([^"]+)"`).FindAllStringSubmatch(idx, -1) {
		ref := m[1]
		if strings.Contains(ref, "://") || strings.HasPrefix(ref, "//") || strings.HasPrefix(ref, "/") {
			t.Errorf("gallery index.html loads %q: only relative files of this repository", ref)
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(path.Clean(ref)))); err != nil {
			t.Errorf("gallery index.html names %q, which does not exist: %v", ref, err)
		}
	}
	for _, f := range []string{"gallery.js", "gallery.css"} {
		b, err := os.ReadFile(dir + "/" + f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		for _, bad := range []string{"fetch(", "XMLHttpRequest", "WebSocket", "sendBeacon", "/api/", "localStorage", "innerHTML", "eval(", "http://", "https://", "url(", "@import"} {
			if strings.Contains(src, bad) {
				t.Errorf("%s contains %q", f, bad)
			}
		}
	}
	gj, _ := os.ReadFile(dir + "/gallery.js")
	if !strings.Contains(string(gj), "FIXTURE / DEMO ONLY") {
		t.Error("gallery.js must start with the fixture notice")
	}
}
