package dns_test

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/noyzilla/oops/internal/dns"
)

func TestParseDNSFile_StandardFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oops-dns-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dnsFile := filepath.Join(tmpDir, "dns")
	content := `
# Oops Static DNS Records
host.oops       192.168.64.1
vm.oops         192.168.64.2
.web.oops       172.18.0.2
xxx.com         127.0.0.1
.wildcard.test  10.0.0.100

# Invalid lines
broken.domain   invalid_ip
incomplete_line
`
	if err := os.WriteFile(dnsFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write dns file: %v", err)
	}

	records, warnings, err := dns.ParseDNSFile(dnsFile)
	if err != nil {
		t.Fatalf("unexpected error parsing dns file: %v", err)
	}

	if len(warnings) != 1 {
		t.Errorf("expected 1 warning for invalid IP, got %d: %v", len(warnings), warnings)
	}

	if len(records) != 5 {
		t.Fatalf("expected 5 valid records, got %d", len(records))
	}

	resolver := dns.NewResolver()
	resolver.SwapStatic(records)

	// Verify exact matches
	if ip, ok := resolver.Resolve("host.oops"); !ok || !ip.Equal(net.ParseIP("192.168.64.1")) {
		t.Errorf("expected host.oops -> 192.168.64.1, got %v", ip)
	}
	if ip, ok := resolver.Resolve("xxx.com"); !ok || !ip.Equal(net.ParseIP("127.0.0.1")) {
		t.Errorf("expected xxx.com -> 127.0.0.1, got %v", ip)
	}

	// Verify wildcard matches
	if ip, ok := resolver.Resolve("web.oops"); !ok || !ip.Equal(net.ParseIP("172.18.0.2")) {
		t.Errorf("expected apex web.oops -> 172.18.0.2, got %v", ip)
	}
	if ip, ok := resolver.Resolve("app.web.oops"); !ok || !ip.Equal(net.ParseIP("172.18.0.2")) {
		t.Errorf("expected sub app.web.oops -> 172.18.0.2, got %v", ip)
	}
	if ip, ok := resolver.Resolve("sub.wildcard.test"); !ok || !ip.Equal(net.ParseIP("10.0.0.100")) {
		t.Errorf("expected sub.wildcard.test -> 10.0.0.100, got %v", ip)
	}
}

func TestAddAndDeleteStaticDNSRecord(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oops-dns-crud-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Add record 1
	p, err := dns.AddStaticDNSRecord(tmpDir, "mysite.test", "127.0.0.1")
	if err != nil {
		t.Fatalf("failed to add record: %v", err)
	}

	// Add wildcard record 2
	_, err = dns.AddStaticDNSRecord(tmpDir, ".api.test", "10.0.0.50")
	if err != nil {
		t.Fatalf("failed to add wildcard record: %v", err)
	}

	resolver := dns.NewResolver()
	count, warnings, err := dns.ReloadStaticRecords(tmpDir, resolver)
	if err != nil || len(warnings) > 0 {
		t.Fatalf("failed to reload records: %v (warnings: %v)", err, warnings)
	}
	if count != 2 {
		t.Errorf("expected 2 records, got %d", count)
	}

	if ip, ok := resolver.Resolve("mysite.test"); !ok || !ip.Equal(net.ParseIP("127.0.0.1")) {
		t.Errorf("mysite.test failed to resolve to 127.0.0.1, got %v", ip)
	}
	if ip, ok := resolver.Resolve("v1.api.test"); !ok || !ip.Equal(net.ParseIP("10.0.0.50")) {
		t.Errorf("v1.api.test failed to resolve to 10.0.0.50, got %v", ip)
	}

	// Update existing record
	_, err = dns.AddStaticDNSRecord(tmpDir, "mysite.test", "192.168.1.10")
	if err != nil {
		t.Fatalf("failed to update record: %v", err)
	}

	dns.ReloadStaticRecords(tmpDir, resolver)
	if ip, ok := resolver.Resolve("mysite.test"); !ok || !ip.Equal(net.ParseIP("192.168.1.10")) {
		t.Errorf("updated mysite.test failed to resolve to 192.168.1.10, got %v", ip)
	}

	// Delete record
	_, err = dns.DeleteStaticDNSRecord(tmpDir, "mysite.test")
	if err != nil {
		t.Fatalf("failed to delete record: %v", err)
	}

	dns.ReloadStaticRecords(tmpDir, resolver)
	if _, ok := resolver.Resolve("mysite.test"); ok {
		t.Errorf("expected mysite.test to be deleted, but still resolved")
	}

	// Delete non-existent record should return error
	_, err = dns.DeleteStaticDNSRecord(tmpDir, "nonexistent.domain")
	if err == nil {
		t.Errorf("expected error deleting non-existent record, got nil")
	}

	_ = p
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
