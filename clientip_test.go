package main

import (
	"net"
	"net/http"
	"testing"
)

func mustParseCIDR(s string) *net.IPNet {
	_, cidr, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return cidr
}

func TestClientIP_NoTrustedProxies(t *testing.T) {
	app := &App{}
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")

	got := app.clientIP(req)
	if got != "1.2.3.4" {
		t.Errorf("clientIP = %q, want %q (header should be ignored)", got, "1.2.3.4")
	}
}

func TestClientIP_TrustedProxy_XFF(t *testing.T) {
	app := &App{
		trustedProxies: []*net.IPNet{
			mustParseCIDR("10.0.0.0/8"),
		},
	}
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.2")

	got := app.clientIP(req)
	if got != "203.0.113.7" {
		t.Errorf("clientIP = %q, want %q", got, "203.0.113.7")
	}
}

func TestClientIP_UntrustedPeer_SpoofedXFF(t *testing.T) {
	app := &App{
		trustedProxies: []*net.IPNet{
			mustParseCIDR("10.0.0.0/8"),
		},
	}
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "5.6.7.8:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.2")

	got := app.clientIP(req)
	if got != "5.6.7.8" {
		t.Errorf("clientIP = %q, want %q (spoofed header should be ignored)", got, "5.6.7.8")
	}
}

func TestClientIP_AllXFFTrusted_FallbackToPeer(t *testing.T) {
	app := &App{
		trustedProxies: []*net.IPNet{
			mustParseCIDR("10.0.0.0/8"),
			mustParseCIDR("172.16.0.0/12"),
		},
	}
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "10.0.0.5, 172.16.0.3")

	got := app.clientIP(req)
	if got != "10.0.0.1" {
		t.Errorf("clientIP = %q, want %q (all XFF trusted, should fall back to peer)", got, "10.0.0.1")
	}
}

func TestClientIP_EmptyXFF_FallbackToPeer(t *testing.T) {
	app := &App{
		trustedProxies: []*net.IPNet{
			mustParseCIDR("10.0.0.0/8"),
		},
	}
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	got := app.clientIP(req)
	if got != "10.0.0.1" {
		t.Errorf("clientIP = %q, want %q (no XFF header, should fall back to peer)", got, "10.0.0.1")
	}
}

func TestClientIP_BareIPTrustedProxy(t *testing.T) {
	// Test that a bare IP (not CIDR) works as trusted proxy
	app := &App{
		trustedProxies: parseTrustedProxies("192.168.1.5"),
	}
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.5:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.50")

	got := app.clientIP(req)
	if got != "203.0.113.50" {
		t.Errorf("clientIP = %q, want %q", got, "203.0.113.50")
	}
}

func TestParseTrustedProxies(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"single IP", "10.0.0.1", 1},
		{"single CIDR", "10.0.0.0/8", 1},
		{"mixed", "10.0.0.0/8, 192.168.1.5", 2},
		{"with spaces", " 10.0.0.0/8 , 192.168.1.5 ", 2},
		{"invalid entry skipped", "10.0.0.0/8, notanip, 192.168.1.5", 2},
		{"empty entries skipped", "10.0.0.0/8,,192.168.1.5", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTrustedProxies(tt.input)
			if len(got) != tt.want {
				t.Errorf("parseTrustedProxies(%q) returned %d entries, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}

func TestClientIP_XFFWithUnparseableEntries(t *testing.T) {
	app := &App{
		trustedProxies: []*net.IPNet{
			mustParseCIDR("10.0.0.0/8"),
		},
	}
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.7, not-an-ip, 10.0.0.2")

	// Walking right-to-left: 10.0.0.2 is trusted, "not-an-ip" is skipped, 203.0.113.7 is untrusted → returned
	got := app.clientIP(req)
	if got != "203.0.113.7" {
		t.Errorf("clientIP = %q, want %q", got, "203.0.113.7")
	}
}
