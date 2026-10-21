package corevms

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/registers"
)

// excmsgHeading is the dump's start, after the message.
const excmsgHeading = "\n  Improperly handled condition, image exit forced.\n\n\tSignal arguments\t      Stack contents\n\n"

// TestImproperlyHandledDump_exception checks the catch-all's dump for an
// access violation that ends the image: the signal array beside ten
// longwords from the program's SP, then the registers, R0 and R1 the
// mechanism array's, SP the program's, FP and AP VMS's own (SP less 68
// and four per signal longword, and FP+40), PC and PSL the signal
// array's (testdata/tb70's layout).
func TestImproperlyHandledDump_exception(t *testing.T) {
	env := conditionFixture(t)
	out := &strings.Builder{}
	env.consoleOut = out
	c := env.cpu

	putLongword(t, env, condFrameA, 0)
	putLongword(t, env, condFrameB, 0)

	for i := range uint32(10) {
		putLongword(t, env, condSP+4*i, 0x100+i)
	}

	for r := registers.R2; r <= registers.R11; r++ {
		c.SetGPR(r, uint32(r)*0x11111111)
	}

	if ok, _ := env.DispatchException(scbAccessViol, []uint32{0x1234, 4}, 0x3000, 0x03C00000); !ok {
		t.Fatal("not dispatched")
	}

	if _, err := serviceSysSrchandler(env, nil); err == nil {
		t.Fatal("expected the catch-all's $EXIT call")
	}

	want := "%SYSTEM-F-ACCVIO, access violation, reason mask=04, virtual address=00001234, PC=00003000, PSL=03C00000\n" +
		excmsgHeading +
		"\tNumber = 00000005\t\t 00000100\n" +
		"\tName   = 0000000C\t\t 00000101\n" +
		"\t\t 00000004\t\t 00000102\n" +
		"\t\t 00001234\t\t 00000103\n" +
		"\t\t 00003000\t\t 00000104\n" +
		"\t\t 03C00000\t\t 00000105\n" +
		"\t\t\t\t\t 00000106\n" +
		"\t\t\t\t\t 00000107\n" +
		"\t\t\t\t\t 00000108\n" +
		"\t\t\t\t\t 00000109\n" +
		"\n\tRegister dump\n\n" +
		"\tR0 = 0000AAAA  R1 = 0000BBBB  R2 = 22222222  R3 = 33333333\n" +
		"\tR4 = 44444444  R5 = 55555555  R6 = 66666666  R7 = 77777777\n" +
		"\tR8 = 88888888  R9 = 99999999  R10= AAAAAAAA  R11= BBBBBBBB\n" +
		fmt.Sprintf("\tAP = %08X  FP = %08X  SP = %08X  PC = 00003000\n", condSP-0x7C+0x40, condSP-0x7C, condSP) +
		"\tPSL= 03C00000\n\n"

	if out.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", out.String(), want)
	}
}

// TestImproperlyHandledDump_signaled checks which conditions get the
// dump when LIB$SIGNAL or LIB$STOP raises them (testdata/tb70's rounds
// 1 and 2): the exceptions' conditions do, however raised, and others
// don't, even severe ones. Its SP is the caller's once the call returns:
// the stub's frame, its argument list, and the CALLS's arguments popped.
func TestImproperlyHandledDump_signaled(t *testing.T) {
	cases := []struct {
		name string
		stop bool
		args []uint32
		dump bool
	}{
		{"LIB$SIGNAL of SS$_ACCVIO", false, []uint32{ssAccVioCond, 4, 0x1234}, true},
		{"LIB$SIGNAL of SS$_OPCDEC", false, []uint32{simpleExceptions[scbPrivileged]}, true},
		{"LIB$STOP of SS$_ACCVIO", true, []uint32{ssAccVioCond, 4, 0x1234}, true},
		{"LIB$SIGNAL of SS$_BADPARAM", false, []uint32{ssBadParam, 1, 2}, false},
		{"LIB$STOP of a warning", true, []uint32{userWarning}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env, out := signalFixture(t)

			putLongword(t, env, condFrameA, 0)
			putLongword(t, env, condFrameB, 0)

			// The stub was called by CALLS #3: its argument list is
			// just above the frame's five longwords.
			putLongword(t, env, signalFrame+frameMaskPSW, 1<<29|0x0C)
			putLongword(t, env, signalFrame+frameSavedRegs, 3)

			if _, err := env.Signal(tc.args, tc.stop); err != nil {
				t.Fatal(err)
			}

			if _, err := serviceSysSrchandler(env, nil); err == nil {
				t.Fatal("expected the catch-all's $EXIT call")
			}

			got := out.String()
			if strings.Contains(got, excmsgHeading) != tc.dump {
				t.Fatalf("dump %v, want %v:\n%s", !tc.dump, tc.dump, got)
			}

			sp := uint32(signalFrame + frameSavedRegs + 16)
			if tc.dump && !strings.Contains(got, fmt.Sprintf("SP = %08X", sp)) {
				t.Errorf("want SP %08X:\n%s", sp, got)
			}
		})
	}
}

// TestImproperlyHandledDump_longSignal checks a signal array longer than
// the stack's ten longwords: its rows go on alone (unconfirmed: VMS's
// probe had none).
func TestImproperlyHandledDump_longSignal(t *testing.T) {
	env, out := signalFixture(t)

	putLongword(t, env, condFrameA, 0)
	putLongword(t, env, condFrameB, 0)

	args := []uint32{ssAccVioCond, 4, 0x1234}
	for i := range uint32(9) {
		args = append(args, 0xF00+i)
	}

	if _, err := env.Signal(args, true); err != nil {
		t.Fatal(err)
	}

	if _, err := serviceSysSrchandler(env, nil); err == nil {
		t.Fatal("expected the catch-all's $EXIT call")
	}

	// 14 longwords after the count: the condition, 11 arguments, PC, and
	// PSL; the stack's column ends beside F05.
	if !strings.Contains(out.String(), "\tNumber = 0000000E\t\t ") ||
		!strings.Contains(out.String(), "\t\t 00000F05\t\t ") ||
		!strings.Contains(out.String(), "\t\t 00003010\n\t\t 0000000C\n\n") {
		t.Errorf("output:\n%s", out.String())
	}
}
