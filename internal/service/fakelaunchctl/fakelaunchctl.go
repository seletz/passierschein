// Package fakelaunchctl is a fake of /bin/launchctl for tests. It keeps
// the state of one job in memory and never calls launchd.
package fakelaunchctl

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/seletz/passierschein/internal/service"
)

// Fake is a fake [service.Launchctl] for one job.
type Fake struct {
	mu sync.Mutex
	// Calls lists each call with its arguments.
	Calls [][]string
	// Loaded is true if the job is loaded.
	Loaded bool
	// PID is the process id of the running daemon; 0 if it does not run.
	PID int
	// LastExit is the text of the line "last exit code"; empty for
	// "(never exited)".
	LastExit string
	// Disabled is true after "disable" or before "enable".
	Disabled bool
	// BootstrapCode is the exit code of the next bootstrap; 0 succeeds.
	BootstrapCode int
	// StartFails makes the daemon end at once with exit code 1 after a
	// bootstrap, as with a configuration error.
	StartFails bool
	// Program is the program that print reports.
	Program string
	nextPID int
}

// New returns a fake with no job.
func New() *Fake { return &Fake{nextPID: 4242} }

// Run implements [service.Launchctl].
func (f *Fake) Run(_ context.Context, args ...string) (service.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, slices.Clone(args))
	if len(args) == 0 {
		return service.Result{}, fmt.Errorf("no subcommand")
	}
	notFound := service.Result{Code: 113, Stderr: "Could not find service \"" + service.Label + "\" in domain for user gui: 501\n"}
	switch args[0] {
	case "print":
		if !f.Loaded {
			return notFound, nil
		}
		return service.Result{Stdout: f.print(args[1])}, nil
	case "print-disabled":
		v := "enabled"
		if f.Disabled {
			v = "disabled"
		}
		return service.Result{Stdout: "disabled services = {\n\t\"com.example.other\" => disabled\n\t\"" + service.Label + "\" => " + v + "\n}\n"}, nil
	case "enable":
		f.Disabled = false
		return service.Result{}, nil
	case "bootstrap":
		if f.BootstrapCode != 0 {
			return service.Result{Code: f.BootstrapCode, Stderr: fmt.Sprintf("Bootstrap failed: %d: Operation not permitted\n", f.BootstrapCode)}, nil
		}
		if f.Loaded {
			return service.Result{Code: 5, Stderr: "Bootstrap failed: 5: Input/output error\n"}, nil
		}
		f.Loaded = true
		f.start()
		return service.Result{}, nil
	case "bootout":
		if !f.Loaded {
			return notFound, nil
		}
		f.Loaded, f.PID = false, 0
		return service.Result{}, nil
	case "kickstart":
		if !f.Loaded {
			return notFound, nil
		}
		f.start()
		return service.Result{}, nil
	}
	return service.Result{Code: 64, Stderr: "Unrecognized subcommand: " + args[0] + "\n"}, nil
}

func (f *Fake) start() {
	if f.StartFails {
		f.PID, f.LastExit = 0, "1"
		return
	}
	f.nextPID++
	f.PID = f.nextPID
}

// print returns an output in the form of "launchctl print" on macOS 15.
func (f *Fake) print(target string) string {
	state, pid, last := "not running", "", "(never exited)"
	if f.PID > 0 {
		state, pid = "running", fmt.Sprintf("\tpid = %d\n", f.PID)
	}
	if f.LastExit != "" {
		last = f.LastExit
	}
	program := f.Program
	if program == "" {
		program = "/Applications/Passierschein.app/Contents/MacOS/a38"
	}
	var b strings.Builder
	b.WriteString(target + " = {\n\tactive count = 1\n\tpath = /Users/beispiel/Library/LaunchAgents/" + service.Label + ".plist\n")
	b.WriteString("\ttype = LaunchAgent\n\tstate = " + state + "\n\n\tprogram = " + program + "\n")
	b.WriteString("\targuments = {\n\t\t" + program + "\n\t\tserve\n\t}\n\n")
	b.WriteString("\tstdout path = /Users/beispiel/Library/Logs/a38/serve.log\n\tinherited environment = {\n\t\tSSH_AUTH_SOCK => /private/tmp/x\n\t}\n\n")
	b.WriteString("\tdomain = gui/501 [100005]\n\truns = 1\n" + pid + "\timmediate reason = speculative\n")
	b.WriteString("\tlast exit code = " + last + "\n\n\tspawn type = interactive (4)\n\tproperties = runatload | inferred program\n}\n")
	return b.String()
}

// Subcommands returns the subcommands of all calls in order.
func (f *Fake) Subcommands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var s []string
	for _, c := range f.Calls {
		s = append(s, c[0])
	}
	return s
}

// Changing returns the calls that change the state of launchd.
func (f *Fake) Changing() []string {
	var s []string
	for _, c := range f.Subcommands() {
		if c != "print" && c != "print-disabled" {
			s = append(s, c)
		}
	}
	return s
}

// Reset forgets the calls.
func (f *Fake) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = nil
}
