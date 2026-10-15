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

func TestLoadOopsConfig_XOops(t *testing.T) {
	dir := t.TempDir()
	yml := `services:
  oops:
    image: ghcr.io/noyzilla/oops:latest
x-oops:
  registries:
    my_org: ghcr.io/my_org
  backups:
    retention: 14d
  dns:
    tld: mytld
    upstreams:
      - 1.1.1.1:53
  volume:
    mount_prefixes: [/mnt/data]
`
	if err := os.WriteFile(filepath.Join(dir, "oops.yaml"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadOopsConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Registries["my_org"] != "ghcr.io/my_org" {
		t.Errorf("expected registry alias, got %v", cfg.Registries)
	}
	if cfg.Backups.Retention != "14d" {
		t.Errorf("expected retention 14d, got %s", cfg.Backups.Retention)
	}
	if cfg.DNS.TLD != "mytld" {
		t.Errorf("expected tld mytld, got %s", cfg.DNS.TLD)
	}
	if len(cfg.Volume.MountPrefixes) != 1 || cfg.Volume.MountPrefixes[0] != "/mnt/data" {
		t.Errorf("expected mount_prefixes [/mnt/data], got %v", cfg.Volume.MountPrefixes)
	}
}
