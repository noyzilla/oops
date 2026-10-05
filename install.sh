#!/usr/bin/env bash
set -e

# ==============================================================================
# Oops CLI One-Line Standalone Installer
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/noyzilla/oops/main/install.sh | bash
# ==============================================================================

# ANSI Color Codes
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BOLD='\033[1m'
NC='\033[0m' # No Color

echo -e "${BOLD}${BLUE}==============================================================================${NC}"
echo -e "${BOLD}${BLUE}  Oops CLI Standalone Installer${NC}"
echo -e "${BOLD}${BLUE}==============================================================================${NC}"

# 1. Detect Operating System & Architecture
OS_RAW="$(uname -s)"
case "$OS_RAW" in
    Darwin*) OS="darwin" ;;
    Linux*)  OS="linux" ;;
    *)       echo -e "${RED}Error: Unsupported operating system: ${OS_RAW}${NC}" >&2; exit 1 ;;
esac

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
    x86_64|amd64)         ARCH="amd64" ;;
    arm64|aarch64|armv8*) ARCH="arm64" ;;
    *)                    echo -e "${RED}Error: Unsupported architecture: ${ARCH_RAW}${NC}" >&2; exit 1 ;;
esac

echo -e "==> Detected Platform: ${BOLD}${OS}-${ARCH}${NC}"

# 2. Check Prerequisite Tools
command -v curl >/dev/null 2>&1 || { echo -e "${RED}Error: curl is required but not installed.${NC}" >&2; exit 1; }

# 3. Determine Target Binary Installation Path & Sudo Requirement
TARGET_BIN_DIR=""
USE_SUDO=false

if [[ -f /etc/os-release ]] && (grep -qi "Container-Optimized OS" /etc/os-release 2>/dev/null || grep -qi "^ID=.*cos" /etc/os-release 2>/dev/null); then
    TARGET_BIN_DIR="/var/lib/google/bin"
fi

if [[ -z "$TARGET_BIN_DIR" ]]; then
    if [[ -d "/usr/local/bin" && -w "/usr/local/bin" ]]; then
        TARGET_BIN_DIR="/usr/local/bin"
    elif [[ -w "/usr/local" ]]; then
        TARGET_BIN_DIR="/usr/local/bin"
    elif command -v sudo >/dev/null 2>&1; then
        TARGET_BIN_DIR="/usr/local/bin"
    else
        TARGET_BIN_DIR="$HOME/.local/bin"
    fi
fi

# Detect whether sudo permissions are required for the target directory
if [[ ! -w "$TARGET_BIN_DIR" && ! -w "$(dirname "$TARGET_BIN_DIR")" ]]; then
    if command -v sudo >/dev/null 2>&1 && [[ "$EUID" -ne 0 ]]; then
        USE_SUDO=true
    fi
fi

echo -e "==> Installation Destination: ${BOLD}${TARGET_BIN_DIR}/oops${NC}"

# 4. Download Oops Binary
TMP_FILE="$(mktemp)"
cleanup() {
    rm -f "$TMP_FILE"
}
trap cleanup EXIT INT TERM

BINARY_URL="https://github.com/noyzilla/oops/releases/latest/download/oops-${OS}-${ARCH}"
echo -e "==> Downloading latest release binary from GitHub..."
if ! curl -fsSL "$BINARY_URL" -o "$TMP_FILE" || [[ ! -s "$TMP_FILE" ]]; then
    echo -e "${RED}Error: Failed to download binary from ${BINARY_URL}.${NC}" >&2
    echo -e "${YELLOW}Please verify internet access or compile from source via: go install github.com/noyzilla/oops@latest${NC}" >&2
    exit 1
fi

chmod +x "$TMP_FILE"

if [[ "$USE_SUDO" == true ]]; then
    echo -e "==> Requesting sudo permissions to install to ${TARGET_BIN_DIR}..."
    sudo mkdir -p "$TARGET_BIN_DIR"
    sudo chmod 755 "$TARGET_BIN_DIR"
    sudo cp "$TMP_FILE" "$TARGET_BIN_DIR/oops"
    sudo chmod 755 "$TARGET_BIN_DIR/oops"
else
    mkdir -p "$TARGET_BIN_DIR"
    chmod 755 "$TARGET_BIN_DIR"
    cp "$TMP_FILE" "$TARGET_BIN_DIR/oops"
    chmod 755 "$TARGET_BIN_DIR/oops"
fi

echo -e "${GREEN}✓ Successfully installed Oops CLI binary to ${TARGET_BIN_DIR}/oops${NC}"

# 5. Configure Shell Auto-completion
SHELL_CONFIGURED=false

# 5. Configure Shell PATH & Auto-completion
SHELL_PROFILES=()
[[ -f "$HOME/.bashrc" ]] && SHELL_PROFILES+=("$HOME/.bashrc")
[[ -f "$HOME/.zshrc" ]] && SHELL_PROFILES+=("$HOME/.zshrc")
[[ -f "$HOME/.profile" ]] && SHELL_PROFILES+=("$HOME/.profile")

if [[ ${#SHELL_PROFILES[@]} -eq 0 ]]; then
    SHELL_PROFILES+=("$HOME/.bashrc")
fi

for PROFILE in "${SHELL_PROFILES[@]}"; do
    # 5.1 Ensure TARGET_BIN_DIR is in PATH
    if ! grep -q "$TARGET_BIN_DIR" "$PROFILE" 2>/dev/null; then
        echo -e "\n# Oops CLI PATH" >> "$PROFILE"
        echo -e "export PATH=\"${TARGET_BIN_DIR}:\$PATH\"" >> "$PROFILE"
        echo -e "${GREEN}✓ Added ${TARGET_BIN_DIR} to PATH in ${PROFILE}${NC}"
    fi

    # 5.2 Configure Shell Autocompletion
    if ! grep -q "oops completion" "$PROFILE" 2>/dev/null; then
        SHELL_TYPE="bash"
        [[ "$PROFILE" == *".zshrc"* ]] && SHELL_TYPE="zsh"
        echo -e "\n# Oops CLI autocompletion" >> "$PROFILE"
        echo -e "if command -v oops >/dev/null 2>&1; then\n  source <(oops completion ${SHELL_TYPE})\nfi" >> "$PROFILE"
        echo -e "${GREEN}✓ Configured shell completion in ${PROFILE}${NC}"
    fi
done

# 7. Print Completion & Quickstart Instructions
echo -e "\n${BOLD}${GREEN}==============================================================================${NC}"
echo -e "${BOLD}${GREEN}  Oops CLI Installation Complete!${NC}"
echo -e "${BOLD}${GREEN}==============================================================================${NC}"
echo -e "Version: ${BOLD}$("$TARGET_BIN_DIR/oops" --help 2>&1 | head -n 1)${NC}\n"

echo -e "Quickstart:"
echo -e "  1. Create a new Oopsbox workspace:"
echo -e "     ${BOLD}oops box create ~/oopsbox${NC}"
echo -e "  2. Boot the environment:"
echo -e "     ${BOLD}cd ~/oopsbox && oops box start${NC}"
echo -e "  3. Verify status:"
echo -e "     ${BOLD}oops status${NC}\n"

echo -e "Tip: Tab completion is active! Type ${BOLD}oops box <tab>${NC} to view available subcommands."
echo -e "${BOLD}${GREEN}==============================================================================${NC}\n"
