package librtl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
)

// putWord writes the word v at addr.
func putWord(t *testing.T, env *corevms.Environment, addr uint32, v uint16) {
	t.Helper()

	if err := env.Memory().StoreWord(env.CPU(), addr, v); err != nil {
		t.Fatal(err)
	}
}

// TestFormatSogwProt: each category's name, then its granted letters
// after the separator, or the name alone when it has no access; custom
// names, separators, and access-name tables; STR$_TRU when the output is
// too short.
func TestFormatSogwProt(t *testing.T) {
	env := fixture(t)

	format := func(mask uint16, names, owners, ownSep, listSep uint32) (uint32, string, uint16) {
		t.Helper()
		putWord(t, env, 0x1000, mask)
		putDesc(t, env, 0x1100, 0, dscClassD, 0)
		r0 := call(t, env, "LIB$FORMAT_SOGW_PROT", 0x1000, names, owners, ownSep, listSep, 0x1100, 0x1010)

		return r0, descText(t, env, 0x1100), word(t, env, 0x1010)
	}

	for _, c := range []struct {
		mask uint16
		want string
	}{
		{0xEC00, "System: RWED, Owner: RWED, Group: RW, World: R"},
		{0x0000, "System: RWED, Owner: RWED, Group: RWED, World: RWED"},
		{0xFF00, "System: RWED, Owner: RWED, Group, World"},
		{0xFFFF, "System, Owner, Group, World"},
		{0x0A5F, "System, Owner: WD, Group: RE, World: RWED"},
	} {
		r0, got, n := format(c.mask, 0, 0, 0, 0)
		if r0 != ssNormal || got != c.want || int(n) != len(c.want) {
			t.Errorf("mask %#04x: %#x, %q (length %d), want %q", c.mask, r0, got, n, c.want)
		}
	}

	// Custom ownership names and separators.
	for i, name := range []string{"S", "O", "G", "W"} {
		putDescriptor(t, env, 0x2000+uint32(i)*8, 0x2100+uint32(i)*8, name)
	}

	putDescriptor(t, env, 0x2200, 0x2210, "=")
	putDescriptor(t, env, 0x2220, 0x2230, ",")

	if r0, got, _ := format(0xEC00, 0, 0x2000, 0x2200, 0x2220); r0 != ssNormal || got != "S=RWED,O=RWED,G=RW,W=R" {
		t.Errorf("custom names: %#x, %q", r0, got)
	}

	// A table of the program's own: the first letters, lowercase made
	// upper, and an empty name skipped.
	for i, name := range []string{"alpha", "", "Gamma", "delta"} {
		putDescriptor(t, env, 0x2400+uint32(i)*8, 0x2500+uint32(i)*8, name)
	}

	if r0, got, _ := format(0x0000, 0x2400, 0x2000, 0x2200, 0x2220); r0 != ssNormal || got != "S=AGD,O=AGD,G=AGD,W=AGD" {
		t.Errorf("custom table: %#x, %q", r0, got)
	}

	// A fixed string too short: STR$_TRU, and the length is what fit.
	putWord(t, env, 0x1000, 0xEC00)
	putDescriptor(t, env, 0x1200, 0x1300, "0123456789")

	if r0 := call(t, env, "LIB$FORMAT_SOGW_PROT", 0x1000, 0, 0, 0, 0, 0x1200, 0x1010); r0 != strTru ||
		bytesAt(t, env, 0x1300, 10) != "System: RW" || word(t, env, 0x1010) != 10 {
		t.Errorf("truncated: %#x, %q, length %d", r0, bytesAt(t, env, 0x1300, 10), word(t, env, 0x1010))
	}

	for _, c := range []struct {
		argv []uint32
		want uint32
	}{
		{[]uint32{0x1000, 0, 0, 0, 0}, libWroNumArg},
		{[]uint32{0x1000, 0, 0, 0, 0, 0x1200, 0, 0}, libWroNumArg},
		{[]uint32{0, 0, 0, 0, 0, 0x1200}, libInvArg},
		{[]uint32{0x1000, 0, 0, 0, 0, 0}, libInvArg},
	} {
		if r0 := call(t, env, "LIB$FORMAT_SOGW_PROT", c.argv...); r0 != c.want {
			t.Errorf("FORMAT_SOGW_PROT%v = %#x, want %#x", c.argv, r0, c.want)
		}
	}
}

// TestParseSogwProt: names in full and abbreviated, either case, blanks
// ignored, ":" or "=", a category without access; the errors' offsets.
func TestParseSogwProt(t *testing.T) {
	env := fixture(t)

	parse := func(s string) (r0 uint32, prot, own, end uint16) {
		t.Helper()
		putDescriptor(t, env, 0x1000, 0x1100, s)
		putWord(t, env, 0x1010, 0x5555)
		putWord(t, env, 0x1012, 0x5555)
		putWord(t, env, 0x1014, 0x5555)
		r0 = call(t, env, "LIB$PARSE_SOGW_PROT", 0x1000, 0, 0x1010, 0x1012, 0x1014)

		return r0, word(t, env, 0x1010), word(t, env, 0x1012), word(t, env, 0x1014)
	}

	for _, c := range []struct {
		s         string
		prot, own uint16
	}{
		{"SYSTEM=RWED,OWNER:RWED,GROUP,WORLD:R", 0xEF00, 0xFFFF},
		{"System: RWED, Owner: RWED, Group: RW, World: R", 0xEC00, 0xFFFF},
		{"s:rwed,o:rwed,g:rw,w:r", 0xEC00, 0xFFFF},
		{" G r o u p = R E ", 0x0A00, 0x0F00},
		{"W:", 0xF000, 0xF000},
		{"WORLD", 0xF000, 0xF000},
		{"OWN:D", 0x0070, 0x00F0},
		{"G:R,G:W", 0x0D00, 0x0F00}, // the later value wins
		{"", 0, 0},
	} {
		r0, prot, own, end := parse(c.s)
		if r0 != ssNormal || prot != c.prot || own != c.own || int(end) != len(c.s) {
			t.Errorf("%q: %#x, protection %#04x, ownership %#04x, end %d; want %#04x, %#04x, %d",
				c.s, r0, prot, own, end, c.prot, c.own, len(c.s))
		}
	}

	for _, c := range []struct {
		s   string
		end uint16
	}{
		{"SYX:R", 2},     // no category starts "SYX"
		{"S:RX", 3},      // X is no access's letter
		{"S:R;W:R", 3},   // ";" isn't a separator
		{":R", 0},        // no name
		{"S:R,", 4},      // an empty item at the end
		{"S:R,,W", 4},    // an empty item
		{"SYSTEMS:R", 6}, // longer than the name
		{"S:R:W", 3},     // ":" among the letters
	} {
		r0, prot, own, end := parse(c.s)
		if r0 != libInvArg || end != c.end || prot != 0x5555 || own != 0x5555 {
			t.Errorf("%q: %#x, end %d, masks %#04x %#04x; want LIB$_INVARG at %d, masks untouched",
				c.s, r0, end, prot, own, c.end)
		}
	}

	// A table with its own letters.
	for i, name := range []string{"ALPHA", "BETA", "", "DELTA"} {
		putDescriptor(t, env, 0x2400+uint32(i)*8, 0x2500+uint32(i)*8, name)
	}

	putDescriptor(t, env, 0x1000, 0x1100, "S:AD,W:B")

	if r0 := call(t, env, "LIB$PARSE_SOGW_PROT", 0x1000, 0x2400, 0x1010, 0x1012); r0 != ssNormal ||
		word(t, env, 0x1010) != 0xD006 || word(t, env, 0x1012) != 0xF00F {
		t.Errorf("custom table: %#x, %#04x, %#04x", r0, word(t, env, 0x1010), word(t, env, 0x1012))
	}

	for _, c := range []struct {
		argv []uint32
		want uint32
	}{
		{[]uint32{0x1000, 0, 0x1010}, libWroNumArg},
		{[]uint32{0x1000, 0, 0x1010, 0x1012, 0, 0}, libWroNumArg},
		{[]uint32{0x1000, 0, 0, 0x1012}, libInvArg},
		{[]uint32{0x1000, 0, 0x1010, 0}, libInvArg},
	} {
		if r0 := call(t, env, "LIB$PARSE_SOGW_PROT", c.argv...); r0 != c.want {
			t.Errorf("PARSE_SOGW_PROT%v = %#x, want %#x", c.argv, r0, c.want)
		}
	}
}

// TestParseAccessCode: the letters' bits land in the category's nibble,
// set for granted; a bad category or letter is LIB$_INVARG.
func TestParseAccessCode(t *testing.T) {
	env := fixture(t)

	parse := func(s string, category uint16) (r0 uint32, mask, end uint16) {
		t.Helper()
		putDescriptor(t, env, 0x1000, 0x1100, s)
		putWord(t, env, 0x1010, category)
		putWord(t, env, 0x1012, 0x5555)
		putWord(t, env, 0x1014, 0x5555)
		r0 = call(t, env, "LIB$PARSE_ACCESS_CODE", 0x1000, 0, 0x1010, 0x1012, 0x1014)

		return r0, word(t, env, 0x1012), word(t, env, 0x1014)
	}

	for _, c := range []struct {
		s        string
		category uint16
		want     uint16
	}{
		{"RWED", 0x000F, 0x000F},
		{"RWE", 0x0F00, 0x0700},
		{"d", 0x00F0, 0x0080},
		{"R W", 0xF000, 0x3000},
		{"", 0x000F, 0x0000},
	} {
		r0, mask, end := parse(c.s, c.category)
		if r0 != ssNormal || mask != c.want || int(end) != len(c.s) {
			t.Errorf("%q for %#04x: %#x, mask %#04x, end %d; want %#04x", c.s, c.category, r0, mask, end, c.want)
		}
	}

	for _, c := range []struct {
		s        string
		category uint16
		end      uint16
	}{
		{"RWX", 0x000F, 2},
		{"R,W", 0x000F, 1},
	} {
		r0, mask, end := parse(c.s, c.category)
		if r0 != libInvArg || mask != 0x5555 || end != c.end {
			t.Errorf("%q: %#x, mask %#04x, end %d; want LIB$_INVARG at %d", c.s, r0, mask, end, c.end)
		}
	}

	for _, category := range []uint16{0, 0x00FF, 0x0001, 0xFFFF} {
		if r0, _, _ := parse("R", category); r0 != libInvArg {
			t.Errorf("category %#04x: %#x, want LIB$_INVARG", category, r0)
		}
	}

	if r0 := call(t, env, "LIB$PARSE_ACCESS_CODE", 0x1000, 0, 0x1010); r0 != libWroNumArg {
		t.Errorf("three arguments: %#x, want LIB$_WRONUMARG", r0)
	}

	if r0 := call(t, env, "LIB$PARSE_ACCESS_CODE", 0x1000, 0, 0, 0x1012); r0 != libInvArg {
		t.Errorf("no category: %#x, want LIB$_INVARG", r0)
	}
}
