package corevms

import (
	"testing"
	"time"

	"github.com/tucats/govax/internal/vmsdef"
)

// callFAO calls $FAO with control string ctrl and parameters params,
// into an 80-byte buffer (the manual examples' size), returning the
// output and status.
func callFAO(t *testing.T, env *Environment, a *arena, ctrl string, params ...uint32) (string, uint32) {
	t.Helper()

	outbuf, buf := a.outDesc(80)
	outlen := a.alloc(2)

	argv := append([]uint32{a.desc(ctrl), outlen, outbuf}, params...)
	r0 := callLNM(t, env, serviceSysFao, argv...)

	n, err := env.mem.LoadWord(env.cpu, outlen)
	if err != nil {
		t.Fatal(err)
	}

	return a.readString(buf, n), r0
}

// callFAOL is callFAO for $FAOL, with the parameters in a list.
func callFAOL(t *testing.T, env *Environment, a *arena, ctrl string, params ...uint32) (string, uint32) {
	t.Helper()

	outbuf, buf := a.outDesc(80)
	outlen := a.alloc(2)

	prmlst := a.alloc(uint32(4 * max(len(params), 1)))
	for i, p := range params {
		putLongword(t, env, prmlst+uint32(i)*4, p)
	}

	r0 := callLNM(t, env, serviceSysFaol, a.desc(ctrl), outlen, outbuf, prmlst)

	n, err := env.mem.LoadWord(env.cpu, outlen)
	if err != nil {
		t.Fatal(err)
	}

	return a.readString(buf, n), r0
}

// ascic lays out a counted string (.ASCIC) and returns its address.
func (a *arena) ascic(s string) uint32 {
	addr := a.alloc(uint32(len(s)) + 1)
	putByte(a.t, a.env, addr, byte(len(s)))
	putBytes(a.t, a.env, addr+1, []byte(s))

	return addr
}

func wantFAO(t *testing.T, got string, r0 uint32, want string, wantStatus uint32) {
	t.Helper()

	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}

	wantR0(t, r0, wantStatus)
}

// TestFAO_manualExamples runs the VMS System Services manual's $FAO and
// $FAOL examples 1-10 (the eleventh is FORTRAN).
func TestFAO_manualExamples(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	// A fixed "now" for the time directives: 30-DEC-1988 04:15:28.00.
	now := vmsdef.Time(time.Date(1988, 12, 30, 4, 15, 28, 0, time.Local))
	env.Clock = func() uint64 { return now }

	t.Run("1 !AC !AS !AD !/", func(t *testing.T) {
		nod := a.str("NOD")
		got, r0 := callFAO(t, env, a, "!/SAILORS: !AC !AS !AD", a.ascic("WINKEN"), a.desc("BLINKEN"), 3, nod)
		wantFAO(t, got, r0, "\r\nSAILORS: WINKEN BLINKEN NOD", ssNormal)
	})

	t.Run("2 repeat count and width", func(t *testing.T) {
		got, r0 := callFAO(t, env, a, "UNABLE TO LOCATE !3(8AS)!!", a.desc("JONES"), a.desc("HARRIS"), a.desc("WILSON"))
		wantFAO(t, got, r0, "UNABLE TO LOCATE JONES   HARRIS  WILSON  !", ssNormal)

		got, r0 = callFAO(t, env, a, "UNABLE TO LOCATE !3(AS)!!", a.desc("JONES"), a.desc("HARRIS"), a.desc("WILSON"))
		wantFAO(t, got, r0, "UNABLE TO LOCATE JONESHARRISWILSON!", ssNormal)
	})

	minus400 := uint32(0xFFFFFE70)

	t.Run("3 $FAO longwords", func(t *testing.T) {
		got, r0 := callFAO(t, env, a, "VALUES !UL (DEC) !XL (HEX) !SL (SIGNED)", 200, 300, minus400)
		wantFAO(t, got, r0, "VALUES 200 (DEC) 0000012C (HEX) -400 (SIGNED)", ssNormal)
	})

	t.Run("4 $FAOL longwords", func(t *testing.T) {
		got, r0 := callFAOL(t, env, a, "VALUES !UL (DEC) !XL (HEX) !SL (SIGNED)", 200, 300, minus400)
		wantFAO(t, got, r0, "VALUES 200 (DEC) 0000012C (HEX) -400 (SIGNED)", ssNormal)
	})

	t.Run("5 $FAOL bytes", func(t *testing.T) {
		got, r0 := callFAOL(t, env, a, "VALUES !UB (DEC) !XB (HEX) !SB (SIGNED)", 200, 300, minus400)
		wantFAO(t, got, r0, "VALUES 200 (DEC) 2C (HEX) 112 (SIGNED)", ssNormal)
	})

	t.Run("6 !XW !ZW !-", func(t *testing.T) {
		got, r0 := callFAO(t, env, a, "HEX: !2(6XW) ZERO-DEC: !2(-) !2(7ZW)", 10000, 9999)
		wantFAO(t, got, r0, "HEX:   2710  270F ZERO-DEC:  00100000009999", ssNormal)
	})

	t.Run("7 !%S, !-, variable repeat count", func(t *testing.T) {
		ctrl := "!AS RECEIVED !UB ARG!%S: !-!#(4UB)"

		got, r0 := callFAOL(t, env, a, ctrl, a.desc("ORION"), 3, 10, 123, 210)
		wantFAO(t, got, r0, "ORION RECEIVED 3 ARGS:   10 123 210", ssNormal)

		got, r0 = callFAOL(t, env, a, ctrl, a.desc("LYRA"), 1, 255)
		wantFAO(t, got, r0, "LYRA RECEIVED 1 ARG:  255", ssNormal)
	})

	t.Run("8 !n*c, !%D", func(t *testing.T) {
		got, r0 := callFAO(t, env, a, "!5*> NOW IS: !%D", 0)
		wantFAO(t, got, r0, ">>>>> NOW IS: 30-DEC-1988 04:15:28.00", ssNormal)
	})

	t.Run("9 truncated !%D and !%T, !#*c", func(t *testing.T) {
		got, r0 := callFAO(t, env, a, "DATE: !11%D!#*_TIME: !5%T", 0, 5, 0)
		wantFAO(t, got, r0, "DATE: 30-DEC-1988_____TIME: 04:15", ssNormal)
	})

	t.Run("10 !n< !>", func(t *testing.T) {
		ctrl := "!25<VAR: !AC VAL: !UL!>TOTAL: !7UL"

		got, r0 := callFAO(t, env, a, ctrl, a.ascic("INVENTORY"), 334, 6554)
		wantFAO(t, got, r0, "VAR: INVENTORY VAL: 334  TOTAL:    6554", ssNormal)

		got, r0 = callFAO(t, env, a, ctrl, a.ascic("SALES"), 280, 10750)
		wantFAO(t, got, r0, "VAR: SALES VAL: 280      TOTAL:   10750", ssNormal)
	})
}

// TestFAO_directives covers the directives and field rules the manual's
// examples don't.
func TestFAO_directives(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	quad := a.alloc(8)
	putLongword(t, env, quad, 0x89ABCDEF)
	putLongword(t, env, quad+4, 0x01234567)

	cases := []struct {
		ctrl   string
		params []uint32
		want   string
	}{
		// Octal is zero-filled to 3, 6, and 11 digits.
		{"!OB !OW !OL", []uint32{8, 8, 8}, "010 000010 00000000010"},
		// Hexadecimal: a longer width blank-fills, a shorter one keeps the
		// rightmost digits.
		{"[!10XL] [!4XL] [!1XB]", []uint32{0x12C, 0x12345678, 0xAB}, "[  0000012C] [5678] [B]"},
		// Decimal too wide for its field is all asterisks.
		{"[!2UL] [!3SL] [!2ZL]", []uint32{1000, 0xFFFFFC18, 123}, "[**] [***] [**]"},
		// Signed words and bytes are sign-extended; unsigned aren't.
		{"!SW !UW !SB", []uint32{0xFFFF, 0xFFFF, 0x80}, "-1 65535 -128"},
		{"!ZL !5ZL", []uint32{42, 42}, "42 00042"},
		// Quadwords are by reference.
		{"!XQ !UQ", []uint32{quad, quad}, "0123456789ABCDEF 81985529216486895"},
		// VMS 7's address and integer sizes are longwords on a VAX.
		{"!XH !XA !UI !SJ", []uint32{0x7FFE0000, 0x200, 12, 0xFFFFFFFF}, "7FFE0000 00000200 12 -1"},
		// Layout characters.
		{"a!_b!^c!!d", nil, "a\tb\fc!d"},
		// !+ skips a parameter; !- reuses one.
		{"!UL !+!UL !-!UL", []uint32{1, 2, 3}, "1 3 3"},
		// !%S follows the case of the character before it.
		{"!UL FILE!%S, !UL file!%S", []uint32{2, 1}, "2 FILES, 1 file"},
		{"!UL file!%S", []uint32{0}, "0 files"},
		// !AF shows nonprintable characters as periods.
		{"!AF", []uint32{4, a.str("a\x01\x7Fb")}, "a..b"},
		// A string's width truncates or pads it.
		{"[!3AS] [!6AS]", []uint32{a.desc("ABCDEF"), a.desc("AB")}, "[ABC] [AB    ]"},
		// UICs, in octal, and identifiers.
		{"!%U", []uint32{0x00080010}, "[10,20]"},
		{"!%I !%I !%I", []uint32{NominalUIC, 0x00080010, 0x80010002}, "[SYSTEM] [10,20] %X80010002"},
		{"!%I", []uint32{0x80000003}, "INTERACTIVE"},
		// A field narrower than its text truncates it.
		{"!3<ABCDEF!>|", nil, "ABC|"},
		// A repeated !n*c.
		{"!2(3*-)", nil, "------"},
		// A variable width and a variable repeat count sharing one width.
		{"!#(#UL)", []uint32{2, 4, 7, 8}, "   7   8"},
	}

	for _, c := range cases {
		got, r0 := callFAO(t, env, a, c.ctrl, c.params...)
		if got != c.want || r0 != ssNormal {
			t.Errorf("$FAO %q = %q, %#x; want %q, SS$_NORMAL", c.ctrl, got, r0, c.want)
		}
	}
}

func TestFAO_statuses(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	// A directive $FAO doesn't know stops it, keeping what came before.
	for _, ctrl := range []string{"ok !QQ more", "ok !ul more", "ok !", "ok !%", "ok !3(UL more"} {
		got, r0 := callFAO(t, env, a, ctrl, 1)
		wantFAO(t, got, r0, "ok ", ssBadParam)
	}

	// A result longer than the buffer is truncated, with SS$_BUFFEROVF.
	outbuf, buf := a.outDesc(5)
	outlen := a.alloc(2)
	wantR0(t, callLNM(t, env, serviceSysFao, a.desc("!UL and more"), outlen, outbuf, 1234), ssBufferOvf)

	if n, _ := env.mem.LoadWord(env.cpu, outlen); a.readString(buf, n) != "1234 " {
		t.Errorf("truncated output = %q (length %d), want \"1234 \"", a.readString(buf, n), n)
	}

	// outlen is optional.
	outbuf, buf = a.outDesc(8)
	wantR0(t, callLNM(t, env, serviceSysFao, a.desc("hi"), 0, outbuf), ssNormal)

	if got := a.readString(buf, 2); got != "hi" {
		t.Errorf("output without outlen = %q, want \"hi\"", got)
	}

	// Access violations: no control string or buffer, an unreadable
	// string parameter, $FAOL without a parameter list.
	outbuf, _ = a.outDesc(8)
	wantR0(t, callLNM(t, env, serviceSysFao, 0, 0, outbuf), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysFao, a.desc("x"), 0, 0), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysFao, a.desc("!AS"), 0, outbuf, 0xFFFFFFF0), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysFaol, a.desc("!UL"), 0, outbuf), ssAccVio)

	// $FAOL without a list is fine when nothing needs a parameter.
	wantR0(t, callLNM(t, env, serviceSysFaol, a.desc("text!/"), 0, outbuf), ssNormal)

	// $FAO's parameters past p20 read as 0.
	params := make([]uint32, 21)
	for i := range params {
		params[i] = 7
	}

	got, r0 := callFAO(t, env, a, "!21(UL)", params...)
	wantFAO(t, got, r0, "77777777777777777777"+"0", ssNormal)
}
