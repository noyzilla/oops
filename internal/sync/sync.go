package sync

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/noyzilla/oops/internal/remote"
)

// Options defines the parameters for a push/pull synchronization operation.
type Options struct {
	WorkDir        string
	RemoteName     string // e.g., "prod" or "@prod"
	Stack          string // optional stack filter e.g., "crm"
	RequiredStacks []string
	SyncEnv        bool // backward-compatibility flag
	SyncSecret     bool // backward-compatibility flag
	SyncAll        bool // backward-compatibility flag
}

// Push orchestrates uploading files from /@<remote>/ to the remote server, strictly filtered by .oopsallow.
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

	// Load .oopsallow rules
	allowFile := filepath.Join(capsuleDir, ".oopsallow")
	if _, err := os.Stat(allowFile); os.IsNotExist(err) {
		allowFile = filepath.Join(opts.WorkDir, ".oopsallow")
	}
	rules, err := LoadAllowRuleSet(allowFile)
	if err != nil {
		return fmt.Errorf("failed loading .oopsallow rules: %w", err)
	}

	files, err := CollectCapsuleFiles(capsuleDir, rules, opts.Stack, opts.RequiredStacks...)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		fmt.Printf("No matching files found to push in /@%s (checked against .oopsallow)\n", rName)
		return nil
	}

	fmt.Printf("==> Pushing %d file(s) from /@%s/ -> %s:%s/ (via .oopsallow whitelist)\n", len(files), rName, cfg.Host, cfg.Path)
	for _, f := range files {
		fmt.Printf("  • /@%s/%s -> %s\n", rName, f, f)
	}

	if err := remote.StreamTarFilesSSH(capsuleDir, files, cfg.Host, cfg.Path); err != nil {
		return fmt.Errorf("failed pushing files to @%s: %w", rName, err)
	}

	return nil
}

// Pull orchestrates downloading files from remote server into /@<remote>/, strictly filtered by .oopsallow.
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

	// Load .oopsallow rules
	allowFile := filepath.Join(capsuleDir, ".oopsallow")
	if _, err := os.Stat(allowFile); os.IsNotExist(err) {
		allowFile = filepath.Join(opts.WorkDir, ".oopsallow")
	}
	rules, err := LoadAllowRuleSet(allowFile)
	if err != nil {
		return fmt.Errorf("failed loading .oopsallow rules: %w", err)
	}

	// Discover remote files
	findCmd := fmt.Sprintf("cd %s && find . -maxdepth 4 -type f 2>/dev/null", cfg.Path)
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=accept-new",
		cfg.Host,
		findCmd,
	}

	out, err := exec.Command("ssh", sshArgs...).CombinedOutput()
	if err != nil && len(out) == 0 {
		return fmt.Errorf("failed scanning remote files on @%s: %w", rName, err)
	}

	lines := strings.Split(string(out), "\n")
	var matchingFiles []string
	for _, line := range lines {
		clean := strings.TrimSpace(line)
		clean = strings.TrimPrefix(clean, "./")
		if clean == "" || strings.Contains(clean, "No such file or directory") {
			continue
		}

		if !rules.IsAllowed(clean) {
			continue
		}

		// Stack filter if specified
		if opts.Stack != "" && strings.HasPrefix(clean, "compose.") && strings.HasSuffix(clean, ".local.yaml") {
			allowed := false
			for _, s := range strings.Split(opts.Stack, ",") {
				cleanStack := strings.TrimSpace(strings.TrimPrefix(s, "/"))
				if cleanStack != "" && (clean == "compose."+cleanStack+".local.yaml" || clean == "compose.local.yaml") {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}

		matchingFiles = append(matchingFiles, clean)
	}

	if len(matchingFiles) == 0 {
		fmt.Printf("No matching files found on remote server @%s to pull (checked against .oopsallow)\n", rName)
		return nil
	}

	fmt.Printf("==> Pulling %d file(s) from %s:%s/ -> /@%s/ (via .oopsallow whitelist)\n", len(matchingFiles), cfg.Host, cfg.Path, rName)
	for _, f := range matchingFiles {
		fmt.Printf("  • %s -> /@%s/%s\n", f, rName, f)
	}

	// Stream matching files using remote tar
	tarCmdStr := fmt.Sprintf("cd %s && tar -czf - %s", cfg.Path, strings.Join(matchingFiles, " "))
	sshStreamArgs := []string{
		"-o", "StrictHostKeyChecking=accept-new",
		cfg.Host,
		tarCmdStr,
	}
	sshCmd := exec.Command("ssh", sshStreamArgs...)
	localTarCmd := exec.Command("tar", "-xzf", "-", "-C", capsuleDir)

	pipeReader, pipeWriter := io.Pipe()
	sshCmd.Stdout = pipeWriter
	sshCmd.Stderr = os.Stderr
	localTarCmd.Stdin = pipeReader
	localTarCmd.Stderr = os.Stderr

	if err := sshCmd.Start(); err != nil {
		return fmt.Errorf("failed starting remote tar on @%s: %w", rName, err)
	}
	if err := localTarCmd.Start(); err != nil {
		_ = pipeWriter.Close()
		return fmt.Errorf("failed starting local extract in /@%s: %w", rName, err)
	}

	sshErr := sshCmd.Wait()
	_ = pipeWriter.Close()
	tarErr := localTarCmd.Wait()

	if sshErr != nil || tarErr != nil {
		// Fallback to scp individual files
		return pullFilesFallback(cfg.Host, cfg.Path, capsuleDir, rName, matchingFiles)
	}

	return nil
}

func pullFilesFallback(sshHost, remotePath, capsuleDir, rName string, files []string) error {
	for _, relPath := range files {
		localAbs := filepath.Join(capsuleDir, relPath)
		if err := os.MkdirAll(filepath.Dir(localAbs), 0755); err != nil {
			return err
		}
		remoteSource := fmt.Sprintf("%s:%s/%s", sshHost, remotePath, relPath)
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

// CollectCapsuleFiles discovers files to sync inside capsuleDir according to .oopsallow rules and stack filter.
func CollectCapsuleFiles(capsuleDir string, rules *AllowRuleSet, stack string, requiredStacks ...string) ([]string, error) {
	var files []string

	allowedStacks := make(map[string]bool)
	if stack != "" {
		for _, s := range strings.Split(stack, ",") {
			clean := strings.TrimSpace(strings.TrimPrefix(s, "/"))
			if clean != "" {
				allowedStacks[clean] = true
			}
		}
	}
	for _, s := range requiredStacks {
		allowedStacks[s] = true
	}

	err := filepath.Walk(capsuleDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(capsuleDir, path)
		if err != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)

		if !rules.IsAllowed(relSlash) {
			return nil
		}

		if stack != "" && strings.HasPrefix(relSlash, "compose.") {
			// If this is a stack-specific file (compose.<name>.yaml or compose.<name>.local.yaml)
			if relSlash != "compose.yaml" && relSlash != "compose.local.yaml" {
				base := strings.TrimPrefix(relSlash, "compose.")
				sName := strings.TrimSuffix(strings.TrimSuffix(base, ".local.yaml"), ".yaml")
				if sName != "" && !allowedStacks[sName] {
					return nil
				}
			}
		}

		files = append(files, relSlash)
		return nil
	})

	return files, err
}

// CollectSyncFiles is preserved for backward compatibility in existing tests.
func CollectSyncFiles(capsuleDir string, opts Options) ([]string, error) {
	rules, err := LoadAllowRuleSet(filepath.Join(capsuleDir, ".oopsallow"))
	if err != nil {
		return nil, err
	}
	allFiles, err := CollectCapsuleFiles(capsuleDir, rules, opts.Stack)
	if err != nil {
		return nil, err
	}

	if !opts.SyncEnv && !opts.SyncSecret {
		return allFiles, nil
	}

	var filtered []string
	for _, f := range allFiles {
		isEnv := f == ".env" || strings.HasPrefix(f, "config/env/")
		isSecret := strings.HasPrefix(f, "config/secrets/")
		if opts.SyncEnv && isEnv {
			filtered = append(filtered, f)
		} else if opts.SyncSecret && isSecret {
			filtered = append(filtered, f)
		}
	}
	return filtered, nil
}
