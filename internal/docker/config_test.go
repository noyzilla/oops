package docker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOopsConfigStorageAndPriority(t *testing.T) {
	dir := t.TempDir()
	yml := "storage:\n  mount_prefixes: []\npriority:\n  - /edge\n  - \"@core\"\n"
	if err := os.WriteFile(filepath.Join(dir, "oops.yml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadOopsConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.MountPrefixes == nil || len(cfg.Storage.MountPrefixes) != 0 {
		t.Fatalf("empty list must be preserved as non-nil, got %#v", cfg.Storage.MountPrefixes)
	}
	if len(cfg.Priority) != 2 || cfg.Priority[1] != "@core" {
		t.Fatalf("unexpected priority %v", cfg.Priority)
	}

	empty, err := LoadOopsConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if empty.Storage.MountPrefixes != nil {
		t.Fatal("omitted mount_prefixes must stay nil so defaults apply")
	}
}
