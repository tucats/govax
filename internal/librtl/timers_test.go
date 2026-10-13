package librtl

import (
	"bytes"
	"errors"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vm"
)

// timerFixture is fixture with its output kept, a stack, and a clock the
// test moves by hand (the CPU time follows it: without an engine the
// process has had the CPU since boot).
func timerFixture(t *testing.T) (*corevms.Environment, *bytes.Buffer, *uint64) {
	t.Helper()

	out := &bytes.Buffer{}

	env, err := corevms.NewEnvironment(corevms.NewSystem(registers.New(), vm.NewMemory(1<<20), iodev.NewDeviceTable(),
		rms.NewMountTable()), lnm.NewDatabase(corevms.NominalUIC), bytes.NewReader(nil), out)
	if err != nil {
		t.Fatal(err)
	}

	Register(env.Shims())
	env.CPU().SetGPR(registers.SP, 0xF0000)

	now := env.BootTime
	env.Clock = func() uint64 { return now }

	return env, out, &now
}

// The test's addresses: a handle, a code, and a quadword result.
const (
	timerHandle = 0x1000
	timerCode   = 0x1008
	timerValue  = 0x1010
)

// moveOn advances the clock by 1 day 2:03:04.567 and the counts by 3
// buffered I/Os, 2 direct ones, and 5 page faults.
func moveOn(env *corevms.Environment, now *uint64) {
	*now += ((26*60+3)*60+4)*10_000_000 + 5_670_000
	env.Process.BufferedIO += 3
	env.Process.DirectIO += 2
	env.Process.PageFaults += 5
}

const wantTimerLine = " ELAPSED: 1 02:03:04.56  CPU: 26:03:04.56  BUFIO: 3  DIRIO: 2  FAULTS: 5\n"

// TestShowTimerStatic: the static block, LIB$_INVARG until INIT_TIMER
// fills it; SHOW_TIMER's whole line and each field; codes outside 0 to 5.
// The figures are taken before the line is written, so its own write
// counts in the next call's BUFIO.
func TestShowTimerStatic(t *testing.T) {
	env, out, now := timerFixture(t)

	if got := call(t, env, "LIB$SHOW_TIMER"); got != libInvArg {
		t.Errorf("SHOW_TIMER before INIT_TIMER: %#x, want LIB$_INVARG", got)
	}

	if got := call(t, env, "LIB$INIT_TIMER"); got != ssNormal {
		t.Fatalf("INIT_TIMER: %#x", got)
	}

	moveOn(env, now)

	if got := call(t, env, "LIB$SHOW_TIMER"); got != ssNormal || out.String() != wantTimerLine {
		t.Errorf("SHOW_TIMER: %#x, %q, want %q", got, out.String(), wantTimerLine)
	}

	for code, want := range []string{
		0: wantTimerLine,
		1: " ELAPSED: 1 02:03:04.56\n",
		2: " CPU: 26:03:04.56\n",
		3: " BUFIO: 3\n",
		4: " DIRIO: 2\n",
		5: " FAULTS: 5\n",
	} {
		out.Reset()
		putLongword(t, env, timerCode, uint32(code))

		env.Process.BufferedIO = 3 // each line written is a buffered I/O itself

		if got := call(t, env, "LIB$SHOW_TIMER", 0, timerCode); got != ssNormal || out.String() != want {
			t.Errorf("SHOW_TIMER code %d: %#x, %q, want %q", code, got, out.String(), want)
		}
	}

	out.Reset()

	for _, code := range []uint32{6, 0xFFFFFFFF} {
		putLongword(t, env, timerCode, code)

		if got := call(t, env, "LIB$SHOW_TIMER", 0, timerCode); got != libInvArg {
			t.Errorf("SHOW_TIMER code %d: %#x, want LIB$_INVARG", int32(code), got)
		}
	}

	if out.Len() != 0 {
		t.Errorf("the INVARG calls wrote %q", out.String())
	}

	// Image rundown forgets the static block.
	env.ImageRundown()

	if got := call(t, env, "LIB$SHOW_TIMER"); got != libInvArg {
		t.Errorf("SHOW_TIMER after image rundown: %#x, want LIB$_INVARG", got)
	}
}

// TestStatTimer: each statistic, the elapsed time a negative (delta)
// quadword; codes outside 1 to 5 are LIB$_INVARG.
func TestStatTimer(t *testing.T) {
	env, _, now := timerFixture(t)

	call(t, env, "LIB$INIT_TIMER")
	moveOn(env, now)

	elapsed := uint64(((26*60+3)*60+4)*10_000_000 + 5_670_000)
	delta := -elapsed

	putLongword(t, env, timerCode, timerElapsed)

	if got := call(t, env, "LIB$STAT_TIMER", timerCode, timerValue); got != ssNormal ||
		longword(t, env, timerValue) != uint32(delta) || longword(t, env, timerValue+4) != uint32(delta>>32) {
		t.Errorf("STAT_TIMER 1: %#x, %08X%08X, want %016X", got,
			longword(t, env, timerValue+4), longword(t, env, timerValue), delta)
	}

	for code, want := range map[uint32]uint32{timerCPU: uint32(elapsed / cpuTimeUnit), timerBufIO: 3, timerDirIO: 2, timerFaults: 5} {
		putLongword(t, env, timerCode, code)
		putLongword(t, env, timerValue+4, 0xDEADBEEF)

		if got := call(t, env, "LIB$STAT_TIMER", timerCode, timerValue); got != ssNormal || longword(t, env, timerValue) != want {
			t.Errorf("STAT_TIMER %d: %#x, %d, want %d", code, got, longword(t, env, timerValue), want)
		}

		if longword(t, env, timerValue+4) != 0xDEADBEEF {
			t.Errorf("STAT_TIMER %d wrote more than a longword", code)
		}
	}

	for _, code := range []uint32{0, 6} {
		putLongword(t, env, timerCode, code)

		if got := call(t, env, "LIB$STAT_TIMER", timerCode, timerValue); got != libInvArg {
			t.Errorf("STAT_TIMER %d: %#x, want LIB$_INVARG", code, got)
		}
	}
}

// TestTimerHeapBlock: INIT_TIMER of a handle holding 0 allocates a block
// and writes its address; the block holds the figures; a second
// INIT_TIMER refills it; SHOW_TIMER and STAT_TIMER read it by handle;
// FREE_TIMER frees it and zeroes the handle, after which it, and an
// address INIT_TIMER never gave, are LIB$_INVARG.
func TestTimerHeapBlock(t *testing.T) {
	env, out, now := timerFixture(t)
	env.Process.BufferedIO = 7

	putLongword(t, env, timerHandle, 0)

	if got := call(t, env, "LIB$INIT_TIMER", timerHandle); got != ssNormal {
		t.Fatalf("INIT_TIMER: %#x", got)
	}

	block := longword(t, env, timerHandle)
	if block == 0 {
		t.Fatal("INIT_TIMER wrote no block address")
	}

	if got := longword(t, env, block+timerBlockBufIO); got != 7 {
		t.Errorf("the block's BUFIO is %d, want 7", got)
	}

	if got := longword(t, env, block+timerBlockTime); got != uint32(*now) {
		t.Errorf("the block's time is %#x, want %#x", got, uint32(*now))
	}

	// The static block isn't filled by a heap INIT_TIMER.
	if got := call(t, env, "LIB$SHOW_TIMER"); got != libInvArg {
		t.Errorf("SHOW_TIMER of the static block: %#x, want LIB$_INVARG", got)
	}

	*now += 10_000_000
	env.Process.BufferedIO = 8

	if got := call(t, env, "LIB$INIT_TIMER", timerHandle); got != ssNormal || longword(t, env, timerHandle) != block {
		t.Fatalf("INIT_TIMER again: %#x, block %#x, want %#x", got, longword(t, env, timerHandle), block)
	}

	moveOn(env, now)

	if got := call(t, env, "LIB$SHOW_TIMER", timerHandle); got != ssNormal || out.String() != wantTimerLine {
		t.Errorf("SHOW_TIMER: %#x, %q, want %q", got, out.String(), wantTimerLine)
	}

	putLongword(t, env, timerCode, timerFaults)

	if got := call(t, env, "LIB$STAT_TIMER", timerCode, timerValue, timerHandle); got != ssNormal || longword(t, env, timerValue) != 5 {
		t.Errorf("STAT_TIMER: %#x, %d, want 5", got, longword(t, env, timerValue))
	}

	if got := call(t, env, "LIB$FREE_TIMER", timerHandle); got != ssNormal || longword(t, env, timerHandle) != 0 {
		t.Errorf("FREE_TIMER: %#x, handle %#x", got, longword(t, env, timerHandle))
	}

	if got := call(t, env, "LIB$FREE_TIMER", timerHandle); got != libInvArg {
		t.Errorf("FREE_TIMER of 0: %#x, want LIB$_INVARG", got)
	}

	putLongword(t, env, timerHandle, block)

	for _, name := range []string{"LIB$INIT_TIMER", "LIB$SHOW_TIMER", "LIB$FREE_TIMER"} {
		if got := call(t, env, name, timerHandle); got != libInvArg {
			t.Errorf("%s of a freed block: %#x, want LIB$_INVARG", name, got)
		}
	}

	if got := call(t, env, "LIB$STAT_TIMER", timerCode, timerValue, timerHandle); got != libInvArg {
		t.Errorf("STAT_TIMER of a freed block: %#x, want LIB$_INVARG", got)
	}
}

// TestFreeTimerBadBlock: a block the program freed itself with
// LIB$FREE_VM is LIB$_BADBLOADR to FREE_TIMER.
func TestFreeTimerBadBlock(t *testing.T) {
	env, _, _ := timerFixture(t)

	putLongword(t, env, timerHandle, 0)
	call(t, env, "LIB$INIT_TIMER", timerHandle)

	if !env.FreeVM(longword(t, env, timerHandle)) {
		t.Fatal("FreeVM of the timer block failed")
	}

	if got := call(t, env, "LIB$FREE_TIMER", timerHandle); got != libBadBlockAddress {
		t.Errorf("FREE_TIMER: %#x, want LIB$_BADBLOADR", got)
	}
}

// TestShowTimerAction: with an action routine, SHOW_TIMER writes nothing
// and asks for a tail call of the routine, its argument list below SP:
// 2, a class S descriptor of the line, and the user argument.
func TestShowTimerAction(t *testing.T) {
	env, out, now := timerFixture(t)

	call(t, env, "LIB$INIT_TIMER")
	moveOn(env, now)

	const action = 0x2000

	sp := env.CPU().GPR(registers.SP)

	_, err := shim(t, env, "LIB$SHOW_TIMER")(env, []uint32{0, 0, action, 42})

	var req *corevms.CallRequest
	if !errors.As(err, &req) {
		t.Fatalf("SHOW_TIMER with an action routine: %v, want a CallRequest", err)
	}

	if req.Routine != action || !req.Tail || req.ArgList != env.CPU().GPR(registers.SP) || req.ArgList >= sp {
		t.Fatalf("CallRequest %+v, SP %#x (was %#x)", *req, env.CPU().GPR(registers.SP), sp)
	}

	if n := longword(t, env, req.ArgList); n != 2 || longword(t, env, req.ArgList+8) != 42 {
		t.Errorf("argument list: count %d, value %d; want 2, 42", n, longword(t, env, req.ArgList+8))
	}

	desc := longword(t, env, req.ArgList+4)

	line, err := readString(env, desc)
	if err != nil || line != wantTimerLine[:len(wantTimerLine)-1] {
		t.Errorf("the routine's line: %q, %v", line, err)
	}

	if longword(t, env, desc)>>16 != 0x010E {
		t.Errorf("descriptor %#x, want class S, type T", longword(t, env, desc))
	}

	if out.Len() != 0 {
		t.Errorf("SHOW_TIMER wrote %q", out.String())
	}
}
