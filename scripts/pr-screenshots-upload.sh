#!/usr/bin/env bash
# Put screenshots for a pull request on the branch `review-assets` and print
# the Markdown lines for the pull request description.
#
# The branch `review-assets` holds only pictures. No one merges it. The
# pictures stay out of `develop`.
#
# Usage: scripts/pr-screenshots-upload.sh <pr-number> <file.png>...
set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "usage: $0 <pr-number> <file.png>..." >&2
  exit 2
fi
pr=$1
shift

repo=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
git fetch --quiet origin review-assets

index=$(mktemp)
trap 'rm -f "$index"' EXIT
export GIT_INDEX_FILE=$index
git read-tree origin/review-assets
for file in "$@"; do
  hash=$(git hash-object -w "$file")
  git update-index --add --cacheinfo "100644,$hash,pr$pr/$(basename "$file")"
done
tree=$(git write-tree)
commit=$(git -c commit.gpgsign=false commit-tree "$tree" -p origin/review-assets \
  -m "docs: add screenshots for pull request $pr")
unset GIT_INDEX_FILE
git push --quiet origin "$commit:refs/heads/review-assets"

for file in "$@"; do
  name=$(basename "$file")
  echo "![${name%.*}](https://github.com/$repo/blob/review-assets/pr$pr/$name?raw=true)"
done
