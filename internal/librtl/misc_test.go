package librtl

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmsdef"
)

// Addresses the miscellaneous routines' tests use: LIB$WAIT's seconds,
// flags, and float type, and the two channel words of LIB$ASN_WTH_MBX.
const (
	waitSecs   = 0x6000
	waitFlags  = 0x6010
	waitType   = 0x6014
	asnDevChan = 0x6020
	asnMbxChan = 0x6024
	waitFP     = 0xE0000 // the stub frame's FP
)

// waitFixture is fixture with SP and FP set as a stub's frame would
// have them.
func waitFixture(t *testing.T) *corevms.Environment {
	t.Helper()

	env := fixture(t)
	env.CPU().SetGPR(registers.SP, 0xF0000)
	env.CPU().SetGPR(registers.FP, waitFP)

	return env
}

// putSeconds stores seconds at waitSecs in the VAX format f.
func putSeconds(t *testing.T, env *corevms.Environment, f vaxfloat.Format, seconds float64) {
	t.Helper()

	b, err := vaxfloat.Pack(f, vaxfloat.FromFloat64(seconds))
	if err != nil {
		t.Fatal(err)
	}

	putLongword(t, env, waitSecs, uint32(b.Lo))
	putLongword(t, env, waitSecs+4, uint32(b.Lo>>32))
	putLongword(t, env, waitSecs+8, uint32(b.Hi))
	putLongword(t, env, waitSecs+12, uint32(b.Hi>>32))
}

// tryWait calls LIB$WAIT with argv, returning its status and error
// (corevms.ErrWait while it waits).
func tryWait(t *testing.T, env *corevms.Environment, argv ...uint32) (uint32, error) {
	t.Helper()

	return shim(t, env, "LIB$WAIT")(env, argv)
}

// TestWaitHibernates: LIB$WAIT schedules a wakeup and hibernates; a
// retry of the same call waits again without scheduling another, and a
// $WAKE ends it. SP is as it was.
func TestWaitHibernates(t *testing.T) {
	env := waitFixture(t)
	putSeconds(t, env, vaxfloat.F, 30)

	if _, err := tryWait(t, env, waitSecs); !errors.Is(err, corevms.ErrWait) {
		t.Fatalf("first call: %v, want ErrWait", err)
	}

	if _, err := tryWait(t, env, waitSecs); !errors.Is(err, corevms.ErrWait) {
		t.Fatalf("retry: %v, want ErrWait", err)
	}

	if len(waits(env)) != 1 {
		t.Fatalf("pending waits %d, want 1", len(waits(env)))
	}

	if st, err := env.CallService("SYS$WAKE", []uint32{0, 0}); err != nil || st != ssNormal {
		t.Fatalf("$WAKE: %#x, %v", st, err)
	}

	if st, err := tryWait(t, env, waitSecs); err != nil || st != ssNormal {
		t.Fatalf("after $WAKE: %#x, %v", st, err)
	}

	if len(waits(env)) != 0 {
		t.Error("the wait is still remembered")
	}

	if sp := env.CPU().GPR(registers.SP); sp != 0xF0000 {
		t.Errorf("SP %#x, want 0xF0000", sp)
	}
}

// TestWaitPendingWake: a wake already pending ends LIB$WAIT at once.
func TestWaitPendingWake(t *testing.T) {
	env := waitFixture(t)
	putSeconds(t, env, vaxfloat.F, 5)

	if st, err := env.CallService("SYS$WAKE", []uint32{0, 0}); err != nil || st != ssNormal {
		t.Fatalf("$WAKE: %#x, %v", st, err)
	}

	if st, err := tryWait(t, env, waitSecs); err != nil || st != ssNormal {
		t.Errorf("with a wake pending: %#x, %v", st, err)
	}
}

// TestWaitZero: zero seconds is a wakeup already due.
func TestWaitZero(t *testing.T) {
	env := waitFixture(t)
	putLongword(t, env, waitSecs, 0)

	if st, err := tryWait(t, env, waitSecs); err != nil || st != ssNormal {
		t.Errorf("0 seconds: %#x, %v", st, err)
	}
}

// TestWaitNoWake: with LIB$K_NOWAKE the wait is on an event flag from
// LIB$GET_EF's pool (63 at image start), which a $WAKE doesn't end and
// setting the flag does; the flag goes back to the pool.
func TestWaitNoWake(t *testing.T) {
	env := waitFixture(t)
	putSeconds(t, env, vaxfloat.F, 30)
	putLongword(t, env, waitFlags, waitNoWake)

	if _, err := tryWait(t, env, waitSecs, waitFlags); !errors.Is(err, corevms.ErrWait) {
		t.Fatalf("first call: %v, want ErrWait", err)
	}

	if p := waits(env)[waitFP]; p == nil || p.efn != lastHighEF {
		t.Fatalf("pending wait %+v, want flag %d", p, lastHighEF)
	}

	if st, err := env.CallService("SYS$WAKE", []uint32{0, 0}); err != nil || st != ssNormal {
		t.Fatalf("$WAKE: %#x, %v", st, err)
	}

	if _, err := tryWait(t, env, waitSecs, waitFlags); !errors.Is(err, corevms.ErrWait) {
		t.Fatalf("after $WAKE: %v, want ErrWait", err)
	}

	if st, err := env.CallService("SYS$SETEF", []uint32{lastHighEF}); err != nil || st&1 == 0 {
		t.Fatalf("$SETEF: %#x, %v", st, err)
	}

	if st, err := tryWait(t, env, waitSecs, waitFlags); err != nil || st != ssNormal {
		t.Fatalf("after $SETEF: %#x, %v", st, err)
	}

	if got := call(t, env, "LIB$GET_EF", 0x1000); got != ssNormal || longword(t, env, 0x1000) != lastHighEF {
		t.Errorf("GET_EF after the wait: %#x, flag %d, want %d", got, longword(t, env, 0x1000), lastHighEF)
	}
}

// TestWaitNoWakeNoFlag: NOWAKE with no flag free is LIB$_INSEF.
func TestWaitNoWakeNoFlag(t *testing.T) {
	env := waitFixture(t)
	putSeconds(t, env, vaxfloat.F, 1)
	putLongword(t, env, waitFlags, waitNoWake)

	pools(env).flags = math.MaxUint64

	if st, err := tryWait(t, env, waitSecs, waitFlags); err != nil || st != libInsEF {
		t.Errorf("no flag free: %#x, %v, want LIB$_INSEF", st, err)
	}
}

// TestWaitInvalid: LIB$_WRONUMARG for no arguments or four; LIB$_INVARG
// for a negative time, one over 100,000 seconds, a reserved operand, an
// IEEE NaN, and an unknown float type.
func TestWaitInvalid(t *testing.T) {
	env := waitFixture(t)

	if st, _ := tryWait(t, env); st != libWroNumArg {
		t.Errorf("no arguments: %#x, want LIB$_WRONUMARG", st)
	}

	if st, _ := tryWait(t, env, waitSecs, 0, 0, 0); st != libWroNumArg {
		t.Errorf("four arguments: %#x, want LIB$_WRONUMARG", st)
	}

	for _, c := range []struct {
		name  string
		ftype uint32
		set   func()
	}{
		{"negative", floatVAXF, func() { putSeconds(t, env, vaxfloat.F, -1) }},
		{"too long", floatVAXF, func() { putSeconds(t, env, vaxfloat.F, 100_001) }},
		{"G past the limit", floatVAXG, func() { putSeconds(t, env, vaxfloat.G, 100_000.01) }},
		{"NaN", floatIEEET, func() {
			q := math.Float64bits(math.NaN())
			putLongword(t, env, waitSecs, uint32(q))
			putLongword(t, env, waitSecs+4, uint32(q>>32))
		}},
	} {
		c.set()
		putLongword(t, env, waitType, c.ftype)

		if st, err := tryWait(t, env, waitSecs, 0, waitType); err != nil || st != libInvArg {
			t.Errorf("%s: %#x, %v, want LIB$_INVARG", c.name, st, err)
		}
	}

	// A reserved operand signals SS$_ROPRAND, as VMS 7.1 did. (This
	// fixture has no SYS$SRCHANDLER stub to signal through, so the signal
	// fails with an error.)
	putLongword(t, env, waitSecs, 0x8000)
	putLongword(t, env, waitType, floatVAXF)

	if _, err := tryWait(t, env, waitSecs, 0, waitType); err == nil || !strings.Contains(err.Error(), "signal") {
		t.Errorf("reserved operand: err = %v, want a signal", err)
	}
}

// TestWaitUnknownFloatType: a float type past IEEE T is read as F (VMS
// 7.1 took type 6 with SS$_NORMAL).
func TestWaitUnknownFloatType(t *testing.T) {
	env := waitFixture(t)
	putSeconds(t, env, vaxfloat.F, 0)
	putLongword(t, env, waitType, 6)

	if st, err := tryWait(t, env, waitSecs, 0, waitType); err != nil || st != ssNormal {
		t.Errorf("float type 6: %#x, %v, want SS$_NORMAL", st, err)
	}
}

// TestWaitHundredths: seconds in each format, rounded to the nearest
// hundredth.
func TestWaitHundredths(t *testing.T) {
	env := waitFixture(t)

	for _, c := range []struct {
		ftype   uint32
		seconds float64
		want    uint64
	}{
		{floatVAXF, 0.125, 13},
		{floatVAXD, 100_000, 10_000_000},
		{floatVAXG, 2.5, 250},
		{floatVAXH, 0.004, 0},
		{floatIEEES, 0.125, 13},
		{floatIEEET, 1.5, 150},
	} {
		switch c.ftype {
		case floatIEEES:
			putLongword(t, env, waitSecs, math.Float32bits(float32(c.seconds)))
		case floatIEEET:
			q := math.Float64bits(c.seconds)
			putLongword(t, env, waitSecs, uint32(q))
			putLongword(t, env, waitSecs+4, uint32(q>>32))
		default:
			putSeconds(t, env, vaxFloatTypes[c.ftype], c.seconds)
		}

		got, ok, err := waitHundredths(env, waitSecs, c.ftype)
		if err != nil || !ok || got != c.want {
			t.Errorf("type %d, %v s: %d, %v, %v; want %d", c.ftype, c.seconds, got, ok, err, c.want)
		}
	}
}

// TestAstInProg: outside an AST, LIB$AST_IN_PROG is 0.
func TestAstInProg(t *testing.T) {
	env := fixture(t)

	if got := call(t, env, "LIB$AST_IN_PROG"); got != 0 {
		t.Errorf("AST_IN_PROG = %d, want 0", got)
	}
}

// TestAsnWthMbx: the device (a mailbox, by its logical name) and the new
// temporary mailbox each get a channel; a device that doesn't exist is
// $ASSIGN's error, and the mailbox's channel is deassigned.
func TestAsnWthMbx(t *testing.T) {
	env := itemFixture(t, "DVI$_DEVNAM")
	env.CPU().SetGPR(registers.SP, 0xF0000)

	putDescriptor(t, env, itemName, itemNameText, "TARGET")

	if st, err := env.CallService("SYS$CREMBX", []uint32{0, itemChannel, 0, 0, 0, 0, itemName}); err != nil || st != ssNormal {
		t.Fatalf("$CREMBX: %#x, %v", st, err)
	}

	putLongword(t, env, 0x6030, 100)

	if got := call(t, env, "LIB$ASN_WTH_MBX", itemName, 0x6030, 0, asnDevChan, asnMbxChan); got != ssNormal {
		t.Fatalf("ASN_WTH_MBX: %#x", got)
	}

	dev, mbx := word(t, env, asnDevChan), word(t, env, asnMbxChan)
	if dev == 0 || mbx == 0 || dev == mbx {
		t.Fatalf("channels %d and %d", dev, mbx)
	}

	putLongword(t, env, itemChannel, uint32(mbx))

	if got := call(t, env, "LIB$GETDVI", itemCode, itemChannel, 0, 0, itemOut); got != ssNormal || descText(t, env, itemOut)[:4] != "_MBA" {
		t.Errorf("the mailbox's DEVNAM: %#x, %q", got, descText(t, env, itemOut))
	}

	// A device that doesn't exist.
	putDescriptor(t, env, itemName, itemNameText, "XYZ0:")

	got := call(t, env, "LIB$ASN_WTH_MBX", itemName, 0, 0, asnDevChan, asnMbxChan)
	if got&1 != 0 {
		t.Fatalf("no such device: %#x, want a failure", got)
	}

	putLongword(t, env, itemChannel, uint32(word(t, env, asnMbxChan)))

	// $GETDVI's status for a channel that isn't assigned is SS$_IVCHAN.
	if got := call(t, env, "LIB$GETDVI", itemCode, itemChannel, 0, 0, itemOut); got != vmsdef.Symbols["SS$_IVCHAN"] {
		t.Errorf("the mailbox's channel after the failure: %#x, want SS$_IVCHAN", got)
	}
}

// TestGetUsersLanguage: SYS$LANGUAGE's translation; without it the
// vax.language setting, then the host's locale; with none of them,
// ENGLISH and LIB$_ENGLUSED. A fixed string too short is LIB$_STRTRU.
func TestGetUsersLanguage(t *testing.T) {
	env := fixture(t)
	putDesc(t, env, itemOut, 0, dscClassD, 0)

	for _, v := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		t.Setenv(v, "")
	}

	old, had := settings.Get(languageSetting), settings.Exists(languageSetting)
	t.Cleanup(func() {
		if had {
			settings.Set(languageSetting, old)
		} else {
			_ = settings.Delete(languageSetting)
		}
	})

	_ = settings.Delete(languageSetting)

	check := func(name string, wantStatus uint32, want string) {
		t.Helper()

		if got := call(t, env, "LIB$GET_USERS_LANGUAGE", itemOut); got != wantStatus || descText(t, env, itemOut) != want {
			t.Errorf("%s: %#x, %q; want %#x, %q", name, got, descText(t, env, itemOut), wantStatus, want)
		}
	}

	check("nothing set", libEnglUsed, "ENGLISH")

	t.Setenv("LANG", "C")
	check("C locale", libEnglUsed, "ENGLISH")

	t.Setenv("LANG", "de_DE.UTF-8")
	check("LANG", ssNormal, "GERMAN")

	t.Setenv("LC_ALL", "fr_CA")
	check("LC_ALL", ssNormal, "FRENCH")

	settings.Set(languageSetting, "spanish")
	check("vax.language", ssNormal, "SPANISH")

	equivalences := []lnm.Equivalence{{Value: "ITALIAN"}}
	if _, err := env.Logicals.Define(lnm.ProcessTableName, "SYS$LANGUAGE", lnm.User, 0, equivalences); err != nil {
		t.Fatal(err)
	}

	check("SYS$LANGUAGE", ssNormal, "ITALIAN")

	putDescriptor(t, env, itemOut, itemOutText, "ABC")

	// Cut short, the status is still SS$_NORMAL (VMS 7.1).
	if got := call(t, env, "LIB$GET_USERS_LANGUAGE", itemOut); got != ssNormal || descText(t, env, itemOut) != "ITA" {
		t.Errorf("short string: %#x, %q; want SS$_NORMAL, ITA", got, descText(t, env, itemOut))
	}
}
