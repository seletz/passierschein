package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Manager installs, removes, restarts and describes the LaunchAgent.
type Manager struct {
	// Layout names the files and the launchd targets.
	Layout Layout
	// Launchctl runs /bin/launchctl.
	Launchctl Launchctl
	// Self is the absolute real path of the running a38 (see
	// [ResolveProgram]).
	Self string
}

// ErrRefused wraps each refusal of [Manager.Install] and
// [Manager.Uninstall]; the command ends with the exit code 1.
var ErrRefused = errors.New("refused")

// InstallResult says what [Manager.Install] did.
type InstallResult struct {
	// Plist is the content of the plist.
	Plist []byte
	// PlistPath is the path of the plist.
	PlistPath string
	// Written is true if the file changed.
	Written bool
	// Loaded is true if launchd loaded the job in this call.
	Loaded bool
	// Unchanged is true if the job was installed and loaded with the same
	// plist already; nothing changed.
	Unchanged bool
	// Warnings do not stop the install.
	Warnings []string
	// Preflight is the outcome of the checks.
	Preflight Preflight
}

// Spec returns the plist for the options.
func (m Manager) Spec(o Options) Spec {
	env := map[string]string{}
	for k, v := range o.Env {
		if v != "" {
			env[k] = v
		}
	}
	return Spec{Program: m.Self, Args: o.ServeArgs(), LogPath: m.Layout.LogPath(), Env: env}
}

// Install checks the program and the preflight, writes the plist and loads
// the job: launchctl enable, then bootstrap (a disabled service cannot be
// loaded). A second install with the same options changes nothing. A plist
// that a38 did not write stays, unless o.Force is set.
func (m Manager) Install(ctx context.Context, o Options, probe Probe) (InstallResult, error) {
	r := InstallResult{PlistPath: m.Layout.PlistPath()}
	if err := o.Validate(); err != nil {
		return r, err
	}
	if err := CheckProgram(m.Self, probe.UID); err != nil {
		return r, fmt.Errorf("%w: %v; launchd would start this file at each login. Install a38 from the release (how-to: Daemon als Dienst einrichten)", ErrRefused, err)
	}
	r.Warnings = append(r.Warnings, ProgramWarnings(m.Self)...)
	if !InBundle(m.Self) {
		r.Warnings = append(r.Warnings, "a38 does not run from Passierschein.app; System Settings may show the login item as a38 and not as Passierschein")
	}
	if os.Getenv("SSH_CONNECTION") != "" {
		r.Warnings = append(r.Warnings, "this is an SSH session; launchd loads the service only into the desktop session of the user. Run the command in a terminal on the desktop if it fails")
	}
	r.Plist = m.Spec(o).Render()

	job, err := m.job(ctx)
	if err != nil {
		return r, err
	}
	r.Preflight = probe.Run(ctx, o, job.Running())
	r.Warnings = append(r.Warnings, r.Preflight.Warnings...)
	if len(r.Preflight.Findings) > 0 {
		return r, fmt.Errorf("%w: the daemon would not start; fix the findings first", ErrRefused)
	}

	old, ours, exists, err := m.readPlist()
	if err != nil {
		return r, err
	}
	if exists && !ours && !o.Force {
		return r, fmt.Errorf("%w: %s exists and a38 service install did not write it; check the file, then use --force to replace it", ErrRefused, r.PlistPath)
	}
	same := exists && ours && bytes.Equal(old, r.Plist)
	if o.DryRun {
		return r, nil
	}
	if same && job.Loaded {
		r.Unchanged = true
		return r, nil
	}
	if err := os.MkdirAll(m.Layout.LogDir(), 0o700); err != nil {
		return r, fmt.Errorf("cannot create the log directory: %w", err)
	}
	if err := touch(m.Layout.LogPath()); err != nil {
		return r, fmt.Errorf("cannot create the log file: %w", err)
	}
	if !same {
		if job.Loaded {
			// The old job runs with the old plist: unload it first. The
			// daemon stops with SIGTERM and ends its grants (T16).
			if err := m.bootout(ctx); err != nil {
				return r, err
			}
		}
		if err := writeAtomic(r.PlistPath, r.Plist); err != nil {
			return r, fmt.Errorf("cannot write %s: %w", r.PlistPath, err)
		}
		r.Written = true
	}
	if err := m.load(ctx); err != nil {
		return r, err
	}
	r.Loaded = true
	return r, nil
}

// load enables the service and bootstraps the plist into the GUI domain.
func (m Manager) load(ctx context.Context) error {
	if res, err := m.Launchctl.Run(ctx, "enable", m.Layout.Target()); err != nil {
		return err
	} else if res.Code != 0 {
		return fmt.Errorf("launchctl enable %s failed (exit code %d): %s", m.Layout.Target(), res.Code, res.Text())
	}
	res, err := m.Launchctl.Run(ctx, "bootstrap", m.Layout.Domain(), m.Layout.PlistPath())
	if err != nil {
		return err
	}
	if res.Code != 0 {
		return fmt.Errorf("launchctl bootstrap %s failed (exit code %d): %s; the plist stays in place and loads at the next login. Check that you run the command in a terminal on the desktop and that System Settings allows the login item", m.Layout.Domain(), res.Code, res.Text())
	}
	return nil
}

// bootout unloads the job. An unknown job is no error.
func (m Manager) bootout(ctx context.Context) error {
	res, err := m.Launchctl.Run(ctx, "bootout", m.Layout.Target())
	if err != nil {
		return err
	}
	if res.Code != 0 && res.Code != notFoundCode {
		return fmt.Errorf("launchctl bootout %s failed (exit code %d): %s", m.Layout.Target(), res.Code, res.Text())
	}
	return nil
}

// job asks launchd for the job. An unknown job gives Loaded false.
func (m Manager) job(ctx context.Context) (Job, error) {
	res, err := m.Launchctl.Run(ctx, "print", m.Layout.Target())
	if err != nil {
		return Job{}, err
	}
	if res.Code != 0 {
		return Job{}, nil
	}
	return ParsePrint(res.Stdout), nil
}

// readPlist reads the plist. ours is true if it holds the marker. A link
// or a directory at the path is not ours.
func (m Manager) readPlist() (data []byte, ours, exists bool, err error) {
	path := m.Layout.PlistPath()
	fi, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, false, nil
	}
	if err != nil {
		return nil, false, false, err
	}
	if !fi.Mode().IsRegular() {
		return nil, false, true, nil
	}
	data, err = os.ReadFile(path)
	if err != nil {
		return nil, false, true, err
	}
	return data, HasMarker(data), true, nil
}

// UninstallResult says what [Manager.Uninstall] did.
type UninstallResult struct {
	// WasInstalled is false if neither the plist nor the job existed.
	WasInstalled bool
	// Unloaded is true if launchd knew the job.
	Unloaded bool
	// Removed is true if the plist was removed.
	Removed bool
}

// Uninstall unloads the job and removes the plist. It keeps the log. A
// plist that a38 did not write stays, unless force is set.
func (m Manager) Uninstall(ctx context.Context, force bool) (UninstallResult, error) {
	var r UninstallResult
	_, ours, exists, err := m.readPlist()
	if err != nil {
		return r, err
	}
	job, err := m.job(ctx)
	if err != nil {
		return r, err
	}
	if !exists && !job.Loaded {
		return r, nil
	}
	r.WasInstalled = true
	if exists && !ours && !force {
		return r, fmt.Errorf("%w: %s exists and a38 service install did not write it; check the file, then use --force to remove it", ErrRefused, m.Layout.PlistPath())
	}
	if job.Loaded {
		if err := m.bootout(ctx); err != nil {
			return r, err
		}
		r.Unloaded = true
	}
	if exists {
		if err := os.Remove(m.Layout.PlistPath()); err != nil {
			return r, fmt.Errorf("cannot remove %s: %w", m.Layout.PlistPath(), err)
		}
		r.Removed = true
	}
	return r, nil
}

// Restart stops and starts the daemon: launchctl kickstart -k. If the job
// is not loaded, Restart loads it. The daemon reads the configuration and
// the token again (D49, D51); the restart ends all grants (D33) and lifts
// the global lock if its cause is gone (D48).
func (m Manager) Restart(ctx context.Context) error {
	_, ours, exists, err := m.readPlist()
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: the service is not installed; run a38 service install", ErrRefused)
	}
	if !ours {
		return fmt.Errorf("%w: %s exists and a38 service install did not write it", ErrRefused, m.Layout.PlistPath())
	}
	job, err := m.job(ctx)
	if err != nil {
		return err
	}
	if !job.Loaded {
		return m.load(ctx)
	}
	res, err := m.Launchctl.Run(ctx, "kickstart", "-k", m.Layout.Target())
	if err != nil {
		return err
	}
	if res.Code != 0 {
		return fmt.Errorf("launchctl kickstart -k %s failed (exit code %d): %s", m.Layout.Target(), res.Code, res.Text())
	}
	return nil
}

// Status describes the service. It changes nothing.
type Status struct {
	// PlistPath is the path of the plist.
	PlistPath string `json:"plist_path"`
	// Installed is true if the plist exists.
	Installed bool `json:"installed"`
	// Foreign is true if the plist exists and a38 did not write it.
	Foreign bool `json:"foreign,omitempty"`
	// PlistError says why the plist cannot be read.
	PlistError string `json:"plist_error,omitempty"`
	// Program is the program in the plist.
	Program string `json:"program,omitempty"`
	// Arguments are the arguments after the program in the plist.
	Arguments []string `json:"arguments,omitempty"`
	// ConfigPath is the value of --config in the plist; empty for the
	// default.
	ConfigPath string `json:"config,omitempty"`
	// ProgramMissing is true if the program of the plist does not exist.
	ProgramMissing bool `json:"program_missing,omitempty"`
	// Current is the real path of the running a38.
	Current string `json:"current"`
	// ProgramDiffers is true if the plist starts another a38 than the
	// running one, for example after an update moved the binary.
	ProgramDiffers bool `json:"program_differs,omitempty"`
	// Loaded is true if launchd knows the job.
	Loaded bool `json:"loaded"`
	// Disabled is true if launchctl print-disabled lists the job as
	// disabled.
	Disabled bool `json:"disabled,omitempty"`
	// Running is true if the job runs.
	Running bool `json:"running"`
	// PID is the process id of the daemon; 0 if it does not run.
	PID int `json:"pid,omitempty"`
	// State is the state that launchd reports.
	State string `json:"state,omitempty"`
	// LastExit is the last exit; nil if unknown or none.
	LastExit *Exit `json:"last_exit,omitempty"`
	// LastExitMeaning explains LastExit.
	LastExitMeaning string `json:"last_exit_meaning,omitempty"`
	// LogPath is the log file of the daemon.
	LogPath string `json:"log_path"`
	// LogSize is the size of the log file in bytes; -1 if it is missing.
	LogSize int64 `json:"log_size"`
	// LaunchctlError says why launchctl did not answer.
	LaunchctlError string `json:"launchctl_error,omitempty"`
}

// OK reports whether the service is installed by a38, runs and starts the
// running a38.
func (s Status) OK() bool {
	return s.Installed && !s.Foreign && s.Running && !s.ProgramMissing && !s.ProgramDiffers
}

// Status reads the plist and asks launchd. It never changes a file or a
// job.
func (m Manager) Status(ctx context.Context) Status {
	s := Status{PlistPath: m.Layout.PlistPath(), Current: m.Self, LogPath: m.Layout.LogPath(), LogSize: -1}
	if fi, err := os.Stat(s.LogPath); err == nil {
		s.LogSize = fi.Size()
	}
	data, ours, exists, err := m.readPlist()
	s.Installed = exists
	switch {
	case err != nil:
		s.PlistError = err.Error()
	case exists:
		s.Foreign = !ours
		if p, err := ParsePlist(data); err != nil {
			s.PlistError = err.Error()
		} else {
			s.Program = p.Program()
			if len(p.ProgramArguments) > 1 {
				s.Arguments = p.ProgramArguments[1:]
			}
			s.ConfigPath = p.Arg("--config")
		}
	}
	if s.Program != "" {
		real, err := filepath.EvalSymlinks(s.Program)
		switch {
		case err != nil:
			s.ProgramMissing = true
		case m.Self != "" && real != m.Self:
			s.ProgramDiffers = true
		}
	}
	job, err := m.job(ctx)
	if err != nil {
		s.LaunchctlError = err.Error()
	} else {
		s.Loaded = job.Loaded
		s.Running = job.Running()
		s.PID = job.PID
		s.State = job.State
		s.LastExit = job.LastExit
		if job.LastExit != nil {
			s.LastExitMeaning = Explain(*job.LastExit)
		}
	}
	if res, err := m.Launchctl.Run(ctx, "print-disabled", m.Layout.Domain()); err == nil && res.Code == 0 {
		s.Disabled = ParseDisabled(res.Stdout, Label)
	}
	return s
}

// touch creates the file with the mode 0600 if it does not exist.
func touch(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	return f.Close()
}

// writeAtomic writes data to a temporary file in the directory of path
// with the mode 0644 and renames it to path. A link at path is replaced,
// never followed.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".a38-service-*.plist")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() { _ = os.Remove(tmp) }()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Chmod(0o644); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
