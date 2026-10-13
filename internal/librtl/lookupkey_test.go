package librtl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
)

// putKeyTable writes a keyword table at addr (the manual's Figure LIB-5),
// its counted strings at text, with values 1, 2, ... in order.
func putKeyTable(t *testing.T, env *corevms.Environment, addr, text uint32, keys ...string) {
	t.Helper()

	putLongword(t, env, addr, uint32(2*len(keys)))

	for i, k := range keys {
		putLongword(t, env, addr+4+8*uint32(i), text)
		putLongword(t, env, addr+8+8*uint32(i), uint32(i+1))
		putBytes(t, env, text, string(rune(len(k)))+k)
		text += uint32(len(k)) + 1
	}
}

// TestLookupKey: exact and abbreviated matches, ambiguity, an exact match
// that is also another keyword's abbreviation, no match, and the
// outputs.
func TestLookupKey(t *testing.T) {
	env := fixture(t)
	putKeyTable(t, env, 0x5000, 0x5100, "ADD", "DELETE", "EXIT", "DEL", "EXTRA")

	cases := []struct {
		search string
		status uint32
		value  uint32
		full   string
	}{
		{"ADD", ssNormal, 1, "ADD"},
		{"A", ssNormal, 1, "ADD"},
		{"DELE", ssNormal, 2, "DELETE"},
		{"DEL", ssNormal, 4, "DEL"}, // exact, though DELETE starts with it too
		{"D", libAmbKey, 2, "DELETE"},
		{"EX", libAmbKey, 3, "EXIT"},
		{"EXT", ssNormal, 5, "EXTRA"},
		{"ADDS", libUnrKey, 0, ""},
		{"Q", libUnrKey, 0, ""},
	}

	for _, c := range cases {
		putBytes(t, env, 0x3000, c.search)
		putDesc(t, env, 0x1000, uint16(len(c.search)), dscClassS, 0x3000)
		putDesc(t, env, 0x1008, 10, dscClassS, 0x4000)
		putBytes(t, env, 0x4000, "**********")
		putLongword(t, env, 0x2000, 0xEEEE)
		putLongword(t, env, 0x2004, 0xEEEE)

		got := call(t, env, "LIB$LOOKUP_KEY", 0x1000, 0x5000, 0x2000, 0x1008, 0x2004)
		if got != c.status {
			t.Errorf("%q: status %#x, want %#x", c.search, got, c.status)

			continue
		}

		if c.status == libUnrKey {
			if longword(t, env, 0x2000) != 0xEEEE || bytesAt(t, env, 0x4000, 1) != "*" {
				t.Errorf("%q: outputs written for LIB$_UNRKEY", c.search)
			}

			continue
		}

		value, full, length := longword(t, env, 0x2000), bytesAt(t, env, 0x4000, 10), longword(t, env, 0x2004)&0xFFFF
		if value != c.value || full[:len(c.full)] != c.full || length != uint32(len(c.full)) {
			t.Errorf("%q: value %d, keyword %q, length %d; want %d, %q, %d", c.search, value, full, length, c.value, c.full, len(c.full))
		}
	}
}

// TestLookupKeyOutputs: the outputs are optional; a keyword cut short is
// still SS$_NORMAL, its length the characters written; a dynamic keyword
// string gets the keyword's length.
func TestLookupKeyOutputs(t *testing.T) {
	env := fixture(t)
	putKeyTable(t, env, 0x5000, 0x5100, "DELETE")
	putBytes(t, env, 0x3000, "DE")
	putDesc(t, env, 0x1000, 2, dscClassS, 0x3000)

	if got := call(t, env, "LIB$LOOKUP_KEY", 0x1000, 0x5000); got != ssNormal {
		t.Errorf("no outputs: %#x", got)
	}

	putDesc(t, env, 0x1008, 3, dscClassS, 0x4000)

	// Cut short, with no status saying so (VMS).
	if got := call(t, env, "LIB$LOOKUP_KEY", 0x1000, 0x5000, 0, 0x1008, 0x2004); got != ssNormal || longword(t, env, 0x2004)&0xFFFF != 3 || bytesAt(t, env, 0x4000, 3) != "DEL" {
		t.Errorf("into S(3): %#x, length %d, %q", got, longword(t, env, 0x2004)&0xFFFF, bytesAt(t, env, 0x4000, 3))
	}

	putDesc(t, env, dynDesc, 0, dscClassD, 0)
	call(t, env, "LIB$LOOKUP_KEY", 0x1000, 0x5000, 0, dynDesc)

	if length, _, ptr := descriptorAt(t, env, dynDesc); length != 6 || bytesAt(t, env, ptr, 6) != "DELETE" {
		t.Errorf("into D: length %d", length)
	}
}

// TestLookupKeyInvalid: too few arguments are LIB$_INVARG; a table's
// count isn't checked (VMS reads count/2 entries: 3 is one).
func TestLookupKeyInvalid(t *testing.T) {
	env := fixture(t)
	putBytes(t, env, 0x3000, "A")
	putDesc(t, env, 0x1000, 1, dscClassS, 0x3000)
	putKeyTable(t, env, 0x5000, 0x5100, "ADD")
	putLongword(t, env, 0x5000, 3)

	if got := call(t, env, "LIB$LOOKUP_KEY", 0x1000); got != libInvArg {
		t.Errorf("one argument: %#x, want LIB$_INVARG", got)
	}

	if got := call(t, env, "LIB$LOOKUP_KEY", 0x1000, 0x5000); got != ssNormal {
		t.Errorf("an odd count, 3: %#x, want SS$_NORMAL (one entry, ADD)", got)
	}
}
