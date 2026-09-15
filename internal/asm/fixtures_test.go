package asm

import (
	"os"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("../../testdata/asm/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestAssembleFixtures assembles every self-contained testdata/asm program
// (excluding ssdef.asm, an .INCLUDE-only fragment with no code of its own —
// see docs/PHASE-11.md's own open question about it — and kernel.asm/
// forth.asm, which need extra configuration and get their own tests below),
// matching docs/PHASE-11.md's deliverable to assemble every fixture.
func TestAssembleFixtures(t *testing.T) {
	names := []string{
		"atoi.asm", "bench.asm", "dbl.asm", "ff.asm", "float1.asm", "fmt.asm",
		"foo.asm", "hello.asm", "input.asm", "insv.asm", "logname.asm",
		"movc3.asm", "movq.asm", "rotl.asm", "test.asm", "xor.asm",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			a := New()
			out, err := a.Assemble(readFixture(t, name))
			if err != nil {
				t.Fatalf("assemble: %v", err)
			}
			if len(out) == 0 {
				t.Fatal("expected a non-empty assembled program")
			}
		})
	}
}

// TestAssembleForth assembles forth.asm, which needs .MICROKERNEL enabled
// up front: it uses .REGION without a .MICROKERNEL statement of its own,
// matching how the reference tool's own vax.init boots kernel.asm (which
// does declare .MICROKERNEL) before a user might separately ASM forth.asm
// in the same, by-then-already-microkernel-valid console session — see
// pseudoRegion's doc comment.
func TestAssembleForth(t *testing.T) {
	a := New()
	a.SetMicrokernel(true)
	if _, err := a.Assemble(readFixture(t, "forth.asm")); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	// Like kernel.asm, forth.asm switches to S0 (.region s0) near the top
	// and never switches back, so its content lives in the S0 range, not
	// Bytes()'s P0 range.
	s0 := a.BytesRange(a.S0Origin(), a.S0End())
	if len(s0) == 0 {
		t.Fatal("expected non-empty S0 content")
	}
}

// TestAssembleKernel assembles kernel.asm, the microkernel image
// vax.init's own "asm kernel.asm" builds: it needs an .INCLUDE resolver
// (for ssdef.asm) and, per docs/PHASE-11.md's own scope note, exercises
// nearly every microkernel-gated pseudo-op (.MICROKERNEL/.SCB/.SHIM/
// .REGION/.SCOPE/.P1VECTOR). Its bytes end up almost entirely in S0 (it
// switches there early via .REGION SYSTEM and stays there for the rest of
// the file), so this checks the S0 region rather than Bytes()'s P0 range.
func TestAssembleKernel(t *testing.T) {
	a := New()
	a.SetIncludeResolver(func(name string) (string, error) {
		b, err := os.ReadFile("../../testdata/asm/" + name)
		if err != nil {
			return "", err
		}
		return string(b), nil
	})

	if _, err := a.Assemble(readFixture(t, "kernel.asm")); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	s0 := a.BytesRange(a.S0Origin(), a.S0End())
	if len(s0) == 0 {
		t.Fatal("expected non-empty S0 content")
	}

	// .SCB defines exc$chmk's vector, which requires EXC$CHMK (a built-in
	// system symbol, see builtins.go) and exe$chmk (a label kernel.asm
	// defines) to both resolve — a reasonable proxy for "the whole file's
	// forward references and .INCLUDE actually resolved", since an
	// unresolved forward reference would silently leave this slot zero
	// rather than fail the assembly outright (see docs/PHASE-11.md's
	// design notes on why: matching the reference tool's own single-pass,
	// no-mandatory-unresolved-check model).
	sym, ok := a.symbols.find("EXE$CHMK")
	if !ok || len(sym.forward) != 0 {
		t.Fatal("expected EXE$CHMK to be a fully resolved label")
	}
	scbAddr := a.S0Origin() + 0x40 // EXC$CHMK
	got := a.BytesRange(scbAddr, scbAddr+4)
	want := []byte{
		byte(sym.value), byte(sym.value >> 8), byte(sym.value >> 16), byte(sym.value >> 24),
	}
	requireBytes(t, got, want...)
}

// TestRoundTripFixtures assembles each small fixture, disassembles every
// instruction from the program's entry point onward for as long as
// decoding keeps succeeding, reassembles each disassembled instruction on
// its own, and checks it reproduces the same bytes the original assembly
// produced at that address — the round-trip property docs/PHASE-11.md's
// deliverables call for. Decoding necessarily stops at the first place
// code turns into data (a following .ASCII/.BLKB/... block, which would
// otherwise "decode" as nonsense instructions), so this only walks up to
// that point rather than the whole assembled image.
func TestRoundTripFixtures(t *testing.T) {
	cases := []struct {
		name      string
		dataLabel string // the fixture's first data label; "" if it's all code
	}{
		{"hello.asm", "MSG"},
		{"insv.asm", "BITS"},
		{"movq.asm", "DATA"},
		{"xor.asm", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := New()
			out, err := a.Assemble(readFixture(t, tc.name))
			if err != nil {
				t.Fatalf("assemble: %v", err)
			}

			stop := a.origin + uint32(len(out))
			
			if tc.dataLabel != "" {
				sym, ok := a.symbols.find(tc.dataLabel)
				if !ok {
					t.Fatalf("expected label %q to be defined", tc.dataLabel)
				}
				stop = sym.value
			}

			for pc := a.origin; pc < stop; {
				dec, err := Disassemble(a, pc)
				if err != nil {
					t.Fatalf("Disassemble at %08X: %v", pc, err)
				}

				original := a.BytesRange(pc, pc+dec.Length)
				reassembled := assembleBytesAt(t, pc, dec.String())
				if len(reassembled) != len(original) {
					t.Fatalf("at %08X: %s reassembled to %d bytes, want %d (% X vs % X)",
						pc, dec.String(), len(reassembled), len(original), reassembled, original)
				}

				for i := range original {
					if reassembled[i] != original[i] {
						t.Fatalf("at %08X: %s reassembled to % X, want % X",
							pc, dec.String(), reassembled, original)
					}
				}

				pc += dec.Length
			}
		})
	}
}
