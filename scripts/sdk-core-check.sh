#!/usr/bin/env bash
# Check that an a38 with hardened runtime runs the WebAssembly core of the
# 1Password SDK (test case T30, decision D59, issue 89). macOS only.
#
#   sdk-core-check.sh            build a38 into a temporary directory, sign
#                                it ad-hoc with hardened runtime and
#                                scripts/a38.entitlements, and check it; then
#                                sign a copy without entitlements and check
#                                that macOS kills it (task test:sdk-core)
#   sdk-core-check.sh <a38>      check this binary as it is, for example the
#                                a38 in the signed bundle
#                                (release-app-check.sh)
#
# The check runs "a38 selftest sdk-core". It needs no token and no network.
# The 1Password SDK runs its core in wazero, which compiles it into memory
# without MAP_JIT. The hardened runtime allows that only with the
# entitlement com.apple.security.cs.allow-unsigned-executable-memory.
# Without it, macOS kills a38 with SIGKILL (code signing), exit status 137.
#
# The check fails if a38 has no hardened runtime, if its entitlements are
# not exactly those of scripts/a38.entitlements, or if the self-test does
# not pass. The copy without entitlements shows that the entitlement is
# still necessary; when the SDK stops compiling into memory, that part
# fails, and D59 can drop the entitlement.
set -euo pipefail

[ "$(uname -s)" = Darwin ] || {
  echo "sdk-core-check: macOS only" >&2
  exit 1
}

ENTITLEMENTS="$(cd "$(dirname "$0")" && pwd)/a38.entitlements"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

status=0
fail() {
  echo "::error::sdk-core-check: $*" >&2
  status=1
}

# keys prints the sorted keys of an entitlements plist on stdin.
keys() { grep -o '<key>[^<]*</key>' | sed -E 's#</?key>##g' | sort; }

# selftest runs "a38 selftest sdk-core" with the binary $1 and prints the
# exit status.
selftest() {
  set +e
  "$1" selftest sdk-core >&2
  local code=$?
  set -e
  echo "$code"
}

own=0
if [ $# -ge 1 ]; then
  bin=$1
  [ -x "$bin" ] || {
    echo "::error::sdk-core-check: $bin is not an executable file" >&2
    exit 1
  }
else
  own=1
  bin=$tmp/a38
  CGO_ENABLED=0 go build -trimpath -o "$tmp/a38.unsigned" ./cmd/a38
  cp "$tmp/a38.unsigned" "$bin"
  codesign --force --sign - --options runtime --entitlements "$ENTITLEMENTS" --identifier io.github.seletz.passierschein.a38 "$bin"
fi

flags=$(codesign -d --verbose=2 "$bin" 2>&1 | sed -nE 's/^CodeDirectory .* flags=0x[0-9a-f]+\(([^)]*)\).*/\1/p')
echo "sdk-core-check: $bin flags=$flags"
case ",$flags," in *,runtime,*) ;; *) fail "$bin has no hardened runtime; the check would prove nothing" ;; esac
got=$(codesign -d --entitlements - --xml "$bin" 2>/dev/null | keys || true)
want=$(keys <"$ENTITLEMENTS")
echo "sdk-core-check: entitlements: $(echo "$got" | paste -sd ' ' -)"
[ "$got" = "$want" ] || fail "$bin has the entitlements '$(echo "$got" | paste -sd ' ' -)', want exactly '$(echo "$want" | paste -sd ' ' -)' (D59)"

code=$(selftest "$bin")
case "$code" in
  0) ;;
  137) fail "macOS killed a38 (SIGKILL): the hardened runtime forbids the code of the SDK core (OS_REASON_CODESIGNING)" ;;
  *) fail "a38 selftest sdk-core ended with exit status $code" ;;
esac

if [ "$own" = 1 ]; then
  # The counter-check: without the entitlement macOS kills a38.
  cp "$tmp/a38.unsigned" "$tmp/a38-plain"
  codesign --force --sign - --options runtime --identifier io.github.seletz.passierschein.a38 "$tmp/a38-plain"
  code=$(selftest "$tmp/a38-plain" 2>/dev/null)
  if [ "$code" = 137 ]; then
    echo "sdk-core-check: without the entitlement macOS kills a38 (exit status 137), as expected"
  else
    fail "without the entitlement a38 selftest sdk-core ended with $code, want 137: the entitlement may be unnecessary now; check D59"
  fi
fi

if [ "$status" = 0 ]; then
  echo "sdk-core-check: $bin runs the SDK core with hardened runtime"
fi
exit "$status"
