package remote

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/noyzilla/oops/internal/key"
)

// GenerateCOSPluginScript returns the bash script string for installing Docker Compose CLI plugin on COS.
func GenerateCOSPluginScript() string {
	return `#!/bin/bash
set -euo pipefail

PLUGIN_DIR="/var/lib/google/docker-cli-plugins"
PLUGIN_PATH="${PLUGIN_DIR}/docker-compose"

echo "==> Verifying/Installing Docker Compose CLI plugin on COS..."

sudo mkdir -p "${PLUGIN_DIR}"

ARCH="$(uname -m)"
case "${ARCH}" in
    x86_64)       COMPOSE_ARCH="x86_64" ;;
    aarch64|arm64) COMPOSE_ARCH="aarch64" ;;
    *) echo "Unsupported architecture: ${ARCH}" >&2; exit 1 ;;
esac

if [ ! -f "${PLUGIN_PATH}" ]; then
    sudo curl -fsSL \
        "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-${COMPOSE_ARCH}" \
        -o "${PLUGIN_PATH}"
    sudo chmod 755 "${PLUGIN_PATH}"
fi

configure_docker_json() {
    local target_file="$1"
    local dir_path
    dir_path="$(dirname "${target_file}")"

    sudo mkdir -p "${dir_path}"

    if [ ! -f "${target_file}" ]; then
        sudo tee "${target_file}" > /dev/null <<EOF
{
  "cliPluginsExtraDirs": [
    "${PLUGIN_DIR}"
  ]
}
EOF
    else
        if ! grep -q "${PLUGIN_DIR}" "${target_file}"; then
            if grep -q '"cliPluginsExtraDirs"' "${target_file}"; then
                sudo sed -i "s|\(\"cliPluginsExtraDirs\"[[:space:]]*:[[:space:]]*\[\)|\1\n    \"${PLUGIN_DIR}\",|" "${target_file}"
            else
                sudo sed -i "0,/{/s|{|{\n  \"cliPluginsExtraDirs\": [\n    \"${PLUGIN_DIR}\"\n  ],|" "${target_file}"
            fi
        fi
    fi
}

configure_docker_json "${HOME}/.docker/config.json"
sudo chown -R "$(id -u):$(id -g)" "${HOME}/.docker" 2>/dev/null || true
configure_docker_json "/root/.docker/config.json" 2>/dev/null || true

docker compose version || true
`
}

// GenerateBootstrapRemoteScript generates the bash script to execute on the remote server via SSH.
func GenerateBootstrapRemoteScript(barePath, boxPath string) string {
	cosScript := GenerateCOSPluginScript()

	return fmt.Sprintf(`#!/bin/bash
set -euo pipefail

BARE_PATH="%s"
BOX_PATH="%s"

# 1. Detect COS vs standard Linux
IS_COS=0
if [ -f /etc/os-release ] && grep -qi "cloud-developed\|container-optimized" /etc/os-release; then
    IS_COS=1
fi

# 2. Ensure docker compose is installed
if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
    if [ "$IS_COS" -eq 1 ]; then
        %s
    else
        if command -v apt-get >/dev/null 2>&1; then
            sudo apt-get update -qq && sudo apt-get install -y -qq docker-compose-plugin || true
        elif command -v yum >/dev/null 2>&1; then
            sudo yum install -y docker-compose-plugin || true
        fi
    fi
fi

# 3. Ensure oops CLI is installed
if ! command -v oops >/dev/null 2>&1; then
    if [ "$IS_COS" -eq 1 ]; then
        sudo mkdir -p /var/lib/google/bin
        curl -fsSL https://raw.githubusercontent.com/noyzilla/jarn/main/install.sh | sh || true
        if [ -f /tmp/oops ]; then
            sudo mv /tmp/oops /var/lib/google/bin/oops
            sudo chmod 755 /var/lib/google/bin/oops
        fi
    else
        curl -fsSL https://raw.githubusercontent.com/noyzilla/jarn/main/install.sh | sh || true
    fi
fi

# 4. Initialize server bare repo
if command -v oops >/dev/null 2>&1; then summit_cmd="oops"; elif [ -f /var/lib/google/bin/oops ]; then summit_cmd="/var/lib/google/bin/oops"; else summit_cmd=""; fi

if [ -n "$summit_cmd" ]; then
    $summit_cmd box init-bare "$BARE_PATH" "$BOX_PATH"
else
    mkdir -p "$BARE_PATH" "$BOX_PATH"
    git init --bare "$BARE_PATH"
fi
`, barePath, boxPath, cosScript)
}

// ExecuteRemoteSSH executes a bash script string on sshTarget using the default SSH Deploy Key.
func ExecuteRemoteSSH(sshTarget, script string) (string, error) {
	keyPath, err := key.DefaultKeyPath()
	if err != nil {
		return "", fmt.Errorf("failed locating SSH key: %w", err)
	}

	cleanTarget := strings.TrimPrefix(sshTarget, "ssh://")
	if idx := strings.Index(cleanTarget, "/"); idx != -1 {
		cleanTarget = cleanTarget[:idx]
	}

	sshArgs := []string{
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=accept-new",
		cleanTarget,
		"bash -s",
	}

	cmd := exec.Command("ssh", sshArgs...)
	cmd.Stdin = bytes.NewBufferString(script)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("SSH execution to %s failed: %w\nStderr: %s", sshTarget, err, stderr.String())
	}

	return stdout.String(), nil
}
