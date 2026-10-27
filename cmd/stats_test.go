package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestCalculateCPUPercent(t *testing.T) {
	stats := &container.StatsResponse{
		CPUStats: container.CPUStats{
			CPUUsage: container.CPUUsage{
				TotalUsage: 200000000,
			},
			SystemUsage: 2000000000,
			OnlineCPUs:  2,
		},
		PreCPUStats: container.CPUStats{
			CPUUsage: container.CPUUsage{
				TotalUsage: 100000000,
			},
			SystemUsage: 1000000000,
		},
	}

	perc := calculateCPUPercent(stats, nil)
	expected := 20.0
	if perc != expected {
		t.Errorf("calculateCPUPercent() = %v, expected %v", perc, expected)
	}

	prev := &cpuSample{
		totalUsage:  150000000,
		systemUsage: 1500000000,
	}
	percPrev := calculateCPUPercent(stats, prev)
	if percPrev != expected {
		t.Errorf("calculateCPUPercent() with prev = %v, expected %v", percPrev, expected)
	}
}

func TestCalculateMemUsage(t *testing.T) {
	mem := container.MemoryStats{
		Usage: 104857600, // 100 MiB
		Limit: 209715200, // 200 MiB
		Stats: map[string]uint64{
			"inactive_file": 4194304, // 4 MiB cache
		},
	}

	usage, limit, perc := calculateMemUsage(mem)
	expectedUsage := float64(104857600 - 4194304) // 96 MiB
	expectedLimit := float64(209715200)           // 200 MiB
	expectedPerc := (expectedUsage / expectedLimit) * 100.0

	if usage != expectedUsage {
		t.Errorf("calculateMemUsage() usage = %v, expected %v", usage, expectedUsage)
	}
	if limit != expectedLimit {
		t.Errorf("calculateMemUsage() limit = %v, expected %v", limit, expectedLimit)
	}
	if perc != expectedPerc {
		t.Errorf("calculateMemUsage() perc = %v, expected %v", perc, expectedPerc)
	}
}

func TestFormatBinaryFixed(t *testing.T) {
	tests := []struct {
		bytes    float64
		expected string
	}{
		{0, "  0.00B  "},
		{128 * 1024 * 1024, "128.00MiB"},
		{5.258 * 1024 * 1024, "  5.26MiB"},
		{1.333 * 1024 * 1024 * 1024, "  1.33GiB"},
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

func TestFormatDecimalFixed(t *testing.T) {
	tests := []struct {
		bytes    float64
		expected string
	}{
		{0, "  0.00B "},
		{11600, " 11.60kB"},
		{149600000, "149.60MB"},
	}

	for _, tt := range tests {
		actual := formatDecimalFixed(tt.bytes)
		if len(actual) != 8 {
			t.Errorf("formatDecimalFixed(%v) width = %d, expected 8", tt.bytes, len(actual))
		}
		if actual != tt.expected {
			t.Errorf("formatDecimalFixed(%v) = %q, expected %q", tt.bytes, actual, tt.expected)
		}
	}
}

func TestRenderStatsTable(t *testing.T) {
	items := []containerStatsItem{
		{
			ContainerID: "ddadb950dd83",
			Name:        "oops",
			CPUPerc:     "  0.01%",
			MemLimitUse: "128.00MiB /   5.32MiB",
			MemPerc:     "  4.15%",
			NetIO:       " 20.04kB /  22.26kB",
			BlockIO:     "  0.00B  /   0.00B ",
			Pids:        "    6",
		},
		{
			ContainerID: "98ba9bf3e428",
			Name:        "caddy-proxy",
			CPUPerc:     "  0.45%",
			MemLimitUse: "256.00MiB /  55.33MiB",
			MemPerc:     " 21.61%",
			NetIO:       "149.60MB / 162.50MB",
			BlockIO:     "180.20kB /   8.19kB",
			Pids:        "   10",
		},
	}

	var buf bytes.Buffer
	renderStatsTable(&buf, items)
	out := buf.String()

	if !strings.Contains(out, "MEM (LIMIT / USAGE)") {
		t.Errorf("renderStatsTable header missing MEM (LIMIT / USAGE): \n%s", out)
	}
	if !strings.Contains(out, "128.00MiB /   5.32MiB") {
		t.Errorf("renderStatsTable missing oops row data: \n%s", out)
	}
}
