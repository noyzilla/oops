package cmd

import (
	"testing"
)

func TestNewUpCmd_Flags(t *testing.T) {
	cmd := newUpCmd()

	noFollowFlag := cmd.Flags().Lookup("no-follow")
	if noFollowFlag == nil {
		t.Fatalf("expected --no-follow flag to exist on up command")
	}
	if noFollowFlag.Shorthand != "d" {
		t.Errorf("expected shorthand for --no-follow to be 'd', got '%s'", noFollowFlag.Shorthand)
	}
	if noFollowFlag.DefValue != "false" {
		t.Errorf("expected default value of --no-follow to be 'false', got '%s'", noFollowFlag.DefValue)
	}

	waitFlag := cmd.Flags().Lookup("wait")
	if waitFlag == nil {
		t.Fatalf("expected --wait flag to exist on up command")
	}
	if waitFlag.Shorthand != "w" {
		t.Errorf("expected shorthand for --wait to be 'w', got '%s'", waitFlag.Shorthand)
	}
	if waitFlag.DefValue != "0s" {
		t.Errorf("expected default value of --wait to be '0s', got '%s'", waitFlag.DefValue)
	}
}
