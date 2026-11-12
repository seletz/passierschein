package appui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
)

// CodeSignature is what the system reports about the code signature of a
// file.
type CodeSignature struct {
	// Signed is false when the file has no code signature at all.
	Signed bool
	// AdHoc is true for an ad-hoc signature (no certificate). The Go linker
	// signs each darwin/arm64 binary ad-hoc.
	AdHoc bool
	// Runtime is true when the hardened runtime flag is set.
	Runtime bool
	// Identifier is the signing identifier.
	Identifier string
	// TeamID is the team identifier of the certificate. It is empty for an
	// ad-hoc signature.
	TeamID string
}

// CodeSigner reads and checks code signatures. On macOS the default runs
// /usr/bin/codesign ([SystemCodeSigner]); the tests use fakes.
type CodeSigner interface {
	// Describe returns the signature of the file at path. A file without
	// a signature is no error: the result has Signed false.
	Describe(ctx context.Context, path string) (CodeSignature, error)
	// VerifyTeam checks that the signature of path is valid (strict, with
	// nested code) and that it satisfies the requirement of a Developer
	// ID signature of the team: the chain ends at an Apple root, the
	// certificate is a Developer ID Application certificate and its
	// organizational unit is the team.
	VerifyTeam(ctx context.Context, path, team string) error
}

// teamIDPattern is the form of an Apple team ID. The team ID goes into
// the requirement text of codesign, so only this form is allowed.
var teamIDPattern = regexp.MustCompile(`^[A-Z0-9]{10}$`)

// SignatureOptions holds the settings for [NewSignatureVerifier].
type SignatureOptions struct {
	// Self is the real path of the running a38 binary (links resolved).
	Self string
	// Signer reads the signatures. Nil means [SystemCodeSigner]; if the
	// platform has none, the check is off.
	Signer CodeSigner
	// Logger receives one line at the start that says whether the check
	// is on. Nil means no log.
	Logger *slog.Logger
}

// NewSignatureVerifier returns the [Verifier] for the code signature of
// the approval app (G5). It reads the signature of a38 itself once:
//
//   - a38 has a Developer ID signature (a certificate with a team ID, not
//     ad-hoc): the verifier accepts only an app with a valid Developer ID
//     signature of the same team and the hardened runtime;
//   - a38 is ad-hoc signed or unsigned (development, CI, a local
//     install): the check is off, so nothing changes for these builds;
//   - the signature of a38 cannot be read, or its team ID has an
//     unexpected form: each start of the app fails (fail closed, D35).
//
// The function logs one line with the result. It does not log per
// request.
func NewSignatureVerifier(ctx context.Context, opts SignatureOptions) Verifier {
	log := opts.Logger
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	signer := opts.Signer
	if signer == nil {
		signer = SystemCodeSigner()
	}
	if signer == nil {
		log.Info("the signature check of the approval app is off: this platform has no codesign")
		return NoopVerifier{}
	}
	if opts.Self == "" {
		log.Error("the approval app is refused: the path of a38 is unknown, so its signature is unknown")
		return refuseVerifier{errors.New("the path of a38 is unknown")}
	}
	self, err := signer.Describe(ctx, opts.Self)
	if err != nil {
		log.Error("the approval app is refused: cannot read the signature of a38", "path", opts.Self, "error", err)
		return refuseVerifier{fmt.Errorf("cannot read the signature of a38: %w", err)}
	}
	if !self.Signed || self.AdHoc || self.TeamID == "" {
		log.Info("the signature check of the approval app is off: a38 has no Developer ID signature",
			"signed", self.Signed, "adhoc", self.AdHoc)
		return NoopVerifier{}
	}
	if !teamIDPattern.MatchString(self.TeamID) {
		log.Error("the approval app is refused: the team ID of a38 has an unexpected form")
		return refuseVerifier{errors.New("the team ID of a38 has an unexpected form")}
	}
	log.Info("the approval app must carry a Developer ID signature of the team of a38", "team", self.TeamID)
	return teamVerifier{signer: signer, team: self.TeamID}
}

// refuseVerifier refuses each app.
type refuseVerifier struct{ err error }

func (r refuseVerifier) Verify(context.Context, string) error { return r.err }

// teamVerifier accepts only an app signed by the team.
type teamVerifier struct {
	signer CodeSigner
	team   string
}

func (t teamVerifier) Verify(ctx context.Context, path string) error {
	if err := t.signer.VerifyTeam(ctx, path, t.team); err != nil {
		return fmt.Errorf("the signature of the approval app is not valid for team %s: %w", t.team, err)
	}
	sig, err := t.signer.Describe(ctx, path)
	switch {
	case err != nil:
		return fmt.Errorf("cannot read the signature of the approval app: %w", err)
	case !sig.Signed:
		return errors.New("the approval app is not signed")
	case sig.AdHoc:
		return errors.New("the approval app is only ad-hoc signed")
	case sig.TeamID != t.team:
		return fmt.Errorf("the approval app is signed by team %s, a38 by team %s", sig.TeamID, t.team)
	case !sig.Runtime:
		return errors.New("the approval app has no hardened runtime")
	}
	return nil
}

// parseCodesignOutput reads the output of "codesign -d --verbose=2".
func parseCodesignOutput(out string) CodeSignature {
	var s CodeSignature
	for line := range strings.Lines(out) {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Identifier="):
			s.Identifier = strings.TrimPrefix(line, "Identifier=")
			s.Signed = true
		case line == "Signature=adhoc":
			s.AdHoc = true
		case strings.HasPrefix(line, "TeamIdentifier="):
			if v := strings.TrimPrefix(line, "TeamIdentifier="); v != "not set" {
				s.TeamID = v
			}
		case strings.HasPrefix(line, "CodeDirectory "):
			s.Signed = true
			for _, f := range strings.Fields(line) {
				v, ok := strings.CutPrefix(f, "flags=")
				if !ok {
					continue
				}
				_, list, _ := strings.Cut(v, "(")
				for flag := range strings.SplitSeq(strings.TrimSuffix(list, ")"), ",") {
					switch flag {
					case "runtime":
						s.Runtime = true
					case "adhoc":
						s.AdHoc = true
					}
				}
			}
		}
	}
	return s
}
