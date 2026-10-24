package doctor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noyzilla/oops/internal/box"
)

func TestAuditComposeSubnets_ContainmentAndReservations(t *testing.T) {
	tempDir := t.TempDir()
	composePath := filepath.Join(tempDir, "compose.yaml")

	// Compose with:
	// - valid backend network (10.240.1.0/24)
	// - out-of-bounds external network (192.168.1.0/24)
	// - service using .1 (gateway)
	// - service using .2 (dns)
	composeContent := `
x-oops:
  network:
    subnet: "10.240.0.0/16"

services:
  gateway-abuser:
    networks:
      backend:
        ipv4_address: "10.240.1.1"
  dns-abuser:
    networks:
      backend:
        ipv4_address: "10.240.1.2"
  clean-service:
    networks:
      backend:
        ipv4_address: "10.240.1.50"

networks:
  backend:
    ipam:
      config:
        - subnet: "10.240.1.0/24"
  external:
    ipam:
      config:
        - subnet: "192.168.1.0/24"
`
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		t.Fatalf("failed to write test compose: %v", err)
	}

	issues := AuditComposeSubnets(composePath, "")

	hasContainmentErr := false
	hasGatewayWarn := false
	hasDNSWarn := false

	for _, issue := range issues {
		if issue.Scope == "compose" && strings.Contains(issue.Message, "not contained within parent CIDR") {
			hasContainmentErr = true
		}
		if issue.Scope == "compose" && strings.Contains(issue.Message, "reserved for the Docker network gateway") {
			hasGatewayWarn = true
		}
		if issue.Scope == "compose" && strings.Contains(issue.Message, "reserved for the Oops DNS daemon") {
			hasDNSWarn = true
		}
	}

	if !hasContainmentErr {
		t.Errorf("expected containment error for 192.168.1.0/24 within 10.240.0.0/16")
	}
	if !hasGatewayWarn {
		t.Errorf("expected gateway reservation warning for .1")
	}
	if !hasDNSWarn {
		t.Errorf("expected DNS daemon reservation warning for .2")
	}
}

func TestAuditComposeAll_SubnetCollisionBetweenRemotes(t *testing.T) {
	tempDir := t.TempDir()

	// Create two remote capsules: @b9tpe and @anthole, both claiming 10.240.0.0/16
	b9tpeDir := filepath.Join(tempDir, "@b9tpe")
	antholeDir := filepath.Join(tempDir, "@anthole")
	_ = os.MkdirAll(b9tpeDir, 0755)
	_ = os.MkdirAll(antholeDir, 0755)

	oopsB9tpe := "x-oops:\n  network:\n    subnet: \"10.240.0.0/16\"\n"
	oopsAnthole := "x-oops:\n  network:\n    subnet: \"10.240.0.0/16\"\n"

	_ = os.WriteFile(filepath.Join(b9tpeDir, "oops.yaml"), []byte(oopsB9tpe), 0644)
	_ = os.WriteFile(filepath.Join(antholeDir, "oops.yaml"), []byte(oopsAnthole), 0644)
	_ = os.WriteFile(filepath.Join(b9tpeDir, "remote.yaml"), []byte("host: b9tpe.example.com\n"), 0644)
	_ = os.WriteFile(filepath.Join(antholeDir, "remote.yaml"), []byte("host: anthole.example.com\n"), 0644)

	issues := AuditComposeAll(tempDir)
	hasCollision := false
	for _, issue := range issues {
		if issue.Scope == "network" && strings.Contains(issue.Message, "Subnet collision detected") {
			hasCollision = true
			break
		}
	}

	if !hasCollision {
		t.Errorf("expected subnet collision between @b9tpe and @anthole")
	}
}

func TestAuditResolverEntries(t *testing.T) {
	tempResolverDir := t.TempDir()
	cleanup := box.SetResolverDirForTest(tempResolverDir)
	defer cleanup()

	tempWorkDir := t.TempDir()
	// Create capsule @b9tpe in workDir
	b9tpeDir := filepath.Join(tempWorkDir, "@b9tpe")
	_ = os.MkdirAll(b9tpeDir, 0755)
	_ = os.WriteFile(filepath.Join(b9tpeDir, "remote.yaml"), []byte("host: b9tpe.example.com\n"), 0644)

	// Resolver dir has: oops (valid), stale-resolver (orphaned), but MISSING b9tpe
	_ = os.WriteFile(filepath.Join(tempResolverDir, "oops"), []byte("nameserver 127.0.0.1\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempResolverDir, "stale-resolver"), []byte("nameserver 127.0.0.1\n"), 0644)

	issues := AuditResolverEntries(tempWorkDir)

	hasOrphaned := false
	hasMissing := false

	for _, issue := range issues {
		if strings.Contains(issue.Message, "Orphaned macOS DNS resolver for domain .stale-resolver") {
			hasOrphaned = true
		}
		if strings.Contains(issue.Message, "Missing DNS resolver for registered remote '@b9tpe'") {
			hasMissing = true
		}
	}

	if !hasOrphaned {
		t.Errorf("expected orphaned resolver warning for stale-resolver")
	}
	if !hasMissing {
		t.Errorf("expected missing resolver warning for @b9tpe")
	}
}

func TestRunFullAudit_CleanWorkspace(t *testing.T) {
	tempResolverDir := t.TempDir()
	cleanup := box.SetResolverDirForTest(tempResolverDir)
	defer cleanup()

	tempWorkDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempWorkDir, "compose.yaml"), []byte("services:\n  web:\n    image: nginx\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempResolverDir, "oops"), []byte("nameserver 127.0.0.1\n"), 0644)

	report := RunFullAudit(context.Background(), tempWorkDir)
	if report.HasErrors() {
		t.Fatalf("expected clean audit, got errors: %s", report.Render())
	}
}

