package web

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The Motion setting's file is served like the other scripts of the page (#571): from the editor's own origin, with its type, nosniff, the loopback and Host rules, and no token in it.
func TestTheMotionScriptIsServedLikeTheOtherScripts(t *testing.T) {
	s := newTestServer(t)
	hdr := map[string]string{"Host": "127.0.0.1:8090"}
	w := do(t, s, "GET", "http://127.0.0.1/motion-boot.js", loop, hdr, nil)
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/javascript") || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("/motion-boot.js: code %d, type %q", w.Code, w.Header().Get("Content-Type"))
	}
	if strings.Contains(w.Body.String(), s.token) {
		t.Error("/motion-boot.js contains the token")
	}
	if w := do(t, s, "GET", "http://127.0.0.1/motion-boot.js", "192.168.1.5:1", hdr, nil); w.Code != 403 {
		t.Errorf("the script is still subject to the loopback-peer rule: %d", w.Code)
	}
	if w := do(t, s, "GET", "http://127.0.0.1/motion-boot.js", loop, map[string]string{"Host": "evil.example"}, nil); w.Code != 403 {
		t.Errorf("the script is still subject to the Host rule: %d", w.Code)
	}
}

// The setting is read before the first paint, so its file is in the head, before the stylesheets and before every other script, and it is an external file (the policy has no inline script).
func TestTheMotionScriptIsInTheHeadBeforeTheStylesheets(t *testing.T) {
	b, err := os.ReadFile("static/index.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	head := page[:strings.Index(page, "</head>")]
	script := strings.Index(head, `<script src="/motion-boot.js"></script>`)
	if script < 0 {
		t.Fatal("the head does not load /motion-boot.js")
	}
	if css := strings.Index(head, `rel="stylesheet"`); css < script {
		t.Error("the Motion script must come before the first stylesheet")
	}
	if strings.Index(page, `src="/app.js"`) < strings.Index(page, "</head>") {
		t.Error("app.js must come after the head")
	}
	if !strings.Contains(page, `id="theme-host"`) || !strings.Contains(page, `id="savedmark"`) || !strings.Contains(page, `id="toast" class="d-toasts"`) {
		t.Error("the page lost the Theme menu host, the written mark or the toast region")
	}
}

// What the script writes on the page and keeps in storage is the one contract with the module's motion.css: these names are read there.
func TestTheMotionScriptKeepsToItsContract(t *testing.T) {
	b, err := os.ReadFile("static/motion-boot.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{"'tauMotion'", "['full', 'calm', 'off']", "'data-motion'", "'data-motion-src'", "'data-doc-hidden'"} {
		if !strings.Contains(src, want) {
			t.Errorf("motion-boot.js lacks %s", want)
		}
	}
	for _, bad := range []string{"eval(", "new Function", "innerHTML", "document.write", "XMLHttpRequest", "fetch("} {
		if strings.Contains(src, bad) {
			t.Errorf("motion-boot.js contains %q", bad)
		}
	}
	// every touch of the storage is inside a try
	for _, m := range regexp.MustCompile(`localStorage\.(get|set)Item`).FindAllStringIndex(src, -1) {
		if !strings.Contains(src[max(0, m[0]-120):m[0]], "try {") {
			t.Errorf("a storage call near %q is not in a try", src[m[0]:m[1]])
		}
	}
}

// What the editor adds to the module's motion is small and stops by itself: no loop (nothing repeats), only keyframes of its own and the module's draw, and no rule that would keep a
// animation going where the module has switched it off (Calm and Off cut the module's own animations; the editor's bump is only played in Full by the script).
func TestTheEditorsOwnAnimationsAreOneShotsAndNeverLoop(t *testing.T) {
	b, err := os.ReadFile("static/app.css")
	if err != nil {
		t.Fatal(err)
	}
	css := strings.ReplaceAll(string(b), "\r", "")
	if strings.Contains(css, "infinite") {
		t.Error("app.css has a loop")
	}
	if strings.Contains(css, "animation-play-state") || strings.Contains(css, "animation:none!important") {
		t.Error("app.css must not take over the module's control of animations")
	}
	allowed := map[string]bool{"x-bump": true, "d-draw": true}
	for _, m := range regexp.MustCompile(`animation:\s*([a-z0-9-]+)`).FindAllStringSubmatch(css, -1) {
		if !allowed[m[1]] {
			t.Errorf("app.css animates with %q, which is not one of its own one-shots or the module's draw", m[1])
		}
	}
	for _, m := range regexp.MustCompile(`@keyframes\s+([a-z0-9-]+)`).FindAllStringSubmatch(css, -1) {
		if m[1] != "x-bump" {
			t.Errorf("app.css defines the keyframes %q: only x-bump is its own", m[1])
		}
	}
	// the durations are the module's tokens, which Calm and Off re-define: no number of milliseconds of its own
	if regexp.MustCompile(`(animation|transition)[^;}]*[0-9]ms`).MatchString(css) || regexp.MustCompile(`(animation|transition)[^;}]*[0-9.]s\b`).MatchString(css) {
		t.Error("app.css gives an animation or transition a duration of its own: use the --mo-* tokens")
	}
}

// The page reaches the setting only as window.TauMotion (a page where the script did not run has none, and the page must still work), and only /motion-boot.js keeps it.
func TestThePageReachesTheMotionObjectOnlyAsAWindowProperty(t *testing.T) {
	b, err := os.ReadFile("static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for i := strings.Index(src, "TauMotion"); i >= 0; {
		if !strings.HasSuffix(src[:i], "window.") {
			t.Errorf("app.js reaches TauMotion other than as window.TauMotion near %q", src[max(0, i-20):i+9])
		}
		next := strings.Index(src[i+1:], "TauMotion")
		if next < 0 {
			break
		}
		i += 1 + next
	}
	if strings.Contains(src, "'tauMotion'") {
		t.Error("only /motion-boot.js keeps the setting")
	}
}
