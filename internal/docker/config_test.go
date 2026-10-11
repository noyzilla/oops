package docker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOopsConfigVolumeAndPriority(t *testing.T) {
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

	empty, err := LoadOopsConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if empty.Volume.MountPrefixes != nil {
		t.Fatal("omitted mount_prefixes must stay nil so defaults apply")
	}
}
