package broker

import (
	"context"

	"github.com/seletz/passierschein/internal/audit"
	"github.com/seletz/passierschein/internal/backend"
	"github.com/seletz/passierschein/internal/protocol"
)

// deliver reads and returns the values of each reference that is not
// rejected. rejected maps a reference with the names of the backend to its
// code; deliver adds the references that no live grant covers any more.
func (b *Broker) deliver(ctx context.Context, c *call, rejected map[protocol.Reference]protocol.Code) (*protocol.Credentials, *protocol.Error) {
	refs, e := b.authorize(c, rejected)
	if e != nil {
		return nil, e
	}
	values, e := b.read(ctx, c, refs)
	if e != nil {
		return nil, e
	}
	return answer(c, rejected, values)
}

// authorize finds the live grant for each reference that is not rejected
// and writes the event "delivery" for each grant. It does both under the
// lock of the broker: a revocation or the end of the session comes either
// before the check, and nothing is delivered, or after the event. This is
// the last gate before a value is read.
func (b *Broker) authorize(c *call, rejected map[protocol.Reference]protocol.Code) ([]protocol.Reference, *protocol.Error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.expireLocked()
	// Gate of the delivery path (D48).
	if e := b.faults.Check(); e != nil {
		return nil, e
	}
	now := b.clock.Now()
	sid := c.s.Facts.SessionID
	var order []string
	groups := map[string][]protocol.Reference{}
	var ended []protocol.Reference
	seen := map[protocol.Reference]bool{}
	for _, ref := range c.req.References {
		canon := c.canon[ref]
		if _, no := rejected[canon]; no || seen[canon] {
			continue
		}
		seen[canon] = true
		g, ok := b.grants.Cover(sid, canon, now)
		if !ok {
			// The grant ended after the approval: no value.
			rejected[canon] = protocol.CodeDenied
			ended = append(ended, canon)
			continue
		}
		if _, known := groups[g.ID]; !known {
			order = append(order, g.ID)
		}
		groups[g.ID] = append(groups[g.ID], canon)
	}
	if len(ended) > 0 {
		if e := b.record(b.event(c, audit.EventDenial, ended, audit.Result{
			Code: protocol.CodeDenied, Detail: "no live grant covers the entries",
		})); e != nil {
			return nil, e
		}
	}
	var refs []protocol.Reference
	for _, id := range order {
		// D36: the event is written before the value is read.
		if e := b.record(b.event(c, audit.EventDelivery, groups[id], audit.Result{GrantID: id})); e != nil {
			return nil, e
		}
		refs = append(refs, groups[id]...)
	}
	return refs, nil
}

// read reads the values from the backend. The broker keeps no value after
// the answer (D34). A failure ends the whole request; values that were
// read are dropped.
func (b *Broker) read(ctx context.Context, c *call, refs []protocol.Reference) (map[protocol.Reference]protocol.Secret, *protocol.Error) {
	values := make(map[protocol.Reference]protocol.Secret, len(refs))
	for _, ref := range refs {
		v, err := b.be.Read(ctx, ref)
		if err != nil {
			code := backend.Code(err)
			b.log.Warn("the backend failed to read a value", "code", string(code))
			return nil, b.reject(c, audit.EventError, refs, protocol.Errorf(code, "the backend failed: %s", code))
		}
		values[ref] = v
	}
	return values, nil
}

// answer builds the answer in the sequence of the request. If no entry has
// a value and all entries have the same code, the answer is that error.
func answer(c *call, rejected map[protocol.Reference]protocol.Code, values map[protocol.Reference]protocol.Secret) (*protocol.Credentials, *protocol.Error) {
	resp := &protocol.Credentials{ID: c.req.ID}
	codes := map[protocol.Code]bool{}
	for _, ref := range c.req.References {
		canon := c.canon[ref]
		entry := protocol.CredentialEntry{Reference: ref}
		if v, ok := values[canon]; ok {
			entry.Value = &v
		} else {
			code, ok := rejected[canon]
			if !ok {
				// Fail closed: an entry without a value has a code.
				code = protocol.CodeDenied
			}
			entry.Error = entryError(code)
			codes[code] = true
		}
		resp.Entries = append(resp.Entries, entry)
	}
	if len(values) == 0 && len(codes) == 1 {
		for code := range codes {
			return nil, entryError(code)
		}
	}
	return resp, nil
}

// entryError returns the error of one entry.
func entryError(code protocol.Code) *protocol.Error {
	switch code {
	case protocol.CodeBundleTooLarge:
		return protocol.Errorf(code, "the bundle of the session is full; ask for the credential again in a later request")
	case protocol.CodeDenied:
		return protocol.Errorf(code, "the human did not approve the credential")
	default:
		return protocol.Errorf(code, "the credential is not delivered")
	}
}
