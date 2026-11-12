#!/usr/bin/env bash
# Check that the linux build of a38 has no code for 1Password (test T13).
# The list of packages must not contain:
# - a package of the 1Password SDK (github.com/1password/),
# - a package of the WebAssembly runtime of the SDK (github.com/extism/,
#   github.com/tetratelabs/wazero, github.com/dylibso/),
# - a package of this module whose path contains "onepassword" or "1password".
# Task deps:linux:check. With "--self-test" the script checks itself with
# faked package lists, and then with real package lists: the darwin build
# must contain the 1Password backend and the SDK, and a linux build with the
# build constraint "darwin" switched on (as if the constraint were removed)
# must be caught. So the check is known to guard something.
set -euo pipefail

MODULE=github.com/seletz/passierschein

# forbidden reads a package list on stdin and prints each forbidden package.
forbidden() {
  grep -iE "^github\.com/1password/|^github\.com/extism/|^github\.com/tetratelabs/wazero|^github\.com/dylibso/|^${MODULE}/.*(onepassword|1password)" || true
}

if [ "${1:-}" = "--self-test" ]; then
  fail=0
  expect() { # expect <name> <want-count> <list>
    got=$(printf '%s\n' "$3" | forbidden | grep -c . || true)
    if [ "$got" != "$2" ]; then
      echo "self-test '$1': found $got forbidden packages, want $2" >&2
      fail=1
    fi
  }
  expect clean 0 $'fmt\ngithub.com/seletz/passierschein/internal/protocol'
  expect sdk 1 $'fmt\ngithub.com/1password/onepassword-sdk-go'
  expect backend 1 $'fmt\ngithub.com/seletz/passierschein/internal/backend/onepassword'
  expect backend-digit 1 $'github.com/seletz/passierschein/internal/1password'
  expect other-module 0 $'github.com/other/onepassword'
  expect wasm-runtime 2 $'github.com/extism/go-sdk\ngithub.com/tetratelabs/wazero/api'

  # must_catch <name> <package list>: the list must hold the backend and
  # the SDK, and the filter must find them.
  must_catch() {
    found=$(printf '%s\n' "$2" | forbidden)
    for pkg in "${MODULE}/internal/backend/onepassword" "github.com/1password/onepassword-sdk-go"; do
      if ! printf '%s\n' "$found" | grep -qxF "$pkg"; then
        echo "self-test '$1': $pkg is not found; the check would guard nothing" >&2
        fail=1
      fi
    done
  }
  # Control: the darwin build contains the backend and the SDK.
  must_catch darwin-build "$(GOOS=darwin go list -deps ./cmd/a38)"
  # Negative test: with the constraint "darwin" switched on in a linux
  # build (as if the build constraint were removed), the check fails.
  must_catch linux-without-constraint "$(GOOS=linux go list -tags darwin -deps ./cmd/a38)"
  [ "$fail" = 0 ] && echo "self-test ok"
  exit "$fail"
fi

list=$(GOOS=linux go list -deps ./cmd/a38)
found=$(printf '%s\n' "$list" | forbidden)
if [ -n "$found" ]; then
  echo "::error::the linux build of a38 contains code for 1Password:" >&2
  printf '%s\n' "$found" >&2
  exit 1
fi
echo "the linux build of a38 has no 1Password packages ($(printf '%s\n' "$list" | wc -l | tr -d ' ') packages checked)"
