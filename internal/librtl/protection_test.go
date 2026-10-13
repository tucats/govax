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
// names, separators, and access-name tables (an empty name takes FILE's
// letter); STR$_TRU when the output is too short, the length still the
// whole text's. VMS 7.1's results (testdata/lib55).
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
	// upper, and an empty name FILE's letter.
	for i, name := range []string{"alpha", "", "Gamma", "delta"} {
		putDescriptor(t, env, 0x2400+uint32(i)*8, 0x2500+uint32(i)*8, name)
	}

	if r0, got, _ := format(0x0000, 0x2400, 0x2000, 0x2200, 0x2220); r0 != ssNormal || got != "S=AWGD,O=AWGD,G=AWGD,W=AWGD" {
		t.Errorf("custom table: %#x, %q", r0, got)
	}

	// A fixed string too short: STR$_TRU, and the length is the text's.
	putWord(t, env, 0x1000, 0xEC00)
	putDescriptor(t, env, 0x1200, 0x1300, "0123456789")

	if r0 := call(t, env, "LIB$FORMAT_SOGW_PROT", 0x1000, 0, 0, 0, 0, 0x1200, 0x1010); r0 != strTru ||
		bytesAt(t, env, 0x1300, 10) != "System: RW" || word(t, env, 0x1010) != 46 {
		t.Errorf("truncated: %#x, %q, length %d", r0, bytesAt(t, env, 0x1300, 10), word(t, env, 0x1010))
	}

	for _, c := range []struct {
		argv []uint32
		want uint32
	}{
		{[]uint32{0x1000, 0, 0, 0, 0}, libInvArg},
		{[]uint32{0x1000, 0, 0, 0, 0, 0x1200, 0, 0}, libWroNumArg},
		{[]uint32{0, 0, 0, 0, 0, 0x1200}, libInvArg},
		{[]uint32{0x1000, 0, 0, 0, 0, 0}, libInvArg},
	} {
		if r0 := call(t, env, "LIB$FORMAT_SOGW_PROT", c.argv...); r0 != c.want {
			t.Errorf("FORMAT_SOGW_PROT%v = %#x, want %#x", c.argv, r0, c.want)
		}
	}
}

// TestParseSogwProt: names in full and abbreviated, either case, ":" or
// "=", a category without access, a trailing comma; a category not named
// is denied everything; the errors' statuses and offsets. VMS 7.1's
// results (testdata/lib55).
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
		{"s:rwed,o:rwed,g:rw,w:r", 0xEC00, 0xFFFF},
		{"W:", 0xFFFF, 0xF000},
		{"WORLD", 0xFFFF, 0xF000},
		{"OWN:D", 0xFF7F, 0x00F0},
		{"S:R,", 0xFFFE, 0x000F},
		{"", 0xFFFF, 0},
	} {
		r0, prot, own, end := parse(c.s)
		if r0 != ssNormal || prot != c.prot || own != c.own || int(end) != len(c.s) {
			t.Errorf("%q: %#x, protection %#04x, ownership %#04x, end %d; want %#04x, %#04x, %d",
				c.s, r0, prot, own, end, c.prot, c.own, len(c.s))
		}
	}

	for _, c := range []struct {
		s      string
		status uint32
		end    uint16
	}{
		{"System: RWED, Owner: RWED", libSyntaxErr, 7}, // a blank
		{" G r o u p = R E ", libSyntaxErr, 0},
		{"G:R,G:W", libInvArg, 4}, // named twice
		{"SYX:R", libUnrKey, 0},
		{"SYSTEMS:R", libUnrKey, 0},
		{"S:RX", libSyntaxErr, 2}, // at the letters' start
		{"S:C", libSyntaxErr, 2},
		{"S:R;W:R", libSyntaxErr, 3},
		{":R", libSyntaxErr, 0},
		{"S:R,,W", libSyntaxErr, 4},
		{"S:R:W", libSyntaxErr, 3},
		{"SYSTEM RW", libSyntaxErr, 6},
	} {
		r0, prot, own, end := parse(c.s)
		if r0 != c.status || end != c.end || prot != 0x5555 || own != 0x5555 {
			t.Errorf("%q: %#x, end %d, masks %#04x %#04x; want %#x at %d, masks untouched",
				c.s, r0, end, prot, own, c.status, c.end)
		}
	}

	// A table with its own letters.
	for i, name := range []string{"ALPHA", "BETA", "", "DELTA"} {
		putDescriptor(t, env, 0x2400+uint32(i)*8, 0x2500+uint32(i)*8, name)
	}

	putDescriptor(t, env, 0x1000, 0x1100, "S:AD,W:B")

	if r0 := call(t, env, "LIB$PARSE_SOGW_PROT", 0x1000, 0x2400, 0x1010, 0x1012); r0 != ssNormal ||
		word(t, env, 0x1010) != 0xDFF6 || word(t, env, 0x1012) != 0xF00F {
		t.Errorf("custom table: %#x, %#04x, %#04x", r0, word(t, env, 0x1010), word(t, env, 0x1012))
	}

	for _, c := range []struct {
		argv []uint32
		want uint32
	}{
		{[]uint32{0x1000, 0, 0x1010}, libInvArg},
		{[]uint32{0x1000, 0, 0x1010, 0x1012, 0, 0}, libWroNumArg},
		{[]uint32{0x1000, 0, 0, 0x1012}, libInvArg},
	} {
		if r0 := call(t, env, "LIB$PARSE_SOGW_PROT", c.argv...); r0 != c.want {
			t.Errorf("PARSE_SOGW_PROT%v = %#x, want %#x", c.argv, r0, c.want)
		}
	}
}

// TestParseAccessCode: the letters' bits go where the category's bits
// are, the mask's other bits kept; a character that's no access's letter
// is LIB$_INVARG at its offset. VMS 7.1's results (testdata/lib55), the
// mask preset to 0x5555.
func TestParseAccessCode(t *testing.T) {
	env := fixture(t)

	parse := func(s string, category uint16) (r0 uint32, mask, next, end uint16) {
		t.Helper()
		putDescriptor(t, env, 0x1000, 0x1100, s)
		putWord(t, env, 0x1010, category)
		putWord(t, env, 0x1012, 0x5555)
		putWord(t, env, 0x1014, 0)
		putWord(t, env, 0x1016, 0x5555)
		r0 = call(t, env, "LIB$PARSE_ACCESS_CODE", 0x1000, 0, 0x1010, 0x1012, 0x1016)

		return r0, word(t, env, 0x1012), word(t, env, 0x1014), word(t, env, 0x1016)
	}

	for _, c := range []struct {
		s              string
		category, want uint16
		next           uint16
	}{
		{"RWED", 0x000F, 0x555F, 0},
		{"RWE", 0x0F00, 0x5755, 0},
		{"d", 0x00F0, 0x5585, 0},
		{"", 0x000F, 0x5550, 0},
		{"R", 0x00FF, 0x5501, 0},
		{"R", 0x0000, 0x5555, 1}, // bit 16: the word after
	} {
		r0, mask, next, end := parse(c.s, c.category)
		if r0 != ssNormal || mask != c.want || next != c.next || int(end) != len(c.s) {
			t.Errorf("%q for %#04x: %#x, mask %#04x %#04x, end %d; want %#04x %#04x", c.s, c.category, r0, mask, next, end, c.want, c.next)
		}
	}

	for _, c := range []struct {
		s   string
		end uint16
	}{
		{"R W", 1},
		{"RWX", 2},
		{"R,W", 1},
		{"C", 0},
	} {
		r0, mask, _, end := parse(c.s, 0xF000)
		if r0 != libInvArg || mask != 0x5555 || end != c.end {
			t.Errorf("%q: %#x, mask %#04x, end %d; want LIB$_INVARG at %d", c.s, r0, mask, end, c.end)
		}
	}

	if r0 := call(t, env, "LIB$PARSE_ACCESS_CODE", 0x1000, 0, 0x1010, 0x1012, 0, 0); r0 != libWroNumArg {
		t.Errorf("six arguments: %#x, want LIB$_WRONUMARG", r0)
	}

	if r0 := call(t, env, "LIB$PARSE_ACCESS_CODE", 0x1000, 0, 0, 0x1012); r0 != libInvArg {
		t.Errorf("no category: %#x, want LIB$_INVARG", r0)
	}
}

// TestGetAccnam: LIB$GET_ACCNAM's FILE table, the same address each
// call; another class is SS$_NOCLASS; the argument checks.
func TestGetAccnam(t *testing.T) {
	env := fixture(t)

	if r0 := call(t, env, "LIB$GET_ACCNAM", 0, 0, 0x1000); r0 != ssNormal {
		t.Fatalf("GET_ACCNAM: %#x", r0)
	}

	table := longword(t, env, 0x1000)

	for i, want := range []string{"READ", "WRITE", "EXECUTE", "DELETE", "CONTROL", "", ""} {
		if got := descText(t, env, table+uint32(i)*8); got != want {
			t.Errorf("entry %d: %q, want %q", i, got, want)
		}
	}

	putDescriptor(t, env, 0x1100, 0x1200, "file")

	if r0 := call(t, env, "LIB$GET_ACCNAM", 0x1100, 0, 0x1004); r0 != ssNormal || longword(t, env, 0x1004) != table {
		t.Errorf("GET_ACCNAM of \"file\": %#x, table %#x, want %#x", r0, longword(t, env, 0x1004), table)
	}

	putDescriptor(t, env, 0x1100, 0x1200, "QUEUE")

	if r0 := call(t, env, "LIB$GET_ACCNAM", 0x1100, 0, 0x1004); r0 != ssNoClass {
		t.Errorf("GET_ACCNAM of QUEUE: %#x, want SS$_NOCLASS", r0)
	}

	if r0 := call(t, env, "LIB$GET_ACCNAM", 0, 0); r0 != libWroNumArg {
		t.Errorf("two arguments: %#x, want LIB$_WRONUMARG", r0)
	}

	if r0 := call(t, env, "LIB$GET_ACCNAM", 0, 0, 0); r0 != libInvArg {
		t.Errorf("no accnam: %#x, want LIB$_INVARG", r0)
	}
}
