package console_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/corevms"
)

// Phase 44's subtask 11: determinism. In quantum-clock mode a run with
// several processes depends on nothing but its inputs (docs/PHASE-43.md,
// Part A's rule 4): the same run interleaves its processes the same way,
// instruction for instruction, every time; and a workload whose outcome
// doesn't depend on the interleaving comes out the same whatever the
// quantum.

// pingPongRounds is how many times processes 1 and 2 hand the CPU to
// each other in the workload.
const pingPongRounds = 25

// leader is process 1's part: each round, some work, wake process 2
// (PID at dataAddr+4), and hibernate until it wakes this process back;
// count the round. Then mark done (dataAddr+8) and sleep.
var leader = fmt.Sprintf(`
	movl	#%d, r6
loop:	movl	#10, r7			; some work
work:	sobgtr	r7, work
	pushl	#0
	pushal	@#^X604
	calls	#2, @#sys$wake
	calls	#0, @#sys$hiber
	incl	@#^X600
	sobgtr	r6, loop
	movl	#1, @#^X608		; done
sleep:	calls	#0, @#sys$hiber
	brb	sleep
`, pingPongRounds)

// follower is process 2's part: each round, hibernate until woken, count
// the round, do some work, and wake process 1 back.
var follower = fmt.Sprintf(`
	movl	#%d, r6
loop:	calls	#0, @#sys$hiber
	incl	@#^X600
	movl	#15, r7			; some work
work:	sobgtr	r7, work
	pushl	#0
	pushal	@#^X604
	calls	#2, @#sys$wake
	sobgtr	r6, loop
	movl	#1, @#^X608		; done
sleep:	calls	#0, @#sys$hiber
	brb	sleep
`, pingPongRounds)

// ticker is process 3's part: 20 times, wait 5ms for a timer and count.
const ticker = `
	movl	#20, r6
loop:	pushl	#0			; flags
	pushl	#0			; reqidt
	pushl	#0			; astadr
	pushal	@#delta			; daytim
	pushl	#1			; efn
	calls	#5, @#sys$setimr
	pushl	#1
	calls	#1, @#sys$waitfr
	incl	@#^X600
	sobgtr	r6, loop
	movl	#1, @#^X608		; done
sleep:	calls	#0, @#sys$hiber
	brb	sleep
delta:	.quad	-50000			; 5ms
`

// workloadResult is what a run of the workload produced: which process
// ran each instruction, and each process's count.
type workloadResult struct {
	trace  []uint32
	counts [3]uint32
}

// runWorkload boots with the scheduler on at the given quantum, in
// quantum-clock mode, builds the three processes, and runs until each
// has marked itself done, recording which process ran each instruction.
func runWorkload(t *testing.T, quantum int) workloadResult {
	t.Helper()

	setSetting(t, "vax.hardware.clock", "false")

	leaderCode, _ := assembleAt(t, leader)
	followerCode, _ := assembleAt(t, follower)
	tickerCode, _ := assembleAt(t, ticker)

	c, _ := scheduledConsole(t, fmt.Sprint(quantum), leaderCode)
	procs := []*corevms.Environment{
		c.RTL,
		handBuiltProcess(t, c, followerCode),
		handBuiltProcess(t, c, tickerCode),
	}

	setLongword(t, c, procs[0], dataAddr+4, procs[1].Process.PID)
	setLongword(t, c, procs[1], dataAddr+4, procs[0].Process.PID)

	var r workloadResult

	for range 500_000 {
		if allDone(t, c, procs) {
			for i, env := range procs {
				r.counts[i] = countOf(t, c, env)
			}

			return r
		}

		step(t, c, 1)
		r.trace = append(r.trace, c.RTL.Current().Process.PID)
	}

	t.Fatalf("quantum %d: the workload didn't finish", quantum)

	return r
}

// allDone reports whether every process has marked itself done.
func allDone(t *testing.T, c *console.Console, procs []*corevms.Environment) bool {
	t.Helper()

	for _, env := range procs {
		if longwordAt(t, c, env, dataAddr+8) == 0 {
			return false
		}
	}

	return true
}

// TestDeterminism_sameRunSameTrace: the workload run twice interleaves
// its three processes identically, instruction for instruction.
func TestDeterminism_sameRunSameTrace(t *testing.T) {
	first := runWorkload(t, 7)
	second := runWorkload(t, 7)

	if !slices.Equal(first.trace, second.trace) {
		n := min(len(first.trace), len(second.trace))
		at := n

		for i := range n {
			if first.trace[i] != second.trace[i] {
				at = i

				break
			}
		}

		t.Fatalf("traces differ at instruction %d of %d and %d", at, len(first.trace), len(second.trace))
	}

	// All three really took turns.
	ran := map[uint32]int{}
	for _, pid := range first.trace {
		ran[pid]++
	}

	if len(ran) != 3 {
		t.Errorf("processes that ran: %v, want three", ran)
	}
}

// TestDeterminism_quantaAgree: with very different quanta the processes
// interleave differently, but the workload's outcome is the same.
func TestDeterminism_quantaAgree(t *testing.T) {
	want := [3]uint32{pingPongRounds, pingPongRounds, 20}

	var traces [][]uint32

	for _, q := range []int{3, 50, 1000} {
		r := runWorkload(t, q)

		if r.counts != want {
			t.Errorf("quantum %d: counts %v, want %v", q, r.counts, want)
		}

		traces = append(traces, r.trace)
	}

	if slices.Equal(traces[0], traces[2]) {
		t.Error("quanta 3 and 1000 interleaved identically: the quantum had no effect")
	}
}
