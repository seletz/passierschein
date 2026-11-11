#!/usr/bin/env bash
# Interactive semver release: bump the latest v* tag, create an annotated tag
# and push it. The release workflow (.github/workflows/release.yml) builds the
# archives, SBOMs and checksums, creates the GitHub release and publishes the
# documentation. The release notes come from the Conventional Commits since
# the last tag. Run through `mise run release`.
set -euo pipefail

if [ -n "$(git status --porcelain)" ]; then
    echo "The working tree is dirty. Commit or stash first." >&2
    exit 1
fi

git fetch --tags --quiet origin

LATEST=$(git tag -l 'v*' --sort=-v:refname | head -n1)
LATEST=${LATEST:-v0.0.0}

echo "Current version: $LATEST"
echo ""
echo "Select the bump type:"
echo "  1) patch"
echo "  2) minor"
echo "  3) major"
echo ""
read -rp "Choice [1/2/3]: " CHOICE

VERSION="${LATEST#v}"
IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"

case "$CHOICE" in
    1|patch) PATCH=$((PATCH + 1)) ;;
    2|minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
    3|major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
    *) echo "Invalid choice." >&2; exit 1 ;;
esac

NEXT="v${MAJOR}.${MINOR}.${PATCH}"

echo ""
read -rp "Create release $NEXT? [y/N]: " CONFIRM
if [[ "$CONFIRM" != "y" && "$CONFIRM" != "Y" ]]; then
    echo "Aborted."
    exit 0
fi

git tag -a "$NEXT" -m "Release $NEXT"
git push origin "$NEXT"
echo "Tag $NEXT is pushed. The release workflow builds and publishes the release."
