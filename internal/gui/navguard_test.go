package gui

import (
	"strings"
	"testing"
)

const editor = "http://127.0.0.1:8090"

// Where the editor's window may go: its own origin and nothing else (NET-8 of the 2026-10-22 audit).
func TestAllowedNavigation(t *testing.T) {
	allowed := []string{
		"http://127.0.0.1:8090/",
		"http://127.0.0.1:8090",
		"http://127.0.0.1:8090/boot?t=abc123",
		"http://127.0.0.1:8090/api/db/export?name=x#frag",
		"HTTP://127.0.0.1:8090/UPPER",
		"blob:http://127.0.0.1:8090/5f3a-uuid",
		"about:blank",
	}
	for _, u := range allowed {
		if !AllowedNavigation(editor, u) {
			t.Errorf("must be allowed: %q", u)
		}
	}
	refused := []string{
		"",
		"https://example.com/",
		"http://example.com:8090/",
		"http://127.0.0.1:8091/",             // another port
		"http://127.0.0.1/",                  // port 80
		"https://127.0.0.1:8090/",            // another scheme
		"http://127.0.0.2:8090/",             // another loopback address
		"http://localhost:8090/",             // another name for it
		"http://[::1]:8090/",                 // IPv6 loopback
		"http://127.0.0.1.example.com:8090/", // the address as a prefix of a name
		"http://example.com:8090@127.0.0.1:8090/x",
		"http://127.0.0.1:8090@example.com/",    // user info: names example.com
		"http://127.0.0.1:8090:80@example.com/", //
		"http://127.0.0.1:8090\\@example.com/",  // a backslash is a slash to the engine
		"http://example.com\\@127.0.0.1:8090/",  //
		"http://127.0.0.1:8090\t.example.com/",  // a tab is dropped by the engine
		"http://127.0.0.1:8090\n.example.com/",  //
		"http://127.0.0.1:8090 .example.com/",   //
		"//example.com/",                        // scheme relative
		"/api/state",                            // no host
		"file:///C:/Windows/win.ini",            //
		"ftp://127.0.0.1:8090/",                 //
		"javascript:alert(1)",                   //
		"data:text/html,<script>1</script>",     //
		"view-source:http://127.0.0.1:8090/",    //
		"edge://settings",                       //
		"chrome://version",                      //
		"about:srcdoc",                          //
		"blob:https://example.com/5f3a",         // a blob of another origin
		"blob:",                                 //
		"blob:blob:http://127.0.0.1:8090/x",     //
		"ws://127.0.0.1:8090/",                  //
		"http://127.0.0.1:8090:abc/",            // malformed port
		"http://%31%32%37.0.0.1:8090/",          // escaped host is not the host
		"http://127.0.0.1:8090/\x00",            //
		"http://127.0.0.1:8090/a\\b",            // a backslash anywhere is refused, even where Go and the engine would agree
		"http://127.0.0.1:8090/a b",             // so is a space
		"http://127.0.0.1:8090/a\x7fb",          // and DEL
	}
	for _, u := range refused {
		if AllowedNavigation(editor, u) {
			t.Errorf("must be refused: %q", u)
		}
	}
}

// With no usable origin to compare with, nothing is allowed (except the blank page).
func TestNothingIsAllowedWithoutAnOrigin(t *testing.T) {
	for _, o := range []string{"", "not a url", "file:///x", "http://", "ftp://127.0.0.1:8090"} {
		if AllowedNavigation(o, "http://127.0.0.1:8090/") {
			t.Errorf("origin %q: nothing may be allowed", o)
		}
	}
	if AllowedNavigation("ftp://127.0.0.1:8090", "ftp://127.0.0.1:8090/") || AllowedNavigation("ws://127.0.0.1:8090", "ws://127.0.0.1:8090/") {
		t.Error("only http and https origins are ever accepted")
	}
	if !AllowedNavigation("", "about:blank") {
		t.Error("the blank page is always fine")
	}
}

// The default port of a scheme is the port: an origin written without one matches an address written with it.
func TestDefaultPortsAreEqualToTheirNumber(t *testing.T) {
	if !AllowedNavigation("http://127.0.0.1", "http://127.0.0.1:80/") || !AllowedNavigation("https://127.0.0.1:443", "https://127.0.0.1/") {
		t.Error("a default port written out or left out is the same origin")
	}
	if AllowedNavigation("http://127.0.0.1", "https://127.0.0.1/") {
		t.Error("http and https are different origins")
	}
}

// The page shown in place of a refused one links only to the editor, whatever the origin text holds.
func TestBlockedPageLinksOnlyBackToTheEditor(t *testing.T) {
	p := string(BlockedPage(editor))
	if !strings.Contains(p, `href="http://127.0.0.1:8090/"`) || strings.Count(p, "href=") != 1 {
		t.Fatalf("one link, to the editor: %s", p)
	}
	if q := string(BlockedPage("http://127.0.0.1:8090/some/path?x=\"onmouseover=\"y#f")); !strings.Contains(q, `href="http://127.0.0.1:8090/"`) || strings.Contains(q, "onmouseover") {
		t.Fatalf("only the origin is used, not a path or query: %s", q)
	}
	if strings.Contains(p, "<script") {
		t.Fatal("no script in the blocked page")
	}
	for _, bad := range []string{`"><script>alert(1)</script>`, "javascript:alert(1)", ""} {
		q := string(BlockedPage(bad))
		if strings.Contains(q, "<script") || strings.Contains(q, "javascript:") || strings.Contains(q, "evil.example") || !strings.Contains(q, `href="about:blank"`) {
			t.Errorf("an origin that is not an address (%q) must give a link to nowhere: %s", bad, q)
		}
	}
}

// The window is opened on the editor's address; its origin is what the guard compares with.
func TestOriginOf(t *testing.T) {
	for in, want := range map[string]string{
		"http://127.0.0.1:8090/boot?t=abc": "http://127.0.0.1:8090",
		"http://127.0.0.1:8090":            "http://127.0.0.1:8090",
		"HTTP://LOCALHOST:9000/x":          "http://localhost:9000",
		"http://127.0.0.1/":                "http://127.0.0.1:80",
		"http://[::1]:8090/":               "http://[::1]:8090",
		"http://user:pw@127.0.0.1:8090/":   "",
		"file:///C:/x":                     "",
		"javascript:alert(1)":              "",
		"":                                 "",
		"127.0.0.1:8090":                   "",
	} {
		if got := OriginOf(in); got != want {
			t.Errorf("OriginOf(%q) = %q, want %q", in, got, want)
		}
	}
	// the origin it gives is one AllowedNavigation accepts for the same address
	if o := OriginOf("http://127.0.0.1:8090/boot?t=abc"); !AllowedNavigation(o, "http://127.0.0.1:8090/boot?t=abc") {
		t.Error("the opening address must be allowed")
	}
}
