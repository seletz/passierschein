package doctor

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/config"
)

// configCheck checks that the configuration loads, and each project.
type configCheck struct{ env Env }

func (configCheck) Name() string { return "configuration" }

func (c configCheck) Run(_ context.Context, st *State) []Result {
	cfg, err := c.env.LoadConfig()
	if err != nil {
		return []Result{finding(c.Name(), "the configuration does not load", errText(err),
			"fix the configuration file, then run a38 doctor again")}
	}
	st.Config = cfg
	invalid := cfg.InvalidProjects()
	out := []Result{ok(c.Name(), "loaded, %d projects, %d vaults", len(cfg.Projects), len(cfg.Vaults))}
	for _, p := range invalid {
		out = append(out, finding("project "+p.Name, fmt.Sprintf("the configuration of the project %q is invalid", p.Name), p.Invalid,
			"fix the project in the configuration file and restart the daemon (a38 serve)"))
	}
	// A path that does not exist is not an invalid project: the project
	// stays valid, but no session can match the path (issue 44).
	for _, p := range cfg.Projects {
		if len(p.MissingPaths) > 0 {
			out = append(out, finding("project "+p.Name, fmt.Sprintf("the project %q has a path that does not exist", p.Name),
				strings.Join(p.MissingPaths, ", "),
				"create the directory or fix the path in the configuration file, then restart the daemon (a38 serve)"))
		}
	}
	return out
}

// socketCheck checks the modes of the socket directory and of the sockets
// (D43).
type socketCheck struct{ env Env }

func (socketCheck) Name() string { return "socket modes" }

func (c socketCheck) Run(_ context.Context, st *State) []Result {
	if st.Config == nil {
		return nil
	}
	dir := st.Config.Paths.SocketDir
	fi, err := c.env.Lstat(dir)
	switch {
	case isMissing(err):
		return []Result{ok(c.Name(), "%s does not exist yet; the daemon creates it with the mode 0700", dir)}
	case err != nil:
		return []Result{finding(c.Name(), "the socket directory cannot be read", errText(err), "check the path "+dir)}
	case !fi.IsDir():
		return []Result{finding(c.Name(), "the socket path is not a directory", dir+" is not a directory",
			"remove "+dir+" or set paths.socket_dir to another path")}
	}
	if perm := fi.Mode().Perm(); perm != 0o700 || fi.Mode()&(fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky) != 0 {
		return []Result{finding(c.Name(), "the socket directory has the wrong mode",
			fmt.Sprintf("%s has the mode %04o; it must be 0700 (D43)", dir, perm),
			"run: chmod 700 "+dir+", then restart the daemon")}
	}
	if s, okStat := fi.Sys().(*syscall.Stat_t); okStat && int(s.Uid) != os.Getuid() {
		return []Result{finding(c.Name(), "the socket directory belongs to another user",
			dir+" does not belong to the current user", "remove the directory or set paths.socket_dir to another path")}
	}
	for _, name := range []string{config.AgentSocketName, config.AdminSocketName} {
		p := filepath.Join(dir, name)
		sfi, err := c.env.Lstat(p)
		if isMissing(err) {
			continue
		}
		if err != nil {
			return []Result{finding(c.Name(), "a socket cannot be read", errText(err), "check the path "+p)}
		}
		if perm := sfi.Mode().Perm(); perm != 0o600 {
			return []Result{finding(c.Name(), "a socket has the wrong mode",
				fmt.Sprintf("%s has the mode %04o; it must be 0600 (D43)", p, perm),
				"restart the daemon; it sets the mode when it creates the socket")}
		}
	}
	return []Result{ok(c.Name(), "directory 0700, sockets 0600")}
}

// daemonCheck asks the daemon for its status on the admin socket.
type daemonCheck struct{ env Env }

func (daemonCheck) Name() string { return "daemon" }

func (c daemonCheck) Run(ctx context.Context, st *State) []Result {
	if st.Config == nil {
		return nil
	}
	sock := st.Config.AdminSocket()
	status, err := c.env.Status(ctx, sock)
	if err != nil {
		return []Result{finding(c.Name(), "the daemon does not answer", errText(err),
			"start the daemon with a38 serve, or check that it runs with this configuration")}
	}
	st.Status = status
	return []Result{ok(c.Name(), "version %s, %d sessions, %d grants", status.DaemonVersion, status.Sessions, status.Grants)}
}

// lockCheck reports the global delivery lock (D48).
type lockCheck struct{}

func (lockCheck) Name() string { return "delivery lock" }

func (c lockCheck) Run(_ context.Context, st *State) []Result {
	if st.Status == nil {
		return nil
	}
	f := st.Status.Fault
	if f == nil {
		return []Result{ok(c.Name(), "delivery is open")}
	}
	cause := string(f.Code)
	if f.Reason != "" {
		cause += ": " + f.Reason
	}
	if cause == "" {
		cause = "the daemon does not name the cause"
	}
	return []Result{finding(c.Name(), "delivery is locked for all sessions"+since(f.Since), cause,
		"fix the cause, then run: a38 fault clear")}
}

// projectCheck names the projects that the daemon rejects. The check of
// the configuration names the projects of the file; this one adds those
// that only the daemon knows, for example after a change of the file.
type projectCheck struct{}

func (projectCheck) Name() string { return "daemon projects" }

func (c projectCheck) Run(_ context.Context, st *State) []Result {
	if st.Status == nil {
		return nil
	}
	known := map[string]bool{}
	if st.Config != nil {
		for _, p := range st.Config.InvalidProjects() {
			known[p.Name] = true
		}
	}
	var out []Result
	for _, p := range st.Status.InvalidProjects {
		if known[p.Project] {
			continue
		}
		out = append(out, finding("project "+p.Project, fmt.Sprintf("the daemon rejects the project %q", p.Project), p.Reason,
			"fix the project in the configuration file and restart the daemon (a38 serve)"))
	}
	return out
}

// auditFileCheck checks that the audit file exists, that the daemon can
// write it and that its directory is writable.
type auditFileCheck struct{ env Env }

func (auditFileCheck) Name() string { return "audit file" }

func (c auditFileCheck) Run(_ context.Context, st *State) []Result {
	if st.Config == nil {
		return nil
	}
	path := st.Config.Paths.AuditFile
	if _, err := c.env.Lstat(path); err != nil {
		if isMissing(err) {
			return []Result{finding(c.Name(), "the audit file does not exist", path+" does not exist",
				"start the daemon with a38 serve; it creates the file")}
		}
		return []Result{finding(c.Name(), "the audit file cannot be read", errText(err), "check the path and the rights of "+path)}
	}
	if err := c.env.Writable(path); err != nil {
		return []Result{finding(c.Name(), "the audit file is not writable", errText(err),
			"fix the rights of "+path+"; without the audit log the daemon locks delivery")}
	}
	dir := filepath.Dir(path)
	if err := c.env.Writable(dir); err != nil {
		return []Result{finding(c.Name(), "the audit directory is not writable", errText(err),
			"fix the rights of "+dir+"; the audit log needs to create files there")}
	}
	return []Result{ok(c.Name(), "%s exists and is writable", path)}
}

// auditSpaceCheck checks the free space of the audit directory.
type auditSpaceCheck struct{ env Env }

func (auditSpaceCheck) Name() string { return "audit space" }

func (c auditSpaceCheck) Run(_ context.Context, st *State) []Result {
	if st.Config == nil {
		return nil
	}
	dir := filepath.Dir(st.Config.Paths.AuditFile)
	free, err := c.env.FreeBytes(dir)
	if err != nil {
		if isMissing(err) {
			return nil // The check of the audit file has said it.
		}
		return []Result{finding(c.Name(), "the free space cannot be read", errText(err), "check the path "+dir)}
	}
	if free < MinFreeBytes {
		return []Result{finding(c.Name(), "the audit directory has little free space",
			fmt.Sprintf("%d MiB are free in %s; the least is %d MiB", free>>20, dir, MinFreeBytes>>20),
			"free disk space; if the audit log cannot grow, the daemon locks delivery")}
	}
	return []Result{ok(c.Name(), "%d MiB free (least: %d MiB)", free>>20, MinFreeBytes>>20)}
}

// auditChainCheck verifies the hash chain, read-only.
type auditChainCheck struct{ env Env }

func (auditChainCheck) Name() string { return "audit chain" }

func (c auditChainCheck) Run(ctx context.Context, st *State) []Result {
	if st.Config == nil {
		return nil
	}
	path := st.Config.Paths.AuditFile
	if _, err := c.env.Lstat(path); err != nil {
		return nil // The check of the audit file has said it.
	}
	v, err := c.env.VerifyAudit(ctx, path)
	if err != nil {
		return []Result{finding(c.Name(), "the audit log cannot be verified", errText(err),
			"check that "+path+" is an audit log of a38 and that you can read it")}
	}
	if !v.OK {
		return []Result{finding(c.Name(), "the hash chain of the audit log is broken",
			fmt.Sprintf("entry %d: %s", v.FirstBadSeq, v.Finding),
			"keep the file as evidence and look at it with a38 audit list; restore it from a backup or move it away so that the daemon starts a new log")}
	}
	return []Result{ok(c.Name(), "intact, %d entries", v.Entries)}
}

// backendCheck reports the backend state as far as it is known.
type backendCheck struct{ env Env }

func (backendCheck) Name() string { return "backend" }

func (c backendCheck) Run(_ context.Context, st *State) []Result {
	if st.Status == nil {
		return nil
	}
	comp := c.env.Backend(st.Status)
	next := "unlock the vault app of the backend (for 1Password: unlock it) and check that it runs"
	if st.Status.Backend != nil && st.Status.Backend.Kind == string(backend.KindOnePassword) {
		// F12: the service account has no lock. Locked means: 1Password
		// rejects the token.
		switch comp.State {
		case StateLocked:
			comp.Detail = "1Password rejects the token of the service account (revoked, expired or wrong)"
			next = "create a new token for the service account, store it with a38 token set --replace (or in the token file with token_source: file) and restart the daemon"
		case StateUnreachable:
			comp.Detail = "1Password does not answer (network, rate limit or time limit); the daemon log names the class of the error"
			next = "check the network connection; the daemon tries again by itself; each request ends with backend_unavailable until then"
		}
	}
	return component(c.Name(), comp, next)
}

// uiCheck reports the approval UI state as far as it is known.
type uiCheck struct{ env Env }

func (uiCheck) Name() string { return "approval UI" }

func (c uiCheck) Run(_ context.Context, st *State) []Result {
	if st.Status == nil {
		return nil
	}
	return component(c.Name(), c.env.UI(st.Status), "check the approval app (path, owner, mode; the reference page names the search path) and try the request again")
}

func component(name string, comp Component, next string) []Result {
	switch comp.State {
	case StateOK:
		return []Result{ok(name, "%s", detailOr(comp.Detail, "ok"))}
	case StateLocked:
		return []Result{finding(name, name+" is locked", detailOr(comp.Detail, "locked"), next)}
	case StateUnreachable:
		return []Result{finding(name, name+" is not reachable", detailOr(comp.Detail, "no answer"), next)}
	}
	return []Result{ok(name, "state not reported (%s)", comp.Detail)}
}

func detailOr(d, def string) string {
	if d == "" {
		return def
	}
	return d
}
