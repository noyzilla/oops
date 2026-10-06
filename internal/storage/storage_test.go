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

func fakeChecker(devices map[string]uint64) Checker {
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

func TestCheckPathDeadLinkAnywhereInPath(t *testing.T) {
	box := t.TempDir()
	if err := os.Symlink(filepath.Join(box, "missing"), filepath.Join(box, "data")); err != nil {
		t.Fatal(err)
	}
	c := NewChecker()
	c.MountCheck = false
	p := c.CheckPath(filepath.Join(box, "data", "mysql"), nil, false)
	if p == nil || p.Path != filepath.Join(box, "data") {
		t.Fatalf("expected dead link problem at data, got %+v", p)
	}
}

func TestCheckPathHealthyAndMissing(t *testing.T) {
	box := t.TempDir()
	c := NewChecker()
	if p := c.CheckPath(box, DefaultMountPrefixes, false); p != nil {
		t.Fatalf("healthy dir must pass, got %+v", p)
	}
	if p := c.CheckPath(filepath.Join(box, "nope"), DefaultMountPrefixes, false); p != nil {
		t.Fatalf("plain missing path ignored when not strict, got %+v", p)
	}
}

func TestCheckPathRootDeviceUnderPrefix(t *testing.T) {
	target, _ := filepath.EvalSymlinks(t.TempDir())
	link := filepath.Join(t.TempDir(), "data")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	rootOnly := fakeChecker(map[string]uint64{"/": 1})
	if p := rootOnly.CheckPath(link, []string{"/"}, false); p == nil {
		t.Fatal("target on root device under prefix / must be bad")
	}
	if p := rootOnly.CheckPath(link, []string{"/nowhere"}, false); p != nil {
		t.Fatalf("outside prefixes must not be checked, got %+v", p)
	}
	if p := rootOnly.CheckPath(link, nil, false); p != nil {
		t.Fatalf("empty prefixes disable mount check, got %+v", p)
	}

	mounted := fakeChecker(map[string]uint64{"/": 1, target: 2})
	if p := mounted.CheckPath(link, []string{"/"}, false); p != nil {
		t.Fatalf("separate device must pass, got %+v", p)
	}

	noMount := rootOnly
	noMount.MountCheck = false
	if p := noMount.CheckPath(link, []string{"/"}, false); p != nil {
		t.Fatalf("mount check disabled must only detect dead links, got %+v", p)
	}
}

func TestCheckPathStrictMissingUsesNearestParent(t *testing.T) {
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	missing := filepath.Join(dir, "backups", "db")

	rootOnly := fakeChecker(map[string]uint64{"/": 1})
	if p := rootOnly.CheckPath(missing, []string{"/"}, true); p == nil {
		t.Fatal("strict missing path whose parent is on root device must be bad")
	}
	if p := rootOnly.CheckPath(missing, []string{"/"}, false); p != nil {
		t.Fatalf("non-strict must ignore missing path, got %+v", p)
	}
	mounted := fakeChecker(map[string]uint64{"/": 1, dir: 2})
	if p := mounted.CheckPath(missing, []string{"/"}, true); p != nil {
		t.Fatalf("parent on separate device must pass, got %+v", p)
	}
}
