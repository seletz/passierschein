//go:build darwin

package appui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// codesignPath is the fixed path of codesign. The daemon never looks it up
// in PATH.
const codesignPath = "/usr/bin/codesign"

// codesignTimeout limits each call of codesign.
const codesignTimeout = 10 * time.Second

// SystemCodeSigner returns the [CodeSigner] that runs /usr/bin/codesign.
func SystemCodeSigner() CodeSigner { return codesign{} }

type codesign struct{}

// run runs codesign with a timeout and a short environment and returns
// its standard error, where codesign writes its report.
func (codesign) run(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, codesignTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, codesignPath, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	cmd.Dir = "/"
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	if ctx.Err() != nil {
		return "", errors.New("codesign did not end in time")
	}
	return out.String(), err
}

// checkPath allows only an absolute path, so that codesign never reads
// a path as an option.
func checkPath(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("the path %q is not absolute", path)
	}
	return nil
}

func (c codesign) Describe(ctx context.Context, path string) (CodeSignature, error) {
	if err := checkPath(path); err != nil {
		return CodeSignature{}, err
	}
	out, err := c.run(ctx, "-d", "--verbose=2", path)
	if err != nil {
		if strings.Contains(out, "code object is not signed at all") {
			return CodeSignature{}, nil
		}
		return CodeSignature{}, fmt.Errorf("codesign cannot read the signature: %s", verdict(out, err))
	}
	return parseCodesignOutput(out), nil
}

func (c codesign) VerifyTeam(ctx context.Context, path, team string) error {
	if err := checkPath(path); err != nil {
		return err
	}
	if !teamIDPattern.MatchString(team) {
		return errors.New("the team ID has an unexpected form")
	}
	// The requirement of a Developer ID Application signature of the
	// team (TN3127): an Apple anchor, the Developer ID intermediate
	// (1.2.840.113635.100.6.2.6), the Developer ID Application leaf
	// (1.2.840.113635.100.6.1.13) and the team as organizational unit.
	// A self-signed certificate with the same team ID in its subject does
	// not satisfy it.
	req := `=anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] exists` +
		` and certificate leaf[field.1.2.840.113635.100.6.1.13] exists` +
		` and certificate leaf[subject.OU] = "` + team + `"`
	out, err := c.run(ctx, "--verify", "--strict", "--deep", "-R", req, path)
	if err != nil {
		return fmt.Errorf("codesign: %s", verdict(out, err))
	}
	return nil
}

// verdict returns the last non-empty line of the codesign output (the
// verdict), or the error if there is no output. The output names paths
// and requirements only; codesign prints no secret.
func verdict(out string, err error) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if l := strings.TrimSpace(lines[len(lines)-1]); l != "" {
		return l
	}
	return err.Error()
}
