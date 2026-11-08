// Package webguard holds the small rules that a program serving its page only to this computer puts in front of its routes: is this
// address this computer, may a window go to this address, which security headers an answer carries, and is this request one the
// program's own page made.
//
// It is mechanism, not policy. The Content-Security-Policy text, which Host names a program accepts, the token or cookie that proves a
// request came from the page, the routes and the wording of an error stay in the program. Every function is free of state and of any
// dependency but the standard library's net, net/http, net/url and mime, and none of them resolves a name.
package webguard

import (
	"net"
	"strings"
)

// Host says whether hostport, a host or a host:port (a listen address, a Host header, the host part of a URL), names this computer: the
// name "localhost" in any case, or a loopback IP address, with or without brackets and port. Nothing else is resolved: a name that
// points to a loopback address through DNS is not accepted.
//
// A program that accepts fewer names than this (only the exact spellings 127.0.0.1, localhost and [::1], say) keeps its own test; this
// one is the wide rule: any loopback address, any case of localhost.
func Host(hostport string) bool {
	h := hostport
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		h = host
	}
	h = strings.Trim(h, "[]")
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// Peer says whether the connection itself comes from this computer (remote is the peer address host:port of a request, as
// http.Request.RemoteAddr holds it), whatever Host the request names. An address with no port, a name and an address that is not
// loopback are all refused.
func Peer(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
