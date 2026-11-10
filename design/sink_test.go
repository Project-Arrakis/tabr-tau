package design

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The kitchen sink is the module's own page of every component; these tests read it as text (there is no browser here).

var classAttrRE = regexp.MustCompile(`class="([^"]*)"`)
var idAttrRE = regexp.MustCompile(`\sid="([^"]*)"`)

// sinkClasses returns every class the page uses.
func sinkClasses(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, m := range classAttrRE.FindAllStringSubmatch(code(t, "kitchen-sink/index.html"), -1) {
		for _, c := range strings.Fields(m[1]) {
			out[c] = true
		}
	}
	return out
}

// componentClasses returns every class the component stylesheets name in a selector (not the keyframes), by file.
func componentClasses(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, f := range []string{"components-fremen.css", "components-awakening.css", "components-sietch.css", "components.css", "motion.css"} {
		for _, r := range readCSS(t, f) {
			if strings.HasPrefix(r.at, "@keyframes") {
				continue
			}
			for _, m := range classAttrRE2.FindAllStringSubmatch(stripAttrs(r.selector), -1) {
				out[m[1]] = true
			}
		}
	}
	return out
}

var classAttrRE2 = regexp.MustCompile(`\.([a-zA-Z][\w-]*)`)
var attrSelRE = regexp.MustCompile(`\[[^\]]*\]`)

func stripAttrs(sel string) string { return attrSelRE.ReplaceAllString(sel, "") }

// classes that a script puts on a part and that the page shows by a sample or by a button; they are named here so that a new one is a decision.
var scriptApplied = map[string]bool{}

func TestEveryClassOfTheComponentsIsInTheKitchenSink(t *testing.T) {
	used, defined := sinkClasses(t), componentClasses(t)
	var missing []string
	for c := range defined {
		if !used[c] && !scriptApplied[c] {
			missing = append(missing, c)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("classes of the components that the kitchen sink does not use: %s", strings.Join(missing, " "))
	}
	if len(defined) < 100 {
		t.Errorf("read only %d classes from the stylesheets", len(defined))
	}
}

// Every d- class the page uses is a class the stylesheets define (a typo styles nothing), and the page's own layout uses x- classes only.
func TestKitchenSinkUsesOnlyKnownClasses(t *testing.T) {
	defined := componentClasses(t)
	layout := map[string]bool{}
	for _, r := range readCSS(t, "../kitchen-sink/kitchen-sink.css") {
		for _, m := range classAttrRE2.FindAllStringSubmatch(stripAttrs(r.selector), -1) {
			layout[m[1]] = true
		}
	}
	for c := range sinkClasses(t) {
		if strings.HasPrefix(c, "d-") && !defined[c] {
			t.Errorf("the kitchen sink uses .%s, which no stylesheet defines", c)
		}
		if strings.HasPrefix(c, "x-") && !layout[c] {
			t.Errorf("the kitchen sink uses .%s, which kitchen-sink.css does not define", c)
		}
	}
}

func TestKitchenSinkIsWellFormed(t *testing.T) {
	page := code(t, "kitchen-sink/index.html")
	// unique ids; every id that a reference names exists
	ids := map[string]int{}
	for _, m := range idAttrRE.FindAllStringSubmatch(page, -1) {
		ids[m[1]]++
	}
	for id, n := range ids {
		if n > 1 {
			t.Errorf("id %q is used %d times", id, n)
		}
	}
	for _, re := range []*regexp.Regexp{regexp.MustCompile(`aria-describedby="([^"]*)"`), regexp.MustCompile(`aria-controls="([^"]*)"`), regexp.MustCompile(`aria-labelledby="([^"]*)"`), regexp.MustCompile(`data-pop="([^"]*)"`), regexp.MustCompile(`data-collapse="([^"]*)"`), regexp.MustCompile(`data-open-dialog="([^"]*)"`)} {
		for _, m := range re.FindAllStringSubmatch(page, -1) {
			for _, id := range strings.Fields(m[1]) {
				if ids[id] == 0 {
					t.Errorf("%s names #%s, which is not on the page", re, id)
				}
			}
		}
	}
	// tooltips: the tip has role=tooltip and an id, and the trigger inside its part points at it
	parts := regexp.MustCompile(`(?s)<span class="d-tt[^"]*">(.*?)</span></span>`).FindAllStringSubmatch(page, -1)
	if len(parts) < 7 {
		t.Fatalf("found %d tooltip parts", len(parts))
	}
	for _, p := range parts {
		tip := regexp.MustCompile(`<span class="d-tip" role="tooltip" id="([^"]+)"`).FindStringSubmatch(p[0])
		if tip == nil {
			t.Errorf("a tooltip without role=\"tooltip\" and an id: %.80s", p[0])
			continue
		}
		if !strings.Contains(p[0], `aria-describedby="`+tip[1]+`"`) {
			t.Errorf("the trigger of tooltip #%s does not name it with aria-describedby", tip[1])
		}
	}
	// a folding head is a button inside a heading, so that heading navigation lists it
	if !regexp.MustCompile(`<h[1-6] class="d-heading"><button class="d-head" type="button" aria-expanded=`).MatchString(page) {
		t.Error("a folding head must be <hN class=\"d-heading\"><button class=\"d-head\" aria-expanded>")
	}
	for _, m := range regexp.MustCompile(`<(h[1-6]|button)[^>]*class="d-head[^"]*"[^>]*>`).FindAllStringSubmatch(page, -1) {
		if m[1] == "button" && !strings.Contains(m[0], "aria-expanded") {
			t.Errorf("a button.d-head without aria-expanded: %s", m[0])
		}
	}
	// there is one live region at most, and a toast has no role of its own
	if n := strings.Count(page, "aria-live"); n > 1 {
		t.Errorf("%d live regions", n)
	}
	for _, m := range regexp.MustCompile(`<div class="d-toast(?:\s[^"]*)?"[^>]*>`).FindAllString(page, -1) {
		if strings.Contains(m, "role=") {
			t.Errorf("a toast with a role of its own: %s", m)
		}
	}
	// the dialog's default answer is Cancel
	dlg := regexp.MustCompile(`(?s)<dialog.*?</dialog>`).FindString(page)
	if !strings.Contains(dlg, `value="cancel" autofocus`) {
		t.Error("the dialog's Cancel button must be the default (autofocus)")
	}
	// language, viewport, title
	for _, must := range []string{`<html lang="en">`, `name="viewport"`, "<title>Tabr design: kitchen sink</title>"} {
		if !strings.Contains(page, must) {
			t.Errorf("the page lacks %q", must)
		}
	}
	// every icon the page names is in the sprite
	sprite := map[string]bool{}
	for _, s := range parseSVG(t, IconsFile).Symbols {
		sprite[s.ID] = true
	}
	for _, m := range regexp.MustCompile(`<use href="\.\./assets/icons\.svg#([^"]+)"`).FindAllStringSubmatch(page, -1) {
		if !sprite[m[1]] {
			t.Errorf("the page uses icon #%s, which the sprite lacks", m[1])
		}
	}
	// the three result icons of the toasts are in the page: a drop, a can and a shield, each with its level and its check
	for _, shape := range []string{"M12 2.5C12 2.5 5.5 9.4", "M6.5 3.5h8l3 3v14h-11z", "M12 2.8l7 2.9v5.6"} {
		if strings.Count(page, shape) < 2 {
			t.Errorf("the toast icon %q is not drawn in the page", shape)
		}
	}
	// the icons of the sprite for those three are the same shapes
	icons := readFile(t, "assets/icons.svg")
	for _, shape := range []string{"M12 2.5C12 2.5 5.5 9.4", "M6.5 3.5h8l3 3v14h-11z", "M12 2.8l7 2.9v5.6"} {
		if !strings.Contains(icons, shape) {
			t.Errorf("icons.svg lacks the shape %q", shape)
		}
	}
	// every look is offered
	for _, l := range Looks() {
		if !strings.Contains(page, `data-value="`+l+`"`) {
			t.Errorf("the page offers no switch to the %s look", l)
		}
	}
}
