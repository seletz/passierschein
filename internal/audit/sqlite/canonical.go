package sqlite

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"strconv"

	"github.com/seletz/passierschein/internal/audit"
)

// genesisHash is the PrevHash of the first entry: 64 zeros.
const genesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// canonicalVersion is the first field of the canonical form. A change of
// the form needs a new version.
const canonicalVersion = "a38-audit-v1"

// canonical returns the canonical form of an entry. The hash covers it.
//
// The form is a sequence of fields. Each field is written as an unsigned
// varint (the byte length of the field) followed by the bytes of the
// field. The sequence of the fields is fixed:
//
//  1. the version text "a38-audit-v1"
//  2. Seq, in decimal
//  3. Time as Unix nanoseconds (UTC), in decimal
//  4. Type
//  5. Verified.SessionID
//  6. Verified.Origin
//  7. Verified.Project
//  8. Verified.PeerPID, in decimal
//  9. Verified.PeerUID, in decimal
//  10. Verified.Process
//  11. Verified.Cwd
//  12. Claims.SessionName
//  13. Claims.Agent
//  14. Claims.Task
//  15. Claims.Reason
//  16. the number of references, in decimal
//     then for each reference in order: Vault, Item, Field
//     (the count comes first, so that no shift of fields is possible)
//  17. Result.Code
//  18. Result.RequestID
//  19. Result.GrantID
//  20. Result.LifetimeSeconds, in decimal
//  21. Result.Detail
//
// The length prefix makes the form unambiguous: no text of one field can
// move into the next field. The form does not contain PrevHash and Hash;
// [entryHash] adds PrevHash.
func canonical(e audit.Entry) []byte {
	var b []byte
	put := func(s string) {
		b = binary.AppendUvarint(b, uint64(len(s)))
		b = append(b, s...)
	}
	num := func(n int64) { put(strconv.FormatInt(n, 10)) }

	put(canonicalVersion)
	put(strconv.FormatUint(e.Seq, 10))
	num(e.Time.UTC().UnixNano())
	put(string(e.Type))
	put(string(e.Verified.SessionID))
	put(string(e.Verified.Origin))
	put(e.Verified.Project)
	num(int64(e.Verified.PeerPID))
	num(int64(e.Verified.PeerUID))
	put(e.Verified.Process)
	put(e.Verified.Cwd)
	put(e.Claims.SessionName)
	put(e.Claims.Agent)
	put(e.Claims.Task)
	put(e.Claims.Reason)
	num(int64(len(e.References)))
	for _, r := range e.References {
		put(r.Vault)
		put(r.Item)
		put(r.Field)
	}
	put(string(e.Result.Code))
	put(e.Result.RequestID)
	put(e.Result.GrantID)
	num(e.Result.LifetimeSeconds)
	put(e.Result.Detail)
	return b
}

// entryHash returns SHA-256 over the 32 bytes of the previous hash and the
// canonical form of the entry, as lower-case hex. An entry with a
// PrevHash that is not 64 hex digits gets a hash that no stored value
// matches.
func entryHash(e audit.Entry) string {
	prev, err := hex.DecodeString(e.PrevHash)
	if err != nil || len(prev) != sha256.Size {
		// Hash the text as it is: the result differs from each valid
		// chain, so Verify reports the entry.
		prev = []byte("invalid:" + e.PrevHash)
	}
	h := sha256.New()
	h.Write(prev)
	h.Write(canonical(e))
	return hex.EncodeToString(h.Sum(nil))
}
