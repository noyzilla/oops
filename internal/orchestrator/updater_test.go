package orchestrator_test

import (
	"testing"
	"time"

	"github.com/noyzilla/oops/internal/orchestrator"
)

func TestParseDelay(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		hasError bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"0s", 0, false},
		{"5", 5 * time.Second, false},
		{"10s", 10 * time.Second, false},
		{"1m", 1 * time.Minute, false},
		{"2m30s", 150 * time.Second, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		got, err := orchestrator.ParseDelay(tt.input)
		if (err != nil) != tt.hasError {
			t.Errorf("ParseDelay(%q) error = %v, expected error = %v", tt.input, err, tt.hasError)
		}
		if got != tt.expected {
			t.Errorf("ParseDelay(%q) = %v, expected %v", tt.input, got, tt.expected)
		}
	}
}

func TestGetStopTimeout(t *testing.T) {
	labelsWithTimeout := map[string]string{
		"oops.stop.timeout": "45",
	}
	if got := orchestrator.GetStopTimeout(labelsWithTimeout); got != 45 {
		t.Errorf("expected 45, got %d", got)
	}

	labelsDefault := map[string]string{}
	if got := orchestrator.GetStopTimeout(labelsDefault); got != 30 {
		t.Errorf("expected default 30, got %d", got)
	}
}
