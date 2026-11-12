package doctor

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/config"
	"github.com/seletz/passierschein/internal/token"
	"github.com/seletz/passierschein/internal/tokenfile"
)

// tokenSourceCheck checks the source of the token of the 1Password backend
// (D51, issue 59): the keychain item (default) or the token file. It never
// reads the token.
//
// Decision: the unstable signature of a38 (ad-hoc or unsigned, G5) is a
// note, not a finding. It is the known state of stage 1; a finding would
// make the doctor fail on each machine until the signed release.
type tokenSourceCheck struct {
	env  Env
	goos string
}

func (tokenSourceCheck) Name() string { return "1password token" }

func (c tokenSourceCheck) Run(_ context.Context, st *State) []Result {
	if st.Config == nil {
		return nil
	}
	if c.goos != "darwin" {
		return []Result{ok(c.Name(), "not used: the 1Password backend is part of the macOS build only")}
	}
	kind := ""
	if st.Status != nil && st.Status.Backend != nil {
		kind = st.Status.Backend.Kind
	}
	if kind == string(backend.KindFake) {
		return []Result{ok(c.Name(), "not used: the daemon runs with the backend fake")}
	}
	needed := kind == string(backend.KindOnePassword)
	op := st.Config.Backend.OnePassword
	if op.TokenSource == config.TokenSourceFile {
		return c.file(op.TokenFile, needed)
	}
	return c.keychain(op.TokenFile, needed)
}

// file checks the token file: it exists, it is a regular file of the
// user, and its mode is 0600. It never reads the content.
func (c tokenSourceCheck) file(path string, needed bool) []Result {
	fi, err := c.env.Lstat(path)
	switch {
	case isMissing(err) && needed:
		return []Result{finding(c.Name(), "the token file of the 1Password backend does not exist",
			path+" does not exist (token_source: file); the running daemon holds the token in memory, but its next start fails",
			"write the token of the service account to "+path+" with the mode 0600, or use the keychain: remove token_source: file and run a38 config onepassword-token set (how-to: 1Password-Backend einrichten)")}
	case isMissing(err):
		return []Result{ok(c.Name(), "source file: %s does not exist; only a38 serve --backend 1password needs it", path)}
	case err != nil:
		return []Result{finding(c.Name(), "the token file cannot be read", errText(err), "check the path "+path)}
	}
	if err := tokenfile.CheckInfo(fi, os.Getuid()); err != nil {
		return []Result{finding(c.Name(), "the token file is not safe", path+": "+errText(err),
			"make it a regular file of your user and run: chmod 600 "+path+"; then restart the daemon")}
	}
	return []Result{ok(c.Name(), "source file: %s, regular file of the user, mode 0600 (content not read; fallback for development and CI: any process of the user can read it)", path)}
}

// keychain checks the keychain item without its value, the signature of
// a38 and a token file that is left.
func (c tokenSourceCheck) keychain(path string, needed bool) []Result {
	store := c.env.TokenStore()
	if store == nil {
		return []Result{finding(c.Name(), "the keychain is not available", "this build of a38 has no keychain",
			"use the macOS build of a38, or set backend.onepassword.token_source: file")}
	}
	info, err := store.Describe()
	if err != nil {
		return []Result{finding(c.Name(), "the keychain item cannot be read", errText(err),
			"run a38 config onepassword-token status; if the login keychain is locked, unlock it")}
	}
	var out []Result
	sig, sigErr := store.Signature()
	switch {
	case !info.Exists && needed:
		out = append(out, finding(c.Name(), "the keychain has no token of the 1Password backend",
			"no item "+store.Name()+" (token_source: keychain); the running daemon holds the token in memory, but its next start fails",
			"run a38 config onepassword-token set and paste the token of the service account (how-to: 1Password-Backend einrichten)"))
	case !info.Exists:
		out = append(out, ok(c.Name(), "source keychain: no item %s; only a38 serve --backend 1password needs it; store the token with a38 config onepassword-token set", store.Name()))
	default:
		out = append(out, ok(c.Name(), "source keychain: item %s exists, changed %s, trusted: %s (value not read)",
			store.Name(), stampOf(info.Modified), strings.Join(info.Trusted, ", ")))
		if info.AccessListError != "" {
			out = append(out, note(c.Name(), "the access list of the item cannot be read: %s", info.AccessListError))
		}
		if sigErr == nil {
			var others []string
			for _, p := range info.Trusted {
				if p != sig.Path {
					others = append(others, p)
				}
			}
			if len(others) > 0 {
				out = append(out, finding(c.Name(), "the keychain item trusts a program other than this a38",
					"trusted: "+strings.Join(others, ", ")+"; that program reads the token without a dialog",
					"run a38 config onepassword-token delete, then a38 config onepassword-token set; never confirm a macOS dialog for the a38 item that you did not cause yourself"))
			} else if info.AccessListError == "" && !info.ReadableWithoutDialog {
				out = append(out, note(c.Name(), "this build of a38 did not store the item; the next start of the daemon shows a macOS dialog. Store the token again with a38 config onepassword-token set --replace instead of confirming the dialog"))
			}
		}
	}
	switch {
	case sigErr != nil:
		out = append(out, note(c.Name(), "the signature of a38 cannot be read: %v", sigErr))
	case !sig.Stable():
		out = append(out, note(c.Name(), "signature of a38: %s; %s", sig.Kind, token.UnstableSignatureWarning))
	default:
		out = append(out, ok(c.Name(), "signature of a38: Developer ID, team %s", sig.TeamID))
	}
	if _, err := c.env.Lstat(path); err == nil {
		out = append(out, finding(c.Name(), "a token file is left although the source is the keychain",
			path+" exists; any process of the user can read it",
			"check with a38 config onepassword-token status that the keychain item exists and that the daemon starts with it, then delete the file: rm "+path))
	}
	return out
}

// stampOf formats a time of the keychain item.
func stampOf(t time.Time) string {
	if t.IsZero() {
		return "at an unknown time"
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}
