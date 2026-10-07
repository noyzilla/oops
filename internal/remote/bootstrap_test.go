package remote

import (
	"strings"
	"testing"
)

func TestGenerateCOSPluginScript(t *testing.T) {
	script := GenerateCOSPluginScript()
	if !strings.Contains(script, "/var/lib/google/docker-cli-plugins") {
		t.Errorf("expected script to contain COS plugin dir, got:\n%s", script)
	}
	if !strings.Contains(script, "cliPluginsExtraDirs") {
		t.Errorf("expected script to configure cliPluginsExtraDirs, got:\n%s", script)
	}
}

func TestGenerateBootstrapRemoteScript(t *testing.T) {
	barePath := "~/.oops/repos/mybox.git"
	boxPath := "~/oopsbox"

	script := GenerateBootstrapRemoteScript(barePath, boxPath)
	if !strings.Contains(script, "BARE_PATH=\"~/.oops/repos/mybox.git\"") {
		t.Errorf("expected script to contain BARE_PATH, got:\n%s", script)
	}
	if !strings.Contains(script, "BOX_PATH=\"~/oopsbox\"") {
		t.Errorf("expected script to contain BOX_PATH, got:\n%s", script)
	}
	if !strings.Contains(script, "init-bare") {
		t.Errorf("expected script to call init-bare, got:\n%s", script)
	}
}
