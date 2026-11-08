package webguard

import "net/http"

// DefaultCSP is the Content-Security-Policy that SecurityHeaders sends when a Policy names none: nothing may be loaded, framed or
// submitted. It fails closed: a program that forgot its policy gets a page that shows nothing, not a page with no policy.
const DefaultCSP = "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"

// Policy is the part of a program's response headers that differs from program to program.
type Policy struct {
	// ContentSecurityPolicy is the whole header value. Empty means DefaultCSP.
	ContentSecurityPolicy string
	// FrameOptions is the X-Frame-Options value ("DENY"); empty sends none (frame-ancestors in the policy still applies).
	FrameOptions string
	// CacheControl is the Cache-Control value; empty sends none, so the caller can choose per route.
	CacheControl string
}

// SecurityHeaders sets the headers of a program that serves its own page to this computer: the Content-Security-Policy, nosniff, no
// Referer, and same-origin only for the resource policy, always; X-Frame-Options and Cache-Control when the Policy names them. Call it
// before anything is written, and before any check that can refuse the request, so that an error answer carries them too.
func SecurityHeaders(w http.ResponseWriter, p Policy) {
	h := w.Header()
	csp := p.ContentSecurityPolicy
	if csp == "" {
		csp = DefaultCSP
	}
	h.Set("Content-Security-Policy", csp)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	if p.FrameOptions != "" {
		h.Set("X-Frame-Options", p.FrameOptions)
	}
	if p.CacheControl != "" {
		h.Set("Cache-Control", p.CacheControl)
	}
}
