package cmd_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/noyzilla/oops/cmd"
)

func TestRootCommandSubcommands(t *testing.T) {
	rootCmd := cmd.GetRootCommand()

	expectedSubcommands := []string{
		"server",
		"up",
		"stop",
		"restart",
		"down",
		"status",
		"logs",
		"pull",
		"update",
		"db",
		"db-backup",
	}

	for _, sub := range expectedSubcommands {
		found := false
		for _, c := range rootCmd.Commands() {
			if c.Name() == sub || containsAlias(c.Aliases, sub) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand %q not found in root command", sub)
		}
	}
}

func TestSubcommandFlags(t *testing.T) {
	rootCmd := cmd.GetRootCommand()

	tests := []struct {
		cmdName  string
		flagName string
		shorthand string
	}{
		{"up", "delay", "d"},
		{"stop", "delay", "d"},
		{"restart", "delay", "d"},
		{"down", "delay", "d"},
		{"update", "delay", "d"},
		{"server", "port", "p"},
		{"server", "config", "c"},
		{"logs", "tail", "t"},
		{"logs", "follow", "f"},
		{"db-backup", "retention", "r"},
	}

	for _, tt := range tests {
		c, _, err := rootCmd.Find([]string{tt.cmdName})
		if err != nil || c == nil {
			t.Fatalf("could not find command %q: %v", tt.cmdName, err)
		}

		f := c.Flags().Lookup(tt.flagName)
		if f == nil {
			t.Errorf("command %q missing expected flag %q", tt.cmdName, tt.flagName)
			continue
		}
		if f.Shorthand != tt.shorthand {
			t.Errorf("command %q flag %q expected shorthand %q, got %q", tt.cmdName, tt.flagName, tt.shorthand, f.Shorthand)
		}
	}
}

func TestHelpOutput(t *testing.T) {
	rootCmd := cmd.GetRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing help: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "managing multi-stack Docker Compose deployments") {
		t.Errorf("help output missing expected description: %s", out)
	}
}

func containsAlias(aliases []string, name string) bool {
	for _, a := range aliases {
		if a == name {
			return true
		}
	}
	return false
}
