package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestTunnelCommand_Help(t *testing.T) {
	cmd := newTunnelCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error on --help, got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "tunnel [@remote] [services...]") {
		t.Errorf("expected usage string in help output, got: %s", out)
	}
	if !strings.Contains(out, "up") || !strings.Contains(out, "down") || !strings.Contains(out, "list") {
		t.Errorf("expected subcommands in help output, got: %s", out)
	}
}

func TestTunnelCommand_Registration(t *testing.T) {
	root := GetRootCommand()
	found := false
	for _, c := range root.Commands() {
		if c.Name() == "tunnel" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected 'tunnel' command to be registered in rootCmd")
	}
}

func TestTunnelUpCommand_ArgsValidation(t *testing.T) {
	upCmd := newTunnelUpCmd()
	upCmd.SetArgs([]string{}) // Missing @remote
	err := upCmd.Execute()
	if err == nil {
		t.Errorf("expected error when @remote argument is missing, got nil")
	}
}

func TestTunnelDownCommand_ArgsValidation(t *testing.T) {
	downCmd := newTunnelDownCmd()
	downCmd.SetArgs([]string{}) // Missing @remote
	err := downCmd.Execute()
	if err == nil {
		t.Errorf("expected error when @remote argument is missing, got nil")
	}
}

