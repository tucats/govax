package console

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// This file implements the subset of console_show.c's dozens of SHOW
// sub-displays this port covers — see docs/PHASE-16.md sub-phase 1 for the
// inventory this file works through and what's still deliberately left
// out (SHOW COMMAND_ARGS, SHOW EXPAND: no underlying data source yet; SHOW
// DEBUG/ASSEMBLER_FLAGS/STEP_MODE/TRACE, SHOW WATCHPOINTS, SHOW BREAK's
// /FAULT and /INSTRUCTION qualifiers: need new cross-cutting state
// sub-phases 2-4 add; SHOW INSTRUCTIONS' /MODES and /PROFILE qualifiers;
// SHOW ERROR: needs a design decision on whether this port adopts a
// VAX-style status-code space at all).

// ShowRegisters prints R0-R15 (with the AP/FP/SP/PC aliases), matching
// SHOW REGISTERS.
func (c *Console) ShowRegisters() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	for r := vax.R0; r <= vax.R11; r++ {
		c.Printf("R%-3d = %08X\n", int(r), c.CPU.GPR(r))
	}

	c.Printf("AP   = %08X\n", c.CPU.GPR(vax.AP))
	c.Printf("FP   = %08X\n", c.CPU.GPR(vax.FP))
	c.Printf("SP   = %08X\n", c.CPU.GPR(vax.SP))
	c.Printf("PC   = %08X\n", c.CPU.GPR(vax.PC))

	return nil
}

// ShowPSL prints the processor status longword and its named fields,
// matching SHOW PSL.
func (c *Console) ShowPSL() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	p := c.CPU.PSL()
	c.Printf("PSL = %08X\n", uint32(p))
	c.Printf("  CurMod=%d PrvMod=%d IPL=%d IS=%v FPD=%v TP=%v CM=%v\n",
		p.CurMod(), p.PrvMod(), p.IPL(), p.IS(), p.FPD(), p.TP(), p.CM())
	c.Printf("  DV=%v FU=%v IV=%v T=%v  N=%v Z=%v V=%v C=%v\n",
		p.DV(), p.FU(), p.IV(), p.T(), p.N(), p.Z(), p.V(), p.C())

	return nil
}

// ShowMemory prints the size of physical memory, matching SHOW MEMORY.
func (c *Console) ShowMemory() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	size := c.Mem.Size()
	c.Printf("Physical memory: %08X bytes (%d pages)\n", size, size/512)

	return nil
}

// ShowSymbols prints every defined symbol, matching SHOW SYMBOLS/ALL.
func (c *Console) ShowSymbols() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	for _, s := range c.Symbols.All() {
		kind := "user"
		if s.Kind == SymbolSystem {
			kind = "system"
		}

		c.Printf("%-31s = %08X  (%s)\n", s.Name, s.Value, kind)
	}

	return nil
}

// ShowBreakpoints prints every active breakpoint, matching SHOW
// BREAKPOINTS.
func (c *Console) ShowBreakpoints() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if len(c.Breakpoints) == 0 {
		c.Printf("No breakpoints set\n")
		return nil
	}

	for _, bp := range c.Breakpoints {
		c.Printf("Breakpoint at %08X\n", bp.Addr)
	}

	return nil
}

// ShowRadix prints the console's current default radix, matching SHOW
// RADIX.
func (c *Console) ShowRadix() error {
	c.Printf("Radix = %d\n", c.Radix)

	return nil
}

// ShowBase prints the console's current EXAMINE/DEPOSIT cursor, matching
// SHOW BASE (console_show.c's own body is exactly `printf("Next storage
// address is %08X\n", vax.console.deposit)` — nothing to do with region
// base registers, a prior mismatch documented in docs/PHASE-16.md
// sub-phase 1d and fixed here).
func (c *Console) ShowBase() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Printf("\n        Next storage address is %08X\n", c.DepositAddr)

	return nil
}

// StackKind selects which privileged-mode stack ShowStack dumps.
type StackKind int

const (
	StackKSP StackKind = iota
	StackESP
	StackSSP
	StackISP
	StackUSP
)

// modeNames matches console_show.c's own mode_names[] (SHOW MODE, SHOW
// STACK's reported mode name).
var modeNames = [5]string{"KERNEL", "EXEC", "SUPER", "USER", "INTERRUPT"}

func (k StackKind) name() string {
	switch k {
	case StackESP:
		return "ESP"
	case StackSSP:
		return "SSP"
	case StackISP:
		return "ISP"
	case StackUSP:
		return "USP"
	default:
		return "KSP"
	}
}

// stackPointerFor returns the live value of the stack pointer for a given
// privileged mode, without performing an actual mode switch: if that mode
// is the one currently active (PSL cur_mod, or the interrupt stack per
// PSL IS), its value is live in GPR(SP); otherwise it's whatever was last
// saved into the matching privileged register the last time that mode was
// left — the same value a real mode switch would load back into SP. See
// internal/cpu/handlefault.go/call.go's own mode-transition code (e.g.
// handlefault.go's `SetPR(vax.PrivReg(curMod), GPR(SP))` before a fault
// switches to a new mode) for where that save happens; this only reads
// the result, matching console_show.c's own set_mode_stack/restore pair's
// net (observable) effect without needing a real, reversible switch here.
func (c *Console) stackPointerFor(kind StackKind) uint32 {
	psl := c.CPU.PSL()

	if kind == StackISP {
		if psl.IS() {
			return c.CPU.GPR(vax.SP)
		}

		return c.CPU.PR(vax.ISP)
	}

	var (
		mode vax.AccessMode
		reg  vax.PrivReg
	)

	switch kind {
	case StackESP:
		mode, reg = 1, vax.ESP
	case StackSSP:
		mode, reg = 2, vax.SSP
	case StackUSP:
		mode, reg = 3, vax.USP
	default:
		mode, reg = 0, vax.KSP
	}

	if !psl.IS() && psl.CurMod() == mode {
		return c.CPU.GPR(vax.SP)
	}

	return c.CPU.PR(reg)
}

// ShowStack dumps live memory from a privileged mode's stack pointer
// upward (one longword per line, formatted in the console's current
// radix), matching console_show.c's shared SHOW STACK/KSP/ESP/SSP/ISP/USP
// case (id 139-144) — not just the selected stack pointer's register
// value, a prior mismatch documented in docs/PHASE-16.md sub-phase 1d and
// fixed here. current selects the bare SHOW STACK form (id 139: whichever
// mode is actually live right now, kind ignored); count, if nonzero,
// overrides the default of one longword; all dumps until end-of-stack/
// end-of-memory instead.
func (c *Console) ShowStack(kind StackKind, current bool, count uint32, all bool) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	psl := c.CPU.PSL()

	var (
		sp       uint32
		modeName string
	)

	if current {
		sp = c.CPU.GPR(vax.SP)

		modeIdx := int(psl.CurMod())
		if psl.IS() {
			modeIdx = 4
		}

		modeName = modeNames[modeIdx]
	} else {
		sp = c.stackPointerFor(kind)
		modeName = kind.name()
	}

	c.Printf("    %s MODE SP = %08X:\n", modeName, sp)

	if !all && (count < 1 || count > 255) {
		count = 1
	}

	mapen := c.CPU.PR(vax.MAPEN)

	for n := uint32(0); all || n < count*4; n += 4 {
		addr := sp + n

		if (psl.CurMod() == 3 && mapen != 0 && addr >= 0x7FFFFFFF) ||
			(mapen == 0 && addr >= c.Mem.Size()) {
			c.Printf("      <end of memory>\n")
			break
		}

		v, err := c.Mem.LoadLongword(c.CPU, addr)
		if err != nil {
			c.Printf("      <end of stack>\n")
			break
		}

		if c.Radix == 16 {
			c.Printf("      SP+%04X [%08X]:  %08X\n", n, addr, v)
		} else {
			c.Printf("      SP+%04d [%08X]:  %d\n", n, addr, int32(v))
		}
	}

	return nil
}

// ShowCPU prints a one-line machine status summary, matching SHOW
// CPU_STATUS.
func (c *Console) ShowCPU() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	state := "running"
	if c.Engine.Halted() {
		state = "halted"
	}

	c.Printf("CPU is %s, PC = %08X\n", state, c.CPU.GPR(vax.PC))

	return nil
}

// ShowVersion prints a version banner, matching SHOW VERSION (and ABOUT,
// which in the C source shares the same /entry= routine — see doc.go on
// why that indirection isn't ported).
func (c *Console) ShowVersion() error {
	c.Printf("govax — a Go port of eVAX (docs/PLAN.md)\n")

	return nil
}

// ShowRegisterOrPrivReg implements the plain register/privileged-register
// name shortcuts of SHOW (e.g. "SHOW R0", "SHOW PC", "SHOW P0BR") —
// testdata/dcl/evax.dcl's show_types keywords with no /syntax= redirect of
// their own, which stay on the bare SHOW verb (see dispatch.go's
// bindGrammar).
func (c *Console) ShowRegisterOrPrivReg(name string) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	name = strings.ToUpper(name)
	if r, ok := registerNames[name]; ok {
		c.Printf("%-4s = %08X\n", name, c.CPU.GPR(r))
		return nil
	}

	if pr, ok := privRegNames[name]; ok {
		c.Printf("%-6s = %08X\n", name, c.CPU.PR(pr))
		return nil
	}

	return fmt.Errorf("console: SHOW %s is not implemented", name)
}

// ShowMode prints the current privileged access mode, matching SHOW MODE.
// Like the C source (`mode_names[ vax.pslw.cur_mod ]`), this indexes
// purely by cur_mod and never reports "INTERRUPT" even if the interrupt
// stack is currently active — a quirk of the C command, not this port.
func (c *Console) ShowMode() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Printf("Current MODE is %s\n", modeNames[c.CPU.PSL().CurMod()])

	return nil
}

// ShowNVRAM prints the loaded NVRAM image's file name and extent, matching
// SHOW NVRAM.
func (c *Console) ShowNVRAM() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if len(c.NVRAM) == 0 {
		c.Printf("No NVRAM initialized\n")
		return nil
	}

	size := c.NVRAMEnd + 1 - c.NVRAMBase
	c.Printf("    NVRAM  FILE=%q\n", c.NVRAMFile)
	c.Printf("        CONSOLE$NVRAM_BASE = %08X\n", c.NVRAMBase)
	c.Printf("        CONSOLE$NVRAM_END  = %08X\n", c.NVRAMEnd)
	c.Printf("        CONSOLE$NVRAM_SIZE = %08X (%dK)\n", size, size/1024)

	return nil
}

// ShowROM prints the loaded ROM image's file name and extent, matching
// SHOW ROM.
func (c *Console) ShowROM() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if len(c.ROM) == 0 {
		c.Printf("No ROM loaded\n")
		return nil
	}

	size := c.ROMEnd + 1 - c.ROMBase
	c.Printf("    ROM FILE=%q\n", c.ROMFile)
	c.Printf("        CONSOLE$ROM_BASE  = %08X\n", c.ROMBase)
	c.Printf("        CONSOLE$ROM_END   = %08X\n", c.ROMEnd)
	c.Printf("        CONSOLE$ROM_SIZE  = %08X (%dK)\n", size, size/1024)

	return nil
}

// ShowShim prints every synthesized RTL shim stub (kernel.asm's `.shim`
// table, see shim.go's ensureShims) and whether internal/rtl.Environment
// has a live numeric-dispatch handler for it, matching shim.c's own
// shim_dump — with one deliberate reporting difference: C's shim_dump
// resolves each SHIM$<library>_<offset> symbol to a *second*,
// separately-defined kernel.asm label at the same address (the real
// routine it's equated to); this port's ensureShims instead synthesizes a
// fresh stub directly (see that function's own doc comment), so there is
// no second label to resolve — reporting the numeric dispatch code and
// its live/dead status serves the same "does this shim actually do
// anything" purpose.
func (c *Console) ShowShim() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if !c.shimsReady {
		c.Printf("No RTL shims defined.\n")
		return nil
	}

	c.Printf("RTL SHIMS:\n")

	addr := c.shimBase
	for _, e := range shimTable {
		name := fmt.Sprintf("SHIM$%s_%08X", e.library, e.offset)

		status := "dead (no numeric dispatch)"
		if e.code != 0 {
			status = "unimplemented"
			if c.RTL.HasShim(e.code) {
				status = "live"
			}
		}

		c.Printf("    %-32s = %08X  code=%-3d %s\n", name, addr, e.code, status)
		addr += shimStubSize
	}

	return nil
}

// ShowString walks kernel.asm's CONSOLE$STRINGPOOL linked list of VAX
// string descriptors, matching SHOW STRING (console_show.c's own case
// 121). Requires a booted microkernel (the pool's own symbols): reports an
// error if they're undefined rather than the C source's !MKVALID gate,
// since this port has no separate MKVALID-equivalent flag (see
// docs/PHASE-16.md sub-phase 3's SET MKVALID entry).
func (c *Console) ShowString() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	base, ok := c.Symbols.Get("CONSOLE$STRINGPOOL_BASE")
	if !ok {
		return fmt.Errorf("console: CONSOLE$STRINGPOOL_BASE is undefined (boot the microkernel first)")
	}

	size, ok := c.Symbols.Get("CONSOLE$STRINGPOOL_SIZE")
	if !ok {
		return fmt.Errorf("console: CONSOLE$STRINGPOOL_SIZE is undefined")
	}

	current, ok := c.Symbols.Get("CONSOLE$STRINGPOOL")
	if !ok {
		return fmt.Errorf("console: CONSOLE$STRINGPOOL is undefined")
	}

	c.Printf("STRING POOL\n")
	c.Printf("    CONSOLE$STRINGPOOL_BASE       %08X\n", base)
	c.Printf("    CONSOLE$STRINGPOOL_SIZE       %08X\n", size)
	c.Printf("    CONSOLE$STRINGPOOL (Current)  %08X\n", current)

	addr, err := c.Mem.LoadLongword(c.CPU, base)
	if err != nil {
		return err
	}

	if addr != 0 {
		c.Printf("\n")
	}

	for addr != 0 {
		lenRaw, err := c.Mem.LoadLongword(c.CPU, addr)
		if err != nil {
			return err
		}

		descLen := lenRaw & 0xFFFF

		descAddr, err := c.Mem.LoadLongword(c.CPU, addr+4)
		if err != nil {
			return err
		}

		desc := addr

		next, err := c.Mem.LoadLongword(c.CPU, addr+8)
		if err != nil {
			return err
		}

		n := descLen
		truncated := n > 250
		if truncated {
			n = 250
		}

		buf := make([]byte, n)
		if err := c.Mem.Load(c.CPU, descAddr, buf); err != nil {
			return err
		}

		text := string(buf)
		if truncated {
			text += "..."
		}

		c.Printf("    Descriptor %08X [%3d bytes]  \"%s\"\n", desc, descLen, text)

		addr = next
	}

	c.Printf("\n")

	return nil
}

// ShowPage reports one virtual address's page table entry (VALID/PROT/M/
// OWNER/S/PFN, plus the physical address it resolves to and whether the
// given access would be permitted), matching console_show.c's SHOW PAGE
// (body is tracevm()) — a read-only diagnostic that reports a PTE's raw
// contents even for a page a real access would refuse, unlike EXAMINE/
// DEPOSIT which go through the enforcing internal/vm.Memory.Translate.
// write selects /WRITE (the default is /READ, matching tracevm's own
// mode==0 default).
func (c *Console) ShowPage(addrExpr string, write bool) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if c.CPU.PR(vax.MAPEN) == 0 {
		c.Printf("Virtual memory is disabled.\n")
		return nil
	}

	v, _, err := c.Evaluator().Eval(strings.TrimSpace(addrExpr))
	if err != nil {
		return err
	}

	region, pteAddr, pte, err := c.Mem.LookupPTE(c.CPU, v)
	if err != nil {
		c.Printf("ACCVIO, page table region %d length violation\n", region)
		return nil
	}

	byteOffset := v & 0x1FF

	c.Printf("    Virtual address:   %08X\n", v)
	c.Printf("    Region:            %02d\n", region)
	c.Printf("    PTE Address:       %08X\n", pteAddr)
	c.Printf("    PTE Longword:      %08X\n", uint32(pte))
	c.Printf("        VALID: %s\n", boolToDigit(pte.Valid()))
	c.Printf("        PROT:  %02X (PTE$K_%s)\n", pte.Protection(), pte.Protection())
	c.Printf("        M:     %s\n", boolToDigit(pte.Modified()))
	c.Printf("        OWNER: %X\n", pte.Owner())
	c.Printf("        S:     %X\n", pte.Software())
	c.Printf("        PFN:   %08X\n", pte.PFN())
	c.Printf("    Physical address:  %08X\n", pte.PFN()<<9+byteOffset)

	access, label := vm.AccessRead, "VM_READ"
	if write {
		access, label = vm.AccessWrite, "VM_WRITE"
	}

	c.Printf("    Access:      %s\n", label)

	if !pte.Protection().Allows(c.CPU.PSL().CurMod(), access) {
		c.Printf("ACCVIO, protection violation\n")
		return nil
	}

	if !pte.Valid() {
		c.Printf("TNV, valid bit not set\n")
	}

	return nil
}

func boolToDigit(b bool) string {
	if b {
		return "1"
	}

	return "0"
}

// scbVectorNames matches showscb's own vector_name[] table (console_show.c)
// — the EXC$<name> mnemonic for each of the SCB's 64 four-byte slots.
var scbVectorNames = [64]string{
	"UNUSED", "CHECK", "KSNV", "POWER",
	"PRIV", "CUSTOMER", "RESOP", "RESADDR",
	"ACCVIO", "TNV", "TP", "BPT",
	"COMPAT", "ARITH", "RESERVED38", "RESERVED3C",
	"CHMK", "CHME", "CHMS", "CHMU",
	"SBI", "CMRD", "SBIALERT", "SBIFAULT",
	"MWT", "RESERVED64", "RESERVED68", "RESERVED6C",
	"RESERVED70", "RESERVED74", "RESERVED78", "RESERVED7C",
	"RESERVED80", "SOFTWARE1", "SOFTWARE2", "SOFTWARE3",
	"SOFTWARE4", "SOFTWARE5", "SOFTWARE6", "SOFTWARE7",
	"SOFTWARE8", "SOFTWARE9", "SOFTWARE10", "SOFTWARE11",
	"SOFTWARE12", "SOFTWARE13", "SOFTWARE14", "SOFTWARE15",
	"RESERVEDC0", "RESERVEDC4", "RESERVEDC8", "RESERVEDCC",
	"RESERVEDD0", "RESERVEDD4", "RESERVEDD8", "RESERVEDDC",
	"RESERVEDE0", "RESERVEDE4", "RESERVEDE8", "RESERVEDEC",
	"RESERVEDF0", "RESERVEDF4", "CONREAD", "CONWRITE",
}

// exceptionName returns code's EXC$<name> mnemonic from scbVectorNames
// (code doubles as the SCB byte offset, so code/4 is the slot index), or
// "?" if out of range.
func exceptionName(code cpu.Exception) string {
	i := int(code) / 4
	if i < 0 || i >= len(scbVectorNames) {
		return "?"
	}

	return scbVectorNames[i]
}

// ShowSCB dumps the 64 System Control Block exception-vector slots from
// SCBB, matching showscb() (console_show.c). all requests every slot,
// including empty (zero) ones; the default only prints populated slots.
// SCBB is a physical address (matching the C source's own MAPEN==0
// override around this same read loop), read here the same way SHOW SCB's
// C ancestor does.
func (c *Console) ShowSCB() error { return c.showSCB(false) }

// ShowSCBAll implements SHOW SCB/ALL — see ShowSCB.
func (c *Console) ShowSCBAll() error { return c.showSCB(true) }

func (c *Console) showSCB(all bool) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	scbb := c.CPU.PR(vax.SCBB)
	if scbb == 0 {
		c.Printf("No SCB established (SCBB=0)\n")
		return nil
	}

	c.Printf("SYSTEM CONTROL BLOCK\n    SCBB register = %08X\n", scbb)

	savedMapen := c.CPU.PR(vax.MAPEN)
	c.CPU.SetPR(vax.MAPEN, 0)

	defer c.CPU.SetPR(vax.MAPEN, savedMapen)

	count := 0

	for n := uint32(0); n < 64; n++ {
		vector, err := c.Mem.LoadLongword(c.CPU, scbb+n*4)
		if err != nil {
			return err
		}

		if vector == 0 && !all {
			continue
		}

		if count == 0 {
			c.Printf("    Slot  Name            Address  ISP?  Label\n")
		}

		count++

		if vector == 0xFFFFFFFF {
			c.Printf("     %02X   EXC$%-10s  <console handler>\n", n, scbVectorNames[n])
			continue
		}

		stack := vector & 0x3
		addr := vector &^ 0x3

		label := ""
		if name, ok := c.Symbols.FindByValue(addr); ok {
			label = name
		}

		ispFlag := "     "
		if stack != 0 {
			ispFlag = "(ISP)"
		}

		c.Printf("     %02X   EXC$%-10s  %08X %s %s\n", n, scbVectorNames[n], addr, ispFlag, label)
	}

	if count == 0 {
		c.Printf("No exception vector entries defined.\n")
	}

	return nil
}

// ShowCallFrames walks the CALLS/CALLG frame chain from FP, printing
// handler/mask/SPA/calltype/saved-AP/saved-FP/saved-PC/argument list per
// frame, matching show_calls() (console_show.c). Decodes the mask
// longword using this port's own real-VAX-architecture bit layout (spa
// bits 30-31, calltype bit 29, mask bits 16-27, psw bits 0-15 — see
// internal/cpu/call.go's emulRet, the authoritative decode this mirrors),
// not console_show.c's own union MASKREG bit-field declaration (a
// different, C-compiler-bit-field-allocation-order-dependent layout that
// has no bearing on what this port's own frames actually contain).
func (c *Console) ShowCallFrames(countExpr string) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	count := uint32(1)
	if s := strings.TrimSpace(countExpr); s != "" {
		v, err := strconv.ParseUint(s, 16, 32)
		if err != nil {
			return fmt.Errorf("console: invalid count %q: %w", s, err)
		}

		count = uint32(v)
	}

	fp := c.CPU.GPR(vax.FP)
	ap := c.CPU.GPR(vax.AP)

	if fp == 0 || ap == 0 {
		return fmt.Errorf("console: no call frames (FP/AP not established)")
	}

	for ; count > 0; count-- {
		c.Printf("    FRAME: %08X", fp)

		addr := fp

		handler, err := c.Mem.LoadLongword(c.CPU, addr)
		if err != nil {
			return err
		}

		if handler != 0 {
			c.Printf(", Handler: %08X", handler)
		}

		addr += 4

		maskWord, err := c.Mem.LoadLongword(c.CPU, addr)
		if err != nil {
			return err
		}

		spa := maskWord >> 30
		calls := maskWord&(1<<29) != 0
		mask := (maskWord >> 16) & 0x0FFF
		psw := maskWord & 0xFFFF

		callType := "CALLG"
		if calls {
			callType = "CALLS"
		}

		c.Printf(", SPA: %1X, %s, MASK: %04X, PSW: %04X\n", spa, callType, mask, psw)

		addr += 4

		savedAP, err := c.Mem.LoadLongword(c.CPU, addr)
		if err != nil {
			return err
		}

		c.Printf("        Saved AP : %08X\n", savedAP)

		addr += 4

		savedFP, err := c.Mem.LoadLongword(c.CPU, addr)
		if err != nil {
			return err
		}

		if savedFP == cpu.SentinelReturn {
			c.Printf("        Saved FP : <console>\n")
		} else {
			c.Printf("        Saved FP : %08X\n", savedFP)
		}

		addr += 4

		savedPC, err := c.Mem.LoadLongword(c.CPU, addr)
		if err != nil {
			return err
		}

		if savedPC == cpu.SentinelReturn {
			c.Printf("        Saved PC : <console>\n")
		} else {
			c.Printf("        Saved PC : %08X\n", savedPC)
		}

		addr += 4

		for n := uint32(0); n <= 11; n++ {
			if mask&(1<<n) == 0 {
				continue
			}

			v, err := c.Mem.LoadLongword(c.CPU, addr)
			if err != nil {
				return err
			}

			addr += 4

			c.Printf("        Saved R%-2d: %08X\n", n, v)
		}

		var argc uint32

		if ap >= 0x200 && ap < 0x7FFFFFFC {
			argc, err = c.Mem.LoadLongword(c.CPU, ap)
			if err != nil {
				return err
			}
		} else {
			c.Printf("        (AP) is not an argument list\n")
		}

		switch {
		case argc > 255:
			c.Printf("        (AP) is not an argument list\n")

		case argc > 0:
			word, plural := "is", ""
			if argc != 1 {
				word, plural = "are", "s"
			}

			c.Printf("        There %s %d argument%s:\n", word, argc, plural)

			a := ap
			for n := uint32(1); n <= argc; n++ {
				if n > 15 {
					c.Printf("            ...and %d more...\n", argc-n)
					break
				}

				a += 4

				v, err := c.Mem.LoadLongword(c.CPU, a)
				if err != nil {
					return err
				}

				c.Printf("            [%2d]: %08X\n", n, v)
			}

			if argc > 64 {
				c.Printf("            Note: AP is probably bogus for this call frame\n")
			}
		}

		ap = savedAP
		fp = savedFP

		if fp == 0 || fp == cpu.SentinelReturn {
			break
		}

		c.Printf("\n")
	}

	return nil
}

// ShowRegions prints the P0/P1/S0 image-activation region limits, matching
// SHOW REGIONS (console_show.c's own case 162, get_region_size loop) —
// not to be confused with SHOW MEMORY's own internal show_regions() helper
// (a different, unrelated dump; see docs/PHASE-16.md's own note on this
// naming collision in the C source).
func (c *Console) ShowRegions() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	names := [3]string{"P0", "P1", "S0"}

	c.Printf("REGION LIMITS:\n")

	for i, name := range names {
		c.Printf("\t%s   %08X\n", name, c.RTL.RegionSize[i])
	}

	c.Printf("\n")

	return nil
}

// ShowSharePrefix reports the sharable-image search prefix, matching SHOW
// SHARE_PREFIX.
func (c *Console) ShowSharePrefix() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	switch {
	case c.SharePrefix != "" && c.SharePrefix[0] == 0x1B:
		c.Printf("Recursive sharable image loading disabled.\n")

	case c.SharePrefix == "":
		c.Printf("No sharable image prefix defined\n")

	default:
		c.Printf("Sharable images loaded from \"%s\"\n", c.SharePrefix)
	}

	return nil
}

// ShowImages lists every loaded image (main and sharable dependencies),
// matching dump_icb_list (console_run.c). full adds each image's transfer
// address(es) and dependency list.
func (c *Console) ShowImages(full bool) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if len(c.ICBList) == 0 {
		c.Printf("No VMS images loaded in memory.\n")
		return nil
	}

	c.Printf("ACTIVE IMAGES IN MEMORY:\n")

	for _, icb := range c.ICBList {
		c.Printf("    %-39s  %08X  %08X\n", icb.Name, icb.Base, icb.End)

		if !full {
			continue
		}

		if icb.Transfer[0] == 0 {
			c.Printf("        no transfer addresses\n")
		} else {
			suffix := ""
			if icb.Transfer[1] != 0 {
				suffix = "es"
			}

			c.Printf("        transfer address%s ", suffix)

			for _, t := range icb.Transfer {
				if t != 0 {
					c.Printf("%08X  ", t)
				}
			}

			c.Printf("\n")
		}

		if len(icb.SHRList) > 1 {
			c.Printf("        depends on ")

			first := true

			for _, shr := range icb.SHRList {
				if shr.ID == 0 {
					continue // the self-reference SHR entry
				}

				if !first {
					c.Printf(", ")
				}

				first = false

				c.Printf("%s", shr.Name)
			}

			c.Printf("\n")
		}
	}

	return nil
}

// ShowSymbol prints one symbol's value, matching the single-name form of
// SHOW SYMBOL (console_show.c's case 149). Unlike the C source, this
// reports only the value and user/system kind (matching ShowSymbols' own
// existing kind label), not the fuller perm/entry/label/local/string
// attribute set — this port's SymbolKind doesn't track those distinctions
// (see docs/PHASE-16.md sub-phase 1c).
func (c *Console) ShowSymbol(name string) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	name = strings.TrimSpace(name)

	sym, ok := c.Symbols.Find(name)
	if !ok {
		return fmt.Errorf("console: undefined symbol %q", name)
	}

	kind := "user"
	if sym.Kind == SymbolSystem {
		kind = "system"
	}

	c.Printf("    %s = %08X (hex)   %12d (dec)  (%s)\n", sym.Name, sym.Value, int32(sym.Value), kind)

	return nil
}

// ShowSymbolsSystem prints every system (as opposed to user-defined)
// symbol, matching SHOW SYMBOL/SYSTEM (dump_system_symbols).
func (c *Console) ShowSymbolsSystem() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	for _, s := range c.Symbols.All() {
		if s.Kind != SymbolSystem {
			continue
		}

		c.Printf("%-31s = %08X  (system)\n", s.Name, s.Value)
	}

	return nil
}

// ShowQuantum reports the interrupt-admission quantum countdown, matching
// SHOW QUANTUM (console_show.c's case 128) — now portable per Phase 14's
// own Engine.Quantum, see docs/PHASE-16.md sub-phase 1b. The C source's
// second block (vax.uiquantum: a cooperative host-UI-event-polling
// time-slice counter, relevant only to the C source's own Mac/Windows GUI
// event pump) has no equivalent in this port — there is no such polling
// loop to report on — so it's reported as not modeled rather than
// replicated as a dead counter.
func (c *Console) ShowQuantum() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	current, initial := c.Engine.Quantum()

	c.Printf("QUANTUM\n    INTERRUPTS  Initial=%d  Current=%d\n", initial, current)
	c.Printf("    USER INTF   Not modeled by this port (no cooperative host-UI polling loop)\n")

	return nil
}

// ShowClock reports the interval-clock/quantum state, matching SHOW CLOCK
// (console_show.c's case 500) — now portable per Phase 14's own
// ICR/NICR/ICCS state and Engine.Quantum, see docs/PHASE-16.md sub-phase
// 1b. "clock_running" is exactly ICCS bit 0 (vax.h: "Copy of ICCS<0>"), so
// this reads ICCS directly rather than needing a separate mirrored field.
func (c *Console) ShowClock() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	running := c.CPU.PR(vax.ICCS)&1 != 0
	runWord := "not "

	if running {
		runWord = ""
	}

	c.Printf("   CPU CLOCK STATES:\n")
	c.Printf("      HARDWARE CLOCK=implemented, clock is currently %srunning\n", runWord)
	c.Printf("      ICR  = %08X (%4d)  [Current clock value]\n", c.CPU.PR(vax.ICR), c.CPU.PR(vax.ICR))
	c.Printf("      NICR = %08X (%4d)  [Reload  clock value]\n", c.CPU.PR(vax.NICR), c.CPU.PR(vax.NICR))

	current, initial := c.Engine.Quantum()
	c.Printf("  QUANTUM\n    INITIAL=%d\n    CURRENT=%d\n", initial, current)

	return nil
}

// ShowFault reports pending device/software interrupts, matching
// console_show.c's own SHOW FAULT case (id 145) — now partially portable
// per Phase 14's own Engine.PendingInterrupts (vax.interrupt_pending/
// vax.iqueue), see docs/PHASE-16.md sub-phase 1b. The C source's other
// half, show_faults() (a fault/exception *event history* ring buffer, a
// separate mechanism from the live pending-interrupt queue this reports),
// is not ported: it needs new recorder instrumentation hooked into
// internal/cpu/handlefault.go with no existing state to build on, unlike
// the pending-interrupt half — left for a follow-up (see docs/PHASE-16.md
// sub-phase 1b's own recommendation to split these).
func (c *Console) ShowFault() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Printf("    Fault/exception event history is not recorded by this port (see docs/PHASE-16.md sub-phase 1b).\n")

	pending, queued := c.Engine.PendingInterrupts()

	if pending != nil {
		c.Printf("    There is a pending interrupt code %s at ipl %02X\n", exceptionName(pending.Code), pending.IPL)
	}

	if len(queued) > 0 {
		c.Printf("Pending interrupts:\n")
		c.Printf("    IPL  Age Code\n")

		for _, q := range queued {
			c.Printf("    %02X   %4d  %02X %s\n", q.IPL, q.Age, uint8(q.Code), exceptionName(q.Code))
		}
	}

	return nil
}

// ShowMap reports that this port has no RMS field-offset registry to
// dump, matching SHOW MAP's intent (console_show.c's map_dump, over
// structure_mapping.c's declarative FAB/RAB offset table) against a
// deliberately different design: internal/rtl/rms.go reads/writes RMS
// struct fields directly at hardcoded Go offsets rather than through a
// runtime registry (see that file's own doc comment) — there is nothing
// for this command to dump, so it reports that rather than an empty table.
func (c *Console) ShowMap() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Printf("Not applicable to this port: RMS struct fields are read/written directly at\n")
	c.Printf("hardcoded Go offsets (internal/rtl/rms.go), not through a runtime field-offset\n")
	c.Printf("registry — see docs/PHASE-16.md sub-phase 1f.\n")

	return nil
}

// ShowTB reports that this port's virtual memory has no translation cache
// to report statistics on, matching SHOW TB's intent (console_show.c's own
// cached_page_*/tb_*/dump_tb counters) against a deliberately different
// design: internal/vm.Memory.Translate does a direct page-table walk on
// every access (see that file's own doc comment on why a translation
// buffer/"sequential translation cache" wasn't ported) — there are no
// hit/miss counters to report, so this says so rather than fabricating
// zeroes.
func (c *Console) ShowTB() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Printf("Not applicable to this port: internal/vm.Memory.Translate does a direct\n")
	c.Printf("page-table walk on every access, with no translation cache to report\n")
	c.Printf("hit/miss statistics on — see docs/PHASE-16.md sub-phase 1f.\n")

	return nil
}

// accessAbbrev matches show_instructions()'s own operand-kind abbreviation
// (console_show.c): "src"/"dst"/"mod"/"addr"/"br", "x" for anything else
// (AccessNone, or OP_VA/"var" — this port's AccessKind has no variable-
// operand-count kind since no ported instruction uses one).
func accessAbbrev(a cpu.AccessKind) string {
	switch a {
	case cpu.AccessRead:
		return "src"
	case cpu.AccessWrite:
		return "dst"
	case cpu.AccessModify:
		return "mod"
	case cpu.AccessAddress:
		return "addr"
	case cpu.AccessBranch:
		return "br"
	default:
		return "x"
	}
}

// sizeAbbrev matches show_instructions()'s own operand-size suffix.
func sizeAbbrev(n int) string {
	switch n {
	case 1:
		return ".b"
	case 2:
		return ".w"
	case 4:
		return ".l"
	case 8:
		return ".q"
	default:
		return ".x"
	}
}

// ShowInstructions implements SHOW INSTRUCTIONS (console_show.c's case
// 131 plus its own show_instructions() helper): with no qualifier, a
// four-per-line opcode/name grid (implemented instructions by default,
// unimplemented with /UNIMPLEMENTED); with /ALL or an opcode filter, one
// line per instruction with full operand access/size detail. /MODES and
// /PROFILE are not implemented — see docs/PHASE-16.md sub-phase 1e (the
// former needs an addressing-mode legality table this port doesn't
// expose, the latter needs per-opcode execution counters that don't
// exist, arguably a developer/instrumentation feature rather than
// emulated VAX behavior worth adding here).
func (c *Console) ShowInstructions(modes, profile, unimplemented, all bool, opcodeExpr string) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if profile {
		return fmt.Errorf("console: SHOW INSTRUCTIONS/PROFILE is not implemented (no per-opcode execution counters in this port)")
	}

	if modes {
		return fmt.Errorf("console: SHOW INSTRUCTIONS/MODES is not implemented (no addressing-mode legality table exposed by this port)")
	}

	table := cpu.Instructions()

	opmatch := int32(-1)
	if s := strings.TrimSpace(opcodeExpr); s != "" {
		v, err := strconv.ParseUint(s, 16, 8)
		if err != nil {
			return fmt.Errorf("console: invalid opcode %q: %w", s, err)
		}

		opmatch = int32(v)
	}

	if !all && opmatch < 0 {
		return c.showInstructionsGrid(table, unimplemented)
	}

	return c.showInstructionsDetail(table, opmatch)
}

func (c *Console) showInstructionsGrid(table *cpu.Table, unimplemented bool) error {
	c.Printf("INSTRUCTION OPCODES:\n\n")

	count, n := 0, 0

	var line strings.Builder

	for _, inst := range table.All() {
		if strings.HasPrefix(inst.Name, "RSVD_") || strings.HasPrefix(inst.Name, "EXT_") {
			continue
		}

		impl := table.Implemented(inst)
		if unimplemented == impl {
			continue
		}

		count++

		if inst.Opcode.Extended != 0 {
			fmt.Fprintf(&line, "   %02X %02X %-8s  ", inst.Opcode.Extended, inst.Opcode.Function, inst.Name)
		} else {
			fmt.Fprintf(&line, "      %02X %-8s  ", inst.Opcode.Function, inst.Name)
		}

		n++

		if n == 4 {
			c.Printf("%s\n", line.String())
			line.Reset()

			n = 0
		}
	}

	if n > 0 {
		c.Printf("%s\n", line.String())
	}

	c.Printf("\n%d Instructions.\n", count)

	return nil
}

func (c *Console) showInstructionsDetail(table *cpu.Table, opmatch int32) error {
	count, found := 0, false

	for _, inst := range table.All() {
		if !table.Implemented(inst) {
			continue
		}

		if opmatch >= 0 {
			if uint32(inst.Opcode.Function) != uint32(opmatch) {
				continue
			}

			found = true
		}

		var line strings.Builder

		if inst.Opcode.Extended != 0 {
			fmt.Fprintf(&line, "%02X %02X %-8s", inst.Opcode.Extended, inst.Opcode.Function, inst.Name)
		} else {
			fmt.Fprintf(&line, "   %02X %-8s", inst.Opcode.Function, inst.Name)
		}

		for j := 0; j < inst.OperandCount; j++ {
			if j > 0 {
				line.WriteString(", ")
			} else {
				line.WriteString(" ")
			}

			line.WriteString(accessAbbrev(inst.Access[j]))
			line.WriteString(sizeAbbrev(inst.Scale[j]))
		}

		c.Printf("    %s\n", line.String())

		count++
	}

	if opmatch < 0 && count > 1 {
		c.Printf("%d instructions\n", count)
	}

	if opmatch >= 0 && !found {
		c.Printf("No implemented instruction for opcode %02X\n", opmatch)
	}

	return nil
}
