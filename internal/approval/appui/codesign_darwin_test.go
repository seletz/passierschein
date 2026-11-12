//go:build darwin

package appui_test

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seletz/passierschein/internal/approval/appui"
)

// adHocCopy copies the fake app into dir and signs the copy ad-hoc with
// the hardened runtime. Only throwaway files are signed.
func adHocCopy(t *testing.T, dir, name string) string {
	t.Helper()
	src, err := os.Open(helperPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Close() }()
	path := filepath.Join(dir, name)
	dst, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		t.Fatal(err)
	}
	if err := dst.Close(); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("/usr/bin/codesign", "--force", "--sign", "-", "--options", "runtime",
		"--identifier", "beispiel.adhoc", path).CombinedOutput(); err != nil {
		t.Fatalf("codesign: %v: %s", err, out)
	}
	return path
}

// G5 (part): the real codesign on throwaway files.
func TestSystemCodeSigner(t *testing.T) {
	dir := shortDir(t)
	signed := adHocCopy(t, dir, "adhoc")
	script := filepath.Join(dir, "script")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := appui.SystemCodeSigner()
	ctx := context.Background()

	got, err := s.Describe(ctx, signed)
	if err != nil {
		t.Fatalf("Describe ad-hoc: %v", err)
	}
	want := appui.CodeSignature{Signed: true, AdHoc: true, Runtime: true, Identifier: "beispiel.adhoc"}
	if got != want {
		t.Errorf("Describe ad-hoc: %+v, want %+v", got, want)
	}

	got, err = s.Describe(ctx, script)
	if err != nil || got.Signed {
		t.Errorf("Describe unsigned: %+v, %v; want not signed, no error", got, err)
	}

	if _, err := s.Describe(ctx, filepath.Join(dir, "missing")); err == nil {
		t.Error("Describe of a missing file: nil error")
	}
	if _, err := s.Describe(ctx, "adhoc"); err == nil {
		t.Error("Describe of a relative path: nil error")
	}

	// An ad-hoc signature never satisfies the Developer ID requirement.
	err = s.VerifyTeam(ctx, signed, "ABCDE12345")
	if err == nil || !strings.Contains(err.Error(), "codesign") {
		t.Errorf("VerifyTeam ad-hoc: %v, want a codesign error", err)
	}
	if err := s.VerifyTeam(ctx, signed, `X" or true`); err == nil {
		t.Error("VerifyTeam with a bad team ID: nil error")
	}
}

// G5 (part): an ad-hoc signed a38 switches the check off, with the real
// codesign; an app that is not signed at all still starts.
func TestSignatureVerifierAdHocSelf(t *testing.T) {
	dir := shortDir(t)
	self := adHocCopy(t, dir, "a38")
	script := filepath.Join(dir, "Passierschein")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	v := appui.NewSignatureVerifier(context.Background(), appui.SignatureOptions{Self: self})
	if err := v.Verify(context.Background(), script); err != nil {
		t.Fatalf("Verify: %v, want nil (check off)", err)
	}
}
