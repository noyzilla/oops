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
	if !strings.Contains(out, "STATIC DNS RECORDS") {
		t.Errorf("output missing static records section: %s", out)
	}
}
