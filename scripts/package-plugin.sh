#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

PLUGIN_ID="$(node -p "require('./src/plugin.json').id")"
PLUGIN_VERSION="$(node -p "require('./package.json').version")"
PACKAGE_ROOT="build/${PLUGIN_ID}"
ARCHIVE_PATH="build/${PLUGIN_ID}-${PLUGIN_VERSION}.zip"
ROOT_ARCHIVE_PATH="${PLUGIN_ID}.zip"

rm -rf build
mkdir -p "${PACKAGE_ROOT}"

cp -R dist/. "${PACKAGE_ROOT}/"

# Normalize archive permissions so plugin-validator sees deterministic modes.
find "${PACKAGE_ROOT}" -type d -exec chmod 755 {} +
find "${PACKAGE_ROOT}" -type f -exec chmod 644 {} +
find "${PACKAGE_ROOT}" -type f \( -name 'gpx_exasol_*' -o -name '*.exe' \) -exec chmod 755 {} +

umask 022
(
  cd build
  zip -rq "${PLUGIN_ID}-${PLUGIN_VERSION}.zip" "${PLUGIN_ID}"
)

cp "${ARCHIVE_PATH}" "${ROOT_ARCHIVE_PATH}"

printf '%s\n' "${ARCHIVE_PATH}"
