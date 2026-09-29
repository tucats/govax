package rtl

import (
	"bytes"
	"strings"
	"testing"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// Terminal function codes, spelled out for the tests.
var (
	fnReadVBlk   = vmsdef.IOConstants["IO$_READVBLK"]
	fnReadPrompt = vmsdef.IOConstants["IO$_READPROMPT"]
	fnWriteVBlk  = vmsdef.IOConstants["IO$_WRITEVBLK"]
	fnSenseMode  = vmsdef.IOConstants["IO$_SENSEMODE"]
	fnSetMode    = vmsdef.IOConstants["IO$_SETMODE"]
)

// badAddr is past the end of the fixture's 1MB of memory (virtual memory
// is off, so it's used as a physical address), so any access to it fails.
const badAddr = 0x7FFF0000

// qioFixture is an Environment whose console input is input, with a
// terminal TTA0 and a user-mode channel to it.
func qioFixture(t *testing.T, input string) (*Environment, *bytes.Buffer, *arena, uint32) {
	t.Helper()

	env, out := fixture()
	env.consoleIn = strings.NewReader(input)
	defineTestDevice(env, "TTA0", iodev.DeviceClassTT)

	a := newArena(t, env)

	return env, out, a, assignCall(t, env, a, "TTA0", uint32(vax.User))
}

// qioArgs is one $QIO call's arguments.
type qioArgs struct {
	efn, channel, function, iosb, astadr, astprm uint32
	p                                            [6]uint32
}

func callQIO(t *testing.T, env *Environment, q qioArgs) uint32 {
	t.Helper()

	argv := append([]uint32{q.efn, q.channel, q.function, q.iosb, q.astadr, q.astprm}, q.p[:]...)

	return callLNM(t, env, serviceSysQio, argv...)
}

// readIOSB returns an I/O status block's condition value, transfer
// count, and second longword.
func readIOSB(a *arena, iosb uint32) (status, count uint16, info uint32) {
	first := a.readLong(iosb)

	return uint16(first), uint16(first >> 16), a.readLong(iosb + 4)
}

func TestQIO_write(t *testing.T) {
	env, out, a, ch := qioFixture(t, "")
	iosb := a.alloc(8)
	env.Process.LocalEventFlags[0] = 0

	wantR0(t, callQIO(t, env, qioArgs{
		efn: 3, channel: ch, function: fnWriteVBlk, iosb: iosb, astadr: 0x4000, astprm: 77,
		p: [6]uint32{a.str("Hello\r\n"), 7},
	}), ssNormal)

	if got := out.String(); got != "Hello\r\n" {
		t.Errorf("output = %q, want %q", got, "Hello\r\n")
	}

	if st, n, info := readIOSB(a, iosb); st != ssNormal || n != 7 || info != 0 {
		t.Errorf("IOSB = %d, %d, %#x; want SS$_NORMAL, 7, 0", st, n, info)
	}

	if !flagSet(env, 3) {
		t.Error("event flag 3 not set on completion")
	}

	q := env.Process.ast.queue
	if len(q) != 1 || q[0].routine != 0x4000 || q[0].param != 77 || q[0].mode != uint32(vax.Kernel) {
		t.Errorf("AST queue = %+v, want one kernel-mode AST at 0x4000 with parameter 77", q)
	}
}

func TestQIO_carriageControl(t *testing.T) {
	cases := []struct {
		name string
		p4   uint32
		want string
	}{
		{"none", 0, "X"},
		{"FORTRAN space", ' ', "\nX\r"},
		{"FORTRAN zero", '0', "\n\nX\r"},
		{"FORTRAN one", '1', "\fX\r"},
		{"FORTRAN plus", '+', "X\r"},
		{"FORTRAN dollar", '$', "\nX"},
		{"FORTRAN other", 'Q', "\nX\r"},
		{"prefix newlines", 2 << 16, "\n\nX"},
		{"C0 postfix", 0x8D << 24, "X\r"},
		{"C0 prefix and postfix", 0x8A<<16 | 0x8D<<24, "\nX\r"},
		{"C1 prefix", 0xC4 << 16, "\x84X"},
		{"reserved codes", 0xA0<<16 | 0xE0<<24, "X"},
	}

	for _, c := range cases {
		env, out, a, ch := qioFixture(t, "")

		wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnWriteVBlk, p: [6]uint32{a.str("X"), 1, 0, c.p4}}), ssNormal)

		if got := out.String(); got != c.want {
			t.Errorf("%s: output = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestQIO_read(t *testing.T) {
	env, _, a, ch := qioFixture(t, "hello\nworld\r\nnext\n")
	iosb, buf := a.alloc(8), a.alloc(80)

	read := func(size uint32) (string, uint16, uint16, uint32) {
		t.Helper()

		wantR0(t, callQIO(t, env, qioArgs{efn: 1, channel: ch, function: fnReadVBlk, iosb: iosb, p: [6]uint32{buf, size}}), ssNormal)

		st, n, info := readIOSB(a, iosb)

		return a.readString(buf, n), st, n, info
	}

	// A host newline ends the read as RETURN would: the data doesn't
	// include it, and the IOSB reports a carriage return terminator.
	if s, st, _, info := read(80); s != "hello" || st != ssNormal || info != ttCarriageReturn|1<<16 {
		t.Errorf("read = %q, status %d, info %#x; want \"hello\", SS$_NORMAL, CR terminator of size 1", s, st, info)
	}

	// The terminator is also stored after the data.
	if b := readBytes(t, env, buf+5, 1); b[0] != ttCarriageReturn {
		t.Errorf("byte after the data = %#x, want the CR terminator", b[0])
	}

	if !flagSet(env, 1) {
		t.Error("event flag 1 not set")
	}

	// A "\r\n" pair is one terminator.
	if s, _, _, info := read(80); s != "world" || info != ttCarriageReturn|1<<16 {
		t.Errorf("read = %q, info %#x; want \"world\" ended by one CR", s, info)
	}

	// A full buffer ends the read with no terminator; the rest waits for
	// the next read.
	if s, st, _, info := read(2); s != "ne" || st != ssNormal || info != 0 {
		t.Errorf("read(2) = %q, status %d, info %#x; want \"ne\", SS$_NORMAL, no terminator", s, st, info)
	}

	if s, _, _, _ := read(80); s != "xt" {
		t.Errorf("read after a full buffer = %q, want \"xt\"", s)
	}

	// The end of the host's input.
	if s, st, n, _ := read(80); st != uint16(ssEndOfFile) || n != 0 {
		t.Errorf("read at end of input = %q, status %d, count %d; want SS$_ENDOFFILE, 0", s, st, n)
	}
}

func TestQIO_readPartialLineAtEOF(t *testing.T) {
	env, _, a, ch := qioFixture(t, "abc")
	iosb, buf := a.alloc(8), a.alloc(80)

	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnReadVBlk, iosb: iosb, p: [6]uint32{buf, 80}}), ssNormal)

	if st, n, _ := readIOSB(a, iosb); st != uint16(ssEndOfFile) || n != 3 || a.readString(buf, 3) != "abc" {
		t.Errorf("IOSB = %d, %d, data %q; want SS$_ENDOFFILE, 3, \"abc\"", st, n, a.readString(buf, n))
	}
}

func TestQIO_readModifiers(t *testing.T) {
	env, _, a, ch := qioFixture(t, "MixEd\nfirst\nahead\nmore\n")
	iosb, buf := a.alloc(8), a.alloc(80)

	read := func(function, timeout uint32) (string, uint16) {
		t.Helper()

		wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: function, iosb: iosb, p: [6]uint32{buf, 80, timeout}}), ssNormal)

		st, n, _ := readIOSB(a, iosb)

		return a.readString(buf, n), st
	}

	if s, _ := read(fnReadVBlk|ioModCvtLow, 0); s != "MIXED" {
		t.Errorf("IO$M_CVTLOW read = %q, want \"MIXED\"", s)
	}

	// The rest of the input is now buffered, so a zero-time IO$M_TIMED
	// read finds a whole line.
	if s, st := read(fnReadVBlk|ioModTimed, 0); s != "first" || st != ssNormal {
		t.Errorf("timed read = %q, status %d; want \"first\", SS$_NORMAL", s, st)
	}

	// IO$M_PURGE discards what's typed ahead: nothing is left.
	if s, st := read(fnReadVBlk|ioModPurge, 0); st != uint16(ssEndOfFile) {
		t.Errorf("read after IO$M_PURGE = %q, status %d; want SS$_ENDOFFILE", s, st)
	}
}

func TestQIO_readTimedPoll(t *testing.T) {
	env, _, a, ch := qioFixture(t, "x\nab")
	iosb, buf := a.alloc(8), a.alloc(80)

	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnReadVBlk, iosb: iosb, p: [6]uint32{buf, 80}}), ssNormal)

	// "ab" is typed ahead without a terminator: a zero-time read returns
	// it and times out.
	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnReadVBlk | ioModTimed, iosb: iosb, p: [6]uint32{buf, 80, 0}}), ssNormal)

	if st, n, _ := readIOSB(a, iosb); st != uint16(ssTimeout) || a.readString(buf, n) != "ab" {
		t.Errorf("zero-time read = %q, status %d; want \"ab\", SS$_TIMEOUT", a.readString(buf, n), st)
	}

	// Nothing is typed ahead now.
	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnReadVBlk | ioModTimed, iosb: iosb, p: [6]uint32{buf, 80, 0}}), ssNormal)

	if st, n, _ := readIOSB(a, iosb); st != uint16(ssTimeout) || n != 0 {
		t.Errorf("empty zero-time read: status %d, count %d; want SS$_TIMEOUT, 0", st, n)
	}
}

func TestQIO_terminatorSets(t *testing.T) {
	// Short form: only TAB (9) terminates, so RETURN is data.
	env, _, a, ch := qioFixture(t, "a b\nc\td")
	iosb, buf := a.alloc(8), a.alloc(80)

	short := a.alloc(8)
	putLongword(t, env, short, 0)
	putLongword(t, env, short+4, 1<<ttTab)

	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnReadVBlk, iosb: iosb, p: [6]uint32{buf, 80, 0, short}}), ssNormal)

	if st, n, info := readIOSB(a, iosb); a.readString(buf, n) != "a b\rc" || st != ssNormal || info != ttTab|1<<16 {
		t.Errorf("short-form read = %q, info %#x; want \"a b\\rc\" ended by TAB", a.readString(buf, n), info)
	}

	// Long form: a 6-byte mask, with '.' (46: byte 5, bit 6) terminating.
	env, _, a, ch = qioFixture(t, "one.two")
	iosb, buf = a.alloc(8), a.alloc(80)

	mask := a.alloc(8)
	putBytes(t, env, mask, []byte{0, 0, 0, 0, 0, 1 << 6})

	long := a.alloc(8)
	putLongword(t, env, long, 6)
	putLongword(t, env, long+4, mask)

	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnReadVBlk, iosb: iosb, p: [6]uint32{buf, 80, 0, long}}), ssNormal)

	if _, n, info := readIOSB(a, iosb); a.readString(buf, n) != "one" || info != '.'|1<<16 {
		t.Errorf("long-form read = %q, info %#x; want \"one\" ended by '.'", a.readString(buf, n), info)
	}

	// An unreadable terminator descriptor rejects the read.
	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnReadVBlk, iosb: iosb, p: [6]uint32{buf, 80, 0, badAddr}}), ssAccVio)
}

func TestQIO_readPrompt(t *testing.T) {
	env, out, a, ch := qioFixture(t, "Tom\n")
	iosb, buf := a.alloc(8), a.alloc(80)

	wantR0(t, callQIO(t, env, qioArgs{
		channel: ch, function: fnReadPrompt, iosb: iosb,
		p: [6]uint32{buf, 80, 0, 0, a.str("Name: "), 6},
	}), ssNormal)

	if out.String() != "Name: " {
		t.Errorf("prompt output = %q, want \"Name: \"", out.String())
	}

	if _, n, _ := readIOSB(a, iosb); a.readString(buf, n) != "Tom" {
		t.Errorf("prompted read = %q, want \"Tom\"", a.readString(buf, n))
	}

	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnReadPrompt, p: [6]uint32{buf, 80, 0, 0, badAddr, 6}}), ssAccVio)
}

func TestQIO_senseAndSetMode(t *testing.T) {
	env, _, a, ch := qioFixture(t, "")
	dp, _ := env.Devices.Find("TTA0")
	dp.DevType, dp.DevBufSize, dp.DevDepend, dp.DevDepend2 = 96, 80, 0x18000000, 0x1234

	iosb, buf := a.alloc(8), a.alloc(12)

	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnSenseMode, iosb: iosb, p: [6]uint32{buf, 8}}), ssNormal)

	want := []byte{66, 96, 80, 0, 0, 0, 0, 0x18}
	if got := readBytes(t, env, buf, 8); !bytes.Equal(got, want) {
		t.Errorf("8-byte SENSEMODE = % x, want % x", got, want)
	}

	if st, _, _ := readIOSB(a, iosb); st != ssNormal {
		t.Errorf("SENSEMODE IOSB status = %d, want SS$_NORMAL", st)
	}

	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnSenseMode, p: [6]uint32{buf, 12}}), ssNormal)

	if got := a.readLong(buf + 8); got != 0x1234 {
		t.Errorf("12-byte SENSEMODE's extended characteristics = %#x, want 0x1234", got)
	}

	// SETMODE changes type, width, and characteristics; the class stays.
	set := a.alloc(12)
	putBytes(t, env, set, []byte{1, 110, 132, 0, 0x10, 0, 0, 48, 5, 0, 0, 0})
	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnSetMode, p: [6]uint32{set, 12}}), ssNormal)

	if dp.DevClass != iodev.DeviceClassTT || dp.DevType != 110 || dp.DevBufSize != 132 || dp.DevDepend != 0x30000010 || dp.DevDepend2 != 5 {
		t.Errorf("after SETMODE: class %d type %d width %d devdepend %#x devdepend2 %#x",
			dp.DevClass, dp.DevType, dp.DevBufSize, dp.DevDepend, dp.DevDepend2)
	}

	// A SETMODE modifier (IO$M_CTRLCAST: p1 is an AST address) changes
	// nothing.
	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnSetMode | vmsdef.IOConstants["IO$M_CTRLCAST"], p: [6]uint32{0x4000}}), ssNormal)

	if dp.DevType != 110 {
		t.Errorf("SETMODE with IO$M_CTRLCAST changed the terminal type to %d", dp.DevType)
	}

	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnSenseMode, p: [6]uint32{badAddr, 8}}), ssAccVio)
	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnSetMode, p: [6]uint32{badAddr, 8}}), ssAccVio)
}

func TestQIO_typeaheadCount(t *testing.T) {
	env, _, a, ch := qioFixture(t, "x\nyz\n")
	buf := a.alloc(80)

	// Nothing has been read, so nothing is buffered yet.
	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnSenseMode | ioModTypeahead, p: [6]uint32{buf}}), ssNormal)

	if got := readBytes(t, env, buf, 3); !bytes.Equal(got, []byte{0, 0, 0}) {
		t.Errorf("type-ahead before reading = % x, want 00 00 00", got)
	}

	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnReadVBlk, p: [6]uint32{buf, 80}}), ssNormal)
	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnSenseMode | ioModTypeahead, p: [6]uint32{buf}}), ssNormal)

	if got := readBytes(t, env, buf, 3); !bytes.Equal(got, []byte{3, 0, 'y'}) {
		t.Errorf("type-ahead = % x, want 03 00 79 (\"yz\\n\" waiting)", got)
	}
}

func TestQIO_rejected(t *testing.T) {
	env, out, a, ch := qioFixture(t, "")
	defineTestDevice(env, "DUA0", iodev.DeviceClassDisk)
	diskChan := assignCall(t, env, a, "DUA0", 0)
	iosb := a.alloc(8)
	data := a.str("X")

	cases := []struct {
		name      string
		q         qioArgs
		want      uint32
		iosbClear bool // the IOSB was cleared before the rejection
	}{
		{"channel 0", qioArgs{efn: 2, function: fnWriteVBlk}, ssIvChan, false},
		{"unassigned channel", qioArgs{efn: 2, channel: 999, function: fnWriteVBlk}, ssNoPriv, false},
		{"unwritable IOSB", qioArgs{efn: 2, channel: ch, function: fnWriteVBlk, iosb: badAddr}, ssAccVio, false},
		{"no such function", qioArgs{efn: 2, channel: ch, function: 63}, ssIllIoFunc, true},
		{"a function the disk driver lacks", qioArgs{efn: 2, channel: diskChan, function: vmsdef.IOConstants["IO$_READPROMPT"]}, ssIllIoFunc, true},
		{"unreadable write buffer", qioArgs{efn: 2, channel: ch, function: fnWriteVBlk, p: [6]uint32{badAddr, 10}}, ssAccVio, true},
		{"unwritable read buffer", qioArgs{efn: 2, channel: ch, function: fnReadVBlk, p: [6]uint32{badAddr, 10}}, ssAccVio, true},
	}

	for _, c := range cases {
		env.Process.LocalEventFlags[0] = 0
		putLongword(t, env, iosb, 0xDEADBEEF)
		putLongword(t, env, iosb+4, 0xDEADBEEF)

		if c.q.iosb == 0 {
			c.q.iosb = iosb
		}

		c.q.astadr = 0x4000

		wantR0(t, callQIO(t, env, c.q), c.want)

		if !flagSet(env, 2) {
			t.Errorf("%s: event flag not set on rejection", c.name)
		}

		first := a.readLong(iosb)
		if c.iosbClear && first != 0 || !c.iosbClear && c.q.iosb == iosb && first != 0xDEADBEEF {
			t.Errorf("%s: IOSB = %#x, want it %s", c.name, first, map[bool]string{true: "cleared", false: "untouched"}[c.iosbClear])
		}

		if env.PendingASTs() != 0 {
			t.Errorf("%s: an AST was queued for a rejected request", c.name)
		}
	}

	if out.Len() != 0 {
		t.Errorf("rejected writes wrote %q", out.String())
	}

	// A bad event flag is reported before anything else, and leaves the
	// flags alone.
	wantR0(t, callQIO(t, env, qioArgs{efn: 200, channel: ch, function: fnWriteVBlk, p: [6]uint32{data, 1}}), ssIllEfc)

	// A channel assigned from a more privileged mode than the caller's.
	setCurMod(env, vax.User)
	wantR0(t, callQIO(t, env, qioArgs{channel: diskChan, function: fnWriteVBlk}), ssNoPriv)

	// The user-mode channel is fine from user mode.
	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnWriteVBlk, p: [6]uint32{data, 1}}), ssNormal)

}

func TestQIO_readOnlyChannelWord(t *testing.T) {
	env, out, a, ch := qioFixture(t, "")

	// Only the low words of chan and func count.
	wantR0(t, callQIO(t, env, qioArgs{channel: 0xFFFF0000 | ch, function: 0xFFFF0000 | fnWriteVBlk, p: [6]uint32{a.str("ok"), 2}}), ssNormal)

	if out.String() != "ok" {
		t.Errorf("output = %q, want \"ok\"", out.String())
	}
}

func TestServiceSysCancel(t *testing.T) {
	env, _, a, ch := qioFixture(t, "")
	kernelChan := assignCall(t, env, a, "TTA0", 0)

	wantR0(t, callLNM(t, env, serviceSysCancel, ch), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysCancel, 0xFFFF0000|ch), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysCancel, 0), ssIvChan)
	wantR0(t, callLNM(t, env, serviceSysCancel, 999), ssNoPriv)

	setCurMod(env, vax.User)
	wantR0(t, callLNM(t, env, serviceSysCancel, kernelChan), ssNoPriv)
	wantR0(t, callLNM(t, env, serviceSysCancel, ch), ssNormal)
}
