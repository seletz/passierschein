#!/usr/bin/env bash
# Simulated upload of the demo project to the object storage. It connects
# to nothing. It needs OBJEKTSPEICHER_ACCESS_KEY. It prints only the
# length of the key, never the key.
set -euo pipefail

if [ -z "${OBJEKTSPEICHER_ACCESS_KEY:-}" ]; then
  echo "upload: OBJEKTSPEICHER_ACCESS_KEY is necessary (see README.md)" >&2
  exit 1
fi

echo "upload: connected to objektspeicher-dev (access key: ${#OBJEKTSPEICHER_ACCESS_KEY} characters)"
sleep 1
echo "upload: uploaded build/demo-projekt.tar.gz (simulated)"
