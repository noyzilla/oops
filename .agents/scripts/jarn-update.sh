#!/bin/sh
# Update all Jarn standards and skills from GitHub upstream repository.
# Pure basic shell (POSIX sh) with zero external runtime dependencies.
# Usage:
#   Public / HTTP (Default):
#     ./.agents/scripts/jarn-update.sh
#     curl -fsSL https://raw.githubusercontent.com/noyzilla/jarn/main/.agents/scripts/jarn-update.sh | sh
#   Private / GitHub CLI (gh):
#     ./.agents/scripts/jarn-update.sh gh
#     gh api repos/noyzilla/jarn/contents/.agents/scripts/jarn-update.sh -H "Accept: application/vnd.github.raw+json" | sh -s -- gh

set -eu

REPO="${JARN_REPO:-noyzilla/jarn}"
VERSION="${JARN_VERSION:-latest}"
AGENTS_DIR=".agents"
VERSION_FILE="${AGENTS_DIR}/.jarn-version"

RESUME_TMP_DIR=""
FORCE_UPDATE=0
METHOD="${JARN_METHOD:-curl}"

while [ $# -gt 0 ]; do
  case "$1" in
    --internal-resume)
      RESUME_TMP_DIR="$2"
      shift 2
      ;;
    --force|-f)
      FORCE_UPDATE=1
      shift
      ;;
    gh|--gh)
      METHOD="gh"
      shift
      ;;
    *)
      # Ignore other arguments if piped from curl/gh
      shift
      ;;
  esac
done

if [ -z "${RESUME_TMP_DIR}" ]; then
  if [ "${METHOD}" = "curl" ]; then
    HTTP_STATUS=$(curl -o /dev/null -s -w "%{http_code}" "https://api.github.com/repos/${REPO}" || echo "000")
    if [ "${HTTP_STATUS}" != "200" ] && command -v gh >/dev/null 2>&1; then
      METHOD="gh"
    fi
  fi

  if [ "${VERSION}" = "latest" ]; then
    if [ "${METHOD}" = "gh" ]; then
      VERSION=$(gh api "repos/${REPO}/releases/latest" -q '.tag_name' 2>/dev/null || echo "")
    else
      VERSION=$(curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | head -n 1 | sed -E 's/.*"([^"]+)".*/\1/' || true)
    fi
    
    if [ -z "${VERSION}" ] || [ "${VERSION}" = "null" ]; then
      echo "Error: Could not resolve latest tag for ${REPO}." >&2
      echo "  Ensure that the repository has at least one Git tag (release)." >&2
      exit 1
    else
      echo "Resolved latest tag: ${VERSION}"
    fi
  fi

  # Version Check
  if [ "${FORCE_UPDATE}" -eq 0 ] && [ -f "${VERSION_FILE}" ]; then
    CURRENT_VERSION=$(cat "${VERSION_FILE}")
    if [ "${CURRENT_VERSION}" = "${VERSION}" ]; then
      echo "Jarn is already up-to-date (version ${VERSION})."
      echo "To force an update, run: ./.agents/scripts/jarn-update.sh --force"
      exit 0
    fi
  fi

  TARBALL_URL="https://github.com/${REPO}/tarball/${VERSION}"

  echo "Updating Jarn standards and skills from ${REPO}@${VERSION}..."

  TMP_DIR=$(mktemp -d)
  trap 'rm -rf "${TMP_DIR}"' EXIT INT TERM

  if [ "${METHOD}" = "gh" ]; then
    if ! command -v gh >/dev/null 2>&1; then
      echo "Error: GitHub CLI 'gh' is required for gh mode but is not installed or not in PATH." >&2
      exit 1
    fi
    echo "Fetching updates via GitHub CLI (gh api)..."
    gh api "repos/${REPO}/tarball/${VERSION}" | tar -xz -C "${TMP_DIR}" --strip-components=1
  else
    echo "Fetching updates via HTTP (curl)..."
    curl -fsSL "${TARBALL_URL}" | tar -xz -C "${TMP_DIR}" --strip-components=1 || true
  fi

  if [ ! -f "${TMP_DIR}/.agents/rules/jarn-standards.md" ]; then
    echo "Error: Failed to download Jarn updates from '${REPO}'." >&2
    echo "  If '${REPO}' is a private repository, run with GitHub CLI (gh) mode:" >&2
    echo "    ./.agents/scripts/jarn-update.sh gh" >&2
    exit 1
  fi

  # Self-update check (only if executing locally)
  if [ -f "$0" ] && [ -w "$0" ]; then
    if ! cmp -s "$0" "${TMP_DIR}/.agents/scripts/jarn-update.sh"; then
      echo "Updater script has new logic. Self-updating and restarting..."
      mkdir -p "${AGENTS_DIR}/scripts"
      cp "${TMP_DIR}/.agents/scripts/jarn-update.sh" "$0"
      chmod +x "$0"
      exec "$0" --internal-resume "${TMP_DIR}" "$@"
    fi
  fi
else
  # Resuming from a self-update execution
  TMP_DIR="${RESUME_TMP_DIR}"
  trap 'rm -rf "${TMP_DIR}"' EXIT INT TERM
fi

# Synchronize Jarn core rules
mkdir -p "${AGENTS_DIR}/rules"
for rule_file in "${TMP_DIR}/.agents/rules/jarn-"*; do
  if [ -f "${rule_file}" ]; then
    cp "${rule_file}" "${AGENTS_DIR}/rules/"
  fi
done
echo "  - Jarn rules updated in ${AGENTS_DIR}/rules/jarn-*.md"

# Synchronize Jarn skills (matching jarn-*)
for skill_dir in "${TMP_DIR}/${AGENTS_DIR}/skills/jarn-"*; do
  if [ -d "${skill_dir}" ]; then
    skill_name="${skill_dir##*/}"
    mkdir -p "${AGENTS_DIR}/skills/${skill_name}"
    cp -R "${skill_dir}/." "${AGENTS_DIR}/skills/${skill_name}/"
    echo "  - Jarn skill updated: ${AGENTS_DIR}/skills/${skill_name}"
  fi
done

# Synchronize this updater script itself
if [ -f "${TMP_DIR}/.agents/scripts/jarn-update.sh" ]; then
  mkdir -p "${AGENTS_DIR}/scripts"
  cp "${TMP_DIR}/.agents/scripts/jarn-update.sh" "${AGENTS_DIR}/scripts/jarn-update.sh"
  chmod +x "${AGENTS_DIR}/scripts/jarn-update.sh" 2>/dev/null || true
  echo "  - Jarn updater updated in ${AGENTS_DIR}/scripts/jarn-update.sh"
fi

# Synchronize Blueprint Templates via pending-merge
SUFFIX="pending-merge"
echo "  - Synchronizing templates and docs (using .${SUFFIX} for conflicts)..."

# Map templates/ to root in TMP_DIR
if [ -d "${TMP_DIR}/templates" ]; then
  cp -r "${TMP_DIR}/templates/"* "${TMP_DIR}/" 2>/dev/null || true
  rm -rf "${TMP_DIR}/templates"
fi

CREATED_RECORD="${TMP_DIR}/created.list"
PENDING_RECORD="${TMP_DIR}/pending.list"
touch "${CREATED_RECORD}" "${PENDING_RECORD}"

(
  cd "${TMP_DIR}"
  find . -type f
) | while IFS= read -r file_path; do
  rel_path="${file_path#./}"

  case "${rel_path}" in
    docs/README.md) ;;
    docs/specs/0000-template.md) ;;
    docs/decisions/0000-template.md) ;;
    AGENTS.md|ARCHITECTURE.md|CHANGELOG.md|CONTEXT.md|CONTRIBUTING.md|DESIGN.md|README.md|REVIEW.md|TASK.md) ;;
    *) continue ;;
  esac
  
  dest_file="./${rel_path}"
  dest_dir=$(dirname "${dest_file}")
  mkdir -p "${dest_dir}"

  if [ ! -f "${dest_file}" ]; then
    cp "${TMP_DIR}/${rel_path}" "${dest_file}"
    echo "    + Created ${rel_path}"
  elif ! cmp -s "${TMP_DIR}/${rel_path}" "${dest_file}"; then
    pending_file="${dest_file}.${SUFFIX}"
    cp "${TMP_DIR}/${rel_path}" "${pending_file}"
    echo "    * Staged ${rel_path}.${SUFFIX} for AI merge"
  fi
done

# Record new version
echo "${VERSION}" > "${VERSION_FILE}"

# Display Update Notes (Migration instructions for AI/User)
if [ -f "${TMP_DIR}/UPDATE_NOTES.md" ]; then
  echo ""
  echo "================================================================="
  echo "  JARN UPDATE NOTES & AI DIRECTIVE"
  echo "================================================================="
  cat "${TMP_DIR}/UPDATE_NOTES.md"
  echo "================================================================="
  echo ""
fi

echo "Update complete. All Jarn components are up to date."
