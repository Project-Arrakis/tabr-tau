package gui

import (
	"html"
	"net"
	"net/url"
	"strings"
)

// The editor's window has no address bar and shows only the editor's own page, served on a loopback address. AllowedNavigation is the whole rule for where that window may
// go: the editor's own origin (same scheme, host and port), an in-page blob made by the editor's own page (a download), and the blank page the engine starts from. Anything
// else, a web address, a file, a script or data address, is refused. It is kept small and free of the window code so that it can be tested on every platform; the window
// asks it for every page it is about to load (gui_windows.go).
func AllowedNavigation(origin, raw string) bool {
	if raw == "about:blank" {
		return true
	}
	// Characters a web engine reads differently from Go's parser (a backslash is a slash to it, a tab or newline is dropped) are refused outright, so the two can never
	// disagree about which host an address names.
	for i := 0; i < len(raw); i++ {
		if c := raw[i]; c <= 0x20 || c == 0x7f || c == '\\' {
			return false
		}
	}
	o := parseOrigin(origin)
	if o == nil {
		return false
	}
	if len(raw) >= 5 && strings.EqualFold(raw[:5], "blob:") { // an in-page blob is judged by the origin it names
		raw = raw[5:]
	}
	u := parseOrigin(raw)
	if u == nil {
		return false
	}
	return u.scheme == o.scheme && u.host == o.host && u.port == o.port
}

type site struct{ scheme, host, port string }

// parseOrigin reads an http or https address into its origin. Addresses with a user name or password in them are refused (http://127.0.0.1:8090@elsewhere/ names elsewhere).
func parseOrigin(raw string) *site {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Host == "" {
		return nil
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil
	}
	port := u.Port()
	if port == "" {
		port = map[string]string{"http": "80", "https": "443"}[scheme]
	}
	return &site{scheme: scheme, host: strings.ToLower(u.Hostname()), port: port}
}

// BlockedPage is what the window shows in place of a page it refused: a sentence and one link back to the editor.
func BlockedPage(origin string) []byte {
	back := "about:blank"
	if o := parseOrigin(origin); o != nil {
		back = o.scheme + "://" + net.JoinHostPort(o.host, o.port) + "/" // the origin only: no path, query or fragment of what was passed
	}
	return []byte("<!doctype html><meta charset=utf-8><title>Blocked</title><body style=\"font-family:sans-serif;margin:3em\"><h1>Blocked</h1>" +
		"<p>This window only shows the editor. The address it was asked to open is somewhere else, so it was not opened.</p>" +
		"<p><a href=\"" + html.EscapeString(back) + "\">Back to the editor</a></p>")
}

// OriginOf is the origin (scheme, host and port) of an http or https address, or "" for anything else. The window is opened on the editor's address and may then only stay on that
// address's origin.
func OriginOf(raw string) string {
	o := parseOrigin(raw)
	if o == nil {
		return ""
	}
	return o.scheme + "://" + net.JoinHostPort(o.host, o.port)
}
