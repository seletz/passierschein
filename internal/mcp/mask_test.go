package mcp

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf8"
)

// Neutral stand-ins for secret values.
const (
	fakeValue = "beispiel-wert-4711"
	fakeInner = "wert-47"
)

func values(pairs ...string) []maskValue {
	var out []maskValue
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, maskValue{name: pairs[i], value: []byte(pairs[i+1])})
	}
	return out
}

func TestMaskEachExactValue(t *testing.T) {
	// T33: each exact occurrence is masked, in stdout and in stderr; masked
	// counts all of them.
	vals := values("DB_PASSWORD", fakeValue)
	stdout := mask([]byte("eins "+fakeValue+" zwei "+fakeValue+"\n"), false, vals, 64*1024)
	stderr := mask([]byte("fehler: "+fakeValue), false, vals, 64*1024)
	if want := "eins [masked:DB_PASSWORD] zwei [masked:DB_PASSWORD]\n"; stdout.text != want {
		t.Errorf("stdout = %q, want %q", stdout.text, want)
	}
	if want := "fehler: [masked:DB_PASSWORD]"; stderr.text != want {
		t.Errorf("stderr = %q, want %q", stderr.text, want)
	}
	if got := stdout.masked + stderr.masked; got != 3 {
		t.Errorf("masked = %d, want 3", got)
	}
	if stdout.truncated || stderr.truncated {
		t.Error("output is truncated, want complete")
	}
}

func TestMaskLongestValueFirst(t *testing.T) {
	// T33: one value is inside the other; the longer value is masked
	// first, and no rest of a value stays.
	tests := []struct {
		name string
		vals []maskValue
	}{
		{"short value first", values("TEIL", fakeInner, "DB_PASSWORD", fakeValue)},
		{"long value first", values("DB_PASSWORD", fakeValue, "TEIL", fakeInner)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mask([]byte("a="+fakeValue+" b="+fakeInner+"."), false, tt.vals, 64*1024)
			if want := "a=[masked:DB_PASSWORD] b=[masked:TEIL]."; got.text != want {
				t.Errorf("text = %q, want %q", got.text, want)
			}
			if got.masked != 2 {
				t.Errorf("masked = %d, want 2", got.masked)
			}
		})
	}
}

func TestMaskDoesNotMatchInsideMarker(t *testing.T) {
	// T33: a value that is part of the marker text does not match the
	// marker of a different value.
	got := mask([]byte("x "+fakeValue+" y"), false, values("DB_PASSWORD", fakeValue, "KURZ", "masked"), 64*1024)
	if want := "x [masked:DB_PASSWORD] y"; got.text != want {
		t.Errorf("text = %q, want %q", got.text, want)
	}
}

func TestMaskAtTheCutEdge(t *testing.T) {
	// T33: the output is above the limit, and the cut edge is in the
	// middle of a value. No beginning and no end of the value with at
	// least 4 bytes stays.
	const limit = 1024
	vals := values("DB_PASSWORD", fakeValue)
	for cut := 1; cut < len(fakeValue); cut++ {
		// The value starts so that the cut of the raw output is cut bytes
		// into the value.
		pre := strings.Repeat("a", 100)
		tail := strings.Repeat("b", limit-(len(fakeValue)-cut))
		data := pre + fakeValue + tail
		got := mask([]byte(data), false, vals, limit)
		if !got.truncated {
			t.Fatalf("cut %d: not truncated", cut)
		}
		if len(got.text) > limit {
			t.Errorf("cut %d: length %d, want at most %d", cut, len(got.text), limit)
		}
		checkNoPart(t, got.text, fakeValue)
	}
}

func TestMaskAtTheEdgeOfTheHeldOutput(t *testing.T) {
	// T33: the held output lost its beginning (more than 4 MiB), so it
	// starts with the end of a value. Each end with at least 4 bytes is
	// masked; a shorter end stays.
	vals := values("DB_PASSWORD", fakeValue)
	for k := 1; k < len(fakeValue); k++ {
		data := fakeValue[len(fakeValue)-k:] + " rest"
		got := mask([]byte(data), true, vals, 64*1024)
		if !got.truncated {
			t.Errorf("end %d: not truncated", k)
		}
		if k >= 4 {
			if want := "[masked:DB_PASSWORD] rest"; got.text != want {
				t.Errorf("end %d: text = %q, want %q", k, got.text, want)
			}
			if got.masked != 1 {
				t.Errorf("end %d: masked = %d, want 1", k, got.masked)
			}
			continue
		}
		if got.text != data || got.masked != 0 {
			t.Errorf("end %d: text = %q masked %d, want unchanged", k, got.text, got.masked)
		}
	}
}

func TestMaskKeepsEncodedValue(t *testing.T) {
	// T33: the mask finds only the exact value. The Base64 form stays: the
	// test documents the limit of the mask (D64).
	enc := base64.StdEncoding.EncodeToString([]byte(fakeValue))
	got := mask([]byte("b64="+enc), false, values("DB_PASSWORD", fakeValue), 64*1024)
	if got.text != "b64="+enc || got.masked != 0 {
		t.Errorf("text = %q masked %d, want the Base64 form unchanged", got.text, got.masked)
	}
}

func TestMaskKeepsTheEnd(t *testing.T) {
	// T34 (part): above the limit, the output is the end with exactly the
	// limit.
	const limit = 64 * 1024
	data := make([]byte, 200*1024)
	for i := range data {
		data[i] = byte('a' + i%26)
	}
	got := mask(data, false, nil, limit)
	if !got.truncated || got.text != string(data[len(data)-limit:]) {
		t.Errorf("truncated %v, length %d: want the last %d bytes", got.truncated, len(got.text), limit)
	}
}

func TestMaskReplacesInvalidUTF8(t *testing.T) {
	// D64: invalid UTF-8 becomes U+FFFD; the cut does not split a
	// character, and the text stays inside the limit.
	got := mask([]byte("a\xffb "+fakeValue), false, values("DB_PASSWORD", fakeValue), 1024)
	if want := "a�b [masked:DB_PASSWORD]"; got.text != want {
		t.Errorf("text = %q, want %q", got.text, want)
	}
	// "ä" has two bytes; a cut after the first byte moves forward.
	long := strings.Repeat("ä", 1000)
	got = mask([]byte(long), false, nil, 1025)
	if !utf8.ValidString(got.text) || len(got.text) > 1025 || strings.ContainsRune(got.text, utf8.RuneError) {
		t.Errorf("text has length %d, valid %v", len(got.text), utf8.ValidString(got.text))
	}
}

func TestTailBuffer(t *testing.T) {
	// D64: the buffer holds at most its maximum and counts all bytes.
	b := newTailBuffer(10)
	for _, p := range []string{"0123", "4567", "89ab", "cdefghijklmnop", "q"} {
		if _, err := b.Write([]byte(p)); err != nil {
			t.Fatal(err)
		}
	}
	if got := string(b.Bytes()); got != "hijklmnopq" {
		t.Errorf("held = %q, want the last 10 bytes", got)
	}
	if b.Total() != 27 || !b.Dropped() {
		t.Errorf("total %d dropped %v, want 27 true", b.Total(), b.Dropped())
	}
	b.Wipe()
	if len(b.Bytes()) != 0 {
		t.Error("Wipe left bytes")
	}
}

// checkNoPart fails if text holds a beginning or an end of value with at
// least 4 bytes.
func checkNoPart(t *testing.T, text, value string) {
	t.Helper()
	for k := 4; k <= len(value); k++ {
		if strings.Contains(text, value[:k]) || strings.Contains(text, value[len(value)-k:]) {
			t.Errorf("text holds %d bytes of the value", k)
			return
		}
	}
}
