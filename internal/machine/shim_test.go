package machine

import (
	"errors"
	"fmt"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/disasm"
	"github.com/tucats/govax/internal/registers"
)

func TestEnsureShims_definesSymbolsAndIsIdempotent(t *testing.T) {
	c := newRunnableMachine(t)
	c.Engine.SetModeStack(registers.Kernel, false)

	// ensureShims's code-0 entries (e.g. decc$main, below) resolve by
	// looking up kernel.asm's own already-assembled routine by name -- see
	// shim.go's own doc comment -- so it must be assembled first here,
	// matching vax.init's own boot sequence (ASM kernel.asm before any RUN).
	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	if err := c.EnsureShims(); err != nil {
		t.Fatalf("ensureShims: %v", err)
	}

	addr, ok := c.Symbols.Get("SHIM$LIBRTL_00000A70") // lib$adawi, code 1
	if !ok {
		t.Fatal("expected SHIM$LIBRTL_00000A70 to be defined")
	}

	if addr != c.shimBase {
		t.Errorf("first stub address = %#x, want shimBase %#x", addr, c.shimBase)
	}

	addr2, ok := c.Symbols.Get("SHIM$DECC$SHR_00000000") // decc$main, code 0
	if !ok {
		t.Fatal("expected SHIM$DECC$SHR_00000000 to be defined")
	}

	if addr2 == addr {
		t.Error("distinct table entries should get distinct stub addresses")
	}

	// A code-0 entry resolves by symbol lookup against kernel.asm's own
	// real DECC$MAIN routine (see shim.go's own doc comment), not a
	// synthesized stub in the shim page -- so it must fall well outside
	// the shim page's own address range.
	deccMain, ok := c.Symbols.Get("DECC$MAIN")
	if !ok {
		t.Fatal("expected kernel.asm to define DECC$MAIN")
	}

	if addr2 != deccMain {
		t.Errorf("SHIM$DECC$SHR_00000000 = %#x, want kernel.asm's own DECC$MAIN (%#x)", addr2, deccMain)
	}

	if addr2 >= c.shimBase && addr2 < c.shimBase+shimPageBytes {
		t.Errorf("SHIM$DECC$SHR_00000000 = %#x, want an address outside the synthesized shim page", addr2)
	}

	// Idempotent: a second call must not move any stub (a fixup performed
	// after a later RUN must still resolve to the same address).
	if err := c.EnsureShims(); err != nil {
		t.Fatalf("ensureShims (again): %v", err)
	}

	addrAgain, _ := c.Symbols.Get("SHIM$LIBRTL_00000A70")
	if addrAgain != addr {
		t.Errorf("second ensureShims moved the stub: %#x -> %#x", addr, addrAgain)
	}
}

// TestEnsureShims_stubDispatchesThroughXFCShim confirms a synthesized
// "live" (nonzero code) stub is a real, callable procedure: CALLS into it
// dispatches DECC$ISASCII (code 28) via XFC$SHIM and returns its result in
// R0, then RET unwinds cleanly.
func TestEnsureShims_stubDispatchesThroughXFCShim(t *testing.T) {
	c := newRunnableMachine(t)
	c.Engine.SetModeStack(registers.Kernel, false)

	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	if err := c.EnsureShims(); err != nil {
		t.Fatalf("ensureShims: %v", err)
	}

	addr, ok := c.Symbols.Get("SHIM$DECC$SHR_00000028") // decc$isascii, code 28
	if !ok {
		t.Fatal("expected SHIM$DECC$SHR_00000028 to be defined")
	}

	if err := c.Engine.CallEntry(addr); err != nil {
		t.Fatalf("CallEntry: %v", err)
	}

	for {
		err := c.Engine.Step()
		if err == nil {
			continue
		}

		if errors.Is(err, cpu.ErrConsoleCallReturned) {
			break
		}

		t.Fatalf("Step: %v", err)
	}

	if got := c.CPU.GPR(registers.R0); got != 1 {
		t.Errorf("R0 after DECC$ISASCII stub = %d, want 1", got)
	}
}

// TestLibCallg: LIB$CALLG, a LIBRTL routine in VAX code (kernel.asm, a
// code-0 row in librtl.Routines), is reached at its SHIM$ symbol, calls
// the routine it's given with the argument list it's given, and returns
// with that routine's R0 and R1.
func TestLibCallg(t *testing.T) {
	c := newRunnableMachine(t)
	c.Engine.SetModeStack(registers.Kernel, false)

	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	if err := c.EnsureShims(); err != nil {
		t.Fatalf("ensureShims: %v", err)
	}

	callg, ok := c.Symbols.Get("SHIM$LIBRTL_00000810")
	if !ok {
		t.Fatal("expected SHIM$LIBRTL_00000810 (LIB$CALLG) to be defined")
	}

	if entry, _ := c.Symbols.Get("LIB$CALLG"); entry != callg {
		t.Errorf("SHIM$LIBRTL_00000810 = %08X, want kernel.asm's LIB$CALLG, %08X", callg, entry)
	}

	scratch, ok := c.Symbols.Get("CONSOLE$SCRATCH")
	if !ok {
		t.Fatal("expected CONSOLE$SCRATCH to be defined")
	}

	// The routine called: .ENTRY ^M<>; ADDL3 4(AP),8(AP),R0; MOVL #7,R1; RET.
	routine, argList := scratch, scratch+0x40
	code := []byte{0x00, 0x00, 0xC1, 0xAC, 0x04, 0xAC, 0x08, 0x50, 0xD0, 0x07, 0x51, 0x04}

	if err := c.storeBytes(routine, code); err != nil {
		t.Fatal(err)
	}

	for i, v := range []uint32{2, 30, 12} {
		if err := c.Mem.StoreLongword(c.CPU, argList+uint32(4*i), v); err != nil {
			t.Fatal(err)
		}
	}

	if err := c.Engine.CallEntry(callg, argList, routine); err != nil {
		t.Fatalf("CallEntry: %v", err)
	}

	for {
		err := c.Engine.Step()
		if err == nil {
			continue
		}

		if errors.Is(err, cpu.ErrConsoleCallReturned) {
			break
		}

		t.Fatalf("Step: %v", err)
	}

	if r0, r1 := c.CPU.GPR(registers.R0), c.CPU.GPR(registers.R1); r0 != 42 || r1 != 7 {
		t.Errorf("after LIB$CALLG: R0 = %d, R1 = %d; want 42 and 7, the called routine's", r0, r1)
	}
}

// TestShimTable_codesDistinctAndRegistered: every stub's code belongs to
// one routine only, across rtl's shims and internal/librtl's, and each is
// registered in the RTL environment the console builds.
func TestShimTable_codesDistinctAndRegistered(t *testing.T) {
	c := newRunnableMachine(t)
	seen := map[uint32]string{}

	for _, e := range ShimTable {
		if e.Code == 0 {
			continue
		}

		if other, dup := seen[e.Code]; dup {
			t.Errorf("%s and %s share shim code %d", e.Name, other, e.Code)
		}

		seen[e.Code] = e.Name

		if !c.ProcessOne().HasShim(e.Code) {
			t.Errorf("%s: shim code %d isn't registered", e.Name, e.Code)
		}
	}
}

// TestEnsureShims_overflowPages: the stubs that don't fit in the page
// VMInit reserves go to overflow pages from the S0 pool, as many as they
// need (here, every LIBRTL entry's worth and more), so nothing is written
// over the SCB that follows the reserved page; each stub's symbol names it
// where it is, every stub is a whole one at its own address, and the
// stubs in VMInit's page stay where they were.
func TestEnsureShims_overflowPages(t *testing.T) {
	saved := ShimTable

	t.Cleanup(func() { ShimTable = saved })

	const fillers = 400

	ShimTable = append([]corevms.ShimLink{}, saved...)
	for i := 0; i < fillers; i++ {
		ShimTable = append(ShimTable, corevms.ShimLink{Name: fmt.Sprintf("TEST$FILLER%d", i), Library: "TEST", Offset: uint32(i), Code: uint32(1000 + i)})
	}

	c := newRunnableMachine(t)
	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	scb := c.CPU.PR(registers.SCBB)

	before := make([]byte, 512)
	if err := c.Mem.LoadPhysical(scb, before); err != nil {
		t.Fatal(err)
	}

	if err := c.EnsureShims(); err != nil {
		t.Fatalf("ensureShims: %v", err)
	}

	after := make([]byte, 512)
	if err := c.Mem.LoadPhysical(scb, after); err != nil {
		t.Fatal(err)
	}

	if string(before) != string(after) {
		t.Error("ensureShims wrote over the SCB")
	}

	if addr, ok := c.Symbols.Get("SHIM$LIBRTL_00000A70"); !ok || addr != c.shimBase {
		t.Errorf("LIB$ADAWI's stub at %08X (%v); want VMInit's page's first, %08X", addr, ok, c.shimBase)
	}

	seen := map[uint32]string{}

	var err error

	for _, e := range ShimTable {
		if e.Code == 0 {
			continue
		}

		addr, ok := c.Symbols.Get(fmt.Sprintf("SHIM$%s_%08X", e.Library, e.Offset))
		if !ok {
			t.Fatalf("%s: no SHIM$ symbol", e.Name)
		}

		if other, dup := seen[addr]; dup {
			t.Fatalf("%s and %s share stub address %08X", e.Name, other, addr)
		}

		seen[addr] = e.Name

		got := make([]byte, corevms.ShimStubSize)
		for i := range got {
			b, err := c.Mem.LoadByte(c.CPU, addr+uint32(i))
			if err != nil {
				t.Fatalf("%s: reading its stub at %08X: %v", e.Name, addr, err)
			}

			got[i] = b
		}

		want := corevms.ShimStub(e.Code)
		if e.JSB {
			want = corevms.JSBShimStub(e.Code)
		}

		if string(got) != string(want) {
			t.Fatalf("%s: stub at %08X is % X, want % X", e.Name, addr, got, want)
		}
	}

	// The data tables follow the stubs, whole, at their symbols.
	for _, e := range ShimTable {
		if e.Data == nil {
			continue
		}

		addr, ok := c.Symbols.Get(fmt.Sprintf("SHIM$%s_%08X", e.Library, e.Offset))
		if !ok {
			t.Fatalf("%s: no SHIM$ symbol", e.Name)
		}

		if other, dup := seen[addr]; dup {
			t.Fatalf("%s and %s share address %08X", e.Name, other, addr)
		}

		got := make([]byte, len(e.Data))
		for i := range got {
			if got[i], err = c.Mem.LoadByte(c.CPU, addr+uint32(i)); err != nil {
				t.Fatalf("%s: reading it at %08X: %v", e.Name, addr, err)
			}
		}

		if string(got) != string(e.Data) {
			t.Errorf("%s at %08X isn't the table", e.Name, addr)
		}
	}

	last := fmt.Sprintf("TEST$FILLER%d", fillers-1)

	addr, ok := c.Symbols.Get(last)
	if !ok || (addr >= c.shimBase && addr < c.shimBase+shimPageBytes) {
		t.Errorf("%s at %08X (%v); want it in an overflow page, not VMInit's at %08X", last, addr, ok, c.shimBase)
	}
}

// TestEnsureShims_placesASTExit: EnsureShims writes EXE$ASTRET, the AST
// exit, in S0 after the table's stubs (docs/PHASE-70 - traceback.md,
// subtask 9), a JMP to the SYS$CLRAST P1-vector entry's XFC, and
// ASTExit returns its address only once it's there.
func TestEnsureShims_placesASTExit(t *testing.T) {
	c := newRunnableMachine(t)
	c.Engine.SetModeStack(registers.Kernel, false)

	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	if a := c.ASTExit(); a != 0 {
		t.Errorf("ASTExit before EnsureShims = %#x, want 0", a)
	}

	if err := c.EnsureShims(); err != nil {
		t.Fatalf("EnsureShims: %v", err)
	}

	exit := c.ASTExit()
	if exit < maxP1 {
		t.Fatalf("ASTExit = %#x, want an S0 address", exit)
	}

	for _, e := range ShimTable {
		if addr, ok := c.Symbols.Get(fmt.Sprintf("SHIM$%s_%08X", e.Library, e.Offset)); ok && e.Code != 0 && e.Data == nil && addr == exit {
			t.Errorf("EXE$ASTRET shares %s's stub at %#x", e.Name, exit)
		}
	}

	dec, err := disasm.Disassemble(memByteReader{c}, exit)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := dec.String(), "JMP @#^X7FFEE10A"; got != want {
		t.Errorf("EXE$ASTRET's instruction = %q, want %q", got, want)
	}
}
