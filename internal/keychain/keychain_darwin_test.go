//go:build darwin

package keychain

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/seletz/passierschein/internal/token"
)

// Dummy values. They are no tokens; no output may contain them.
const (
	dummy1 = "test-dummy-wert-1-a38"
	dummy2 = "test-dummy-wert-2-a38"
)

// created lists the service names of the items of this test binary. The
// test binary creates, reads and deletes each item itself: a different
// program that reads an item causes a dialog of macOS (spike, V4).
var (
	createdMu sync.Mutex
	created   []string
)

func TestMain(m *testing.M) {
	code := m.Run()
	// Final check: no item of this run is left.
	createdMu.Lock()
	defer createdMu.Unlock()
	for _, service := range created {
		s := Store{Service: service, Account: "a38-test"}
		info, err := s.Describe()
		if err == nil && !info.Exists {
			continue
		}
		_ = s.Delete()
		fmt.Fprintf(os.Stderr, "keychain test item %s is left (describe: %v); deleted it\n", service, err)
		code = 1
	}
	os.Exit(code)
}

// testStore returns a store with a new item name "a38-test-<random>". The
// item is deleted at the end of the test, also on a failure. If the
// environment variable A38_KEYCHAIN_TEST_LOG names a file, the name is
// appended to it, so that a human can check afterwards with
// "security find-generic-password -s <name>" that no item is left.
func testStore(t *testing.T) Store {
	t.Helper()
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	s := Store{Service: TestPrefix + hex.EncodeToString(b[:]), Account: "a38-test", Label: "a38 test item (dummy value)"}
	createdMu.Lock()
	created = append(created, s.Service)
	createdMu.Unlock()
	if path := os.Getenv("A38_KEYCHAIN_TEST_LOG"); path != "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fmt.Fprintln(f, s.Service)
		_ = f.Close()
	}
	t.Cleanup(func() {
		if err := s.Delete(); err != nil && !errors.Is(err, token.ErrNotFound) {
			t.Errorf("cleanup: %v", err)
		}
		if info, err := s.Describe(); err != nil || info.Exists {
			t.Errorf("cleanup: the item %s is left (%v)", s.Service, err)
		}
	})
	return s
}

// put stores v. On a CI runner whose login keychain does not accept a
// new item without interaction (an error, or no answer within 30 s) it
// skips the test; locally a failure fails the test.
func put(t *testing.T, s Store, v string, replace bool) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- s.Put(token.New(v), replace) }()
	var err error
	select {
	case err = <-done:
	case <-time.After(30 * time.Second):
		err = errors.New("the keychain did not answer within 30 s")
	}
	if err == nil {
		return
	}
	if os.Getenv("CI") != "" {
		t.Skipf("the login keychain of this CI runner does not accept a new item without interaction: %v", err)
	}
	t.Fatalf("Put: %v", err)
}

// noValue fails if text contains a dummy value.
func noValue(t *testing.T, what, text string) {
	t.Helper()
	for _, v := range []string{dummy1, dummy2, "test-dummy"} {
		if strings.Contains(text, v) {
			t.Errorf("%s contains the value: %q", what, text)
		}
	}
}

func TestStoreLifecycle(t *testing.T) {
	// D51, issue 59: create, read, refuse a second create, replace,
	// describe and delete one item, all in this test binary.
	s := testStore(t)

	info, err := s.Describe()
	if err != nil || info.Exists {
		t.Fatalf("Describe before Put = %+v, %v", info, err)
	}
	if _, err := s.Token(); !errors.Is(err, token.ErrNotFound) {
		t.Fatalf("Token before Put: %v, want ErrNotFound", err)
	}
	if err := s.Delete(); !errors.Is(err, token.ErrNotFound) {
		t.Fatalf("Delete before Put: %v, want ErrNotFound", err)
	}

	start := time.Now().Add(-2 * time.Second)
	put(t, s, dummy1, false)
	got, err := s.Token()
	if err != nil || got.Reveal() != dummy1 {
		t.Fatalf("Token after Put: %v", err)
	}

	err = s.Put(token.New(dummy2), false)
	if !errors.Is(err, token.ErrExists) {
		t.Fatalf("second Put without replace: %v, want ErrExists", err)
	}
	noValue(t, "error", err.Error())
	if got, _ := s.Token(); got.Reveal() != dummy1 {
		t.Error("Put without replace changed the value")
	}

	put(t, s, dummy2, true)
	if got, err := s.Token(); err != nil || got.Reveal() != dummy2 {
		t.Fatalf("Token after replace: %v", err)
	}

	info, err = s.Describe()
	if err != nil || !info.Exists {
		t.Fatalf("Describe after Put = %+v, %v", info, err)
	}
	if info.Name != s.Name() {
		t.Errorf("Name = %q", info.Name)
	}
	if info.Created.Before(start) || info.Modified.Before(start) || info.Modified.After(time.Now().Add(2*time.Second)) {
		t.Errorf("dates: created %v, modified %v, start %v", info.Created, info.Modified, start)
	}
	if info.AccessListError != "" {
		t.Errorf("access list: %s", info.AccessListError)
	}
	exe, _ := os.Executable()
	exe, _ = filepath.EvalSymlinks(exe)
	if len(info.Trusted) != 1 || info.Trusted[0] != exe {
		t.Errorf("trusted programs = %q, want only this test binary %q", info.Trusted, exe)
	}
	sig, err := s.Signature()
	if err != nil {
		t.Fatal(err)
	}
	if sig.CDHash != "" {
		want := "cdhash:" + sig.CDHash
		if len(info.Partitions) == 0 || !strings.EqualFold(info.Partitions[0], want) {
			t.Errorf("partitions = %q, want %q", info.Partitions, want)
		}
		if !info.ReadableWithoutDialog {
			t.Error("the creating program is not sure to read without a dialog")
		}
	}
	noValue(t, "description", fmt.Sprintf("%+v %v", info, info))

	if err := s.Delete(); err != nil {
		t.Fatal(err)
	}
	if info, err := s.Describe(); err != nil || info.Exists {
		t.Fatalf("Describe after Delete = %+v, %v", info, err)
	}
	if _, err := s.Token(); !errors.Is(err, token.ErrNotFound) {
		t.Fatalf("Token after Delete: %v, want ErrNotFound", err)
	}
}

func TestPutEmpty(t *testing.T) {
	s := testStore(t)
	if err := s.Put(token.Token{}, false); err == nil {
		t.Fatal("Put stored an empty token")
	}
	if info, err := s.Describe(); err != nil || info.Exists {
		t.Fatalf("Describe = %+v, %v", info, err)
	}
}

func TestProductItemRefusedInTests(t *testing.T) {
	// Issue 59: a test never touches the item of the human.
	p := Product()
	if p.Service != ProductService || p.Account != ProductAccount || strings.HasPrefix(p.Service, TestPrefix) {
		t.Fatalf("product item = %+v", p)
	}
	if _, err := p.Token(); err == nil || !strings.Contains(err.Error(), "must not use the product item") {
		t.Errorf("Token: %v", err)
	}
	if err := p.Put(token.New(dummy1), true); err == nil {
		t.Error("Put on the product item works in a test")
	}
	if err := p.Delete(); err == nil {
		t.Error("Delete on the product item works in a test")
	}
	if _, err := p.Describe(); err == nil {
		t.Error("Describe on the product item works in a test")
	}
	if _, err := (Store{}).Describe(); err == nil {
		t.Error("an empty name works")
	}
}

func TestSignature(t *testing.T) {
	// The Go linker signs each darwin/arm64 binary ad-hoc (Go issue 42684).
	sig, err := Store{}.Signature()
	if err != nil {
		t.Fatal(err)
	}
	exe, _ := os.Executable()
	exe, _ = filepath.EvalSymlinks(exe)
	if sig.Path != exe {
		t.Errorf("path = %q, want %q", sig.Path, exe)
	}
	if runtime.GOARCH == "arm64" {
		if sig.Kind != token.SignatureAdhoc {
			t.Errorf("kind = %q, want ad-hoc", sig.Kind)
		}
		if len(sig.CDHash) != 40 || strings.Trim(sig.CDHash, "0123456789abcdef") != "" {
			t.Errorf("cdhash = %q", sig.CDHash)
		}
	}
	if sig.Stable() {
		t.Error("a test binary has a stable signature")
	}
}

func TestReadableBy(t *testing.T) {
	// Fail closed: only the exact path and the exact hash allow a read
	// without a dialog.
	sig := token.Signature{Path: "/x/a38", CDHash: "abcd"}
	good := token.StoreInfo{Exists: true, Trusted: []string{"/x/a38"}, Partitions: []string{"cdhash:ABCD"}}
	tests := []struct {
		name string
		info func(i token.StoreInfo) token.StoreInfo
		sig  token.Signature
		want bool
	}{
		{"same build", func(i token.StoreInfo) token.StoreInfo { return i }, sig, true},
		{"other build", func(i token.StoreInfo) token.StoreInfo { return i }, token.Signature{Path: "/x/a38", CDHash: "ef01"}, false},
		{"other path", func(i token.StoreInfo) token.StoreInfo { i.Trusted = []string{"/usr/bin/security"}; return i }, sig, false},
		{"team partition", func(i token.StoreInfo) token.StoreInfo { i.Partitions = []string{"teamid:ABCDE12345"}; return i }, sig, false},
		{"apple tool", func(i token.StoreInfo) token.StoreInfo { i.Partitions = []string{"apple-tool:"}; return i }, sig, false},
		{"access list unknown", func(i token.StoreInfo) token.StoreInfo { i.AccessListError = "x"; return i }, sig, false},
		{"no hash", func(i token.StoreInfo) token.StoreInfo { return i }, token.Signature{Path: "/x/a38"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := readableBy(tt.info(good), tt.sig); got != tt.want {
				t.Errorf("readableBy = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPartitions(t *testing.T) {
	plist := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>Partitions</key><array>
	<string>cdhash:cbc3b178</string>
	<string>teamid:ABCDE12345</string></array></dict></plist>`
	got := partitions(hex.EncodeToString([]byte(plist)))
	if len(got) != 2 || got[0] != "cdhash:cbc3b178" || got[1] != "teamid:ABCDE12345" {
		t.Errorf("partitions = %q", got)
	}
	if got := partitions("not hex"); got != nil {
		t.Errorf("partitions of no hex = %q", got)
	}
}

func TestStatusError(t *testing.T) {
	tests := []struct {
		status int32
		kind   error
	}{
		{errSecItemNotFound, token.ErrNotFound},
		{errSecDuplicateItem, token.ErrExists},
		{errSecInteractionNotAllowed, token.ErrInteraction},
		{errSecAuthFailed, token.ErrInteraction},
		{errSecUserCanceled, token.ErrInteraction},
		{-50, nil},
	}
	if err := load(); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		err := statusError("SecItemCopyMatching", tt.status)
		if err == nil {
			t.Fatalf("%d: no error", tt.status)
		}
		if tt.kind != nil && !errors.Is(err, tt.kind) {
			t.Errorf("%d: %v is not %v", tt.status, err, tt.kind)
		}
		for _, k := range []error{token.ErrNotFound, token.ErrExists, token.ErrInteraction} {
			if tt.kind == nil && errors.Is(err, k) {
				t.Errorf("%d: %v is %v", tt.status, err, k)
			}
		}
		if !strings.Contains(err.Error(), fmt.Sprint(tt.status)) {
			t.Errorf("%d: text %q has no status", tt.status, err)
		}
	}
	if statusError("x", 0) != nil {
		t.Error("success is an error")
	}
}
