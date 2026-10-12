package librtl

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
)

// The character string tests' layout: descriptors at descA..descE, their
// strings at textA..textE, and a 256-byte table at table.
const (
	descA = 0x1000
	descB = 0x1010
	descC = 0x1020
	descD = 0x1030
	descE = 0x1040
	textA = 0x1100
	textB = 0x1200
	textC = 0x1300
	textD = 0x1400
	textE = 0x1500
	table = 0x2000
	byteA = 0x1080 // a byte or word argument
	byteB = 0x1084
)

// putUpcaseTable writes a translation table at table that makes lowercase
// letters uppercase and leaves every other byte as it is.
func putUpcaseTable(t *testing.T, env *corevms.Environment) {
	t.Helper()

	var b strings.Builder

	for i := range 256 {
		b.WriteByte(strings.ToUpper(string(rune(i)))[0])
	}

	putBytes(t, env, table, b.String())
}

func TestLibLoccSkpc(t *testing.T) {
	for _, c := range []struct {
		name, char, src string
		want            uint32
	}{
		// The manual's examples (LIB-259, LIB-377).
		{"LIB$LOCC", "DAY", "ONE DAY AT A TIME", 5},
		{"LIB$LOCC", "X", "ABCDEFGHIJKLMNOPQRSTUVWXYZ", 24},
		{"LIB$SKPC", " ", " ABC", 2},
		{"LIB$SKPC", "A", "AAA", 0},
		{"LIB$LOCC", "Q", "ABC", 0},
		{"LIB$LOCC", "A", "", 0},
		{"LIB$SKPC", "A", "", 0},
		{"LIB$LOCC", "C", "ABC", 3},
	} {
		env := fixture(t)
		putDescriptor(t, env, descA, textA, c.char)
		putDescriptor(t, env, descB, textB, c.src)

		if got := call(t, env, c.name, descA, descB); got != c.want {
			t.Errorf("%s(%q, %q) = %d, want %d", c.name, c.char, c.src, got, c.want)
		}
	}
}

// TestLibLoccEmptyCharacter: the character string's length isn't
// checked, so an empty one still gives the byte at its address.
func TestLibLoccEmptyCharacter(t *testing.T) {
	env := fixture(t)
	putDescriptor(t, env, descA, textA, "B")
	putDesc(t, env, descA, 0, dscClassS, textA)
	putDescriptor(t, env, descB, textB, "ABC")

	if got := call(t, env, "LIB$LOCC", descA, descB); got != 2 {
		t.Errorf("LIB$LOCC of an empty string at %q = %d, want 2", "B", got)
	}
}

func TestLibMatchc(t *testing.T) {
	for _, c := range []struct {
		sub, src string
		want     uint32
	}{
		{"DAY", "ONE DAY AT A TIME", 5},
		{"TIME", "ONE DAY AT A TIME", 14},
		{"AAB", "AAAAB", 3}, // a partial match first
		{"DAYS", "ONE DAY", 0},
		{"X", "", 0},
		{"", "ABC", 1},
		{"ONE DAY", "ONE DAY", 1},
	} {
		env := fixture(t)
		putDescriptor(t, env, descA, textA, c.sub)
		putDescriptor(t, env, descB, textB, c.src)

		if got := call(t, env, "LIB$MATCHC", descA, descB); got != c.want {
			t.Errorf("LIB$MATCHC(%q, %q) = %d, want %d", c.sub, c.src, got, c.want)
		}
	}
}

func TestLibScancSpanc(t *testing.T) {
	// The table: digits have bit 0, letters bit 1.
	env := fixture(t)

	for ch := range 256 {
		var v byte

		switch {
		case ch >= '0' && ch <= '9':
			v = 1
		case ch >= 'A' && ch <= 'Z':
			v = 2
		}

		putBytes(t, env, table+uint32(ch), string([]byte{v}))
	}

	for _, c := range []struct {
		name, src string
		mask      byte
		want      uint32
	}{
		{"LIB$SCANC", "ABC12", 1, 4},  // the first digit
		{"LIB$SCANC", "ABC", 1, 0},    // none
		{"LIB$SPANC", "123AB", 1, 4},  // the first non-digit
		{"LIB$SPANC", "ABCDE", 2, 0},  // all letters
		{"LIB$SCANC", "--X--", 3, 3},  // a letter or a digit
		{"LIB$SPANC", "", 1, 0},
	} {
		putDescriptor(t, env, descA, textA, c.src)
		putBytes(t, env, byteA, string([]byte{c.mask}))

		if got := call(t, env, c.name, descA, table, byteA); got != c.want {
			t.Errorf("%s(%q, mask %d) = %d, want %d", c.name, c.src, c.mask, got, c.want)
		}
	}
}

func TestLibMovc3Movc5(t *testing.T) {
	env := fixture(t)
	mem, c := env.Memory(), env.CPU()
	putBytes(t, env, textA, "ABCDEFGH")

	// LIB$MOVC3 of 5 bytes, overlapping its own source by two.
	if err := mem.StoreWord(c, byteA, 5); err != nil {
		t.Fatal(err)
	}

	if got := call(t, env, "LIB$MOVC3", byteA, textA, textA+2); got != 0 {
		t.Errorf("LIB$MOVC3 = %d, want 0", got)
	}

	if got := bytesAt(t, env, textA, 8); got != "ABABCDEH" {
		t.Errorf("after LIB$MOVC3: %q, want %q", got, "ABABCDEH")
	}

	// LIB$MOVC5: 3 bytes into 6, filled with '*'; then 6 into 2.
	putBytes(t, env, textB, "xyz")
	putBytes(t, env, byteB, "*")

	for _, step := range []struct {
		srcLen, dstLen uint16
		want           string
		left           uint32
	}{
		{3, 6, "xyz***", 0},
		{6, 2, "xyz***", 4}, // only two bytes written
	} {
		if err := mem.StoreWord(c, byteA, step.srcLen); err != nil {
			t.Fatal(err)
		}

		if err := mem.StoreWord(c, byteA+2, step.dstLen); err != nil {
			t.Fatal(err)
		}

		if got := call(t, env, "LIB$MOVC5", byteA, textB, byteB, byteA+2, textC); got != step.left {
			t.Errorf("LIB$MOVC5 %d into %d = %d, want %d", step.srcLen, step.dstLen, got, step.left)
		}

		if got := bytesAt(t, env, textC, 6); got != step.want {
			t.Errorf("LIB$MOVC5 %d into %d: %q, want %q", step.srcLen, step.dstLen, got, step.want)
		}
	}
}

func TestLibMovtc(t *testing.T) {
	for _, c := range []struct {
		src    string
		dst    int // the destination's length
		want   string
		status uint32
	}{
		{"hello", 8, "HELLO***", ssNormal},
		{"hello, world", 5, "HELLO", libStrTru},
		{"", 3, "***", ssNormal},
	} {
		env := fixture(t)
		putUpcaseTable(t, env)
		putDescriptor(t, env, descA, textA, c.src)
		putDescriptor(t, env, descB, textB, "*")
		putDesc(t, env, descC, 256, dscClassS, table)
		putDescriptor(t, env, descD, textD, strings.Repeat("?", c.dst))

		if got := call(t, env, "LIB$MOVTC", descA, descB, descC, descD); got != c.status {
			t.Errorf("LIB$MOVTC(%q) = %#x, want %#x", c.src, got, c.status)
		}

		if got := bytesAt(t, env, textD, c.dst); got != c.want {
			t.Errorf("LIB$MOVTC(%q) wrote %q, want %q", c.src, got, c.want)
		}
	}
}

// TestLibMovtcDynamic: a dynamic destination gets the source's length.
func TestLibMovtcDynamic(t *testing.T) {
	env := fixture(t)
	putUpcaseTable(t, env)
	putDescriptor(t, env, descA, textA, "abc")
	putDescriptor(t, env, descB, textB, "*")
	putDescriptor(t, env, descC, table, "")
	putDesc(t, env, descD, 0, dscClassD, 0)

	if got := call(t, env, "LIB$MOVTC", descA, descB, descC, descD); got != ssNormal {
		t.Errorf("LIB$MOVTC = %#x, want SS$_NORMAL", got)
	}

	if s, err := readString(env, descD); err != nil || s != "ABC" {
		t.Errorf("destination %q, %v; want %q", s, err, "ABC")
	}
}

func TestLibMovtuc(t *testing.T) {
	for _, c := range []struct {
		src  string
		fill string // "" for none
		dst  int
		want string
		at   uint32
	}{
		{"abc.def", "*", 8, "ABC*****", 4},
		{"abc.def", "", 8, "ABC?????", 4},
		{"abcdef", "*", 8, "ABCDEF**", 0},
		{"abc.def", "*", 2, "AB", 0}, // the destination runs out first
		{".", "*", 3, "***", 1},
	} {
		env := fixture(t)
		putUpcaseTable(t, env)
		putDescriptor(t, env, descA, textA, c.src)
		putDescriptor(t, env, descB, textB, ".")
		putDescriptor(t, env, descC, table, "")
		putDescriptor(t, env, descD, textD, strings.Repeat("?", c.dst))
		argv := []uint32{descA, descB, descC, descD}

		if c.fill != "" {
			putDescriptor(t, env, descE, textE, c.fill)
			argv = append(argv, descE)
		}

		if got := call(t, env, "LIB$MOVTUC", argv...); got != c.at {
			t.Errorf("LIB$MOVTUC(%q, fill %q) = %d, want %d", c.src, c.fill, got, c.at)
		}

		if got := bytesAt(t, env, textD, c.dst); got != c.want {
			t.Errorf("LIB$MOVTUC(%q, fill %q) wrote %q, want %q", c.src, c.fill, got, c.want)
		}
	}
}

// TestLibMovtucInvalid: an invalid destination descriptor is -1.
func TestLibMovtucInvalid(t *testing.T) {
	env := fixture(t)
	putDescriptor(t, env, descA, textA, "abc")
	putDescriptor(t, env, descB, textB, ".")
	putDescriptor(t, env, descC, table, "")
	putDesc(t, env, descD, 3, 3, textD) // class V

	if got := call(t, env, "LIB$MOVTUC", descA, descB, descC, descD); got != movtucFailed {
		t.Errorf("LIB$MOVTUC = %#x, want -1", got)
	}
}
