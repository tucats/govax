package console_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 45's subtask 8: $DELPRC of another process, whose deletion runs
// in its own context when the scheduler gives it the CPU, and the
// subprocesses of a process deleted before it.

// sleeperSource is an image that hibernates for good.
const sleeperSource = `	.title	sleeper
	.psect	code,exe,nowrt
	.entry	start,^m<>
loop:	calls	#0,g^sys$hiber
	brb	loop
	.end	start
`

// spawnerSource is an image that creates a subprocess of its own,
// running the image %[1]s at base priority %[2]d, and then hibernates
// for good, or, with %[3]s "ret", returns $CREPRC's status. (It returns
// that status too if it can't create the subprocess.)
const spawnerSource = `	.title	spawner
	.psect	data,noexe,wrt
args:	.long	12, pid, image, 0, 0, 0, 0, 0, 0, %[2]d, 0, 0, 0
pid:	.long	0
image:	.ascid	|%[1]s|
	.psect	code,exe,nowrt
	.entry	start,^m<>
	callg	args,g^sys$creprc
	blbc	r0,done
	%[3]s
loop:	calls	#0,g^sys$hiber
	brb	loop
done:	ret
	.end	start
`

// delprcParent is process 1's program: create a subprocess running the
// image %[2]s at base priority %[1]d (its PID at dataAddr+4), $DELPRC
// it, and store $DELPRC's R0 at dataAddr; then spin.
const delprcParent = `
	callg	crearg, @#sys$creprc
	callg	delarg, @#sys$delprc
	movl	r0, @#^X600
done:	brb	done
crearg:	.long	12, ^X604, image, 0, 0, 0, 0, 0, 0, %[1]d, 0, 0, 0
delarg:	.long	2, ^X604, 0
image:	.word	%[3]d, 0
	.long	imagetext
imagetext: .ascii "%[2]s"
`

// runDelprcParent puts delprcParent, for image at base priority baspri,
// in process 1, and runs the machine until $DELPRC has returned and the
// processes it deleted are gone, with process 1 running again. It
// returns $DELPRC's status, the child's PID, and the processes deleted,
// in the order they were deleted.
func runDelprcParent(t *testing.T, c *console.Console, image string, baspri uint32) (status, pid uint32, deleted []*corevms.Environment) {
	t.Helper()

	one := c.RTL

	dropped := one.ProcessDeleted
	one.ProcessDeleted = func(env *corevms.Environment) {
		deleted = append(deleted, env)
		dropped(env)
	}

	code, _ := assembleAt(t, fmt.Sprintf(delprcParent, baspri, image, len(image)))
	if codeAddr+len(code) > dataAddr {
		t.Fatalf("the parent's program (%d bytes) runs into its data", len(code))
	}

	if err := c.Mem.StoreIn(c.CPU, one.Space.AddressSpace, codeAddr, code); err != nil {
		t.Fatal(err)
	}

	runUntil(t, c, 400000, func() bool {
		return longwordAt(t, c, one, dataAddr) != 0 && len(one.Processes()) == 1 && one.Current() == one
	})

	return longwordAt(t, c, one, dataAddr), longwordAt(t, c, one, dataAddr+4), deleted
}

// wantGone checks that env has been deleted, with status SS$_ABORT, and
// that none of its memory is left in the S0 pool.
func wantGone(t *testing.T, c *console.Console, env *corevms.Environment) {
	t.Helper()

	if !env.Deleted {
		t.Errorf("%08X wasn't deleted", env.Process.PID)
	}

	if env.Process.ExitStatus != vmsdef.Symbols["SS$_ABORT"] {
		t.Errorf("%08X's final status is %08X, want SS$_ABORT", env.Process.PID, env.Process.ExitStatus)
	}

	if env.Space.Owned() {
		t.Errorf("%08X's page tables outlived it", env.Process.PID)
	}

	for _, a := range c.RTL.S0Pool().Allocations() {
		if a.PID == env.Process.PID {
			t.Errorf("%08X's %s is still allocated", env.Process.PID, a.Purpose)
		}
	}
}

// TestDelprc_hibernatingChild: process 1 creates a subprocess above its
// own priority, which runs at once and hibernates; process 1's $DELPRC
// of it returns SS$_NORMAL, and the child, made computable, is deleted
// as it gets the CPU, before anything of its own runs; then process 1
// runs on.
func TestDelprc_hibernatingChild(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	one := c.RTL
	exe := buildImage(t, c, "sleeper", sleeperSource)

	status, pid, deleted := runDelprcParent(t, c, exe, one.Process.BasePriority+2)

	if status != 1 {
		t.Errorf("$DELPRC returned %08X", status)
	}

	if len(deleted) != 1 || deleted[0].Process.PID != pid {
		t.Fatalf("%d processes deleted, want the child, %08X", len(deleted), pid)
	}

	wantGone(t, c, deleted[0])

	if one.Process.SubprocessCount != 0 || one.Process.Job.SubprocessCount != 0 {
		t.Errorf("subprocess counts: process 1's %d, the job's %d; want 0",
			one.Process.SubprocessCount, one.Process.Job.SubprocessCount)
	}
}

// TestDelprc_subprocessFirst: process 1 creates a child that creates a
// grandchild, and both hibernate. Process 1's $DELPRC of the child
// deletes the grandchild first: the child's deletion marks it and
// waits, and goes on when the grandchild has gone.
func TestDelprc_subprocessFirst(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	one := c.RTL
	pri := one.Process.BasePriority + 2

	sleeper := buildImage(t, c, "sleeper", sleeperSource)
	spawner := buildImage(t, c, "spawner", fmt.Sprintf(spawnerSource, sleeper, pri, ""))

	status, pid, deleted := runDelprcParent(t, c, spawner, pri)

	if status != 1 {
		t.Errorf("$DELPRC returned %08X", status)
	}

	if len(deleted) != 2 {
		t.Fatalf("%d processes deleted, want 2", len(deleted))
	}

	grandchild, child := deleted[0], deleted[1]
	if child.Process.PID != pid || grandchild.Process.Owner != pid {
		t.Errorf("deleted %08X (owner %08X), then %08X; want the grandchild, then the child %08X",
			grandchild.Process.PID, grandchild.Process.Owner, child.Process.PID, pid)
	}

	for _, env := range deleted {
		wantGone(t, c, env)
	}

	if !slices.Equal(one.Processes(), []*corevms.Environment{one}) {
		t.Errorf("%d processes left, want process 1 alone", len(one.Processes()))
	}
}

// TestDeleteProcess_ownerImageExit: a child whose image returns while
// its own subprocess hibernates isn't deleted at once: its deletion marks
// the grandchild and waits, and the grandchild, deleted first, ends with
// SS$_ABORT; the child then ends with its image's status.
func TestDeleteProcess_ownerImageExit(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	one := c.RTL
	pri := one.Process.BasePriority + 2

	sleeper := buildImage(t, c, "sleeper", sleeperSource)
	spawner := buildImage(t, c, "spawner", fmt.Sprintf(spawnerSource, sleeper, pri, "ret"))

	var deleted []*corevms.Environment

	dropped := one.ProcessDeleted
	one.ProcessDeleted = func(env *corevms.Environment) {
		deleted = append(deleted, env)
		dropped(env)
	}

	child, st := one.CreateProcess(corevms.CreateRequest{Image: spawner, BasePriority: pri})
	if st != 1 {
		t.Fatalf("CreateProcess: status %08X", st)
	}

	c.Engine.RequestReschedule()
	runUntil(t, c, 400000, func() bool { return child.Deleted && one.Current() == one })

	if len(deleted) != 2 || deleted[1] != child || deleted[0].Process.Owner != child.Process.PID {
		t.Fatalf("%d processes deleted; want the grandchild, then the child", len(deleted))
	}

	wantGone(t, c, deleted[0])

	if child.Process.ExitStatus != 1 {
		t.Errorf("the child's final status is %08X, want its image's, 1", child.Process.ExitStatus)
	}

	if len(one.Processes()) != 1 {
		t.Errorf("%d processes left, want process 1 alone", len(one.Processes()))
	}
}
