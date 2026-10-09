package sync

import (
	"path/filepath"
	"testing"
)

func TestStripRemoteSuffix(t *testing.T) {
	tests := []struct {
		input      string
		remoteName string
		expected   string
	}{
		{".env.prod", "prod", ".env"},
		{"config/env/api.prod.env", "prod", "config/env/api.env"},
		{"config/secrets/db.prod.txt", "prod", "config/secrets/db.txt"},
		{"config/secrets/mysql.prod.password", "prod", "config/secrets/mysql.password"},
		{".env.staging", "prod", ".env.staging"}, // Should not strip mismatched remote
	}

	for _, tc := range tests {
		actual := stripRemoteSuffix(tc.input, tc.remoteName)
		if actual != tc.expected {
			t.Errorf("stripRemoteSuffix(%q, %q) = %q; want %q", tc.input, tc.remoteName, actual, tc.expected)
		}
	}
}

func TestAppendRemoteSuffix(t *testing.T) {
	tests := []struct {
		input      string
		remoteName string
		expected   string
	}{
		{".env", "prod", ".env.prod"},
		{"config/env/api.env", "prod", filepath.Join("config", "env", "api.prod.env")},
		{"config/secrets/db.txt", "prod", filepath.Join("config", "secrets", "db.prod.txt")},
		{"config/secrets/mysql.password", "prod", filepath.Join("config", "secrets", "mysql.prod.password")},
	}

	for _, tc := range tests {
		actual := appendRemoteSuffix(tc.input, tc.remoteName)
		if actual != tc.expected {
			t.Errorf("appendRemoteSuffix(%q, %q) = %q; want %q", tc.input, tc.remoteName, actual, tc.expected)
		}
	}
}
