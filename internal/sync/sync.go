package sync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Options defines the parameters for a synchronization operation.
type Options struct {
	WorkDir    string
	RemoteName string
	SSHTarget  string
	SyncEnv    bool
	SyncSecret bool
}

// Push orchestrates uploading files to the remote server.
func Push(opts Options) error {
	// Ensure remote directories exist
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=accept-new",
		opts.SSHTarget,
		"mkdir -p ~/oopsbox/config/env ~/oopsbox/config/secrets",
	}
	if err := exec.Command("ssh", sshArgs...).Run(); err != nil {
		return fmt.Errorf("failed to create remote directories: %w", err)
	}

	var filesToSync []string

	if opts.SyncEnv {
		envFiles, err := findLocalEnv(opts.WorkDir, opts.RemoteName)
		if err != nil {
			return err
		}
		filesToSync = append(filesToSync, envFiles...)
	}

	if opts.SyncSecret {
		secretFiles, err := findLocalSecret(opts.WorkDir, opts.RemoteName)
		if err != nil {
			return err
		}
		filesToSync = append(filesToSync, secretFiles...)
	}

	for _, localRelPath := range filesToSync {
		localAbsPath := filepath.Join(opts.WorkDir, localRelPath)
		remoteRelPath := stripRemoteSuffix(localRelPath, opts.RemoteName)
		remoteTarget := fmt.Sprintf("%s:~/oopsbox/%s", opts.SSHTarget, remoteRelPath)

		fmt.Printf("Pushing %s -> %s\n", localRelPath, remoteRelPath)
		scpArgs := []string{
			"-o", "StrictHostKeyChecking=accept-new",
			localAbsPath,
			remoteTarget,
		}
		if out, err := exec.Command("scp", scpArgs...).CombinedOutput(); err != nil {
			return fmt.Errorf("failed to push %s: %w\n%s", localRelPath, err, string(out))
		}
	}

	return nil
}

// Pull orchestrates downloading files from the remote server.
func Pull(opts Options) error {
	var remoteFindCmds []string
	if opts.SyncEnv {
		remoteFindCmds = append(remoteFindCmds, `find . -maxdepth 1 -name ".env" 2>/dev/null`, `find config/env -name "*.env" 2>/dev/null`)
	}
	if opts.SyncSecret {
		remoteFindCmds = append(remoteFindCmds, `find config/secrets -type f 2>/dev/null`)
	}

	cmdStr := fmt.Sprintf("cd ~/oopsbox && (%s)", strings.Join(remoteFindCmds, " ; "))
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=accept-new",
		opts.SSHTarget,
		cmdStr,
	}

	out, err := exec.Command("ssh", sshArgs...).CombinedOutput()
	if err != nil {
		// If it's a 1 exit code from find when nothing exists, it's fine, but let's just parse the output.
		// find without match usually exits 0, but if dir missing it might exit 1 and print to stderr.
		// We ignore error here and just parse stdout.
	}

	lines := strings.Split(string(out), "\n")
	var remoteFiles []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "./")
		if line != "" && !strings.Contains(line, "No such file or directory") {
			remoteFiles = append(remoteFiles, line)
		}
	}

	for _, remoteRelPath := range remoteFiles {
		localRelPath := appendRemoteSuffix(remoteRelPath, opts.RemoteName)
		localAbsPath := filepath.Join(opts.WorkDir, localRelPath)
		remoteSource := fmt.Sprintf("%s:~/oopsbox/%s", opts.SSHTarget, remoteRelPath)

		if err := os.MkdirAll(filepath.Dir(localAbsPath), 0755); err != nil {
			return fmt.Errorf("failed to create local dir for %s: %w", localRelPath, err)
		}

		fmt.Printf("Pulling %s -> %s\n", remoteRelPath, localRelPath)
		scpArgs := []string{
			"-o", "StrictHostKeyChecking=accept-new",
			remoteSource,
			localAbsPath,
		}
		if out, err := exec.Command("scp", scpArgs...).CombinedOutput(); err != nil {
			return fmt.Errorf("failed to pull %s: %w\n%s", remoteRelPath, err, string(out))
		}
	}

	return nil
}

func findLocalEnv(workDir, remoteName string) ([]string, error) {
	var matches []string

	// root .env.<remote>
	rootEnv := fmt.Sprintf(".env.%s", remoteName)
	if _, err := os.Stat(filepath.Join(workDir, rootEnv)); err == nil {
		matches = append(matches, rootEnv)
	}

	// config/env/*.<remote>.env
	envDir := filepath.Join(workDir, "config", "env")
	if entries, err := os.ReadDir(envDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), fmt.Sprintf(".%s.env", remoteName)) {
				matches = append(matches, filepath.Join("config", "env", e.Name()))
			}
		}
	}

	return matches, nil
}

func findLocalSecret(workDir, remoteName string) ([]string, error) {
	var matches []string

	// config/secrets/*.<remote>.*
	secretDir := filepath.Join(workDir, "config", "secrets")
	if entries, err := os.ReadDir(secretDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				parts := strings.Split(e.Name(), ".")
				if len(parts) >= 3 {
					// e.g. name.remote.ext
					if parts[len(parts)-2] == remoteName {
						matches = append(matches, filepath.Join("config", "secrets", e.Name()))
					}
				}
			}
		}
	}

	return matches, nil
}

func stripRemoteSuffix(path, remoteName string) string {
	base := filepath.Base(path)
	dir := filepath.Dir(path)

	// .env.<remote> -> .env
	if base == fmt.Sprintf(".env.%s", remoteName) {
		return filepath.Join(dir, ".env")
	}

	// <name>.<remote>.<ext> -> <name>.<ext>
	parts := strings.Split(base, ".")
	if len(parts) >= 3 && parts[len(parts)-2] == remoteName {
		newBase := strings.Join(append(parts[:len(parts)-2], parts[len(parts)-1]), ".")
		return filepath.Join(dir, newBase)
	}

	return path
}

func appendRemoteSuffix(path, remoteName string) string {
	base := filepath.Base(path)
	dir := filepath.Dir(path)

	// .env -> .env.<remote>
	if base == ".env" {
		return filepath.Join(dir, fmt.Sprintf(".env.%s", remoteName))
	}

	// <name>.<ext> -> <name>.<remote>.<ext>
	parts := strings.Split(base, ".")
	if len(parts) >= 2 {
		newBase := strings.Join(append(parts[:len(parts)-1], append([]string{remoteName}, parts[len(parts)-1])...), ".")
		return filepath.Join(dir, newBase)
	}

	return path
}
