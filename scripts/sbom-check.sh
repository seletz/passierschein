#!/usr/bin/env bash
# Check the SBOMs of the last goreleaser run in dist/:
# - each archive (tar.gz, and the zip of the app bundle when it is there)
#   has one CycloneDX SBOM with the same base name,
# - each SBOM is a CycloneDX document that names at least one component,
# - the checksum file lists each archive and each SBOM.
# Task release:sbom:check (jq pinned there). Run after release:snapshot.
set -euo pipefail

DIST=${1:-dist}
status=0

fail() {
  echo "::error::$*" >&2
  status=1
}

sums=$(ls "$DIST"/checksums-*.txt 2>/dev/null | head -n 1 || true)
[ -n "$sums" ] || { echo "::error::no checksum file in $DIST" >&2; exit 1; }

count=0
for archive in "$DIST"/*.tar.gz "$DIST"/*.zip; do
  [ -e "$archive" ] || continue
  count=$((count + 1))
  case "$archive" in
    *.tar.gz) sbom="${archive%.tar.gz}.cdx.json" ;;
    *) sbom="${archive%.zip}.cdx.json" ;;
  esac
  if [ ! -f "$sbom" ]; then
    fail "$(basename "$archive") has no SBOM $(basename "$sbom")"
    continue
  fi
  format=$(jq -r '.bomFormat // ""' "$sbom")
  components=$(jq -r '[.metadata.component] + (.components // []) | map(select(. != null)) | length' "$sbom")
  [ "$format" = "CycloneDX" ] || fail "$(basename "$sbom") is not a CycloneDX document"
  [ "$components" -gt 0 ] || fail "$(basename "$sbom") names no component"
  for file in "$archive" "$sbom"; do
    grep -q "  $(basename "$file")\$" "$sums" || fail "$(basename "$sums") does not list $(basename "$file")"
  done
done

[ "$count" -gt 0 ] || { echo "::error::no archives in $DIST" >&2; exit 1; }
if [ "$status" -eq 0 ]; then
  echo "$count archives, each with a CycloneDX SBOM; $(basename "$sums") lists all of them"
fi
exit "$status"
