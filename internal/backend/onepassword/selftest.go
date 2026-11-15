package onepassword

import (
	"context"
	"errors"
	"strings"
)

// errSelfTest is the error of [SelfTest] when the core of the SDK ran but
// did not answer as expected.
var errSelfTest = errors.New("the WebAssembly core of the 1Password SDK did not reject the invalid token as expected")

// SelfTest loads the WebAssembly core of the 1Password SDK in this
// process and gives it an invalid token (T30, D59). It reads no token, no
// file and no keychain item, and it calls no network: the core rejects
// the token first. It returns nil if the core ran and rejected the token.
//
// If the hardened runtime of macOS forbids the code of the core, the
// kernel kills the process (SIGKILL, code signing) and SelfTest does not
// return. So run it in a separate process: "a38 selftest sdk-core".
//
// On a platform without the SDK it returns an error.
func SelfTest(ctx context.Context) error {
	err := selfTestConnect(ctx)
	if errors.Is(err, errNotAvailable) {
		return err
	}
	return checkSelfTestAnswer(err)
}

// checkSelfTestAnswer maps the answer of the SDK to the invalid token. The
// error of the SDK stays out of the result: the package never passes on
// the text of an SDK error.
func checkSelfTestAnswer(err error) error {
	switch {
	case err == nil:
		return errors.New("the WebAssembly core of the 1Password SDK accepted the invalid token")
	case strings.Contains(err.Error(), "service account token"):
		return nil
	}
	return errSelfTest
}
