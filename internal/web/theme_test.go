package web

import (
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/design"
)

// The Theme menu (#570, slice 3): the two scripts are served like every other script of the page, and what they hold is the design module's own table of looks.
func TestTheThemeScriptsAreServedLikeTheOtherScripts(t *testing.T) {
	s := newTestServer(t)
	hdr := map[string]string{"Host": "127.0.0.1:8090"}
	for _, p := range []string{"/look-boot.js", "/theme-menu.js"} {
		w := do(t, s, "GET", "http://127.0.0.1"+p, loop, hdr, nil)
		if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/javascript") || w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: code %d, type %q", p, w.Code, w.Header().Get("Content-Type"))
		}
		if strings.Contains(w.Body.String(), s.token) {
			t.Errorf("%s contains the token", p)
		}
		if w := do(t, s, "GET", "http://127.0.0.1"+p, "192.168.1.5:1", hdr, nil); w.Code != 403 {
			t.Errorf("%s is still subject to the loopback-peer rule: %d", p, w.Code)
		}
		if w := do(t, s, "GET", "http://127.0.0.1"+p, loop, map[string]string{"Host": "evil.example"}, nil); w.Code != 403 {
			t.Errorf("%s is still subject to the Host rule: %d", p, w.Code)
		}
	}
}

// The file names a switch may load are a fixed table in the script. It is the module's own: for every look the module has, the table names the module's tokens file and components file of that
// look (design.LookFiles), those files exist and the editor serves them, and the list of looks is the module's list in the module's order. A stored string only ever picks an entry.
func TestTheLookTableIsTheDesignModulesAndNamesOnlyServedFiles(t *testing.T) {
	b, err := os.ReadFile("static/look-boot.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	s := newTestServer(t)
	var quoted []string
	for _, look := range design.Looks() {
		files, ok := design.LookFiles(look)
		if !ok || len(files) < 2 {
			t.Fatalf("the module has no files for %s", look)
		}
		quoted = append(quoted, "'"+look+"'")
		want := look + ": ['/design/" + files[0] + "', '/design/" + files[1] + "'],"
		if !strings.Contains(src, want) {
			t.Errorf("look-boot.js does not hold the module's files for %s: want %s", look, want)
		}
		for _, f := range files[:2] {
			if _, err := fs.Stat(design.Assets, f); err != nil {
				t.Errorf("%s: %v", f, err)
			}
			if w := do(t, s, "GET", "http://127.0.0.1/design/"+f, loop, map[string]string{"Host": "127.0.0.1:8090"}, nil); w.Code != 200 {
				t.Errorf("/design/%s is not served: %d", f, w.Code)
			}
		}
	}
	if want := "const LOOKS = [" + strings.Join(quoted, ", ") + "];"; !strings.Contains(src, want) {
		t.Errorf("the list of looks in look-boot.js is not the module's: want %s", want)
	}
	if want := "const DEFAULT_LOOK = '" + design.DefaultLook + "';"; !strings.Contains(src, want) {
		t.Errorf("the default look is not the module's: want %s", want)
	}
	// the files the page links to begin with are the default look's
	index, err := os.ReadFile("static/index.html")
	if err != nil {
		t.Fatal(err)
	}
	files, _ := design.LookFiles(design.DefaultLook)
	for _, f := range files {
		if !strings.Contains(string(index), `href="/design/`+f+`"`) {
			t.Errorf("index.html does not link the default look's %s", f)
		}
	}
}

// Order in the head: the Motion setting first (it is read before anything is drawn), the look's script after the stylesheets it swaps (it finds the links there), and on both pages; the
// Theme menu's script and the page's come after the body.
func TestTheHeadReadsTheChoicesInTheRightOrder(t *testing.T) {
	for _, f := range []string{"static/index.html", "static/locked.html"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		page := string(b)
		end := strings.Index(page, "</head>")
		head := page[:end]
		look := strings.Index(head, `<script src="/look-boot.js"></script>`)
		if look < 0 {
			t.Errorf("%s: the head does not load /look-boot.js", f)
			continue
		}
		if app := strings.Index(head, `href="/app.css"`); app < 0 || look < app {
			t.Errorf("%s: /look-boot.js must come after the last stylesheet", f)
		}
		if tokens := strings.Index(head, `href="/design/tokens-`); tokens < 0 || look < tokens {
			t.Errorf("%s: /look-boot.js must come after the look's stylesheets", f)
		}
		if f == "static/index.html" {
			if m := strings.Index(head, `<script src="/motion-boot.js"></script>`); m < 0 || m > strings.Index(head, `rel="stylesheet"`) {
				t.Errorf("%s: the Motion script must come before the first stylesheet", f)
			}
			menu := strings.Index(page, `<script src="/theme-menu.js"></script>`)
			if menu < end || !strings.Contains(page, `id="theme-host"`) {
				t.Errorf("%s: the Theme menu's script must be after the head, and the host must be in the header", f)
			}
		}
	}
}

// The scripts keep to the rules of a script under this policy: no code from a string, no markup from a string, no network, every storage touch in a try, the choices read only against fixed lists,
// and each file's keys are its own.
func TestTheThemeScriptsKeepToTheRules(t *testing.T) {
	for _, f := range []string{"static/look-boot.js", "static/theme-menu.js"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		for _, bad := range []string{"eval(", "new Function", "innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "XMLHttpRequest", "fetch(", "setAttribute('style'", `setAttribute("style"`, "javascript:", "setAttribute('on"} {
			if strings.Contains(src, bad) {
				t.Errorf("%s contains %q", f, bad)
			}
		}
		for _, m := range regexp.MustCompile(`localStorage\.(get|set|remove)Item`).FindAllStringIndex(src, -1) {
			if !strings.Contains(src[max(0, m[0]-120):m[0]], "try {") {
				t.Errorf("%s: a storage call near %q is not in a try", f, src[m[0]:m[1]])
			}
		}
	}
	b, err := os.ReadFile("static/look-boot.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"'tauLook'", "'tauTheme'", "'tauFont'", "list.indexOf(v) >= 0 ? v : null", "'data-look'", "'data-theme'", "'data-heading-font'"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("look-boot.js lacks %s", want)
		}
	}
}

// The menu's own look uses the module's colours only; its text and the dot that marks the chosen item reach their ratios on the raised surface in every look and both themes.
func TestTheThemeMenuIsReadableInEveryLookAndTheme(t *testing.T) {
	b, err := os.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	css := strings.ReplaceAll(string(b), "\r", "")
	i := strings.Index(css, ".theme-host{")
	if i < 0 {
		t.Fatal("app.css has no Theme menu rules")
	}
	menu := css[i:]
	if regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`).MatchString(menu) {
		t.Error("the Theme menu rules use a colour of their own: use the module's tokens")
	}
	for _, look := range design.Looks() {
		dark, light := lookTokens(t, look)
		for name, tok := range map[string]map[string]string{"dark": dark, "Dayside": light} {
			for _, c := range []struct {
				fg, bg string
				min    float64
				what   string
			}{
				{"--c-text", "--c-raised", 4.5, "item text"},
				{"--c-dim", "--c-raised", 4.5, "group title"},
				{"--c-spice", "--c-raised", 3, "the dot of the chosen item"},
				{"--c-rule", "--c-raised", 3, "the ring of an item"},
				{"--c-focus", "--c-raised", 3, "the focus ring"},
			} {
				if tok[c.fg] == "" || tok[c.bg] == "" {
					t.Fatalf("%s/%s: %s or %s is not defined", look, name, c.fg, c.bg)
				}
				if r := contrast(tok[c.fg], tok[c.bg]); r < c.min {
					t.Errorf("%s/%s: %s (%s on %s) is %.2f:1, needs %.1f:1", look, name, c.what, tok[c.fg], tok[c.bg], r, c.min)
				}
			}
		}
	}
}
