//go:build darwin

package keychain

import (
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/seletz/passierschein/internal/token"
)

// Names of the authorizations of an access control list entry.
const (
	aclDecrypt   = "ACLAuthorizationDecrypt"
	aclAny       = "ACLAuthorizationAny"
	aclPartition = "ACLAuthorizationPartitionID"
)

// developerIDRequirement is met only by code that a Developer ID
// Application certificate signs (TN3127).
const developerIDRequirement = `anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] exists and certificate leaf[field.1.2.840.113635.100.6.1.13] exists`

// query returns a query for the item of s. The caller closes it.
func (s Store) query() dict {
	q := newDict()
	q.set(kSecClass, kSecClassGenericPassword)
	q.setString(kSecAttrService, s.Service)
	q.setString(kSecAttrAccount, s.Account)
	return q
}

// matchOne returns a query for one item that must never show a dialog for
// authentication. Note: this does not stop the dialog of the access list
// for a program that the item does not trust (spike, V4).
func (s Store) matchOne() dict {
	q := s.query()
	q.set(kSecMatchLimit, kSecMatchLimitOne)
	q.set(kSecUseAuthenticationUI, kSecUseAuthenticationUIFail)
	return q
}

func (s Store) prepare() error {
	if err := s.check(); err != nil {
		return err
	}
	return load()
}

// Token reads the value of the item. Call it only from the program that
// created the item; a different program causes a dialog of macOS.
func (s Store) Token() (token.Token, error) {
	if err := s.prepare(); err != nil {
		return token.Token{}, err
	}
	q := s.matchOne()
	defer q.close()
	q.set(kSecReturnData, kCFBooleanTrue)
	var out uintptr
	if err := statusError("SecItemCopyMatching", secItemCopyMatching(q.ref, &out)); err != nil {
		return token.Token{}, err
	}
	defer release(out)
	data := goBytes(out)
	if data == nil {
		return token.Token{}, errors.New("keychain: the item has no data")
	}
	t := token.New(string(data))
	clear(data)
	return t, nil
}

// Put stores t in the item. The keychain trusts only the running program
// for the new item. With replace, Put deletes an existing item first, so
// that the new item trusts this build; without replace, an existing item
// gives an error that wraps [token.ErrExists].
func (s Store) Put(t token.Token, replace bool) error {
	if err := s.prepare(); err != nil {
		return err
	}
	if t.Empty() {
		return errors.New("keychain: the token is empty")
	}
	info, err := s.Describe()
	if err != nil {
		return err
	}
	if info.Exists {
		if !replace {
			return &StatusError{Op: "Put", Status: errSecDuplicateItem, kind: token.ErrExists}
		}
		if err := s.Delete(); err != nil && !errors.Is(err, token.ErrNotFound) {
			return err
		}
	}
	q := s.query()
	defer q.close()
	if s.Label != "" {
		q.setString(kSecAttrLabel, s.Label)
	}
	value := []byte(t.Reveal())
	data := cfDataCreate(0, &value[0], len(value))
	clear(value)
	if data == 0 {
		return errors.New("keychain: cannot create the data of the item")
	}
	q.set(kSecValueData, data)
	release(data)
	return statusError("SecItemAdd", secItemAdd(q.ref, 0))
}

// Delete removes the item.
func (s Store) Delete() error {
	if err := s.prepare(); err != nil {
		return err
	}
	q := s.query()
	defer q.close()
	return statusError("SecItemDelete", secItemDelete(q.ref))
}

// Describe returns the metadata and the access list of the item without
// its value. It never reads the value; reading the attributes and the
// access list showed no dialog in the spike (V3, V9, V10).
func (s Store) Describe() (token.StoreInfo, error) {
	info := token.StoreInfo{Name: s.Name()}
	if err := s.prepare(); err != nil {
		return info, err
	}
	q := s.matchOne()
	q.set(kSecReturnAttributes, kCFBooleanTrue)
	var attrs uintptr
	err := statusError("SecItemCopyMatching", secItemCopyMatching(q.ref, &attrs))
	q.close()
	if errors.Is(err, token.ErrNotFound) {
		return info, nil
	}
	if err != nil {
		return info, err
	}
	info.Exists = true
	info.Created = goTime(cfDictionaryGetValue(attrs, kSecAttrCreationDate))
	info.Modified = goTime(cfDictionaryGetValue(attrs, kSecAttrModificationDate))
	release(attrs)

	if err := s.readAccess(&info); err != nil {
		info.AccessListError = err.Error()
	}
	if sig, err := s.Signature(); err == nil {
		info.ReadableWithoutDialog = readableBy(info, sig)
	}
	return info, nil
}

// readableBy reports whether sig is sure to read the item without a
// dialog: the access list trusts the path of the program and the
// partition list names the hash of exactly this build. Fail closed: any
// doubt gives false.
func readableBy(info token.StoreInfo, sig token.Signature) bool {
	if sig.Path == "" || sig.CDHash == "" || info.AccessListError != "" {
		return false
	}
	if !slices.Contains(info.Trusted, sig.Path) {
		return false
	}
	want := "cdhash:" + strings.ToLower(sig.CDHash)
	for _, p := range info.Partitions {
		if strings.ToLower(p) == want {
			return true
		}
	}
	return false
}

// readAccess reads the access list of the item: the trusted programs of
// the entry that allows to decrypt, and the partition list.
func (s Store) readAccess(info *token.StoreInfo) error {
	q := s.matchOne()
	q.set(kSecReturnRef, kCFBooleanTrue)
	var item uintptr
	err := statusError("SecItemCopyMatching", secItemCopyMatching(q.ref, &item))
	q.close()
	if err != nil {
		return err
	}
	defer release(item)
	var access, list uintptr
	if err := statusError("SecKeychainItemCopyAccess", secKeychainItemCopyAccess(item, &access)); err != nil {
		return err
	}
	defer release(access)
	if err := statusError("SecAccessCopyACLList", secAccessCopyACLList(access, &list)); err != nil {
		return err
	}
	defer release(list)
	for i := range cfArrayGetCount(list) {
		acl := cfArrayGetValueAtIndex(list, i)
		auths := authorizations(acl)
		var apps, desc uintptr
		var prompt uint16
		if err := statusError("SecACLCopyContents", secACLCopyContents(acl, &apps, &desc, &prompt)); err != nil {
			return err
		}
		switch {
		case slices.Contains(auths, aclPartition):
			info.Partitions = append(info.Partitions, partitions(goString(desc))...)
		case slices.Contains(auths, aclDecrypt) || slices.Contains(auths, aclAny):
			info.Trusted = append(info.Trusted, programs(apps)...)
		}
		release(apps)
		release(desc)
	}
	return nil
}

// authorizations returns the names of the authorizations of an entry.
func authorizations(acl uintptr) []string {
	arr := secACLCopyAuthorizations(acl)
	if arr == 0 {
		return nil
	}
	defer release(arr)
	var out []string
	for j := range cfArrayGetCount(arr) {
		out = append(out, goString(cfArrayGetValueAtIndex(arr, j)))
	}
	return out
}

// programs returns the paths of the trusted applications. No list means:
// each program.
func programs(apps uintptr) []string {
	if apps == 0 {
		return []string{token.AnyProgram}
	}
	var out []string
	for j := range cfArrayGetCount(apps) {
		var data uintptr
		if secTrustedApplicationCopyData(cfArrayGetValueAtIndex(apps, j), &data) != errSecSuccess || data == 0 {
			out = append(out, "(unknown program)")
			continue
		}
		out = append(out, strings.TrimRight(string(goBytes(data)), "\x00"))
		release(data)
	}
	return out
}

var plistString = regexp.MustCompile(`<string>([^<]*)</string>`)

// partitions reads the partition list from the description of the entry:
// a property list in hex.
func partitions(desc string) []string {
	b, err := hex.DecodeString(desc)
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range plistString.FindAllSubmatch(b, -1) {
		out = append(out, string(m[1]))
	}
	return out
}

// Signature reads the code signature of the running program with the API
// of Security.framework. It never asks the human.
func (s Store) Signature() (token.Signature, error) {
	sig := token.Signature{Kind: token.SignatureUnknown}
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		sig.Path = exe
	}
	if err := load(); err != nil {
		return sig, err
	}
	var code uintptr
	if err := statusError("SecCodeCopySelf", secCodeCopySelf(0, &code)); err != nil {
		return sig, err
	}
	defer release(code)
	var info uintptr
	st := secCodeCopySigningInformation(code, secCSSigningInformation, &info)
	if st == errSecCSUnsigned {
		sig.Kind = token.SignatureNone
		return sig, nil
	}
	if err := statusError("SecCodeCopySigningInformation", st); err != nil {
		return sig, err
	}
	defer release(info)
	sig.Identifier = goString(cfDictionaryGetValue(info, kSecCodeInfoIdentifier))
	sig.TeamID = goString(cfDictionaryGetValue(info, kSecCodeInfoTeamIdentifier))
	if cd := goBytes(cfDictionaryGetValue(info, kSecCodeInfoUnique)); len(cd) > 0 {
		sig.CDHash = hex.EncodeToString(cd)
	}
	flags, ok := goInt32(cfDictionaryGetValue(info, kSecCodeInfoFlags))
	switch {
	case !ok && sig.CDHash == "":
		sig.Kind = token.SignatureNone
	case ok && flags&codeSignatureAdhoc != 0:
		sig.Kind = token.SignatureAdhoc
	case developerID(code):
		sig.Kind = token.SignatureDeveloperID
	case sig.TeamID != "":
		sig.Kind = token.SignatureOther
	default:
		sig.Kind = token.SignatureUnknown
	}
	return sig, nil
}

// developerID reports whether the code meets the requirement of a
// Developer ID signature.
func developerID(code uintptr) bool {
	text := cfString(developerIDRequirement)
	defer release(text)
	var req uintptr
	if secRequirementCreateWithString(text, 0, &req) != errSecSuccess || req == 0 {
		return false
	}
	defer release(req)
	return secCodeCheckValidity(code, 0, req) == errSecSuccess
}
