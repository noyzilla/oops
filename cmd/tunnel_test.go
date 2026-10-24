package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noyzilla/oops/internal/tunnel"
)

func TestTunnelCommand_Help(t *testing.T) {
	cmd := newTunnelCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil error on --help, got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "tunnel [@remote] [services...]") {
		t.Errorf("expected usage string in help output, got: %s", out)
	}
	if !strings.Contains(out, "up") || !strings.Contains(out, "down") || !strings.Contains(out, "list") {
		t.Errorf("expected subcommands in help output, got: %s", out)
	}
}

func TestTunnelCommand_Registration(t *testing.T) {
	root := GetRootCommand()
	found := false
	for _, c := range root.Commands() {
		if c.Name() == "tunnel" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected 'tunnel' command to be registered in rootCmd")
	}
}

func TestTunnelUpCommand_ArgsValidation(t *testing.T) {
	upCmd := newTunnelUpCmd()
	upCmd.SetArgs([]string{}) // Missing @remote
	err := upCmd.Execute()
	if err == nil {
		t.Errorf("expected error when @remote argument is missing, got nil")
	}
}

func TestTunnelDownCommand_ArgsValidation(t *testing.T) {
	downCmd := newTunnelDownCmd()
	downCmd.SetArgs([]string{}) // Missing @remote
	err := downCmd.Execute()
	if err == nil {
		t.Errorf("expected error when @remote argument is missing, got nil")
	}
}

func TestFormatTunnelList(t *testing.T) {
	tmpDir := t.TempDir()
	composeYAML := `
x-oops-stack:
  - mysql
  - redis
services:
  mysql:
    image: mysql:8.4
  redis:
    image: redis:7-alpine
`
	if err := os.WriteFile(filepath.Join(tmpDir, "compose.db.yaml"), []byte(composeYAML), 0644); err != nil {
		t.Fatalf("failed creating test compose file: %v", err)
	}

	tunnels := []tunnel.TunnelStatus{
		{
			RemoteName: "b9tpe",
			Container:  "oops-tunnel-b9tpe",
			Status:     "Up 5 minutes",
			Endpoints: []string{
				"mysql.b9tpe.tunnel.oops:3306",
				"redis.b9tpe.tunnel.oops:6379",
			},
		},
	}

	var buf bytes.Buffer
	if err := formatTunnelList(&buf, tunnels, tmpDir); err != nil {
		t.Fatalf("formatTunnelList failed: %v", err)
	}

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d:\n%s", len(lines), out)
	}

	header := lines[0]
	if !strings.Contains(header, "REMOTE") || !strings.Contains(header, "CONTAINER") || !strings.Contains(header, "STATUS") || !strings.Contains(header, "ENDPOINTS") {
		t.Errorf("header missing expected columns: %s", header)
	}
	if strings.Contains(header, "FORWARDED") {
		t.Errorf("expected FORWARDED column to be removed, got: %s", header)
	}

	line1 := lines[1]
	if !strings.Contains(line1, "@b9tpe") || !strings.Contains(line1, "oops-tunnel-b9tpe") || !strings.Contains(line1, "Up 5 minutes") || !strings.Contains(line1, "mysql.b9tpe.tunnel.oops:3306 (/db/mysql)") {
		t.Errorf("unexpected line 1: %s", line1)
	}

	line2 := lines[2]
	if !strings.Contains(line2, "redis.b9tpe.tunnel.oops:6379 (/db/redis)") {
		t.Errorf("unexpected line 2: %s", line2)
	}
	// Line 2 should have blank leading remote/container/status
	trimmedLine2 := strings.TrimSpace(line2)
	if trimmedLine2 != "redis.b9tpe.tunnel.oops:6379 (/db/redis)" {
		t.Errorf("line 2 should only contain endpoint, got: %s", trimmedLine2)
	}
}

func TestTunnelUpCommand_Flags(t *testing.T) {
	upCmd := newTunnelUpCmd()
	meshFlag := upCmd.Flags().Lookup("mesh")
	if meshFlag == nil {
		t.Fatalf("expected --mesh flag on tunnel up")
	}
	portFlag := upCmd.Flags().Lookup("port")
	if portFlag == nil {
		t.Fatalf("expected --port flag on tunnel up")
	}
}


