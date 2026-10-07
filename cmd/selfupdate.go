package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Name    string        `json:"name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func newSelfUpdateCmd() *cobra.Command {
	var checkOnly bool
	var force bool

	cmd := &cobra.Command{
		Use:   "selfupdate",
		Short: "Self-update oops CLI to the latest released version",
		Long:  "Checks GitHub for the latest release of oops, downloads the appropriate binary for your OS and architecture, and replaces the current executable.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSelfUpdate(checkOnly, force)
		},
	}

	cmd.Flags().BoolVarP(&checkOnly, "check", "c", false, "Check for available updates without applying")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force update even if already on the latest version")

	return cmd
}

func runSelfUpdate(checkOnly, force bool) error {
	currentVer := GetVersion()
	fmt.Printf("==> Current oops version: %s (%s/%s)\n", currentVer, runtime.GOOS, runtime.GOARCH)
	fmt.Println("==> Checking GitHub for latest release...")

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", "https://api.github.com/repos/noyzilla/oops/releases/latest", nil)
	if err != nil {
		return fmt.Errorf("failed creating request: %w", err)
	}
	req.Header.Set("User-Agent", "oops-cli/"+currentVer)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed fetching latest release information: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned status %d %s", resp.StatusCode, resp.Status)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return fmt.Errorf("failed parsing release metadata: %w", err)
	}

	latestVer := strings.TrimPrefix(rel.TagName, "v")
	if latestVer == "" {
		return fmt.Errorf("could not find valid tag in latest release metadata")
	}

	isNewer := isVersionNewer(currentVer, latestVer)

	if !isNewer && !force {
		fmt.Printf("✓ Oops is already up to date (version %s)\n", currentVer)
		return nil
	}

	if isNewer {
		fmt.Printf("==> Found new version: %s (current: %s)\n", rel.TagName, currentVer)
	} else if force {
		fmt.Printf("==> Force updating/reinstalling version %s...\n", rel.TagName)
	}

	if checkOnly {
		fmt.Println("==> Run 'oops selfupdate' to install the new version.")
		return nil
	}

	// Determine asset name
	expectedAsset := fmt.Sprintf("oops-%s-%s", runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	for _, asset := range rel.Assets {
		if asset.Name == expectedAsset {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		// Fallback to direct releases download URL
		downloadURL = fmt.Sprintf("https://github.com/noyzilla/oops/releases/download/%s/%s", rel.TagName, expectedAsset)
	}

	// Locate current executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed resolving path of current executable: %w", err)
	}
	if realPath, err := filepath.EvalSymlinks(execPath); err == nil {
		execPath = realPath
	}

	fmt.Printf("==> Target binary path: %s\n", execPath)
	fmt.Printf("==> Downloading %s...\n", downloadURL)

	dlReq, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed creating download request: %w", err)
	}
	dlReq.Header.Set("User-Agent", "oops-cli/"+currentVer)

	dlResp, err := client.Do(dlReq)
	if err != nil {
		return fmt.Errorf("failed downloading release binary: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with HTTP %d %s", dlResp.StatusCode, dlResp.Status)
	}

	// Create temp file in same directory or system temp
	tmpDir := filepath.Dir(execPath)
	tmpFile, err := os.CreateTemp(tmpDir, "oops-selfupdate-*")
	if err != nil {
		// If cannot write to binary dir directly (permissions), use os.TempDir()
		tmpFile, err = os.CreateTemp("", "oops-selfupdate-*")
		if err != nil {
			return fmt.Errorf("failed creating temporary file: %w", err)
		}
	}
	tmpFilePath := tmpFile.Name()
	defer os.Remove(tmpFilePath)

	if _, err := io.Copy(tmpFile, dlResp.Body); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed saving downloaded binary: %w", err)
	}
	_ = tmpFile.Close()

	if err := os.Chmod(tmpFilePath, 0755); err != nil {
		return fmt.Errorf("failed setting executable permissions: %w", err)
	}

	// Attempt replace via atomic rename
	if err := os.Rename(tmpFilePath, execPath); err != nil {
		// If permission denied, attempt sudo atomic move
		fmt.Println("==> Permission required to replace binary. Requesting sudo...")
		cmd := exec.Command("sudo", "mv", "-f", tmpFilePath, execPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed replacing binary at %s: %w\nPermission required. Please run: sudo oops selfupdate", execPath, err)
		}
		_ = exec.Command("sudo", "chmod", "755", execPath).Run()
	}

	fmt.Printf("\n✓ Successfully updated oops to %s at %s\n", rel.TagName, execPath)
	return nil
}

// isVersionNewer returns true if latest is semantically greater than current.
func isVersionNewer(current, latest string) bool {
	if current == "dev" || strings.HasPrefix(current, "dev-") {
		return true
	}

	cParts := parseSemver(current)
	lParts := parseSemver(latest)

	for i := 0; i < 3; i++ {
		if lParts[i] > cParts[i] {
			return true
		}
		if lParts[i] < cParts[i] {
			return false
		}
	}
	return false
}

func parseSemver(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	var res [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		// strip any extra pre-release suffix e.g. "1-beta"
		numStr := strings.Split(parts[i], "-")[0]
		num, _ := strconv.Atoi(numStr)
		res[i] = num
	}
	return res
}
