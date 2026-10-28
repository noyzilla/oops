package box

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractCACertificate_NonExistentContainer(t *testing.T) {
	tmpDir := t.TempDir()
	targetCert := filepath.Join(tmpDir, "root.crt")

	err := ExtractCACertificate(targetCert)
	if err == nil {
		t.Fatalf("expected error when container does not exist or daemon is unreachable, got nil")
	}

	if _, statErr := os.Stat(targetCert); !os.IsNotExist(statErr) {
		t.Errorf("expected targetCert not to exist, got %v", statErr)
	}
}
