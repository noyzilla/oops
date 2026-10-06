package key

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/crypto/ssh"
)

// DefaultKeyPath returns the canonical path to the oops private key (~/.oops/id_ed25519)
func DefaultKeyPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed resolving home directory: %w", err)
	}
	return filepath.Join(home, ".oops", "id_ed25519"), nil
}

// EnsureKeyPair guarantees an ed25519 keypair exists at keyPath, generating one if missing
func EnsureKeyPair(keyPath string) (pubKeyStr string, generated bool, err error) {
	pubPath := keyPath + ".pub"

	if _, statErr := os.Stat(keyPath); statErr == nil {
		pubBytes, readErr := os.ReadFile(pubPath)
		if readErr == nil && len(pubBytes) > 0 {
			return strings.TrimSpace(string(pubBytes)), false, nil
		}
	}

	pubKeyStr, err = GenerateKeyPair(keyPath)
	if err != nil {
		return "", false, err
	}
	return pubKeyStr, true, nil
}

// GenerateKeyPair unconditionally generates a new ed25519 keypair at keyPath
func GenerateKeyPair(keyPath string) (string, error) {
	pubPath := keyPath + ".pub"
	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed creating directory %s: %w", dir, err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("failed generating ed25519 keypair: %w", err)
	}

	pemBlock, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return "", fmt.Errorf("failed marshaling private key: %w", err)
	}
	privBytes := pem.EncodeToMemory(pemBlock)

	sshPubKey, err := ssh.NewPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("failed creating ssh public key: %w", err)
	}
	pubBytes := ssh.MarshalAuthorizedKey(sshPubKey)
	pubStr := strings.TrimSpace(string(pubBytes))

	if err := os.WriteFile(keyPath, privBytes, 0600); err != nil {
		return "", fmt.Errorf("failed writing private key to %s: %w", keyPath, err)
	}
	if err := os.WriteFile(pubPath, []byte(pubStr+"\n"), 0644); err != nil {
		return "", fmt.Errorf("failed writing public key to %s: %w", pubPath, err)
	}

	return pubStr, nil
}

// SetKeyPair writes a custom private key string (and optional public key) to keyPath
func SetKeyPair(keyPath string, privKeyPEM string) (string, error) {
	privKeyPEM = strings.TrimSpace(privKeyPEM)
	if privKeyPEM == "" {
		return "", errors.New("private key input cannot be empty")
	}

	block, _ := pem.Decode([]byte(privKeyPEM))
	if block == nil {
		return "", errors.New("invalid PEM format for private key")
	}

	rawKey, err := ssh.ParseRawPrivateKey([]byte(privKeyPEM))
	if err != nil {
		return "", fmt.Errorf("failed parsing private key: %w", err)
	}

	signer, err := ssh.NewSignerFromKey(rawKey)
	if err != nil {
		return "", fmt.Errorf("failed deriving public key from private key: %w", err)
	}

	pubBytes := ssh.MarshalAuthorizedKey(signer.PublicKey())
	pubStr := strings.TrimSpace(string(pubBytes))

	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed creating directory %s: %w", dir, err)
	}

	if err := os.WriteFile(keyPath, []byte(privKeyPEM+"\n"), 0600); err != nil {
		return "", fmt.Errorf("failed saving private key: %w", err)
	}
	if err := os.WriteFile(keyPath+".pub", []byte(pubStr+"\n"), 0644); err != nil {
		return "", fmt.Errorf("failed saving public key: %w", err)
	}

	return pubStr, nil
}

// GetAddDeployKeyURL parses a repo URL and returns direct Deploy Key URL if supported
func GetAddDeployKeyURL(repoURL string) string {
	repoURL = strings.TrimSpace(repoURL)

	// GitHub SSH: git@github.com:owner/repo.git or git@github.com:owner/repo
	// GitHub HTTPS: https://github.com/owner/repo.git or https://github.com/owner/repo
	ghRegex := regexp.MustCompile(`^(?:git@github\.com:|https://github\.com/)([^/]+)/([^/\.]+)(?:\.git)?$`)
	if matches := ghRegex.FindStringSubmatch(repoURL); len(matches) == 3 {
		owner := matches[1]
		repo := matches[2]
		return fmt.Sprintf("https://github.com/%s/%s/settings/keys/new", owner, repo)
	}

	// GitLab SSH: git@gitlab.com:owner/repo.git
	// GitLab HTTPS: https://gitlab.com/owner/repo.git
	glRegex := regexp.MustCompile(`^(?:git@gitlab\.com:|https://gitlab\.com/)([^/]+)/([^/\.]+)(?:\.git)?$`)
	if matches := glRegex.FindStringSubmatch(repoURL); len(matches) == 3 {
		owner := matches[1]
		repo := matches[2]
		return fmt.Sprintf("https://gitlab.com/%s/%s/-/settings/repository", owner, repo)
	}

	return ""
}
