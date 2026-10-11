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
	expectedSubcommands := []string{"init", "active", "list", "start", "stop", "switch", "cert"}
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
