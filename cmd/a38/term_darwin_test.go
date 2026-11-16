//go:build darwin

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/seletz/passierschein/internal/token"
)

// openPTY opens a pseudo terminal: the controller side (the "keyboard"
// and the screen) and the terminal side (the stdin of the command).
func openPTY(t *testing.T) (controller, term *os.File) {
	t.Helper()
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Skipf("no pseudo terminal: %v", err)
	}
	controller = os.NewFile(uintptr(fd), "ptmx")
	t.Cleanup(func() { _ = controller.Close() })
	if err := unix.IoctlSetInt(fd, unix.TIOCPTYGRANT, 0); err != nil {
		t.Fatal(err)
	}
	if err := unix.IoctlSetInt(fd, unix.TIOCPTYUNLK, 0); err != nil {
		t.Fatal(err)
	}
	var name [128]byte
	//nolint:staticcheck // x/sys/unix has no wrapper for TIOCPTYGNAME on darwin; test code only.
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(unix.TIOCPTYGNAME), uintptr(unsafe.Pointer(&name[0]))); errno != 0 {
		t.Fatal(errno)
	}
	path := string(name[:bytes.IndexByte(name[:], 0)])
	term, err = os.OpenFile(path, os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = term.Close() })
	return controller, term
}

func TestReadSecretLineWithoutEcho(t *testing.T) {
	// Issue 59: "a38 config onepassword-token set" reads the token from the terminal without
	// echo and restores the terminal. The real terminal of the human is
	// part of the practice test.
	controller, term := openPTY(t)
	before, err := unix.IoctlGetTermios(int(term.Fd()), unix.TIOCGETA)
	if err != nil {
		t.Fatal(err)
	}
	if !isTerminal(term) {
		t.Fatal("the pseudo terminal is no terminal")
	}

	// A token longer than MAX_CANON (1024 bytes), typed with a mistake.
	secret := tokenMarker + strings.Repeat("x", 1500)
	type result struct {
		line []byte
		err  error
	}
	done := make(chan result, 1)
	var prompt bytes.Buffer
	go func() {
		line, err := readSecretLine(term, &prompt, "Token: ", maxTokenInput)
		done <- result{line, err}
	}()
	// Wait until the echo is off, then type.
	deadline := time.Now().Add(5 * time.Second)
	for {
		st, err := unix.IoctlGetTermios(int(term.Fd()), unix.TIOCGETA)
		if err != nil {
			t.Fatal(err)
		}
		if st.Lflag&unix.ECHO == 0 {
			if st.Lflag&unix.ICANON != 0 {
				t.Error("the terminal is still in canonical mode")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the echo is still on")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := controller.Write([]byte(secret + "Q\x7f\r")); err != nil {
		t.Fatal(err)
	}
	var r result
	select {
	case r = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("readSecretLine does not return")
	}
	if r.err != nil || string(r.line) != secret {
		t.Fatalf("line of length %d, %v; want length %d", len(r.line), r.err, len(secret))
	}

	after, err := unix.IoctlGetTermios(int(term.Fd()), unix.TIOCGETA)
	if err != nil {
		t.Fatal(err)
	}
	if after.Lflag != before.Lflag || after.Iflag != before.Iflag {
		t.Errorf("the terminal is not restored: lflag %#x, want %#x", after.Lflag, before.Lflag)
	}

	// The echo is on again: a sentinel is echoed. Nothing before it may
	// hold a part of the token.
	if _, err := controller.Write([]byte("SENTINEL\n")); err != nil {
		t.Fatal(err)
	}
	screen := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		b := make([]byte, 4096)
		for !strings.Contains(buf.String(), "SENTINEL") {
			n, err := controller.Read(b)
			buf.Write(b[:n])
			if err != nil {
				break
			}
		}
		screen <- buf.String()
	}()
	select {
	case s := <-screen:
		before, _, _ := strings.Cut(s, "SENTINEL")
		if strings.Contains(before, "token-marker") || strings.Contains(before, "xxxx") {
			t.Errorf("the terminal echoed the token: %q", before)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no echo of the sentinel")
	}
	if prompt.String() != "Token: \n" {
		t.Errorf("prompt = %q", prompt.String())
	}
}

func TestReadSecretLineCancel(t *testing.T) {
	// Ctrl-C ends the input and restores the echo.
	for _, tt := range []struct{ name, input, want string }{
		{"ctrl-c", "ops_beispiel\x03", "cancelled"},
		{"ctrl-d", "\x04", "cancelled"},
		{"too long", strings.Repeat("y", maxTokenInput+10) + "\r", "longer than 4096 bytes"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			controller, term := openPTY(t)
			done := make(chan error, 1)
			go func() {
				_, err := readSecretLine(term, &bytes.Buffer{}, "", maxTokenInput)
				done <- err
			}()
			deadline := time.Now().Add(5 * time.Second)
			for {
				st, _ := unix.IoctlGetTermios(int(term.Fd()), unix.TIOCGETA)
				if st != nil && st.Lflag&unix.ECHO == 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("the echo is still on")
				}
				time.Sleep(5 * time.Millisecond)
			}
			if _, err := controller.Write([]byte(tt.input)); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Errorf("error = %v, want %q", err, tt.want)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("readSecretLine does not return")
			}
			st, err := unix.IoctlGetTermios(int(term.Fd()), unix.TIOCGETA)
			if err != nil || st.Lflag&unix.ECHO == 0 {
				t.Errorf("the echo stays off (%v)", err)
			}
		})
	}
}

func TestIsTerminalPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(); _ = w.Close() }()
	if isTerminal(r) {
		t.Error("a pipe is a terminal")
	}
}

func TestTokenSetFromPseudoTerminal(t *testing.T) {
	// Stefan, 2026-11-12: "set" reads the token from a terminal. The whole
	// command runs with a pseudo terminal as standard input and a fake
	// store; the output names the item only.
	controller, term := openPTY(t)
	store := newFakeStore()
	oldStore, oldStdin := newTokenStore, os.Stdin
	newTokenStore = func(string) (token.Store, error) { return store, nil }
	os.Stdin = term
	t.Cleanup(func() { newTokenStore, os.Stdin = oldStore, oldStdin })
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfg, []byte("version: 1\nbackend:\n  onepassword:\n    accounts:\n      - name: privat\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- runConfig([]string{"onepassword-token", "set", "--account", "privat", "--config", cfg}, &stdout, &stderr)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		st, _ := unix.IoctlGetTermios(int(term.Fd()), unix.TIOCGETA)
		if st != nil && st.Lflag&unix.ECHO == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the echo is still on")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err := controller.Write([]byte(tokenMarker + "\r")); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit code %d; stderr %q", code, stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("set does not return")
	}
	if store.value == nil || *store.value != tokenMarker {
		t.Error("the token is not stored")
	}
	if stdout.String() != "Stored the token of the account \"privat\" in the keychain item a38-test-fake/privat.\n" {
		t.Errorf("stdout = %q", stdout.String())
	}
	noToken(t, stdout.String(), stderr.String())
	noDerived(t, stdout.String(), stderr.String())
}
