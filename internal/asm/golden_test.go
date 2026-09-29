package asm

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite internal/asm/testdata/golden from the current assembler")

// TestGoldenFixtures assembles every testdata/asm fixture and compares the
// whole result (every byte written, every symbol with its value and flags,
// the final location counters, the .END entry, and any error) with a
// snapshot in testdata/golden. It guards refactors that must not change
// what the assembler produces (docs/PHASE-27.md, subtask 4). Run with
// -update to rewrite the snapshots after a deliberate change.
//
// Each fixture is assembled twice: on its own, and after kernel.asm in the
// same Assembler, the way a console session that booted the microkernel
// assembles it (many fixtures call kernel.asm's routines).
func TestGoldenFixtures(t *testing.T) {
	names, err := filepath.Glob("../../testdata/asm/*.asm")
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range names {
		name := filepath.Base(path)

		t.Run(name, func(t *testing.T) {
			var sb strings.Builder

			sb.WriteString("== alone\n")
			dumpAssembly(&sb, goldenAssembler(t), readFixture(t, name))

			if name != "kernel.asm" {
				a := goldenAssembler(t)
				if _, err := a.Assemble(readFixture(t, "kernel.asm")); err != nil {
					t.Fatalf("kernel.asm: %v", err)
				}

				before := snapshotKeys(a)

				sb.WriteString("== after kernel.asm\n")
				dumpAssemblyExcept(&sb, a, readFixture(t, name), before)
			}

			compareGolden(t, strings.TrimSuffix(name, ".asm")+".golden", sb.String())
		})
	}
}

func goldenAssembler(t *testing.T) *Assembler {
	t.Helper()

	a := New(true)
	a.SetMicrokernel(true)
	a.SetIncludeResolver(func(name string) (string, error) {
		b, err := os.ReadFile("../../testdata/asm/" + name)

		return string(b), err
	})

	return a
}

// goldenKeys is what an Assembler held before the fixture was assembled,
// so the "after kernel.asm" dump shows only what the fixture changed.
type goldenKeys struct {
	bytes   map[uint32]byte
	symbols map[string]string
}

func snapshotKeys(a *Assembler) goldenKeys {
	k := goldenKeys{bytes: map[uint32]byte{}, symbols: map[string]string{}}

	for addr, b := range a.image.bytes {
		k.bytes[addr] = b
	}

	for name, s := range a.symbols.byName {
		k.symbols[name] = symbolLine(s)
	}

	return k
}

func dumpAssembly(sb *strings.Builder, a *Assembler, src string) {
	dumpAssemblyExcept(sb, a, src, goldenKeys{})
}

func dumpAssemblyExcept(sb *strings.Builder, a *Assembler, src string, before goldenKeys) {
	out, err := a.Assemble(src)
	if err != nil {
		fmt.Fprintf(sb, "error: %v\n", err)
	}

	fmt.Fprintf(sb, "bytes: %d\n", len(out))
	fmt.Fprintf(sb, "deposit: %08X  p0 end: %08X  s0 end: %08X\n", a.Deposit(), a.p0End(), a.S0End())

	if addr, ok := a.Entry(); ok {
		fmt.Fprintf(sb, "entry: %08X\n", addr)
	}

	if base, end, ok := a.P1VectorRange(); ok {
		fmt.Fprintf(sb, "p1vector: %08X-%08X\n", base, end)
	}

	for _, p := range a.Prints() {
		fmt.Fprintf(sb, "print: %s\n", p)
	}

	// The image, as runs of consecutive changed addresses.
	addrs := make([]uint32, 0, len(a.image.bytes))

	for addr, b := range a.image.bytes {
		if old, ok := before.bytes[addr]; ok && old == b {
			continue
		}

		addrs = append(addrs, addr)
	}

	sort.Slice(addrs, func(i, j int) bool { return addrs[i] < addrs[j] })

	for i := 0; i < len(addrs); {
		j := i + 1
		for j < len(addrs) && addrs[j] == addrs[j-1]+1 && j-i < 16 {
			j++
		}

		fmt.Fprintf(sb, "%08X:", addrs[i])

		for _, addr := range addrs[i:j] {
			fmt.Fprintf(sb, " %02X", a.image.bytes[addr])
		}

		sb.WriteString("\n")

		i = j
	}

	names := make([]string, 0, len(a.symbols.byName))

	for name, s := range a.symbols.byName {
		if s.flags&SymBuiltin != 0 {
			continue
		}

		if old, ok := before.symbols[name]; ok && old == symbolLine(s) {
			continue
		}

		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		fmt.Fprintf(sb, "sym %s %s\n", name, symbolLine(a.symbols.byName[name]))
	}
}

func symbolLine(s *symbol) string {
	return fmt.Sprintf("%08X flags=%X forward=%d", s.value, uint32(s.flags), len(s.forward))
}

func compareGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", "golden", name)

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}

		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update to create it)", err)
	}

	if string(want) == got {
		return
	}

	wl, gl := strings.Split(string(want), "\n"), strings.Split(got, "\n")
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}

		if i < len(gl) {
			g = gl[i]
		}

		if w != g {
			t.Fatalf("%s differs at line %d:\nwant: %s\n got: %s", path, i+1, w, g)
		}
	}
}
