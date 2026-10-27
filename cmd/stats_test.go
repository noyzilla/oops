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
		{0, "  0.00 KB"},
		{256 * 1024 * 1024, "256.00 MB"},
		{11.21 * 1024 * 1024, " 11.21 MB"},
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
		{0, "  0.00 KB"},
		{352.26 * 1000, "352.26 KB"},
		{3.29 * 1000 * 1000, "  3.29 MB"},
		{1.73 * 1000 * 1000 * 1000, "  1.73 GB"},
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
			Name:        "oops",
			CPUPerc:     "0.01 %",
			MemLimit:    "128.00 MB",
			MemUsage:    "  5.30 MB",
			MemPerc:     "4.14 %",
			NetRx:       " 27.69 KB",
			NetTx:       " 31.05 KB",
			BlockIn:     "  0.00 KB",
			BlockOut:    "  0.00 KB",
			Pids:        "6",
		},
		{
			ContainerID: "98ba9bf3e428",
			Name:        "caddy-proxy",
			CPUPerc:     "0.38 %",
			MemLimit:    "256.00 MB",
			MemUsage:    " 50.74 MB",
			MemPerc:     "19.82 %",
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

	expectedHeaders := []string{"CONTAINER ID", "NAME", "CPU %", "MEM LIMIT", "MEM USAGE", "MEM %", "NET RX", "NET TX", "BLOCK IN", "BLOCK OUT", "PIDS"}
	for _, h := range expectedHeaders {
		if !strings.Contains(out, h) {
			t.Errorf("renderStatsTable missing header %q in:\n%s", h, out)
		}
	}
	if !strings.Contains(out, "128.00 MB") || !strings.Contains(out, "5.30 MB") || !strings.Contains(out, "27.69 KB") {
		t.Errorf("renderStatsTable missing oops row values in:\n%s", out)
	}
}
