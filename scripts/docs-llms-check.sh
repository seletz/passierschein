#!/usr/bin/env bash
# Check the LLM output of the built documentation (llmstxt plugin of Zensical).
# The output is public. The script fails when
#   - llms.txt or llms-full.txt is missing or empty,
#   - a link in llms.txt points to a file that does not exist in site/,
#   - the LLM output shows a macro call or template code as text,
#   - the LLM output contains a private key block.
# `mise run docs:build` runs this script after the build.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SITE="${DOCS_LLMS_SITE:-$ROOT/site}"
BASE=$(sed -n 's/^site_url *= *"\(.*\)"/\1/p' "$ROOT/zensical.toml")
BASE=${BASE%/}
fail=0
err() { echo "docs-llms-check: $*" >&2; fail=1; }

for f in llms.txt llms-full.txt; do
  [ -s "$SITE/$f" ] || err "$f is missing or empty; check [project.plugins.llmstxt] in zensical.toml"
done
[ "$fail" -eq 0 ] || exit 1

# Every link in llms.txt leads to a Markdown file in site/.
links=$(grep -o '(<[^>]*>)' "$SITE/llms.txt" | sed 's/^(<//; s/>)$//' || true)
[ -n "$links" ] || err "llms.txt lists no pages"
count=0
while IFS= read -r url; do
  [ -n "$url" ] || continue
  count=$((count + 1))
  case "$url" in
    "$BASE"/*) [ -f "$SITE/${url#"$BASE"/}" ] || err "llms.txt links to a missing file: $url" ;;
    *) err "llms.txt links outside the site: $url" ;;
  esac
done <<< "$links"

# Macro calls and template code must not appear as text. The page that
# describes the macros writes them in code spans without braces.
for f in llms.txt llms-full.txt; do
  hit=$(grep -n -E '\{\{|\{%|\{#' "$SITE/$f" | head -3 || true)
  [ -z "$hit" ] || err "$f shows template code as text: $(printf '%s' "$hit" | cut -c1-120)"
done
# Per-page Markdown files (links are checked above; scan them as a whole).
hit=$(grep -rl --include='*.md' -E '\{\{|\{%' "$SITE" | head -3 || true)
[ -z "$hit" ] || err "a Markdown page shows template code as text: $hit"

# Secrets: no private key block.
if grep -q -E -- '-----BEGIN [A-Z ]*PRIVATE KEY-----' "$SITE/llms-full.txt"; then
  err "llms-full.txt contains a private key block"
fi

[ "$fail" -eq 0 ] || exit 1
echo "docs-llms-check: llms.txt lists $count pages, all files exist, no template code"
