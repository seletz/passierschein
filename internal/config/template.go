package config

import (
	"fmt"
	"strconv"
)

// Template returns the content of a new configuration file: each value
// has its default, and short comments say what it does. The token source
// is written out. The file loads without an error; it names no vault and
// no project yet.
func Template() ([]byte, error) {
	socketDir, err := DefaultSocketDir()
	if err != nil {
		return nil, err
	}
	auditFile, err := DefaultAuditFile()
	if err != nil {
		return nil, err
	}
	tokenFile, err := DefaultTokenFile()
	if err != nil {
		return nil, err
	}
	text := fmt.Sprintf(`# Configuration of a38. Written by "a38 config init".
# Reference: docs/reference/80-befehle-fuer-den-menschen.md and the
# package documentation of internal/config.
# The daemon reads this file once at its start. After a change, restart it.
version: %d

# The vaults that a38 can see. A vault that is not in this list does not
# exist for a38. Only a vault of the environment dev delivers values today.
# Add one with: a38 config vault add NAME --environment dev
vaults: []

# The projects. A session belongs to a project through the working
# directory of the agent. A project can use only the vaults it names.
# Add one with: a38 config project add NAME --path DIR --vault VAULT
projects: []

# The delivery ways, the lifetime of a grant and the binding of a grant for
# each environment. The human can choose a shorter lifetime in the popup,
# never a longer one. The fixed rules: mcp only for dev; prod only with the
# lifetime 0s and the binding command; test never with the binding none.
# Binding: none (any command), program (same program), command (same
# program, arguments and working directory).
environments:
  dev:
    delivery: [mcp, exec]
    grant_lifetime: 4h
    binding: none
  test:
    delivery: [exec]
    grant_lifetime: 30m
    binding: program
  prod:
    # prod is off. Set "delivery: [exec]" to allow it. Each command in
    # prod needs its own approval.
    delivery: []
    grant_lifetime: 0s
    binding: command

# The limits of the exec way. output_limit is for each stream.
exec:
  default_timeout: 2m
  max_timeout: 10m
  output_limit: 64KiB

# Requests of one session within the window go into one popup.
bundle:
  window: 3s
  max_entries: %d

# The rules for the name of a session.
session_name:
  max_length: %d
  allowed_characters: %s

# The sockets and the audit log. The defaults follow XDG_STATE_HOME.
paths:
  socket_dir: %s
  audit_file: %s
  # The approval app. Without it, "a38 serve" searches next to its own binary.
  # ui_app: /path/to/Passierschein.app/Contents/MacOS/Passierschein

# The 1Password backend reads the token of the service account from the
# macOS keychain (store it with: a38 config onepassword-token set). For
# development and CI, set token_source to file: the daemon then reads
# token_file (mode 0600).
backend:
  onepassword:
    token_source: %s
    # token_file: %s
`, FileVersion, DefaultBundleMaxEntries, DefaultSessionNameMaxLength,
		strconv.Quote(DefaultSessionNameCharacters), TildePath(socketDir), TildePath(auditFile),
		DefaultTokenSource, TildePath(tokenFile))
	return []byte(text), nil
}
