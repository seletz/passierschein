#!/usr/bin/env bash
# Check every darwin binary of the last goreleaser run (dist/artifacts.json):
# fail when one is unsigned, only ad-hoc signed, lacks the hardened runtime
# flag, a team ID or a certificate chain, or when its entitlements are not
# exactly those of scripts/a38.entitlements (D59, issue 89). Only flags, team ID and the CMS
# verdict are printed. Whether Apple accepted the notarization is decided by
# goreleaser itself (notarize.wait); the Gatekeeper check needs a Mac.
# Task release:signatures:check (quill and jq pinned there).
set -euo pipefail
status=0
bins=$(jq -r '.[] | select(.type == "Binary" and .goos == "darwin") | .path' dist/artifacts.json)
[ -n "$bins" ] || { echo "::error::no darwin binaries in dist/artifacts.json" >&2; exit 1; }
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
# keys prints the sorted keys of an entitlements plist on stdin.
keys() { grep -o '<key>[^<]*</key>' | sed -E 's#</?key>##g' | sort || true; }
want_ents=$(keys <"$(dirname "$0")/a38.entitlements")
for bin in $bins; do
  quill describe -o json "$bin" > "$tmp"
  flags=$(jq -r '.[0].superBlob.codeDirectories[0].flags.description' "$tmp")
  team=$(jq -r '.[0].superBlob.codeDirectories[0].teamID' "$tmp")
  certs=$(jq -r '[.[0].superBlob.signatures[0].certificates[]?] | length' "$tmp")
  valid=$(jq -r '.[0].superBlob.signatures[0].cmsValidation.isValid' "$tmp")
  ents=$(jq -r '.[0].superBlob.entitlements.entitlements // ""' "$tmp" | keys)
  echo "$bin: flags=$flags teamID=$team certificates=$certs cmsValid=$valid entitlements=$(echo "$ents" | paste -sd ' ' -)"
  if [ "$ents" != "$want_ents" ]; then
    echo "::error::$bin does not have exactly the entitlements of scripts/a38.entitlements (D59)" >&2; status=1
  fi
  case "$flags" in
    *adhoc*) echo "::error::$bin is only ad-hoc signed" >&2; status=1 ;;
    *runtime*) ;;
    *) echo "::error::$bin has no hardened runtime flag" >&2; status=1 ;;
  esac
  if [ -z "$team" ] || [ "$team" = "null" ]; then
    echo "::error::$bin has no team ID" >&2; status=1
  fi
  if [ "$certs" = "0" ]; then
    echo "::error::$bin carries no certificate chain" >&2; status=1
  fi
  if [ "$valid" != "true" ]; then
    echo "::warning::$bin: quill could not validate the CMS signature"
  fi
done
exit "$status"
