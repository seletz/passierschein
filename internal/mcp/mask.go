package mcp

import (
	"bytes"
	"cmp"
	"slices"
	"unicode/utf8"
)

// minEdgePart is the minimum length in bytes of the part of a value that
// the mask removes at a cut edge (D64).
const minEdgePart = 4

// maskValue is one delivered value and the name of its environment
// variable.
type maskValue struct {
	name  string
	value []byte
}

// marker returns the text that replaces the value.
func (v maskValue) marker() []byte { return []byte("[masked:" + v.name + "]") }

// masked is the output of one stream after the mask and the limit.
type masked struct {
	text      string
	masked    int
	truncated bool
}

// span is a part of the output that the mask replaces.
type span struct {
	start, end int
	value      *maskValue
}

// mask replaces each exact occurrence of each value in data with
// "[masked:NAME]" and keeps the end of the result up to limit bytes (D64).
//
// The steps follow D64:
//
//  1. If cut is true, data lost its beginning. Then data can start with
//     the end of a value. mask replaces the longest such end with at least
//     minEdgePart bytes.
//  2. mask replaces the exact values, the longest value first. An
//     occurrence that overlaps an earlier replacement stays.
//  3. Invalid UTF-8 becomes U+FFFD.
//  4. If the result is longer than limit, mask keeps the end. The cut does
//     not split a marker or a character. The new beginning gets step 1
//     again.
//
// The output keeps its end, so each cut edge is at the beginning. There, a
// value can only lose its beginning; its end stays. The mask compares
// bytes: it does not find an encoded or a changed value.
func mask(data []byte, cut bool, values []maskValue, limit int) masked {
	order := make([]*maskValue, 0, len(values))
	for i := range values {
		if len(values[i].value) > 0 {
			order = append(order, &values[i])
		}
	}
	slices.SortStableFunc(order, func(a, b *maskValue) int { return cmp.Compare(len(b.value), len(a.value)) })

	var spans []span
	covered := make([]bool, len(data))
	if cut {
		if v, n := edgePart(data, order); n > 0 {
			spans = append(spans, span{0, n, v})
			fill(covered, 0, n)
		}
	}
	for _, v := range order {
		for pos := 0; pos <= len(data)-len(v.value); {
			i := bytes.Index(data[pos:], v.value)
			if i < 0 {
				break
			}
			start, end := pos+i, pos+i+len(v.value)
			if slices.Contains(covered[start:end], true) {
				pos = start + 1
				continue
			}
			spans = append(spans, span{start, end, v})
			fill(covered, start, end)
			pos = end
		}
	}
	slices.SortFunc(spans, func(a, b span) int { return cmp.Compare(a.start, b.start) })

	// Build the text. markers holds the positions of the markers in out.
	out := make([]byte, 0, len(data))
	var markers []span
	pos := 0
	for _, s := range spans {
		out = append(out, bytes.ToValidUTF8(data[pos:s.start], []byte("�"))...)
		start := len(out)
		out = append(out, s.value.marker()...)
		markers = append(markers, span{start, len(out), s.value})
		pos = s.end
	}
	out = append(out, bytes.ToValidUTF8(data[pos:], []byte("�"))...)
	res := masked{masked: len(spans), truncated: cut}
	if len(out) <= limit {
		res.text = string(out)
		wipe(out)
		return res
	}

	// Keep the end. Move the cut forward out of a marker and out of a
	// character.
	res.truncated = true
	c := len(out) - limit
	inMarker := false
	for _, m := range markers {
		if c == m.start {
			inMarker = true
		}
		if c > m.start && c < m.end {
			c, inMarker = m.end, false
		}
	}
	for c < len(out) && !utf8.RuneStart(out[c]) {
		c++
	}
	kept := out[c:]
	if !inMarker {
		plain := kept
		for _, m := range markers {
			if m.start >= c {
				plain = kept[:m.start-c]
				break
			}
		}
		if v, n := edgePart(plain, order); n > 0 {
			res.masked++
			mk := v.marker()
			if len(kept)-n+len(mk) <= limit {
				kept = append(mk, kept[n:]...)
			} else {
				kept = kept[n:]
			}
		}
	}
	res.text = string(kept)
	wipe(out)
	wipe(kept)
	return res
}

// edgePart returns the value whose end is the longest beginning of data,
// and the length of that end. The end has at least minEdgePart bytes and is
// shorter than the value. It returns 0 if there is no such end.
func edgePart(data []byte, order []*maskValue) (*maskValue, int) {
	var best *maskValue
	n := 0
	for _, v := range order {
		for k := len(v.value) - 1; k >= minEdgePart && k > n; k-- {
			if bytes.HasPrefix(data, v.value[len(v.value)-k:]) {
				best, n = v, k
				break
			}
		}
	}
	return best, n
}

// fill sets b[start:end] to true.
func fill(b []bool, start, end int) {
	for i := start; i < end; i++ {
		b[i] = true
	}
}

// wipe overwrites b with zero bytes.
func wipe(b []byte) { clear(b) }

// tailBuffer holds the end of a stream, at most max bytes, and counts all
// bytes (D64). It holds at most 2*max bytes in memory. It is not safe for
// use by more than one goroutine.
type tailBuffer struct {
	max   int
	buf   []byte
	total int64
}

// newTailBuffer returns a buffer that holds the last max bytes.
func newTailBuffer(max int) *tailBuffer { return &tailBuffer{max: max} }

// Write adds p. It never fails.
func (b *tailBuffer) Write(p []byte) (int, error) {
	n := len(p)
	b.total += int64(n)
	if len(p) > b.max {
		p = p[len(p)-b.max:]
	}
	if len(b.buf)+len(p) > 2*b.max {
		// Keep the last max-len(p) bytes, then add p.
		keep := b.max - len(p)
		copy(b.buf, b.buf[len(b.buf)-keep:])
		wipe(b.buf[keep:])
		b.buf = b.buf[:keep]
	}
	if len(b.buf)+len(p) > cap(b.buf) {
		size := max(2*cap(b.buf), len(b.buf)+len(p), 4096)
		grown := make([]byte, len(b.buf), min(size, 2*b.max))
		copy(grown, b.buf)
		wipe(b.buf)
		b.buf = grown
	}
	b.buf = append(b.buf, p...)
	return n, nil
}

// Bytes returns the held end of the stream: at most max bytes.
func (b *tailBuffer) Bytes() []byte {
	if len(b.buf) > b.max {
		return b.buf[len(b.buf)-b.max:]
	}
	return b.buf
}

// Total returns the number of bytes of the stream.
func (b *tailBuffer) Total() int64 { return b.total }

// Dropped reports that the buffer lost the beginning of the stream.
func (b *tailBuffer) Dropped() bool { return b.total > int64(len(b.Bytes())) }

// Wipe overwrites the held bytes.
func (b *tailBuffer) Wipe() {
	wipe(b.buf[:cap(b.buf)])
	b.buf = b.buf[:0]
}
