#!/usr/bin/env bash
# Check the patched copy of github.com/extism/go-sdk in
# third_party/extism-go-sdk (task deps:extism:check, decision D59, issue 89).
#
# The 1Password SDK runs its core as WebAssembly through extism and wazero.
# The copy makes extism use the interpreter of wazero instead of the
# compiler: the compiler makes executable memory, which the hardened runtime
# of macOS forbids without an entitlement. go.mod replaces the module with
# the copy. The check makes sure that the copy is the upstream module plus
# the patch third_party/extism-go-sdk.patch, and nothing else:
#
# 1. go.mod replaces github.com/extism/go-sdk with ./third_party/extism-go-sdk,
# 2. the required version is the version in third_party/extism-go-sdk.sum,
#    and the download of that version has the recorded checksum,
# 3. each file of the copy is the upstream file; plugin.go is the upstream
#    file with the patch applied,
# 4. the copy calls NewRuntimeConfigInterpreter once and never
#    NewRuntimeConfig() or NewRuntimeConfigCompiler.
#
# When the 1Password SDK needs a newer extism, the check fails at step 2.
# Then copy the new upstream files, apply the patch again and record the
# new checksum (docs/how-to: release signing).
set -euo pipefail

MOD=github.com/extism/go-sdk
DIR=third_party/extism-go-sdk
PATCH=third_party/extism-go-sdk.patch
SUMS=third_party/extism-go-sdk.sum
# The files of the copy; tests and test data are not part of it.
FILES="extism.go host.go module.go plugin.go runtime.go go.mod go.sum extism-runtime.wasm extism-runtime.wasm.version LICENSE"

status=0
fail() {
  echo "::error::extism-fork-check: $*" >&2
  status=1
}

replace=$(go list -m -f '{{if .Replace}}{{.Replace.Path}}{{end}}' "$MOD")
[ "$replace" = "./$DIR" ] || fail "go.mod must replace $MOD with ./$DIR (found '${replace:-none}')"
version=$(go list -m -f '{{.Version}}' "$MOD")
want_sum=$(awk -v m="$MOD" -v v="$version" '$1 == m && $2 == v { print $3 }' "$SUMS")
[ -n "$want_sum" ] || fail "$SUMS has no checksum for $MOD $version: the SDK needs another extism; update the copy"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
if [ "$status" = 0 ]; then
  # Outside the module, so that the replace does not apply.
  info=$(cd "$tmp" && GOFLAGS=-mod=mod go mod download -json "$MOD@$version")
  upstream=$(echo "$info" | sed -nE 's/^[[:space:]]*"Dir": "(.*)",?$/\1/p')
  got_sum=$(echo "$info" | sed -nE 's/^[[:space:]]*"Sum": "(.*)",?$/\1/p')
  [ "$got_sum" = "$want_sum" ] || fail "the download of $MOD $version has the checksum $got_sum, want $want_sum"
  [ -d "$upstream" ] || fail "no upstream directory for $MOD $version"
fi

if [ "$status" = 0 ]; then
  mkdir "$tmp/want"
  for f in $FILES; do cp "$upstream/$f" "$tmp/want/$f"; done
  chmod -R u+w "$tmp/want"
  (cd "$tmp/want" && patch -s -p1) <"$PATCH" || fail "the patch does not apply to $MOD $version"
  for f in $FILES; do
    cmp -s "$tmp/want/$f" "$DIR/$f" || fail "$DIR/$f differs from upstream $version plus the patch"
  done
  extra=$(cd "$DIR" && ls -A | grep -v -x -F -e PASSIERSCHEIN.md $(printf -- '-e %s ' $FILES) || true)
  [ -z "$extra" ] || fail "$DIR has files that are not in the list: $extra"
fi

n=$(grep -c 'wazero.NewRuntimeConfigInterpreter()' "$DIR/plugin.go" || true)
[ "$n" = 1 ] || fail "$DIR/plugin.go must call wazero.NewRuntimeConfigInterpreter() once (found $n)"
if grep -rnE 'wazero\.NewRuntimeConfig\(\)|NewRuntimeConfigCompiler' "$DIR"/*.go; then
  fail "$DIR selects the compiler of wazero"
fi

if [ "$status" = 0 ]; then
  echo "extism-fork-check: $DIR is $MOD $version plus $PATCH (interpreter of wazero)"
fi
exit "$status"
