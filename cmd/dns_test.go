package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noyzilla/oops/cmd"
)

func TestDNSCommands_AddDelReloadList(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "oops-cli-dns-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	rootCmd := cmd.GetRootCommand()

	// 1. oops dns add
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"-C", tempDir, "dns", "add", "my-app.test", "127.0.0.1"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("failed to execute oops dns add: %v", err)
	}

	// 2. oops dns add wildcard
	rootCmd = cmd.GetRootCommand()
	buf.Reset()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"-C", tempDir, "dns", "add", ".wild.test", "192.168.1.100"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("failed to execute oops dns add wildcard: %v", err)
	}

	// Check file created
	dnsFile := filepath.Join(tempDir, "config", "oops", "dns")
	data, err := os.ReadFile(dnsFile)
	if err != nil {
		t.Fatalf("failed to read created dns file: %v", err)
	}
	if !strings.Contains(string(data), "my-app.test") || !strings.Contains(string(data), ".wild.test") {
		t.Errorf("dns file missing added records: %s", string(data))
	}

	// 3. oops dns reload
	rootCmd = cmd.GetRootCommand()
	buf.Reset()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"-C", tempDir, "dns", "reload"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("failed to execute oops dns reload: %v", err)
	}

	// 4. oops dns del
	rootCmd = cmd.GetRootCommand()
	buf.Reset()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"-C", tempDir, "dns", "del", "my-app.test"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("failed to execute oops dns del: %v", err)
	}

	dataAfterDel, _ := os.ReadFile(dnsFile)
	if strings.Contains(string(dataAfterDel), "my-app.test") {
		t.Errorf("expected my-app.test to be deleted, but still present in %s", string(dataAfterDel))
	}
	if !strings.Contains(string(dataAfterDel), ".wild.test") {
		t.Errorf("expected .wild.test to remain, but missing from %s", string(dataAfterDel))
	}
}
