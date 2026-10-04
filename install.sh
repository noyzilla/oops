#!/usr/bin/env bash
set -e

# ==============================================================================
# Oops & Oopsbox One-Line Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/noyzilla/oops/main/install.sh | bash
# ==============================================================================

# ANSI Color Codes
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BOLD='\033[1m'
NC='\033[0m' # No Color

echo -e "${BOLD}${BLUE}==============================================================================${NC}"
echo -e "${BOLD}${BLUE}  Oops & Oopsbox Installer${NC}"
echo -e "${BOLD}${BLUE}==============================================================================${NC}"

# 1. Determine Target Installation Directory
TARGET_DIR="${1:-${OOPSBOX_DIR:-$HOME/oopsbox}}"
# Expand tilde if present
TARGET_DIR="${TARGET_DIR/#\~/$HOME}"

echo -e "==> Target Directory: ${BOLD}${TARGET_DIR}${NC}"

# 2. Check Prerequisite Tools
command -v curl >/dev/null 2>&1 || { echo -e "${RED}Error: curl is required but not installed.${NC}" >&2; exit 1; }
command -v tar >/dev/null 2>&1 || { echo -e "${RED}Error: tar is required but not installed.${NC}" >&2; exit 1; }

# 3. Download and Extract Oopsbox Blueprint
TMP_DIR=$(mktemp -d)
cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

echo -e "==> Downloading Oopsbox blueprint from GitHub..."
curl -fsSL https://github.com/noyzilla/oops/archive/refs/heads/main.tar.gz -o "$TMP_DIR/oops.tar.gz"

echo -e "==> Extracting blueprint..."
tar -xzf "$TMP_DIR/oops.tar.gz" -C "$TMP_DIR"

mkdir -p "$TARGET_DIR"

# Copy oopsbox contents without clobbering existing custom .env
if [ -d "$TMP_DIR/oops-main/oopsbox" ]; then
    SRC_DIR="$TMP_DIR/oops-main/oopsbox"
else
    echo -e "${RED}Error: Failed to locate oopsbox directory in downloaded archive.${NC}" >&2
    exit 1
fi

# Copy all files & directories
cp -R "$SRC_DIR/"* "$TARGET_DIR/" 2>/dev/null || true
cp -R "$SRC_DIR/."* "$TARGET_DIR/" 2>/dev/null || true

# 4. Make developer tool scripts executable
chmod +x "$TARGET_DIR/bin/"* 2>/dev/null || true

# 5. Run DevOops Initialization
if [ -f "$TARGET_DIR/bin/devoops" ]; then
    echo -e "\n==> Initializing workstation configuration..."
    (cd "$TARGET_DIR" && ./bin/devoops install)
fi

# 6. Install Global Oops Wrapper Script if writable
WRAPPER_INSTALLED=false
WRAPPER_PATH=""

for CANDIDATE_DIR in "/usr/local/bin" "$HOME/.local/bin"; do
    if [ -d "$CANDIDATE_DIR" ] && [ -w "$CANDIDATE_DIR" ]; then
        WRAPPER_PATH="$CANDIDATE_DIR/oops"
        cat > "$WRAPPER_PATH" <<EOF
#!/bin/sh
# Oops Docker CLI Wrapper
exec docker run --rm -i \\
  -v /var/run/docker.sock:/var/run/docker.sock \\
  -v "\${OOPSBOX_DIR:-\$HOME/oopsbox}":/workspace \\
  -w /workspace \\
  -e OOPS_DIR=/workspace \\
  -e OOPSBOX_DIR=/workspace \\
  ghcr.io/noyzilla/oops:latest "\$@"
EOF
        chmod +x "$WRAPPER_PATH"
        WRAPPER_INSTALLED=true
        echo -e "${GREEN}✓ Installed global oops wrapper at ${WRAPPER_PATH}${NC}"
        break
    fi
done

# 7. Print Completion & Quickstart Instructions
echo -e "\n${BOLD}${GREEN}==============================================================================${NC}"
echo -e "${BOLD}${GREEN}  Oopsbox Installation Complete!${NC}"
echo -e "${BOLD}${GREEN}==============================================================================${NC}"
echo -e "Oopsbox Location: ${BOLD}${TARGET_DIR}${NC}\n"

if [ "$WRAPPER_INSTALLED" = true ]; then
    echo -e "You can run ${BOLD}oops${NC} from any directory:"
    echo -e "  ${BOLD}export OOPSBOX_DIR=\"${TARGET_DIR}\"${NC}"
    echo -e "  ${BOLD}oops up${NC}\n"
else
    echo -e "Add this alias to your shell profile (~/.bashrc or ~/.zshrc):"
    echo -e "  ${BOLD}export OOPSBOX_DIR=\"${TARGET_DIR}\"${NC}"
    echo -e "  ${BOLD}alias oops='docker run --rm -i -v /var/run/docker.sock:/var/run/docker.sock -v \"\${OOPSBOX_DIR:-\$HOME/oopsbox}\":/workspace -w /workspace -e OOPS_DIR=/workspace -e OOPSBOX_DIR=/workspace ghcr.io/noyzilla/oops:latest'${NC}\n"
fi

echo -e "To start your environment:"
echo -e "  1. ${BOLD}devoops start${NC}"
echo -e "  2. ${BOLD}devoops install-cert${NC} (to enable local trusted HTTPS for *.web.oops)"
echo -e "${BOLD}${GREEN}==============================================================================${NC}\n"
