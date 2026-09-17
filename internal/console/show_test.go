package console

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// newShowDispatcher returns a Dispatcher/Console/output-buffer trio for a
// freshly Init'd (but not VMInit'd) machine — MAPEN stays 0, so
// LoadLongword/StoreLongword through c.Mem address physical memory
// directly, useful for tests that want to seed memory without a page
// table.
func newShowDispatcher(t *testing.T) (*Dispatcher, *Console, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer

	c := New(&buf)
	if err := c.Init(64 * 1024); err != nil {
		t.Fatalf("Init: %v", err)
	}

	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	return d, c, &buf
}

// newShowRunnableDispatcher is newShowDispatcher's VMINIT'd counterpart —
// MAPEN ends up 1 (see vminit.go), for tests that need real P0/S0 page
// tables (SHOW PAGE).
func newShowRunnableDispatcher(t *testing.T) (*Dispatcher, *Console, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer

	c := New(&buf)
	if err := c.Init(4096 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(2000, 100, 0, 4, 4, 4, 4, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	g := loadEvaxGrammar(t)
	d := NewDispatcher(c, g, nil)

	return d, c, &buf
}

func TestShowMode(t *testing.T) {
	d, _, buf := newShowDispatcher(t)

	if err := d.Dispatch("SHOW MODE"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if !strings.Contains(buf.String(), "KERNEL") {
		t.Errorf("output = %q, want it to mention KERNEL (the default cur_mod)", buf.String())
	}
}

func TestShowNVRAM_none(t *testing.T) {
	d, _, buf := newShowDispatcher(t)

	if err := d.Dispatch("SHOW NVRAM"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if !strings.Contains(buf.String(), "No NVRAM") {
		t.Errorf("output = %q, want a \"No NVRAM\" message", buf.String())
	}
}

func TestShowROM_loaded(t *testing.T) {
	d, c, buf := newShowDispatcher(t)

	if err := c.LoadROM(romFixturePath(t)); err != nil {
		t.Fatalf("LoadROM: %v", err)
	}

	buf.Reset()

	if err := d.Dispatch("SHOW ROM"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "20040000") || !strings.Contains(out, romFixturePath(t)) {
		t.Errorf("output = %q, want it to mention the ROM base and file path", out)
	}
}

func TestShowShim(t *testing.T) {
	d, c, buf := newShowRunnableDispatcher(t)

	if err := d.Dispatch("SHOW SHIM"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if !strings.Contains(buf.String(), "No RTL shims") {
		t.Errorf("output = %q, want a \"No RTL shims\" message before any are synthesized", buf.String())
	}

	// ensureShims's code-0 entries resolve by looking up kernel.asm's own
	// already-assembled routine by name (see shim.go's own doc comment), so
	// it must be assembled first here, matching vax.init's own boot
	// sequence.
	if _, _, err := c.Assemble(asmFixturePath(t, "kernel.asm")); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	if err := c.ensureShims(); err != nil {
		t.Fatalf("ensureShims: %v", err)
	}

	buf.Reset()

	if err := d.Dispatch("SHOW SHIM"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "SHIM$") {
		t.Errorf("output = %q, want at least one SHIM$ entry", out)
	}

	if !strings.Contains(out, "live") {
		t.Errorf("output = %q, want at least one shim reported live (e.g. DECC$TIME)", out)
	}
}

func TestShowString(t *testing.T) {
	d, c, buf := newShowDispatcher(t)

	// Fabricate a two-entry CONSOLE$STRINGPOOL chain directly in memory
	// (MAPEN is 0 here, so these are also physical addresses) rather than
	// booting a real microkernel just to exercise the walk.
	const (
		base    = 0x1000 // holds the head-of-chain pointer
		descOne = 0x2000
		strOne  = 0x3000
	)

	c.Symbols.Set("CONSOLE$STRINGPOOL_BASE", base, SymbolSystem)
	c.Symbols.Set("CONSOLE$STRINGPOOL_SIZE", 0x100, SymbolSystem)
	c.Symbols.Set("CONSOLE$STRINGPOOL", descOne, SymbolSystem)

	if err := c.Mem.StoreLongword(c.CPU, base, descOne); err != nil {
		t.Fatalf("seed chain head: %v", err)
	}

	text := "HELLO"
	if err := c.Mem.StoreLongword(c.CPU, descOne, uint32(len(text))); err != nil {
		t.Fatalf("seed descriptor length: %v", err)
	}

	if err := c.Mem.StoreLongword(c.CPU, descOne+4, strOne); err != nil {
		t.Fatalf("seed descriptor address: %v", err)
	}

	if err := c.Mem.StoreLongword(c.CPU, descOne+8, 0); err != nil {
		t.Fatalf("seed next pointer: %v", err)
	}

	if err := c.Mem.Store(c.CPU, strOne, []byte(text)); err != nil {
		t.Fatalf("seed string bytes: %v", err)
	}

	if err := d.Dispatch("SHOW STRING"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "HELLO") {
		t.Errorf("output = %q, want it to contain the descriptor's string %q", out, text)
	}
}

// TestShowPage_beforeFirstTouch covers this port's DYNVM demand paging
// (internal/vm.Memory.AllocatePage, wired into Translate): a freshly
// VMINIT'd P0 page has no physical page assigned yet, matching
// console_vminit.c's own `#ifdef DYNVM` PTE-creation branch, so SHOW
// PAGE's read-only tracevm-style report (LookupPTE never demand-pages)
// shows it invalid until something actually accesses it.
func TestShowPage_beforeFirstTouch(t *testing.T) {
	d, _, buf := newShowRunnableDispatcher(t)

	if err := d.Dispatch("SHOW PAGE 200"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "VALID: 0") {
		t.Errorf("output = %q, want an invalid, not-yet-demand-paged PTE", out)
	}

	if !strings.Contains(out, "Region:            00") {
		t.Errorf("output = %q, want region 00 (P0) for address 0x200", out)
	}
}

func TestShowPage(t *testing.T) {
	d, c, buf := newShowRunnableDispatcher(t)

	// Touch the page first, so Translate's demand paging assigns it a real
	// physical page before SHOW PAGE reports on it.
	if err := c.Mem.StoreLongword(c.CPU, 0x200, 0); err != nil {
		t.Fatalf("StoreLongword through P0: %v", err)
	}

	if err := d.Dispatch("SHOW PAGE 200"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "VALID: 1") {
		t.Errorf("output = %q, want a valid PTE for a demand-paged P0 page", out)
	}

	if !strings.Contains(out, "Region:            00") {
		t.Errorf("output = %q, want region 00 (P0) for address 0x200", out)
	}
}

func TestShowSCB(t *testing.T) {
	d, c, buf := newShowDispatcher(t)

	const scbBase = 0x4000

	c.CPU.SetPR(vax.SCBB, scbBase)

	// Slot 0x40 (CHMK, index 16) gets a real vector; every other slot
	// stays zero and so is skipped by the default (non-/ALL) dump.
	if err := c.Mem.StoreLongword(c.CPU, scbBase+0x40, 0x00012340); err != nil {
		t.Fatalf("seed CHMK vector: %v", err)
	}

	c.Symbols.Set("EXE$CHMK_HANDLER", 0x00012340, SymbolUser)

	if err := d.Dispatch("SHOW SCB"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "EXC$CHMK") {
		t.Errorf("output = %q, want the CHMK slot reported", out)
	}

	if !strings.Contains(out, "EXE$CHMK_HANDLER") {
		t.Errorf("output = %q, want the vector resolved back to its symbol name", out)
	}

	if strings.Count(out, "EXC$") > 1 {
		t.Errorf("output = %q, want only the one populated slot without /ALL", out)
	}
}

func TestShowCallFrames(t *testing.T) {
	d, c, buf := newShowDispatcher(t)

	const (
		fp = 0x1000
		ap = 0x2000
	)

	c.CPU.SetGPR(vax.FP, fp)
	c.CPU.SetGPR(vax.AP, ap)

	if err := c.Mem.StoreLongword(c.CPU, fp, 0); err != nil { // no condition handler
		t.Fatalf("seed handler: %v", err)
	}

	if err := c.Mem.StoreLongword(c.CPU, fp+4, 0); err != nil { // mask word: CALLG, no saved regs
		t.Fatalf("seed mask word: %v", err)
	}

	if err := c.Mem.StoreLongword(c.CPU, fp+8, 0); err != nil { // saved AP
		t.Fatalf("seed saved AP: %v", err)
	}

	if err := c.Mem.StoreLongword(c.CPU, fp+12, 0); err != nil { // saved FP == 0: last frame
		t.Fatalf("seed saved FP: %v", err)
	}

	if err := c.Mem.StoreLongword(c.CPU, fp+16, 0x00001234); err != nil { // saved PC
		t.Fatalf("seed saved PC: %v", err)
	}

	if err := c.Mem.StoreLongword(c.CPU, ap, 0); err != nil { // argc == 0
		t.Fatalf("seed argc: %v", err)
	}

	if err := d.Dispatch("SHOW CALL_FRAMES"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "FRAME: 00001000") {
		t.Errorf("output = %q, want the frame at FP", out)
	}

	if !strings.Contains(out, "Saved PC : 00001234") {
		t.Errorf("output = %q, want the saved PC", out)
	}

	if !strings.Contains(out, "CALLG") {
		t.Errorf("output = %q, want the CALLG calltype", out)
	}
}

func TestShowCallFrames_noFrames(t *testing.T) {
	d, _, _ := newShowDispatcher(t)

	if err := d.Dispatch("SHOW CALL_FRAMES"); err == nil {
		t.Error("expected an error when FP/AP are both zero (no call frames)")
	}
}

func TestShowRegions(t *testing.T) {
	d, c, buf := newShowDispatcher(t)

	c.RTL.RegionSize[0] = 0x1000
	c.RTL.RegionSize[1] = 0x2000
	c.RTL.RegionSize[2] = 0x3000

	if err := d.Dispatch("SHOW REGIONS"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"P0   00001000", "P1   00002000", "S0   00003000"} {
		if !strings.Contains(out, want) {
			t.Errorf("output = %q, want it to contain %q", out, want)
		}
	}
}

func TestShowSharePrefix(t *testing.T) {
	cases := []struct {
		prefix string
		want   string
	}{
		{"", "No sharable image prefix defined"},
		{"/images/", "Sharable images loaded from \"/images/\""},
		{"\x1B", "Recursive sharable image loading disabled"},
	}

	for _, tc := range cases {
		d, c, buf := newShowDispatcher(t)
		c.SharePrefix = tc.prefix

		if err := d.Dispatch("SHOW SHARE_PREFIX"); err != nil {
			t.Fatalf("Dispatch: %v", err)
		}

		if !strings.Contains(buf.String(), tc.want) {
			t.Errorf("SharePrefix=%q: output = %q, want it to contain %q", tc.prefix, buf.String(), tc.want)
		}
	}
}

func TestShowImages(t *testing.T) {
	d, c, buf := newShowDispatcher(t)

	if err := d.Dispatch("SHOW IMAGES"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if !strings.Contains(buf.String(), "No VMS images") {
		t.Errorf("output = %q, want a \"no images\" message with an empty ICBList", buf.String())
	}

	c.ICBList = append(c.ICBList, &ICB{Name: "TEST.EXE", Base: 0x200, End: 0x400})

	buf.Reset()

	if err := d.Dispatch("SHOW IMAGES"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if !strings.Contains(buf.String(), "TEST.EXE") {
		t.Errorf("output = %q, want the loaded image listed", buf.String())
	}
}

func TestShowSymbol(t *testing.T) {
	d, c, buf := newShowDispatcher(t)

	c.Symbols.Set("MYSYM", 0x1234, SymbolUser)

	if err := d.Dispatch("SHOW SYMBOL MYSYM"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if !strings.Contains(buf.String(), "00001234") {
		t.Errorf("output = %q, want the symbol's value", buf.String())
	}
}

func TestShowSymbol_permanentAndLabelAttributes(t *testing.T) {
	d, c, buf := newShowDispatcher(t)

	c.Symbols.SetQualified("MYSYM", 0x1234, true, false, true)

	if err := d.Dispatch("SHOW SYMBOL MYSYM"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "permanent") || !strings.Contains(out, "label") {
		t.Errorf("output = %q, want it to report the permanent and label attributes", out)
	}
}

func TestShowSymbol_undefined(t *testing.T) {
	d, _, _ := newShowDispatcher(t)

	if err := d.Dispatch("SHOW SYMBOL NOSUCH"); err == nil {
		t.Error("expected an error for an undefined symbol")
	}
}

func TestShowSymbolsSystem(t *testing.T) {
	d, c, buf := newShowDispatcher(t)

	c.Symbols.Set("USERSYM", 1, SymbolUser)
	c.Symbols.Set("SYS$SYM", 2, SymbolSystem)

	if err := d.Dispatch("SHOW SYMBOL/SYSTEM"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "SYS$SYM") {
		t.Errorf("output = %q, want the system symbol listed", out)
	}

	if strings.Contains(out, "USERSYM") {
		t.Errorf("output = %q, want the user symbol excluded", out)
	}
}

func TestShowQuantum(t *testing.T) {
	d, _, buf := newShowDispatcher(t)

	if err := d.Dispatch("SHOW QUANTUM"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if !strings.Contains(buf.String(), "Initial=20") {
		t.Errorf("output = %q, want the default quantum (20)", buf.String())
	}
}

func TestShowClock(t *testing.T) {
	d, c, buf := newShowDispatcher(t)

	if err := d.Dispatch("SHOW CLOCK"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if !strings.Contains(buf.String(), "not running") {
		t.Errorf("output = %q, want the clock reported as not running (ICCS Run bit clear)", buf.String())
	}

	c.CPU.SetPR(vax.ICCS, 1)
	buf.Reset()

	if err := d.Dispatch("SHOW CLOCK"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if strings.Contains(buf.String(), "not running") {
		t.Errorf("output = %q, want the clock reported as running once ICCS<0> is set", buf.String())
	}
}

func TestShowDebug(t *testing.T) {
	d, _, buf := newShowDispatcher(t)

	if err := d.Dispatch("SHOW DEBUG"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	// Default flags: REGISTERS/USERHALT/LIBINIT set, everything else clear.
	if !strings.Contains(out, "REGISTERS  ") || strings.Contains(out, "NOREGISTERS") {
		t.Errorf("output = %q, want REGISTERS reported set by default", out)
	}
	
	if !strings.Contains(out, "NOVM  ") {
		t.Errorf("output = %q, want VM reported clear by default", out)
	}
	// MEMORY/P1-P4 are settable but never displayed, matching the C source
	// (SERVICES' own description legitimately contains the substring "P1",
	// so check for the padded name column, not a bare substring).
	if strings.Contains(out, "MEMORY") || strings.Contains(out, "    P1  ") || strings.Contains(out, "NOP1") {
		t.Errorf("output = %q, want MEMORY/P1-P4 absent from SHOW DEBUG", out)
	}
}

func TestShowTrace(t *testing.T) {
	d, c, buf := newShowDispatcher(t)

	if err := d.Dispatch("SHOW TRACE"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if !strings.Contains(buf.String(), "disassembly is disabled") {
		t.Errorf("output = %q, want trace disassembly reported disabled", buf.String())
	}

	if strings.Contains(buf.String(), "Register tracking") {
		t.Errorf("output = %q, want no register-tracking line while trace is disabled", buf.String())
	}

	c.Trace = true
	
	buf.Reset()

	if err := d.Dispatch("SHOW TRACE"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if !strings.Contains(buf.String(), "disassembly is enabled") {
		t.Errorf("output = %q, want trace disassembly reported enabled", buf.String())
	}

	if !strings.Contains(buf.String(), "Register tracking is enabled") {
		t.Errorf("output = %q, want register tracking reported enabled (DebugRegisters is on by default)", buf.String())
	}
}

func TestShowFault(t *testing.T) {
	d, c, buf := newShowDispatcher(t)

	if err := d.Dispatch("SHOW FAULT"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if strings.Contains(buf.String(), "Pending interrupts:") {
		t.Errorf("output = %q, want no pending-interrupt section with nothing queued", buf.String())
	}

	psl := c.CPU.PSL()
	psl.SetIPL(20)
	c.CPU.SetPSL(psl)
	c.Engine.SetQuantum(4)
	c.Engine.Interrupt(cpu.ExcConWrite, 20, 0) // masked by current IPL: queued, not delivered

	buf.Reset()

	if err := d.Dispatch("SHOW FAULT"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Pending interrupts:") || !strings.Contains(out, "CONWRITE") {
		t.Errorf("output = %q, want the queued ExcConWrite interrupt reported", out)
	}
}

func TestShowInstructions_grid(t *testing.T) {
	d, _, buf := newShowDispatcher(t)

	if err := d.Dispatch("SHOW INSTRUCTIONS"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "HALT") {
		t.Errorf("output = %q, want HALT listed among implemented instructions", out)
	}

	if !strings.Contains(out, "Instructions.") {
		t.Errorf("output = %q, want a trailing instruction count", out)
	}
}

func TestShowInstructions_opcodeFilter(t *testing.T) {
	d, _, buf := newShowDispatcher(t)

	// Opcode 00 is HALT (internal/cpu/instructions_table.go), a
	// zero-operand instruction.
	if err := d.Dispatch("SHOW INSTRUCTIONS 00"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "HALT") {
		t.Errorf("output = %q, want HALT for opcode filter 00", out)
	}

	if strings.Contains(out, "No implemented instruction") {
		t.Errorf("output = %q, want opcode 00 to be found", out)
	}
}

func TestShowInstructions_unknownOpcode(t *testing.T) {
	d, _, buf := newShowDispatcher(t)

	// FE/FD are the (reserved) extended-opcode escape prefixes themselves,
	// not a real single-byte instruction opcode's own filter value here --
	// use a function-byte value no CPU instruction table entry claims.
	if err := d.Dispatch("SHOW INSTRUCTIONS FF"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if !strings.Contains(buf.String(), "No implemented instruction for opcode FF") {
		t.Errorf("output = %q, want a not-found message for opcode FF", buf.String())
	}
}

func TestShowMap(t *testing.T) {
	d, _, buf := newShowDispatcher(t)

	// "MAP" alone is ambiguous with the "MAPEN" register keyword; the
	// grammar's own show_types keyword for this command is plural.
	if err := d.Dispatch("SHOW MAPS"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if !strings.Contains(buf.String(), "Not applicable") {
		t.Errorf("output = %q, want a not-applicable stub", buf.String())
	}
}

func TestShowTB(t *testing.T) {
	d, _, buf := newShowDispatcher(t)

	if err := d.Dispatch("SHOW TB"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"Sequential Translation Cache",
		"Translation buffer caching is enabled",
		"Flushes=0   PFlushes=0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output = %q, want it to contain %q", out, want)
		}
	}
}

// TestShowTB_dumpsPopulatedEntry exercises a real, translated memory
// access first (MAPEN must be on), so SHOW TB has a real translation-
// buffer slot to dump — matching dump_tb()'s own per-slot output line.
func TestShowTB_dumpsPopulatedEntry(t *testing.T) {
	d, c, buf := newShowRunnableDispatcher(t)

	if _, err := c.Mem.LoadLongword(c.CPU, 0x200); err != nil {
		t.Fatalf("LoadLongword: %v", err)
	}

	buf.Reset()

	if err := d.Dispatch("SHOW TB"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "TB(") {
		t.Errorf("output = %q, want at least one populated TB(nn) slot", out)
	}
}

func TestShowBase_matchesDepositCursor(t *testing.T) {
	d, c, buf := newShowDispatcher(t)

	if err := c.Deposit("", 0x5678, SizeLongword, 0x11223344); err != nil {
		t.Fatalf("Deposit: %v", err)
	}

	buf.Reset()

	if err := d.Dispatch("SHOW BASE"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if !strings.Contains(buf.String(), "0000567C") {
		t.Errorf("output = %q, want the post-Deposit cursor (0x5678 + 4 bytes)", buf.String())
	}
}

func TestShowStack_bareDumpsLiveMemory(t *testing.T) {
	d, c, buf := newShowDispatcher(t)

	c.CPU.SetGPR(vax.SP, 0x8000)

	if err := c.Mem.StoreLongword(c.CPU, 0x8000, 0xCAFEBABE); err != nil {
		t.Fatalf("seed stack word: %v", err)
	}

	if err := d.Dispatch("SHOW STACK"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "KERNEL MODE SP = 00008000") {
		t.Errorf("output = %q, want the bare form to report the live (KERNEL) mode/SP", out)
	}

	if !strings.Contains(out, "CAFEBABE") {
		t.Errorf("output = %q, want the actual stack memory content, not just the register value", out)
	}
}

// TestShowP1LR regresses a copy-paste typo in evax.dcl's show_types
// keyword table ("p1l4" instead of "p1lr", fixed alongside this sub-phase
// per docs/PHASE-16.md's own cross-cutting open question about it) that
// made SHOW P1LR unparseable — the grammar rejected the keyword before
// parsing ever reached ShowRegisterOrPrivReg's fallback.
func TestShowP1LR(t *testing.T) {
	d, c, buf := newShowDispatcher(t)

	c.CPU.SetPR(vax.P1LR, 0x1234)

	if err := d.Dispatch("SHOW P1LR"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if !strings.Contains(buf.String(), "00001234") {
		t.Errorf("output = %q, want the P1LR value", buf.String())
	}
}

func TestShowStack_namedModeReadsSavedPointer(t *testing.T) {
	d, c, buf := newShowDispatcher(t)

	c.CPU.SetPR(vax.ESP, 0x9000)

	if err := c.Mem.StoreLongword(c.CPU, 0x9000, 0x99887766); err != nil {
		t.Fatalf("seed ESP stack word: %v", err)
	}

	if err := d.Dispatch("SHOW ESP"); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ESP MODE SP = 00009000") {
		t.Errorf("output = %q, want ESP's saved pointer (not the live SP, since cur_mod is KERNEL)", out)
	}

	if !strings.Contains(out, "99887766") {
		t.Errorf("output = %q, want the actual ESP-stack memory content", out)
	}
}
