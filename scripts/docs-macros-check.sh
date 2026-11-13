#!/usr/bin/env bash
# Check the built documentation for macro calls that were not rendered.
# A page that uses a macro, for example the link to a design artefact, needs
# `render_macros: true` in its front matter. Without it the site shows the
# macro call as text. `mise run docs:build` runs this script after the build.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SITE="$ROOT/site"
[ -d "$SITE" ] || { echo "docs-macros-check: no site/ directory; build the docs first" >&2; exit 1; }

found=$(grep -rl --include='*.html' -F '{{ ref(' "$SITE" || true)
if [ -n "$found" ]; then
  echo "docs-macros-check: a built page shows a macro call as text." >&2
  echo "Add 'render_macros: true' to the front matter of the page:" >&2
  printf '%s\n' "$found" | sed "s|^$SITE/||" >&2
  exit 1
fi
echo "docs-macros-check: no unrendered macro call in the built pages"
