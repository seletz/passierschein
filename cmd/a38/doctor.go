package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/seletz/passierschein/internal/doctor"
)

const doctorUsage = `Usage:
  a38 doctor [--config FILE] [--json]

The command runs the checks and prints for each finding the cause and the
next step. It exits with 0 if it finds nothing and with 1 otherwise. It
prints no secret value.
`

// runDoctor checks the state of a38 in the real environment.
func runDoctor(args []string, stdout, stderr io.Writer) int {
	return runDoctorWith(args, stdout, stderr, func(configPath string) doctor.Env {
		return doctor.SystemEnv{ConfigPath: configPath}
	})
}

// runDoctorWith runs the doctor with the environment that newEnv returns
// for the value of --config. A test passes a fake environment.
func runDoctorWith(args []string, stdout, stderr io.Writer, newEnv func(configPath string) doctor.Env) int {
	var af adminFlags
	fs := newFlagSet("doctor", stderr)
	af.register(fs)
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 0 {
		_, _ = fmt.Fprintf(stderr, "a38 doctor: unexpected argument\n\n%s", doctorUsage)
		return exitUsage
	}
	rep := doctor.Run(context.Background(), newEnv(af.configPath))
	if af.asJSON {
		if err := json.NewEncoder(stdout).Encode(rep); err != nil {
			_, _ = fmt.Fprintf(stderr, "a38 doctor: %v\n", err)
			return exitFailure
		}
	} else {
		doctor.Render(stdout, rep)
	}
	if !rep.OK {
		return exitFailure
	}
	return exitOK
}
