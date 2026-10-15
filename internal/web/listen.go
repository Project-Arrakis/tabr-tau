package web

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

// safeFilename keeps only [A-Za-z0-9._-] so a name can never inject a header or a path.
func safeFilename(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-' {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return "export"
	}
	return string(out)
}

// CheckListenAddr refuses any bind address that is not loopback: the editor has no login and can rewrite the save, so
// it never listens beyond this computer (the old --allow-remote override was removed, 2026-10-08). The address must
// always be a well-formed host:port. An empty host (":8090") means every interface and is treated as non-loopback
// (finding F-04 / NET-1).
func CheckListenAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid listen address %q (want host:port): %w", addr, err)
	}
	if n, err := strconv.Atoi(port); err != nil || n < 0 || n > 65535 {
		return fmt.Errorf("invalid port %q in listen address", port)
	}
	if host == "localhost" {
		_, err := localhostIPs()
		return err
	}
	if isLoopbackHost(host) {
		return nil
	}
	return errors.New("refusing to listen on " + addr + ": this editor has no login and can rewrite your save, so it only binds to this computer (127.0.0.1)")
}

func isLoopbackHost(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// lookupIPs resolves a host name. It is a variable so a test can stand in for a resolver and a hosts file.
var lookupIPs = net.LookupIP

// localhostIPs resolves the name "localhost" and requires every address it gives to be loopback. The name is resolved by the operating
// system (its hosts file, its resolver), so on a machine where that was changed "localhost" could name an address that is not this
// computer, and an editor bound there would be reachable from the network (NET-4 of the 2026-10-15 audit).
func localhostIPs() ([]net.IP, error) {
	ips, err := lookupIPs("localhost")
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("cannot listen on localhost: the name does not resolve here (use 127.0.0.1)")
	}
	for _, ip := range ips {
		if !ip.IsLoopback() {
			return nil, fmt.Errorf("refusing to listen on localhost: it resolves to %s, which is not a loopback address (check the hosts file; use 127.0.0.1)", ip)
		}
	}
	return ips, nil
}

// bindAddr is addr with the name "localhost" replaced by the loopback address it was just checked to be, so the address that was checked
// is the address that is bound (the name is not resolved a second time).
func bindAddr(addr string) (string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || host != "localhost" {
		return addr, nil
	}
	ips, err := localhostIPs()
	if err != nil {
		return "", err
	}
	pick := ips[0]
	for _, ip := range ips {
		if ip.To4() != nil {
			pick = ip
			break
		}
	}
	return net.JoinHostPort(pick.String(), port), nil
}

// Listen binds addr. If the operator did not choose the address (explicit=false) and it is busy, it falls back
// to a random free loopback port instead of failing; an explicit address never moves silently.
func Listen(addr string, explicit bool) (net.Listener, error) {
	bind, err := bindAddr(addr)
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", bind)
	if err != nil && !explicit {
		return net.Listen("tcp", "127.0.0.1:0")
	}
	return ln, err
}

// PortChangeNote says, in a sentence for a person, that the editor is not on the port that was asked for: "" when it is. Listen moves
// to a random free port without a word when the default one is busy; a second editor then runs next to the first, and a bookmark or the
// window of the first points at the wrong one (NET-1 of the 2026-10-15 audit).
func PortChangeNote(requested string, ln net.Listener) string {
	_, want, err := net.SplitHostPort(requested)
	if err != nil {
		return ""
	}
	_, got, err := net.SplitHostPort(ln.Addr().String())
	if err != nil || want == got || want == "0" {
		return ""
	}
	return "Port " + want + " is already in use, probably by another tabr-tau that is still open (look for its window or terminal), so this editor is using port " + got + " instead. " +
		"Do not edit the same save in both: the game keeps one copy, and the second editor's save would be refused or overwrite the first's."
}

// NewHTTPServer applies connection limits so a slow or oversized request cannot hold the server open.
func NewHTTPServer(h http.Handler) *http.Server {
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      2 * time.Minute, // exports of large tables
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}
