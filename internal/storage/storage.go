// Package storage validates host paths used for persistent data so containers and backups
// never silently write to the OS disk when a persistent disk is missing or a link is dead.
package storage

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultMountPrefixes are the directory prefixes where persistent disks are expected to be mounted
var DefaultMountPrefixes = []string{"/mnt", "/media", "/Volumes"}

// Problem describes an unhealthy storage path
type Problem struct {
	Path   string
	Target string
	Reason string
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

func underAny(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if UnderPrefix(path, p) {
			return true
		}
	}
	return false
}

// CheckPath inspects a host path. A dead symlink anywhere along the path is always a problem.
// A path resolving under a mount prefix on the same device as "/" is a problem (disk not mounted).
// When strictMissing is false a plain missing path is ignored (a container runtime would create it);
// when true the nearest existing ancestor is checked instead, because creating the path
// would land on the OS disk.
func (c Checker) CheckPath(path string, prefixes []string, strictMissing bool) *Problem {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil
	}

	cumulative := string(filepath.Separator)
	lastExisting := cumulative
	missing := false
	for _, part := range strings.Split(abs, string(filepath.Separator)) {
		if part == "" {
			continue
		}
		cumulative = filepath.Join(cumulative, part)
		fi, err := c.Lstat(cumulative)
		if err != nil {
			missing = true
			break
		}
		lastExisting = cumulative
		if fi.Mode()&os.ModeSymlink != 0 {
			if _, err := c.EvalSymlinks(cumulative); err != nil {
				target, _ := c.Readlink(cumulative)
				return &Problem{Path: cumulative, Target: target, Reason: "link target does not exist"}
			}
		}
	}

	subject := abs
	if missing {
		if !strictMissing {
			return nil
		}
		subject = lastExisting
	}

	resolved, err := c.EvalSymlinks(subject)
	if err != nil || !c.MountCheck || !underAny(resolved, prefixes) {
		return nil
	}

	rootDev, errRoot := c.Device("/")
	targetDev, errTarget := c.Device(resolved)
	if errRoot != nil || errTarget != nil || rootDev != targetDev {
		return nil
	}

	reason := "path is on the OS disk (persistent disk not mounted)"
	if missing {
		reason = "path does not exist and its nearest existing parent is on the OS disk (persistent disk not mounted)"
	}
	return &Problem{Path: abs, Target: resolved, Reason: reason}
}
