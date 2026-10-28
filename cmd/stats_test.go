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

func TestFormatDualMetric(t *testing.T) {
	testCases := []struct {
		leftVal     float64
		isLeftValid bool
		rightVal    float64
		expected    string
	}{
		{15.664, true, 0.587, "  15.664 / 0.587"},
		{1.500, true, 0.500, "   1.500 / 0.500"},
		{0.0, false, 0.050, "       - / 0.050"},
		{2500.0, true, 120.340, "2500.000 / 120.340"},
	}

	for _, tc := range testCases {
		res := formatDualMetric(tc.leftVal, tc.isLeftValid, tc.rightVal)
		if res[9] != '/' {
			t.Errorf("expected '/' at index 9 for %q, got %c", res, res[9])
		}
		if res != tc.expected {
			t.Errorf("formatDualMetric(%v, %v, %v) = %q, expected %q", tc.leftVal, tc.isLeftValid, tc.rightVal, res, tc.expected)
		}
	}
}

func TestRenderCompactStatsTable_SlashAlignment(t *testing.T) {
	items := []containerStatsItem{
		{
			ContainerID: "9937c2d40509",
			Stack:       "/db",
			Name:        "mysql",
			CPUPercent:  0.50,
			MemPercent:  3.72,
			CPUPerc:     "0.50 %",
			MemPerc:     "3.72 %",
			MemLimitGB:  15.66,
			MemUsageGB:  0.58,
			NetRxGB:     1.36,
			NetTxGB:     0.36,
			BlockInGB:   35.78,
			BlockOutGB:  1.75,
			Pids:        "44",
		},
		{
			ContainerID: "ce08bbd2fd3d",
			Stack:       "/edge",
			Name:        "caddy-proxy",
			CPUPercent:  0.03,
			MemPercent:  8.76,
			CPUPerc:     "0.03 %",
			MemPerc:     "8.76 %",
			MemLimitGB:  0.25,
			MemUsageGB:  0.02,
			NetRxGB:     0.00,
			NetTxGB:     0.00,
			BlockInGB:   6.55,
			BlockOutGB:  0.00,
			Pids:        "15",
		},
		{
			ContainerID: "ad786bc2078a",
			Stack:       "/tools",
			Name:        "httpbin",
			CPUPercent:  0.00,
			MemPercent:  0.01,
			CPUPerc:     "0.00 %",
			MemPerc:     "0.01 %",
			MemLimit:    "-",
			MemUsageGB:  0.00,
			NetRxGB:     0.00,
			NetTxGB:     0.00,
			BlockInGB:   0.01,
			BlockOutGB:  0.00,
			Pids:        "7",
		},
	}

	var buf bytes.Buffer
	renderCompactStatsTable(&buf, items)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")

	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d:\n%s", len(lines), buf.String())
	}

	header := lines[0]
	expectedHeaders := []string{"CONTAINER ID", "STACK", "NAME", "CPU%", "MEM%", "MEM LM / US (GB)", "NET I/O (GB)", "BLOCK I/O (GB)", "PIDS"}
	for _, h := range expectedHeaders {
		if !strings.Contains(header, h) {
			t.Errorf("compact header missing %q in:\n%s", h, header)
		}
	}

	// Verify CPU% and MEM% data values have no space before % (e.g. "0.50%", "3.72%")
	if !strings.Contains(lines[1], "0.50%") || !strings.Contains(lines[1], "3.72%") {
		t.Errorf("expected compact row to have percentage without space (0.50%% and 3.72%%), got:\n%s", lines[1])
	}

	// Verify '/' alignment for MEM, NET, and BLOCK columns
	var headerSlashPositions []int
	for idx, ch := range header {
		if ch == '/' {
			headerSlashPositions = append(headerSlashPositions, idx)
		}
	}

	if len(headerSlashPositions) != 3 {
		t.Fatalf("expected 3 slashes in compact header, got %d in: %q", len(headerSlashPositions), header)
	}

	for lineIdx, line := range lines[1:] {
		var rowSlashPositions []int
		for idx, ch := range line {
			if ch == '/' {
				rowSlashPositions = append(rowSlashPositions, idx)
			}
		}

		if len(rowSlashPositions) < 3 {
			t.Fatalf("row %d has fewer than 3 slashes in: %q", lineIdx+1, line)
		}

		metricSlashes := rowSlashPositions[len(rowSlashPositions)-3:]
		for k := 0; k < 3; k++ {
			if metricSlashes[k] != headerSlashPositions[k] {
				t.Errorf("row %d slash #%d position mismatch: got index %d, header index %d in line:\n%s",
					lineIdx+1, k+1, metricSlashes[k], headerSlashPositions[k], line)
			}
		}
	}
}
