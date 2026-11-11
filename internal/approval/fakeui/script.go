package fakeui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/seletz/passierschein/internal/approval"
	"github.com/seletz/passierschein/internal/clock"
)

// maxOutput is the largest output of a script that we read.
const maxOutput = 1 << 20

// Script runs an external program for each request and each notice. The
// program gets the message as JSON on its standard input. For a request it
// writes the decision as JSON to its standard output. For a notice it
// writes nothing that we read.
//
// Each failure is an error or the outcome timeout, never an approval:
// start failure, exit code other than 0 (also with "approved" in the
// output), no JSON, a decision that does not fit the request.
type Script struct {
	command []string
	timeout time.Duration
	clock   clock.Clock
}

// NewScript returns a Script. A zero timeout means
// [approval.DefaultTimeout]. A nil clock means the system clock.
func NewScript(command []string, timeout time.Duration, clk clock.Clock) (*Script, error) {
	if len(command) == 0 || command[0] == "" {
		return nil, errors.New("approval mode script needs a command")
	}
	if timeout <= 0 {
		timeout = approval.DefaultTimeout
	}
	if clk == nil {
		clk = clock.System{}
	}
	return &Script{command: append([]string(nil), command...), timeout: timeout, clock: clk}, nil
}

// Approve runs the program with the request. If the program does not
// answer in time, Approve kills it and returns the outcome timeout.
func (s *Script) Approve(ctx context.Context, req approval.Request) (approval.Decision, error) {
	in, err := json.Marshal(req)
	if err != nil {
		return approval.Decision{}, fmt.Errorf("script: encode request: %w", err)
	}
	out, timedOut, err := s.run(ctx, in)
	if timedOut {
		d := decision(req, approval.OutcomeTimeout)
		d.Auth = nil
		d.ElapsedMS = s.timeout.Milliseconds()
		return d, nil
	}
	if err != nil {
		return approval.Decision{}, err
	}
	var d approval.Decision
	dec := json.NewDecoder(bytes.NewReader(out))
	if err := dec.Decode(&d); err != nil {
		return approval.Decision{}, errors.New("script: the output is not a decision")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return approval.Decision{}, errors.New("script: the output has more than one value")
	}
	d, err = ValidateDecision(req, d)
	if err != nil {
		return approval.Decision{}, fmt.Errorf("script: %w", err)
	}
	return d, nil
}

// Notify runs the program with the notice. The output is ignored. A start
// failure, a timeout or an exit code other than 0 is an error.
func (s *Script) Notify(ctx context.Context, n approval.Notice) error {
	in, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("script: encode notice: %w", err)
	}
	_, timedOut, err := s.run(ctx, in)
	if timedOut {
		return errors.New("script: the notice timed out")
	}
	return err
}

// run starts the program with stdin as input. It returns the standard
// output. timedOut is true if the timer fired first; the program is then
// killed.
func (s *Script) run(ctx context.Context, stdin []byte) (out []byte, timedOut bool, err error) {
	timer := s.clock.NewTimer(s.timeout)
	defer timer.Stop()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var stdout limitedBuffer
	cmd := exec.CommandContext(runCtx, s.command[0], s.command[1:]...) //nolint:gosec // the command is the operator's choice
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = &stdout
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Start(); err != nil {
		return nil, false, errors.New("script: cannot start the program")
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			// The message holds no output of the program.
			return nil, false, fmt.Errorf("script: the program failed: %s", exitText(err))
		}
		return stdout.Bytes(), false, nil
	case <-timer.C():
		cancel()
		waitKilled(done)
		return nil, true, nil
	case <-ctx.Done():
		cancel()
		waitKilled(done)
		return nil, false, ctx.Err()
	}
}

// exitText describes the failure of a program without its output.
func exitText(err error) string {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return fmt.Sprintf("exit status %d", ee.ExitCode())
	}
	return "run error"
}

// limitedBuffer collects output up to maxOutput bytes and fails after that.
type limitedBuffer struct{ buf bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.buf.Len()+len(p) > maxOutput {
		return 0, errors.New("output too large")
	}
	return b.buf.Write(p)
}

func (b *limitedBuffer) Bytes() []byte { return b.buf.Bytes() }

// waitKilled waits for the end of a program that we killed. The result is
// the kill that we caused, so we drop it.
func waitKilled(done <-chan error) {
	for range done {
		return
	}
}
