package librtl

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmsdef"
)

// Addresses the jacket tests use: a control string's or name's
// descriptor and text, the output descriptor and its text, a word for
// the result length, and arguments by reference.
const (
	jacketIn      = 0x1000
	jacketInText  = 0x1100
	jacketOut     = 0x2000
	jacketOutText = 0x2100
	jacketLength  = 0x3000
	jacketArg     = 0x3100
)

// jacketFixture is fixture with a stack, which the jackets' scratch
// buffers need.
func jacketFixture(t *testing.T) *corevms.Environment {
	t.Helper()

	env := fixture(t)
	env.CPU().SetGPR(registers.SP, 0xF0000)

	return env
}

// resultLength is the word the jackets write their result length to,
// read as the low word of a longword first set to 0xFFFFFFFF (so the
// high word shows it wasn't written beyond).
func resultLength(t *testing.T, env *corevms.Environment) uint32 {
	t.Helper()

	return longword(t, env, jacketLength)
}

// TestSysFaoDynamic: LIB$SYS_FAO formats into a dynamic string, which
// takes the text's length, and stores the length.
func TestSysFaoDynamic(t *testing.T) {
	env := jacketFixture(t)
	putDescriptor(t, env, jacketIn, jacketInText, "!UL apple!%S")
	putDesc(t, env, jacketOut, 0, dscClassD, 0)
	putLongword(t, env, jacketLength, 0xFFFFFFFF)

	got := call(t, env, "LIB$SYS_FAO", jacketIn, jacketLength, jacketOut, 5)
	if got != ssNormal || descText(t, env, jacketOut) != "5 apples" || resultLength(t, env) != 0xFFFF0008 {
		t.Errorf("%#x, %q, length %#x", got, descText(t, env, jacketOut), resultLength(t, env))
	}
}

// TestSysFaoFixed: a fixed string gets a copy of the text, as VMS gives
// it one (testdata/lib55): cut to it is LIB$_STRTRU, and a shorter text
// is padded with blanks.
func TestSysFaoFixed(t *testing.T) {
	env := jacketFixture(t)
	putDescriptor(t, env, jacketIn, jacketInText, "!UL apple!%S")
	putDesc(t, env, jacketOut, 4, dscClassS, jacketOutText)

	if got := call(t, env, "LIB$SYS_FAO", jacketIn, jacketLength, jacketOut, 5); got != libStrTru || bytesAt(t, env, jacketOutText, 4) != "5 ap" || resultLength(t, env)&0xFFFF != 4 {
		t.Errorf("cut: %#x, %q, length %d", got, bytesAt(t, env, jacketOutText, 4), resultLength(t, env)&0xFFFF)
	}

	putBytes(t, env, jacketOutText, "xxxxxxxxxx")
	putDesc(t, env, jacketOut, 10, dscClassS, jacketOutText)

	if got := call(t, env, "LIB$SYS_FAO", jacketIn, 0, jacketOut, 1); got != ssNormal || bytesAt(t, env, jacketOutText, 10) != "1 apple   " {
		t.Errorf("short: %#x, %q", got, bytesAt(t, env, jacketOutText, 10))
	}
}

// TestSysFaoLimits: the text is at most 256 bytes (SS$_BUFFEROVF), and a
// varying string cut shorter still is LIB$_STRTRU.
func TestSysFaoLimits(t *testing.T) {
	env := jacketFixture(t)
	putDescriptor(t, env, jacketIn, jacketInText, "!300*X")
	putDesc(t, env, jacketOut, 0, dscClassD, 0)

	if got := call(t, env, "LIB$SYS_FAO", jacketIn, jacketLength, jacketOut); got != ssBufferOvf || descText(t, env, jacketOut) != strings.Repeat("X", 256) || resultLength(t, env)&0xFFFF != 256 {
		t.Errorf("dynamic: %#x, %d bytes, length %d", got, len(descText(t, env, jacketOut)), resultLength(t, env)&0xFFFF)
	}

	putDescriptor(t, env, jacketIn, jacketInText, "!UL apples")
	putDesc(t, env, jacketOut, 4, dscClassVS, jacketOutText)

	if got := call(t, env, "LIB$SYS_FAO", jacketIn, jacketLength, jacketOut, 12); got != libStrTru || descText(t, env, jacketOut) != "12 a" || resultLength(t, env)&0xFFFF != 4 {
		t.Errorf("varying: %#x, %q, length %d", got, descText(t, env, jacketOut), resultLength(t, env)&0xFFFF)
	}
}

// TestSysFaoFailures: $FAO's failure is returned with nothing copied,
// and an output descriptor of a class that isn't a string's is
// LIB$_INVSTRDES.
func TestSysFaoFailures(t *testing.T) {
	env := jacketFixture(t)
	putDescriptor(t, env, jacketIn, jacketInText, "ok !Q")
	putDesc(t, env, jacketOut, 0, dscClassD, 0)

	if got := call(t, env, "LIB$SYS_FAO", jacketIn, 0, jacketOut); got != vmsdef.Symbols["SS$_BADPARAM"] || descText(t, env, jacketOut) != "" {
		t.Errorf("bad directive: %#x, %q", got, descText(t, env, jacketOut))
	}

	putDescriptor(t, env, jacketIn, jacketInText, "ok")
	putDesc(t, env, jacketOut, 5, 3, jacketOutText) // DSC$K_CLASS_V

	if got := call(t, env, "LIB$SYS_FAO", jacketIn, 0, jacketOut); got != libInvStrDes {
		t.Errorf("class V: %#x, want LIB$_INVSTRDES", got)
	}
}

// TestSysFaol: LIB$SYS_FAOL takes its directive arguments from a
// longword array.
func TestSysFaol(t *testing.T) {
	env := jacketFixture(t)
	putDescriptor(t, env, jacketIn, jacketInText, "!UL-!XW")
	putDesc(t, env, jacketOut, 0, dscClassD, 0)
	putLongword(t, env, jacketArg, 42)
	putLongword(t, env, jacketArg+4, 0xBEEF)

	if got := call(t, env, "LIB$SYS_FAOL", jacketIn, 0, jacketOut, jacketArg); got != ssNormal || descText(t, env, jacketOut) != "42-BEEF" {
		t.Errorf("%#x, %q", got, descText(t, env, jacketOut))
	}
}

// putQuad stores v at addr.
func putQuad(t *testing.T, env *corevms.Environment, addr uint32, v uint64) {
	t.Helper()

	putLongword(t, env, addr, uint32(v))
	putLongword(t, env, addr+4, uint32(v>>32))
}

// TestSysAsctim: the whole text goes to a dynamic string; a 12-byte
// fixed one gets the date (LIB$_STRTRU); flags by reference asks for the
// time of day; a delta of 10,000 days is SS$_IVTIME.
func TestSysAsctim(t *testing.T) {
	env := jacketFixture(t)
	day := uint64(24 * 3600 * vmsdef.TicksPerSecond)

	putQuad(t, env, jacketArg, -(day + day/24)) // a delta: 1 day, 1 hour
	putDesc(t, env, jacketOut, 0, dscClassD, 0)

	if got := call(t, env, "LIB$SYS_ASCTIM", jacketLength, jacketOut, jacketArg); got != ssNormal || descText(t, env, jacketOut) != "   1 01:00:00.00" || resultLength(t, env)&0xFFFF != 16 {
		t.Errorf("delta: %#x, %q, length %d", got, descText(t, env, jacketOut), resultLength(t, env)&0xFFFF)
	}

	when, ok := env.ParseTime("13-OCT-2026 06:10:00.00")
	if !ok {
		t.Fatal("ParseTime")
	}

	putQuad(t, env, jacketArg, when)
	putDesc(t, env, jacketOut, 12, dscClassS, jacketOutText)

	if got := call(t, env, "LIB$SYS_ASCTIM", jacketLength, jacketOut, jacketArg); got != libStrTru || bytesAt(t, env, jacketOutText, 12) != "13-OCT-2026 " || resultLength(t, env)&0xFFFF != 12 {
		t.Errorf("date: %#x, %q, length %d", got, bytesAt(t, env, jacketOutText, 12), resultLength(t, env)&0xFFFF)
	}

	putLongword(t, env, jacketArg+8, 1)
	putDesc(t, env, jacketOut, 0, dscClassD, 0)

	if got := call(t, env, "LIB$SYS_ASCTIM", 0, jacketOut, jacketArg, jacketArg+8); got != ssNormal || descText(t, env, jacketOut) != "06:10:00.00" {
		t.Errorf("time of day: %#x, %q", got, descText(t, env, jacketOut))
	}

	putQuad(t, env, jacketArg, -(10000 * day))

	if got := call(t, env, "LIB$SYS_ASCTIM", 0, jacketOut, jacketArg); got != vmsdef.Symbols["SS$_IVTIME"] {
		t.Errorf("10,000 days: %#x, want SS$_IVTIME", got)
	}
}

// TestSysAsctimNow: with no user-time, the current time.
func TestSysAsctimNow(t *testing.T) {
	env := jacketFixture(t)
	putDesc(t, env, jacketOut, 0, dscClassD, 0)

	want, _ := corevms.FormatTime(env.Clock(), false)

	if got := call(t, env, "LIB$SYS_ASCTIM", 0, jacketOut); got != ssNormal || descText(t, env, jacketOut)[:12] != want[:12] {
		t.Errorf("%#x, %q, want about %q", got, descText(t, env, jacketOut), want)
	}
}

// TestSysGetmsg: message-id and flags by reference; flags omitted is the
// whole message; the array gets the FAO count; an unknown code is
// SS$_MSGNOTFND with its stand-in line.
func TestSysGetmsg(t *testing.T) {
	env := jacketFixture(t)
	putLongword(t, env, jacketArg, ssNormal)
	putDesc(t, env, jacketOut, 0, dscClassD, 0)

	if got := call(t, env, "LIB$SYS_GETMSG", jacketArg, jacketLength, jacketOut); got != ssNormal || descText(t, env, jacketOut) != "%SYSTEM-S-NORMAL, normal successful completion" {
		t.Errorf("whole: %#x, %q", got, descText(t, env, jacketOut))
	}

	putLongword(t, env, jacketArg+4, 1) // the text only
	putLongword(t, env, jacketArg+8, 0xFFFFFFFF)

	if got := call(t, env, "LIB$SYS_GETMSG", jacketArg, 0, jacketOut, jacketArg+4, jacketArg+8); got != ssNormal || descText(t, env, jacketOut) != "normal successful completion" || longword(t, env, jacketArg+8) != 0 {
		t.Errorf("text: %#x, %q, array %#x", got, descText(t, env, jacketOut), longword(t, env, jacketArg+8))
	}

	putLongword(t, env, jacketArg, vmsdef.Symbols["SS$_ACCVIO"])

	if call(t, env, "LIB$SYS_GETMSG", jacketArg, 0, jacketOut, 0, jacketArg+8); longword(t, env, jacketArg+8) != 4<<8 {
		t.Errorf("SS$_ACCVIO's FAO count: array %#x, want 4 in byte 1", longword(t, env, jacketArg+8))
	}

	putLongword(t, env, jacketArg, 0x0FFF0002)

	if got := call(t, env, "LIB$SYS_GETMSG", jacketArg, 0, jacketOut); got != vmsdef.Symbols["SS$_MSGNOTFND"] || !strings.HasPrefix(descText(t, env, jacketOut), "%NONAME-") {
		t.Errorf("unknown: %#x, %q", got, descText(t, env, jacketOut))
	}
}

// TestSysTrnlog: $TRNLOG's arguments in its order, the result by the
// output's class; an undefined name is SS$_NOTRAN with the name itself.
func TestSysTrnlog(t *testing.T) {
	env := jacketFixture(t)
	defineName(t, env, lnm.ProcessTableName, "JACKET", "the translation", lnm.User)
	putDescriptor(t, env, jacketIn, jacketInText, "JACKET")
	putDesc(t, env, jacketOut, 0, dscClassD, 0)
	putLongword(t, env, jacketArg, 0xFFFFFFFF)

	if got := call(t, env, "LIB$SYS_TRNLOG", jacketIn, jacketLength, jacketOut, jacketArg); got != ssNormal || descText(t, env, jacketOut) != "the translation" || resultLength(t, env)&0xFFFF != 15 || longword(t, env, jacketArg)&0xFF != 2 {
		t.Errorf("defined: %#x, %q, length %d, table %d", got, descText(t, env, jacketOut), resultLength(t, env)&0xFFFF, longword(t, env, jacketArg)&0xFF)
	}

	putDesc(t, env, jacketOut, 3, dscClassS, jacketOutText)

	if got := call(t, env, "LIB$SYS_TRNLOG", jacketIn, 0, jacketOut); got != libStrTru || bytesAt(t, env, jacketOutText, 3) != "the" {
		t.Errorf("cut: %#x, %q", got, bytesAt(t, env, jacketOutText, 3))
	}

	putDescriptor(t, env, jacketIn, jacketInText, "NOSUCH")
	putDesc(t, env, jacketOut, 0, dscClassD, 0)

	if got := call(t, env, "LIB$SYS_TRNLOG", jacketIn, 0, jacketOut); got != vmsdef.Symbols["SS$_NOTRAN"] || descText(t, env, jacketOut) != "NOSUCH" {
		t.Errorf("undefined: %#x, %q", got, descText(t, env, jacketOut))
	}

	// The disable mask is by reference: bit 2 passes over the process
	// table, where JACKET is.
	putDescriptor(t, env, jacketIn, jacketInText, "JACKET")
	putLongword(t, env, jacketArg, 4)

	if got := call(t, env, "LIB$SYS_TRNLOG", jacketIn, 0, jacketOut, 0, 0, jacketArg); got != vmsdef.Symbols["SS$_NOTRAN"] {
		t.Errorf("process table disabled: %#x, want SS$_NOTRAN", got)
	}
}

// TestJacketScratchBelowSP: a jacket's scratch buffer is below SP, in
// its own frame; it moves SP, which the routine's RET restores.
func TestJacketScratchBelowSP(t *testing.T) {
	env := jacketFixture(t)
	putDescriptor(t, env, jacketIn, jacketInText, "x")
	putDesc(t, env, jacketOut, 0, dscClassD, 0)

	call(t, env, "LIB$SYS_FAO", jacketIn, 0, jacketOut)

	if sp := env.CPU().GPR(registers.SP); sp >= 0xF0000 || sp < 0xF0000-faoScratch-16 {
		t.Errorf("SP %#x after the call, want just below 0xF0000", sp)
	}
}
