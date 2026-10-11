package remote

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// StreamTarSSH streams files from local source dir into remote path via tar over SSH.
// Exclude patterns can be provided (e.g. "*.local.yaml", "@*", ".git", "remote.yaml").
func StreamTarSSH(srcDir string, excludes []string, sshHost, remoteTargetDir string) error {
	mkdirCmd := fmt.Sprintf("mkdir -p %s", remoteTargetDir)
	if err := ExecuteSSH(sshHost, mkdirCmd, nil, os.Stderr); err != nil {
		return fmt.Errorf("failed creating remote target directory: %w", err)
	}

	var tarArgs []string
	tarArgs = append(tarArgs, "-czf", "-")
	for _, ex := range excludes {
		tarArgs = append(tarArgs, "--exclude", ex)
	}
	tarArgs = append(tarArgs, "-C", srcDir, ".")

	tarCmd := exec.Command("tar", tarArgs...)

	extractCmd := fmt.Sprintf("tar -xzf - -C %s", remoteTargetDir)
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=accept-new",
		sshHost,
		extractCmd,
	}
	sshCmd := exec.Command("ssh", sshArgs...)

	pipeReader, pipeWriter := io.Pipe()
	tarCmd.Stdout = pipeWriter
	tarCmd.Stderr = os.Stderr
	sshCmd.Stdin = pipeReader
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr

	if err := tarCmd.Start(); err != nil {
		return fmt.Errorf("failed starting local tar: %w", err)
	}
	if err := sshCmd.Start(); err != nil {
		_ = pipeWriter.Close()
		return fmt.Errorf("failed starting remote ssh extract: %w", err)
	}

	tarErr := tarCmd.Wait()
	_ = pipeWriter.Close()
	sshErr := sshCmd.Wait()

	if tarErr != nil {
		return fmt.Errorf("local tar failed: %w", tarErr)
	}
	if sshErr != nil {
		return fmt.Errorf("remote extract failed: %w", sshErr)
	}

	return nil
}

// ExecuteSSH runs a command on remote host via SSH with specified stdout and stderr.
func ExecuteSSH(sshHost, cmdStr string, stdout, stderr io.Writer) error {
	sshArgs := []string{
		"-o", "StrictHostKeyChecking=accept-new",
		sshHost,
		cmdStr,
	}
	cmd := exec.Command("ssh", sshArgs...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
