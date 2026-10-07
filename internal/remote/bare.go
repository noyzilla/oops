package remote

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GeneratePostReceiveHook generates the shell script content for server-side hooks/post-receive.
func GeneratePostReceiveHook(oopsboxPath string) string {
	return fmt.Sprintf(`#!/bin/sh
OOPSBOX_DIR="%s"
case "$OOPSBOX_DIR" in
  \~/*) OOPSBOX_DIR="$HOME/${OOPSBOX_DIR#\~/}" ;;
  \~)   OOPSBOX_DIR="$HOME" ;;
esac
export GIT_WORK_TREE="$OOPSBOX_DIR"
TARGET_REF=""
DO_DEPLOY=0

i=0
while [ $i -lt ${GIT_PUSH_OPTION_COUNT:-0} ]; do
  eval "opt=\$GIT_PUSH_OPTION_$i"
  if [ "$opt" = "deploy" ] || [ "$opt" = "up" ]; then
    DO_DEPLOY=1
  fi
  i=$((i + 1))
done

if [ "$DO_DEPLOY" -ne 1 ]; then
  exit 0
fi

while read oldrev newrev refname; do
  case "$refname" in
    refs/tags/*)
      TARGET_REF="tags/${refname#refs/tags/}"
      ;;
    refs/heads/*)
      TARGET_REF="${refname#refs/heads/}"
      ;;
  esac
done

if [ -n "$TARGET_REF" ]; then
  git checkout -f "$TARGET_REF"
  if command -v oops >/dev/null 2>&1; then
    oops up -C "$OOPSBOX_DIR" || true
  elif [ -f /var/lib/google/bin/oops ]; then
    /var/lib/google/bin/oops up -C "$OOPSBOX_DIR" || true
  fi
fi
`, oopsboxPath)
}

// CanonicalPath returns absolute clean path, handling tilde.
func CanonicalPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			p = filepath.Join(home, p[2:])
		}
	} else if p == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			p = home
		}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return abs
}

// InitBareRepo initializes a bare Git repository with post-receive hook and seeds existing workspace.
func InitBareRepo(barePath, oopsboxPath string) error {
	canonBare := CanonicalPath(barePath)
	canonBox := CanonicalPath(oopsboxPath)

	if err := os.MkdirAll(canonBare, 0755); err != nil {
		return fmt.Errorf("failed creating bare directory %s: %w", canonBare, err)
	}

	headFile := filepath.Join(canonBare, "HEAD")
	if _, err := os.Stat(headFile); os.IsNotExist(err) {
		cmd := exec.Command("git", "init", "--bare", canonBare)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git init --bare failed: %s (%w)", string(out), err)
		}
	}
	_ = exec.Command("git", "-C", canonBare, "config", "receive.advertisePushOptions", "true").Run()

	hooksDir := filepath.Join(canonBare, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed creating hooks directory: %w", err)
	}

	hookPath := filepath.Join(hooksDir, "post-receive")
	hookContent := GeneratePostReceiveHook(canonBox)
	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		return fmt.Errorf("failed writing post-receive hook: %w", err)
	}
	_ = os.Chmod(hookPath, 0755)

	// Seed bare repo if oopsboxPath exists and has files
	if entries, err := os.ReadDir(canonBox); err == nil && len(entries) > 0 {
		hasWorkspaceFiles := false
		for _, e := range entries {
			if e.Name() != ".git" {
				hasWorkspaceFiles = true
				break
			}
		}

		if hasWorkspaceFiles {
			gitDir := filepath.Join(canonBox, ".git")
			if _, err := os.Stat(gitDir); os.IsNotExist(err) {
				_ = exec.Command("git", "-C", canonBox, "init").Run()
				_ = exec.Command("git", "-C", canonBox, "add", ".").Run()
				_ = exec.Command("git", "-C", canonBox, "commit", "-m", "chore: initial server oopsbox state").Run()
				_ = exec.Command("git", "-C", canonBox, "remote", "add", "origin", canonBare).Run()
				_ = exec.Command("git", "-C", canonBox, "push", "-u", "origin", "main").Run()
			}
		}
	}

	return nil
}
