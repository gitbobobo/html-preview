package config

import (
	"net"
	"strings"
	"testing"
)

func TestLANURLsAllInterfaces(t *testing.T) {
	cfg := Config{Host: "0.0.0.0", Port: 7849}
	urls := cfg.LANURLs()
	for _, u := range urls {
		if !strings.HasPrefix(u, "http://") {
			t.Fatalf("bad url %q", u)
		}
		if strings.Contains(u, "127.0.0.1") {
			t.Fatalf("loopback should not appear: %q", u)
		}
		hostPort := strings.TrimPrefix(u, "http://")
		host, _, err := net.SplitHostPort(hostPort)
		if err != nil {
			// port always present as :7849
			t.Fatalf("SplitHostPort %q: %v", hostPort, err)
		}
		ip := net.ParseIP(host)
		if ip == nil || ip.IsLoopback() {
			t.Fatalf("expected non-loopback ip in %q", u)
		}
	}
}

func TestLANURLsSpecificHost(t *testing.T) {
	cfg := Config{Host: "192.168.1.10", Port: 7849}
	urls := cfg.LANURLs()
	if len(urls) != 1 || urls[0] != "http://192.168.1.10:7849" {
		t.Fatalf("got %#v", urls)
	}
}

func TestLANURLsLoopbackOnly(t *testing.T) {
	cfg := Config{Host: "127.0.0.1", Port: 7849}
	if urls := cfg.LANURLs(); len(urls) != 0 {
		t.Fatalf("expected empty, got %#v", urls)
	}
}
