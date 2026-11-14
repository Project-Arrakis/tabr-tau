// publictextcheck applies the public-text rule (package internal/hygiene) to text that is not a tracked file: the title and body of an issue or a
// pull request, a comment or a review, and commit messages. CI runs it before merge on every such event (.github/workflows/public-text.yml).
//
//	publictextcheck                       the event of the running workflow (GITHUB_EVENT_NAME, GITHUB_EVENT_PATH)
//	publictextcheck -items FILE           a JSON array of issues or pull requests as `gh issue list --json` writes them (a one-off scan)
//	publictextcheck -commits FILE         commit messages, from `git log --format='%H%x00%B%x01' RANGE > FILE` (the tool starts no program)
//	publictextcheck -digest WORD...       print the digest of a word, or of up to four words in a row, to add to the list
//
// A report names the place of a hit and its rule, never the text, because a CI log of a public repository is public. -show prints the text as
// well and is refused in CI. The extra digest list is read from the environment variable PUBLIC_TEXT_EXTRA_DIGESTS.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Project-Arrakis/tabr-tau/internal/hygiene"
)

// text is one piece of public text and where it came from.
type text struct {
	where string
	body  string
	pr    bool // a pull request's title or body: it becomes the squash message, so the attribution rule applies too
}

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	show, itemsFile, commits := false, "", ""
	var digestWords []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-show":
			show = true
		case "-items":
			if i++; i >= len(args) {
				fmt.Fprintln(stderr, "publictextcheck: -items needs a file")
				return 2
			}
			itemsFile = args[i]
		case "-commits":
			if i++; i >= len(args) {
				fmt.Fprintln(stderr, "publictextcheck: -commits needs a file")
				return 2
			}
			commits = args[i]
		case "-digest":
			digestWords = args[i+1:]
			i = len(args)
		default:
			fmt.Fprintf(stderr, "publictextcheck: unknown argument %q\n", args[i])
			return 2
		}
	}
	if digestWords != nil {
		if len(digestWords) == 0 || len(digestWords) > 4 {
			fmt.Fprintln(stderr, "publictextcheck: -digest takes one word, or up to four words that follow one another")
			return 2
		}
		fmt.Fprintln(stdout, hygiene.Digest(digestWords...))
		return 0
	}
	if show && getenv("GITHUB_ACTIONS") != "" {
		fmt.Fprintln(stderr, "publictextcheck: -show is refused in CI, where the log is public")
		return 2
	}
	checker := hygiene.New(getenv("PUBLIC_TEXT_EXTRA_DIGESTS"))

	var texts []text
	var err error
	switch {
	case itemsFile != "":
		var b []byte
		if b, err = os.ReadFile(itemsFile); err == nil {
			texts, err = itemTexts(b)
		}
	case commits != "":
		var b []byte
		if b, err = os.ReadFile(commits); err == nil {
			texts = commitTexts(string(b))
		}
	default:
		name, path := getenv("GITHUB_EVENT_NAME"), getenv("GITHUB_EVENT_PATH")
		if name == "" || path == "" {
			fmt.Fprintln(stderr, "publictextcheck: no event (GITHUB_EVENT_NAME and GITHUB_EVENT_PATH are not set) and no -items or -commits")
			return 2
		}
		var b []byte
		if b, err = os.ReadFile(path); err == nil {
			texts, err = eventTexts(name, b)
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "publictextcheck:", err)
		return 2
	}

	failed := 0
	for _, t := range texts {
		hits := checker.Check(t.body)
		if t.pr {
			hits = append(hits, hygiene.CheckAttribution(t.body)...)
		}
		for _, h := range hits {
			failed++
			fmt.Fprintf(stdout, "%s, line %d: %s\n", t.where, h.Line, h.Kind)
			if show {
				fmt.Fprintf(stdout, "    %q\n", h.Text)
			}
		}
	}
	if failed > 0 {
		fmt.Fprintf(stderr, "\npublic text rule: %d place(s) above break it. Public text (issues, pull requests, comments, commit messages and files)\n"+
			"never names another private tool or project and never carries a path under a personal profile or an account name.\n"+
			"Edit the text to say what is needed in neutral words (for example %%USERPROFILE%% for a profile path) and the\n"+
			"check runs again when the text is edited. The log names the place and the rule only, on purpose.\n", failed)
		return 1
	}
	fmt.Fprintf(stdout, "public text OK (%d text(s) checked)\n", len(texts))
	return 0
}

type event struct {
	Issue *struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		Body        string `json:"body"`
		PullRequest *struct {
			URL string `json:"url"`
		} `json:"pull_request"`
	} `json:"issue"`
	PullRequest *struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Head   struct {
			Ref string `json:"ref"`
		} `json:"head"`
	} `json:"pull_request"`
	Comment *struct {
		Body string `json:"body"`
	} `json:"comment"`
	Review *struct {
		Body string `json:"body"`
	} `json:"review"`
}

// eventTexts lists the public text of a workflow event payload: the title and body of the issue or pull request, the branch name of a pull
// request, and the body of the comment or review the event is about.
func eventTexts(name string, payload []byte) ([]text, error) {
	var ev event
	if err := json.Unmarshal(payload, &ev); err != nil {
		return nil, fmt.Errorf("the event payload of %q is not JSON: %w", name, err)
	}
	var out []text
	if i := ev.Issue; i != nil {
		isPR := i.PullRequest != nil
		kind := "issue"
		if isPR {
			kind = "pull request"
		}
		out = append(out, text{fmt.Sprintf("%s #%d title", kind, i.Number), i.Title, isPR}, text{fmt.Sprintf("%s #%d body", kind, i.Number), i.Body, isPR})
	}
	if p := ev.PullRequest; p != nil {
		out = append(out,
			text{fmt.Sprintf("pull request #%d title", p.Number), p.Title, true},
			text{fmt.Sprintf("pull request #%d body", p.Number), p.Body, true},
			text{fmt.Sprintf("pull request #%d branch name", p.Number), p.Head.Ref, false})
	}
	if c := ev.Comment; c != nil {
		out = append(out, text{name + " comment", c.Body, false})
	}
	if r := ev.Review; r != nil {
		out = append(out, text{name + " review", r.Body, false})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("the event %q carries no issue, pull request, comment or review text", name)
	}
	return out, nil
}

type item struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	HeadRefName string `json:"headRefName"`
	URL         string `json:"url"`
	HTMLURL     string `json:"html_url"`
	Comments    []struct {
		Body string `json:"body"`
	} `json:"comments"`
	Reviews []struct {
		Body string `json:"body"`
	} `json:"reviews"`
}

// itemTexts lists the text of issues or pull requests in the JSON form of `gh issue list --json number,title,body,comments`.
func itemTexts(b []byte) ([]text, error) {
	var items []item
	if err := json.Unmarshal(b, &items); err != nil {
		return nil, fmt.Errorf("the items file is not a JSON array of issues or pull requests: %w", err)
	}
	var out []text
	for _, it := range items {
		id := fmt.Sprintf("#%d", it.Number)
		if it.Number == 0 {
			id = it.HTMLURL + it.URL
		}
		out = append(out, text{id + " title", it.Title, false}, text{id + " body", it.Body, false})
		if it.HeadRefName != "" {
			out = append(out, text{id + " branch name", it.HeadRefName, false})
		}
		for i, c := range it.Comments {
			out = append(out, text{fmt.Sprintf("%s comment %d", id, i+1), c.Body, false})
		}
		for i, r := range it.Reviews {
			out = append(out, text{fmt.Sprintf("%s review %d", id, i+1), r.Body, false})
		}
	}
	return out, nil
}

// commitTexts lists the messages of the commits in the output of `git log --format='%H%x00%B%x01'`.
func commitTexts(log string) []text {
	var out []text
	for _, rec := range strings.Split(log, "\x01") {
		rec = strings.TrimLeft(rec, "\n")
		sha, msg, ok := strings.Cut(rec, "\x00")
		if !ok || sha == "" {
			continue
		}
		out = append(out, text{"commit " + sha[:min(12, len(sha))] + " message", msg, false})
	}
	return out
}
