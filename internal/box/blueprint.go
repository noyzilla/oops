package box

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	LatestReleaseURL = "https://github.com/noyzilla/oops/releases/latest/download/oopsbox.tar.gz"
	MainArchiveURL   = "https://github.com/noyzilla/oops/archive/refs/heads/main.tar.gz"
)

// FetchAndExtractBlueprint downloads the oopsbox template archive and extracts it to targetDir.
// It tries the latest GitHub Release asset first, falling back to the repository main branch archive.
func FetchAndExtractBlueprint(targetDir string) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", targetDir, err)
	}

	client := &http.Client{Timeout: 60 * time.Second}

	// Try release asset first
	err := downloadAndExtract(client, LatestReleaseURL, targetDir, "")
	if err == nil {
		return nil
	}

	// Fallback to main branch archive
	fmt.Printf("Notice: Release asset unavailable (%v). Falling back to main branch archive...\n", err)
	return downloadAndExtract(client, MainArchiveURL, targetDir, "oopsbox")
}

func downloadAndExtract(client *http.Client, url, targetDir, subDirFilter string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "oops-cli")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP request failed with status %d (%s)", resp.StatusCode, resp.Status)
	}

	return ExtractTarGz(resp.Body, targetDir, subDirFilter)
}

// ExtractTarGz extracts an archive stream into targetDir.
// If subDirFilter is non-empty (e.g. "oopsbox"), only files under `*/<subDirFilter>/` are extracted,
// and the path prefix up to `<subDirFilter>/` is stripped.
func ExtractTarGz(r io.Reader, targetDir string, subDirFilter string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	extractedCount := 0

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed reading tar stream: %w", err)
		}

		relPath := header.Name
		if subDirFilter != "" {
			// Find prefix ending with /<subDirFilter>/
			idx := strings.Index(relPath, "/"+subDirFilter+"/")
			if idx == -1 {
				// Also check if path starts with <subDirFilter>/
				if strings.HasPrefix(relPath, subDirFilter+"/") {
					relPath = strings.TrimPrefix(relPath, subDirFilter+"/")
				} else {
					continue
				}
			} else {
				relPath = relPath[idx+len("/"+subDirFilter+"/"):]
			}
		}

		// Clean and prevent Zip Slip
		relPath = filepath.Clean(relPath)
		if strings.HasPrefix(relPath, "..") || strings.HasPrefix(relPath, "/") {
			continue
		}
		if relPath == "." || relPath == "" {
			continue
		}

		destPath := filepath.Join(targetDir, relPath)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
			extractedCount++
		}
	}

	if extractedCount == 0 {
		return fmt.Errorf("no files extracted from archive")
	}

	return nil
}
