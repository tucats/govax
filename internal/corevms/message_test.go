package corevms

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// Condition values the message tests use.
const (
	testSSAbort   = 0x2C     // SS$_ABORT: %SYSTEM-F-ABORT, abort
	testSSDuplnam = 0x94     // SS$_DUPLNAM: %SYSTEM-F-DUPLNAM, duplicate name
	testSSAccvio  = 0x0C     // SS$_ACCVIO: four $FAO parameters
	testRMSFnf    = 0x18292  // RMS$_FNF: %RMS-E-FNF, file not found
	testRMSRtb    = 0x181A8  // RMS$_RTB (warning): "!UL byte record too large ..."
	testLIBBadccc = 0x15C000 // LIB$_BADCCC (warning): "illegal compilation code (!UL.) in module !AC"
)

// getmsg calls $GETMSG for msgid with flags into a 256-byte buffer,
// returning the line, the four outadr bytes, and R0.
func getmsg(t *testing.T, env *Environment, a *arena, msgid, flags uint32) (string, uint32, uint32) {
	t.Helper()

	bufadr, buf := a.outDesc(256)
	msglen, outadr := a.alloc(2), a.long(0xFFFFFFFF)

	r0 := callLNM(t, env, serviceSysGetmsg, msgid, msglen, bufadr, flags, outadr)

	n, err := env.mem.LoadWord(env.cpu, msglen)
	if err != nil {
		t.Fatal(err)
	}

	return a.readString(buf, n), a.readLong(outadr), r0
}

func TestGetmsg(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	cases := []struct {
		msgid, flags uint32
		want         string
	}{
		// Every part (flags 15, or 0: the process default).
		{testSSDuplnam, 15, "%SYSTEM-F-DUPLNAM, duplicate name"},
		{testSSDuplnam, 0, "%SYSTEM-F-DUPLNAM, duplicate name"},
		// The manual's example: the text alone.
		{testSSDuplnam, 1, "duplicate name"},
		{testSSDuplnam, 14, "%SYSTEM-F-DUPLNAM"},
		{testSSDuplnam, 2, "%DUPLNAM"},
		{testSSDuplnam, 9, "%SYSTEM, duplicate name"},
		// The severity is the code's, not the definition's.
		{testSSDuplnam &^ 7, 15, "%SYSTEM-W-DUPLNAM, duplicate name"},
		// The text is returned unformatted.
		{testSSAccvio, 1, "access violation, reason mask=!XB, virtual address=!XH, PC=!XH, PS=!XL"},
		// Other facilities.
		{testRMSFnf, 15, "%RMS-E-FNF, file not found"},
		{testLIBBadccc, 15, "%LIB-W-BADCCC, illegal compilation code (!UL.) in module !AC"},
	}

	for _, c := range cases {
		got, _, r0 := getmsg(t, env, a, c.msgid, c.flags)
		if got != c.want || r0 != ssNormal {
			t.Errorf("$GETMSG(%#x, %d) = %q, %#x; want %q, SS$_NORMAL", c.msgid, c.flags, got, r0, c.want)
		}
	}

	// outadr's second byte is the $FAO parameter count.
	if _, out, _ := getmsg(t, env, a, testSSAccvio, 15); out != 4<<8 {
		t.Errorf("outadr for SS$_ACCVIO = %#08x, want 0x00000400", out)
	}

	// flags and outadr are optional.
	bufadr, buf := a.outDesc(64)
	msglen := a.alloc(2)
	wantR0(t, callLNM(t, env, serviceSysGetmsg, testSSAbort, msglen, bufadr), ssNormal)

	if n, _ := env.mem.LoadWord(env.cpu, msglen); a.readString(buf, n) != "%SYSTEM-F-ABORT, abort" {
		t.Errorf("$GETMSG without flags = %q", a.readString(buf, n))
	}
}

func TestGetmsg_statuses(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	// A code with no message: VMS 7.3's stand-in, and SS$_MSGNOTFND.
	got, out, r0 := getmsg(t, env, a, 0x7FF8, 15)
	wantFAO(t, got, r0, "%SYSTEM-W-NOMSG, Message number 00007FF8", ssMsgNotFnd)

	if out != 0 {
		t.Errorf("outadr for a missing message = %#x, want 0", out)
	}

	got, _, r0 = getmsg(t, env, a, 0x0999800A, 15)
	wantFAO(t, got, r0, "%NONAME-E-NOMSG, Message number 0999800A", ssMsgNotFnd)

	// A buffer too small.
	bufadr, buf := a.outDesc(8)
	msglen := a.alloc(2)
	wantR0(t, callLNM(t, env, serviceSysGetmsg, testSSAbort, msglen, bufadr, 15), ssBufferOvf)

	if n, _ := env.mem.LoadWord(env.cpu, msglen); n != 8 || a.readString(buf, 8) != "%SYSTEM-" {
		t.Errorf("truncated = %q (length %d), want \"%%SYSTEM-\"", a.readString(buf, n), n)
	}

	// Too few arguments; addresses that can't be written.
	wantR0(t, callLNM(t, env, serviceSysGetmsg, testSSAbort, msglen), ssInsfArg)
	wantR0(t, callLNM(t, env, serviceSysGetmsg, testSSAbort, msglen, 0xFFFFFFF0), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysGetmsg, testSSAbort, 0xFFFFFFF0, bufadr), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysGetmsg, testSSAbort, msglen, bufadr, 15, 0xFFFFFFF0), ssAccVio)
}

// msgvec lays out a message vector: the head longword (count and default
// flags) then the longwords given.
func (a *arena) msgvec(flags uint32, longs ...uint32) uint32 {
	addr := a.alloc(uint32(4 * (len(longs) + 1)))
	putLongword(a.t, a.env, addr, uint32(len(longs))|flags<<16)

	for i, v := range longs {
		putLongword(a.t, a.env, addr+uint32(4*(i+1)), v)
	}

	return addr
}

func TestPutmsg(t *testing.T) {
	env, out := fixture()
	a := newArena(t, env)

	cases := []struct {
		name   string
		msgvec uint32
		facnam uint32
		want   string
	}{
		{
			"the manual's example: a system message, then RMS with a zero STV",
			a.msgvec(0, testSSAbort, testRMSFnf, 0), 0,
			"%SYSTEM-F-ABORT, abort\n-RMS-E-FNF, file not found\n",
		},
		{
			"a system exception message's $FAO parameters",
			a.msgvec(0, testSSAccvio, 4, 0x1000, 0x2000, 0x1B), 0,
			"%SYSTEM-F-ACCVIO, access violation, reason mask=04, virtual address=00001000, PC=00002000, PS=0000001B\n",
		},
		{
			"an RMS STV as a parameter, then as a secondary message",
			a.msgvec(0, testRMSRtb, 300, testRMSFnf, testSSAbort), 0,
			"%RMS-W-RTB, 300 byte record too large for user's buffer\n-RMS-E-FNF, file not found\n-SYSTEM-F-ABORT, abort\n",
		},
		{
			"another facility's parameter count, and new flags for the rest",
			a.msgvec(0, testSSAbort, testLIBBadccc, 2|1<<16, 7, a.ascic("MOD"), testSSDuplnam), 0,
			"%SYSTEM-F-ABORT, abort\nillegal compilation code (7.) in module MOD\nduplicate name\n",
		},
		{
			"the vector's default flags",
			a.msgvec(14, testSSAbort), 0,
			"%SYSTEM-F-ABORT\n",
		},
		{
			"facnam replaces only the first message's facility",
			a.msgvec(0, testSSAbort, testSSDuplnam), a.desc("MYPROG"),
			"%MYPROG-F-ABORT, abort\n-SYSTEM-F-DUPLNAM, duplicate name\n",
		},
		{
			"parameters missing from the vector leave the text unformatted",
			a.msgvec(0, testSSAccvio, 4), 0,
			"%SYSTEM-F-ACCVIO, access violation, reason mask=!XB, virtual address=!XH, PC=!XH, PS=!XL\n",
		},
		{
			"a code with no message",
			a.msgvec(0, 0x7FF8), 0,
			"%SYSTEM-W-NOMSG, Message number 00007FF8\n",
		},
	}

	for _, c := range cases {
		out.Reset()
		wantR0(t, callLNM(t, env, serviceSysPutmsg, c.msgvec, 0, c.facnam, 0), ssNormal)

		if out.String() != c.want {
			t.Errorf("%s: wrote %q, want %q", c.name, out.String(), c.want)
		}
	}

	out.Reset()
	wantR0(t, callLNM(t, env, serviceSysPutmsg, 0), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysPutmsg, 0xFFFFFFF0), ssAccVio)

	if out.Len() != 0 {
		t.Errorf("a vector that can't be read wrote %q", out.String())
	}
}

// TestPutmsg_actionRoutine steps through $PUTMSG's calls of an action
// routine, standing in for the engine: each call's argument list and
// descriptor, the line written only when the routine returns R0 odd, and
// SP restored.
func TestPutmsg_actionRoutine(t *testing.T) {
	env, out := fixture()
	a := newArena(t, env)
	c := env.cpu

	const routine, actprm, fp, sp = 0x5000, 0xABCD, 0x7000, 0x8000

	c.SetGPR(vax.FP, fp)
	c.SetGPR(vax.SP, sp)

	argv := []uint32{a.msgvec(0, testSSAbort, testSSDuplnam), routine, 0, actprm}

	// The first line: a call request, with its argument list on the stack.
	r0, err := serviceSysPutmsg(env, argv)

	call, ok := err.(*CallRequest)
	if !ok || r0 != 0 || call.Routine != routine {
		t.Fatalf("first call = %#x, %v; want a call of %#x", r0, err, routine)
	}

	newSP := c.GPR(vax.SP)
	if call.ArgList != newSP || newSP >= sp {
		t.Fatalf("argument list at %#x, SP %#x; want the argument list at SP, below %#x", call.ArgList, newSP, sp)
	}

	if argc, prm := a.readLong(newSP), a.readLong(newSP+8); argc != 2 || prm != actprm {
		t.Errorf("argument list = %d args, actprm %#x; want 2, %#x", argc, prm, actprm)
	}

	desc := a.readLong(newSP + 4)
	if got, want := a.readLong(desc), uint32(len("%SYSTEM-F-ABORT, abort"))|dscTextStatic; got != want {
		t.Errorf("descriptor's first longword = %#x, want %#x", got, want)
	}

	if got := a.readString(a.readLong(desc+4), 22); got != "%SYSTEM-F-ABORT, abort" {
		t.Errorf("descriptor text = %q", got)
	}

	if out.Len() != 0 {
		t.Fatalf("wrote %q before the action routine returned", out.String())
	}

	// The routine returns 1: the line is written, and the second line's
	// call follows, from the restored SP.
	c.SetGPR(vax.R0, 1)

	if _, err := serviceSysPutmsg(env, argv); err == nil {
		t.Fatal("no call for the second line")
	}

	if out.String() != "%SYSTEM-F-ABORT, abort\n" {
		t.Errorf("after R0=1, wrote %q", out.String())
	}

	if got := a.readLong(a.readLong(c.GPR(vax.SP)+4) + 4); a.readString(got, 1) != "-" {
		t.Error("the second line doesn't start with \"-\"")
	}

	// It returns 0: that line isn't written, and $PUTMSG finishes.
	c.SetGPR(vax.R0, 0)

	r0, err = serviceSysPutmsg(env, argv)

	if err != nil || r0 != ssNormal {
		t.Fatalf("last return = %#x, %v; want SS$_NORMAL", r0, err)
	}

	if out.String() != "%SYSTEM-F-ABORT, abort\n" {
		t.Errorf("after R0=0, wrote %q", out.String())
	}

	if c.GPR(vax.SP) != sp || len(env.Process.putmsg) != 0 {
		t.Errorf("SP = %#x with %d calls pending; want %#x and none", c.GPR(vax.SP), len(env.Process.putmsg), sp)
	}

	// Without actprm, the routine gets one argument.
	argv = argv[:3]
	if _, err := serviceSysPutmsg(env, argv); err == nil {
		t.Fatal("no call")
	}

	if argc := a.readLong(c.GPR(vax.SP)); argc != 1 {
		t.Errorf("without actprm, %d arguments; want 1", argc)
	}

	// Image rundown forgets the call.
	env.ImageRundown()

	if len(env.Process.putmsg) != 0 {
		t.Error("image rundown kept a pending $PUTMSG")
	}
}

// TestMessages_generated pins a few generated message definitions.
func TestMessages_generated(t *testing.T) {
	for code, want := range map[uint32]vmsdef.Message{
		testSSAccvio: {Facility: "SYSTEM", Ident: "ACCVIO", Text: "access violation, reason mask=!XB, virtual address=!XH, PC=!XH, PS=!XL", FAOCount: 4},
		testRMSFnf:   {Facility: "RMS", Ident: "FNF", Text: "file not found"},
		// A line the listing cut before its /FAO qualifier: counted.
		0xC63: {Facility: "SYSTEM", Ident: "EMULATED", Text: "an instruction not implemented on this processor was emulated at PC=!XH, PS=!XL", FAOCount: 2},
		// /ID= replaces the identifier.
		0x15C048: {Facility: "LIB", Ident: "ILLRECLEN", Text: "illegal record length (!UL)", FAOCount: 1},
	} {
		if got, ok := vmsdef.LookupMessage(code); !ok || got != want {
			t.Errorf("LookupMessage(%#x) = %+v, %v; want %+v", code, got, ok, want)
		}
	}
}
