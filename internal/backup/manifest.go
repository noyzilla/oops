package backup

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const (
	ManifestFileName = ".oops-backup.json"
	CurrentVersion   = "0.3.0"
)

// DataManifest contains metadata for filesystem volume backups
type DataManifest struct {
	Version     string   `json:"version"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	CreatedAt   string   `json:"created_at"`
	OopsVersion string   `json:"oops_version,omitempty"`
	SourcePaths []string `json:"source_paths"`
	FileCount   int      `json:"file_count"`
	TotalBytes  int64    `json:"total_bytes"`
}

// DBManifest contains metadata for database dumps
type DBManifest struct {
	Engine      string `json:"engine"`
	Container   string `json:"container"`
	Database    string `json:"database"`
	CreatedAt   string `json:"created_at"`
	OopsVersion string `json:"oops_version,omitempty"`
}

// WriteSQLHeader writes the metadata comment block at the beginning of a SQL dump stream
func WriteSQLHeader(w io.Writer, m DBManifest) {
	if m.CreatedAt == "" {
		m.CreatedAt = time.Now().Format(time.RFC3339)
	}
	if m.OopsVersion == "" {
		m.OopsVersion = CurrentVersion
	}

	header := fmt.Sprintf(`-- ==============================================================================
-- Oops Database Backup Manifest
-- ==============================================================================
-- Engine: %s
-- Container: %s
-- Database: %s
-- CreatedAt: %s
-- OopsVersion: %s
-- ==============================================================================

`, m.Engine, m.Container, m.Database, m.CreatedAt, m.OopsVersion)

	_, _ = io.WriteString(w, header)
}

// ReadSQLHeader reads the beginning of a SQL dump stream or .sql.gz file to extract DBManifest
func ReadSQLHeader(r io.Reader) (*DBManifest, error) {
	scanner := bufio.NewScanner(r)
	manifest := &DBManifest{}
	foundAny := false
	lineCount := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineCount++
		if lineCount > 30 {
			break
		}

		if strings.HasPrefix(line, "-- Engine:") {
			manifest.Engine = strings.TrimSpace(strings.TrimPrefix(line, "-- Engine:"))
			foundAny = true
		} else if strings.HasPrefix(line, "-- Container:") {
			manifest.Container = strings.TrimSpace(strings.TrimPrefix(line, "-- Container:"))
			foundAny = true
		} else if strings.HasPrefix(line, "-- Database:") {
			manifest.Database = strings.TrimSpace(strings.TrimPrefix(line, "-- Database:"))
			foundAny = true
		} else if strings.HasPrefix(line, "-- CreatedAt:") {
			manifest.CreatedAt = strings.TrimSpace(strings.TrimPrefix(line, "-- CreatedAt:"))
			foundAny = true
		} else if strings.HasPrefix(line, "-- OopsVersion:") {
			manifest.OopsVersion = strings.TrimSpace(strings.TrimPrefix(line, "-- OopsVersion:"))
			foundAny = true
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read sql header: %w", err)
	}

	if !foundAny {
		return nil, nil
	}
	return manifest, nil
}

// ReadDBManifestFromFile opens a .sql or .sql.gz file and extracts DBManifest
func ReadDBManifestFromFile(filePath string) (*DBManifest, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if strings.HasSuffix(filePath, ".gz") {
		gzr, err := gzip.NewReader(f)
		if err != nil {
			return nil, err
		}
		defer gzr.Close()
		return ReadSQLHeader(gzr)
	}

	return ReadSQLHeader(f)
}

// ReadDataManifestFromArchive reads .oops-backup.json from a .tar.gz archive
func ReadDataManifestFromArchive(archivePath string) (*DataManifest, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if hdr.Name == ManifestFileName || strings.HasSuffix(hdr.Name, "/"+ManifestFileName) {
			var m DataManifest
			if err := json.NewDecoder(tr).Decode(&m); err != nil {
				return nil, err
			}
			return &m, nil
		}
	}

	return nil, nil
}
