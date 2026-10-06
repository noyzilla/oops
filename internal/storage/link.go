package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Link creates boxDir/name as a symlink to target after validating the target.
// An existing symlink is replaced only with force; a real directory is replaced only when empty.
func (c Checker) Link(boxDir, name, target string, prefixes []string, force bool) (string, error) {
	if name == "" || strings.ContainsAny(name, "/\\") || name == "." || name == ".." {
		return "", fmt.Errorf("invalid link name %q", name)
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(absTarget)
	if err != nil {
		return "", fmt.Errorf("target %s does not exist", absTarget)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("target %s is not a directory", absTarget)
	}
	if p := c.CheckPath(absTarget, prefixes, false); p != nil {
		return "", fmt.Errorf("target %s is unhealthy: %s", absTarget, p.Reason)
	}

	linkPath := filepath.Join(boxDir, name)
	if existing, err := c.Lstat(linkPath); err == nil {
		switch {
		case existing.Mode()&os.ModeSymlink != 0:
			if !force {
				current, _ := c.Readlink(linkPath)
				return "", fmt.Errorf("%s already links to %s (use --force to replace)", linkPath, current)
			}
			if err := os.Remove(linkPath); err != nil {
				return "", err
			}
		case existing.IsDir():
			entries, err := os.ReadDir(linkPath)
			if err != nil {
				return "", err
			}
			if len(entries) > 0 {
				return "", fmt.Errorf("%s is a directory containing data; move its contents to %s first", linkPath, absTarget)
			}
			if err := os.Remove(linkPath); err != nil {
				return "", err
			}
		default:
			return "", fmt.Errorf("%s exists and is not a directory or link", linkPath)
		}
	}

	if err := os.Symlink(absTarget, linkPath); err != nil {
		return "", err
	}
	return linkPath, nil
}
