package console_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 44's subtask 4: waiting services give the CPU to other processes.
// Each test boots with the scheduler on and a quantum far longer than the
// test runs, so a switch can only come from a wait: without the wait
// states, a waiting process would spin on its service until its quantum
// ended and the other process would never run.

// longQuantum is longer than any of these tests runs.
const longQuantum = "10000000"

// assembleAt assembles a program for codeAddr, with the system services
// it names (SYS$WAKE, ...) defined as their P1-vector entries.
func assembleAt(t *testing.T, src string) ([]byte, *asm.Assembler) {
	t.Helper()

	var defs strings.Builder

	for _, e := range vmsdef.P1VectorTable {
		if strings.Contains(strings.ToUpper(src), e.Name) {
			fmt.Fprintf(&defs, "%s = ^X%08X\n", strings.ToLower(e.Name), e.Addr)
		}
	}

	a := asm.New(false)
	a.SetOrigin(codeAddr)

	code, err := a.Assemble(defs.String() + src)
	if err != nil {
		t.Fatalf("assembling:\n%s\n%v", src, err)
	}

	return code, a
}

// longwordAt reads the longword at addr in env's own P0.
func longwordAt(t *testing.T, c *console.Console, env *corevms.Environment, addr uint32) uint32 {
	t.Helper()

	v, err := c.Mem.LoadLongwordIn(c.CPU, env.Space.AddressSpace, addr)
	if err != nil {
		t.Fatal(err)
	}

	return v
}

// setLongword writes the longword at addr in env's own P0.
func setLongword(t *testing.T, c *console.Console, env *corevms.Environment, addr, v uint32) {
	t.Helper()

	if err := c.Mem.StoreLongwordIn(c.CPU, env.Space.AddressSpace, addr, v); err != nil {
		t.Fatal(err)
	}
}

// stateOf is env's scheduling state.
func stateOf(env *corevms.Environment) sched.State {
	info, _ := env.Scheduler().Info(sched.Handle(env.Process.PID))

	return info.State
}

// pingPongHiber: each turn, count it, wake the other process (its PID at
// dataAddr+4), and hibernate.
const pingPongHiber = `
loop:	incl	@#^X600			; count this turn
	pushl	#0			; prcnam: none
	pushal	@#^X604			; pidadr: the other process
	calls	#2, @#sys$wake
	calls	#0, @#sys$hiber
	brb	loop
`

// TestSchedulerWait_wakeHiber: two processes take turns by waking each
// other and hibernating ($WAKE/$HIBER). Every switch is a wait, and a
// hibernating process is in the HIB state until it's woken.
func TestSchedulerWait_wakeHiber(t *testing.T) {
	code, _ := assembleAt(t, pingPongHiber)

	c, _ := scheduledConsole(t, longQuantum, code)
	one := c.RTL
	two := handBuiltProcess(t, c, code)

	setLongword(t, c, one, dataAddr+4, two.Process.PID)
	setLongword(t, c, two, dataAddr+4, one.Process.PID)

	sawHIB := false

	for range 3000 {
		step(t, c, 1)

		for _, env := range []*corevms.Environment{one, two} {
			if env != c.RTL.Current() && stateOf(env) == sched.StateHIB {
				sawHIB = true
			}
		}
	}

	a, b := countOf(t, c, one), countOf(t, c, two)
	if a < 20 || b < 20 || max(a, b)-min(a, b) > 1 {
		t.Errorf("turns %d and %d: want many each, at most one apart", a, b)
	}

	if !sawHIB {
		t.Error("no process was ever seen hibernating (HIB)")
	}
}

// pingPongCEF: two processes share common event flag cluster 2 (flags
// 64-95), named "PINGPONG", and take turns by flags: process 1 sets 65
// and waits for 64; process 2 waits for 65 and sets 64. Each clears the
// flag it waited for and counts the turn. The flags are .set below.
const pingPongCEF = `
	pushl	#0			; perm
	pushl	#0			; prot
	pushal	@#name
	pushl	#64
	calls	#4, @#sys$ascefc
loop:	pushl	#give
	calls	#1, @#sys$setef
	pushl	#take
	calls	#1, @#sys$waitfr
	pushl	#take
	calls	#1, @#sys$clref
	incl	@#^X600			; count this turn
	brb	loop
name:	.ascid	"PINGPONG"
`

// TestSchedulerWait_commonFlags: two processes take turns through a
// common event flag cluster. A process waiting for a common flag is in
// the CEF state until the other sets it.
func TestSchedulerWait_commonFlags(t *testing.T) {
	codeOne, _ := assembleAt(t, "give = 65\ntake = 64\n"+pingPongCEF)
	codeTwo, _ := assembleAt(t, "give = 64\ntake = 65\n"+pingPongCEF)

	c, _ := scheduledConsole(t, longQuantum, codeOne)
	one := c.RTL
	two := handBuiltProcess(t, c, codeTwo)

	sawCEF := false

	for range 4000 {
		step(t, c, 1)

		for _, env := range []*corevms.Environment{one, two} {
			if env != c.RTL.Current() && stateOf(env) == sched.StateCEF {
				sawCEF = true
			}
		}
	}

	a, b := countOf(t, c, one), countOf(t, c, two)
	if a < 20 || b < 20 || max(a, b)-min(a, b) > 1 {
		t.Errorf("turns %d and %d: want many each, at most one apart", a, b)
	}

	if !sawCEF {
		t.Error("no process was ever seen waiting for a common flag (CEF)")
	}
}

// astWakes: set a timer 10ms ahead whose AST wakes this process, and
// hibernate; when woken, set the longword at dataAddr+8 and spin.
const astWakes = `
	pushl	#0			; flags
	pushl	#0			; reqidt
	pushal	@#wake			; astadr
	pushal	@#delta			; daytim
	pushl	#0			; efn
	calls	#5, @#sys$setimr
	calls	#0, @#sys$hiber
	movl	#1, @#^X608		; woken
spin:	brb	spin
	.entry	wake, ^m<>
	pushl	#0
	pushl	#0
	calls	#2, @#sys$wake		; wake this process
	ret
delta:	.quad	-100000			; 10ms, as a delta time
`

// TestSchedulerWait_astWakesWaiter: a hibernating process with an AST
// to take becomes computable, takes it, and its $HIBER, run again after
// the AST, finds the wakeup the AST routine left. Process 1 counts all
// the while and never waits.
func TestSchedulerWait_astWakesWaiter(t *testing.T) {
	code, _ := assembleAt(t, astWakes)

	c, _ := scheduledConsole(t, "500", counter())
	two := handBuiltProcess(t, c, code)

	for i := 0; i < 200_000 && longwordAt(t, c, two, dataAddr+8) == 0; i++ {
		step(t, c, 1)
	}

	if longwordAt(t, c, two, dataAddr+8) != 1 {
		t.Fatalf("process 2 never woke: state %s", stateOf(two))
	}

	if countOf(t, c, c.RTL) == 0 {
		t.Error("process 1 never ran")
	}
}

// Phase 44's subtask 5: idling, and timers that end waits on time.

// timerWait: note the time ($GETTIM, at dataAddr+0x18), set a timer
// DELTA ahead on event flag 1, wait for it, note the time again (at
// dataAddr+0x10), mark the wait over (dataAddr+8), and hibernate for
// good. DELTA is .set by the caller.
const timerWait = `
	pushal	@#^X618
	calls	#1, @#sys$gettim	; when the timer was set
	pushl	#0			; flags
	pushl	#0			; reqidt
	pushl	#0			; astadr
	pushal	@#delta			; daytim
	pushl	#1			; efn
	calls	#5, @#sys$setimr
	pushl	#1
	calls	#1, @#sys$waitfr
	pushal	@#^X610
	calls	#1, @#sys$gettim	; when it woke
	movl	#1, @#^X608		; done
sleep:	calls	#0, @#sys$hiber
	brb	sleep
delta:	.quad	DELTA
`

// timerProgram assembles timerWait with a delay of ms milliseconds.
func timerProgram(t *testing.T, ms int) []byte {
	t.Helper()

	code, _ := assembleAt(t, strings.Replace(timerWait, "DELTA", fmt.Sprint(-ms*10_000), 1))

	return code
}

// quadAt reads the quadword at addr in env's own P0.
func quadAt(t *testing.T, c *console.Console, env *corevms.Environment, addr uint32) uint64 {
	t.Helper()

	lo, hi := longwordAt(t, c, env, addr), longwordAt(t, c, env, addr+4)

	return uint64(hi)<<32 | uint64(lo)
}

// runUntilDone steps until every env has marked its wait over (at
// dataAddr+8), failing after limit instructions; it returns how many it
// ran.
func runUntilDone(t *testing.T, c *console.Console, limit int, envs ...*corevms.Environment) int {
	t.Helper()

	for n := 0; n < limit; n++ {
		done := true

		for _, env := range envs {
			if longwordAt(t, c, env, dataAddr+8) == 0 {
				done = false
			}
		}

		if done {
			return n
		}

		step(t, c, 1)
	}

	t.Fatalf("not done after %d instructions", limit)

	return limit
}

// waitSlack allows for the instructions between timerWait's two $GETTIMs
// and its timer: vax.init runs the quantum clock at one millisecond per
// instruction (SET QUANTUM 1), so they add a few milliseconds.
const waitSlack = 20

// waitedMS is how long env waited for its timer, in milliseconds of
// system time, and when it woke.
func waitedMS(t *testing.T, c *console.Console, env *corevms.Environment) (float64, uint64) {
	t.Helper()

	set, woke := quadAt(t, c, env, dataAddr+0x18), quadAt(t, c, env, dataAddr+0x10)

	return float64(woke-set) / 10_000, woke
}

// TestSchedulerIdle_timersInOrder: two processes wait for timers, 2s and
// 1s ahead. With nothing else to run, the scheduler idles, jumping the
// quantum clock to each timer in turn: each process wakes in time order,
// on time, after a few dozen instructions instead of the thousands that
// two seconds of spinning would take.
func TestSchedulerIdle_timersInOrder(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, timerProgram(t, 2000))
	one := c.RTL
	two := handBuiltProcess(t, c, timerProgram(t, 1000))

	c.CPU.SetDebug(c.CPU.Debug() | vax.DebugProcess)

	if n := runUntilDone(t, c, 5000, one, two); n > 200 {
		t.Errorf("took %d instructions; idling should have skipped the waits", n)
	}

	waitOne, wokeOne := waitedMS(t, c, one)
	waitTwo, wokeTwo := waitedMS(t, c, two)

	if wokeTwo >= wokeOne {
		t.Errorf("process 2 (1s timer) woke at %d, not before process 1 (2s) at %d", wokeTwo, wokeOne)
	}

	if waitOne < 2000 || waitOne > 2000+waitSlack || waitTwo < 1000 || waitTwo > 1000+waitSlack {
		t.Errorf("waited %.1f and %.1f ms, want 2000 and 1000", waitOne, waitTwo)
	}

	if !strings.Contains(out.String(), "DEBUG(PROCESS): IDLE for") {
		t.Errorf("no idle traced:\n%s", out.String())
	}
}

// TestSchedulerIdle_timerWhileBusy: a process waiting for a timer wakes
// on time while another process computes through a quantum far longer
// than the wait: the scheduler is called when the timer is due, not only
// at quantum ends.
func TestSchedulerIdle_timerWhileBusy(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, counter())
	two := handBuiltProcess(t, c, timerProgram(t, 10))

	runUntilDone(t, c, 100_000, two)

	if waited, _ := waitedMS(t, c, two); waited < 10 || waited > 10+waitSlack {
		t.Errorf("waited %.1f ms, want 10", waited)
	}

	if countOf(t, c, c.RTL) == 0 {
		t.Error("process 1 never ran")
	}
}

// TestSchedulerIdle_hardwareClock: with the host's clock, idling sleeps
// until the timer is due.
func TestSchedulerIdle_hardwareClock(t *testing.T) {
	setSetting(t, "vax.hardware.clock", "true")

	c, _ := scheduledConsole(t, longQuantum, timerProgram(t, 40))
	one := c.RTL
	two := handBuiltProcess(t, c, timerProgram(t, 20))

	begin := time.Now()

	if n := runUntilDone(t, c, 100_000, one, two); n > 1000 {
		t.Errorf("took %d instructions; idling should have slept instead", n)
	}

	if d := time.Since(begin); d < 40*time.Millisecond {
		t.Errorf("finished in %v, before the 40ms timer", d)
	}

	if w, _ := waitedMS(t, c, one); w < 40 {
		t.Errorf("process 1 waited %.1f ms, want 40 at least", w)
	}
}

// TestSchedulerIdle_noTimer: with every process hibernating and no timer
// due, the processes retry their waits, as a lone waiting process always
// has, and the trace says so once.
func TestSchedulerIdle_noTimer(t *testing.T) {
	code, _ := assembleAt(t, "sleep:	calls	#0, @#sys$hiber\n	brb	sleep\n")

	c, out := scheduledConsole(t, longQuantum, code)
	handBuiltProcess(t, c, code)

	c.CPU.SetDebug(c.CPU.Debug() | vax.DebugProcess)

	step(t, c, 500)

	if n := strings.Count(out.String(), "no timer is due"); n != 1 {
		t.Errorf("traced %d times, want once:\n%s", n, out.String())
	}
}
