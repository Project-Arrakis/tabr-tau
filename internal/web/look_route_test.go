package web

import (
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Project-Arrakis/tabr-tau/design"
)

// The design module's files are served from the editor's own origin under /design/ (#570), read-only, each with its own type, and under the same loopback and Host rules as every
// other asset. The editor's policy already allows a font from its own origin (font-src 'self'), which the heading typeface needs; nothing else is opened for it.
func TestTheDesignModuleIsServedFromTheEditorsOwnOrigin(t *testing.T) {
	s := newTestServer(t)
	hdr := map[string]string{"Host": "127.0.0.1:8090"}
	types := map[string]string{"tokens-fremen.css": "text/css", "components-fremen.css": "text/css", "components.css": "text/css", "motion.css": "text/css", "icons.svg": "image/svg+xml", "fonts/DuneRise.ttf": "font/ttf", "fonts/DuneRise-OFL.txt": "text/plain"}
	for name, ct := range types {
		want, err := fs.ReadFile(design.Assets, name)
		if err != nil {
			t.Fatal(err)
		}
		w := do(t, s, "GET", "http://127.0.0.1/design/"+name, loop, hdr, nil)
		if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), ct) || w.Body.Len() != len(want) {
			t.Errorf("/design/%s: code %d, type %q, %d of %d bytes", name, w.Code, w.Header().Get("Content-Type"), w.Body.Len(), len(want))
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("/design/%s: no nosniff", name)
		}
		if strings.Contains(w.Body.String(), s.token) {
			t.Errorf("/design/%s contains the token", name)
		}
	}
	for _, c := range []struct{ method, path string }{{"GET", "/design/"}, {"GET", "/design/fonts/"}, {"GET", "/design/nothing.css"}, {"GET", "/design/../go.mod"}, {"POST", "/design/tokens-fremen.css"}} {
		if w := do(t, s, c.method, "http://127.0.0.1"+c.path, loop, hdr, nil); w.Code == 200 {
			t.Errorf("%s %s must not be served", c.method, c.path)
		}
	}
	if w := do(t, s, "GET", "http://127.0.0.1/design/tokens-fremen.css", "192.168.1.5:1", hdr, nil); w.Code != 403 {
		t.Errorf("the design files are still subject to the loopback-peer rule: %d", w.Code)
	}
	if w := do(t, s, "GET", "http://127.0.0.1/design/tokens-fremen.css", loop, map[string]string{"Host": "evil.example"}, nil); w.Code != 403 {
		t.Errorf("the design files are still subject to the Host rule: %d", w.Code)
	}
}

// The policy that lets the typeface load is the one the editor already had: fonts from its own origin and nothing from anywhere else.
func TestThePolicyAllowsTheHeadingTypefaceFromTheEditorsOwnOriginOnly(t *testing.T) {
	s := newTestServer(t)
	_, _, h := getIndex(t, s, "", "tabr_session="+s.token)
	csp := h.Get("Content-Security-Policy")
	if !strings.Contains(csp, "font-src 'self';") {
		t.Errorf("the policy needs font-src 'self' for /design/fonts/DuneRise.ttf: %s", csp)
	}
	if strings.Contains(csp, "data:") || strings.Contains(csp, "unsafe-") {
		t.Errorf("the policy must open nothing else for the look: %s", csp)
	}
}

// No inline style or script anywhere in what the editor serves: the module's parts are classes, and a value that changes at run time is set through the CSSOM.
func TestThePagesHaveNoInlineStyleOrScript(t *testing.T) {
	for _, f := range []string{"static/index.html", "static/locked.html"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		s := strings.ToLower(string(b))
		if strings.Contains(s, "<style") || strings.Contains(s, " style=") {
			t.Errorf("%s has an inline style", f)
		}
		for _, m := range regexp.MustCompile(`<script([^>]*)>`).FindAllStringSubmatch(s, -1) {
			if !strings.Contains(m[1], "src=") {
				t.Errorf("%s has an inline script: <script%s>", f, m[1])
			}
		}
	}
}

// Every card is the module's panel beside the old class, every fixed heading of a card is the module's heading, and a heading that holds a name or a number from the save stays a plain
// heading (the typeface of the headings is for fixed words only, never for something that comes from a save, the game or a person).
func TestCardsArePanelsAndOnlyFixedHeadingsUseTheHeadingTypeface(t *testing.T) {
	for _, f := range []string{"static/app.js", "static/livemap.js", "static/locked.html"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		for _, m := range regexp.MustCompile(`class="card[^"]*"`).FindAllString(src, -1) {
			if !strings.Contains(m, "d-panel") {
				t.Errorf("%s: %s is a card without the module's d-panel", f, m)
			}
		}
		for _, m := range regexp.MustCompile(`<h3[^>]*class="[^"]*d-head[^"]*"[^>]*>([^<]*)</h3>`).FindAllStringSubmatch(src, -1) {
			if strings.Contains(m[1], "${") {
				t.Errorf("%s: a heading with data in it uses the heading typeface: %s", f, m[0])
			}
		}
	}
}
