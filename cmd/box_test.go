package cmd

import (
	"bytes"
	"testing"
)

func TestBoxSubcommandsHelp(t *testing.T) {
	cmd := newBoxCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Help(); err != nil {
		t.Fatalf("unexpected error running help: %v", err)
	}

	helpText := out.String()
	expectedSubcommands := []string{"init", "clone", "active", "list", "start", "stop", "switch", "cert"}
	for _, sub := range expectedSubcommands {
		if !bytes.Contains(out.Bytes(), []byte(sub)) {
			t.Errorf("expected subcommand %q in help text, got:\n%s", sub, helpText)
		}
	}
}

func TestBoxActiveCommandNoArgs(t *testing.T) {
	cmd := newBoxActiveCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error executing active command: %v", err)
	}
}

func TestDeriveCloneTargetPath(t *testing.T) {
	tests := []struct {
		sshTarget string
		arg       string
		expected  string
	}{
		{"user@prod.example.com", "", "prod.example.com"},
		{"user@server:~/oopsbox", "my-app", "my-app"},
		{"ssh://user@1.2.3.4:22/repo", "", "1.2.3.4"},
		{"alias-name", "", "alias-name"},
	}

	for _, tt := range tests {
		got := deriveCloneTargetPath(tt.sshTarget, tt.arg)
		if got != tt.expected {
			t.Errorf("deriveCloneTargetPath(%q, %q) = %q; want %q", tt.sshTarget, tt.arg, got, tt.expected)
		}
	}
}
