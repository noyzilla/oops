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
		{"down", "wipe-all", ""},
		{"down", "yes", "y"},
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
	tests := []struct {
		name string
		args []string
	}{
		{"WithHelpFlag", []string{"--help"}},
		{"BareCommand", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootCmd := cmd.GetRootCommand()
			buf := new(bytes.Buffer)
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)
			rootCmd.SetArgs(tt.args)

			err := rootCmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error executing %v: %v", tt.args, err)
			}

			out := buf.String()
			if !strings.Contains(out, "managing multi-stack Docker Compose deployments") {
				t.Errorf("output missing expected description for args %v: %s", tt.args, out)
			}
			if !strings.Contains(out, "Available Commands:") {
				t.Errorf("output missing command list for args %v: %s", tt.args, out)
			}
		})
	}
}

func TestResolveWorkDir(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

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

	// 4. Default fallback
	fallback := cmd.ResolveWorkDir("")
	if fallback == "" {
		t.Errorf("ResolveWorkDir(\"\") returned empty string")
	}
}

func TestActiveBox_SetGetValidate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oopsbox-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test Set and Get
	err = cmd.SetActiveBox(tmpDir)
	if err != nil {
		t.Fatalf("SetActiveBox failed: %v", err)
	}

	got, err := cmd.GetActiveBox()
	if err != nil {
		t.Fatalf("GetActiveBox failed: %v", err)
	}
	if got != tmpDir {
		t.Errorf("GetActiveBox = %q, expected %q", got, tmpDir)
	}

	// Validate matching path succeeds
	if err := cmd.ValidateActiveBox(tmpDir); err != nil {
		t.Errorf("ValidateActiveBox with matching path failed: %v", err)
	}

	// Validate mismatched path fails
	otherDir, _ := os.MkdirTemp("", "oopsbox-other-*")
	defer os.RemoveAll(otherDir)

	if err := cmd.ValidateActiveBox(otherDir); err == nil {
		t.Errorf("ValidateActiveBox with mismatched path expected error, got nil")
	} else if !strings.Contains(err.Error(), "active oopsbox mismatch") {
		t.Errorf("expected mismatch error, got: %v", err)
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
