package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noyzilla/oops/cmd"
	"github.com/noyzilla/oops/internal/box"
)

func TestDoctorCommands(t *testing.T) {
	tempResolverDir := t.TempDir()
	cleanup := box.SetResolverDirForTest(tempResolverDir)
	defer cleanup()

	tempWorkDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempWorkDir, "compose.yaml"), []byte("services:\n  app:\n    image: redis\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempResolverDir, "oops"), []byte("nameserver 127.0.0.1\n"), 0644)

	rootCmd := cmd.GetRootCommand()

	// 1. oops doctor (full audit)
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"-C", tempWorkDir, "doctor"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("oops doctor failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Oops Health Diagnostic Report") {
		t.Errorf("expected doctor report, got: %s", out)
	}

	// 2. oops doctor compose
	rootCmd = cmd.GetRootCommand()
	buf.Reset()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"-C", tempWorkDir, "doctor", "compose"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("oops doctor compose failed: %v", err)
	}
	out = buf.String()
	if !strings.Contains(out, "Oops Compose Diagnostic") {
		t.Errorf("expected compose diagnostic report, got: %s", out)
	}

	// 3. oops doctor dns
	rootCmd = cmd.GetRootCommand()
	buf.Reset()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"-C", tempWorkDir, "doctor", "dns"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("oops doctor dns failed: %v", err)
	}
	out = buf.String()
	if !strings.Contains(out, "Oops DNS & Resolver Diagnostic") {
		t.Errorf("expected dns diagnostic report, got: %s", out)
	}
}

