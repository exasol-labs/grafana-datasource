#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

ARCHIVE_PATH="${1:-exasol-exasol-datasource.zip}"

# The validator appears to honor extracted file modes, so keep umask permissive
# enough for executable bits to survive on all platforms.
umask 022

npx @grafana/plugin-validator@latest "${ARCHIVE_PATH}"
