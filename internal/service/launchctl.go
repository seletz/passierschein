package service

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// LaunchctlTimeout limits one call of launchctl.
const LaunchctlTimeout = 15 * time.Second

// Launchctl runs /bin/launchctl. The real one exists on darwin only
// ([NewLaunchctl]); a test passes a fake.
type Launchctl interface {
	// Run runs launchctl with args, without a shell. It returns the
	// output and the exit code. err is set only if launchctl could not
	// run or did not end in time; a non-zero exit code is no error.
	Run(ctx context.Context, args ...string) (Result, error)
}

// Result is the outcome of one call of launchctl.
type Result struct {
	// Stdout and Stderr are the output.
	Stdout, Stderr string
	// Code is the exit code.
	Code int
}

// Text returns the output for a message: stderr, else stdout, on one
// line.
func (r Result) Text() string {
	s := strings.TrimSpace(r.Stderr)
	if s == "" {
		s = strings.TrimSpace(r.Stdout)
	}
	return strings.Join(strings.Fields(s), " ")
}

// allowedSubcommands are the subcommands that the package may call.
var allowedSubcommands = map[string]bool{
	"bootstrap": true, "bootout": true, "enable": true, "kickstart": true,
	"print": true, "print-disabled": true,
}

// checkArgs refuses a subcommand that the package does not use.
func checkArgs(args []string) error {
	if len(args) == 0 || !allowedSubcommands[args[0]] {
		return fmt.Errorf("launchctl: the subcommand %q is not allowed", strings.Join(args, " "))
	}
	return nil
}

// notFoundCode is the exit code of "launchctl print" and "bootout" for an
// unknown service ("Could not find service").
const notFoundCode = 113

// Job is what "launchctl print" says about the job. The output of print is
// no API (launchctl(1)); each field may be empty, and the package then
// says "unknown".
type Job struct {
	// Loaded is true if launchd knows the job.
	Loaded bool
	// State is the state, for example "running" or "not running".
	State string
	// PID is the process id; 0 if the job does not run.
	PID int
	// LastExit is the last exit; nil if the job never exited or print
	// does not say it.
	LastExit *Exit
	// Program is the program that launchd starts.
	Program string
	// Raw holds each top-level line "key = value" of the output.
	Raw map[string]string
}

// Running reports whether the job runs.
func (j Job) Running() bool { return j.PID > 0 || j.State == "running" }

var signalLine = regexp.MustCompile(`^[^:]*: (\d+)$`)

// ParsePrint reads the output of "launchctl print <service-target>". It
// reads only the lines "key = value" one level below the top.
func ParsePrint(out string) Job {
	j := Job{Loaded: true, Raw: map[string]string{}}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		// One tab or up to four spaces: the top level of the service.
		if !strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "\t\t") {
			continue
		}
		k, v, ok := strings.Cut(strings.TrimSpace(line), " = ")
		if !ok || strings.HasSuffix(v, "{") {
			continue
		}
		j.Raw[k] = v
	}
	j.State = j.Raw["state"]
	if n, err := strconv.Atoi(j.Raw["pid"]); err == nil && n > 0 {
		j.PID = n
	}
	j.Program = j.Raw["program"]
	if v, ok := j.Raw["last terminating signal"]; ok {
		if m := signalLine.FindStringSubmatch(v); m != nil {
			n, _ := strconv.Atoi(m[1])
			j.LastExit = &Exit{Signal: n}
		}
	}
	if j.LastExit == nil {
		if n, err := strconv.Atoi(j.Raw["last exit code"]); err == nil {
			j.LastExit = &Exit{Code: n}
		}
	}
	return j
}

// ParseDisabled reads the output of "launchctl print-disabled <domain>"
// and reports whether it lists the label as disabled.
func ParseDisabled(out, label string) bool {
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=>")
		if !ok {
			continue
		}
		if strings.Trim(strings.TrimSpace(k), `"`) == label {
			v = strings.TrimSpace(v)
			return v == "disabled" || v == "true"
		}
	}
	return false
}
