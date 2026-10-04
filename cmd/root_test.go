package cmd_test

import (
	"bytes"
	"os"
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
		"switch",
		"db",
		"backup",
		"backup-db",
		"backup-data",
		"restore",
		"restore-db",
		"restore-data",
		"dns",
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
		{"stop", "except", "x"},
		{"restart", "delay", "d"},
		{"down", "delay", "d"},
		{"update", "delay", "d"},
		{"switch", "delay", "d"},
		{"server", "port", "p"},
		{"server", "config", "c"},
		{"logs", "tail", "t"},
		{"logs", "follow", "f"},
		{"backup", "retention", "r"},
		{"backup-db", "retention", "r"},
		{"backup-data", "retention", "r"},
		{"restore-db", "yes", "y"},
		{"restore-data", "yes", "y"},
		{"restore-data", "dry-run", "n"},
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

	// Verify prune subcommands exist
	for _, parent := range []string{"backup", "backup-db", "backup-data"} {
		pruneCmd, _, err := rootCmd.Find([]string{parent, "prune"})
		if err != nil || pruneCmd == nil || pruneCmd.Name() != "prune" {
			t.Errorf("expected %s prune subcommand to exist", parent)
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

func TestResolveWorkDir(t *testing.T) {
	// 1. Explicit Custom Dir
	custom := "/tmp/my-oopsbox"
	if got := cmd.ResolveWorkDir(custom); got != custom {
		t.Errorf("ResolveWorkDir(%q) = %q, expected %q", custom, got, custom)
	}

	// 2. OOPSBOX_DIR env
	os.Setenv("OOPSBOX_DIR", "/tmp/env-oopsbox")
	if got := cmd.ResolveWorkDir(""); got != "/tmp/env-oopsbox" {
		t.Errorf("ResolveWorkDir with OOPSBOX_DIR = %q, expected /tmp/env-oopsbox", got)
	}
	os.Unsetenv("OOPSBOX_DIR")

	// 3. OOPS_DIR env
	os.Setenv("OOPS_DIR", "/tmp/env-oops")
	if got := cmd.ResolveWorkDir(""); got != "/tmp/env-oops" {
		t.Errorf("ResolveWorkDir with OOPS_DIR = %q, expected /tmp/env-oops", got)
	}
	os.Unsetenv("OOPS_DIR")

	// 4. Default fallback
	fallback := cmd.ResolveWorkDir("")
	if fallback == "" {
		t.Errorf("ResolveWorkDir(\"\") returned empty string")
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
