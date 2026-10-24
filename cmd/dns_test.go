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

func TestDNSCommands_AddDelReloadList(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "oops-cli-dns-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	rootCmd := cmd.GetRootCommand()

	// 1. oops dns set (upsert)
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"-C", tempDir, "dns", "set", "my-app.test", "127.0.0.1"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("failed to execute oops dns set: %v", err)
	}

	// 2. oops dns set wildcard
	rootCmd = cmd.GetRootCommand()
	buf.Reset()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"-C", tempDir, "dns", "set", ".wild.test", "192.168.1.100"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("failed to execute oops dns set wildcard: %v", err)
	}

	// Check file created at data/oops/dns.records
	dnsFile := filepath.Join(tempDir, "data", "oops", "dns.records")
	data, err := os.ReadFile(dnsFile)
	if err != nil {
		t.Fatalf("failed to read created dns file at %s: %v", dnsFile, err)
	}
	if !strings.Contains(string(data), "my-app.test") || !strings.Contains(string(data), ".wild.test") {
		t.Errorf("dns file missing added records: %s", string(data))
	}

	// 3. oops dns get
	rootCmd = cmd.GetRootCommand()
	buf.Reset()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"-C", tempDir, "dns", "get", "my-app.test"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("failed to execute oops dns get: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "127.0.0.1" {
		t.Errorf("expected oops dns get to return 127.0.0.1, got %q", buf.String())
	}

	// 4. oops dns reload
	rootCmd = cmd.GetRootCommand()
	buf.Reset()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"-C", tempDir, "dns", "reload"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("failed to execute oops dns reload: %v", err)
	}

	// 5. oops dns del
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

func TestDNSPruneCommand(t *testing.T) {
	tempResolverDir := t.TempDir()
	cleanup := box.SetResolverDirForTest(tempResolverDir)
	defer cleanup()

	// Seed resolvers: oops (active), b9tpe (stale unless remote registered), stale1, stale2
	_ = os.WriteFile(filepath.Join(tempResolverDir, "oops"), []byte("nameserver 127.0.0.1\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempResolverDir, "stale1"), []byte("nameserver 127.0.0.1\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempResolverDir, "stale2"), []byte("nameserver 127.0.0.1\n"), 0644)

	tempWorkDir := t.TempDir()

	// 1. Dry run
	rootCmd := cmd.GetRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"-C", tempWorkDir, "dns", "prune", "--dry-run"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("dns prune --dry-run failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "stale1") || !strings.Contains(output, "stale2") {
		t.Errorf("expected dry-run to identify stale1 and stale2, got: %s", output)
	}

	// Files should still exist
	if _, err := os.Stat(filepath.Join(tempResolverDir, "stale1")); err != nil {
		t.Fatalf("expected stale1 to still exist after dry-run")
	}

	// 2. Real prune
	rootCmd = cmd.GetRootCommand()
	buf.Reset()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"-C", tempWorkDir, "dns", "prune"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("dns prune failed: %v", err)
	}
	output = buf.String()
	if !strings.Contains(output, "Pruned 2 stale DNS resolver") {
		t.Errorf("expected success message, got: %s", output)
	}

	// Verify stale files removed, oops preserved
	if _, err := os.Stat(filepath.Join(tempResolverDir, "stale1")); !os.IsNotExist(err) {
		t.Errorf("expected stale1 to be deleted")
	}
	if _, err := os.Stat(filepath.Join(tempResolverDir, "stale2")); !os.IsNotExist(err) {
		t.Errorf("expected stale2 to be deleted")
	}
	if _, err := os.Stat(filepath.Join(tempResolverDir, "oops")); err != nil {
		t.Errorf("expected oops resolver to be preserved")
	}
}
