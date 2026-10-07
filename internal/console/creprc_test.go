package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 45's subtask 4: $CREPRC creates a process, on a machine booted
// as cmd/govax boots it (vax.init), with the scheduler on. The process
// exists and is computable, but its startup (subtask 5) is pending:
// these tests look at it without running it. The checks that refuse a
// request are tested in internal/corevms's creprc_test.go.

// createChild: process 1 calls $CREPRC for a subprocess named WORKER at
// base priority 2 (below process 1's 4, so the new process doesn't
// preempt it), with resource wait mode off (PRC$M_SSRWAIT) and mailbox
// unit 7 for its termination message, and stores R0 at dataAddr; the PID
// goes to dataAddr+4.
const createChild = `
	callg	arglist, @#sys$creprc
	movl	r0, @#^X600
done:	brb	done
arglist: .long	12, ^X604, image, 0, 0, 0, 0, 0, name, 2, 0, 7, 1
image:	.word	9, 0
	.long	imagetext
name:	.word	6, 0
	.long	nametext
imagetext: .ascii "CHILD.EXE"
nametext: .ascii "WORKER"
`

// stepUntilStored runs process 1 until its program has stored a nonzero
// longword at dataAddr.
func stepUntilStored(t *testing.T, c *console.Console) uint32 {
	t.Helper()

	for range 200 {
		step(t, c, 1)

		if v := longwordAt(t, c, c.RTL, dataAddr); v != 0 {
			return v
		}
	}

	t.Fatal("the program never stored its status")

	return 0
}

// TestCreprc_subprocess: a MACRO program's $CREPRC makes a subprocess:
// in the process table and the scheduler (computable, at the priority
// asked for), in process 1's job and owned by it, with its own address
// space, stacks, and PCB, the name, flags, and mailbox unit asked for,
// and its startup pending with the image named.
func TestCreprc_subprocess(t *testing.T) {
	code, _ := assembleAt(t, createChild)
	c, _ := scheduledConsole(t, longQuantum, code)
	one := c.RTL
	sys := one.System

	poolFree := sys.S0Pool().FreePages()

	if r0 := stepUntilStored(t, c); r0 != 1 {
		t.Fatalf("$CREPRC returned %08X, want SS$_NORMAL", r0)
	}

	pid := longwordAt(t, c, one, dataAddr+4)

	child, ok := sys.FindProcess(pid)
	if !ok {
		t.Fatalf("no process %08X (the PID $CREPRC returned)", pid)
	}

	if c.RTL.Current() != one {
		t.Error("the new process (priority 2) preempted process 1 (priority 4)")
	}

	p := child.Process

	if named, found := sys.FindProcessName(one.Process.UICGroup(), "WORKER"); !found || named != child {
		t.Error("no process is named WORKER in process 1's group")
	}

	if p.Owner != one.Process.PID || p.Job != one.Process.Job || one.Process.SubprocessCount != 1 || p.Job.SubprocessCount != 1 {
		t.Errorf("owner %08X, same job %v, counts %d/%d; want process 1's subprocess, counted once",
			p.Owner, p.Job == one.Process.Job, one.Process.SubprocessCount, p.Job.SubprocessCount)
	}

	if p.UIC != one.Process.UIC || p.Username != one.Process.Username {
		t.Errorf("UIC %08X, user %q; want the creator's", p.UIC, p.Username)
	}

	if !p.ResourceWaitDisabled || p.CreateFlags != 1 || p.TerminationMailbox != 7 {
		t.Errorf("resource wait off %v, flags %X, mailbox %d; want true, 1, 7",
			p.ResourceWaitDisabled, p.CreateFlags, p.TerminationMailbox)
	}

	info, _ := sys.Scheduler().Info(sched.Handle(pid))
	if info.State != sched.StateCOM || info.Base != 2 || p.BasePriority != 2 {
		t.Errorf("scheduler: %v at base %d (process says %d); want COM at 2", info.State, info.Base, p.BasePriority)
	}

	if child.Startup == nil || child.Startup.Image != "CHILD.EXE" || child.Startup.Output != "" {
		t.Errorf("startup %+v; want pending, image CHILD.EXE, no output", child.Startup)
	}

	// Its memory: page tables the size of process 1's, its own stacks,
	// and a PCB that starts it in user mode on its stacks.
	if child.Space == nil || child.Space.P0Pages != one.Space.P0Pages || child.Space.P1Pages != one.Space.P1Pages {
		t.Fatalf("address space %+v; want process 1's sizes", child.Space)
	}

	if child.Stacks == nil || child.Stacks.PCB == 0 {
		t.Fatal("no stacks or PCB")
	}

	pcb, err := cpu.ReadPCB(c.Mem, child.Stacks.PCBB)
	if err != nil {
		t.Fatal(err)
	}

	if pcb.P0BR != child.Space.P0BR || pcb.SP[vax.Kernel] != child.Stacks.KSP || pcb.SP[vax.User] != corevms.UserStackTop ||
		pcb.PSL.CurMod() != vax.User {
		t.Errorf("PCB %+v doesn't start the process in user mode on its own stacks and tables", pcb)
	}

	// Page tables (128 + 64 pages for vax.init's sizes), 22 pages of
	// privileged stacks and guard pages, and the PCB's page.
	if used := poolFree - sys.S0Pool().FreePages(); used != 128+64+22+1 {
		t.Errorf("the new process took %d pool pages, want %d", used, 128+64+22+1)
	}

	// $GETJPI's new items report what $CREPRC set.
	if p.CPULimit != 0 || p.BufferedIOLimit != 18 || p.DirectIOLimit != 18 || p.ASTLimit != 24 {
		t.Errorf("quotas CPULM %d, BIOLM %d, DIOLM %d, ASTLM %d; want 0, 18, 18, 24",
			p.CPULimit, p.BufferedIOLimit, p.DirectIOLimit, p.ASTLimit)
	}
}

// TestCreprc_detached: a uic makes a detached process: a job of its own
// (with its own logical-name table and the pooled quotas asked for), no
// owner, its UIC's group, and nothing counted against the creator's job.
func TestCreprc_detached(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, nil)
	one := c.RTL

	child, st := one.CreateProcess(corevms.CreateRequest{
		Name:   "DAEMON",
		UIC:    0x00200001, // [40,1] (octal)
		Image:  "DAEMON.EXE",
		Input:  "NL:",
		Quotas: []corevms.QuotaItem{{Code: vmsdef.Symbols["PQL$_PRCLM"], Value: 3}},
	})
	if st != 1 {
		t.Fatalf("CreateProcess: status %08X", st)
	}

	p := child.Process

	if p.Owner != 0 || p.Job == one.Process.Job || p.Job.MasterPID != p.PID || p.Job.SubprocessLimit != 3 {
		t.Errorf("owner %08X, own job %v, master %08X, PRCLM %d; want a new job with PRCLM 3",
			p.Owner, p.Job != one.Process.Job, p.Job.MasterPID, p.Job.SubprocessLimit)
	}

	if one.Process.SubprocessCount != 0 || one.Process.Job.SubprocessCount != 0 {
		t.Error("a detached process was counted as process 1's subprocess")
	}

	if p.UIC != 0x00200001 || child.Logicals.JobTableName == one.Logicals.JobTableName ||
		child.Logicals.GroupTableName != "LNM$GROUP_000040" {
		t.Errorf("UIC %08X, job table %s (process 1's %s), group table %s",
			p.UIC, child.Logicals.JobTableName, one.Logicals.JobTableName, child.Logicals.GroupTableName)
	}

	if _, found := c.RTL.FindProcessName(0x20, "DAEMON"); !found {
		t.Error("DAEMON isn't found in group 40")
	}

	// The name is unique only within a group: process 1's group can have
	// a DAEMON of its own.
	if _, st := one.CreateProcess(corevms.CreateRequest{Name: "DAEMON"}); st != 1 {
		t.Errorf("a second DAEMON, in another group: status %08X", st)
	}

	if _, st := one.CreateProcess(corevms.CreateRequest{Name: "DAEMON", UIC: 0x00200002}); st != vmsdef.Symbols["SS$_DUPLNAM"] {
		t.Errorf("a second DAEMON in group 40: status %08X, want SS$_DUPLNAM", st)
	}
}

// TestCreprc_limitedCreator: without SETPRV and ALTPRI, a creator gives
// only privileges it has and no priority above its own.
func TestCreprc_limitedCreator(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, nil)
	one := c.RTL

	setprv, altpri := uint64(1)<<vmsdef.Symbols["PRV$V_SETPRV"], uint64(1)<<vmsdef.Symbols["PRV$V_SETPRI"]
	tmpmbx := uint64(1) << vmsdef.Symbols["PRV$V_TMPMBX"]
	one.Process.CurrentPrivileges = tmpmbx | 1<<vmsdef.Symbols["PRV$V_NETMBX"]

	child, st := one.CreateProcess(corevms.CreateRequest{
		Privileges: tmpmbx | setprv | altpri, HasPrivileges: true,
		BasePriority: 10,
	})
	if st != 1 {
		t.Fatalf("CreateProcess: status %08X", st)
	}

	p := child.Process
	if p.CurrentPrivileges != tmpmbx || p.ProcessPrivileges != tmpmbx {
		t.Errorf("privileges %X (permanent %X), want %X: only the creator's", p.CurrentPrivileges, p.ProcessPrivileges, tmpmbx)
	}

	if p.BasePriority != one.Process.BasePriority {
		t.Errorf("base priority %d, want the creator's %d", p.BasePriority, one.Process.BasePriority)
	}

	// With no privilege mask, the creator's current privileges.
	child, _ = one.CreateProcess(corevms.CreateRequest{})
	if child.Process.CurrentPrivileges != one.Process.CurrentPrivileges {
		t.Errorf("privileges %X, want the creator's %X", child.Process.CurrentPrivileges, one.Process.CurrentPrivileges)
	}
}

// TestCreprc_limits: PRCLM, and a pool too full for a new process: both
// refuse, and the pool and process table are as they were.
func TestCreprc_limits(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, nil)
	one := c.RTL
	sys := one.System

	one.Process.Job.SubprocessLimit = 2

	for range 2 {
		if _, st := one.CreateProcess(corevms.CreateRequest{}); st != 1 {
			t.Fatalf("CreateProcess: status %08X", st)
		}
	}

	if _, st := one.CreateProcess(corevms.CreateRequest{}); st != vmsdef.Symbols["SS$_EXQUOTA"] {
		t.Errorf("a third subprocess with PRCLM 2: status %08X, want SS$_EXQUOTA", st)
	}

	// Leave room for the page tables but not the stacks, so the tables
	// are allocated and must be given back. The pool hands out the
	// lowest free run first, so single pages taken from the bottom leave
	// the rest in one run.
	pool := sys.S0Pool()
	room := uint32(128 + 64 + 10)

	for pool.FreePages() > room {
		if _, err := sys.AllocateS0(1, 0x7777, "test filler"); err != nil {
			t.Fatal(err)
		}
	}

	processes := len(sys.Processes())

	one.Process.Job.SubprocessLimit = 10

	_, st := one.CreateProcess(corevms.CreateRequest{Name: "TOOBIG"})
	if st != vmsdef.Symbols["SS$_NOSLOT"] {
		t.Errorf("no room in the pool: status %08X, want SS$_NOSLOT", st)
	}

	if pool.FreePages() != room || len(sys.Processes()) != processes || one.Process.Job.SubprocessCount != 2 {
		t.Errorf("after the failure: %d pool pages free (want %d), %d processes (want %d), job count %d (want 2)",
			pool.FreePages(), room, len(sys.Processes()), processes, one.Process.Job.SubprocessCount)
	}

	for _, a := range pool.Allocations() {
		if strings.Contains(a.Purpose, "table") && a.PID != one.Process.PID && a.PID != 0x7777 {
			if _, ok := sys.FindProcess(a.PID); !ok {
				t.Errorf("pool run %+v belongs to no process", a)
			}
		}
	}
}

// Phase 45's subtask 5: process startup. A created process's first
// dispatch defines its SYS$ names, activates its image in its own P0,
// and calls it as RUN calls process 1's; the image's end stops the
// process (subtask 6 will delete it).

// childSource is the child's image: it writes a line through
// LIB$PUT_OUTPUT, on the terminal it shares with process 1, and returns
// status 3 (a success other than SS$_NORMAL, so the test sees it's the
// image's).
const childSource = `	.title	child
	.psect	code,exe,nowrt
	.entry	start,^m<>
	pushaq	msg
	calls	#1,g^lib$put_output
	movl	#3,r0
	ret
	.psect	data,noexe,wrt
msg:	.ascid	/Hello from the child/
	.end	start
`

// buildChildImage assembles and links childSource with govax's MACRO and
// LINK, returning the image's host path.
func buildChildImage(t *testing.T, c *console.Console) string {
	t.Helper()

	dir := t.TempDir()
	src, obj, exe := filepath.Join(dir, "child.mar"), filepath.Join(dir, "child.obj"), filepath.Join(dir, "child.exe")

	if err := os.WriteFile(src, []byte(childSource), 0o644); err != nil {
		t.Fatal(err)
	}

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	for _, cmd := range []string{
		`MACRO "` + src + `"/OBJECT="` + obj + `"`,
		`LINK "` + obj + `"/EXECUTABLE="` + exe + `"`,
	} {
		if err := d.Dispatch(cmd); err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
	}

	return exe
}

// runUntil runs the machine, as the console's run loop does
// (StepMachine: another process's image end stops that process), until
// done reports true, failing after limit instructions.
func runUntil(t *testing.T, c *console.Console, limit int, done func() bool) {
	t.Helper()

	for range limit {
		if done() {
			return
		}

		if err := c.StepMachine(); err != nil {
			t.Fatalf("at PC %08X: %v", c.CPU.GPR(vax.PC), err)
		}
	}

	t.Fatalf("not done after %d instructions", limit)
}

// TestCreprc_startup: a subprocess runs its image: it starts the first
// time the scheduler switches to it (preempting process 1, at the same
// priority), prints its line on the shared terminal, and stops when its
// image returns, with the image's status, after which process 1 runs on.
// Its SYS$OUTPUT is the output $CREPRC gave, in its own process table,
// and its default directory is process 1's.
func TestCreprc_startup(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	exe := buildChildImage(t, c)
	one := c.RTL

	out.Reset()

	child, st := one.CreateProcess(corevms.CreateRequest{
		Image: exe, Output: "TTA0:", Name: "CHILD", BasePriority: one.Process.BasePriority,
	})
	if st != 1 {
		t.Fatalf("CreateProcess: status %08X", st)
	}

	c.Engine.RequestReschedule()

	sawChild := false

	runUntil(t, c, 100000, func() bool {
		sawChild = sawChild || c.RTL.Current() == child

		return child.Stopped
	})

	if !sawChild {
		t.Error("the child never ran")
	}

	if !strings.Contains(out.String(), "Hello from the child\n") {
		t.Errorf("terminal output %q; want the child's line", out.String())
	}

	if child.Process.ExitStatus != 3 || child.Startup != nil {
		t.Errorf("exit status %08X, startup %+v; want 3, done", child.Process.ExitStatus, child.Startup)
	}

	// Process 1 runs again.
	step(t, c, 2)

	if c.RTL.Current() != one || c.CPU.GPR(vax.PC) != codeAddr {
		t.Errorf("after the child: process %08X at %08X; want process 1 spinning at %08X",
			c.RTL.Current().Process.PID, c.CPU.GPR(vax.PC), codeAddr)
	}

	e, err := child.Logicals.Translate(lnm.ProcessTableName, "SYS$OUTPUT", lnm.User, 0)
	if err != nil || e.Equivalences[0].Value != "TTA0:" || e.Mode != lnm.Executive {
		t.Errorf("the child's SYS$OUTPUT: %+v, %v; want TTA0:, executive mode", e, err)
	}

	if _, err := child.Logicals.Translate(lnm.ProcessTableName, "SYS$INPUT", lnm.User, 0); err == nil {
		t.Error("the child has a SYS$INPUT, though $CREPRC gave none")
	}

	if child.Session == nil || child.Session == one.Session || child.Session.DefaultString() != one.Session.DefaultString() {
		t.Errorf("the child's default directory isn't a copy of process 1's")
	}
}

// TestCreprc_startupHibernates: PRC$M_HIBER makes the new process
// hibernate before its image runs, until process 1 wakes it.
func TestCreprc_startupHibernates(t *testing.T) {
	code, _ := assembleAt(t, `
loop:	tstl	@#^X604			; wait for the child's PID
	beql	loop
	pushl	#0
	pushal	@#^X604
	calls	#2, @#sys$wake
done:	brb	done
`)

	c, out := scheduledConsole(t, longQuantum, code)
	exe := buildChildImage(t, c)
	one := c.RTL

	out.Reset()

	child, st := one.CreateProcess(corevms.CreateRequest{
		Image: exe, BasePriority: one.Process.BasePriority, Flags: vmsdef.Symbols["PRC$M_HIBER"],
	})
	if st != 1 {
		t.Fatalf("CreateProcess: status %08X", st)
	}

	c.Engine.RequestReschedule()

	runUntil(t, c, 100000, func() bool { return stateOf(child) == sched.StateHIB })

	if strings.Contains(out.String(), "Hello") {
		t.Errorf("the hibernating child wrote its line: %q", out.String())
	}

	setLongword(t, c, one, dataAddr+4, child.Process.PID)

	runUntil(t, c, 100000, func() bool { return child.Stopped })

	if !strings.Contains(out.String(), "Hello from the child") || child.Process.ExitStatus != 3 {
		t.Errorf("after $WAKE: output %q, status %08X", out.String(), child.Process.ExitStatus)
	}
}

// TestCreprc_startupFails: a process whose image can't be activated
// stops at once with the failure's status (RMS$_FNF for a missing image;
// SS$_UNSUPPORTED for LOGINOUT, the command interpreter Phase 48 adds),
// and the scheduler chooses again before anything runs: process 1 goes
// on.
func TestCreprc_startupFails(t *testing.T) {
	for _, tt := range []struct {
		image string
		want  uint32
	}{
		{"NOSUCH.EXE", vmsdef.Symbols["RMS$_FNF"]},
		{"", vmsdef.Symbols["RMS$_FNF"]},
		{"SYS$SYSTEM:LOGINOUT.EXE", vmsdef.Symbols["SS$_UNSUPPORTED"]},
	} {
		c, _ := scheduledConsole(t, longQuantum, brbSelf)
		one := c.RTL

		child, st := one.CreateProcess(corevms.CreateRequest{Image: tt.image, BasePriority: 10})
		if st != 1 {
			t.Fatalf("CreateProcess: status %08X", st)
		}

		c.Engine.RequestReschedule()
		step(t, c, 1)

		if !child.Stopped || child.Process.ExitStatus != tt.want {
			t.Errorf("%q: stopped %v, status %08X; want stopped with %08X",
				tt.image, child.Stopped, child.Process.ExitStatus, tt.want)
		}

		if c.RTL.Current() != one || c.CPU.GPR(vax.PC) != codeAddr {
			t.Errorf("%q: process %08X at %08X runs; want process 1 at %08X",
				tt.image, c.RTL.Current().Process.PID, c.CPU.GPR(vax.PC), codeAddr)
		}
	}
}
