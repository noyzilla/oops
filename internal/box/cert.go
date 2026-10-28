package box

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// ExtractCACertificate extracts Caddy's root CA certificate directly from the caddy-proxy container.
func ExtractCACertificate(dstPath string) error {
	cpCmd := exec.Command("docker", "cp", "caddy-proxy:/data/caddy/pki/authorities/local/root.crt", dstPath)
	if out, err := cpCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("could not extract Caddy root CA certificate from caddy-proxy container: %s (ensure caddy-proxy container exists)", string(out))
	}
	return nil
}

// InstallCACertificate extracts Caddy's root CA certificate from the caddy-proxy container and installs it into the OS trust store.
func InstallCACertificate(workDir string) error {
	tmpFile, err := os.CreateTemp("", "oops-root-*.crt")
	if err != nil {
		return fmt.Errorf("failed to create temporary certificate file: %w", err)
	}
	tmpCert := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpCert)

	if err := ExtractCACertificate(tmpCert); err != nil {
		return err
	}

	fmt.Printf("==> Installing local Caddy root CA certificate (%s)...\n", tmpCert)

	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("sudo", "security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", "/Library/Keychains/System.keychain", tmpCert)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to add certificate to macOS Keychain: %w", err)
		}
		fmt.Println("==> Certificate successfully added to macOS System Keychain.")
		fmt.Println("    Tip: For Firefox / Firefox Dev, enable 'security.enterprise_roots.enabled = true' in about:config")
		return nil

	case "linux":
		// Copy to /usr/local/share/ca-certificates/
		dest := "/usr/local/share/ca-certificates/oops-root.crt"
		copyCmd := exec.Command("sudo", "cp", tmpCert, dest)
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
		fmt.Println("    Tip: For Firefox / Firefox Dev, enable 'security.enterprise_roots.enabled = true' in about:config")
		return nil

	default:
		return fmt.Errorf("automatic CA installation not supported on OS: %s", runtime.GOOS)
	}
}

