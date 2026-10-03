package dns_test

import (
	"net"
	"testing"

	"github.com/noyzilla/oops/internal/dns"
)

func TestResolver_ExactMatch(t *testing.T) {
	r := dns.NewResolver()
	ip := net.ParseIP("172.20.0.5")

	r.Register("c1", "mail.test", ip)

	// Exact match
	got, ok := r.Resolve("mail.test")
	if !ok || !got.Equal(ip) {
		t.Fatalf("expected %v, got %v (ok=%v)", ip, got, ok)
	}

	// Trailing dot in DNS query
	gotDot, okDot := r.Resolve("mail.test.")
	if !okDot || !gotDot.Equal(ip) {
		t.Fatalf("expected %v with trailing dot, got %v (ok=%v)", ip, gotDot, okDot)
	}

	// Case insensitive
	gotCase, okCase := r.Resolve("MAIL.TEST")
	if !okCase || !gotCase.Equal(ip) {
		t.Fatalf("expected case insensitive match, got %v (ok=%v)", gotCase, okCase)
	}

	// Non-matching
	_, okOther := r.Resolve("other.test")
	if okOther {
		t.Fatal("expected other.test to not resolve")
	}
}

func TestResolver_WildcardMatch(t *testing.T) {
	r := dns.NewResolver()
	caddyIP := net.ParseIP("172.20.0.2")

	// Register wildcard hostname .web.oops
	r.Register("caddy", ".web.oops", caddyIP)

	tests := []struct {
		query    string
		expected bool
	}{
		{"web.oops", true},            // Apex
		{"app.web.oops", true},        // 1-level subdomain
		{"api.web.oops", true},        // 1-level subdomain
		{"deep.sub.web.oops", true},   // multi-level subdomain
		{"WEB.OOPS", true},            // case-insensitive apex
		{"APP.WEB.OOPS", true},        // case-insensitive subdomain
		{"oops", false},               // Parent domain without suffix
		{"notweb.oops", false},        // Suffix mismatch
		{"web.other", false},          // Suffix mismatch
	}

	for _, tt := range tests {
		got, ok := r.Resolve(tt.query)
		if ok != tt.expected {
			t.Errorf("Resolve(%q) ok = %v, expected %v", tt.query, ok, tt.expected)
		}
		if ok && !got.Equal(caddyIP) {
			t.Errorf("Resolve(%q) IP = %v, expected %v", tt.query, got, caddyIP)
		}
	}
}

func TestResolver_Unregister(t *testing.T) {
	r := dns.NewResolver()
	ip := net.ParseIP("172.20.0.10")

	r.Register("c1", "db.internal", ip)
	r.Register("c1", ".cluster.local", ip)

	if _, ok := r.Resolve("db.internal"); !ok {
		t.Fatal("expected db.internal to resolve before unregister")
	}
	if _, ok := r.Resolve("node1.cluster.local"); !ok {
		t.Fatal("expected node1.cluster.local to resolve before unregister")
	}

	r.Unregister("c1")

	if _, ok := r.Resolve("db.internal"); ok {
		t.Fatal("expected db.internal to NOT resolve after unregister")
	}
	if _, ok := r.Resolve("node1.cluster.local"); ok {
		t.Fatal("expected node1.cluster.local to NOT resolve after unregister")
	}
}
