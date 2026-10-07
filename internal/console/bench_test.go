package console

import (
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// BenchmarkSieve is Phase 12's own named performance-pass target
// (docs/PHASE-12.md's scope: "profile the fetch-decode-execute loop under
// bench.asm"): calls bench.asm's own SIEVE routine directly (not MAIN,
// which prints its result through LIB$PUT_OUTPUT-style RTL calls this
// port's console-I/O interrupt gap -- see docs/PHASE-14.md -- would stall
// on) with a fixed argument, so this profiles a pure, self-contained,
// CPU-bound VAX computation: run with
//
//	go test ./internal/console -run '^$' -bench BenchmarkSieve -cpuprofile cpu.prof
//	go tool pprof -top cpu.prof
func BenchmarkSieve(b *testing.B) {
	benchmarkSieve(b, false)
}

// BenchmarkSieveScheduled is BenchmarkSieve with the scheduler installed
// as the engine's hook (vax.process.scheduler on; docs/PHASE-44.md,
// subtask 2), at the default quantum: what one process pays for the
// scheduler's per-instruction count.
func BenchmarkSieveScheduled(b *testing.B) {
	benchmarkSieve(b, true)
}

// benchmarkSieve runs SIEVE b.N times, with or without the scheduler.
func benchmarkSieve(b *testing.B, scheduled bool) {
	c := newRunnableConsole(b)
	if scheduled {
		c.Engine.SetScheduler(c.RTL.System, cpu.PreemptAllModes)
	}

	if _, _, err := c.Assemble(asmFixturePath(b, "bench.asm")); err != nil {
		b.Fatalf("Assemble: %v", err)
	}

	addr, ok := c.Symbols.Get("SIEVE")
	if !ok {
		b.Fatal("expected SIEVE to be defined")
	}

	for b.Loop() {
		if err := c.Call(addr, false, 10000); err != nil {
			b.Fatalf("Call(SIEVE): %v", err)
		}
	}

	if got := c.CPU.GPR(vax.R0); got != 9973 {
		b.Fatalf("R0 = %d, want 9973 (the largest prime below 10000)", got)
	}
}
