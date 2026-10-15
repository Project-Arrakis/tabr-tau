package web

import (
	"net"
	"strings"
	"testing"
)

func withResolver(t *testing.T, ips ...string) {
	t.Helper()
	old := lookupIPs
	lookupIPs = func(host string) ([]net.IP, error) {
		if host != "localhost" {
			t.Fatalf("only localhost is resolved here, not %q", host)
		}
		var out []net.IP
		for _, s := range ips {
			out = append(out, net.ParseIP(s))
		}
		return out, nil
	}
	t.Cleanup(func() { lookupIPs = old })
}

// "localhost" is resolved by the operating system. If a changed hosts file made it name another address, the editor would be bound where
// the network can reach it, so the name is resolved, every answer must be loopback, and that answer (not the name) is what is bound (NET-4
// of the 2026-10-15 audit).
func TestLocalhostMustResolveToLoopbackOnly(t *testing.T) {
	withResolver(t, "127.0.0.1", "::1")
	if err := CheckListenAddr("localhost:8090"); err != nil {
		t.Fatalf("an ordinary localhost is fine: %v", err)
	}
	withResolver(t, "127.0.0.1", "192.168.1.20")
	err := CheckListenAddr("localhost:8090")
	if err == nil || !strings.Contains(err.Error(), "192.168.1.20") || !strings.Contains(err.Error(), "hosts file") {
		t.Fatalf("a localhost that names a LAN address must be refused with the reason: %v", err)
	}
	withResolver(t)
	if err := CheckListenAddr("localhost:8090"); err == nil {
		t.Fatal("a localhost that resolves to nothing must be refused")
	}
	// literal addresses never touch the resolver
	withResolver(t, "10.0.0.1")
	for _, a := range []string{"127.0.0.1:8090", "[::1]:8090", "127.5.5.5:1"} {
		if err := CheckListenAddr(a); err != nil {
			t.Errorf("%s: %v", a, err)
		}
	}
}

func TestListenBindsTheCheckedAddressNotTheName(t *testing.T) {
	withResolver(t, "::1", "127.0.0.1")
	got, err := bindAddr("localhost:8090")
	if err != nil || got != "127.0.0.1:8090" {
		t.Fatalf("the IPv4 loopback is preferred and bound by number: %q %v", got, err)
	}
	withResolver(t, "::1")
	if got, err := bindAddr("localhost:8090"); err != nil || got != "[::1]:8090" {
		t.Fatalf("%q %v", got, err)
	}
	withResolver(t, "203.0.113.9")
	if _, err := bindAddr("localhost:8090"); err == nil {
		t.Fatal("a non-loopback answer must not be bound")
	}
	if ln, err := Listen("localhost:8090", true); err == nil {
		ln.Close()
		t.Fatal("Listen must refuse it too")
	}
	if got, err := bindAddr("127.0.0.1:8090"); err != nil || got != "127.0.0.1:8090" {
		t.Fatalf("a literal address is left alone: %q %v", got, err)
	}
	withResolver(t, "127.0.0.1")
	ln, err := Listen("localhost:0", true)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if !strings.HasPrefix(ln.Addr().String(), "127.0.0.1:") {
		t.Fatalf("bound %s", ln.Addr())
	}
}
