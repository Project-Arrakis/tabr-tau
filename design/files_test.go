package design

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
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

// Checks of the files of the module: what is in the package, how it is served, what it refers to, the font, and what must not be in it (design review
// register, checks 2, 3, 4, 5 and 9; public issues 572, 573 and 574).

// diskFiles lists every file of the module's directory (the tests run there), as slash paths relative to it, without .git.
func diskFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "coverage.out" || d.Name() == "go.work" || d.Name() == "go.work.sum" {
			return nil // made by a test run or by a developer, never part of the module (they are in .gitignore)
		}
		out = append(out, filepath.ToSlash(p))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}

func withExt(files []string, exts ...string) []string {
	var out []string
	for _, f := range files {
		for _, e := range exts {
			if strings.HasSuffix(f, e) {
				out = append(out, f)
			}
		}
	}
	return out
}

// ---- what is in the package, and how it is served ----

func TestAssetsHoldTheFilesOfVersion030(t *testing.T) {
	got := assetNames(t, "")
	want := []string{
		"components-awakening.css", "components-fremen.css", "components-sietch.css", "components.css", "fonts/DuneRise-OFL.txt", "fonts/SHA256SUMS",
		"grain.svg", "icons.svg", "motion.css", "palettes.css", "tokens-awakening.css", "tokens-fremen.css", "tokens-sietch.css", "tokens.css",
	}
	if _, err := os.Stat("assets/fonts/DuneRise.ttf"); err == nil {
		want = append(want, "fonts/DuneRise.ttf")
	}
	sort.Strings(want)
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Assets holds\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// Every file is sent with the type of its extension, whatever the host's table says, and with nosniff; an extension the table does not know is a failure
// of this test, so a new kind of file cannot be added without its type.
func TestHandlerSendsTheRightTypeForEveryFile(t *testing.T) {
	h := http.StripPrefix("/design/", Handler())
	want := map[string]string{".css": "text/css; charset=utf-8", ".svg": "image/svg+xml", ".ttf": "font/ttf", ".txt": "text/plain; charset=utf-8"}
	n := 0
	for _, f := range assetNames(t, "") {
		n++
		w := get(h, "GET", "/design/"+f)
		if w.Code != 200 {
			t.Errorf("GET %s: %d", f, w.Code)
			continue
		}
		wt, ok := want[path.Ext(f)]
		if path.Base(f) == "SHA256SUMS" {
			wt, ok = "text/plain; charset=utf-8", true
		}
		if !ok {
			t.Errorf("%s has an extension this test does not know; add its type to Handler and to this test", f)
			continue
		}
		if ct := w.Header().Get("Content-Type"); ct != wt {
			t.Errorf("%s is sent as %q, want %q", f, ct, wt)
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: no nosniff", f)
		}
		b, _ := fs.ReadFile(Assets, f)
		if w.Body.String() != string(b) {
			t.Errorf("%s: the body is not the embedded file", f)
		}
	}
	if n < 12 {
		t.Errorf("walked only %d files", n)
	}
	if ct := contentType("a/b.woff2"); ct != "application/octet-stream" {
		t.Errorf("an unknown extension is sent as %q", ct)
	}
	for _, p := range []string{"/design/fonts/", "/design/fonts", "/design/../go.mod"} {
		if w := get(h, "GET", p); w.Code == 200 {
			t.Errorf("GET %s answered 200", p)
		}
	}
}

// ---- no outside reference, no data: URL, no inline style or script, no HTML from strings (checks 2 and 5) ----

var cssURLRE = regexp.MustCompile(`url\(\s*(['"]?)([^'")]*)['"]?\s*\)`)
var htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)
var lineComment = regexp.MustCompile(`(?m)(^|[^:])//[^\n]*`)

// code returns a file's text with comments removed, so that a sentence in a comment is not read as code.
func code(t *testing.T, name string) string {
	t.Helper()
	s := readFile(t, name)
	switch {
	case strings.HasSuffix(name, ".css"):
		return blockComment.ReplaceAllString(s, "")
	case strings.HasSuffix(name, ".svg"), strings.HasSuffix(name, ".html"):
		return htmlComment.ReplaceAllString(s, "")
	case strings.HasSuffix(name, ".js"):
		return lineComment.ReplaceAllString(blockComment.ReplaceAllString(s, ""), "$1")
	}
	return s
}

func TestNoOutsideReferenceAndNoDataURL(t *testing.T) {
	files := withExt(diskFiles(t), ".css", ".svg", ".html", ".js")
	if len(files) < 14 {
		t.Fatalf("found only %d code files", len(files))
	}
	for _, f := range files {
		c := code(t, f)
		for _, bad := range []string{"data:", "@import", "javascript:", "expression(", "blob:", "//cdn", "http://", "https://"} {
			if strings.Contains(c, bad) {
				// the XML namespace of an SVG file is the one address allowed
				if bad == "http://" && strings.HasSuffix(f, ".svg") && strings.Count(c, "http://") == strings.Count(c, `xmlns="http://www.w3.org/2000/svg"`) {
					continue
				}
				t.Errorf("%s contains %q", f, bad)
			}
		}
		if strings.HasSuffix(f, ".css") {
			for _, m := range cssURLRE.FindAllStringSubmatch(c, -1) {
				u := m[2]
				if strings.Contains(u, ":") || strings.HasPrefix(u, "/") || strings.HasPrefix(u, "#") {
					t.Errorf("%s: url(%s) is not a relative file of the module", f, u)
					continue
				}
				target := path.Join(path.Dir(f), u)
				if _, err := os.Stat(filepath.FromSlash(target)); err != nil && target != "assets/"+FontFile {
					t.Errorf("%s: url(%s) names a file that is not in the module", f, u)
				}
			}
		}
		if strings.HasSuffix(f, ".html") {
			for _, m := range regexp.MustCompile(`(?:href|src)="([^"#][^"]*)"`).FindAllStringSubmatch(c, -1) {
				if strings.Contains(m[1], ":") || strings.HasPrefix(m[1], "//") {
					t.Errorf("%s: %q is not a relative address", f, m[1])
				}
			}
		}
	}
}

// No inline style, no <style> element, no inline script and no event attribute in the kitchen sink, and no HTML built from a string in any script.
func TestNoInlineStyleOrScriptAndNoHTMLFromStrings(t *testing.T) {
	page := code(t, "kitchen-sink/index.html")
	for name, re := range map[string]*regexp.Regexp{
		"a style attribute":    regexp.MustCompile(`(?i)\sstyle\s*=`),
		"a <style> element":    regexp.MustCompile(`(?i)<style`),
		"an inline script":     regexp.MustCompile(`(?i)<script(?:\s[^>]*)?>\s*[^<\s]`),
		"a script without src": regexp.MustCompile(`(?i)<script(?:\s+(?:defer|async|type="[^"]*"))*\s*>`),
		"an event attribute":   regexp.MustCompile(`(?i)\son[a-z]+\s*=`),
		"a javascript: URL":    regexp.MustCompile(`(?i)javascript:`),
	} {
		if re.MatchString(page) {
			t.Errorf("kitchen-sink/index.html has %s", name)
		}
	}
	if !regexp.MustCompile(`<script src="kitchen-sink\.js" defer></script>`).MatchString(page) {
		t.Error("the kitchen sink does not load its script as an external file")
	}
	for _, f := range withExt(diskFiles(t), ".css", ".svg") {
		if regexp.MustCompile(`(?i)<script|<foreignObject|<image\b|<style`).MatchString(code(t, f)) {
			t.Errorf("%s holds a script, a foreign object, an embedded image or a style element", f)
		}
	}
	for _, f := range withExt(diskFiles(t), ".js") {
		c := code(t, f)
		for _, bad := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "eval(", "new Function", "setAttribute('style'", `setAttribute("style"`, "cssText", "createContextualFragment", "srcdoc", "localStorage", "XMLHttpRequest", "fetch("} {
			if strings.Contains(c, bad) {
				t.Errorf("%s uses %s", f, bad)
			}
		}
		if regexp.MustCompile(`\.style\.[a-zA-Z]+\s*=`).MatchString(c) {
			t.Errorf("%s assigns a style property; use style.setProperty", f)
		}
	}
	for _, f := range withExt(diskFiles(t), ".css") {
		if f != "kitchen-sink/kitchen-sink.css" && !strings.HasPrefix(f, "assets/") {
			t.Errorf("unexpected stylesheet %s", f)
		}
	}
}

// ---- the sprite and the grain ----

type svgSymbol struct {
	ID      string `xml:"id,attr"`
	ViewBox string `xml:"viewBox,attr"`
}

type svgDoc struct {
	XMLName xml.Name
	Symbols []svgSymbol `xml:"defs>symbol"`
}

func parseSVG(t *testing.T, name string) svgDoc {
	t.Helper()
	b, err := fs.ReadFile(Assets, name)
	if err != nil {
		t.Fatal(err)
	}
	dec := xml.NewDecoder(strings.NewReader(string(b)))
	dec.Strict = true
	var d svgDoc
	if err := dec.Decode(&d); err != nil {
		t.Fatalf("%s is not well-formed SVG: %v", name, err)
	}
	if d.XMLName.Local != "svg" || d.XMLName.Space != "http://www.w3.org/2000/svg" {
		t.Errorf("%s: root is %v", name, d.XMLName)
	}
	return d
}

func TestIconSprite(t *testing.T) {
	d := parseSVG(t, IconsFile)
	ids := map[string]bool{}
	for _, s := range d.Symbols {
		if !strings.HasPrefix(s.ID, "i-") || s.ViewBox != "0 0 24 24" {
			t.Errorf("symbol %q (viewBox %q): an icon is i-NAME on a 24 px grid", s.ID, s.ViewBox)
		}
		if ids[s.ID] {
			t.Errorf("symbol %s is defined twice", s.ID)
		}
		ids[s.ID] = true
	}
	for _, must := range []string{"i-drop", "i-fuel", "i-wrench", "i-mark", "i-check", "i-warn", "i-info", "i-x", "i-fx-water", "i-fx-can", "i-fx-shield", "i-fx-check", "i-chev", "i-search"} {
		if !ids[must] {
			t.Errorf("the sprite lacks %s", must)
		}
	}
	if len(ids) < 50 {
		t.Errorf("the sprite has %d icons", len(ids))
	}
	// the sprite uses one colour, the current text colour
	raw := readFile(t, "assets/icons.svg")
	if regexp.MustCompile(`(fill|stroke)="#|rgb\(`).MatchString(raw) {
		t.Error("an icon names a colour; icons take currentColor")
	}
	g := parseSVG(t, GrainFile)
	if len(g.Symbols) != 0 {
		t.Error("the grain is a picture, not a sprite")
	}
}

// ---- the font (check 3, public issue 573) ----

func TestFontLicenceAndChecksum(t *testing.T) {
	lic := readFile(t, "assets/"+FontLicence)
	for _, must := range []string{"Copyright (c) 2021, Fontswan", "Reserved Font Name Dune Rise", "SIL OPEN FONT LICENSE Version 1.1", "PERMISSION & CONDITIONS", "DISCLAIMER"} {
		if !strings.Contains(lic, must) {
			t.Errorf("the licence file lacks %q", must)
		}
	}
	sums := readFile(t, "assets/fonts/SHA256SUMS")
	const want = "cb9c956d14b4e743658904484d2e7e7bd6621d16faaafe7d0e47c022f70a99f9  DuneRise.ttf\n"
	if sums != want {
		t.Errorf("SHA256SUMS is %q, want %q", sums, want)
	}
	b, err := os.ReadFile("assets/" + FontFile)
	if err != nil {
		if os.Getenv("DESIGN_RELEASE") != "" {
			t.Fatalf("a release needs the font file (DESIGN_RELEASE is set): %v", err)
		}
		t.Skip("assets/fonts/DuneRise.ttf is not in the module yet; the release workflow refuses a tag without it")
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != strings.Fields(sums)[0] {
		t.Errorf("DuneRise.ttf has sha256 %x, SHA256SUMS says %s: the file must be used unmodified", sum, strings.Fields(sums)[0])
	}
	if len(b) != 22964 {
		t.Errorf("DuneRise.ttf is %d bytes, want 22964", len(b))
	}
	// the font's own name table carries the licence (UTF-16 big-endian on the Windows platform)
	utf16 := func(s string) []byte {
		out := make([]byte, 0, 2*len(s))
		for _, c := range []byte(s) {
			out = append(out, 0, c)
		}
		return out
	}
	if !strings.Contains(string(b), "SIL Open Font License") && !strings.Contains(string(b), string(utf16("SIL Open Font License"))) {
		t.Error("the font's name table does not say SIL Open Font License")
	}
	if !strings.Contains(string(b), "Dune Rise") && !strings.Contains(string(b), string(utf16("Dune Rise"))) {
		t.Error("the font's name table does not say Dune Rise")
	}
}

// The notices carry both third-party works.
func TestNoticeCarriesTheThirdPartyWorks(t *testing.T) {
	n := readFile(t, "NOTICE")
	for _, must := range []string{
		"Copyright (c) 2021, Fontswan", "Reserved Font Name Dune Rise", "SIL Open Font License, Version 1.1", "assets/fonts/DuneRise-OFL.txt",
		"btriapitsyn/zed-fedaykin-themes", "Copyright (c) 2025 FedaykinDev", "Permission is hereby granted, free of charge", "WITHOUT WARRANTY OF ANY KIND",
		"not affiliated with or endorsed by Funcom",
	} {
		if !strings.Contains(n, must) {
			t.Errorf("NOTICE lacks %q", must)
		}
	}
	p := readFile(t, "PROVENANCE.md")
	for _, must := range []string{"DuneRise.ttf", "cb9c956d14b4e743658904484d2e7e7bd6621d16faaafe7d0e47c022f70a99f9", "ba33b46ff46000eef81676b88c8c0994eedd3a5de2545ebca20d7fe4ca02b615", "SIL Open Font License", "MIT"} {
		if !strings.Contains(p, must) {
			t.Errorf("PROVENANCE.md lacks %q", must)
		}
	}
	// every file of the module is listed in PROVENANCE.md
	for _, f := range diskFiles(t) {
		if strings.HasSuffix(f, "_test.go") || f == "go.mod" || f == "LICENSE" || f == "NOTICE" || f == "VERSION" || f == "CHANGELOG.md" || f == "PROVENANCE.md" || f == "README.md" || f == "design.go" || f == "doc.go" {
			continue
		}
		if !strings.Contains(p, "`"+f+"`") && !strings.Contains(p, "`"+strings.TrimPrefix(f, "assets/")+"`") && !strings.Contains(p, "`"+path.Base(f)+"`") {
			t.Errorf("PROVENANCE.md does not list %s", f)
		}
	}
}

// ---- public safety (check 9, public issue 573) ----

var imageExts = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".bmp", ".tif", ".tiff", ".avif", ".heic", ".psd", ".dds", ".tga", ".svgz"}

// A path under a personal profile; the other reserved words and phrases are digests, so this file does not spell them.
var privateWords = regexp.MustCompile(`(?i)c:\\users|/users/|/home/`)

// reservedDigests are SHA-256 digests of lower-case reserved words and phrases (up to four words in a row, joined by one space). A report names the
// file and never the text.
var reservedDigests = map[string]bool{
	"06ea17a8981d25b0315ff17bd1ebad3d95c15a3f8b764eaadb93612d9b61a54e": true,
	"06e55b633481f7bb072957eabcf110c972e86691c3cfedabe088024bffe42f23": true,
	"c064fbca9d9de8dd9bb0624984403b28d0da807a69365d4f7fb09123ecb0c405": true,
	"c989edf18246e9c8cdd647369a4835f7dd9a0f47b8e755d37b16431fc5035981": true,
	"f7c797b02e33b99c0c31fe90c26d58c36eb57b99c068028b8c67b3908c5b1e01": true,
	"c857d09db23e6822e3600bc06ad8d58f92ed62bc8efd81c753f77048662cb97d": true,
	"da182ddba7f3b92d5780d543c44d2bcb0e400735486af73221f23b19c2a1f0e8": true,
	"9d68e857a23bf30a7be9591e75755e05c353db12dbf6cc34bfb69ee72a559b80": true,
	"56ab30fa0c6104ade9296f88f9c5e44c145a1bf9d8d9403f59a1b26adebd97af": true,
	"20cabea1f0d3d419d6045737ccc85dcc54f962b536b5fe6f4b2667f4422a4d49": true,
	"7764be7a6b274539c3d5faebe17914b87712f3c25e365f343f9136640f4259dd": true,
}

var wordRE = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*`)

// reserved reports whether s holds a reserved word or phrase.
func reserved(s string) bool {
	words := wordRE.FindAllString(strings.ToLower(s), -1)
	for i := range words {
		for n := 1; n <= 4 && i+n <= len(words); n++ {
			sum := sha256.Sum256([]byte(strings.Join(words[i:i+n], " ")))
			if reservedDigests[hex.EncodeToString(sum[:])] {
				return true
			}
		}
	}
	return false
}

var issueRE = regexp.MustCompile(`(?:^|[\s(])#(\d{2,5})\b`)

// issues of this repository that the module may name; any other number needs a line here, put there on purpose.
var publicIssues = map[string]bool{"553": true, "572": true, "573": true, "574": true}

func TestPublicSafety(t *testing.T) {
	files := diskFiles(t)
	for _, f := range files {
		ext := strings.ToLower(path.Ext(f))
		for _, e := range imageExts {
			if ext == e {
				t.Errorf("%s: no image file but the module's own SVG belongs in the module", f)
			}
		}
		if ext == ".woff" || ext == ".woff2" || ext == ".otf" || (ext == ".ttf" && f != "assets/"+FontFile) {
			t.Errorf("%s: the one font of the module is %s, unconverted", f, FontFile)
		}
		if strings.HasSuffix(f, "_test.go") {
			continue // the tests name the words they look for
		}
		if f == "assets/"+FontFile {
			continue
		}
		s := readFile(t, f)
		if privateWords.MatchString(s) || reserved(s) {
			t.Errorf("%s breaks the public text rule (a reserved word, phrase or profile path); the report names the file, never the text", f)
		}
		if strings.HasSuffix(f, ".css") || strings.HasSuffix(f, ".svg") {
			continue // hex colours and ids look like issue numbers
		}
		for _, m := range issueRE.FindAllStringSubmatch(s, -1) {
			if !publicIssues[m[1]] {
				t.Errorf("%s names #%s, which is not an issue the module may name", f, m[1])
			}
		}
	}
	// the module names no file by a game's own file or marker name
	for _, f := range files {
		if regexp.MustCompile(`(?i)marker|hagga|deepdesert|deep-desert`).MatchString(f) {
			t.Errorf("%s: a file name taken from the game", f)
		}
	}
}

// ---- the kitchen sink's script uses the same look files as the Go package ----

func TestKitchenSinkScriptUsesTheLookFiles(t *testing.T) {
	js := readFile(t, "kitchen-sink/kitchen-sink.js")
	for _, l := range Looks() {
		files, _ := LookFiles(l)
		want := l + ": ['" + files[0] + "', '" + files[1] + "']"
		if !strings.Contains(js, want) {
			t.Errorf("kitchen-sink.js lacks %s", want)
		}
	}
	page := readFile(t, "kitchen-sink/index.html")
	files, _ := LookFiles(DefaultLook)
	for _, f := range files {
		if !strings.Contains(page, `href="../assets/`+f+`"`) {
			t.Errorf("the kitchen sink does not link %s of the default look", f)
		}
	}
}
