package remote

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
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
	hookScript := GeneratePostReceiveHook(boxPath)

	return fmt.Sprintf(`#!/bin/bash
set -euo pipefail

BARE_PATH="%s"
BOX_PATH="%s"

case "$BARE_PATH" in
  \~/*) BARE_PATH="$HOME/${BARE_PATH#\~/}" ;;
  \~)   BARE_PATH="$HOME" ;;
esac

case "$BOX_PATH" in
  \~/*) BOX_PATH="$HOME/${BOX_PATH#\~/}" ;;
  \~)   BOX_PATH="$HOME" ;;
esac

# 1. Detect COS vs standard Linux & setup SUDO helper
IS_COS=0
if [ -f /etc/os-release ] && grep -qi "cloud-developed\|container-optimized" /etc/os-release; then
    IS_COS=1
fi

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
    if command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
        SUDO="sudo -n"
    fi
fi

# 2. Ensure docker compose is installed
if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
    if [ "$IS_COS" -eq 1 ]; then
        %s
    else
        if command -v apt-get >/dev/null 2>&1; then
            $SUDO apt-get update -qq && $SUDO apt-get install -y -qq docker-compose-plugin || true
        elif command -v yum >/dev/null 2>&1; then
            $SUDO yum install -y docker-compose-plugin || true
        fi
    fi
fi

# 3. Ensure oops CLI is installed
if ! command -v oops >/dev/null 2>&1 && [ ! -f /var/lib/google/bin/oops ]; then
    ARCH="$(uname -m)"
    case "${ARCH}" in
        x86_64)        OOPS_ARCH="amd64" ;;
        aarch64|arm64) OOPS_ARCH="arm64" ;;
        *)             OOPS_ARCH="amd64" ;;
    esac
    TARGET_BIN="/usr/local/bin/oops"
    if [ "$IS_COS" -eq 1 ]; then
        TARGET_BIN="/var/lib/google/bin/oops"
    fi
    if [ -n "$SUDO" ] || [ "$(id -u)" -eq 0 ]; then
        $SUDO mkdir -p "$(dirname "$TARGET_BIN")" 2>/dev/null || true
        $SUDO curl -fsSL "https://github.com/noyzilla/oops/releases/latest/download/oops-linux-${OOPS_ARCH}" -o "$TARGET_BIN" 2>/dev/null || true
        $SUDO chmod 755 "$TARGET_BIN" 2>/dev/null || true
    fi
fi

if [ "$IS_COS" -eq 1 ]; then
    for profile in "$HOME/.bashrc" "$HOME/.bash_profile" "$HOME/.profile"; do
        if [ -f "$profile" ] || [ "$profile" = "$HOME/.bashrc" ]; then
            if ! grep -q "/var/lib/google/bin" "$profile" 2>/dev/null; then
                if [ -s "$profile" ]; then
                    sed -i '1s|^|export PATH="/var/lib/google/bin:$PATH"\n|' "$profile" 2>/dev/null || echo 'export PATH="/var/lib/google/bin:$PATH"' >> "$profile"
                else
                    echo 'export PATH="/var/lib/google/bin:$PATH"' > "$profile"
                fi
            fi
        fi
    done
fi

# 4. Initialize server bare repo
mkdir -p "$BARE_PATH/hooks" "$BOX_PATH"
git init --bare "$BARE_PATH"
git -C "$BARE_PATH" config receive.advertisePushOptions true
cat << 'HOOK_EOF' > "$BARE_PATH/hooks/post-receive"
%s
HOOK_EOF
chmod 755 "$BARE_PATH/hooks/post-receive"
`, barePath, boxPath, cosScript, hookScript)
}


// ExecuteRemoteSSH executes a bash script string on sshTarget using system SSH.
func ExecuteRemoteSSH(sshTarget, script string) (string, error) {
	cleanTarget := strings.TrimPrefix(sshTarget, "ssh://")
	if idx := strings.Index(cleanTarget, "/"); idx != -1 {
		cleanTarget = cleanTarget[:idx]
	}

	sshArgs := []string{
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
