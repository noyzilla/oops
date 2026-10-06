package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func linkChecker() Checker {
	c := NewChecker()
	c.MountCheck = false
	return c
}

func TestLinkCreatesAndReplaces(t *testing.T) {
	box, target, other := t.TempDir(), t.TempDir(), t.TempDir()
	c := linkChecker()

	path, err := c.Link(box, "data", target, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.Readlink(path); got != target {
		t.Fatalf("link points to %s want %s", got, target)
	}

	if _, err := c.Link(box, "data", other, nil, false); err == nil {
		t.Fatal("existing link must not be replaced without force")
	}
	if _, err := c.Link(box, "data", other, nil, true); err != nil {
		t.Fatalf("force replace failed: %v", err)
	}
}

func TestLinkDirectoryRules(t *testing.T) {
	box, target := t.TempDir(), t.TempDir()
	c := linkChecker()
	dir := filepath.Join(box, "data")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Link(box, "data", target, nil, true); err == nil {
		t.Fatal("non-empty directory must be refused even with force")
	}

	if err := os.Remove(filepath.Join(dir, "f")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Link(box, "data", target, nil, false); err != nil {
		t.Fatalf("empty directory should be replaced: %v", err)
	}
}

func TestLinkRejectsBadInput(t *testing.T) {
	box := t.TempDir()
	c := linkChecker()
	if _, err := c.Link(box, "a/b", t.TempDir(), nil, false); err == nil {
		t.Fatal("name with slash must be rejected")
	}
	if _, err := c.Link(box, "data", filepath.Join(box, "missing"), nil, false); err == nil {
		t.Fatal("missing target must be rejected")
	}
}
