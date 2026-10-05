#!/usr/bin/env bash
set -e

# ==============================================================================
# Oops & Oopsbox One-Line Installer
# Usage:
#   # Install to current directory:
#   curl -fsSL https://raw.githubusercontent.com/noyzilla/oops/main/install.sh | bash
#   # Install to specified project directory:
#   curl -fsSL https://raw.githubusercontent.com/noyzilla/oops/main/install.sh | bash -s -- <project-directory>
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
TARGET_INPUT=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --dir|-d|--path|-o)
            TARGET_INPUT="$2"
            shift 2
            ;;
        -*)
            shift
            ;;
        *)
            if [[ -z "$TARGET_INPUT" ]]; then
                TARGET_INPUT="$1"
            fi
            shift
            ;;
    esac
done

if [[ -n "$TARGET_INPUT" ]]; then
    TARGET_DIR="$TARGET_INPUT"
else
    TARGET_DIR="$(pwd)"
fi

# Expand tilde if present
TARGET_DIR="${TARGET_DIR/#\~/$HOME}"

# Ensure target directory exists and resolve absolute path
mkdir -p "$TARGET_DIR"
TARGET_DIR="$(cd "$TARGET_DIR" && pwd)"

echo -e "==> Target Directory: ${BOLD}${TARGET_DIR}${NC}"

# 2. Check Prerequisite Tools
command -v curl >/dev/null 2>&1 || { echo -e "${RED}Error: curl is required but not installed.${NC}" >&2; exit 1; }
command -v tar >/dev/null 2>&1 || { echo -e "${RED}Error: tar is required but not installed.${NC}" >&2; exit 1; }

# 3. Download and Extract Oopsbox Blueprint
TMP_DIR="$(mktemp -d)"
cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

echo -e "==> Downloading Oopsbox blueprint from GitHub..."
if curl -fsSL https://github.com/noyzilla/oops/releases/latest/download/oopsbox.tar.gz -o "$TMP_DIR/oopsbox.tar.gz" 2>/dev/null && [[ -s "$TMP_DIR/oopsbox.tar.gz" ]]; then
    echo -e "==> Extracting release blueprint..."
    mkdir -p "$TMP_DIR/extracted"
    tar -xzf "$TMP_DIR/oopsbox.tar.gz" -C "$TMP_DIR/extracted"
    SRC_DIR="$TMP_DIR/extracted"
else
    echo -e "==> Downloading fallback blueprint from main branch..."
    curl -fsSL https://github.com/noyzilla/oops/archive/refs/heads/main.tar.gz -o "$TMP_DIR/oops.tar.gz"
    echo -e "==> Extracting blueprint..."
    tar -xzf "$TMP_DIR/oops.tar.gz" -C "$TMP_DIR"
    if [[ -d "$TMP_DIR/oops-main/oopsbox" ]]; then
        SRC_DIR="$TMP_DIR/oops-main/oopsbox"
    else
        echo -e "${RED}Error: Failed to locate oopsbox directory in downloaded archive.${NC}" >&2
        exit 1
    fi
fi

# Copy all blueprint files & directories into target directory
mkdir -p "$TARGET_DIR"
cp -R "$SRC_DIR/." "$TARGET_DIR/"

# 4. Download and Install Native Oops CLI Binary as Primary
CLI_INSTALLED=false
OS_RAW="$(uname -s)"
case "$OS_RAW" in
    Darwin*) OS="darwin" ;;
    Linux*)  OS="linux" ;;
    *)       OS="" ;;
esac

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
    x86_64|amd64)        ARCH="amd64" ;;
    arm64|aarch64|armv8*) ARCH="arm64" ;;
    *)                   ARCH="" ;;
esac

mkdir -p "$TARGET_DIR/bin"

if [[ -n "$OS" && -n "$ARCH" ]]; then
    echo -e "==> Downloading native Oops CLI binary for ${OS}-${ARCH}..."
    BINARY_URL="https://github.com/noyzilla/oops/releases/latest/download/oops-${OS}-${ARCH}"
    if curl -fsSL "$BINARY_URL" -o "$TARGET_DIR/bin/oops" 2>/dev/null && [[ -s "$TARGET_DIR/bin/oops" ]]; then
        chmod +x "$TARGET_DIR/bin/oops"
        CLI_INSTALLED=true
        echo -e "${GREEN}✓ Downloaded native Oops CLI to ${TARGET_DIR}/bin/oops${NC}"
    else
        echo -e "${YELLOW}Notice: Native release binary not found; will use wrapper fallback.${NC}"
    fi
fi

# Make developer tool scripts executable
chmod +x "$TARGET_DIR/bin/"* 2>/dev/null || true

# 5. Link Global Oops CLI (Native Binary Primary, Docker Wrapper Fallback)
GLOBAL_OOPS_INSTALLED=false
for CANDIDATE_DIR in "/usr/local/bin" "$HOME/.local/bin"; do
    if [[ -d "$CANDIDATE_DIR" && -w "$CANDIDATE_DIR" ]]; then
        if [[ "$CLI_INSTALLED" == true ]]; then
            ln -sf "$TARGET_DIR/bin/oops" "$CANDIDATE_DIR/oops"
            GLOBAL_OOPS_INSTALLED=true
            echo -e "${GREEN}✓ Linked native oops CLI binary at ${CANDIDATE_DIR}/oops${NC}"
            break
        else
            WRAPPER_PATH="$CANDIDATE_DIR/oops"
            cat > "$WRAPPER_PATH" <<'EOF'
#!/bin/sh
# Oops Docker CLI Wrapper Fallback
BOX_DIR="${OOPSBOX_DIR:-$HOME/oopsbox}"
exec docker run --rm -i \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "${BOX_DIR}":"${BOX_DIR}" \
  -w "${BOX_DIR}" \
  -e OOPSBOX_DIR="${BOX_DIR}" \
  ghcr.io/noyzilla/oops:latest "$@"
EOF
            chmod +x "$WRAPPER_PATH"
            GLOBAL_OOPS_INSTALLED=true
            echo -e "${YELLOW}✓ Installed global oops Docker wrapper fallback at ${WRAPPER_PATH}${NC}"
            break
        fi
    fi
done

# 6. Run Oopsbox Initialization
if [[ -f "$TARGET_DIR/bin/oopsbox" ]]; then
    echo -e "\n==> Initializing workstation configuration..."
    (cd "$TARGET_DIR" && ./bin/oopsbox install)
fi

# 7. Print Completion & Quickstart Instructions
echo -e "\n${BOLD}${GREEN}==============================================================================${NC}"
echo -e "${BOLD}${GREEN}  Oopsbox Installation Complete!${NC}"
echo -e "${BOLD}${GREEN}==============================================================================${NC}"
echo -e "Oopsbox Location: ${BOLD}${TARGET_DIR}${NC}\n"

if [[ "$GLOBAL_OOPS_INSTALLED" == true ]]; then
    echo -e "You can run ${BOLD}oops${NC} from any directory:"
    echo -e "  ${BOLD}export OOPSBOX_DIR=\"${TARGET_DIR}\"${NC}"
    echo -e "  ${BOLD}oops up${NC}\n"
else
    echo -e "Add this alias to your shell profile (~/.bashrc or ~/.zshrc):"
    echo -e "  ${BOLD}export OOPSBOX_DIR=\"${TARGET_DIR}\"${NC}"
    echo -e "  ${BOLD}alias oops='${TARGET_DIR}/bin/oops'${NC}\n"
fi

echo -e "To start your environment:"
echo -e "  1. ${BOLD}cd ${TARGET_DIR}${NC}"
echo -e "  2. ${BOLD}oopsbox start${NC}"
echo -e "  3. ${BOLD}oopsbox install-cert${NC} (to enable local trusted HTTPS for *.web.oops)"
echo -e "${BOLD}${GREEN}==============================================================================${NC}\n"
