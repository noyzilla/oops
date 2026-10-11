package sync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/noyzilla/oops/internal/remote"
)

// Options defines the parameters for a synchronization operation.
type Options struct {
	WorkDir    string
	RemoteName string // e.g., "prod" or "@prod"
	SyncEnv    bool
	SyncSecret bool
	SyncAll    bool
}

// Push orchestrates uploading files from /@<remote>/ to the remote server.
func Push(opts Options) error {
	rName := strings.TrimPrefix(opts.RemoteName, "@")
	cfg, err := remote.LoadConfig(opts.WorkDir, rName)
	if err != nil {
		return err
	}

	capsuleDir := remote.CapsuleDir(opts.WorkDir, rName)
	if _, err := os.Stat(capsuleDir); os.IsNotExist(err) {
		return fmt.Errorf("remote overlay capsule %s does not exist", capsuleDir)
	}

	files, err := CollectSyncFiles(capsuleDir, opts)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		fmt.Printf("No matching files found to push in /@%s\n", rName)
		return nil
	}

	for _, relPath := range files {
		localAbs := filepath.Join(capsuleDir, relPath)
		remoteTarget := fmt.Sprintf("%s:%s/%s", cfg.Host, cfg.Path, relPath)
		remoteDir := filepath.Dir(fmt.Sprintf("%s/%s", cfg.Path, relPath))

		// Ensure remote directory exists
		mkdirCmd := fmt.Sprintf("mkdir -p %s", remoteDir)
		_ = remote.ExecuteSSH(cfg.Host, mkdirCmd, nil, nil)

		fmt.Printf("Pushing /@%s/%s -> %s\n", rName, relPath, relPath)
		scpArgs := []string{
			"-o", "StrictHostKeyChecking=accept-new",
			localAbs,
			remoteTarget,
		}
		if out, err := exec.Command("scp", scpArgs...).CombinedOutput(); err != nil {
			return fmt.Errorf("failed to push %s: %w\n%s", relPath, err, string(out))
		}
	}

	return nil
}

// Pull orchestrates downloading files from remote server into /@<remote>/.
func Pull(opts Options) error {
	rName := strings.TrimPrefix(opts.RemoteName, "@")
	cfg, err := remote.LoadConfig(opts.WorkDir, rName)
	if err != nil {
		return err
	}

	capsuleDir := remote.CapsuleDir(opts.WorkDir, rName)
	if err := os.MkdirAll(capsuleDir, 0755); err != nil {
		return fmt.Errorf("failed creating capsule dir %s: %w", capsuleDir, err)
	}

	var remoteFindCmds []string
	if opts.SyncAll || opts.SyncEnv {
		remoteFindCmds = append(remoteFindCmds, `find . -maxdepth 1 -name ".env" 2>/dev/null`, `find config/env -type f -name "*.env" 2>/dev/null`)
	}
	if opts.SyncAll || opts.SyncSecret {
		remoteFindCmds = append(remoteFindCmds, `find config/secrets -type f 2>/dev/null`)
	}

	cmdStr := fmt.Sprintf("cd %s && (%s)", cfg.Path, strings.Join(remoteFindCmds, " ; "))
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=accept-new",
		cfg.Host,
		cmdStr,
	}

	out, _ := exec.Command("ssh", sshArgs...).CombinedOutput()
	lines := strings.Split(string(out), "\n")
	var remoteFiles []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "./")
		if line != "" && !strings.Contains(line, "No such file or directory") {
			remoteFiles = append(remoteFiles, line)
		}
	}

	if len(remoteFiles) == 0 {
		fmt.Printf("No remote files found to pull for @%s\n", rName)
		return nil
	}

	for _, relPath := range remoteFiles {
		localAbs := filepath.Join(capsuleDir, relPath)
		remoteSource := fmt.Sprintf("%s:%s/%s", cfg.Host, cfg.Path, relPath)

		if err := os.MkdirAll(filepath.Dir(localAbs), 0755); err != nil {
			return fmt.Errorf("failed creating local dir for %s: %w", relPath, err)
		}

		fmt.Printf("Pulling %s -> /@%s/%s\n", relPath, rName, relPath)
		scpArgs := []string{
			"-o", "StrictHostKeyChecking=accept-new",
			remoteSource,
			localAbs,
		}
		if out, err := exec.Command("scp", scpArgs...).CombinedOutput(); err != nil {
			return fmt.Errorf("failed to pull %s: %w\n%s", relPath, err, string(out))
		}
	}

	return nil
}

// CollectSyncFiles discovers files to sync inside capsuleDir according to sync options.
func CollectSyncFiles(capsuleDir string, opts Options) ([]string, error) {
	var files []string

	// 1. Root .env and config/env/*.env
	if opts.SyncAll || opts.SyncEnv {
		rootEnv := filepath.Join(capsuleDir, ".env")
		if _, err := os.Stat(rootEnv); err == nil {
			files = append(files, ".env")
		}

		envDir := filepath.Join(capsuleDir, "config", "env")
		if entries, err := os.ReadDir(envDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".env") {
					files = append(files, filepath.Join("config", "env", e.Name()))
				}
			}
		}
	}

	// 2. config/secrets/*
	if opts.SyncAll || opts.SyncSecret {
		secretsDir := filepath.Join(capsuleDir, "config", "secrets")
		if err := filepath.Walk(secretsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				rel, err := filepath.Rel(capsuleDir, path)
				if err == nil {
					files = append(files, rel)
				}
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}

	return files, nil
}
