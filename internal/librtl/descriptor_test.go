package librtl

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/corevms"
)

// unmapped is an address past the fixture's 1MB of memory: reading or
// writing it fails, as an access violation would.
const unmapped = 0x00200000

// putDesc writes a descriptor's first two longwords at addr: length,
// DSC$K_DTYPE_T, class, and pointer.
func putDesc(t *testing.T, env *corevms.Environment, addr uint32, length uint16, class byte, ptr uint32) {
	t.Helper()

	putLongword(t, env, addr, uint32(length)|0x0E<<16|uint32(class)<<24)
	putLongword(t, env, addr+4, ptr)
}

// putBytes writes s at addr.
func putBytes(t *testing.T, env *corevms.Environment, addr uint32, s string) {
	t.Helper()

	for i := 0; i < len(s); i++ {
		if err := env.Memory().StoreByte(env.CPU(), addr+uint32(i), s[i]); err != nil {
			t.Fatal(err)
		}
	}
}

// bytesAt reads n bytes at addr.
func bytesAt(t *testing.T, env *corevms.Environment, addr uint32, n int) string {
	t.Helper()

	buf := make([]byte, n)

	for i := range buf {
		ch, err := env.Memory().LoadByte(env.CPU(), addr+uint32(i))
		if err != nil {
			t.Fatal(err)
		}

		buf[i] = ch
	}

	return string(buf)
}

// TestReadString reads a string from each class the RTL accepts.
func TestReadString(t *testing.T) {
	env := fixture(t)
	putBytes(t, env, 0x2000, "ABCDEFGH")

	cases := []struct {
		name  string
		class byte
		setup func(addr uint32)
		want  string
	}{
		{"Z", dscClassZ, nil, "ABCDE"},
		{"S", dscClassS, nil, "ABCDE"},
		{"D", dscClassD, nil, "ABCDE"},
		{"SD", dscClassSD, nil, "ABCDE"},
		{"A uses ARSIZE", dscClassA, func(addr uint32) { putLongword(t, env, addr+12, 7) }, "ABCDEFG"},
		{"NCA uses ARSIZE", dscClassNCA, func(addr uint32) { putLongword(t, env, addr+12, 3) }, "ABC"},
	}

	for i, c := range cases {
		addr := 0x1000 + uint32(i)*0x20
		putDesc(t, env, addr, 5, c.class, 0x2000)

		if c.setup != nil {
			c.setup(addr)
		}

		if got, err := readString(env, addr); err != nil || got != c.want {
			t.Errorf("%s: readString = %q, %v; want %q", c.name, got, err, c.want)
		}
	}
}

// TestReadString_varying reads CURLEN bytes after the length word.
func TestReadString_varying(t *testing.T) {
	env := fixture(t)
	putDesc(t, env, 0x1000, 20, dscClassVS, 0x2000) // MAXSTRLEN 20
	putBytes(t, env, 0x2000, "\x03\x00XYZW")         // CURLEN 3

	if got, err := readString(env, 0x1000); err != nil || got != "XYZ" {
		t.Errorf("readString = %q, %v; want XYZ", got, err)
	}
}

// TestReadString_errors: an unsupported class, an oversized array, and
// memory that can't be read.
func TestReadString_errors(t *testing.T) {
	env := fixture(t)

	putDesc(t, env, 0x1000, 5, 3, 0x2000) // DSC$K_CLASS_V
	if _, err := readString(env, 0x1000); !errors.Is(err, errInvStrDes) {
		t.Errorf("class V: err = %v, want errInvStrDes", err)
	}

	putDesc(t, env, 0x1020, 5, dscClassA, 0x2000)
	putLongword(t, env, 0x1020+12, 65536)

	if _, err := readString(env, 0x1020); !errors.Is(err, errInvStrDes) {
		t.Errorf("ARSIZE 65536: err = %v, want errInvStrDes", err)
	}

	putDesc(t, env, 0x1040, 5, dscClassS, unmapped)

	_, err := readString(env, 0x1040)
	if err == nil || errors.Is(err, errInvStrDes) {
		t.Errorf("unreadable text: err = %v, want an access violation", err)
	}

	if _, err := readString(env, unmapped); err == nil {
		t.Error("unreadable descriptor: no error")
	}
}

// TestWriteString_fixed pads and truncates S, Z, SD, A, and NCA strings.
func TestWriteString_fixed(t *testing.T) {
	env := fixture(t)

	for i, class := range []byte{dscClassZ, dscClassS, dscClassSD} {
		addr, text := 0x1000+uint32(i)*0x20, 0x3000+uint32(i)*0x20
		putDesc(t, env, addr, 6, class, text)

		stored, truncated, err := writeString(env, addr, "HI")
		if err != nil || stored != 2 || truncated || bytesAt(t, env, text, 6) != "HI    " {
			t.Errorf("class %d short: %d, %v, %v, %q", class, stored, truncated, err, bytesAt(t, env, text, 6))
		}

		stored, truncated, err = writeString(env, addr, "TOOLONGTEXT")
		if err != nil || stored != 6 || !truncated || bytesAt(t, env, text, 6) != "TOOLON" {
			t.Errorf("class %d long: %d, %v, %v, %q", class, stored, truncated, err, bytesAt(t, env, text, 6))
		}
	}

	putDesc(t, env, 0x1100, 1, dscClassA, 0x3100)
	putLongword(t, env, 0x1100+12, 4)
	putBytes(t, env, 0x3104, "!")

	stored, truncated, err := writeString(env, 0x1100, "ABCDEF")
	if err != nil || stored != 4 || !truncated || bytesAt(t, env, 0x3100, 5) != "ABCD!" {
		t.Errorf("class A: %d, %v, %v, %q", stored, truncated, err, bytesAt(t, env, 0x3100, 5))
	}
}

// TestWriteString_varying sets CURLEN, copies up to MAXSTRLEN, and
// doesn't pad.
func TestWriteString_varying(t *testing.T) {
	env := fixture(t)
	putDesc(t, env, 0x1000, 5, dscClassVS, 0x2000)
	putBytes(t, env, 0x2000, "\x00\x00.......")

	stored, truncated, err := writeString(env, 0x1000, "ABC")
	if err != nil || stored != 3 || truncated || word(t, env, 0x2000) != 3 || bytesAt(t, env, 0x2002, 5) != "ABC.." {
		t.Errorf("short: %d, %v, %v, CURLEN %d, %q", stored, truncated, err, word(t, env, 0x2000), bytesAt(t, env, 0x2002, 5))
	}

	stored, truncated, err = writeString(env, 0x1000, "ABCDEFGH")
	if err != nil || stored != 5 || !truncated || word(t, env, 0x2000) != 5 || bytesAt(t, env, 0x2002, 6) != "ABCDE." {
		t.Errorf("long: %d, %v, %v, CURLEN %d, %q", stored, truncated, err, word(t, env, 0x2000), bytesAt(t, env, 0x2002, 6))
	}

	if got, _ := readString(env, 0x1000); got != "ABCDE" {
		t.Errorf("read back = %q", got)
	}
}

// TestWriteString_dynamic allocates storage, keeps it for a string that
// fits, and replaces it for one that doesn't.
func TestWriteString_dynamic(t *testing.T) {
	env := fixture(t)
	env.RegionSize[0] = 0x4000
	putDesc(t, env, 0x1000, 0, dscClassD, 0)

	if _, truncated, err := writeString(env, 0x1000, "HELLO"); err != nil || truncated {
		t.Fatalf("first: %v, %v", truncated, err)
	}

	first := longword(t, env, 0x1004)
	if first == 0 || word(t, env, 0x1000) != 5 {
		t.Fatalf("first: pointer %#x, length %d", first, word(t, env, 0x1000))
	}

	if _, _, err := writeString(env, 0x1000, "HI"); err != nil {
		t.Fatal(err)
	}

	if got := longword(t, env, 0x1004); got != first {
		t.Errorf("shorter string moved: %#x, was %#x", got, first)
	}

	if got, _ := readString(env, 0x1000); got != "HI" {
		t.Errorf("shorter string = %q", got)
	}

	long := "A STRING LONGER THAN THE FIRST"
	if _, truncated, err := writeString(env, 0x1000, long); err != nil || truncated {
		t.Fatalf("longer: %v, %v", truncated, err)
	}

	if got, _ := readString(env, 0x1000); got != long {
		t.Errorf("longer string = %q", got)
	}

	if _, _, err := writeString(env, 0x1000, ""); err != nil || word(t, env, 0x1000) != 0 {
		t.Errorf("empty: %v, length %d", err, word(t, env, 0x1000))
	}
}

// TestWriteString_errors: an unsupported class is LIB$_INVSTRDES, and
// storage that can't be written an access violation.
func TestWriteString_errors(t *testing.T) {
	env := fixture(t)

	putDesc(t, env, 0x1000, 5, 15, 0x2000) // DSC$K_CLASS_SB
	if _, _, err := writeString(env, 0x1000, "X"); !errors.Is(err, errInvStrDes) {
		t.Errorf("class SB: err = %v, want LIB$_INVSTRDES", err)
	}

	putDesc(t, env, 0x1020, 5, dscClassS, unmapped)
	var wf writeFault
	if _, _, err := writeString(env, 0x1020, "X"); !errors.As(err, &wf) {
		t.Errorf("unwritable: err = %v, want a write's memory error", err)
	}
}

// TestTooFewArgs is the LIB$_WRONUMARG test.
func TestTooFewArgs(t *testing.T) {
	if !tooFewArgs([]uint32{1}, 2) || tooFewArgs([]uint32{1, 2}, 2) || tooFewArgs([]uint32{1, 2, 3}, 2) {
		t.Error("tooFewArgs is wrong")
	}
}

// TestRoutinesTakeEveryClass: the existing routines read and write
// strings through the helpers, so a varying string works where only S
// and D did, and an unsupported class gets each routine's status.
func TestRoutinesTakeEveryClass(t *testing.T) {
	env, out := foreignFixture(t, "FOREIGN TEXT", "typed line\n")

	putDesc(t, env, 0x1000, 40, dscClassVS, 0x2000)

	if r0 := call(t, env, "LIB$GET_FOREIGN", 0x1000); r0 != ssNormal {
		t.Errorf("LIB$GET_FOREIGN to VS = %#x", r0)
	}

	if got := descText(t, env, 0x1000); got != "FOREIGN TEXT" {
		t.Errorf("LIB$GET_FOREIGN to VS = %q", got)
	}

	if r0 := call(t, env, "LIB$PUT_OUTPUT", 0x1000); r0 != ssNormal || out.String() != "FOREIGN TEXT\n" {
		t.Errorf("LIB$PUT_OUTPUT of VS = %#x, %q", r0, out.String())
	}

	putDesc(t, env, 0x1100, 5, 3, 0x2100) // DSC$K_CLASS_V

	if r0 := call(t, env, "LIB$PUT_OUTPUT", 0x1100); r0 != libInvStrDes {
		t.Errorf("LIB$PUT_OUTPUT of class V = %#x, want LIB$_INVSTRDES", r0)
	}

	if r0 := call(t, env, "LIB$GET_FOREIGN", 0x1100); r0 != libInvStrDes {
		t.Errorf("LIB$GET_FOREIGN to class V = %#x, want LIB$_INVSTRDES", r0)
	}

	// LIB$GET_INPUT's manual entry calls an unknown or 0 class LIB$_INVARG,
	// and checks before it reads a line.
	if r0 := call(t, env, "LIB$GET_INPUT", 0x1100); r0 != libInvArg {
		t.Errorf("LIB$GET_INPUT to class V = %#x, want LIB$_INVARG", r0)
	}

	putDesc(t, env, 0x1200, 5, dscClassZ, 0x2200)

	if r0 := call(t, env, "LIB$GET_INPUT", 0x1200); r0 != libInvArg {
		t.Errorf("LIB$GET_INPUT to class Z = %#x, want LIB$_INVARG", r0)
	}

	if r0 := call(t, env, "LIB$GET_INPUT", 0x1000); r0 != ssNormal || descText(t, env, 0x1000) != "typed line" {
		t.Errorf("LIB$GET_INPUT to VS = %#x, %q; want the first line", r0, descText(t, env, 0x1000))
	}
}
