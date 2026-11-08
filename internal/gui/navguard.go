package gui

import (
	"html"

	"github.com/Project-Arrakis/tabr-tau/kit/webguard"
)

// The editor's window has no address bar and shows only the editor's own page, served on a loopback address. The rule for where that window may go (webguard.AllowedNavigation:
// the editor's own origin, an in-page blob made by the editor's own page, and the blank page) lives in the kit module, shared and tested on every platform; the window asks it for
// every page it is about to load (gui_windows.go). What stays here is the page the window shows in place of a refused one, which carries the editor's wording.

// BlockedPage is what the window shows in place of a page it refused: a sentence and one link back to the editor.
func BlockedPage(origin string) []byte {
	back := "about:blank"
	if o := webguard.OriginOf(origin); o != "" {
		back = o + "/" // the origin only: no path, query or fragment of what was passed
	}
	return []byte("<!doctype html><meta charset=utf-8><title>Blocked</title><body style=\"font-family:sans-serif;margin:3em\"><h1>Blocked</h1>" +
		"<p>This window only shows the editor. The address it was asked to open is somewhere else, so it was not opened.</p>" +
		"<p><a href=\"" + html.EscapeString(back) + "\">Back to the editor</a></p>")
}
