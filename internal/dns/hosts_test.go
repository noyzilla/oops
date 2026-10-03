package dns_test

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/noyzilla/oops/internal/dns"
)

func TestLoadHostsFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oops-hosts-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	hostsFile := filepath.Join(tmpDir, "hosts")
	content := `
# Comment line
192.168.5.2   hostmac hostmac.oops host.docker.internal hostdocker # inline comment
127.0.0.1     .local.dev
10.0.0.5      api.internal

# Invalid line
invalid_ip foo.bar
`
	if err := os.WriteFile(hostsFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write hosts file: %v", err)
	}

	resolver := dns.NewResolver()
	count, err := dns.LoadHostsFile(hostsFile, resolver)
	if err != nil {
		t.Fatalf("unexpected error loading hosts file: %v", err)
	}

	if count != 6 {
		t.Errorf("expected 6 loaded records, got %d", count)
	}

	// Verify exact mappings
	tests := []struct {
		domain   string
		expected net.IP
	}{
		{"hostmac", net.ParseIP("192.168.5.2")},
		{"hostmac.oops", net.ParseIP("192.168.5.2")},
		{"host.docker.internal", net.ParseIP("192.168.5.2")},
		{"hostdocker", net.ParseIP("192.168.5.2")},
		{"api.internal", net.ParseIP("10.0.0.5")},
		// Wildcard match
		{"local.dev", net.ParseIP("127.0.0.1")},
		{"app.local.dev", net.ParseIP("127.0.0.1")},
		{"api.v1.local.dev", net.ParseIP("127.0.0.1")},
	}

	for _, tt := range tests {
		ip, ok := resolver.Resolve(tt.domain)
		if !ok {
			t.Errorf("expected domain %q to resolve, but got false", tt.domain)
			continue
		}
		if !ip.Equal(tt.expected) {
			t.Errorf("domain %q resolved to %v, expected %v", tt.domain, ip, tt.expected)
		}
	}
}

func TestLoadCustomHosts_EnvRecords(t *testing.T) {
	os.Setenv("OOPS_DNS_RECORDS", "colima=192.168.64.1, .test=127.0.0.1")
	defer os.Unsetenv("OOPS_DNS_RECORDS")

	resolver := dns.LoadCustomHosts("/nonexistent")

	if ip, ok := resolver.Resolve("colima"); !ok || !ip.Equal(net.ParseIP("192.168.64.1")) {
		t.Errorf("expected colima to resolve to 192.168.64.1, got %v", ip)
	}

	if ip, ok := resolver.Resolve("my-app.test"); !ok || !ip.Equal(net.ParseIP("127.0.0.1")) {
		t.Errorf("expected my-app.test to resolve to 127.0.0.1, got %v", ip)
	}
}
