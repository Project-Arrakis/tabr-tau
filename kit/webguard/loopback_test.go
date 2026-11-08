package webguard

import "testing"

func TestHostAcceptsOnlyThisComputer(t *testing.T) {
	for _, ok := range []string{
		"127.0.0.1", "127.0.0.1:8099", "localhost", "LocalHost:8098", "LOCALHOST", "[::1]:8099", "::1", "[::1]", "127.5.6.7:1", "[::ffff:127.0.0.1]:80",
	} {
		if !Host(ok) {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{
		"", "0.0.0.0:8099", "192.168.0.4:80", "example.com", "127.0.0.1.evil.example", "localhost.evil.example:80", "[::]:80", "10.0.0.1",
		"localhost@evil.example", " localhost", "localhost ", "127.0.0.1:80:80", "::2", "[2001:db8::1]:80", "localhost.", "0x7f.0.0.1",
	} {
		if Host(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestPeerWantsAnAddressWithAPort(t *testing.T) {
	for _, ok := range []string{"127.0.0.1:5000", "[::1]:5000", "127.9.9.9:1", "[::ffff:127.0.0.1]:5"} {
		if !Peer(ok) {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{
		"", "127.0.0.1", "::1", "192.168.0.4:5000", "localhost:5000", "LOCALHOST:5000", "[::]:1", "0.0.0.0:1", "[2001:db8::1]:1", "[::ffff:192.168.0.4]:1", "999.0.0.1:5",
	} {
		if Peer(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}
