package console_test

import (
	"fmt"
	"testing"

	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 45's subtask 9: the process-control services on other
// processes, in a booted machine. Process 1 creates a higher-priority
// child that hibernates, suspends it and wakes it (the wakeup is pending
// but the suspended child does not run), and then resumes it and forces
// it to exit, with the status given.

// crossParent is process 1's program, a script of services on the child,
// each group followed by a pause until the test stores 1 at the pause's
// longword (the test looks at the child there). Results are stored from
// dataAddr on: $CREPRC's R0 and the PID at +4, then each service's R0.
const crossParent = `
	callg	crearg, @#sys$creprc
	movl	r0, @#^X600
w1:	tstl	@#^X640
	beql	w1
	callg	susarg, @#sys$suspnd
	movl	r0, @#^X608
	callg	wakarg, @#sys$wake
	movl	r0, @#^X60C
	callg	susarg, @#sys$suspnd
	movl	r0, @#^X610
w2:	tstl	@#^X644
	beql	w2
	callg	resarg, @#sys$resume
	movl	r0, @#^X614
	callg	frcarg, @#sys$forcex
	movl	r0, @#^X618
done:	brb	done
crearg:	.long	11, ^X604, image, 0, 0, 0, 0, 0, 0, %[1]d, 0
resarg:	.long	2, ^X604, 0
susarg:	.long	2, ^X604, 0, 0
wakarg:	.long	2, ^X604, 0
frcarg:	.long	3, ^X604, 0, ^X2C
image:	.word	%[3]d, 0
	.long	imagetext
imagetext: .ascii "%[2]s"
`

func TestCross_suspendedChild(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	one := c.RTL
	exe := buildImage(t, c, "sleeper", sleeperSource)

	code, _ := assembleAt(t, fmt.Sprintf(crossParent, one.Process.BasePriority+2, exe, len(exe)))
	if codeAddr+len(code) > dataAddr+0x40 {
		t.Fatalf("the parent's program (%d bytes) runs into its data", len(code))
	}

	if err := c.Mem.StoreIn(c.CPU, one.Space.AddressSpace, codeAddr, code); err != nil {
		t.Fatal(err)
	}

	// atPause reports whether process 1 is spinning at the pause whose
	// longword is at addr, not yet released: the result stored just
	// before the pause is there.
	atPause := func(addr, result uint32) bool {
		return longwordAt(t, c, one, addr) == 0 && longwordAt(t, c, one, result) != 0 && one.Current() == one
	}

	// The child, of higher priority, ran at once and hibernates.
	runUntil(t, c, 400000, func() bool { return atPause(0x640, dataAddr) })

	if r := longwordAt(t, c, one, dataAddr); r != 1 {
		t.Fatalf("$CREPRC returned %08X", r)
	}

	child, found := one.FindProcess(longwordAt(t, c, one, dataAddr+4))
	if !found {
		t.Fatal("the child isn't in the table")
	}

	stateOf := func() sched.State {
		info, _ := c.RTL.Scheduler().Info(sched.Handle(child.Process.PID))

		return info.State
	}

	if stateOf() != sched.StateHIB || child.Startup != nil {
		t.Fatalf("child's state %s, startup pending %v; want HIB, started", stateOf(), child.Startup != nil)
	}

	setLongword(t, c, one, 0x640, 1)

	// $SUSPND, $WAKE, and a second $SUSPND (already suspended): the
	// wakeup is pending, and the suspended child does not take it.
	runUntil(t, c, 400000, func() bool { return atPause(0x644, dataAddr+0x10) })

	suspended := vmsdef.Symbols["SS$_SUSPENDED"]
	if a, b, d := longwordAt(t, c, one, dataAddr+8), longwordAt(t, c, one, dataAddr+0xC), longwordAt(t, c, one, dataAddr+0x10); a != 1 || b != 1 || d != suspended {
		t.Errorf("$SUSPND returned %08X, $WAKE %08X, $SUSPND again %08X", a, b, d)
	}

	if stateOf() != sched.StateSUSP || !child.Process.WakePending || !child.Suspended() {
		t.Fatalf("suspended child: state %s, wake pending %v; want SUSP, pending", stateOf(), child.Process.WakePending)
	}

	setLongword(t, c, one, 0x644, 1)

	// $RESUME and $FORCEX: the child takes its wakeup, hibernates again,
	// takes the forced exit, and is deleted with the status given.
	runUntil(t, c, 400000, func() bool { return child.Deleted && longwordAt(t, c, one, dataAddr+0x18) != 0 })

	if r, f := longwordAt(t, c, one, dataAddr+0x14), longwordAt(t, c, one, dataAddr+0x18); r != 1 || f != 1 {
		t.Errorf("$RESUME returned %08X, $FORCEX %08X", r, f)
	}

	if child.Process.ExitStatus != 0x2C {
		t.Errorf("the child's exit status is %08X, want 2C", child.Process.ExitStatus)
	}
}
