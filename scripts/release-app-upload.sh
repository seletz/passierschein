#!/usr/bin/env bash
# Add the app zip, its SBOM and the extended checksum file to the draft
# release of the tag (task release:app:upload, CI only, release workflow).
# goreleaser created the draft; release:finalize publishes it.
set -euo pipefail
. "$(dirname "$0")/release-app-lib.sh"

TAG=${GITHUB_REF_NAME:-}
[ -n "$TAG" ] || die "GITHUB_REF_NAME is not set; release:app:upload runs in the release workflow only"
[ "$(release_tag)" = "$TAG" ] || die "$DIST holds tag $(release_tag), the workflow runs for $TAG"
NAME=$(app_asset)
draft=$(gh release view "$TAG" --json isDraft --jq .isDraft) || die "no release for $TAG"
[ "$draft" = true ] || die "the release $TAG is published already; the app is added only to a draft"
gh release upload "$TAG" --clobber "$DIST/$NAME.zip" "$DIST/$NAME.cdx.json" "$DIST/checksums-$TAG.txt"
echo "release-app-upload: $NAME.zip, $NAME.cdx.json and checksums-$TAG.txt are in the draft $TAG"
