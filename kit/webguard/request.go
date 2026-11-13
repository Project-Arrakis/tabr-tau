package webguard

import (
	"mime"
	"net/http"
	"net/url"
	"strings"
)

// OriginPolicy is where programs that use SameOrigin differ. The zero value is the strictest.
type OriginPolicy struct {
	// AllowFetchSiteNone lets "Sec-Fetch-Site: none" pass: a request the person started in the browser itself (the address bar, a
	// bookmark). The strictest policy refuses it, so only a request made by the page counts.
	AllowFetchSiteNone bool
	// FoldHostCase compares the host in Origin with the Host header without regard to case. The strictest policy compares them byte
	// for byte (a browser sends both in lower case).
	FoldHostCase bool
}

// SameOrigin says whether a request that changes something was made by the program's own page and not by another site: when the
// browser sends Fetch Metadata, Sec-Fetch-Site must be "same-origin" (or "none" if the policy allows it); when it sends an Origin, that
// Origin must be an http address whose host (with its port) is the Host the request was sent to. A request with neither header (curl,
// a script) passes: something else, a token or a session, has to prove where it came from. Another scheme (https, file, an empty
// scheme, "null") never passes, because the programs that use this serve plain http.
func SameOrigin(r *http.Request, p OriginPolicy) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "", "same-origin":
	case "none":
		if !p.AllowFetchSiteNone {
			return false
		}
	default:
		return false
	}
	if o := r.Header.Get("Origin"); o != "" {
		u, err := url.Parse(o)
		if err != nil || u.Scheme != "http" {
			return false
		}
		if p.FoldHostCase {
			return strings.EqualFold(u.Host, r.Host)
		}
		return u.Host == r.Host
	}
	return true
}

// JSONRequest says whether the request's Content-Type is application/json by its parsed media type: any case, parameters such as a
// charset allowed, and nothing that merely starts with the words (application/jsonx, application/json-seq). A browser cannot send
// this type across sites without a preflight request, which is why a write route asks for it.
func JSONRequest(r *http.Request) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mt == "application/json"
}
