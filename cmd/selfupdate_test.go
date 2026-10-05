package cmd

import (
	"testing"
)

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input    string
		expected [3]int
	}{
		{"0.12.0", [3]int{0, 12, 0}},
		{"v0.12.1", [3]int{0, 12, 1}},
		{"1.0.0-rc1", [3]int{1, 0, 0}},
		{"2.5.99", [3]int{2, 5, 99}},
	}

	for _, tt := range tests {
		got := parseSemver(tt.input)
		if got != tt.expected {
			t.Errorf("parseSemver(%q) = %v, expected %v", tt.input, got, tt.expected)
		}
	}
}

func TestIsVersionNewer(t *testing.T) {
	tests := []struct {
		current  string
		latest   string
		expected bool
	}{
		{"0.12.0", "0.12.1", true},
		{"0.12.0", "0.13.0", true},
		{"0.12.0", "1.0.0", true},
		{"0.12.1", "0.12.1", false},
		{"0.12.1", "0.12.0", false},
		{"dev", "0.12.0", true},
		{"dev-1a2b3c4", "0.12.0", true},
	}

	for _, tt := range tests {
		got := isVersionNewer(tt.current, tt.latest)
		if got != tt.expected {
			t.Errorf("isVersionNewer(%q, %q) = %v, expected %v", tt.current, tt.latest, got, tt.expected)
		}
	}
}
