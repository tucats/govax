package console_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/sched"
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
crearg:	.long	12, ^X604, image, 0, 0, 0, 0, 0, 0, %[1]d, 0, 0, 0
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

	suspended := uint32(1) // VMS 7.1: no error either
	if a, b, d := longwordAt(t, c, one, dataAddr+8), longwordAt(t, c, one, dataAddr+0xC), longwordAt(t, c, one, dataAddr+0x10); a != 1 || b != 1 || d != suspended {
		t.Errorf("$SUSPND returned %08X, $WAKE %08X, $SUSPND again %08X", a, b, d)
	}

	if stateOf() != sched.StateHIB || !child.Process.WakePending || !child.Suspended() {
		t.Fatalf("suspended child: state %s, wake pending %v; want HIB (still), pending", stateOf(), child.Process.WakePending)
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

// TestNullDevice_defined: vax.init defines the null device, NLA0:.
func TestNullDevice_defined(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, brbSelf)

	d, found := c.Devices.Find("NLA0:")
	if !found || d.DevClass != iodev.DeviceClassMailbox || d.DevType != iodev.DeviceTypeNull {
		t.Fatalf("NLA0: found %v, class %v; want the null device (mailbox class, type 3)", found, d)
	}
}

// TestShowDeviceFull_nla0: SHOW DEVICE/FULL NLA0: in the layout the VMS 7.1
// system gives it (docs/PHASE-45.md, probe 2), with govax's own counts, and
// the name taken with or without its colon.
func TestShowDeviceFull_nla0(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	want := strings.Join([]string{
		"Device NLA0:, device type null device, is online, record-oriented device,",
		"    shareable, mailbox device.",
		"",
		"    Error count                    0    Operations completed                  0",
		`    Owner process                 ""    Owner UIC                         [1,1]`,
		"    Owner process ID        00000000    Dev Prot    S:RWPL,O:RWPL,G:RWPL,W:RWPL",
		"    Reference count                0    Default buffer size                 512",
		"",
	}, "\n")

	for _, line := range []string{"SHOW DEVICE/FULL NLA0:", "SHOW DEVICE/FULL nla0"} {
		got, err := sayConsole(t, c, d, line)
		if err != nil {
			t.Fatalf("%s: %v", line, err)
		}

		if got = strings.ReplaceAll(got, "\r", ""); got != want {
			t.Errorf("%s:\n%s\nwant:\n%s", line, got, want)
		}
	}
}

// showMailboxSource creates a permanent mailbox, SHOWBOX, that only the
// system and its owner may use, and ends: the mailbox outlives it.
const showMailboxSource = `	.title	showbox
	.psect	data,noexe,wrt
chan:	.blkw	1
name:	.ascid	/SHOWBOX/
	.psect	code,exe,nowrt
	.entry	start,^m<>
	$crembx_s prmflg=#1,chan=chan,maxmsg=#256,promsk=#^XFF00,lognam=name
	ret
	.end	start
`

// TestShowDeviceFull_terminalAndMailbox: SHOW DEVICE/FULL of TTA0: and of
// a mailbox, in the layouts VMS 7.1 gave a terminal and a mailbox
// (docs/PHASE-45.md, subtask 14's note): "Terminal" first for a
// terminal, its sentence wrapped a word at a time at 80 columns, the
// owner UIC [1,4] as [SYSTEM], and the protection as VMS shows it, the
// mailbox's from its $CREMBX promsk.
func TestShowDeviceFull_terminalAndMailbox(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	if err := c.Run(buildImage(t, c, "showbox", showMailboxSource), console.RunOptions{}); err != nil {
		t.Fatal(err)
	}

	show := func(name string) []string {
		t.Helper()

		got, err := sayConsole(t, c, d, "SHOW DEVICE/FULL "+name)
		if err != nil {
			t.Fatalf("SHOW DEVICE/FULL %s: %v", name, err)
		}

		return strings.Split(strings.ReplaceAll(got, "\r", ""), "\n")
	}

	tt := show("TTA0:")
	want := []string{
		"Terminal TTA0:, device type VT100, is online, record-oriented device, carriage",
		"    control.",
		"",
	}

	if len(tt) < 7 || strings.Join(tt[:3], "\n") != strings.Join(want, "\n") ||
		!strings.HasSuffix(tt[4], "Owner UIC                      [SYSTEM]") ||
		!strings.HasSuffix(tt[5], "Dev Prot              S:RWPL,O:RWPL,G,W") ||
		!strings.HasSuffix(tt[6], "Default buffer size                  80") {
		t.Errorf("TTA0:\n%s", strings.Join(tt, "\n"))
	}

	unit := ""

	for _, dev := range c.Devices.All() {
		if dev.DevClass == iodev.DeviceClassMailbox && dev.DevType != iodev.DeviceTypeNull {
			unit = dev.Name
		}
	}

	if unit == "" {
		t.Fatal("no mailbox")
	}

	mbx := show(unit)
	want = []string{
		"Device " + unit + ":, device type local memory mailbox, is online, record-oriented",
		"    device, shareable, mailbox device.",
		"",
	}

	if len(mbx) < 7 || strings.Join(mbx[:3], "\n") != strings.Join(want, "\n") ||
		!strings.HasSuffix(mbx[4], "Owner UIC                      [SYSTEM]") ||
		!strings.HasSuffix(mbx[5], "Dev Prot              S:RWPL,O:RWPL,G,W") ||
		!strings.HasSuffix(mbx[6], "Default buffer size                 256") {
		t.Errorf("%s:\n%s", unit, strings.Join(mbx, "\n"))
	}
}
