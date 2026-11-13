package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/audit/sqlite"
)

// auditReader reads an audit file for "a38 tui". Each call opens the file
// read-only and closes it again, so that the view holds no lock between
// two reads and sees the entries that the daemon wrote in between.
type auditReader struct{ path string }

// Load returns all entries in the sequence of their numbers.
func (r auditReader) Load(ctx context.Context) ([]audit.Entry, error) {
	s, err := sqlite.OpenReadOnly(r.path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = s.Close() }()
	return s.List(ctx, audit.Filter{})
}

// Verify checks the hash chain.
func (r auditReader) Verify(ctx context.Context) (audit.Verification, error) {
	s, err := sqlite.OpenReadOnly(r.path)
	if err != nil {
		return audit.Verification{}, err
	}
	defer func() { _ = s.Close() }()
	return s.Verify(ctx)
}

// displayPath returns the path for the status line, with "~" for the home
// directory.
func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if rel, err := filepath.Rel(home, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.Join("~", rel)
	}
	return path
}
