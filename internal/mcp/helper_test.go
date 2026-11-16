package mcp_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// helperArg makes the test binary a harmless helper program. The tool
// starts the test binary with this first argument.
const helperArg = "a38-helper"

// TestMain runs the helper program if the first argument is helperArg.
// Otherwise it runs the tests.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == helperArg {
		os.Exit(helper(os.Args[2:]))
	}
	os.Exit(m.Run())
}

// helper is the helper program. It only reads its environment and writes
// fixed text.
func helper(args []string) int {
	if len(args) == 0 {
		return 2
	}
	switch args[0] {
	case "envlen":
		// Writes the length of a variable and a fixed text.
		fmt.Printf("len=%d\nbeispiel-ausgabe\n", len(os.Getenv(args[1])))
	case "exit":
		n, _ := strconv.Atoi(args[1])
		return n
	case "echo":
		// Writes the value twice to stdout and once to stderr.
		v := os.Getenv(args[1])
		fmt.Printf("eins %s zwei %s\n", v, v)
		fmt.Fprintf(os.Stderr, "fehler: %s\n", v)
	case "stdin":
		b, _ := io.ReadAll(os.Stdin)
		fmt.Printf("stdin=%d\n", len(b))
	case "exe":
		// Writes the path of the started file and argv[0].
		exe, _ := os.Executable()
		fmt.Printf("%s\n%s", exe, os.Args[0])
	case "pwd":
		dir, _ := os.Getwd()
		fmt.Print(dir)
	case "big":
		n, _ := strconv.Atoi(args[1])
		buf := make([]byte, n)
		for i := range buf {
			buf[i] = byte('a' + i%26)
		}
		_, _ = os.Stdout.Write(buf)
	case "sleep":
		time.Sleep(time.Minute)
	case "child", "background", "ignore-term":
		// Starts a child in the same process group, writes both process
		// IDs and sleeps; "background" exits at once.
		if args[0] == "ignore-term" {
			signal.Ignore(syscall.SIGTERM)
		}
		child := exec.Command(os.Args[0], helperArg, "sleep")
		child.Stdout = os.Stdout
		if err := child.Start(); err != nil {
			return 2
		}
		fmt.Printf("pid=%d child=%d\n", os.Getpid(), child.Process.Pid)
		if args[0] == "background" {
			return 0
		}
		time.Sleep(time.Minute)
	default:
		return 2
	}
	return 0
}
