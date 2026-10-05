package dns_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noyzilla/oops/internal/dns"
)

func TestCollectStaticRecords(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "oops-dns-inspect-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dnsDir := filepath.Join(tempDir, "config", "oops")
	if err := os.MkdirAll(dnsDir, 0755); err != nil {
		t.Fatalf("failed to create dns dir: %v", err)
	}

	dnsContent := `# Static DNS records for oops
host.oops   192.168.64.1
vm.oops     192.168.64.2
.web.oops   172.18.0.2
`
	if err := os.WriteFile(filepath.Join(dnsDir, "dns"), []byte(dnsContent), 0644); err != nil {
		t.Fatalf("failed to write dns file: %v", err)
	}

	records := dns.CollectStaticRecords(tempDir)
	if len(records) != 3 {
		t.Fatalf("expected 3 static records, got %d", len(records))
	}

	if records[0].Hostname != ".web.oops" || records[0].Type != dns.RecordTypeWildcard {
		t.Errorf("unexpected record[0]: %+v", records[0])
	}
	if records[1].Hostname != "host.oops" || records[1].IP != "192.168.64.1" {
		t.Errorf("unexpected record[1]: %+v", records[1])
	}
	if records[2].Hostname != "vm.oops" || records[2].IP != "192.168.64.2" {
		t.Errorf("unexpected record[2]: %+v", records[2])
	}
}

func TestInspectDNSRecordsAndRender(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "oops-dns-report-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	report, err := dns.InspectDNSRecords(context.Background(), tempDir, nil)
	if err != nil {
		t.Fatalf("unexpected error inspecting dns: %v", err)
	}

	out := dns.RenderDNSTable(report)
	if !strings.Contains(out, "OOPS DNS & SERVICE DISCOVERY") {
		t.Errorf("output missing header: %s", out)
	}
	if !strings.Contains(out, "CUSTOM & INFRA DNS RECORDS") {
		t.Errorf("output missing custom & infra records section: %s", out)
	}
}

func TestLookupRecord(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "oops-dns-lookup-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	_, err = dns.SetStaticDNSRecord(tempDir, "myservice.oops", "10.200.0.15")
	if err != nil {
		t.Fatalf("failed to set static dns: %v", err)
	}

	rec, err := dns.LookupRecord(context.Background(), tempDir, "myservice.oops", nil)
	if err != nil {
		t.Fatalf("unexpected error looking up record: %v", err)
	}
	if rec.IP != "10.200.0.15" {
		t.Errorf("expected IP 10.200.0.15, got %s", rec.IP)
	}

	_, err = dns.LookupRecord(context.Background(), tempDir, "nonexistent.oops", nil)
	if err == nil {
		t.Errorf("expected error for nonexistent domain, got nil")
	}
}

func TestCompareIP(t *testing.T) {
	tests := []struct {
		ip1      string
		ip2      string
		expected bool
	}{
		{"172.17.0.2", "172.17.0.10", true},
		{"172.17.0.10", "172.17.0.2", false},
		{"172.17.0.2", "192.168.1.1", true},
		{"192.168.97.2", "192.168.97.10", true},
		{"192.168.97.3", "192.168.97.3", false},
	}

	for _, tt := range tests {
		got := dns.CompareIP(tt.ip1, tt.ip2)
		if got != tt.expected {
			t.Errorf("CompareIP(%q, %q) = %v, expected %v", tt.ip1, tt.ip2, got, tt.expected)
		}
	}
}
