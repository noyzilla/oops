package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestPadUnderscores(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  2.44", "__2.44"},
		{" 17.93", "_17.93"},
		{"256.00", "256.00"},
		{" 0.23%", "_0.23%"},
		{"10.00%", "10.00%"},
	}

	for _, tt := range tests {
		actual := padUnderscores(tt.input)
		if actual != tt.expected {
			t.Errorf("padUnderscores(%q) = %q, expected %q", tt.input, actual, tt.expected)
		}
	}
}

func TestFormatBinaryFixedWithUnderscores(t *testing.T) {
	tests := []struct {
		bytes    float64
		expected string
	}{
		{0, "__0.00B  "},
		{256 * 1024 * 1024, "256.00MiB"},
		{17.93 * 1024 * 1024, "_17.93MiB"},
		{2.44 * 1024 * 1024 * 1024, "__2.44GiB"},
	}

	for _, tt := range tests {
		actual := formatBinaryFixed(tt.bytes)
		if len(actual) != 9 {
			t.Errorf("formatBinaryFixed(%v) width = %d, expected 9", tt.bytes, len(actual))
		}
		if actual != tt.expected {
			t.Errorf("formatBinaryFixed(%v) = %q, expected %q", tt.bytes, actual, tt.expected)
		}
	}
}

func TestRenderStatsTableWithUnderscores(t *testing.T) {
	items := []containerStatsItem{
		{
			ContainerID: "ddadb950dd83",
			Name:        "oops",
			CPUPerc:     "_0.01%",
			MemLimitUse: "128.00MiB / __5.32MiB",
			MemPerc:     "_4.15%",
			NetIO:       "_20.04kB / _22.26kB",
			BlockIO:     "__0.00B  / __0.00B ",
			Pids:        "6",
		},
		{
			ContainerID: "98ba9bf3e428",
			Name:        "caddy-proxy",
			CPUPerc:     "_0.45%",
			MemLimitUse: "256.00MiB / _55.33MiB",
			MemPerc:     "21.61%",
			NetIO:       "149.60MB / 162.50MB",
			BlockIO:     "180.20kB / __8.19kB",
			Pids:        "10",
		},
	}

	var buf bytes.Buffer
	renderStatsTable(&buf, items)
	out := buf.String()

	if !strings.Contains(out, "MEM (LIMIT / USAGE)") {
		t.Errorf("renderStatsTable header missing MEM (LIMIT / USAGE): \n%s", out)
	}
	if !strings.Contains(out, "128.00MiB / __5.32MiB") {
		t.Errorf("renderStatsTable missing oops row data: \n%s", out)
	}
}
