package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatBinary(t *testing.T) {
	tests := []struct {
		bytes    float64
		expected string
	}{
		{0, "  0.00 GB"},
		{256 * 1024 * 1024, "  0.25 GB"},
		{11.21 * 1024 * 1024, "  0.01 GB"},
		{2.44 * 1024 * 1024 * 1024, "  2.44 GB"},
	}

	for _, tt := range tests {
		actual := formatBinary(tt.bytes)
		if actual != tt.expected {
			t.Errorf("formatBinary(%v) = %q, expected %q", tt.bytes, actual, tt.expected)
		}
	}
}

func TestFormatDecimal(t *testing.T) {
	tests := []struct {
		bytes    float64
		expected string
	}{
		{0, "  0.00 GB"},
		{352.26 * 1000, "  0.00 GB"},
		{350 * 1000 * 1000, "  0.35 GB"},
		{1.73 * 1000 * 1000 * 1000, "  1.73 GB"},
		{2500.0 * 1000 * 1000 * 1000, "  2.50 TB"},
	}

	for _, tt := range tests {
		actual := formatDecimal(tt.bytes)
		if actual != tt.expected {
			t.Errorf("formatDecimal(%v) = %q, expected %q", tt.bytes, actual, tt.expected)
		}
	}
}

func TestRenderStatsTableSeparated(t *testing.T) {
	items := []containerStatsItem{
		{
			ContainerID: "ddadb950dd83",
			Stack:       "/edge",
			Name:        "oops",
			CPUPerc:     "0.01 %",
			MemPerc:     "4.14 %",
			MemLimit:    "128.00 MB",
			MemUsage:    "  5.30 MB",
			NetRx:       " 27.69 KB",
			NetTx:       " 31.05 KB",
			BlockIn:     "  0.00 KB",
			BlockOut:    "  0.00 KB",
			Pids:        "6",
		},
		{
			ContainerID: "98ba9bf3e428",
			Stack:       "/edge",
			Name:        "caddy-proxy",
			CPUPerc:     "0.38 %",
			MemPerc:     "19.82 %",
			MemLimit:    "256.00 MB",
			MemUsage:    " 50.74 MB",
			NetRx:       "199.90 MB",
			NetTx:       "217.70 MB",
			BlockIn:     "180.22 KB",
			BlockOut:    "  8.19 KB",
			Pids:        "10",
		},
	}

	var buf bytes.Buffer
	renderStatsTable(&buf, items)
	out := buf.String()

	expectedHeaders := []string{"CONTAINER ID", "STACK", "NAME", "CPU %", "MEM %", "MEM LM", "MEM US", "NET RX", "NET TX", "BLOCK RD", "BLOCK WR", "PIDS"}
	for _, h := range expectedHeaders {
		if !strings.Contains(out, h) {
			t.Errorf("renderStatsTable missing header %q in:\n%s", h, out)
		}
	}
	if !strings.Contains(out, "/edge") || !strings.Contains(out, "128.00 MB") || !strings.Contains(out, "5.30 MB") || !strings.Contains(out, "27.69 KB") {
		t.Errorf("renderStatsTable missing oops row values in:\n%s", out)
	}

	// Verify column order: STACK before NAME, MEM % before MEM LM
	stackIdx := strings.Index(out, "STACK")
	nameIdx := strings.Index(out, "NAME")
	cpuIdx := strings.Index(out, "CPU %")
	memPercIdx := strings.Index(out, "MEM %")
	memLmIdx := strings.Index(out, "MEM LM")

	if !(stackIdx < nameIdx && nameIdx < cpuIdx && cpuIdx < memPercIdx && memPercIdx < memLmIdx) {
		t.Errorf("header column order unexpected in:\n%s", out)
	}
}

func TestStatsStackSorting(t *testing.T) {
	items := []containerStatsItem{
		{ContainerID: "1", Stack: "%", Name: "standalone"},
		{ContainerID: "2", Stack: "/tools", Name: "mailpit"},
		{ContainerID: "3", Stack: "/", Name: "app"},
		{ContainerID: "4", Stack: "/db", Name: "redis"},
		{ContainerID: "5", Stack: "/tools", Name: "httpbin"},
		{ContainerID: "6", Stack: "/db", Name: "mysql"},
	}

	sortStatsItems(items)

	expectedOrder := []string{"app", "mysql", "redis", "httpbin", "mailpit", "standalone"}
	for i, it := range items {
		if it.Name != expectedOrder[i] {
			t.Errorf("item at index %d = %q, expected %q", i, it.Name, expectedOrder[i])
		}
	}
}
