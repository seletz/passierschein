package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// ErrExists means that a new configuration file would overwrite a file.
var ErrExists = errors.New("the file exists")

// ErrSymlink means that the path of the configuration file is a symbolic
// link. a38 never writes through a link: the target can be a file that
// another tool owns.
var ErrSymlink = errors.New("the path is a symbolic link")

// ErrChanged means that another program changed the file during an edit.
var ErrChanged = errors.New("the file changed while a38 edited it")

// ErrLocked means that a lock file of another edit exists.
var ErrLocked = errors.New("another change is in progress")

// lock creates the lock file of path. It is a file next to the
// configuration file, created with O_EXCL: only one change at a time.
// The caller calls the returned function when done.
func lock(path string) (func(), error) {
	lockPath := path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("%w: the lock file %s exists. If no other a38 command runs, remove the file", ErrLocked, lockPath)
		}
		return nil, err
	}
	_ = f.Close()
	return func() { _ = os.Remove(lockPath) }, nil
}

// refuseLink returns ErrSymlink if path is a symbolic link.
func refuseLink(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s. Write the file at its real location, or use --config with the target", ErrSymlink, path)
	}
	return nil
}

// WriteNew writes a new configuration file. It creates the directory with
// the mode 0700 if it is missing and the file with the mode 0600. It never
// overwrites a file: an existing file gives ErrExists. With force, it
// moves the old file to path.bak-<timestamp> first and returns the name of
// the backup.
func WriteNew(path string, data []byte, force bool, now time.Time) (backup string, err error) {
	if err := refuseLink(path); err != nil {
		return "", err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("cannot create the directory %s: %w", dir, err)
	}
	if _, err := os.Lstat(path); err == nil {
		if !force {
			return "", fmt.Errorf("%w: %s", ErrExists, path)
		}
		unlock, err := lock(path)
		if err != nil {
			return "", err
		}
		defer unlock()
		backup = path + ".bak-" + now.UTC().Format("20060102T150405Z")
		if _, err := os.Lstat(backup); err == nil {
			return "", fmt.Errorf("the backup file %s exists already; try again in a second", backup)
		}
		if err := os.Rename(path, backup); err != nil {
			return "", fmt.Errorf("cannot move the old file: %w", err)
		}
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return backup, fmt.Errorf("%w: %s", ErrExists, path)
		}
		return backup, fmt.Errorf("cannot create %s: %w", path, err)
	}
	err = writeSync(f, data)
	if err != nil {
		_ = os.Remove(path)
		return backup, fmt.Errorf("cannot write %s: %w", path, err)
	}
	// The umask can only remove bits, but make the mode certain.
	if err := os.Chmod(path, 0o600); err != nil {
		return backup, err
	}
	return backup, nil
}

// Replace writes data as the new content of the file at path. old is the
// content that the caller has read and changed. Replace refuses a
// symbolic link, takes the lock file, checks that the file still has the
// content old, writes a temporary file in the same directory with the mode
// of the old file and renames it over the file. A reader sees the old or
// the new file, never a part.
func Replace(path string, old, data []byte) error {
	if err := refuseLink(path); err != nil {
		return err
	}
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	unlock, err := lock(path)
	if err != nil {
		return err
	}
	defer unlock()
	if err := sameContent(path, old); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("cannot create a temporary file in %s: %w", dir, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := tmp.Chmod(fi.Mode().Perm()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := writeSync(tmp, data); err != nil {
		return fmt.Errorf("cannot write %s: %w", tmp.Name(), err)
	}
	// Compare again right before the rename: a program that does not
	// know the lock file can still have written.
	if err := sameContent(path, old); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("cannot replace %s: %w", path, err)
	}
	return nil
}

func sameContent(path string, old []byte) error {
	cur, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(cur, old) {
		return fmt.Errorf("%w: %s. Nothing is written; run the command again", ErrChanged, path)
	}
	return nil
}

// writeSync writes data to the file, flushes it and closes it.
func writeSync(f *os.File, data []byte) error {
	_, err := f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}
