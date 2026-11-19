// Package help holds the Help pages of the editor: Markdown files that are read on GitHub as they are and are embedded in the program, so the Help tab works with no network
// and always matches the running build (#980). The pages are plain Markdown (headings, paragraphs, lists, tables, bold, code and links to the other pages); no HTML.
package help

import (
	"embed"
	"sort"
	"strings"
)

//go:embed *.md
var files embed.FS

// Page is one Help page. ID is the file name without ".md" ("index" for README.md), File is the file name (the link target inside the pages and on GitHub), Title is the first heading.
type Page struct {
	ID    string `json:"id"`
	File  string `json:"file"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// RepoURL is the folder of the same pages in the repository: the Help tab offers it as an optional link ("also on GitHub"). It is only ever shown as a link; nothing is fetched from it.
const RepoURL = "https://github.com/Project-Arrakis/tabr-tau/blob/main/docs/help/"

// Pages returns the pages in reading order: the index first, then the others in the order the index lists them (a page the index does not list comes last, by file name).
func Pages() []Page {
	ents, err := files.ReadDir(".")
	if err != nil {
		return nil
	}
	var out []Page
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		b, err := files.ReadFile(name)
		if err != nil {
			continue
		}
		body := strings.ReplaceAll(string(b), "\r\n", "\n")
		id := strings.TrimSuffix(name, ".md")
		if name == "README.md" {
			id = "index"
		}
		out = append(out, Page{ID: id, File: name, Title: titleOf(body, id), Body: body})
	}
	index := ""
	for _, p := range out {
		if p.ID == "index" {
			index = p.Body
		}
	}
	rank := map[string]int{}
	for _, p := range out {
		at := strings.Index(index, "("+p.File+")")
		switch {
		case p.ID == "index":
			rank[p.ID] = -1
		case at >= 0:
			rank[p.ID] = at
		default:
			rank[p.ID] = 1 << 30
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if ri, rj := rank[out[i].ID], rank[out[j].ID]; ri != rj {
			return ri < rj
		}
		return out[i].File < out[j].File
	})
	return out
}

func titleOf(body, fallback string) string {
	for _, l := range strings.Split(body, "\n") {
		if t, ok := strings.CutPrefix(l, "# "); ok {
			return strings.TrimSpace(t)
		}
	}
	return fallback
}
