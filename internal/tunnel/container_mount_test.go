package tunnel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSSHReferencedDirectories(t *testing.T) {
	tempHome := t.TempDir()
	sshDir := filepath.Join(tempHome, ".ssh")
	_ = os.MkdirAll(sshDir, 0700)

	// Create an external key directory
	customKeyDir := filepath.Join(tempHome, "custom_keys", "project_a")
	_ = os.MkdirAll(customKeyDir, 0700)
	keyFile := filepath.Join(customKeyDir, "id_rsa_custom")
	_ = os.WriteFile(keyFile, []byte("fake-key"), 0600)

	// Create an internal key
	internalKey := filepath.Join(sshDir, "id_rsa_internal")
	_ = os.WriteFile(internalKey, []byte("fake-key"), 0600)

	// Write ~/.ssh/config referencing both
	configContent := "Host myhost\n" +
		"  IdentityFile ~/custom_keys/project_a/id_rsa_custom\n" +
		"  IdentityFile ~/.ssh/id_rsa_internal\n" +
		"  IdentityFile /non/existent/path/key\n"
	_ = os.WriteFile(filepath.Join(sshDir, "config"), []byte(configContent), 0600)

	dirs := ResolveSSHReferencedDirectories(tempHome, "")
	if len(dirs) != 1 {
		t.Fatalf("expected exactly 1 external directory, got %d: %v", len(dirs), dirs)
	}

	expectedDir := filepath.Clean(customKeyDir)
	if dirs[0] != expectedDir {
		t.Errorf("expected %s, got %s", expectedDir, dirs[0])
	}
}
