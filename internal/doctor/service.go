package doctor

import (
	"context"
	"fmt"
)

// ServiceState is what the doctor knows about the LaunchAgent of the
// daemon (F15, issue 70). The package doctor does not import the package
// service, so that the linux build stays without launchd code; the
// command fills this type.
type ServiceState struct {
	// PlistPath is the path of the plist.
	PlistPath string
	// Installed is true if the plist exists.
	Installed bool
	// Foreign is true if a38 service install did not write the plist.
	Foreign bool
	// Program is the program in the plist.
	Program string
	// ProgramMissing is true if that program does not exist.
	ProgramMissing bool
	// Current is the real path of the running a38.
	Current string
	// ProgramDiffers is true if the plist starts another a38.
	ProgramDiffers bool
	// Loaded is true if launchd knows the job.
	Loaded bool
	// Disabled is true if the job is disabled.
	Disabled bool
	// Running is true if the daemon of the job runs.
	Running bool
	// PID is the process id of the daemon.
	PID int
	// LastExit explains the last exit; empty if unknown.
	LastExit string
	// LogPath is the log file of the daemon.
	LogPath string
	// Error says why launchd did not answer.
	Error string
}

// ServiceEnv is an [Env] that knows the LaunchAgent. Only the macOS build
// of a38 fills it; on other platforms the check says nothing.
type ServiceEnv interface {
	// Service returns the state of the LaunchAgent; nil if the platform
	// has none.
	Service(ctx context.Context) *ServiceState
}

// Service implements [ServiceEnv].
func (e SystemEnv) Service(ctx context.Context) *ServiceState {
	if e.ServiceState == nil {
		return nil
	}
	return e.ServiceState(ctx)
}

// serviceCheck reports the LaunchAgent. A missing service is no finding:
// the human may start the daemon by hand.
type serviceCheck struct{ env Env }

func (serviceCheck) Name() string { return "service" }

func (c serviceCheck) Run(ctx context.Context, _ *State) []Result {
	se, okEnv := c.env.(ServiceEnv)
	if !okEnv {
		return nil
	}
	s := se.Service(ctx)
	if s == nil {
		return nil
	}
	name := c.Name()
	switch {
	case !s.Installed && !s.Loaded:
		return []Result{ok(name, "not installed; the daemon starts only by hand. a38 service install starts it with each login")}
	case s.Foreign:
		return []Result{finding(name, "a plist with the label of a38 exists, but a38 service install did not write it",
			s.PlistPath, "check the file; a38 service install --force replaces it, a38 service uninstall --force removes it")}
	case s.ProgramMissing:
		return []Result{finding(name, "the LaunchAgent starts a program that does not exist",
			fmt.Sprintf("%s does not exist, for example after an update moved a38", s.Program),
			"run a38 service install with the a38 that the service shall start")}
	case s.ProgramDiffers:
		return []Result{finding(name, "the LaunchAgent starts another a38 than this one",
			fmt.Sprintf("the plist starts %s, this command is %s", s.Program, s.Current),
			"if this a38 is the right one, run a38 service install; it writes the plist again")}
	case s.Error != "":
		return []Result{finding(name, "launchd does not answer", s.Error, "run a38 service status in a terminal on the desktop")}
	case s.Disabled:
		return []Result{finding(name, "the LaunchAgent is disabled",
			"launchd does not start it; System Settings > General > Login Items may have switched it off",
			"allow Passierschein (or a38) in System Settings > General > Login Items, then run a38 service install")}
	case !s.Loaded:
		return []Result{finding(name, "the LaunchAgent is installed but not loaded",
			s.PlistPath+" exists; launchd loads it at the next login",
			"run a38 service restart, or log out and in")}
	case !s.Running:
		cause := "the daemon of the LaunchAgent does not run"
		if s.LastExit != "" {
			cause += "; last exit: " + s.LastExit
		}
		return []Result{finding(name, "the LaunchAgent is installed, but the daemon does not run", cause,
			"read the log "+s.LogPath+", fix the cause, then run a38 service restart")}
	}
	return []Result{ok(name, "installed, running with pid %d, log %s", s.PID, s.LogPath)}
}
