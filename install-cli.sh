#!/usr/bin/env bash
set -e

# ==============================================================================
# Oops CLI Standalone Installer
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/noyzilla/oops/main/install-cli.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/noyzilla/oops/main/install-cli.sh | bash -s -- /usr/local/bin/oops
# ==============================================================================

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BOLD='\033[1m'
NC='\033[0m' # No Color

echo -e "${BOLD}${BLUE}==============================================================================${NC}"
echo -e "${BOLD}${BLUE}  Oops CLI Standalone Installer${NC}"
echo -e "${BOLD}${BLUE}==============================================================================${NC}"

# 1. Check prerequisite tools
command -v curl >/dev/null 2>&1 || { echo -e "${RED}Error: curl is required but not installed.${NC}" >&2; exit 1; }

# 2. Detect OS
OS_RAW="$(uname -s)"
case "$OS_RAW" in
    Darwin*) OS="darwin" ;;
    Linux*)  OS="linux" ;;
    *)
        echo -e "${RED}Error: Unsupported operating system: ${OS_RAW}${NC}" >&2
        exit 1
        ;;
esac

# 3. Detect Architecture
ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
    x86_64|amd64)        ARCH="amd64" ;;
    arm64|aarch64|armv8*) ARCH="arm64" ;;
    *)
        echo -e "${RED}Error: Unsupported CPU architecture: ${ARCH_RAW}${NC}" >&2
        exit 1
        ;;
esac

echo -e "==> Detected Platform: ${BOLD}${OS}/${ARCH}${NC}"

# 4. Determine Installation Target Path
TARGET_INPUT="${1:-${INSTALL_PATH:-}}"

# Parse flags if passed as --install-path <path> or -o <path>
while [[ $# -gt 0 ]]; do
    case "$1" in
        --install-path|--path|-o)
            TARGET_INPUT="$2"
            shift 2
            ;;
        *)
            TARGET_INPUT="$1"
            shift
            ;;
    esac
done

if [ -n "$TARGET_INPUT" ]; then
    # Expand tilde if present
    TARGET_INPUT="${TARGET_INPUT/#\~/$HOME}"
    if [ -d "$TARGET_INPUT" ]; then
        DEST_FILE="${TARGET_INPUT%/}/oops"
    else
        DEST_FILE="$TARGET_INPUT"
    fi
else
    # Default search order: /usr/local/bin/oops -> $HOME/.local/bin/oops
    if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
        DEST_FILE="/usr/local/bin/oops"
    elif [ -d "$HOME/.local/bin" ]; then
        DEST_FILE="$HOME/.local/bin/oops"
    elif [ -d "/usr/local/bin" ]; then
        DEST_FILE="/usr/local/bin/oops"
    else
        mkdir -p "$HOME/.local/bin"
        DEST_FILE="$HOME/.local/bin/oops"
    fi
fi

DEST_DIR="$(dirname "$DEST_FILE")"
mkdir -p "$DEST_DIR" 2>/dev/null || true

echo -e "==> Installation Destination: ${BOLD}${DEST_FILE}${NC}"

# 5. Download Binary from GitHub Releases
VERSION="${OOPS_VERSION:-latest}"
if [ "$VERSION" = "latest" ]; then
    DOWNLOAD_URL="https://github.com/noyzilla/oops/releases/latest/download/oops-${OS}-${ARCH}"
else
    DOWNLOAD_URL="https://github.com/noyzilla/oops/releases/download/${VERSION}/oops-${OS}-${ARCH}"
fi

TMP_FILE="$(mktemp)"
cleanup() {
    rm -f "$TMP_FILE"
}
trap cleanup EXIT

echo -e "==> Downloading Oops CLI from ${DOWNLOAD_URL}..."
if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_FILE" || [ ! -s "$TMP_FILE" ]; then
    echo -e "${RED}Error: Failed to download Oops CLI binary for ${OS}-${ARCH}.${NC}" >&2
    exit 1
fi

chmod +x "$TMP_FILE"

# 6. Install Binary
if [ -w "$DEST_DIR" ]; then
    mv -f "$TMP_FILE" "$DEST_FILE"
else
    echo -e "${YELLOW}==> Elevated permissions required to write to ${DEST_DIR}...${NC}"
    sudo mv -f "$TMP_FILE" "$DEST_FILE"
    sudo chmod +x "$DEST_FILE"
fi

# 7. Verification & Summary
echo -e "\n${BOLD}${GREEN}==============================================================================${NC}"
echo -e "${BOLD}${GREEN}  Oops CLI Installation Complete!${NC}"
echo -e "${BOLD}${GREEN}==============================================================================${NC}"
echo -e "Binary Location: ${BOLD}${DEST_FILE}${NC}"

if command -v oops >/dev/null 2>&1; then
    echo -e "\n${GREEN}✓ 'oops' is ready to use in your current PATH.${NC}"
else
    echo -e "\n${YELLOW}Notice: ${DEST_DIR} is not in your current PATH.${NC}"
    echo -e "Add this to your shell profile (~/.bashrc or ~/.zshrc):"
    echo -e "  ${BOLD}export PATH=\"${DEST_DIR}:\$PATH\"${NC}"
fi

echo -e "\nQuick test command:"
echo -e "  ${BOLD}oops --help${NC}"
echo -e "${BOLD}${GREEN}==============================================================================${NC}\n"
