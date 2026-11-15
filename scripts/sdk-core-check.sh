#!/usr/bin/env bash
# Check that an a38 with hardened runtime runs the WebAssembly core of the
# 1Password SDK (test case T30, decision D59, issue 89). macOS only.
#
#   sdk-core-check.sh            build a38 into a temporary directory, sign
#                                it ad-hoc with hardened runtime and no
#                                entitlements, and check it (task
#                                test:sdk-core)
#   sdk-core-check.sh <a38>      check this binary as it is, for example the
#                                a38 in the signed bundle
#                                (release-app-check.sh)
#
# The check runs "a38 selftest sdk-core". It needs no token and no network.
# If the hardened runtime forbids the code of the core, macOS kills a38
# with SIGKILL (code signing), and the check fails with the exit status.
# The check also fails if a38 has an entitlement that weakens the
# hardened runtime for code in memory (allow-jit,
# allow-unsigned-executable-memory, disable-executable-page-protection):
# D59 runs the core without them.
set -euo pipefail

[ "$(uname -s)" = Darwin ] || {
  echo "sdk-core-check: macOS only" >&2
  exit 1
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

if [ $# -ge 1 ]; then
  bin=$1
  [ -x "$bin" ] || {
    echo "::error::sdk-core-check: $bin is not an executable file" >&2
    exit 1
  }
else
  bin=$tmp/a38
  CGO_ENABLED=0 go build -trimpath -o "$bin" ./cmd/a38
  codesign --force --sign - --options runtime --identifier io.github.seletz.passierschein.a38 "$bin"
fi

status=0
fail() {
  echo "::error::sdk-core-check: $*" >&2
  status=1
}

flags=$(codesign -d --verbose=2 "$bin" 2>&1 | sed -nE 's/^CodeDirectory .* flags=0x[0-9a-f]+\(([^)]*)\).*/\1/p')
echo "sdk-core-check: $bin flags=$flags"
case ",$flags," in *,runtime,*) ;; *) fail "$bin has no hardened runtime; the check would prove nothing" ;; esac
ents=$(codesign -d --entitlements - --xml "$bin" 2>/dev/null || true)
for e in allow-jit allow-unsigned-executable-memory disable-executable-page-protection; do
  case "$ents" in *"com.apple.security.cs.$e"*) fail "$bin has the entitlement com.apple.security.cs.$e (D59 runs without it)" ;; esac
done

set +e
"$bin" selftest sdk-core
code=$?
set -e
case "$code" in
  0) ;;
  137) fail "macOS killed a38 (SIGKILL): the hardened runtime forbids the code of the SDK core (OS_REASON_CODESIGNING)" ;;
  *) fail "a38 selftest sdk-core ended with exit status $code" ;;
esac

if [ "$status" = 0 ]; then
  echo "sdk-core-check: $bin runs the SDK core with hardened runtime"
fi
exit "$status"
