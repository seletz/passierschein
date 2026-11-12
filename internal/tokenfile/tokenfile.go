// Package tokenfile reads the token file of the 1Password service account
// and checks its owner and mode. The package holds no code for 1Password
// itself, so that "a38 doctor" can check the file also in the linux build.
//
// # Working assumption (G3)
//
// The design does not say where the token lives. Stage 3 decides as a
// working assumption, to be confirmed by Stefan:
//
//   - The configuration names a token file (backend.onepassword.token_file,
//     default ~/.config/a38/onepassword-token).
//   - The file is a regular file (not a symbolic link, not a directory),
//     belongs to the user of the daemon and has the mode 0600 exactly.
//     Otherwise the daemon does not start with the 1Password backend.
//   - The daemon reads the file once at its start and keeps the token in
//     memory only, in a [token.Token].
//   - The file is the interim source for development and tests (D51). The
//     target is the macOS keychain; a keychain source implements
//     [token.Source] like [File].
//   - There is no environment variable for the token: a child process
//     inherits the environment.
//
// No error of this package contains the content of the file.
package tokenfile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"

	"github.com/seletz/passierschein/internal/token"
)

// MaxSize is the largest token file in bytes that Read accepts. A service
// account token is about 1 KiB.
const MaxSize = 16 << 10

// The findings of a check. Each error of the package wraps one of them.
var (
	// ErrMissing: the file does not exist.
	ErrMissing = errors.New("the token file does not exist")
	// ErrSymlink: the path is a symbolic link.
	ErrSymlink = errors.New("the token file is a symbolic link")
	// ErrNotRegular: the path is a directory or another special file.
	ErrNotRegular = errors.New("the token file is not a regular file")
	// ErrOwner: the file belongs to another user, or the owner is unknown.
	ErrOwner = errors.New("the token file does not belong to the user")
	// ErrMode: the mode is not 0600.
	ErrMode = errors.New("the token file must have the mode 0600")
	// ErrEmpty: the file holds only white space.
	ErrEmpty = errors.New("the token file is empty")
	// ErrTooLarge: the file is larger than MaxSize.
	ErrTooLarge = errors.New("the token file is larger than a token")
)

// File is the token source of a token file (D51: interim source for
// development and tests; the target is the macOS keychain).
type File struct {
	// Path is the token file.
	Path string
}

// Token reads the token file with [Read].
func (f File) Token() (token.Token, error) { return Read(f.Path) }

var _ token.Source = File{}

// CheckInfo checks the file information of a token file: a regular file
// of the user uid with the mode 0600 and no special bit. fi must come from
// os.Lstat or from Stat of the open file.
func CheckInfo(fi fs.FileInfo, uid int) error {
	m := fi.Mode()
	switch {
	case m&fs.ModeSymlink != 0:
		return ErrSymlink
	case !m.IsRegular():
		return ErrNotRegular
	}
	if m.Perm() != 0o600 || m&(fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky) != 0 {
		return fmt.Errorf("%w; it has the mode %04o", ErrMode, m.Perm())
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		// Fail closed: without the owner we cannot judge the file.
		return fmt.Errorf("%w: the owner is unknown", ErrOwner)
	}
	if int(st.Uid) != uid {
		return fmt.Errorf("%w; it belongs to user %d", ErrOwner, st.Uid)
	}
	return nil
}

// Check checks the token file at path without reading its content.
func Check(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return statError(err)
	}
	return CheckInfo(fi, os.Getuid())
}

// Read checks the token file at path and returns its content without the
// white space at the start and the end. It opens the file without following
// a symbolic link and checks the open file, so that the file cannot change
// between the check and the read.
func Read(path string) (token.Token, error) {
	if err := Check(path); err != nil {
		return token.Token{}, err
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		if errors.Is(err, syscall.ELOOP) {
			return token.Token{}, ErrSymlink
		}
		return token.Token{}, statError(err)
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return token.Token{}, errors.New("cannot read the information of the token file")
	}
	if err := CheckInfo(fi, os.Getuid()); err != nil {
		return token.Token{}, err
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxSize+1))
	if err != nil {
		return token.Token{}, errors.New("cannot read the token file")
	}
	if len(data) > MaxSize {
		return token.Token{}, ErrTooLarge
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return token.Token{}, ErrEmpty
	}
	return token.New(string(data)), nil
}

// statError turns an error of os.Lstat or os.Open into a finding. The
// text of an *fs.PathError names the path and the cause only.
func statError(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return ErrMissing
	}
	if errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("%w: no permission to read it", ErrOwner)
	}
	return fmt.Errorf("cannot read the token file: %w", err)
}
