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

# 3. Determine Target Binary Installation Path
TARGET_BIN_DIR=""
USE_SUDO=false

if [[ -f /etc/os-release ]] && (grep -qi "Container-Optimized OS" /etc/os-release 2>/dev/null || grep -qi "^ID=.*cos" /etc/os-release 2>/dev/null); then
    TARGET_BIN_DIR="/var/lib/google/bin"
    if [[ ! -d "$TARGET_BIN_DIR" ]]; then
        if [[ -w "/var/lib/google" ]]; then
            mkdir -p "$TARGET_BIN_DIR"
        elif command -v sudo >/dev/null 2>&1; then
            sudo mkdir -p "$TARGET_BIN_DIR"
        fi
    fi
fi

if [[ -z "$TARGET_BIN_DIR" ]]; then
    if [[ -d "/usr/local/bin" && -w "/usr/local/bin" ]]; then
        TARGET_BIN_DIR="/usr/local/bin"
    elif [[ -w "/usr/local" ]]; then
        mkdir -p "/usr/local/bin"
        TARGET_BIN_DIR="/usr/local/bin"
    elif command -v sudo >/dev/null 2>&1; then
        TARGET_BIN_DIR="/usr/local/bin"
        USE_SUDO=true
    else
        TARGET_BIN_DIR="$HOME/.local/bin"
        mkdir -p "$TARGET_BIN_DIR"
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
    sudo cp "$TMP_FILE" "$TARGET_BIN_DIR/oops"
    sudo chmod +x "$TARGET_BIN_DIR/oops"
else
    mkdir -p "$TARGET_BIN_DIR"
    cp "$TMP_FILE" "$TARGET_BIN_DIR/oops"
    chmod +x "$TARGET_BIN_DIR/oops"
fi

echo -e "${GREEN}✓ Successfully installed Oops CLI binary to ${TARGET_BIN_DIR}/oops${NC}"

# 5. Configure Shell Auto-completion
SHELL_CONFIGURED=false

# Configure Zsh
if [[ -f "$HOME/.zshrc" ]]; then
    if ! grep -q "oops completion" "$HOME/.zshrc" 2>/dev/null; then
        echo -e "\n# Oops CLI autocompletion" >> "$HOME/.zshrc"
        echo -e "if command -v oops >/dev/null 2>&1; then\n  source <(oops completion zsh)\nfi" >> "$HOME/.zshrc"
        SHELL_CONFIGURED=true
        echo -e "${GREEN}✓ Configured shell completion in ~/.zshrc${NC}"
    fi
fi

# Configure Bash
if [[ -f "$HOME/.bashrc" ]]; then
    if ! grep -q "oops completion" "$HOME/.bashrc" 2>/dev/null; then
        echo -e "\n# Oops CLI autocompletion" >> "$HOME/.bashrc"
        echo -e "if command -v oops >/dev/null 2>&1; then\n  source <(oops completion bash)\nfi" >> "$HOME/.bashrc"
        SHELL_CONFIGURED=true
        echo -e "${GREEN}✓ Configured shell completion in ~/.bashrc${NC}"
    fi
fi

# 6. Verify PATH
if [[ ":$PATH:" != *":$TARGET_BIN_DIR:"* ]]; then
    echo -e "${YELLOW}Warning: ${TARGET_BIN_DIR} is not in your PATH.${NC}"
    echo -e "Please add the following line to your shell profile (~/.zshrc or ~/.bashrc):"
    echo -e "  ${BOLD}export PATH=\"${TARGET_BIN_DIR}:\$PATH\"${NC}"
fi

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
