package webguard

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersAlwaysSendTheFixedFour(t *testing.T) {
	for name, p := range map[string]Policy{
		"zero":    {},
		"csp":     {ContentSecurityPolicy: "default-src 'self'"},
		"all":     {ContentSecurityPolicy: "default-src 'self'", FrameOptions: "DENY", CacheControl: "no-store"},
		"no csp":  {FrameOptions: "DENY"},
		"cache":   {CacheControl: "no-cache"},
		"no warn": {ContentSecurityPolicy: " "},
	} {
		w := httptest.NewRecorder()
		SecurityHeaders(w, p)
		h := w.Header()
		if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Referrer-Policy") != "no-referrer" || h.Get("Cross-Origin-Resource-Policy") != "same-origin" {
			t.Errorf("%s: fixed headers missing or wrong: %v", name, h)
		}
		if h.Get("Content-Security-Policy") == "" {
			t.Errorf("%s: no Content-Security-Policy", name)
		}
	}
}

func TestSecurityHeadersTakeThePoliciesValuesVerbatim(t *testing.T) {
	const csp = "default-src 'none'; script-src 'self'; frame-ancestors 'none'"
	w := httptest.NewRecorder()
	SecurityHeaders(w, Policy{ContentSecurityPolicy: csp, FrameOptions: "DENY", CacheControl: "no-store"})
	for k, v := range map[string]string{"Content-Security-Policy": csp, "X-Frame-Options": "DENY", "Cache-Control": "no-store"} {
		if got := w.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestSecurityHeadersSendOnlyWhatThePolicyNames(t *testing.T) {
	w := httptest.NewRecorder()
	w.Header().Set("Cache-Control", "max-age=60") // the caller's own choice for a route is not erased
	SecurityHeaders(w, Policy{ContentSecurityPolicy: "default-src 'none'"})
	if got := w.Header().Get("X-Frame-Options"); got != "" {
		t.Errorf("X-Frame-Options = %q, want none", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "max-age=60" {
		t.Errorf("Cache-Control = %q, want the caller's", got)
	}
}

// A program that forgets its policy must not be left with none: the default shows nothing and allows nothing.
func TestAnEmptyPolicyFailsClosed(t *testing.T) {
	w := httptest.NewRecorder()
	SecurityHeaders(w, Policy{})
	if got := w.Header().Get("Content-Security-Policy"); got != DefaultCSP {
		t.Fatalf("CSP = %q, want the default", got)
	}
	for _, d := range []string{"default-src 'none'", "frame-ancestors 'none'", "base-uri 'none'", "form-action 'none'"} {
		if !strings.Contains(DefaultCSP, d) {
			t.Errorf("DefaultCSP lacks %q", d)
		}
	}
	for _, loose := range []string{"'self'", "*", "unsafe", "data:", "http"} {
		if strings.Contains(DefaultCSP, loose) {
			t.Errorf("DefaultCSP contains %q", loose)
		}
	}
}
