package box

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMacOSResolverLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	cleanup := SetResolverDirForTest(tmpDir)
	defer cleanup()

	// 1. Setup resolver for b9tpe
	if err := SetupMacOSResolver("b9tpe"); err != nil {
		t.Fatalf("SetupMacOSResolver failed: %v", err)
	}

	b9tpePath := filepath.Join(tmpDir, "b9tpe")
	content, err := os.ReadFile(b9tpePath)
	if err != nil {
		t.Fatalf("expected resolver file %s: %v", b9tpePath, err)
	}
	if len(content) == 0 {
		t.Fatalf("expected non-empty resolver content")
	}

	// 2. Setup resolver for anthole and oops
	if err := SetupMacOSResolver("anthole"); err != nil {
		t.Fatalf("SetupMacOSResolver anthole failed: %v", err)
	}
	if err := SetupMacOSResolver("oops"); err != nil {
		t.Fatalf("SetupMacOSResolver oops failed: %v", err)
	}

	// 3. List resolvers
	resolvers, err := ListMacOSResolvers()
	if err != nil {
		t.Fatalf("ListMacOSResolvers failed: %v", err)
	}
	if len(resolvers) != 3 {
		t.Fatalf("expected 3 resolvers, got %d (%v)", len(resolvers), resolvers)
	}

	// 4. Rename b9tpe to prod
	if err := RenameMacOSResolver("b9tpe", "prod"); err != nil {
		t.Fatalf("RenameMacOSResolver failed: %v", err)
	}
	if _, err := os.Stat(b9tpePath); !os.IsNotExist(err) {
		t.Fatalf("expected old resolver file to be deleted")
	}
	prodPath := filepath.Join(tmpDir, "prod")
	if _, err := os.Stat(prodPath); err != nil {
		t.Fatalf("expected new resolver file %s: %v", prodPath, err)
	}

	// 5. Prune resolvers, keeping only "prod" and "oops"
	// "anthole" should be pruned
	pruned, err := PruneMacOSResolvers([]string{"prod"})
	if err != nil {
		t.Fatalf("PruneMacOSResolvers failed: %v", err)
	}
	if len(pruned) != 1 || pruned[0] != "anthole" {
		t.Fatalf("expected anthole to be pruned, got %v", pruned)
	}

	// Verify remaining: prod and oops
	resolvers, err = ListMacOSResolvers()
	if err != nil {
		t.Fatalf("ListMacOSResolvers failed: %v", err)
	}
	if len(resolvers) != 2 {
		t.Fatalf("expected 2 resolvers remaining (prod, oops), got %d (%v)", len(resolvers), resolvers)
	}

	// 6. Remove prod
	if err := RemoveMacOSResolver("prod"); err != nil {
		t.Fatalf("RemoveMacOSResolver failed: %v", err)
	}
	if _, err := os.Stat(prodPath); !os.IsNotExist(err) {
		t.Fatalf("expected prod resolver file to be deleted")
	}

	// 7. Test SetupMacOSResolverWithIP & ReadMacOSResolver
	if err := SetupMacOSResolverWithIP("custom", "10.240.0.15"); err != nil {
		t.Fatalf("SetupMacOSResolverWithIP failed: %v", err)
	}
	readIP, err := ReadMacOSResolver("custom")
	if err != nil {
		t.Fatalf("ReadMacOSResolver failed: %v", err)
	}
	if readIP != "10.240.0.15" {
		t.Errorf("ReadMacOSResolver = %q, want %q", readIP, "10.240.0.15")
	}

	// 8. Update custom resolver with new IP
	if err := SetupMacOSResolverWithIP("custom", "10.240.0.99"); err != nil {
		t.Fatalf("SetupMacOSResolverWithIP update failed: %v", err)
	}
	readIP2, err := ReadMacOSResolver("custom")
	if err != nil {
		t.Fatalf("ReadMacOSResolver after update failed: %v", err)
	}
	if readIP2 != "10.240.0.99" {
		t.Errorf("ReadMacOSResolver after update = %q, want %q", readIP2, "10.240.0.99")
	}

	// 9. Flush cache
	if err := FlushMacOSDNSCache(); err != nil {
		t.Fatalf("FlushMacOSDNSCache failed: %v", err)
	}
}

