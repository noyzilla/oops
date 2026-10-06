package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnderPrefix(t *testing.T) {
	cases := []struct {
		path, prefix string
		want         bool
	}{
		{"/mnt/disks/x/y", "/mnt", true},
		{"/mnt", "/mnt", true},
		{"/mntx/a", "/mnt", false},
		{"/anything/at/all", "/", true},
		{"/media/a", "/mnt", false},
	}
	for _, c := range cases {
		if got := UnderPrefix(c.path, c.prefix); got != c.want {
			t.Errorf("UnderPrefix(%q,%q)=%v want %v", c.path, c.prefix, got, c.want)
		}
	}
}

func fakeChecker(t *testing.T, devices map[string]uint64) Checker {
	c := NewChecker()
	c.MountCheck = true
	c.Device = func(p string) (uint64, error) {
		best, bestLen := uint64(1), 0
		for prefix, dev := range devices {
			if UnderPrefix(p, prefix) && len(prefix) >= bestLen {
				best, bestLen = dev, len(prefix)
			}
		}
		return best, nil
	}
	return c
}

func TestInspectDeadLink(t *testing.T) {
	box := t.TempDir()
	if err := os.Symlink(filepath.Join(box, "missing"), filepath.Join(box, "data")); err != nil {
		t.Fatal(err)
	}
	c := NewChecker()
	c.MountCheck = false
	got := c.Inspect(box, nil)
	if len(got) != 1 || got[0].LinkPath != filepath.Join(box, "data") {
		t.Fatalf("expected dead link problem, got %+v", got)
	}
}

func TestInspectRealDirectoryPasses(t *testing.T) {
	box := t.TempDir()
	if err := os.Mkdir(filepath.Join(box, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := NewChecker().Inspect(box, DefaultMountPrefixes); len(got) != 0 {
		t.Fatalf("real directory must pass, got %+v", got)
	}
}

func TestInspectRootDeviceUnderPrefix(t *testing.T) {
	box := t.TempDir()
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(box, "data")); err != nil {
		t.Fatal(err)
	}
	resolved, _ := filepath.EvalSymlinks(target)

	rootOnly := fakeChecker(t, map[string]uint64{"/": 1})
	if got := rootOnly.Inspect(box, []string{"/"}); len(got) != 1 {
		t.Fatalf("target on root device under prefix / must be bad, got %+v", got)
	}
	if got := rootOnly.Inspect(box, []string{"/nowhere"}); len(got) != 0 {
		t.Fatalf("target outside prefixes must not be checked, got %+v", got)
	}
	if got := rootOnly.Inspect(box, nil); len(got) != 0 {
		t.Fatalf("empty prefixes disable mount check, got %+v", got)
	}

	mounted := fakeChecker(t, map[string]uint64{"/": 1, resolved: 2})
	if got := mounted.Inspect(box, []string{"/"}); len(got) != 0 {
		t.Fatalf("target on separate device must pass, got %+v", got)
	}

	noMount := rootOnly
	noMount.MountCheck = false
	if got := noMount.Inspect(box, []string{"/"}); len(got) != 0 {
		t.Fatalf("mount check disabled must only detect dead links, got %+v", got)
	}
}

func TestPassesThrough(t *testing.T) {
	p := Problem{LinkPath: "/box/data"}
	if !p.PassesThrough("/box/data/mysql") || !p.PassesThrough("/box/data") {
		t.Fatal("expected pass through")
	}
	if p.PassesThrough("/box/database") {
		t.Fatal("directory boundary violated")
	}
}
