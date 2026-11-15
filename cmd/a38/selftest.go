package main

import (
	"context"
	"fmt"
	"io"
	"time"
)

// selftestTimeout limits the self-test of the SDK core.
const selftestTimeout = 60 * time.Second

const selftestUsage = `Usage:
  a38 selftest sdk-core   Run the WebAssembly core of the 1Password SDK once (macOS)

sdk-core loads the core of the 1Password SDK in this process and gives it an
invalid token and the version of this a38, as the daemon does. It reads no token, no file and no keychain item, and it calls
no network. Exit code 0: the core runs in this a38. If the code signature of
this a38 forbids the core, macOS kills the process (SIGKILL, code signing),
and the exit status is 137 in a shell.
`

// runSelftest runs a check of this a38 binary (T30, D59).
func runSelftest(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, selftestUsage)
		return exitUsage
	}
	switch args[0] {
	case "--help", "-h", "help":
		_, _ = fmt.Fprint(stdout, selftestUsage)
		return exitOK
	case "sdk-core":
	default:
		_, _ = fmt.Fprintf(stderr, "a38 selftest: unknown check %q.\n\n%s", args[0], selftestUsage)
		return exitUsage
	}
	if len(args) > 1 {
		_, _ = fmt.Fprintf(stderr, "a38 selftest sdk-core: unexpected argument %q.\n", args[1])
		return exitUsage
	}
	ctx, cancel := context.WithTimeout(context.Background(), selftestTimeout)
	defer cancel()
	start := time.Now()
	if err := selftestSDKCore(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, "a38 selftest sdk-core: %v\n", err)
		return exitFailure
	}
	_, _ = fmt.Fprintf(stdout, "a38 selftest sdk-core: the WebAssembly core of the 1Password SDK runs in this a38 (%.2f s)\n", time.Since(start).Seconds())
	return exitOK
}
