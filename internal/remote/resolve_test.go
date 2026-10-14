package remote

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRequiredDependencies(t *testing.T) {
	tempDir := t.TempDir()

	// Create compose.crm.yaml with x-oops-services and depends_on
	crmContent := `x-oops-services:
  - postgres
  - redis
services:
  crm-app:
    image: custom/crm:latest
    depends_on:
      - caddy
`
	_ = os.WriteFile(filepath.Join(tempDir, "compose.crm.yaml"), []byte(crmContent), 0644)

	// Create dummy service templates
	_ = os.MkdirAll(filepath.Join(tempDir, "services"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "services", "postgres.yaml"), []byte("services: {}\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "services", "redis.yaml"), []byte("services: {}\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "services", "caddy.yaml"), []byte("services: {}\n"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "services", "unused.yaml"), []byte("services: {}\n"), 0644)

	// Create capsule dir with overrides
	capsuleDir := filepath.Join(tempDir, "@prod")
	_ = os.MkdirAll(capsuleDir, 0755)
	_ = os.WriteFile(filepath.Join(capsuleDir, "compose.yaml"), []byte("name: oops\n"), 0644)

	inspector, err := ResolveRequiredDependencies(tempDir, "crm", "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := inspector.Services["postgres"]; !ok {
		t.Errorf("expected postgres in resolved services")
	}
	if _, ok := inspector.Services["redis"]; !ok {
		t.Errorf("expected redis in resolved services")
	}
	if _, ok := inspector.Services["caddy"]; !ok {
		t.Errorf("expected caddy in resolved services")
	}
	if _, ok := inspector.Services["unused"]; ok {
		t.Errorf("unused service should not be in resolved services")
	}

	if len(inspector.ServiceFiles) != 3 {
		t.Errorf("expected 3 service files, got %d: %v", len(inspector.ServiceFiles), inspector.ServiceFiles)
	}
}
