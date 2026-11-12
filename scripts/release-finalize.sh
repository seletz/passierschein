#!/usr/bin/env bash
# The last job of the release workflow (tasks release:assets:check and
# release:finalize, CI only).
#
#   release-finalize.sh check    the draft of the tag has every asset: per
#                                platform the archive and its SBOM, the app
#                                zip and its SBOM, the checksum file; the
#                                checksum file lists each of them. The
#                                checksum file is saved to dist/ for the
#                                attestation step.
#   release-finalize.sh publish  publishes the draft.
#
# A missing asset fails the run before anything is published.
set -euo pipefail
. "$(dirname "$0")/release-app-lib.sh"

TAG=${GITHUB_REF_NAME:-}
[ -n "$TAG" ] || die "GITHUB_REF_NAME is not set; this task runs in the release workflow only"
draft=$(gh release view "$TAG" --json isDraft --jq .isDraft) || die "no release for $TAG"
[ "$draft" = true ] || die "the release $TAG is not a draft"

case "${1:-}" in
  check)
    assets=$(gh release view "$TAG" --json assets --jq '.assets[].name')
    want=("checksums-$TAG.txt" "passierschein-app-$TAG-darwin-universal.zip" "passierschein-app-$TAG-darwin-universal.cdx.json")
    for p in darwin-amd64 darwin-arm64 linux-amd64 linux-arm64; do
      want+=("passierschein-$TAG-$p.tar.gz" "passierschein-$TAG-$p.cdx.json")
    done
    mkdir -p "$DIST"
    rm -f "$DIST/checksums-$TAG.txt"
    gh release download "$TAG" --pattern "checksums-$TAG.txt" --dir "$DIST"
    status=0
    for a in "${want[@]}"; do
      grep -qx -F "$a" <<<"$assets" || { echo "::error::the draft $TAG lacks $a" >&2; status=1; }
      [ "$a" = "checksums-$TAG.txt" ] && continue
      grep -q -E "  ${a//./\\.}\$" "$DIST/checksums-$TAG.txt" || { echo "::error::checksums-$TAG.txt does not list $a" >&2; status=1; }
    done
    [ "$status" = 0 ] && echo "release-finalize: the draft $TAG has all ${#want[@]} assets"
    exit "$status"
    ;;
  publish)
    gh release edit "$TAG" --draft=false
    echo "release-finalize: $TAG is published"
    ;;
  *) die "usage: release-finalize.sh check|publish" ;;
esac
