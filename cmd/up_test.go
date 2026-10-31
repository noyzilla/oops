package cmd

import (
	"testing"
)

func TestNewUpCmd_Flags(t *testing.T) {
	cmd := newUpCmd()

	detachFlag := cmd.Flags().Lookup("detach")
	if detachFlag == nil {
		t.Fatalf("expected --detach flag to exist on up command")
	}
	if detachFlag.DefValue != "false" {
		t.Errorf("expected default value of --detach to be 'false', got '%s'", detachFlag.DefValue)
	}

	delayFlag := cmd.Flags().Lookup("delay")
	if delayFlag == nil {
		t.Fatalf("expected --delay flag to exist on up command")
	}
	if delayFlag.Shorthand != "d" {
		t.Errorf("expected shorthand for --delay to be 'd', got '%s'", delayFlag.Shorthand)
	}
}

