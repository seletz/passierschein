// Package service installs "a38 serve" as a LaunchAgent of the user, so
// that the daemon starts with the login (F15, UC11, D42, D53).
//
// The package writes the property list (plist) of the LaunchAgent to
// ~/Library/LaunchAgents and controls the job with /bin/launchctl. Each
// call of launchctl goes through the [Launchctl] interface, so that a test
// uses a fake and never touches the real launchd. Only the darwin build
// has a real [Launchctl]; on other platforms [NewLaunchctl] returns an
// error ("not available on this platform", D53).
//
// The policy of the job (decision of issue 70, an assumption for Stefan to
// confirm):
//
//   - RunAtLoad true, KeepAlive false: launchd starts the daemon at the
//     login and never restarts it by itself. A restart lifts the global
//     delivery lock (D48), and only the human may do that. A start error
//     (configuration, token source, socket directory) stays until the
//     human fixes it, so a restart loop helps nothing; with the keychain
//     each start can show a macOS dialog, and a loop of dialogs teaches
//     the human to confirm them (D51). The human restarts the daemon with
//     "a38 service restart".
//   - LimitLoadToSessionType Aqua: the daemon runs only in the GUI session
//     of the user. It needs the session for the approval popup and for the
//     login keychain.
//   - ProcessType Interactive: the approval app is a child process of the
//     daemon and shares its resource limits; a human waits for it.
//   - The log goes to ~/Library/Logs/a38/serve.log (stdout and stderr).
//     launchd does not rotate it; the daemon writes no secret value.
//   - EnvironmentVariables holds only XDG_CONFIG_HOME and XDG_STATE_HOME
//     of the installing shell, never a secret, so that the daemon uses the
//     same default paths as "a38 mcp" in that shell.
package service

import (
	"errors"
	"fmt"
	"path/filepath"
)

// Label is the label of the LaunchAgent. It starts with the bundle
// identifier of the approval app and names the role of the job.
const Label = BundleID + ".daemon"

// BundleID is the bundle identifier of Passierschein.app. System Settings
// shows the login item under the name of this app (key
// AssociatedBundleIdentifiers, macOS 13 and later).
const BundleID = "io.github.seletz.passierschein"

// LaunchctlPath is the only program that this package starts.
const LaunchctlPath = "/bin/launchctl"

// Marker is the comment in each plist that "a38 service install" writes.
// The package changes or removes a plist only if it holds the marker,
// unless the human gives --force.
const Marker = "Written by a38 service install. a38 service uninstall removes it. Do not edit; a38 service install writes it again."

// LogName is the name of the log file of the daemon.
const LogName = "serve.log"

// ErrUnavailable: the platform has no launchd (D53).
var ErrUnavailable = errors.New("a38 service is not available on this platform; the daemon runs on macOS only")

// Layout names the files and the launchd targets of the service of one
// user.
type Layout struct {
	// Home is the home directory of the user.
	Home string
	// UID is the user id; launchd names the GUI domain after it.
	UID int
}

// PlistPath returns ~/Library/LaunchAgents/<Label>.plist.
func (l Layout) PlistPath() string {
	return filepath.Join(l.Home, "Library", "LaunchAgents", Label+".plist")
}

// LogDir returns ~/Library/Logs/a38.
func (l Layout) LogDir() string { return filepath.Join(l.Home, "Library", "Logs", "a38") }

// LogPath returns the log file of the daemon.
func (l Layout) LogPath() string { return filepath.Join(l.LogDir(), LogName) }

// Domain returns the launchd domain of the GUI session: gui/<uid>.
func (l Layout) Domain() string { return fmt.Sprintf("gui/%d", l.UID) }

// Target returns the service target: gui/<uid>/<Label>.
func (l Layout) Target() string { return l.Domain() + "/" + Label }
