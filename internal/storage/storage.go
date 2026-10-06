// Package storage validates persistent storage links (data/ and backups/) of an oopsbox
// so containers never silently write to the OS disk when a persistent disk is missing.
package storage

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultMountPrefixes are the directory prefixes where persistent disks are expected to be mounted
var DefaultMountPrefixes = []string{"/mnt", "/media", "/Volumes"}

// LinkNames are the oopsbox root entries inspected by the guard
var LinkNames = []string{"data", "backups"}

// Problem describes a Bad Link
type Problem struct {
	LinkPath string
	Target   string
	Reason   string
}

// Checker abstracts filesystem access so the rules can be tested without real mounts
type Checker struct {
	Lstat        func(string) (os.FileInfo, error)
	EvalSymlinks func(string) (string, error)
	Readlink     func(string) (string, error)
	Device       func(string) (uint64, error)
	// MountCheck enables the root-device comparison, effective on Linux only
	MountCheck bool
}

// NewChecker returns a Checker bound to the real filesystem
func NewChecker() Checker {
	return Checker{
		Lstat:        os.Lstat,
		EvalSymlinks: filepath.EvalSymlinks,
		Readlink:     os.Readlink,
		Device:       deviceOf,
		MountCheck:   runtime.GOOS == "linux",
	}
}

// UnderPrefix reports whether path is inside prefix, respecting directory boundaries
func UnderPrefix(path, prefix string) bool {
	path = filepath.Clean(path)
	prefix = filepath.Clean(prefix)
	if prefix == "/" {
		return strings.HasPrefix(path, "/")
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// Inspect checks every symlink named in LinkNames under boxDir and returns the Bad Links
func (c Checker) Inspect(boxDir string, prefixes []string) []Problem {
	var problems []Problem
	for _, name := range LinkNames {
		linkPath := filepath.Join(boxDir, name)
		fi, err := c.Lstat(linkPath)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			continue
		}

		target, _ := c.Readlink(linkPath)
		resolved, err := c.EvalSymlinks(linkPath)
		if err != nil {
			problems = append(problems, Problem{LinkPath: linkPath, Target: target, Reason: "link target does not exist"})
			continue
		}

		if !c.MountCheck || !underAny(resolved, prefixes) {
			continue
		}

		rootDev, errRoot := c.Device("/")
		targetDev, errTarget := c.Device(resolved)
		if errRoot != nil || errTarget != nil {
			continue
		}
		if rootDev == targetDev {
			problems = append(problems, Problem{
				LinkPath: linkPath,
				Target:   resolved,
				Reason:   "target is on the OS disk (persistent disk not mounted)",
			})
		}
	}
	return problems
}

func underAny(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if UnderPrefix(path, p) {
			return true
		}
	}
	return false
}

// PassesThrough reports whether source is the link itself or located beneath it
func (p Problem) PassesThrough(source string) bool {
	return UnderPrefix(source, p.LinkPath)
}
