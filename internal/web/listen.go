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
	if isLoopbackHost(host) {
		return nil
	}
	return errors.New("refusing to listen on " + addr + ": this editor has no login and can rewrite your save, so it only binds to this computer (127.0.0.1)")
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Listen binds addr. If the operator did not choose the address (explicit=false) and it is busy, it falls back
// to a random free loopback port instead of failing; an explicit address never moves silently.
func Listen(addr string, explicit bool) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil && !explicit {
		return net.Listen("tcp", "127.0.0.1:0")
	}
	return ln, err
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
