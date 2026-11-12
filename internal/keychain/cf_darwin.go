//go:build darwin

package keychain

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// This file binds the few functions of CoreFoundation and Security.framework
// that the package needs, through purego (no CGO). A Core Foundation object
// is an opaque uintptr. Each object that a Create or Copy function returns
// is released with release.

const (
	cfPath  = "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
	secPath = "/System/Library/Frameworks/Security.framework/Security"

	// kCFStringEncodingUTF8
	utf8Encoding = 0x08000100
	// kCFNumberSInt32Type
	cfNumberSInt32 = 3
	// kSecCSSigningInformation
	secCSSigningInformation = 1 << 1
	// kSecCodeSignatureAdhoc
	codeSignatureAdhoc = 0x2
	// errSecCSUnsigned
	errSecCSUnsigned = -67062
)

// cfEpoch is the reference date of CFAbsoluteTime.
var cfEpoch = time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

var (
	cfStringCreateWithCString         func(alloc uintptr, s string, enc uint32) uintptr
	cfStringGetCString                func(s uintptr, buf *byte, size int, enc uint32) bool
	cfStringGetLength                 func(s uintptr) int
	cfStringGetMaximumSizeForEncoding func(n int, enc uint32) int
	cfDictionaryCreateMutable         func(alloc uintptr, capacity int, keyCB, valCB uintptr) uintptr
	cfDictionarySetValue              func(d, k, v uintptr)
	cfDictionaryGetValue              func(d, k uintptr) uintptr
	cfDataCreate                      func(alloc uintptr, b *byte, n int) uintptr
	cfDataGetLength                   func(d uintptr) int
	cfDataGetBytePtr                  func(d uintptr) unsafe.Pointer
	cfArrayGetCount                   func(a uintptr) int
	cfArrayGetValueAtIndex            func(a uintptr, i int) uintptr
	cfNumberGetValue                  func(n uintptr, typ int, out *int32) bool
	cfDateGetAbsoluteTime             func(d uintptr) float64
	cfGetTypeID                       func(x uintptr) uintptr
	cfStringGetTypeID                 func() uintptr
	cfDataGetTypeID                   func() uintptr
	cfDateGetTypeID                   func() uintptr
	cfNumberGetTypeID                 func() uintptr
	cfRelease                         func(x uintptr)

	secItemAdd                     func(attrs uintptr, result uintptr) int32
	secItemCopyMatching            func(query uintptr, result *uintptr) int32
	secItemDelete                  func(query uintptr) int32
	secKeychainItemCopyAccess      func(item uintptr, access *uintptr) int32
	secAccessCopyACLList           func(access uintptr, list *uintptr) int32
	secACLCopyContents             func(acl uintptr, apps *uintptr, desc *uintptr, prompt *uint16) int32
	secACLCopyAuthorizations       func(acl uintptr) uintptr
	secTrustedApplicationCopyData  func(app uintptr, data *uintptr) int32
	secCopyErrorMessageString      func(status int32, reserved uintptr) uintptr
	secCodeCopySelf                func(flags uint32, code *uintptr) int32
	secCodeCopySigningInformation  func(code uintptr, flags uint32, info *uintptr) int32
	secRequirementCreateWithString func(text uintptr, flags uint32, req *uintptr) int32
	secCodeCheckValidity           func(code uintptr, flags uint32, req uintptr) int32
)

// Constants of the frameworks: pointers to Core Foundation objects, read
// once at load.
var (
	kCFBooleanTrue                  uintptr
	kCFTypeDictionaryKeyCallBacks   uintptr // address of the struct
	kCFTypeDictionaryValueCallBacks uintptr // address of the struct

	kSecClass                   uintptr
	kSecClassGenericPassword    uintptr
	kSecAttrService             uintptr
	kSecAttrAccount             uintptr
	kSecAttrLabel               uintptr
	kSecAttrCreationDate        uintptr
	kSecAttrModificationDate    uintptr
	kSecValueData               uintptr
	kSecReturnData              uintptr
	kSecReturnRef               uintptr
	kSecReturnAttributes        uintptr
	kSecMatchLimit              uintptr
	kSecMatchLimitOne           uintptr
	kSecUseAuthenticationUI     uintptr
	kSecUseAuthenticationUIFail uintptr
	kSecCodeInfoFlags           uintptr
	kSecCodeInfoTeamIdentifier  uintptr
	kSecCodeInfoIdentifier      uintptr
	kSecCodeInfoUnique          uintptr
)

var (
	loadOnce sync.Once
	errLoad  error
)

// load opens the frameworks and binds the functions once. A missing
// function makes purego panic; load turns that into an error.
func load() error {
	loadOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				errLoad = fmt.Errorf("keychain: cannot bind Security.framework: %v", r)
			}
		}()
		errLoad = bind()
	})
	return errLoad
}

func bind() error {
	cf, err := purego.Dlopen(cfPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("keychain: cannot open CoreFoundation: %w", err)
	}
	sec, err := purego.Dlopen(secPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("keychain: cannot open Security.framework: %w", err)
	}
	for name, fn := range map[string]any{
		"CFStringCreateWithCString":         &cfStringCreateWithCString,
		"CFStringGetCString":                &cfStringGetCString,
		"CFStringGetLength":                 &cfStringGetLength,
		"CFStringGetMaximumSizeForEncoding": &cfStringGetMaximumSizeForEncoding,
		"CFDictionaryCreateMutable":         &cfDictionaryCreateMutable,
		"CFDictionarySetValue":              &cfDictionarySetValue,
		"CFDictionaryGetValue":              &cfDictionaryGetValue,
		"CFDataCreate":                      &cfDataCreate,
		"CFDataGetLength":                   &cfDataGetLength,
		"CFDataGetBytePtr":                  &cfDataGetBytePtr,
		"CFArrayGetCount":                   &cfArrayGetCount,
		"CFArrayGetValueAtIndex":            &cfArrayGetValueAtIndex,
		"CFNumberGetValue":                  &cfNumberGetValue,
		"CFDateGetAbsoluteTime":             &cfDateGetAbsoluteTime,
		"CFGetTypeID":                       &cfGetTypeID,
		"CFStringGetTypeID":                 &cfStringGetTypeID,
		"CFDataGetTypeID":                   &cfDataGetTypeID,
		"CFDateGetTypeID":                   &cfDateGetTypeID,
		"CFNumberGetTypeID":                 &cfNumberGetTypeID,
		"CFRelease":                         &cfRelease,
	} {
		purego.RegisterLibFunc(fn, cf, name)
	}
	for name, fn := range map[string]any{
		"SecItemAdd":                     &secItemAdd,
		"SecItemCopyMatching":            &secItemCopyMatching,
		"SecItemDelete":                  &secItemDelete,
		"SecKeychainItemCopyAccess":      &secKeychainItemCopyAccess,
		"SecAccessCopyACLList":           &secAccessCopyACLList,
		"SecACLCopyContents":             &secACLCopyContents,
		"SecACLCopyAuthorizations":       &secACLCopyAuthorizations,
		"SecTrustedApplicationCopyData":  &secTrustedApplicationCopyData,
		"SecCopyErrorMessageString":      &secCopyErrorMessageString,
		"SecCodeCopySelf":                &secCodeCopySelf,
		"SecCodeCopySigningInformation":  &secCodeCopySigningInformation,
		"SecRequirementCreateWithString": &secRequirementCreateWithString,
		"SecCodeCheckValidity":           &secCodeCheckValidity,
	} {
		purego.RegisterLibFunc(fn, sec, name)
	}

	var err2 error
	sym := func(lib uintptr, name string) uintptr {
		p, err := purego.Dlsym(lib, name)
		if err != nil && err2 == nil {
			err2 = fmt.Errorf("keychain: missing symbol %s: %w", name, err)
		}
		return p
	}
	// value reads the object pointer that a constant symbol holds.
	value := func(lib uintptr, name string) uintptr {
		p := sym(lib, name)
		if p == 0 {
			return 0
		}
		return **(**uintptr)(unsafe.Pointer(&p))
	}
	kCFBooleanTrue = value(cf, "kCFBooleanTrue")
	kCFTypeDictionaryKeyCallBacks = sym(cf, "kCFTypeDictionaryKeyCallBacks")
	kCFTypeDictionaryValueCallBacks = sym(cf, "kCFTypeDictionaryValueCallBacks")
	for name, dst := range map[string]*uintptr{
		"kSecClass":                   &kSecClass,
		"kSecClassGenericPassword":    &kSecClassGenericPassword,
		"kSecAttrService":             &kSecAttrService,
		"kSecAttrAccount":             &kSecAttrAccount,
		"kSecAttrLabel":               &kSecAttrLabel,
		"kSecAttrCreationDate":        &kSecAttrCreationDate,
		"kSecAttrModificationDate":    &kSecAttrModificationDate,
		"kSecValueData":               &kSecValueData,
		"kSecReturnData":              &kSecReturnData,
		"kSecReturnRef":               &kSecReturnRef,
		"kSecReturnAttributes":        &kSecReturnAttributes,
		"kSecMatchLimit":              &kSecMatchLimit,
		"kSecMatchLimitOne":           &kSecMatchLimitOne,
		"kSecUseAuthenticationUI":     &kSecUseAuthenticationUI,
		"kSecUseAuthenticationUIFail": &kSecUseAuthenticationUIFail,
		"kSecCodeInfoFlags":           &kSecCodeInfoFlags,
		"kSecCodeInfoTeamIdentifier":  &kSecCodeInfoTeamIdentifier,
		"kSecCodeInfoIdentifier":      &kSecCodeInfoIdentifier,
		"kSecCodeInfoUnique":          &kSecCodeInfoUnique,
	} {
		*dst = value(sec, name)
	}
	if err2 != nil {
		return err2
	}
	if kCFBooleanTrue == 0 || kSecClass == 0 || kSecClassGenericPassword == 0 {
		return errors.New("keychain: a constant of Security.framework is missing")
	}
	return nil
}

// release releases a Core Foundation object; 0 is allowed.
func release(x uintptr) {
	if x != 0 {
		cfRelease(x)
	}
}

// cfString returns a new CFString. The caller releases it.
func cfString(s string) uintptr { return cfStringCreateWithCString(0, s, utf8Encoding) }

// goString returns the text of a CFString, or "" for another object.
func goString(s uintptr) string {
	if s == 0 || cfGetTypeID(s) != cfStringGetTypeID() {
		return ""
	}
	size := cfStringGetMaximumSizeForEncoding(cfStringGetLength(s), utf8Encoding) + 1
	buf := make([]byte, size)
	if !cfStringGetCString(s, &buf[0], len(buf), utf8Encoding) {
		return ""
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

// goBytes returns a copy of the bytes of a CFData, or nil for another
// object.
func goBytes(d uintptr) []byte {
	if d == 0 || cfGetTypeID(d) != cfDataGetTypeID() {
		return nil
	}
	n := cfDataGetLength(d)
	if n == 0 {
		return []byte{}
	}
	p := cfDataGetBytePtr(d)
	if p == nil {
		return nil
	}
	out := make([]byte, n)
	copy(out, unsafe.Slice((*byte)(p), n))
	return out
}

// goTime returns the time of a CFDate, or the zero time for another
// object.
func goTime(d uintptr) time.Time {
	if d == 0 || cfGetTypeID(d) != cfDateGetTypeID() {
		return time.Time{}
	}
	sec := cfDateGetAbsoluteTime(d)
	return cfEpoch.Add(time.Duration(sec * float64(time.Second))).UTC()
}

// goInt32 returns the value of a CFNumber.
func goInt32(n uintptr) (int32, bool) {
	if n == 0 || cfGetTypeID(n) != cfNumberGetTypeID() {
		return 0, false
	}
	var v int32
	ok := cfNumberGetValue(n, cfNumberSInt32, &v)
	return v, ok
}

// statusError returns the error of an OSStatus, or nil for success.
func statusError(op string, status int32) error {
	if status == errSecSuccess {
		return nil
	}
	msg := ""
	if m := secCopyErrorMessageString(status, 0); m != 0 {
		msg = goString(m)
		release(m)
	}
	return &StatusError{Op: op, Status: status, Message: msg, kind: kindOf(status)}
}

// dict is a mutable CFDictionary for a query of the keychain.
type dict struct{ ref uintptr }

func newDict() dict {
	return dict{cfDictionaryCreateMutable(0, 0, kCFTypeDictionaryKeyCallBacks, kCFTypeDictionaryValueCallBacks)}
}

// set sets a value; the dictionary retains it.
func (d dict) set(key, value uintptr) { cfDictionarySetValue(d.ref, key, value) }

// setString sets a new CFString and releases its own reference.
func (d dict) setString(key uintptr, value string) {
	s := cfString(value)
	d.set(key, s)
	release(s)
}

func (d dict) close() { release(d.ref) }
