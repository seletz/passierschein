package doctor

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
	"time"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/audit/sqlite"
	"github.com/seletz/passierschein/internal/client"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/protocol"
)

// ComponentState is the state of a part that the doctor cannot check
// itself, for example the backend.
type ComponentState string

// The states of a component.
const (
	// StateUnknown: the daemon does not report the state.
	StateUnknown ComponentState = "unknown"
	// StateOK: the component works.
	StateOK ComponentState = "ok"
	// StateLocked: the component refuses to work until the human unlocks it.
	StateLocked ComponentState = "locked"
	// StateUnreachable: the component does not answer.
	StateUnreachable ComponentState = "unreachable"
)

// Component describes the state of the backend or of the approval UI.
type Component struct {
	State  ComponentState
	Detail string
}

// Env is the environment of the checks.
type Env interface {
	// LoadConfig loads and checks the configuration file.
	LoadConfig() (*config.Config, error)
	// Lstat reads the file information without following a link.
	Lstat(path string) (fs.FileInfo, error)
	// Writable reports why the audit file or its directory is not
	// writable. It changes nothing that stays.
	Writable(path string) error
	// FreeBytes returns the free space of the file system of dir.
	FreeBytes(dir string) (uint64, error)
	// Status asks the daemon on the admin socket.
	Status(ctx context.Context, socket string) (*protocol.StatusResult, error)
	// VerifyAudit checks the hash chain of the audit file, read-only.
	VerifyAudit(ctx context.Context, path string) (audit.Verification, error)
	// Backend returns the state of the backend as far as the status of
	// the daemon tells it.
	Backend(st *protocol.StatusResult) Component
	// UI returns the state of the approval UI as far as the status of
	// the daemon tells it.
	UI(st *protocol.StatusResult) Component
}

// SystemEnv is the real environment.
type SystemEnv struct {
	// ConfigPath is the configuration file; empty means the default.
	ConfigPath string
}

// LoadConfig implements [Env].
func (e SystemEnv) LoadConfig() (*config.Config, error) {
	path := e.ConfigPath
	if path == "" {
		var err error
		if path, err = config.DefaultPath(); err != nil {
			return nil, err
		}
	}
	return config.Load(path)
}

// Lstat implements [Env].
func (SystemEnv) Lstat(path string) (fs.FileInfo, error) { return os.Lstat(path) }

// Writable implements [Env]. For a directory it creates and removes a
// temporary file; for a file it opens the file for writing without
// changing it.
func (SystemEnv) Writable(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		f, err := os.CreateTemp(path, ".a38-doctor-*")
		if err != nil {
			return err
		}
		name := f.Name()
		_ = f.Close()
		return os.Remove(name)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return err
	}
	return f.Close()
}

// FreeBytes implements [Env].
func (SystemEnv) FreeBytes(dir string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil //nolint:gosec // sizes are not negative
}

// Status implements [Env].
func (SystemEnv) Status(ctx context.Context, socket string) (*protocol.StatusResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	c, err := client.DialAdmin(ctx, socket)
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.Close() }()
	return c.Status(ctx)
}

// VerifyAudit implements [Env].
func (SystemEnv) VerifyAudit(ctx context.Context, path string) (audit.Verification, error) {
	s, err := sqlite.OpenReadOnly(path)
	if err != nil {
		return audit.Verification{}, err
	}
	defer func() { _ = s.Close() }()
	return s.Verify(ctx)
}

// Backend implements [Env]. It reads the backend state from the status of
// the daemon. An old daemon without the field gives the state unknown.
func (SystemEnv) Backend(st *protocol.StatusResult) Component {
	if st == nil || st.Backend == nil {
		return Component{State: StateUnknown, Detail: "the daemon does not report the backend state; a locked or unreachable backend fails the single request and does not lock delivery"}
	}
	switch st.Backend.State {
	case protocol.BackendOK:
		return Component{State: StateOK}
	case protocol.BackendLocked:
		return Component{State: StateLocked, Detail: st.Backend.Reason}
	case protocol.BackendUnreachable:
		return Component{State: StateUnreachable, Detail: st.Backend.Reason}
	}
	return Component{State: StateUnknown, Detail: "the daemon reports an unknown backend state"}
}

// UI implements [Env]. It reads the mode of the approval UI and whether
// the daemon can use it from the status of the daemon. An old daemon
// without the field gives the state unknown.
func (SystemEnv) UI(st *protocol.StatusResult) Component {
	if st == nil || st.UI == nil {
		return Component{State: StateUnknown, Detail: "the daemon does not report the approval UI state"}
	}
	if !st.UI.Usable {
		return Component{State: StateUnreachable, Detail: "mode " + st.UI.Mode + ": " + detailOr(st.UI.Reason, "not usable")}
	}
	return Component{State: StateOK, Detail: "mode " + st.UI.Mode}
}

// isMissing reports whether err says that a file does not exist.
func isMissing(err error) bool { return errors.Is(err, fs.ErrNotExist) }

// errText returns the text of err for an output line.
func errText(err error) string { return fmt.Sprintf("%v", err) }
