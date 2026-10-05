package orchestrator_test

import (
	"os"
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

func TestParseDurationWithDefault(t *testing.T) {
	tests := []struct {
		input       string
		defaultDur  time.Duration
		expected    time.Duration
	}{
		{"", 30 * time.Second, 30 * time.Second},
		{"45", 30 * time.Second, 45 * time.Second},
		{"45s", 30 * time.Second, 45 * time.Second},
		{"1m", 30 * time.Second, 1 * time.Minute},
		{"10m", 30 * time.Second, 10 * time.Minute},
		{"invalid", 10 * time.Second, 10 * time.Second},
	}

	for _, tt := range tests {
		got := orchestrator.ParseDurationWithDefault(tt.input, tt.defaultDur)
		if got != tt.expected {
			t.Errorf("ParseDurationWithDefault(%q, %v) = %v, expected %v", tt.input, tt.defaultDur, got, tt.expected)
		}
	}
}

func TestGetStopTimeout(t *testing.T) {
	labelsWithTimeout := map[string]string{
		"oops.stop.timeout": "45s",
	}
	if got := orchestrator.GetStopTimeout(labelsWithTimeout); got != 45*time.Second {
		t.Errorf("expected 45s, got %v", got)
	}

	labelsNumeric := map[string]string{
		"oops.stop.timeout": "60",
	}
	if got := orchestrator.GetStopTimeout(labelsNumeric); got != 60*time.Second {
		t.Errorf("expected 60s, got %v", got)
	}

	labelsDefault := map[string]string{}
	if got := orchestrator.GetStopTimeout(labelsDefault); got != 30*time.Second {
		t.Errorf("expected default 30s, got %v", got)
	}
}

func TestFindEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := tmpDir + "/.env"
	_ = os.WriteFile(envPath, []byte("OOPS_SECRET=test-secret"), 0644)

	// Compose file in sub-directory stacks/edge/compose.yml
	stacksEdgeDir := tmpDir + "/stacks/edge"
	_ = os.MkdirAll(stacksEdgeDir, 0755)
	composeFile := stacksEdgeDir + "/compose.yml"
	_ = os.WriteFile(composeFile, []byte("services: {}"), 0644)

	found := orchestrator.FindEnvFile(composeFile)
	if found != envPath {
		t.Errorf("expected FindEnvFile to find %q, got %q", envPath, found)
	}
}
