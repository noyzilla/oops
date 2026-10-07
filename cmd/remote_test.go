package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRemoteAddArgs(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		expectedName   string
		expectedTarget string
	}{
		{
			name:           "1 argument - SSH alias host name",
			args:           []string{"anthole"},
			expectedName:   "anthole",
			expectedTarget: "anthole",
		},
		{
			name:           "1 argument - user@host target defaults name to oopsbox",
			args:           []string{"captain@anthole.local"},
			expectedName:   "oopsbox",
			expectedTarget: "captain@anthole.local",
		},
		{
			name:           "2 arguments - explicit name and target",
			args:           []string{"staging", "captain@anthole.local"},
			expectedName:   "staging",
			expectedTarget: "captain@anthole.local",
		},
		{
			name:           "2 arguments - explicit name and SSH alias",
			args:           []string{"prod", "anthole"},
			expectedName:   "prod",
			expectedTarget: "anthole",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotTarget := parseRemoteAddArgs(tt.args)
			if gotName != tt.expectedName || gotTarget != tt.expectedTarget {
				t.Errorf("parseRemoteAddArgs(%v) = (%q, %q), want (%q, %q)",
					tt.args, gotName, gotTarget, tt.expectedName, tt.expectedTarget)
			}
		})
	}
}

func TestRequireOopsboxWorkspace_OutsideWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := RequireOopsboxWorkspace(tmpDir)
	if err == nil {
		t.Errorf("RequireOopsboxWorkspace(%q) expected error when outside workspace, got nil", tmpDir)
	}
}

func TestRequireOopsboxWorkspace_ValidWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "stacks"), 0755); err != nil {
		t.Fatalf("failed creating stacks dir: %v", err)
	}

	got, err := RequireOopsboxWorkspace(tmpDir)
	if err != nil {
		t.Errorf("RequireOopsboxWorkspace(%q) unexpected error: %v", tmpDir, err)
	}
	if got == "" {
		t.Errorf("RequireOopsboxWorkspace(%q) returned empty string", tmpDir)
	}
}
