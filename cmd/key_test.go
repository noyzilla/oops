package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKeyCmd(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cmd := newKeyCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("oops key failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ssh-ed25519") {
		t.Errorf("expected public key in output, got: %s", out)
	}

	keyFile := filepath.Join(tmpHome, ".oops", "id_ed25519")
	if _, err := os.Stat(keyFile); err != nil {
		t.Errorf("expected key file to exist at %s: %v", keyFile, err)
	}
}

func TestKeyResetCmd(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cmd1 := newKeyCmd()
	buf1 := new(bytes.Buffer)
	cmd1.SetOut(buf1)
	cmd1.SetArgs([]string{})
	_ = cmd1.Execute()
	out1 := buf1.String()

	cmd2 := newKeyResetCmd()
	buf2 := new(bytes.Buffer)
	cmd2.SetOut(buf2)
	cmd2.SetArgs([]string{})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("oops key reset failed: %v", err)
	}
	out2 := buf2.String()

	if !strings.Contains(out2, "✓ Regenerated new ed25519 SSH Deploy Key") {
		t.Errorf("unexpected output: %s", out2)
	}
	_ = out1
}
