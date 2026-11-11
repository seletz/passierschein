#!/usr/bin/env bash
# Check that the linux build of a38 has no code for 1Password (test T13).
# The list of packages must not contain:
# - a package of the 1Password SDK (github.com/1password/),
# - a package of this module whose path contains "onepassword" or "1password".
# Task deps:linux:check. With "--self-test" the script checks itself with
# faked package lists.
set -euo pipefail

MODULE=github.com/seletz/passierschein

# forbidden reads a package list on stdin and prints each forbidden package.
forbidden() {
  grep -iE "^github\.com/1password/|^${MODULE}/.*(onepassword|1password)" || true
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
