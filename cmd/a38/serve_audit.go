package main

import (
	"context"
	"sync"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/audit/sqlite"
)

// auditFile is the audit log of the daemon. It opens the SQLite file at
// the first use and again after a failed open. So the daemon starts also
// if the file cannot be opened: the self-check then raises the global
// lock (D48), and a38 doctor and the admin commands can reach the daemon.
// After the repair, the command fault_clear opens the file.
type auditFile struct {
	path string

	mu sync.Mutex
	s  *sqlite.Store
}

var _ audit.Store = (*auditFile)(nil)

// openAuditFile opens the file if it can. An error is no failure: the
// store tries again at the next use.
func openAuditFile(path string) (*auditFile, error) {
	a := &auditFile{path: path}
	_, err := a.store()
	return a, err
}

// store returns the open store and opens it if necessary.
func (a *auditFile) store() (*sqlite.Store, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.s != nil {
		return a.s, nil
	}
	s, err := sqlite.Open(a.path)
	if err != nil {
		return nil, err
	}
	a.s = s
	return s, nil
}

// Append writes the event.
func (a *auditFile) Append(ctx context.Context, ev audit.Event) (audit.Entry, error) {
	s, err := a.store()
	if err != nil {
		return audit.Entry{}, err
	}
	return s.Append(ctx, ev)
}

// List lists the entries.
func (a *auditFile) List(ctx context.Context, f audit.Filter) ([]audit.Entry, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	return s.List(ctx, f)
}

// Verify checks the hash chain.
func (a *auditFile) Verify(ctx context.Context) (audit.Verification, error) {
	s, err := a.store()
	if err != nil {
		return audit.Verification{}, err
	}
	return s.Verify(ctx)
}

// Probe checks that the file can be written. It implements fault.Prober.
func (a *auditFile) Probe(ctx context.Context) error {
	s, err := a.store()
	if err != nil {
		return err
	}
	return s.Probe(ctx)
}

// Close closes the file if it is open.
func (a *auditFile) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.s == nil {
		return nil
	}
	err := a.s.Close()
	a.s = nil
	return err
}
