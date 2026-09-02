#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ANDROID_RES_DIR="$ROOT_DIR/mobile/android/app/src"
FILE_PATHS="$ANDROID_RES_DIR/main/res/xml/file_paths.xml"
KEEP_RULE="$ANDROID_RES_DIR/main/res/raw/keep.xml"

require_file() {
  if [[ ! -f "$1" ]]; then
    printf 'missing required Android resource: %s\n' "$1" >&2
    exit 1
  fi
}

require_unique_resource() {
  local resource_path="$1"
  local label="$2"
  local count
  count="$(find "$ANDROID_RES_DIR" -type f -path "*/res/$resource_path" | wc -l | tr -d '[:space:]')"
  if [[ "$count" != "1" ]]; then
    printf 'expected exactly one %s resource, found %s\n' "$label" "$count" >&2
    find "$ANDROID_RES_DIR" -type f -path "*/res/$resource_path" -print >&2
    exit 1
  fi
}

require_file "$FILE_PATHS"
require_file "$KEEP_RULE"
require_unique_resource "xml/file_paths.xml" "@xml/file_paths"
require_unique_resource "raw/keep.xml" "@raw/keep"

if find "$ANDROID_RES_DIR" -type f -path '*/res/values/keep.xml' -print -quit | grep -q .; then
  printf 'legacy values/keep.xml would redeclare @xml/file_paths; use raw/keep.xml tools:keep instead\n' >&2
  exit 1
fi

if ! grep -Fq 'tools:keep="@xml/file_paths"' "$KEEP_RULE"; then
  printf 'Android resource keep rule is missing @xml/file_paths: %s\n' "$KEEP_RULE" >&2
  exit 1
fi

if ! grep -Fq '<files-path name="update_downloads" path="update_downloads/" />' "$FILE_PATHS"; then
  printf 'FileProvider resource must expose only the private update_downloads directory: %s\n' "$FILE_PATHS" >&2
  exit 1
fi

if grep -Eq '<(root-path|external-path|external-files-path|cache-path|external-cache-path)([[:space:]>])' "$FILE_PATHS"; then
  printf 'FileProvider resource exposes a disallowed broad path: %s\n' "$FILE_PATHS" >&2
  exit 1
fi

printf 'Android FileProvider resources are unique and restricted.\n'
