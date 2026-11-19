package web

import (
	"encoding/json"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type helpPage struct {
	ID    string `json:"id"`
	File  string `json:"file"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

func getHelp(t *testing.T) (pages []helpPage, repo string) {
	t.Helper()
	s := newTestServer(t)
	w := do(t, s, "GET", "http://127.0.0.1/api/help", loop, aboutHeaders(s), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/help: %d %s", w.Code, w.Body.String())
	}
	var m struct {
		Pages []helpPage `json:"pages"`
		Repo  string     `json:"repo"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	return m.Pages, m.Repo
}

// #980: the Help is the Markdown of docs/help, embedded in the program, answered by the editor's own route: no network, no save, no input.
func TestHelpAnswersTheEmbeddedPagesInReadingOrder(t *testing.T) {
	pages, repo := getHelp(t)
	if len(pages) < 6 {
		t.Fatalf("only %d help pages", len(pages))
	}
	if pages[0].ID != "index" || pages[0].File != "README.md" {
		t.Errorf("the index is not first: %+v", pages[0])
	}
	want := []string{"landsraad-board.md", "fill-and-tiers.md", "term-and-decrees.md", "specializations.md", "checked-in-the-game.md"}
	for i, f := range want {
		if pages[i+1].File != f {
			t.Errorf("page %d is %s, want %s (the order of the index)", i+1, pages[i+1].File, f)
		}
	}
	for _, p := range pages {
		if strings.TrimSpace(p.Body) == "" || p.Title == "" || p.ID == "" {
			t.Errorf("an empty page: %+v", p)
		}
		if !strings.HasPrefix(p.Body, "# ") {
			t.Errorf("%s does not start with its heading", p.File)
		}
	}
	if !strings.HasPrefix(repo, "https://github.com/Project-Arrakis/tabr-tau/") {
		t.Errorf("the repository link is %q", repo)
	}
}

// Every file of docs/help is embedded, and nothing else is: a page added to the folder is in the program.
func TestEveryHelpFileIsEmbedded(t *testing.T) {
	ents, err := os.ReadDir("../../docs/help")
	if err != nil {
		t.Fatal(err)
	}
	var disk []string
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".md") {
			disk = append(disk, e.Name())
		}
	}
	pages, _ := getHelp(t)
	var got []string
	for _, p := range pages {
		got = append(got, p.File)
	}
	sort.Strings(disk)
	sort.Strings(got)
	if strings.Join(disk, ",") != strings.Join(got, ",") {
		t.Errorf("docs/help has %v, the program has %v", disk, got)
	}
}

// The pages are plain Markdown: no HTML (the page draws text only), and a link goes to another page of the Help, never to an address.
func TestHelpPagesAreSafeMarkdown(t *testing.T) {
	pages, _ := getHelp(t)
	files := map[string]bool{}
	for _, p := range pages {
		files[p.File] = true
	}
	html := regexp.MustCompile(`<[A-Za-z/!]`)
	link := regexp.MustCompile(`\]\(([^)]*)\)`)
	for _, p := range pages {
		if html.MatchString(p.Body) {
			t.Errorf("%s holds HTML", p.File)
		}
		for _, m := range link.FindAllStringSubmatch(p.Body, -1) {
			if !files[m[1]] {
				t.Errorf("%s links to %q, which is not a Help page", p.File, m[1])
			}
		}
		if strings.Contains(p.Body, "http://") || strings.Contains(p.Body, "https://") {
			t.Errorf("%s names an address; the pages link only to each other", p.File)
		}
	}
}

// What was taken out of the board and the Term section as standing text is in the Help (#980, #981, #973, #985).
func TestHelpHoldsWhatTheBoardNoLongerSays(t *testing.T) {
	pages, _ := getHelp(t)
	all := ""
	for _, p := range pages {
		all += "\n" + p.Body
	}
	for _, s := range []string{
		"three separate states", "How each click is written", "Favor won", "Earned is not collected", "Not checked in the game",
		"Next term's decree", "Ends in", "counts down only while the game is running", "the plaque above only shows what these hold",
		"Active Landsraad missions are not recorded", "A ranking of contributors is not recorded", "Reopen", "Going back",
	} {
		if !strings.Contains(strings.ToLower(all), strings.ToLower(s)) {
			t.Errorf("the Help does not say %q", s)
		}
	}
}
