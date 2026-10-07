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

// Phase 44's subtask 6: priority preemption.

// setBasePriority gives env base priority pri, as $SETPRI would.
func setBasePriority(t *testing.T, env *corevms.Environment, pri int) {
	t.Helper()

	env.Process.BasePriority, env.Process.Priority = uint32(pri), uint32(pri)

	if err := env.Scheduler().SetBasePriority(sched.Handle(env.Process.PID), pri); err != nil {
		t.Fatal(err)
	}

	// SetBasePriority leaves a waiting process's current priority alone
	// (as VMS does); these tests want it at the new base.
	if err := env.Scheduler().SetCurrentPriority(sched.Handle(env.Process.PID), pri); err != nil {
		t.Fatal(err)
	}
}

// wakeThenMark: wake the process whose PID is at dataAddr+4, then mark
// dataAddr+8 and spin.
const wakeThenMark = `
	pushl	#0
	pushal	@#^X604
	calls	#2, @#sys$wake
	movl	#1, @#^X608		; after the $WAKE
spin:	brb	spin
`

// hiberCount: hibernate; each time woken, count.
const hiberCount = `
loop:	calls	#0, @#sys$hiber
	incl	@#^X600
	brb	loop
`

// TestSchedulerPriority_wakePreempts: a process woken at a higher
// priority than the waker's runs at the next instruction boundary, before
// the waker's next instruction; one woken at a lower priority (even with
// its boost) waits for the CPU.
func TestSchedulerPriority_wakePreempts(t *testing.T) {
	tests := []struct {
		name    string
		base    int
		preempt bool
	}{
		{"higher", 8, true},
		{"lower, boosted short of the waker", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			waker, _ := assembleAt(t, wakeThenMark)
			sleeper, _ := assembleAt(t, hiberCount)

			c, _ := scheduledConsole(t, longQuantum, waker)
			one := c.RTL
			two := handBuiltProcess(t, c, sleeper)

			setLongword(t, c, one, dataAddr+4, two.Process.PID)

			// Process 2, outranking process 1, runs first and hibernates;
			// then it gets the priority under test.
			setBasePriority(t, two, 8)

			for range 100 {
				if stateOf(two) == sched.StateHIB {
					break
				}

				step(t, c, 1)
			}

			if stateOf(two) != sched.StateHIB {
				t.Fatalf("process 2 is %s, not hibernating", stateOf(two))
			}

			setBasePriority(t, two, tt.base)

			// Until process 1's $WAKE has run.
			for range 2000 {
				if two.Process.WakePending {
					break
				}

				step(t, c, 1)
			}

			step(t, c, 1) // the boundary after the $WAKE's XFC

			if got := c.RTL.Current() == two; got != tt.preempt {
				t.Fatalf("process 2 current %v, want %v", got, tt.preempt)
			}

			if tt.preempt && longwordAt(t, c, one, dataAddr+8) != 0 {
				t.Error("process 1 went on past its $WAKE before process 2 ran")
			}

			step(t, c, 1000)

			if n := countOf(t, c, two); (n == 1) != tt.preempt || n > 1 {
				t.Errorf("process 2 woke %d times", n)
			}

			if longwordAt(t, c, one, dataAddr+8) != 1 {
				t.Error("process 1 never went on")
			}
		})
	}
}

// lowerSelf: $SETPRI this process to 2, then mark dataAddr+8 and spin.
const lowerSelf = `
	pushl	#0			; prvpri
	pushl	#2			; pri
	pushl	#0			; prcnam
	pushl	#0			; pidadr
	calls	#4, @#sys$setpri
	movl	#1, @#^X608		; after the $SETPRI
spin:	brb	spin
`

// TestSchedulerPriority_setpriLowers: a process that lowers its priority
// below a computable process's gives it the CPU at the next instruction
// boundary, and gets it back only when that process waits.
func TestSchedulerPriority_setpriLowers(t *testing.T) {
	code, _ := assembleAt(t, lowerSelf)

	c, _ := scheduledConsole(t, longQuantum, code)
	one := c.RTL
	two := handBuiltProcess(t, c, counter())

	setBasePriority(t, two, 3) // below process 1's 4: never runs yet

	for range 1000 {
		if c.RTL.Current() == two {
			break
		}

		step(t, c, 1)
	}

	if c.RTL.Current() != two {
		t.Fatal("process 2 never ran")
	}

	if longwordAt(t, c, one, dataAddr+8) != 0 {
		t.Error("process 1 went on past its $SETPRI before process 2 ran")
	}

	if info, _ := one.Scheduler().Info(sched.Handle(one.Process.PID)); info.Base != 2 || info.Priority != 2 {
		t.Errorf("process 1's priorities %+v, want 2", info)
	}

	step(t, c, 1000)

	if longwordAt(t, c, one, dataAddr+8) != 0 || countOf(t, c, two) < 400 {
		t.Errorf("process 1 ran again (%d) while process 2 (counted %d) outranks it",
			longwordAt(t, c, one, dataAddr+8), countOf(t, c, two))
	}
}
