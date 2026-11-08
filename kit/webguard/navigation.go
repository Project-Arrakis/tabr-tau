package webguard

import (
	"net"
	"net/url"
	"strings"
)

// AllowedNavigation is the whole rule for where a program's own window, one with no address bar, may go: the program's own origin (same
// scheme, host and port as origin), an in-page blob made by the program's own page (a download), and the blank page the engine starts
// from. Anything else, a web address, a file, a script or data address, is refused. It is free of any window code so that it can be
// tested on every platform; a window asks it for every page it is about to load.
//
// origin is the address the window was opened on (OriginOf makes one from it). With no usable origin nothing is allowed except the
// blank page.
func AllowedNavigation(origin, raw string) bool {
	if raw == "about:blank" {
		return true
	}
	// Characters a web engine reads differently from Go's parser (a backslash is a slash to it, a tab or newline is dropped) are refused
	// outright, so the two can never disagree about which host an address names.
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

// parseOrigin reads an http or https address into its origin. Addresses with a user name or password in them are refused
// (http://127.0.0.1:8090@elsewhere/ names elsewhere).
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

// OriginOf is the origin (scheme, host and port, the port written out) of an http or https address, or "" for anything else. A window
// is opened on the program's address and may then only stay on that address's origin.
func OriginOf(raw string) string {
	o := parseOrigin(raw)
	if o == nil {
		return ""
	}
	return o.scheme + "://" + net.JoinHostPort(o.host, o.port)
}
