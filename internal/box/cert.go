package box

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// InstallCACertificate locates Caddy's root CA certificate and installs it into the OS trust store.
func InstallCACertificate(workDir string) error {
	certPath := filepath.Join(workDir, "stacks", "edge", "data", "caddy", "pki", "authorities", "local", "root.crt")

	// If not found in host bind mount, try docker cp from caddy-proxy container
	if _, err := os.Stat(certPath); err != nil {
		tmpCert := filepath.Join(os.TempDir(), "oops-root.crt")
		cpCmd := exec.Command("docker", "cp", "caddy-proxy:/data/caddy/pki/authorities/local/root.crt", tmpCert)
		if cpErr := cpCmd.Run(); cpErr == nil {
			certPath = tmpCert
			defer os.Remove(tmpCert)
		} else {
			return fmt.Errorf("could not locate Caddy root CA certificate at %s or inside running caddy-proxy container", certPath)
		}
	}

	fmt.Printf("==> Installing local Caddy root CA certificate (%s)...\n", certPath)

	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("sudo", "security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", "/Library/Keychains/System.keychain", certPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to add certificate to macOS Keychain: %w", err)
		}
		fmt.Println("==> Certificate successfully added to macOS System Keychain.")
		return nil

	case "linux":
		// Copy to /usr/local/share/ca-certificates/
		dest := "/usr/local/share/ca-certificates/oops-root.crt"
		copyCmd := exec.Command("sudo", "cp", certPath, dest)
		copyCmd.Stdin = os.Stdin
		copyCmd.Stdout = os.Stdout
		copyCmd.Stderr = os.Stderr
		if err := copyCmd.Run(); err != nil {
			return fmt.Errorf("failed to copy cert to /usr/local/share/ca-certificates/: %w", err)
		}

		updateCmd := exec.Command("sudo", "update-ca-certificates")
		updateCmd.Stdin = os.Stdin
		updateCmd.Stdout = os.Stdout
		updateCmd.Stderr = os.Stderr
		if err := updateCmd.Run(); err != nil {
			return fmt.Errorf("failed to run update-ca-certificates: %w", err)
		}
		fmt.Println("==> Certificate successfully added to Linux trust store.")
		return nil

	default:
		return fmt.Errorf("automatic CA installation not supported on OS: %s", runtime.GOOS)
	}
}
