package console

import (
	"testing"

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
	c := newRunnableConsole(b)
	if _, _, err := c.Assemble(asmFixturePath(b, "bench.asm")); err != nil {
		b.Fatalf("Assemble: %v", err)
	}
	
	addr, ok := c.Symbols.Get("SIEVE")
	if !ok {
		b.Fatal("expected SIEVE to be defined")
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := c.Call(addr, false, 10000); err != nil {
			b.Fatalf("Call(SIEVE): %v", err)
		}
	}

	b.StopTimer()

	if got := c.CPU.GPR(vax.R0); got != 9973 {
		b.Fatalf("R0 = %d, want 9973 (the largest prime below 10000)", got)
	}
}
