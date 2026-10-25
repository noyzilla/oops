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

func TestDNSRepairCommand(t *testing.T) {
	tempResolverDir := t.TempDir()
	cleanup := box.SetResolverDirForTest(tempResolverDir)
	defer cleanup()

	cleanupIP := box.SetOopsContainerIPForTest("10.240.0.2")
	defer cleanupIP()

	tempWorkDir := t.TempDir()
	// Create registered remotes @anthole and @b9tpe
	for _, r := range []string{"anthole", "b9tpe"} {
		rDir := filepath.Join(tempWorkDir, "@"+r)
		_ = os.MkdirAll(rDir, 0755)
		_ = os.WriteFile(filepath.Join(rDir, "remote.yaml"), []byte("host: "+r+".example.com\n"), 0644)
	}

	// Seed stale resolver files:
	// anthole: 192.168.97.3 (stale)
	// b9tpe: 192.168.97.3 (stale)
	// oops: missing
	_ = os.WriteFile(filepath.Join(tempResolverDir, "anthole"), []byte("nameserver 192.168.97.3\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempResolverDir, "b9tpe"), []byte("nameserver 192.168.97.3\n"), 0644)

	// 1. Test dry-run
	rootCmd := cmd.GetRootCommand()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"-C", tempWorkDir, "dns", "repair", "--dry-run"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("dns repair --dry-run failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Would update /etc/resolver/anthole (nameserver 192.168.97.3 -> 10.240.0.2)") {
		t.Errorf("expected dry-run to show anthole update, got:\n%s", output)
	}
	if !strings.Contains(output, "Would update /etc/resolver/b9tpe (nameserver 192.168.97.3 -> 10.240.0.2)") {
		t.Errorf("expected dry-run to show b9tpe update, got:\n%s", output)
	}
	if !strings.Contains(output, "Would create /etc/resolver/oops (nameserver 10.240.0.2)") {
		t.Errorf("expected dry-run to show oops create, got:\n%s", output)
	}

	// Files should still have old IP
	ipAnthole, _ := box.ReadMacOSResolver("anthole")
	if ipAnthole != "192.168.97.3" {
		t.Errorf("expected anthole to still have 192.168.97.3 after dry-run, got %s", ipAnthole)
	}

	// 2. Real repair
	rootCmd = cmd.GetRootCommand()
	buf.Reset()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"-C", tempWorkDir, "dns", "repair"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("dns repair failed: %v", err)
	}
	output = buf.String()
	if !strings.Contains(output, "Repaired 3 resolver(s)") {
		t.Errorf("expected 3 repaired resolvers, got:\n%s", output)
	}

	// Verify all 3 now have 10.240.0.2
	for _, domain := range []string{"anthole", "b9tpe", "oops"} {
		ip, err := box.ReadMacOSResolver(domain)
		if err != nil {
			t.Errorf("failed reading resolver for %s: %v", domain, err)
		}
		if ip != "10.240.0.2" {
			t.Errorf("expected %s to have 10.240.0.2, got %s", domain, ip)
		}
	}

	// 3. Run again when already up to date
	rootCmd = cmd.GetRootCommand()
	buf.Reset()
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"-C", tempWorkDir, "dns", "repair"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("dns repair second run failed: %v", err)
	}
	output = buf.String()
	if !strings.Contains(output, "is up to date") {
		t.Errorf("expected up to date message, got:\n%s", output)
	}
}
