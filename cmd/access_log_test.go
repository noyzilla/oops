package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestAccessLogCmd_Flags(t *testing.T) {
	cmd := newAccessLogCmd()

	// Check default flag values
	followFlag := cmd.Flag("follow")
	if followFlag == nil || followFlag.DefValue != "true" {
		t.Errorf("expected --follow default to be 'true', got %v", followFlag)
	}

	tailFlag := cmd.Flag("tail")
	if tailFlag == nil || tailFlag.DefValue != "100" {
		t.Errorf("expected --tail default to be '100', got %v", tailFlag)
	}

	noFollowFlag := cmd.Flag("no-follow")
	if noFollowFlag == nil || noFollowFlag.Shorthand != "d" {
		t.Errorf("expected --no-follow shorthand to be 'd', got %v", noFollowFlag)
	}

	// Verify all 1:1 filter flags are registered
	expectedFlags := []string{
		"follow", "no-follow", "tail",
		"level", "status", "method", "host", "domain",
		"path", "scheme", "client", "ip", "latency", "all",
	}

	for _, f := range expectedFlags {
		if cmd.Flag(f) == nil {
			t.Errorf("expected flag --%s to be defined on access-log command", f)
		}
	}
}

func TestAccessLogCmd_HelpOutput(t *testing.T) {
	cmd := newAccessLogCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing --help: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "access-log [container]") {
		t.Errorf("help output missing usage string: %s", output)
	}
	if !strings.Contains(output, "--latency") {
		t.Errorf("help output missing --latency description")
	}
	if !strings.Contains(output, "--no-follow") {
		t.Errorf("help output missing --no-follow description")
	}
}

