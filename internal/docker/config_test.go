package docker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOopsConfigStorageAndPriority(t *testing.T) {
	dir := t.TempDir()
	yml := "volume:\n  mount_prefixes: []\n"
	if err := os.WriteFile(filepath.Join(dir, "oops.yaml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadOopsConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Volume.MountPrefixes == nil || len(cfg.Volume.MountPrefixes) != 0 {
		t.Fatalf("empty list must be preserved as non-nil, got %#v", cfg.Volume.MountPrefixes)
	}
	if cfg.Storage.MountPrefixes == nil || len(cfg.Storage.MountPrefixes) != 0 {
		t.Fatalf("storage alias must also be populated, got %#v", cfg.Storage.MountPrefixes)
	}

	empty, err := LoadOopsConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if empty.Volume.MountPrefixes != nil || empty.Storage.MountPrefixes != nil {
		t.Fatal("omitted mount_prefixes must stay nil so defaults apply")
	}
}
