#!/usr/bin/env bash
# Demo kit of a38 (issue 62). The how-to docs/how-to/50-demo-vorfuehren.md
# is the runbook.
#
#   demo.sh setup             build and install the demo
#   demo.sh preflight         check the demo (PASS/FAIL lines), start nothing
#   demo.sh start             preflight, then run the daemon in the foreground
#   demo.sh status            doctor, grants, fault status, last audit entries, audit verify
#   demo.sh fault             make the audit directory read-only (global lock)
#   demo.sh fault-fix         restore the mode; the presenter runs a38 fault clear
#   demo.sh request <act>     send examples/demo/requests/<act>.jsonl to a38 mcp
#   demo.sh teardown          remove what setup installed
#
# Two modes:
#
# - Real (default): installs into the home directory of the user:
#   ~/.local/bin/a38, ~/Applications/Passierschein.app, the configuration
#   ~/.config/a38/config.yaml (default path, so a38 needs no --config) and
#   the demo project ~/develop/demo-projekt. The list of installed items is
#   in ~/.local/share/a38-demo/installed. Setup never overwrites an item
#   that it did not install. Teardown removes the binary, the app and the
#   project, and keeps the configuration and the audit log
#   (DEMO_PURGE=1 moves the configuration to a .bak file and removes the
#   state directory). DEMO_FORCE_CONFIG=1 moves an existing configuration
#   to config.yaml.bak-<time> before setup writes its own.
#
# - Rehearsal (DEMO_BACKEND=fake): everything in DEMO_DIR (default
#   /tmp/a38-probe) with --config, the fake backend and a fake approval
#   (DEMO_UI=auto-approve|auto-deny|debug-app at start). It never touches
#   the files of the real mode. Give DEMO_BACKEND=fake to each task.
#
# DEMO_SKIP_BUILD=1 skips the builds at setup.
#
# The script never reads the token file and never prints a credential
# value.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
MODE=real
case "${DEMO_BACKEND:-1password}" in
  1password) ;;
  fake) MODE=fake ;;
  *)
    echo "demo: DEMO_BACKEND must be 1password or fake" >&2
    exit 1
    ;;
esac

SESSION_NAME=demo-praesentation
HAND_SESSION_NAME=demo-handaufruf
CONFIG_HOME=${XDG_CONFIG_HOME:-$HOME/.config}
TOKEN_FILE="$CONFIG_HOME/a38/onepassword-token"

if [ "$MODE" = real ]; then
  STATE_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/a38"
  KIT_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/a38-demo"
  INSTALLED="$KIT_DIR/installed"
  A38="$HOME/.local/bin/a38"
  APP_BUNDLE="$HOME/Applications/Passierschein.app"
  CONFIG="$CONFIG_HOME/a38/config.yaml"
  PROJECT_DIR="$HOME/develop/demo-projekt"
  SOCKET_DIR="$STATE_DIR/run"
  AUDIT_DIR="$STATE_DIR"
  FAULT_MODE_FILE="$KIT_DIR/fault-mode"
else
  DEMO_DIR=${DEMO_DIR:-/tmp/a38-probe}
  case "$DEMO_DIR" in
    /tmp/?* | /private/tmp/?*) ;;
    *)
      echo "demo: the rehearsal directory DEMO_DIR must be below /tmp" >&2
      exit 1
      ;;
  esac
  MARKER="$DEMO_DIR/.a38-demo"
  A38="$DEMO_DIR/bin/a38"
  APP_BUNDLE="$DEMO_DIR/bin/Passierschein.app"
  CONFIG="$DEMO_DIR/config.yaml"
  PROJECT_DIR="$DEMO_DIR/demo-projekt"
  SOCKET_DIR="$DEMO_DIR/run"
  AUDIT_DIR="$DEMO_DIR/audit"
  FAULT_MODE_FILE="$DEMO_DIR/fault-mode"
  PID_FILE="$DEMO_DIR/daemon.pid"
fi
APP_EXE="$APP_BUNDLE/Contents/MacOS/Passierschein"
PROJECT_MARKER="$PROJECT_DIR/.a38-demo"

die() {
  echo "demo: $*" >&2
  exit 1
}

# tilde prints a path below the home directory in the form ~/...
tilde() {
  # shellcheck disable=SC2088 # the tilde is text for the reader
  case "$1" in
    "$HOME"/*) echo "~/${1#"$HOME"/}" ;;
    *) echo "$1" ;;
  esac
}

# ours reports whether setup of the real mode installed the path.
ours() {
  [ -f "$INSTALLED" ] && grep -qxF "$1" "$INSTALLED"
}

need_setup() {
  if [ "$MODE" = real ]; then
    [ -f "$INSTALLED" ] || die "the demo is not installed; run: mise run demo:setup"
  else
    [ -f "$MARKER" ] || die "no rehearsal in $DEMO_DIR; run: DEMO_BACKEND=fake mise run demo:setup"
  fi
}

# daemon_answers reports whether a daemon answers on the admin socket of
# the demo configuration.
daemon_answers() {
  [ -x "$A38" ] && "$A38" grants list --config "$CONFIG" >/dev/null 2>&1
}

# write_config writes the configuration of the mode to the file $1.
write_config() {
  local project_real=$1 out=$2
  if [ "$MODE" = real ]; then
    cat >"$out" <<EOF
# Configuration of a38. Written by "mise run demo:setup" (demo kit).
# Reference: docs/reference/80-befehle-fuer-den-menschen.md and the
# package documentation of internal/config.
version: 1

# The vaults that a38 can see. A vault that is not in this list does not
# exist for a38. Only a vault of the environment dev delivers values today.
vaults:
  - name: passierschein.dev
    environment: dev

# The projects. A session belongs to a project through the working
# directory of the agent. A project can use only the vaults it names.
projects:
  - name: demo-projekt
    paths:
      - $(tilde "$project_real")
    vaults:
      - passierschein.dev

# How long an approval lasts. The human can choose a shorter time in the
# popup, never a longer one.
grant:
  lifetime: 4h

# Requests of one session within the window go into one popup.
bundle:
  window: 3s
  max_entries: 10

# The approval app. The sockets and the audit log use the defaults:
# ~/.local/state/a38/run and ~/.local/state/a38/audit.db.
paths:
  ui_app: $(tilde "$APP_EXE")

# The 1Password backend reads the token of the service account from this
# file (mode 0600). Only "a38 serve --backend 1password" reads it. The
# demo uses the token file; without token_source the daemon would read the
# macOS keychain (a38 config onepassword-token set).
backend:
  onepassword:
    token_source: file
    token_file: $(tilde "$TOKEN_FILE")
EOF
  else
    cat >"$out" <<EOF
# Configuration of the a38 rehearsal (DEMO_BACKEND=fake mise run demo:setup).
version: 1
vaults:
  - name: passierschein.dev
    environment: dev
projects:
  - name: demo-projekt
    paths:
      - $project_real
    vaults:
      - passierschein.dev
paths:
  socket_dir: $SOCKET_DIR
  audit_file: $AUDIT_DIR/audit.db
  ui_app: $APP_EXE
EOF
  fi
}

# write_project fills the copied demo project: MCP server, skill, settings.
write_project() {
  local command args
  if [ "$MODE" = real ]; then
    command=a38
    args="\"mcp\", \"--session-name\", \"$SESSION_NAME\""
  else
    command=$A38
    args="\"mcp\", \"--config\", \"$CONFIG\", \"--session-name\", \"$SESSION_NAME\""
  fi
  # Project-scoped MCP server for Claude Code (.mcp.json at the project
  # root, https://code.claude.com/docs/en/mcp).
  cat >"$PROJECT_DIR/.mcp.json" <<EOF
{
  "mcpServers": {
    "a38": {
      "type": "stdio",
      "command": "$command",
      "args": [$args]
    }
  }
}
EOF
  # Project skill (.claude/skills/<name>/SKILL.md,
  # https://code.claude.com/docs/en/skills). The popup is the control, so
  # the two tools of a38 need no extra prompt of the agent tool.
  mkdir -p "$PROJECT_DIR/.claude/skills/a38"
  "$A38" llm --skill >"$PROJECT_DIR/.claude/skills/a38/SKILL.md"
  write_settings
  echo "created by mise run demo:setup" >"$PROJECT_MARKER"
}

# write_settings writes .claude/settings.json of the demo project
# (https://code.claude.com/docs/en/settings): the two tools of a38 need no
# extra prompt, and the hook "a38 hook op-guard" blocks a direct call of op
# (https://code.claude.com/docs/en/hooks, F16). The hooks block comes from
# "a38 hook op-guard --print-config": the bare name "a38" in the real mode
# (like .mcp.json), the absolute path of the rehearsal binary otherwise.
# A file that keep_settings saved from an earlier setup is merged: the kit
# adds its entries once and keeps every other entry of the owner.
write_settings() {
  local file="$PROJECT_DIR/.claude/settings.json" scope=project kit merged
  [ "$MODE" = real ] || scope=user
  command -v jq >/dev/null || die "jq is missing; run through mise run demo:setup"
  kit=$("$A38" hook op-guard --print-config --scope "$scope" |
    jq '{permissions: {allow: ["mcp__a38__request_credentials", "mcp__a38__grant_status"]}} + .')
  if [ -f "$file" ]; then
    merged=$(jq --argjson kit "$kit" '
      .permissions.allow = ((.permissions.allow // []) + ($kit.permissions.allow - (.permissions.allow // [])))
      | .hooks.PreToolUse = ((.hooks.PreToolUse // [])
          | if any(.[]; any(.hooks[]?; (.args // []) == ["hook", "op-guard"])) then . else . + $kit.hooks.PreToolUse end)
    ' "$file") || die "$file is not valid JSON; fix or remove it, then run setup again"
    printf '%s\n' "$merged" >"$file"
  else
    printf '%s\n' "$kit" >"$file"
  fi
}

# keep_settings copies .claude/settings.json of an earlier demo project to
# the file $1, so that setup keeps the edits of the owner. It copies only
# from a project with the kit marker.
keep_settings() {
  if [ -f "$PROJECT_MARKER" ] && [ -f "$PROJECT_DIR/.claude/settings.json" ]; then
    command -v jq >/dev/null || die "jq is missing; run through mise run demo:setup"
    jq empty "$PROJECT_DIR/.claude/settings.json" 2>/dev/null ||
      die "$PROJECT_DIR/.claude/settings.json is not valid JSON; fix or remove it, then run setup again"
    cp "$PROJECT_DIR/.claude/settings.json" "$1"
  fi
}

# restore_settings puts the file of keep_settings into the new project.
restore_settings() {
  if [ -s "$1" ]; then
    mkdir -p "$PROJECT_DIR/.claude"
    cp "$1" "$PROJECT_DIR/.claude/settings.json"
  fi
  rm -f "$1"
}

build() {
  if [ "${DEMO_SKIP_BUILD:-}" != 1 ]; then
    echo "demo: build a38 and the approval app (the first build of the app takes minutes)"
    (cd "$ROOT" && mise run build && mise run app:build)
  fi
  [ -x "$ROOT/bin/a38" ] || die "$ROOT/bin/a38 is missing; run: mise run build"
  [ -x "$ROOT/app/.build/bundle/Passierschein.app/Contents/MacOS/Passierschein" ] ||
    die "the app bundle is missing; run: mise run app:build"
}

# install_app copies the bundle with ditto (keeps the ad-hoc signature)
# and verifies the signature.
install_app() {
  local tmp="$APP_BUNDLE.new.$$"
  rm -rf "$tmp"
  ditto "$ROOT/app/.build/bundle/Passierschein.app" "$tmp"
  codesign --verify --strict "$tmp" || die "the signature of the copied app is not valid"
  rm -rf "$APP_BUNDLE"
  mv "$tmp" "$APP_BUNDLE"
  chmod -R go-w "$APP_BUNDLE"
}

cmd_setup_real() {
  local longest="$SOCKET_DIR/admin.sock"
  [ "${#longest}" -le 103 ] || die "the socket path $longest is longer than 103 bytes"

  # Check everything before the first change.
  if [ -e "$A38" ] && ! ours "$A38"; then
    die "$A38 exists and the demo kit did not install it; remove it yourself or keep it, setup stops"
  fi
  if [ -e "$APP_BUNDLE" ] && ! ours "$APP_BUNDLE"; then
    die "$APP_BUNDLE exists and the demo kit did not install it; setup stops"
  fi
  if [ -e "$PROJECT_DIR" ] && [ ! -f "$PROJECT_MARKER" ]; then
    die "$PROJECT_DIR exists and the demo kit did not create it; setup stops"
  fi
  if daemon_answers; then
    die "a daemon runs with $CONFIG; stop it (Ctrl-C) before setup"
  fi
  mkdir -p "$HOME/develop"
  local project_real new_config
  project_real="$(cd "$HOME/develop" && pwd -P)/demo-projekt"
  new_config=$(mktemp "${TMPDIR:-/tmp}/a38-demo-config.XXXXXX")
  # shellcheck disable=SC2064 # expand now
  trap "rm -f '$new_config'" EXIT
  write_config "$project_real" "$new_config"
  local config_action=write
  if [ -e "$CONFIG" ]; then
    if cmp -s "$CONFIG" "$new_config"; then
      config_action=keep
    elif [ "${DEMO_FORCE_CONFIG:-}" = 1 ]; then
      config_action=backup
    else
      die "$CONFIG exists and differs from the demo configuration; setup does not overwrite it.
      Keep it, or run with DEMO_FORCE_CONFIG=1: setup then moves it to config.yaml.bak-<time>"
    fi
  fi

  build

  mkdir -p "$KIT_DIR" "$HOME/.local/bin" "$HOME/Applications"
  touch "$INSTALLED"
  local tmp="$A38.new.$$"
  cp "$ROOT/bin/a38" "$tmp"
  chmod 755 "$tmp"
  mv "$tmp" "$A38" # atomic: a running a38 keeps its old file
  ours "$A38" || echo "$A38" >>"$INSTALLED"
  install_app
  ours "$APP_BUNDLE" || echo "$APP_BUNDLE" >>"$INSTALLED"

  local kept_settings="$KIT_DIR/settings.json.kept"
  rm -f "$kept_settings"
  keep_settings "$kept_settings"
  rm -rf "$PROJECT_DIR"
  cp -R "$ROOT/examples/demo/demo-projekt" "$PROJECT_DIR"
  chmod 755 "$PROJECT_DIR"/*.sh
  restore_settings "$kept_settings"
  write_project
  ours "$PROJECT_DIR" || echo "$PROJECT_DIR" >>"$INSTALLED"

  local backup=""
  case "$config_action" in
    backup)
      backup="$CONFIG.bak-$(date +%Y%m%d-%H%M%S)"
      mv "$CONFIG" "$backup"
      ;;
  esac
  if [ "$config_action" != keep ]; then
    if [ ! -d "$CONFIG_HOME/a38" ]; then
      mkdir -p "$CONFIG_HOME/a38"
      chmod 700 "$CONFIG_HOME/a38"
    fi
    (umask 077 && cp "$new_config" "$CONFIG")
  fi

  echo
  echo "demo: installed"
  echo "  a38:           $A38"
  echo "  app:           $APP_BUNDLE (ad-hoc signed, signature verified)"
  echo "  project:       $PROJECT_DIR (.mcp.json, .claude/skills/a38/SKILL.md, .claude/settings.json with the hook op-guard)"
  case "$config_action" in
    write) echo "  configuration: $CONFIG (new)" ;;
    keep) echo "  configuration: $CONFIG (unchanged, same content)" ;;
    backup) echo "  configuration: $CONFIG (new; the old file is $backup)" ;;
  esac
  echo "  list:          $INSTALLED"
  echo "Kept as is: the token file and the state directory $STATE_DIR."
  echo "Next step: mise run demo:preflight"
}

cmd_setup_fake() {
  if [ -e "$DEMO_DIR" ]; then
    [ -f "$MARKER" ] || die "$DEMO_DIR exists and setup did not create it; choose another DEMO_DIR"
    if daemon_answers; then die "the rehearsal daemon runs; stop it (Ctrl-C) before a new setup"; fi
  fi
  build
  local kept_settings
  kept_settings=$(mktemp "${TMPDIR:-/tmp}/a38-demo-settings.XXXXXX")
  if [ -e "$DEMO_DIR" ]; then
    keep_settings "$kept_settings"
    chmod -R u+w "$DEMO_DIR"
    rm -rf "$DEMO_DIR"
  fi
  umask 077
  mkdir -p "$DEMO_DIR/bin" "$AUDIT_DIR"
  printf 'created=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >"$MARKER"
  cp "$ROOT/bin/a38" "$A38"
  install_app
  cp -R "$ROOT/examples/demo/demo-projekt" "$PROJECT_DIR"
  chmod 700 "$PROJECT_DIR"/*.sh
  restore_settings "$kept_settings"
  cp "$ROOT/examples/demo/fake.yaml" "$DEMO_DIR/fake.yaml"
  cat >"$DEMO_DIR/passierschein-debug" <<EOF
#!/bin/sh
# Debug build of the approval app with fake authentication (never Touch ID).
exec "$ROOT/app/.build/debug/Passierschein" --skip-auth --auto-close 60
EOF
  chmod 700 "$DEMO_DIR/passierschein-debug"
  write_config "$(cd "$PROJECT_DIR" && pwd -P)" "$CONFIG"
  write_project
  echo
  echo "demo: rehearsal ready in $DEMO_DIR (fake backend; nothing installed outside)"
  echo "  config:   $CONFIG"
  echo "  project:  $PROJECT_DIR (.mcp.json, .claude/skills/a38/SKILL.md, .claude/settings.json with the hook op-guard)"
  echo "Next step: DEMO_BACKEND=fake mise run demo:start"
}

pass() { printf 'PASS  %s\n' "$*"; }
fail() {
  printf 'FAIL  %s\n' "$*"
  failed=1
}

# preflight sets the array SERVE_ARGS and fails on a FAIL line.
preflight() {
  need_setup
  failed=0
  echo "demo: preflight ($MODE mode)"
  if [ -x "$A38" ]; then pass "a38 $A38"; else fail "a38 $A38 is missing; run setup"; fi

  local ui=app
  if [ "$MODE" = real ]; then
    SERVE_ARGS=(--backend 1password --ui app)
    local on_path
    on_path=$(command -v a38 || true)
    if [ "$on_path" = "$A38" ]; then
      pass "a38 on the PATH is $A38"
    else
      fail "a38 on the PATH is '${on_path:-none}', not $A38; put ~/.local/bin first in the PATH"
    fi
    if [ -z "${A38_CONFIG:-}" ] || [ "$A38_CONFIG" = "$CONFIG" ]; then
      pass "A38_CONFIG is not set: a38 uses $CONFIG"
    else
      fail "A38_CONFIG is set to $A38_CONFIG; unset it, so that a38 uses $CONFIG"
    fi
  else
    SERVE_ARGS=(--config "$CONFIG" --backend fake --fake-file "$DEMO_DIR/fake.yaml")
    ui=${DEMO_UI:-auto-approve}
    case "$ui" in
      auto-approve | auto-deny) SERVE_ARGS+=(--ui "$ui") ;;
      debug-app)
        if [ -x "$ROOT/app/.build/debug/Passierschein" ]; then
          pass "debug app with fake authentication (no Touch ID; an approval ends with ui_failed)"
        else
          fail "debug app is missing; run: mise run app:build"
        fi
        SERVE_ARGS+=(--ui app --ui-app "$DEMO_DIR/passierschein-debug")
        ;;
      *) die "DEMO_UI must be auto-approve, auto-deny or debug-app" ;;
    esac
  fi

  if [ "$ui" = app ]; then
    if [ ! -x "$APP_EXE" ]; then
      fail "approval app $APP_EXE is missing; run setup"
    elif ! codesign --verify --strict "$APP_BUNDLE" 2>/dev/null; then
      fail "the signature of $APP_BUNDLE is not valid; run setup"
    elif [ "$(stat -f '%Su' "$APP_EXE")" != "$(id -un)" ] || [ -n "$(find "$APP_EXE" -perm -g+w -o -perm -o+w)" ]; then
      fail "$APP_EXE must belong to $(id -un) and be writable only by the owner"
    else
      pass "approval app $APP_EXE (signature valid, owner $(id -un), not writable by others)"
    fi
  fi

  if [ "$MODE" = real ]; then
    # Metadata only. The script never reads the content.
    local mode owner
    if [ ! -e "$TOKEN_FILE" ]; then
      fail "token file $TOKEN_FILE is missing (docs/how-to/30-1password-backend-einrichten.md)"
    elif [ ! -f "$TOKEN_FILE" ] || [ -L "$TOKEN_FILE" ]; then
      fail "token file $TOKEN_FILE is not a regular file"
    else
      mode=$(stat -f '%Lp' "$TOKEN_FILE")
      owner=$(stat -f '%Su' "$TOKEN_FILE")
      if [ "$mode" = 600 ] && [ "$owner" = "$(id -un)" ]; then
        pass "token file $TOKEN_FILE (mode 0600, owner $owner; content not read)"
      else
        fail "token file $TOKEN_FILE has mode 0$mode and owner $owner; need 0600 and $(id -un)"
      fi
    fi
  fi

  local doctor
  doctor=$("$A38" doctor --config "$CONFIG" 2>&1 || true)
  if grep -q '^OK *configuration' <<<"$doctor"; then
    pass "configuration $CONFIG loads"
  else
    fail "configuration $CONFIG does not load:"
    grep -A2 'configuration' <<<"$doctor" | sed 's/^/        /' || true
  fi

  if daemon_answers; then
    fail "a daemon runs already with $CONFIG; use that terminal or stop it with Ctrl-C"
  else
    pass "no daemon runs with $CONFIG; the socket directory $SOCKET_DIR is free"
  fi
  if [ -f "$FAULT_MODE_FILE" ]; then
    fail "the fault act is still active; run: mise run demo:fault:fix"
  elif [ -d "$AUDIT_DIR" ] && [ ! -w "$AUDIT_DIR" ]; then
    fail "the audit directory $AUDIT_DIR is not writable"
  else
    pass "audit directory $AUDIT_DIR writable (or created at the start)"
  fi
  if [ "$failed" != 0 ]; then
    die "preflight failed"
  fi
}

cmd_preflight() {
  preflight
  echo "demo: preflight passed; nothing was started"
}

cmd_start() {
  preflight
  if [ "$MODE" = real ]; then
    cat <<EOF

Terminal 2 (agent):   cd $(tilde "$PROJECT_DIR") && claude
Terminal 3 (human):   a38 doctor | a38 grants list | a38 audit list | a38 audit verify | a38 fault status
Without an agent:     mise run demo:request -- akt2   (acts akt1 ... akt7)

Stop the daemon with Ctrl-C. The daemon log follows.

\$ a38 serve ${SERVE_ARGS[*]}
EOF
  else
    cat <<EOF

Terminal 2 (agent):   cd $PROJECT_DIR && claude
Terminal 3 (human):   export PATH="$DEMO_DIR/bin:\$PATH" A38_CONFIG=$CONFIG
                      then: a38 doctor | a38 grants list | a38 audit list
Without an agent:     DEMO_BACKEND=fake mise run demo:request -- akt2

Stop the daemon with Ctrl-C. The daemon log follows.

EOF
    echo $$ >"$PID_FILE"
  fi
  exec "$A38" serve "${SERVE_ARGS[@]}"
}

cmd_status() {
  need_setup
  local c
  for c in "doctor" "grants list" "fault status" "audit list --limit 12" "audit verify"; do
    echo "\$ a38 $c"
    # shellcheck disable=SC2086 # split the subcommand on purpose
    "$A38" $c --config "$CONFIG" || true
    echo
  done
}

# check_audit_dir refuses to change a directory that is not the audit
# directory of the demo configuration.
check_audit_dir() {
  local audit_file
  audit_file=$("$A38" doctor --config "$CONFIG" --json 2>/dev/null | sed -nE 's#.*"detail": *"([^"]*audit\.db) exists.*#\1#p' | head -n 1 || true)
  [ -d "$AUDIT_DIR" ] || die "$AUDIT_DIR does not exist; start the daemon first"
  [ -f "$AUDIT_DIR/audit.db" ] || die "$AUDIT_DIR has no audit.db; it is not the audit directory of a38"
  if [ -n "$audit_file" ] && [ "$(dirname "$audit_file")" != "$AUDIT_DIR" ]; then
    die "the configuration uses the audit file $audit_file, not $AUDIT_DIR/audit.db; the fault act stops"
  fi
}

cmd_fault() {
  need_setup
  check_audit_dir
  [ ! -f "$FAULT_MODE_FILE" ] || die "the fault act is already active; repair: mise run demo:fault:fix"
  # Same method as the process test TestServeGlobalLock (cmd/a38): without
  # write permission on its directory, SQLite cannot create its journal,
  # so each write of the audit log fails; reading still works.
  stat -f '%Lp' "$AUDIT_DIR" >"$FAULT_MODE_FILE"
  chmod 500 "$AUDIT_DIR"
  echo "demo: the audit directory $AUDIT_DIR is now read-only (mode 0500; was 0$(cat "$FAULT_MODE_FILE"))."
  echo "The next request ends with audit_failed and the daemon locks delivery for all sessions."
  echo "Repair: mise run demo:fault:fix"
}

cmd_fault_fix() {
  need_setup
  local mode=700
  if [ -f "$FAULT_MODE_FILE" ]; then
    mode=$(cat "$FAULT_MODE_FILE")
    [[ "$mode" =~ ^[0-7]{3,4}$ ]] || mode=700
  fi
  [ -d "$AUDIT_DIR" ] || die "$AUDIT_DIR does not exist"
  chmod "$mode" "$AUDIT_DIR"
  rm -f "$FAULT_MODE_FILE"
  echo "demo: the audit directory $AUDIT_DIR has its mode 0$mode again and is writable."
  echo "The lock stays until you clear it. Next step:"
  if [ "$MODE" = real ]; then
    echo "  a38 fault clear"
  else
    echo "  $A38 fault clear --config $CONFIG"
  fi
}

# mask_and_show prints one answer of a38 mcp with each value replaced.
mask_and_show() {
  jq -c '
    def mask: walk(if type == "object" and has("value") then .value = "[redacted]" else . end);
    mask
    | if (.result.content? | type) == "array" then
        .result.content |= map(if .type == "text" then .text |= (try (fromjson | mask | tojson) catch .) else . end)
      else . end
    | if .result.serverInfo? then {id, server: .result.serverInfo}
      elif .result.tools? then {id, tools: [.result.tools[].name]}
      elif .result.structuredContent? then {id, answer: .result.structuredContent, isError: (.result.isError // false)}
      elif .result.content? then {id, answer: [.result.content[].text], isError: (.result.isError // false)}
      else . end'
}

cmd_request() {
  need_setup
  local act=${1:-}
  local file="$ROOT/examples/demo/requests/$act.jsonl"
  [ -n "$act" ] && [ -f "$file" ] || die "usage: mise run demo:request -- <act>; acts: akt1 ... akt7"
  command -v jq >/dev/null || die "jq is missing; run through mise run demo:request"
  daemon_answers || echo "demo: warning: no daemon answers (mise run demo:start)" >&2

  local tmp
  tmp=$(mktemp -d /tmp/a38-demo-req.XXXXXX)
  # The raw answers hold values; they stay in a 0700 directory and go away.
  chmod 700 "$tmp"
  local mcp_pid=""
  # shellcheck disable=SC2064 # expand tmp now
  trap "exec 3>&- 2>/dev/null || true; rm -rf '$tmp'" EXIT
  mkfifo "$tmp/in"
  (cd "$PROJECT_DIR" && exec "$A38" mcp --config "$CONFIG" --session-name "$HAND_SESSION_NAME" <"$tmp/in" >"$tmp/out" 2>"$tmp/err") &
  mcp_pid=$!
  exec 3>"$tmp/in"
  echo "demo: act $act, session $HAND_SESSION_NAME, project directory $PROJECT_DIR"

  local line id shown=0 n i
  while IFS= read -r line <&4 || [ -n "$line" ]; do
    case "$line" in
      "") continue ;;
      "# wait "*)
        echo
        echo ">>> ${line#\# wait }"
        read -r _ || true
        continue
        ;;
      "#"*) continue ;;
    esac
    echo "$line" >&3
    id=$(jq -r '.id // empty' <<<"$line")
    [ -n "$id" ] || continue
    echo "--> $(jq -c '{id, method, tool: .params.name?, references: .params.arguments.references?} | with_entries(select(.value != null))' <<<"$line")"
    for ((i = 0; i < 1800; i++)); do
      grep -q "\"id\":${id}[,}]" "$tmp/out" 2>/dev/null && break
      kill -0 "$mcp_pid" 2>/dev/null || break
      [ "$i" = 20 ] && echo "    (waiting for the answer; the popup may be open)"
      sleep 0.1
    done
    n=$(wc -l <"$tmp/out")
    if [ "$n" -gt "$shown" ]; then
      tail -n +"$((shown + 1))" "$tmp/out" | while IFS= read -r ans; do
        echo "<-- $(mask_and_show <<<"$ans")"
      done
      shown=$n
    fi
    if ! kill -0 "$mcp_pid" 2>/dev/null; then
      echo "demo: a38 mcp ended:" >&2
      cat "$tmp/err" >&2
      break
    fi
  done 4<"$file"
  exec 3>&-
  wait "$mcp_pid" 2>/dev/null || true
  echo "demo: session ended"
}

cmd_teardown_real() {
  if [ ! -f "$INSTALLED" ]; then
    echo "demo: nothing to remove (no $INSTALLED)"
    return 0
  fi
  if daemon_answers; then die "a daemon runs with $CONFIG; stop it (Ctrl-C) before teardown"; fi
  if [ -f "$FAULT_MODE_FILE" ]; then
    chmod "$(cat "$FAULT_MODE_FILE")" "$AUDIT_DIR" 2>/dev/null || chmod 700 "$AUDIT_DIR"
    rm -f "$FAULT_MODE_FILE"
  fi
  local item
  while IFS= read -r item; do
    case "$item" in
      "$A38") rm -f "$item" && echo "demo: removed $item" ;;
      "$APP_BUNDLE") rm -rf "$item" && echo "demo: removed $item" ;;
      "$PROJECT_DIR")
        if [ -f "$PROJECT_MARKER" ]; then
          rm -rf "$item" && echo "demo: removed $item"
        fi
        ;;
    esac
  done <"$INSTALLED"
  rm -rf "$KIT_DIR"
  if [ "${DEMO_PURGE:-}" = 1 ]; then
    if [ -f "$CONFIG" ]; then
      local backup
      backup="$CONFIG.bak-$(date +%Y%m%d-%H%M%S)"
      mv "$CONFIG" "$backup"
      echo "demo: moved the configuration to $backup"
    fi
    if [ -d "$STATE_DIR" ]; then
      chmod -R u+w "$STATE_DIR"
      rm -rf "$STATE_DIR"
      echo "demo: removed the state directory $STATE_DIR (audit log)"
    fi
  else
    echo "demo: kept $CONFIG and the audit log in $STATE_DIR (DEMO_PURGE=1 removes them)"
  fi
  echo "demo: the token file stays untouched."
}

cmd_teardown_fake() {
  if [ ! -e "$DEMO_DIR" ]; then
    echo "demo: nothing to remove ($DEMO_DIR does not exist)"
    return 0
  fi
  [ -f "$MARKER" ] || die "$DEMO_DIR has no demo marker; setup did not create it, so teardown does not remove it"
  local pid=""
  [ -f "$PID_FILE" ] && pid=$(cat "$PID_FILE")
  if [[ "$pid" =~ ^[0-9]+$ ]] && ps -p "$pid" -o command= 2>/dev/null | grep -qF "serve --config $CONFIG"; then
    echo "demo: stop the rehearsal daemon (PID $pid)"
    kill -TERM "$pid"
    for _ in 1 2 3 4 5 6 7 8 9 10; do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.5
    done
    if kill -0 "$pid" 2>/dev/null; then die "the daemon (PID $pid) does not stop; stop it with Ctrl-C"; fi
  fi
  chmod -R u+w "$DEMO_DIR"
  rm -rf "$DEMO_DIR"
  echo "demo: removed $DEMO_DIR"
}

case "${1:-}" in
  setup) if [ "$MODE" = real ]; then cmd_setup_real; else cmd_setup_fake; fi ;;
  preflight) cmd_preflight ;;
  start) cmd_start ;;
  status) cmd_status ;;
  fault) cmd_fault ;;
  fault-fix) cmd_fault_fix ;;
  request) shift && cmd_request "$@" ;;
  teardown) if [ "$MODE" = real ]; then cmd_teardown_real; else cmd_teardown_fake; fi ;;
  *) die "usage: demo.sh setup|preflight|start|status|fault|fault-fix|request <act>|teardown" ;;
esac
