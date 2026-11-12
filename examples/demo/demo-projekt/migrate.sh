#!/usr/bin/env bash
# Simulated database migration of the demo project. It connects to
# nothing. It needs DB_USER and DB_PASSWORD. It prints the user and only
# the length of the password, never the password.
set -euo pipefail

if [ -z "${DB_USER:-}" ] || [ -z "${DB_PASSWORD:-}" ]; then
  echo "migrate: DB_USER and DB_PASSWORD are necessary (see README.md)" >&2
  exit 1
fi

echo "migrate: connected to datenbank-dev as user ${DB_USER} (password: ${#DB_PASSWORD} characters)"
sleep 1
echo "migrate: applied 0001_create_tables (simulated)"
echo "migrate: applied 0002_add_index (simulated)"
echo "migrate: done, schema version 2"
