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
	expectedSubcommands := []string{"init", "list", "start", "stop", "switch", "cert"}
	for _, sub := range expectedSubcommands {
		if !bytes.Contains(out.Bytes(), []byte(sub)) {
			t.Errorf("expected subcommand %q in help text, got:\n%s", sub, helpText)
		}
	}
}
