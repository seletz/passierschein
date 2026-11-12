//go:build darwin

package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// isTerminal reports whether f is a terminal.
func isTerminal(f *os.File) bool {
	_, err := unix.IoctlGetTermios(int(f.Fd()), unix.TIOCGETA) //nolint:gosec // a file descriptor fits in an int
	return err == nil
}

// readSecretLine reads one line from the terminal f without echo. It
// writes the prompt to w (stderr). It switches the terminal to
// non-canonical mode: macOS limits a line in canonical mode to 1024 bytes
// (MAX_CANON), and a token can be about that long. It also switches off
// the signal keys, so that Ctrl-C arrives as a byte, the function restores
// the terminal and returns an error; the terminal never stays without
// echo. Backspace removes the last byte. The line has at most maxLen
// bytes; a longer line is read to its end and discarded, and the input
// that waits after the line is flushed, so that no part of a token
// reaches the shell.
func readSecretLine(f *os.File, w io.Writer, prompt string, maxLen int) ([]byte, error) {
	fd := int(f.Fd()) //nolint:gosec // a file descriptor fits in an int
	old, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return nil, fmt.Errorf("cannot read the terminal settings: %w", err)
	}
	raw := *old
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Iflag |= unix.ICRNL
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, &raw); err != nil {
		return nil, fmt.Errorf("cannot switch off the echo of the terminal: %w", err)
	}
	defer func() {
		// Flush the input that waits (FREAD), then restore the settings.
		_ = unix.IoctlSetPointerInt(fd, unix.TIOCFLUSH, 1)
		_ = unix.IoctlSetTermios(fd, unix.TIOCSETA, old)
	}()

	_, _ = fmt.Fprint(w, prompt)
	defer func() { _, _ = fmt.Fprintln(w) }()
	line := make([]byte, 0, maxLen)
	tooLong := false
	var b [1]byte
	for {
		n, err := f.Read(b[:])
		if n == 0 || err != nil {
			clear(line)
			if err == nil || errors.Is(err, io.EOF) {
				return nil, errors.New("the input ended before the end of the line")
			}
			return nil, fmt.Errorf("cannot read the terminal: %w", err)
		}
		switch c := b[0]; c {
		case '\n', '\r':
			if tooLong {
				return nil, fmt.Errorf("the token is longer than %d bytes", maxLen)
			}
			return line, nil
		case 0x03: // Ctrl-C
			clear(line)
			return nil, errors.New("cancelled")
		case 0x04: // Ctrl-D
			if len(line) == 0 || tooLong {
				return nil, errors.New("cancelled")
			}
			return line, nil
		case 0x7f, 0x08: // Backspace
			if len(line) > 0 {
				line[len(line)-1] = 0
				line = line[:len(line)-1]
			}
		default:
			if tooLong || len(line) >= maxLen {
				// Discard the rest of the line.
				clear(line)
				line, tooLong = line[:0], true
				continue
			}
			line = append(line, c)
		}
	}
}
