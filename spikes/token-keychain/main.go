// Command kcspike is throwaway code for spike 53 (token in the macOS
// keychain). It calls Security.framework through purego, without CGO.
//
// It only handles dummy items whose service name starts with "a38-spike-".
// It reads a value from stdin, never from the arguments, and it prints only
// the length of a value that it reads.
//
// Usage:
//
//	printf %s spike-dummy-wert-1 | kcspike add  -s a38-spike-x
//	kcspike read   -s a38-spike-x [-noui]
//	kcspike acl    -s a38-spike-x
//	kcspike delete -s a38-spike-x
//	printf %s spike-dummy-wert-1 | kcspike add -dp -s a38-spike-y
package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"
)

// variant makes two builds of this program differ (other cdhash).
var variant = "a"

const (
	account   = "a38-spike"
	prefix    = "a38-spike-"
	utf8Enc   = 0x08000100
	secFwPath = "/System/Library/Frameworks/Security.framework/Security"
	cfFwPath  = "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
)

var (
	cfStringCreateWithCString  func(alloc uintptr, s string, enc uint32) uintptr
	cfStringGetCString         func(s uintptr, buf *byte, size int, enc uint32) bool
	cfDictionaryCreateMutable  func(alloc uintptr, capacity int, keyCB, valCB uintptr) uintptr
	cfDictionarySetValue       func(d, k, v uintptr)
	cfDataCreate               func(alloc uintptr, b *byte, n int) uintptr
	cfDataGetLength            func(d uintptr) int
	cfDataGetBytePtr           func(d uintptr) uintptr
	cfArrayGetCount            func(a uintptr) int
	cfArrayGetValueAtIndex     func(a uintptr, i int) uintptr
	cfGetTypeID                func(x uintptr) uintptr
	cfStringGetTypeID          func() uintptr
	cfRelease                  func(x uintptr)
	secItemAdd                 func(attrs uintptr, result uintptr) int32
	secItemCopyMatching        func(query uintptr, result *uintptr) int32
	secItemDelete              func(query uintptr) int32
	secKeychainItemCopyAccess  func(item uintptr, access *uintptr) int32
	secAccessCopyACLList       func(access uintptr, list *uintptr) int32
	secACLCopyContents         func(acl uintptr, apps *uintptr, desc *uintptr, prompt *uint16) int32
	secACLCopyAuthorizations   func(acl uintptr) uintptr
	secTrustedApplicationCopyD func(app uintptr, data *uintptr) int32
	secCopyErrorMessageString  func(status int32, reserved uintptr) uintptr

	cfDictKeyCB, cfDictValCB, cfTrue uintptr
	sym                              = map[string]uintptr{}
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: kcspike add|read|acl|delete -s a38-spike-...")
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	service := fs.String("s", "", "service name (must start with a38-spike-)")
	noUI := fs.Bool("noui", false, "set kSecUseAuthenticationUI to fail instead of a prompt")
	dp := fs.Bool("dp", false, "use the data protection keychain")
	_ = fs.Parse(os.Args[2:])
	if !strings.HasPrefix(*service, prefix) {
		fmt.Fprintln(os.Stderr, "service must start with", prefix)
		os.Exit(2)
	}
	load()
	fmt.Printf("kcspike variant %s, cmd %s, service %s, dp=%v, noui=%v\n", variant, cmd, *service, *dp, *noUI)

	q := baseQuery(*service, *dp)
	switch cmd {
	case "add":
		v, err := io.ReadAll(os.Stdin)
		if err != nil || len(bytes.TrimSpace(v)) == 0 {
			fmt.Fprintln(os.Stderr, "no value on stdin")
			os.Exit(2)
		}
		v = bytes.TrimRight(v, "\r\n")
		data := cfDataCreate(0, &v[0], len(v))
		set(q, "kSecValueData", data)
		report("SecItemAdd", secItemAdd(q, 0))
	case "read":
		set(q, "kSecReturnData", cfTrue)
		set(q, "kSecMatchLimit", sym["kSecMatchLimitOne"])
		if *noUI {
			set(q, "kSecUseAuthenticationUI", sym["kSecUseAuthenticationUIFail"])
		}
		var out uintptr
		st := secItemCopyMatching(q, &out)
		report("SecItemCopyMatching", st)
		if st == 0 {
			fmt.Printf("read ok, value length %d\n", cfDataGetLength(out))
		}
	case "acl":
		set(q, "kSecReturnRef", cfTrue)
		set(q, "kSecMatchLimit", sym["kSecMatchLimitOne"])
		set(q, "kSecUseAuthenticationUI", sym["kSecUseAuthenticationUIFail"])
		var item uintptr
		st := secItemCopyMatching(q, &item)
		report("SecItemCopyMatching(ref)", st)
		if st == 0 {
			dumpACL(item)
		}
	case "delete":
		report("SecItemDelete", secItemDelete(q))
	default:
		fmt.Fprintln(os.Stderr, "unknown command", cmd)
		os.Exit(2)
	}
}

func load() {
	cf, err := purego.Dlopen(cfFwPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	must(err)
	sec, err := purego.Dlopen(secFwPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	must(err)
	purego.RegisterLibFunc(&cfStringCreateWithCString, cf, "CFStringCreateWithCString")
	purego.RegisterLibFunc(&cfStringGetCString, cf, "CFStringGetCString")
	purego.RegisterLibFunc(&cfDictionaryCreateMutable, cf, "CFDictionaryCreateMutable")
	purego.RegisterLibFunc(&cfDictionarySetValue, cf, "CFDictionarySetValue")
	purego.RegisterLibFunc(&cfDataCreate, cf, "CFDataCreate")
	purego.RegisterLibFunc(&cfDataGetLength, cf, "CFDataGetLength")
	purego.RegisterLibFunc(&cfDataGetBytePtr, cf, "CFDataGetBytePtr")
	purego.RegisterLibFunc(&cfArrayGetCount, cf, "CFArrayGetCount")
	purego.RegisterLibFunc(&cfArrayGetValueAtIndex, cf, "CFArrayGetValueAtIndex")
	purego.RegisterLibFunc(&cfGetTypeID, cf, "CFGetTypeID")
	purego.RegisterLibFunc(&cfStringGetTypeID, cf, "CFStringGetTypeID")
	purego.RegisterLibFunc(&cfRelease, cf, "CFRelease")
	purego.RegisterLibFunc(&secItemAdd, sec, "SecItemAdd")
	purego.RegisterLibFunc(&secItemCopyMatching, sec, "SecItemCopyMatching")
	purego.RegisterLibFunc(&secItemDelete, sec, "SecItemDelete")
	purego.RegisterLibFunc(&secKeychainItemCopyAccess, sec, "SecKeychainItemCopyAccess")
	purego.RegisterLibFunc(&secAccessCopyACLList, sec, "SecAccessCopyACLList")
	purego.RegisterLibFunc(&secACLCopyContents, sec, "SecACLCopyContents")
	purego.RegisterLibFunc(&secACLCopyAuthorizations, sec, "SecACLCopyAuthorizations")
	purego.RegisterLibFunc(&secTrustedApplicationCopyD, sec, "SecTrustedApplicationCopyData")
	purego.RegisterLibFunc(&secCopyErrorMessageString, sec, "SecCopyErrorMessageString")

	cfDictKeyCB = dlsym(cf, "kCFTypeDictionaryKeyCallBacks")
	cfDictValCB = dlsym(cf, "kCFTypeDictionaryValueCallBacks")
	cfTrue = deref(dlsym(cf, "kCFBooleanTrue"))
	for _, n := range []string{
		"kSecClass", "kSecClassGenericPassword", "kSecAttrService", "kSecAttrAccount",
		"kSecValueData", "kSecReturnData", "kSecReturnRef", "kSecMatchLimit",
		"kSecMatchLimitOne", "kSecUseAuthenticationUI", "kSecUseAuthenticationUIFail",
		"kSecUseDataProtectionKeychain",
	} {
		sym[n] = deref(dlsym(sec, n))
	}
}

func baseQuery(service string, dp bool) uintptr {
	q := cfDictionaryCreateMutable(0, 0, cfDictKeyCB, cfDictValCB)
	set(q, "kSecClass", sym["kSecClassGenericPassword"])
	set(q, "kSecAttrService", cfStringCreateWithCString(0, service, utf8Enc))
	set(q, "kSecAttrAccount", cfStringCreateWithCString(0, account, utf8Enc))
	if dp {
		set(q, "kSecUseDataProtectionKeychain", cfTrue)
	}
	return q
}

func set(d uintptr, key string, v uintptr) { cfDictionarySetValue(d, sym[key], v) }

func dumpACL(item uintptr) {
	var access, list uintptr
	report("SecKeychainItemCopyAccess", secKeychainItemCopyAccess(item, &access))
	if access == 0 {
		return
	}
	report("SecAccessCopyACLList", secAccessCopyACLList(access, &list))
	for i := 0; i < cfArrayGetCount(list); i++ {
		acl := cfArrayGetValueAtIndex(list, i)
		var apps, desc uintptr
		var prompt uint16
		secACLCopyContents(acl, &apps, &desc, &prompt)
		var names []string
		aa := secACLCopyAuthorizations(acl)
		for j := 0; j < cfArrayGetCount(aa); j++ {
			names = append(names, goString(cfArrayGetValueAtIndex(aa, j)))
		}
		fmt.Printf("ACL entry %d: authorizations=%v prompt=0x%x\n", i, names, prompt)
		d := goString(desc)
		if b, err := hex.DecodeString(d); err == nil && bytes.Contains(b, []byte("<plist")) {
			fmt.Printf("  description (hex plist, partition list): %s\n", compact(string(b)))
		} else {
			fmt.Printf("  description: %q\n", d)
		}
		if apps == 0 {
			fmt.Println("  applications: any application (nil list)")
			continue
		}
		n := cfArrayGetCount(apps)
		fmt.Printf("  applications: %d\n", n)
		for j := 0; j < n; j++ {
			var data uintptr
			secTrustedApplicationCopyD(cfArrayGetValueAtIndex(apps, j), &data)
			if data != 0 {
				b := unsafe.Slice((*byte)(unsafe.Pointer(cfDataGetBytePtr(data))), cfDataGetLength(data))
				fmt.Printf("   - %s\n", strings.TrimRight(string(b), "\x00"))
			}
		}
	}
}

func compact(s string) string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "<string>") {
			out = append(out, strings.TrimSuffix(strings.TrimPrefix(l, "<string>"), "</string>"))
		}
	}
	return strings.Join(out, ", ")
}

func goString(s uintptr) string {
	if s == 0 || cfGetTypeID(s) != cfStringGetTypeID() {
		return ""
	}
	buf := make([]byte, 4096)
	if !cfStringGetCString(s, &buf[0], len(buf), utf8Enc) {
		return "?"
	}
	return string(buf[:bytes.IndexByte(buf, 0)])
}

func report(what string, st int32) {
	msg := ""
	if m := secCopyErrorMessageString(st, 0); m != 0 {
		msg = goString(m)
	}
	fmt.Printf("%s: status %d (%s)\n", what, st, msg)
}

func dlsym(h uintptr, name string) uintptr {
	p, err := purego.Dlsym(h, name)
	must(err)
	return p
}

func deref(p uintptr) uintptr { return *(*uintptr)(unsafe.Pointer(p)) }

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
