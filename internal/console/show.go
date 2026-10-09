package console

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/symtab"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmserrors"
)

const (
	enabledState  = "enabled"
	disabledState = "disabled"
)

// ShowRegisters prints R0-R11 plus the AP/FP/SP/PC aliases in the same
// four-column grid as the PSL beneath them, matching SHOW REGISTERS.
func (c *Console) ShowRegisters() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.dumpRegisters()

	return nil
}

// dumpRegisters prints the R0-R11/AP/FP/SP/PC grid followed by the PSL
// block, shared by SHOW REGISTERS (ShowRegisters) and SHOW CPU (ShowCPU).
func (c *Console) dumpRegisters() {
	c.Printf("\n\n    Registers:\n\n")
	c.Printf("    R0:  %08X      R4:  %08X     R8:  %08X     AP:  %08X\n",
		c.CPU.GPR(vax.R0), c.CPU.GPR(vax.R4), c.CPU.GPR(vax.R8), c.CPU.GPR(vax.AP))
	c.Printf("    R1:  %08X      R5:  %08X     R9:  %08X     FP:  %08X\n",
		c.CPU.GPR(vax.R1), c.CPU.GPR(vax.R5), c.CPU.GPR(vax.R9), c.CPU.GPR(vax.FP))
	c.Printf("    R2:  %08X      R6:  %08X     R10: %08X     SP:  %08X\n",
		c.CPU.GPR(vax.R2), c.CPU.GPR(vax.R6), c.CPU.GPR(vax.R10), c.CPU.GPR(vax.SP))
	c.Printf("    R3:  %08X      R7:  %08X     R11: %08X     PC:  %08X\n\n",
		c.CPU.GPR(vax.R3), c.CPU.GPR(vax.R7), c.CPU.GPR(vax.R11), c.CPU.GPR(vax.PC))

	c.dumpPSL()
}

// ShowPSL prints the processor status longword and its named fields.
func (c *Console) ShowPSL() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.dumpPSL()

	return nil
}

// dumpPSL prints the PSL longword plus its PSW and PRIV field breakdowns.
func (c *Console) dumpPSL() {
	p := c.CPU.PSL()

	c.Printf("    PSL: %08X\n", uint32(p))
	c.Printf("         PSW:  C=%s, V=%s, Z=%s, N=%s, T=%s, IV=%s, FU=%s, DV=%s\n",
		boolToDigit(p.C()), boolToDigit(p.V()), boolToDigit(p.Z()), boolToDigit(p.N()),
		boolToDigit(p.T()), boolToDigit(p.IV()), boolToDigit(p.FU()), boolToDigit(p.DV()))
	c.Printf("         PRIV: IPL=%d, CUR_MOD=%s, PRV_MOD=%s, FPD=%s, TP=%s, CM=%s\n",
		p.IPL(), modeNames[p.CurMod()], modeNames[p.PrvMod()],
		boolToDigit(p.FPD()), boolToDigit(p.TP()), boolToDigit(p.CM()))
}

// ShowMemory prints physical memory size and, once VMINIT has established
// page tables, per-region P0/P1/S0 accounting.
func (c *Console) ShowMemory() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	size := c.Mem.Size()
	pages := size / 512

	c.Printf("\n    Physical Memory\n        %08X (%d decimal) pages\n        Addresses  00000000 - %08X\n\n",
		pages, pages, size-1)

	state := disabledState
	if c.CPU.PR(vax.MAPEN) != 0 {
		state = enabledState
	}

	c.Printf("    Virtual Memory (currently %s)\n", state)

	// If VMINIT has never been issued, this port has no console-specific
	// knowledge of the memory layout, so it can't report on it.
	if !c.VMInitValid {
		c.Printf("        Virtual memory configuration is unknown.\n")

		return nil
	}

	mapped := c.Mem.MappedPages()
	c.Printf("    There are %d physical pages mapped, %d free.\n\n", mapped, int(pages)-mapped)

	bases := [3]uint32{c.CPU.PR(vax.P0BR), c.CPU.PR(vax.P1BR), c.CPU.PR(vax.SBR)}
	lens := [3]uint32{c.CPU.PR(vax.P0LR), c.CPU.PR(vax.P1LR), c.CPU.PR(vax.SLR)}

	for n, r := range c.Regions {
		plural := "s"
		if r.pteCount == 1 {
			plural = " "
		}

		c.Printf("        %s Region\n            Size = %08X (%5d decimal) pages\n            PFN database = %d page%s",
			r.name, r.size, r.size, r.pteCount, plural)

		c.Printf("  %sBR = %08X    %sLR = %08X\n", r.name, bases[n], r.name, lens[n])

		c.Printf("            Physical addresses = %08X-%08X\n            Virtual addresses  = %08X-%08X\n\n",
			r.pStart, r.pEnd-1, r.vStart, r.vEnd-1)
	}

	return nil
}

// ShowSymbols prints every defined symbol whose name matches pattern,
// for the debugger's SHOW SYMBOL/ALL [pattern]: VMS wildcards, "*" for any run of
// characters and "%" for exactly one; an empty pattern matches every
// name. The assembler's predefined system symbols are listed too, as the
// C source's one shared symbol table listed them.
func (c *Console) ShowSymbols(pattern string) error {
	return c.showSymbolList(pattern, false)
}

// showSymbolList prints the symbols listSymbols selects, one per line, or
// reports CLI_UNDEFSYM when a pattern matched nothing.
func (c *Console) showSymbolList(pattern string, systemOnly bool) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	syms := c.listSymbols(pattern, systemOnly)
	if len(syms) == 0 && strings.TrimSpace(pattern) != "" {
		return vmserrors.New(vmserrors.CLI_UNDEFSYM, strings.TrimSpace(pattern))
	}

	for _, s := range syms {
		c.Printf("%-31s = %08X  (%s)\n", s.Name, s.Value, symbolKindLabel(s))
	}

	return nil
}

// listSymbols returns, sorted by name, every console symbol and every
// predefined symbol (asm.BuiltinSymbols) not shadowed by a console symbol
// of the same name, whose name matches pattern (VMS wildcards; empty
// matches all). systemOnly leaves out user symbols.
func (c *Console) listSymbols(pattern string, systemOnly bool) []*Symbol {
	pattern = strings.ToUpper(strings.TrimSpace(pattern))
	match := func(name string) bool { return pattern == "" || lnm.Match(pattern, name) }

	var out []*Symbol

	seen := map[string]bool{}

	for _, s := range c.Symbols.All() {
		seen[s.Name] = true

		if (!systemOnly || s.IsSystem()) && match(s.Name) {
			out = append(out, s)
		}
	}

	for name, value := range asm.BuiltinSymbols() {
		if !seen[name] && match(name) {
			out = append(out, &Symbol{Name: name, Value: value, Flags: symtab.System | symtab.Builtin})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return out
}

// symbolKindLabel is the attribute list SHOW SYMBOL prints in parentheses.
func symbolKindLabel(s *Symbol) string {
	kind := "user"

	switch {
	case s.IsBuiltin():
		kind = "system, predefined"

	case s.IsSystem():
		kind = "system"
	}

	if s.IsPermanent() {
		kind += ", permanent"
	}

	if s.IsEntry() {
		kind += ", entry"
	}

	if s.IsLabel() {
		kind += ", label"
	}

	return kind
}

// ShowRadix prints the console's current default radix, matching SHOW
// RADIX.
func (c *Console) ShowRadix() error {
	c.Printf("Radix = %d\n", c.Radix)

	return nil
}

// ShowBase prints the console's current EXAMINE/DEPOSIT cursor.
func (c *Console) ShowBase() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.printBase()

	return nil
}

// printBase is ShowBase's body, factored out so ShowCPU can reproduce
// SHOW CPU followed by SHOW BASE without duplicating the format string.
func (c *Console) printBase() {
	c.Printf("\n        Next storage address is %08X\n", c.DepositAddr)
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

var modeNames = [5]string{"KERNEL", "EXEC", "SUPER", "USER", "INTERRUPT"}

func (k StackKind) String() string {
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
// the result.
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
// radix).
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
		modeName = kind.String()
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

// privRegDisplay lists the privileged registers SHOW CPU's own privileged-
// register block.
var privRegDisplay = []struct {
	name string
	reg  vax.PrivReg
}{
	{"P0BR", vax.P0BR}, {"P0LR", vax.P0LR}, {"P1BR", vax.P1BR}, {"P1LR", vax.P1LR},
	{"SBR", vax.SBR}, {"SLR", vax.SLR},
	{"PCBB", vax.PCBB}, {"SCBB", vax.SCBB}, {"IPL", vax.IPL}, {"ASTLVL", vax.ASTLVL},
	{"SIRR", vax.SIRR}, {"SISR", vax.SISR},
	{"ICCS", vax.ICCS}, {"NICR", vax.NICR}, {"ICR", vax.ICR}, {"TODR", vax.TODR},
	{"RXCS", vax.RXCS}, {"RXDB", vax.RXDB}, {"TXCS", vax.TXCS}, {"TXDB", vax.TXDB}, {"TBDR", vax.TBDR},
	{"MAPEN", vax.MAPEN}, {"TBIA", vax.TBIA}, {"TBIS", vax.TBIS},
	{"PMR", vax.PMR}, {"SID", vax.SID}, {"TBCHK", vax.TBCHK},
}

// ShowCPU prints the registers/PSL, mode stack pointers, and privileged
// register block, then falls through to SHOW BASE's "next storage
// address" line.
func (c *Console) ShowCPU() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.dumpRegisters()

	c.Printf("\n    Stack pointers:\n\n")
	c.Printf("    USP: %08X   SSP: %08X   ESP: %08X   KSP: %08X\n    ISP: %08X\n\n",
		c.CPU.PR(vax.USP), c.CPU.PR(vax.SSP), c.CPU.PR(vax.ESP), c.CPU.PR(vax.KSP),
		c.CPU.PR(vax.ISP))

	c.Printf("    Privileged registers:\n\n")

	// Build up a line of entries, flushing it once it gets long enough --
	// matching the C source's own buff/strcat loop. That loop's trailing
	// flush checks buff[1], which is always non-null (buff is initialized
	// to spaces, not empty) and so always true regardless of whether
	// anything was actually appended -- a condition that plainly
	// contradicts its own comment ("it would be null if the buffer was
	// only initialized to a tab character"). Fixed here to check whether
	// anything was appended since the last flush.
	buff := "    "

	for _, e := range privRegDisplay {
		buff += fmt.Sprintf("    %-6s:  %08X", e.name, c.Engine.ReadPR(e.reg))

		if len(buff) > 60 {
			c.Printf("%s\n", buff)

			buff = "    "
		}
	}

	if len(buff) > 4 {
		c.Printf("%s\n", buff)
	}

	c.printBase()

	return nil
}

// ShowRegisterOrPrivReg implements the plain register/privileged-register
// name shortcuts of SHOW (e.g. "SHOW R0", "SHOW PC", "SHOW P0BR").
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
		// ReadPR, not the register file: TODR is computed when read.
		c.Printf("%-6s = %08X\n", name, c.Engine.ReadPR(pr))

		return nil
	}

	return vmserrors.New(vmserrors.CLI_NOTIMPL, name)
}

// ShowMode prints the current privileged access mode, matching SHOW MODE.
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

	if len(c.Engine.Memory().NVRAM) == 0 {
		c.Printf("No NVRAM initialized\n")

		return nil
	}

	size := c.Engine.Memory().NVRAMEnd + 1 - c.Engine.Memory().NVRAMBase
	c.Printf("    NVRAM  FILE=%q\n", c.Engine.Memory().NVRAMFile)
	c.Printf("        CONSOLE$NVRAM_BASE = %08X\n", c.Engine.Memory().NVRAMBase)
	c.Printf("        CONSOLE$NVRAM_END  = %08X\n", c.Engine.Memory().NVRAMEnd)
	c.Printf("        CONSOLE$NVRAM_SIZE = %08X (%dK)\n", size, size/1024)

	return nil
}

// ShowROM prints the loaded ROM image's file name and extent, matching
// SHOW ROM.
func (c *Console) ShowROM() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if len(c.Engine.Memory().ROM) == 0 {
		c.Printf("No ROM loaded\n")

		return nil
	}

	size := c.Engine.Memory().ROMEnd + 1 - c.Engine.Memory().ROMBase
	c.Printf("    ROM FILE=%q\n", c.Engine.Memory().ROMFile)
	c.Printf("        CONSOLE$ROM_BASE  = %08X\n", c.Engine.Memory().ROMBase)
	c.Printf("        CONSOLE$ROM_END   = %08X\n", c.Engine.Memory().ROMEnd)
	c.Printf("        CONSOLE$ROM_SIZE  = %08X (%dK)\n", size, size/1024)

	return nil
}

// ShowShim prints every RTL shim symbol (kernel.asm's `.shim` table, see
// shim.go's ensureShims): for a nonzero
// numeric-dispatch code, whether internal/corevms.Environment has a live
// handler for it; for a code-0 entry, the already-assembled kernel.asm
// routine it resolves to by name -- exactly shim_dump's own "resolves each
// SHIM$<library>_<offset> symbol to a second, separately-defined label at
// the same address" behavior, not a port-specific difference (an earlier
// version of ensureShims synthesized a stub for every entry regardless and
// this function's own doc comment claimed that as the reason for a
// reporting difference from shim_dump; see shim.go's own doc comment for
// why that was wrong, not just under-documented). Addresses are read back
// from each entry's own registered symbol rather than recomputed from
// shimBase, since a code-0 entry's address lives wherever kernel.asm
// defined its routine, not in the synthesized-stub page at all.
func (c *Console) ShowShim() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if !c.shimsReady {
		c.Printf("No RTL shims defined.\n")

		return nil
	}

	c.Printf("RTL SHIMS:\n")

	for _, e := range shimTable {
		name := fmt.Sprintf("SHIM$%s_%08X", e.library, e.offset)

		addr, _ := c.Symbols.Get(name)

		status := fmt.Sprintf("resolved to %s", e.name)
		if e.code != 0 {
			status = "unimplemented"

			if c.RTL.HasShim(e.code) {
				status = "live"
			}
		}

		c.Printf("    %-32s = %08X  code=%-3d %s\n", name, addr, e.code, status)
	}

	return nil
}

// ShowString walks kernel.asm's CONSOLE$STRINGPOOL linked list of VAX
// string descriptors. Requires a booted microkernel.
func (c *Console) ShowString() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	base, ok := c.Symbols.Get("CONSOLE$STRINGPOOL_BASE")
	if !ok {
		return vmserrors.New(vmserrors.CLI_NOSTRINGPOOL)
	}

	size, ok := c.Symbols.Get("CONSOLE$STRINGPOOL_SIZE")
	if !ok {
		return vmserrors.New(vmserrors.CLI_NOPOOLSIZE)
	}

	current, ok := c.Symbols.Get("CONSOLE$STRINGPOOL")
	if !ok {
		return vmserrors.New(vmserrors.CLI_NOPOOL)
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
// given access would be permitted) - a read-only diagnostic that reports
// a PTE's raw contents even for a page a real access would refuse,
// unlike EXAMINE/DEPOSIT which go through the enforcing
// internal/vm.Memory.Translate. The write flag selects /WRITE (the default
// is /READ, matching tracevm's own mode==0 default).
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
	c.Printf("        PROT:  %02X (PTE$K_%s)\n", uint8(pte.Protection()), pte.Protection())
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

// scbVectorNames maps SCP vector byte offsets with exception names.
type scbVectorInfo struct {
	name string
	desc string
}

var scbVectorNames = map[cpu.Exception]scbVectorInfo{
	0x00: {"UNUSED", "Passive Release/Unused"},
	0x04: {"CHECK", " Machine Check"},
	0x08: {"KSNV", "Kernel Stack Not Valid"},
	0x0C: {"POWER", "Power Fail"},
	0x10: {"PRIV", "Reserved or Illegal Instruction"},
	0x14: {"CUSTOMER", "Customer Reserved Exception"},
	0x18: {"RESOP", "Reserved Operand Fault"},
	0x1c: {"READDR", "Reserved Addressing Mode Fault"},
	0x20: {"ACCVIO", "Access Violation"},
	0x24: {"TNV", "Translation Not Valid"},
	0x29: {"TP", "Trace Pending (debugger)"},
	0x2C: {"BPT", "Breakpoint Trap"},
	0x30: {"COMPAT", "Compatibility Mode"},
	0x34: {"ARITH", "Arithmetic Exception"},
	0x40: {"CHMK", "Change Mode to Kernel"},
	0x44: {"CHME", "Change Mode to Exec"},
	0x48: {"CHMS", "Chnage Mode to Supervisor"},
	0x4C: {"CHMU", "Change Mode to User"},
	0x84: {"SOFTWARE1", "Software Interrupt 1"},
	0x88: {"SOFTWARE2", "Software Interrupt 2"},
	0x8C: {"SOFTWARE3", "Software Interrupt 3"},
	0x90: {"SOFTWARE4", "Software Interrupt 4"},
	0xC0: {"INTERVAL", "Interval Timer Interrupt"},
	0xF8: {"CONREAD", "Console Receive"},
	0xFC: {"CONWRITE", "Console Transmit"},
}

// exceptionName returns code's EXC$<name> mnemonic from scbVectorNames
// (code doubles as the SCB byte offset, so code/4 is the slot index), or
// "?" if out of range.
func exceptionName(code cpu.Exception) string {
	name, found := scbVectorNames[code]
	if !found {
		return fmt.Sprintf("RESERVED%0x2", code)
	}

	return name.name
}

// exceptionDesc returns code's EXC$<name> mnemonic from scbVectorNames
// (code doubles as the SCB byte offset, so code/4 is the slot index), or
// "?" if out of range.
func exceptionDesc(code cpu.Exception) string {
	name, found := scbVectorNames[code]
	if !found {
		return fmt.Sprintf("RESERVED Exception %0x2", code)
	}

	return fmt.Sprintf("%-10s %s", name.name, name.desc)
}

// ShowSCB dumps the 64 System Control Block exception-vector slots from
// SCBB. The all flag  requests every slot,
// including empty (zero) ones; the default only prints populated slots.
// SCBB is a physical address.
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

	for n := range uint32(64) {
		// Calculate offset into SCB vector slot
		offset := n << 2

		vector, err := c.Mem.LoadLongword(c.CPU, scbb+offset)
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
		code := cpu.Exception(offset) // Convert SCB offset to exception code

		// Get exception code name, or fall back to default name
		name := fmt.Sprintf("RESERVED%0x2", code)
		if item, found := scbVectorNames[code]; found {
			name = item.name
		}

		if vector == 0xFFFFFFFF {
			c.Printf("     %02X   EXC$%-10s  <emul>  \n", code, name)

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

		c.Printf("     %02X   EXC$%-10s  %08X %s %s\n", code, name, addr, ispFlag, label)
	}

	if count == 0 {
		c.Printf("No exception vector entries defined.\n")
	}

	return nil
}

// ShowCallFrames walks the CALLS/CALLG frame chain from FP, printing
// handler/mask/SPA/calltype/saved-AP/saved-FP/saved-PC/argument list per
// frame. Decodes the mask
// longword using this port's own real-VAX-architecture bit layout (spa
// bits 30-31, calltype bit 29, mask bits 16-27, psw bits 0-15 — see
// internal/cpu/call.go's emulRet, the authoritative decode this mirrors).
func (c *Console) ShowCallFrames(countExpr string) error {
	return c.ShowCalls(countExpr, false)
}

// ShowCalls implements SHOW CALLS[/[NO]SYMBOLIC] [count]. With symbolic,
// and the PC in a module of a loaded image's debug symbol table, it's the
// VMS debugger's table of frames (showDebugCalls; docs/PHASE-41.md,
// subtask 12), every frame when no count is given, as the debugger shows
// them. Otherwise it's the console's own dump of the frames at FP
// (ShowCallFrames), one frame when no count is given.
func (c *Console) ShowCalls(countExpr string, symbolic bool) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	count, given := uint32(1), false

	if s := strings.TrimSpace(countExpr); s != "" {
		v, err := strconv.ParseUint(s, 16, 32)
		if err != nil {
			return vmserrors.Wrap(vmserrors.CLI_BADCOUNT, err, s)
		}

		count, given = uint32(v), true
	}

	if symbolic {
		debugCount := uint32(0)
		if given {
			debugCount = max(count, 1)
		}

		if shown, err := c.showDebugCalls(debugCount); shown || err != nil {
			return err
		}
	}

	fp := c.CPU.GPR(vax.FP)
	ap := c.CPU.GPR(vax.AP)

	if fp == 0 || ap == 0 {
		return vmserrors.New(vmserrors.CLI_NOFRAMES)
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

// ShowRegions prints the P0/P1/S0 image-activation region limits.
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

// ShowImages lists every loaded image (main and sharable dependencies).
// The full flag adds each image's transfer address(es) and dependency list.
func (c *Console) ShowImages(full bool) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if len(c.images().ICBList) == 0 {
		c.Printf("No VMS images loaded in memory.\n")

		return nil
	}

	c.Printf("ACTIVE IMAGES IN MEMORY:\n")

	for _, icb := range c.images().ICBList {
		c.Printf("    %-39s  %08X  %08X%s\n", icb.Name, icb.Base, icb.End, debugKind(icb))

		if !full {
			continue
		}

		if icb.DebugErr != nil {
			c.Printf("        debug symbol table unreadable: %v\n", icb.DebugErr)
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

// debugKind is SHOW IMAGES' note of the debug data an image has: "DEBUG"
// for one linked /DEBUG (a DST with line numbers and symbols, a debug
// module table, and a global symbol table), "TRACEBACK" for one linked
// with traceback only (a DST of modules, routines, and psects), and
// nothing for one linked /NOTRACEBACK.
func debugKind(icb *ICB) string {
	if icb.Debug == nil {
		return ""
	}

	for _, m := range icb.Debug.Modules {
		if len(m.Ranges) > 0 || len(m.Lines) > 0 {
			return "  DEBUG"
		}
	}

	return "  TRACEBACK"
}

// ShowSymbol prints one symbol's value, for the debugger's SHOW SYMBOL
// when the program's debug symbols don't have the name. A name with VMS
// wildcards ("*", "%")
// lists every symbol it matches instead, as ShowSymbols does; a name the
// console's table lacks may still be a predefined system symbol
// (asm.BuiltinSymbol).
func (c *Console) ShowSymbol(name string) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	name = strings.TrimSpace(name)

	if lnm.HasWildcards(name) {
		return c.showSymbolList(name, false)
	}

	sym, ok := c.Symbols.Find(name)
	if !ok {
		v, found := asm.BuiltinSymbol(name)
		if !found {
			return vmserrors.New(vmserrors.CLI_UNDEFSYM, name)
		}

		sym = &Symbol{Name: strings.ToUpper(name), Value: v, Flags: symtab.System | symtab.Builtin}
	}

	c.Printf("    %s = %08X (hex)   %12d (dec)  (%s)\n", sym.Name, sym.Value, int32(sym.Value), symbolKindLabel(sym))

	return nil
}

// ShowSymbolsSystem prints every system (as opposed to user-defined)
// symbol whose name matches pattern (see ShowSymbols), for the
// debugger's SHOW SYMBOL/SYSTEM [pattern].
func (c *Console) ShowSymbolsSystem(pattern string) error {
	return c.showSymbolList(pattern, true)
}

// ShowQuantum reports the interrupt-admission quantum countdown, matching
// SHOW QUANTUM.
func (c *Console) ShowQuantum() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	current, initial := c.Engine.Quantum()

	if c.Engine.HardwareClock() {
		c.Printf("QUANTUM\n    INTERRUPTS      Disabled, Initial=%d\n", initial)
		c.Printf("    HARDWARE CLOCK  Enabled\n")
	} else {
		c.Printf("QUANTUM\n    INTERRUPTS      Initial=%d  Current=%d\n", initial, current)
		c.Printf("    HARDWARE CLOCK  Disabled\n")
	}

	return nil
}

// ShowClock reports the interval-clock/quantum state.  "clock_running" is exactly
// ICCS bit 0 (vax.h: "Copy of ICCS<0>"), so this reads ICCS directly rather than
// needing a separate mirrored field.
func (c *Console) ShowClock() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	// 1. Get the number of 10 ms ticks since January 1st from the TODR
	// register, which the engine computes from its clock when read.
	var ticks int64 = int64(c.Engine.TODR())

	// 2. Define the target year's starting point (January 1st at midnight)
	currentYear := time.Now().Year()
	baseTime := time.Date(currentYear, time.January, 1, 0, 0, 0, 0, time.UTC)

	// 3. Convert ticks to a Go time.Duration
	// 10 milliseconds * number of ticks
	duration := time.Duration(ticks) * 10 * time.Millisecond

	// 4. Add the duration to the baseline date and format a value for the TODR
	resultingTime := baseTime.Add(duration)
	todr := resultingTime.Format("Jan-02 15:04:05")

	running := c.CPU.PR(vax.ICCS)&1 != 0
	runWord := "not "

	if running {
		runWord = ""
	}

	hwClock := "configured"
	if !c.Engine.HardwareClock() {
		hwClock = "emulated"
	}

	c.Printf("   CPU CLOCK STATES:\n")
	c.Printf("      HARDWARE CLOCK=%s, clock is currently %srunning\n", hwClock, runWord)
	c.Printf("      ICR  = %08X (%4d)  [Current clock value]\n", c.CPU.PR(vax.ICR), c.CPU.PR(vax.ICR))
	c.Printf("      NICR = %08X (%4d)  [Reload  clock value]\n", c.CPU.PR(vax.NICR), c.CPU.PR(vax.NICR))
	c.Printf("      TODR = %08X (%s)\n", ticks, strings.ToUpper(todr))

	// If we are using the quantum clock mechanism, report on that now.
	if !c.Engine.HardwareClock() {
		current, initial := c.Engine.Quantum()
		c.Printf("  QUANTUM\n    INITIAL=%d\n    CURRENT=%d\n", initial, current)
	}

	return nil
}

// ShowFault reports the fault/exception event history and pending device/
// software interrupts.
func (c *Console) ShowFault() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	history := c.Engine.FaultHistory()

	if len(history) == 0 {
		c.Printf("    No exceptions or interrupts have occurred yet\n")
	} else {
		c.Printf("FAULT/EXCEPTION EVENT HISTORY:\n")

		for _, fr := range history {
			c.Printf("%02X %-40s SEQ(%d)  PC(%08X) PSL(%08X) ",
				uint8(fr.Code), exceptionDesc(fr.Code), fr.Seq, fr.PC, uint32(fr.PSL))

			if len(fr.Args) > 0 {
				c.Printf(" ARGS(")

				for i, a := range fr.Args {
					if i > 0 {
						c.Printf(",")
					}

					c.Printf("%08X", a)
				}

				c.Printf(") ")
			}

			c.Printf("\n")
		}
	}

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

var tbModeNames = [2]string{"READ", "WRITE"}

// ShowTB implements SHOW TB, the sequential translation cache's own try/hit/
// miss/ratio line, whether TBDR has TB caching enabled, the 128-entry
// translation buffer's own try/hit/miss/ratio/flush/pflush counters, and
// one line per currently-populated TB slot. Ported as of Phase 21, which
// added the real cache this command reports on -- see docs/PHASE-21.md
// (supersedes the "not applicable to this port" stub docs/PHASE-16.md sub-
// phase 1f left behind).
func (c *Console) ShowTB() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	stcTries, stcHits := c.Mem.STCStats()

	c.Printf("Sequential Translation Cache\n")
	c.Printf("    Tries=%d    Hits=%d    Misses=%d    Ratio = %d%%\n",
		stcTries, stcHits, stcTries-stcHits, ratioPercent(stcTries, stcHits))

	enabled := enabledState
	if c.CPU.PR(vax.TBDR) != 0 {
		enabled = disabledState
	}

	c.Printf("\nTranslation buffer caching is %s\n", enabled)

	tries, hits, flushes, pflushes := c.Mem.TBStats()

	c.Printf("    Tries=%d    Hits=%d    Misses=%d    Ratio = %d%%\n",
		tries, hits, tries-hits, ratioPercent(tries, hits))
	c.Printf("    Flushes=%d   PFlushes=%d\n", flushes, pflushes)

	// Each entry's MODE= says what a hit on it lets the CPU's current
	// access mode do: "KERNEL WRITE" (reads and writes), "KERNEL READ"
	// (reads; a write walks the page table, to set the page's modify bit
	// or to fault), or "-NONE-" (every access walks).
	curMod := c.CPU.PSL().CurMod()

	for _, e := range c.Mem.TBSnapshot() {
		mode := "-NONE-"
		if access, ok := e.Permits(curMod); ok {
			mode = modeNames[curMod] + " " + tbModeNames[access]
		}

		c.Printf("    TB(%02X)  VA=%08X  PA=%08X  PROT=%-4s  MODE=%s\n",
			e.Index, e.VA, e.PA, e.Prot, mode)
	}

	return nil
}

// ratioPercent matches SHOW TB's own hit-ratio calculation (a float cast
// truncated to an int for display), reporting 0 rather than dividing by
// zero when tries hasn't happened yet.
func ratioPercent(tries, hits int64) int64 {
	if tries == 0 {
		return 0
	}

	return int64(float64(hits) / float64(tries) * 100.0)
}

// accessAbbrev matches show_instructions()'s own operand-kind abbreviation:
// "src"/"dst"/"mod"/"addr"/"br", "field" for a bit
// field's base (OP_VA, which the generated table now uses; see
// internal/cpu/gen's fieldBases), and "x" for anything else.
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

	case cpu.AccessVarField:
		return "field"

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

// ShowInstructions implements SHOW INSTRUCTIONS: with no qualifier, a
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
		return vmserrors.New(vmserrors.CLI_NOPROFILE)
	}

	if modes {
		return vmserrors.New(vmserrors.CLI_NOMODES)
	}

	table := cpu.Instructions()
	opmatch := int32(-1)

	if s := strings.TrimSpace(opcodeExpr); s != "" {
		v, err := strconv.ParseUint(s, 16, 8)
		if err != nil {
			return vmserrors.Wrap(vmserrors.CLI_BADOPCODE, err, s)
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

// debugShowEntry is one row of ShowDebug's display.
type debugShowEntry struct {
	flag vax.DebugFlags
	name string
	desc string
}

var debugShowEntries = []debugShowEntry{
	{vax.DebugCHM, "CHM", "Debug change-mode operations"},
	{vax.DebugExpand, "COMMAND", "Display command line expansions"},
	{vax.DebugDCL, "DCL", "Debug DCL parsing"},
	{vax.DebugNative, "DEBUG", "Invoke native debugger"},
	{vax.DebugDevices, "DEVICES", "Debug device operations"},
	{vax.DebugExceptions, "EXCEPTIONS", "Debug exception handling"},
	{vax.DebugFullDisasm, "FULLDISASM", "Display operand values on disasm"},
	{vax.DebugImages, "IMAGES", "Display image info on RUN command"},
	{vax.DebugInterrupts, "INTERRUPTS", "Debug interrupt handling"},
	{vax.DebugKeyboard, "KEYBOARD", "Debug console keyboard input"},
	{vax.DebugLibinit, "LIBINIT", "Invoke LIB$INITIALIZE for images"},
	{vax.DebugLogicals, "LOGICALS", "Debug logical name operations"},
	{vax.DebugProcess, "PROCESSES", "Debug process operations"},
	{vax.DebugRegisters, "REGISTERS", "Display changed registers on STEP"},
	{vax.DebugRMS, "RMS", "Debug RMS operations"},
	{vax.DebugServices, "SERVICES", "Debug P1 system service calls"},
	{vax.DebugSymbols, "SYMBOLS", "Debug symbol table handling"},
	{vax.DebugTB, "TB", "Debug translation buffer caching"},
	{vax.DebugUserHalt, "USERHALT", "HALT in user mode halts CPU"},
	{vax.DebugUserStep, "USERSTEP", "Step only affects USER mode"},
	{vax.DebugVM, "VM", "Debug virtual memory translations"},
}

// ShowDebug implements SHOW DEBUG a bit that's set prints its plain name,
// a clear bit prints "NO"+name, both followed by the flag's description.
func (c *Console) ShowDebug() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	debug := c.CPU.Debug()

	c.Printf("DEBUG SETTINGS:\n")

	for _, e := range debugShowEntries {
		name := e.name
		if debug&e.flag == 0 {
			name = "NO" + name
		}

		c.Printf("    %-20s    %s\n", name, e.desc)
	}

	return nil
}

// ShowTrace implements SHOW TRACE (also reached via the DISASSEMBLY
// keyword).
func (c *Console) ShowTrace() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	state := disabledState
	if c.Trace {
		state = enabledState
	}

	c.Printf("    Execution trace disassembly is %s\n", state)

	if c.Trace {
		regState := disabledState
		if c.CPU.DebugEnabled(vax.DebugRegisters) {
			regState = enabledState
		}

		c.Printf("    Register tracking is %s\n", regState)
	}

	return nil
}
