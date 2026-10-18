package machine

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/symtab"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmserrors"
)

const (
	enabledState  = "enabled"
	disabledState = "disabled"
)

// ShowRegisters prints R0-R11 plus the AP/FP/SP/PC aliases in the same
// four-column grid as the PSL beneath them, matching SHOW REGISTERS.
func (m *Machine) ShowRegisters() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	m.dumpRegisters()

	return nil
}

// dumpRegisters prints the R0-R11/AP/FP/SP/PC grid followed by the PSL
// block, shared by SHOW REGISTERS (ShowRegisters) and SHOW CPU (ShowCPU).
func (m *Machine) dumpRegisters() {
	m.Printf("\n\n    Registers:\n\n")
	m.Printf("    R0:  %08X      R4:  %08X     R8:  %08X     AP:  %08X\n",
		m.CPU.GPR(registers.R0), m.CPU.GPR(registers.R4), m.CPU.GPR(registers.R8), m.CPU.GPR(registers.AP))
	m.Printf("    R1:  %08X      R5:  %08X     R9:  %08X     FP:  %08X\n",
		m.CPU.GPR(registers.R1), m.CPU.GPR(registers.R5), m.CPU.GPR(registers.R9), m.CPU.GPR(registers.FP))
	m.Printf("    R2:  %08X      R6:  %08X     R10: %08X     SP:  %08X\n",
		m.CPU.GPR(registers.R2), m.CPU.GPR(registers.R6), m.CPU.GPR(registers.R10), m.CPU.GPR(registers.SP))
	m.Printf("    R3:  %08X      R7:  %08X     R11: %08X     PC:  %08X\n\n",
		m.CPU.GPR(registers.R3), m.CPU.GPR(registers.R7), m.CPU.GPR(registers.R11), m.CPU.GPR(registers.PC))

	m.dumpPSL()
}

// ShowPSL prints the processor status longword and its named fields.
func (m *Machine) ShowPSL() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	m.dumpPSL()

	return nil
}

// dumpPSL prints the PSL longword plus its PSW and PRIV field breakdowns.
func (m *Machine) dumpPSL() {
	p := m.CPU.PSL()

	m.Printf("    PSL: %08X\n", uint32(p))
	m.Printf("         PSW:  C=%s, V=%s, Z=%s, N=%s, T=%s, IV=%s, FU=%s, DV=%s\n",
		boolToDigit(p.C()), boolToDigit(p.V()), boolToDigit(p.Z()), boolToDigit(p.N()),
		boolToDigit(p.T()), boolToDigit(p.IV()), boolToDigit(p.FU()), boolToDigit(p.DV()))
	m.Printf("         PRIV: IPL=%d, CUR_MOD=%s, PRV_MOD=%s, FPD=%s, TP=%s, CM=%s\n",
		p.IPL(), modeNames[p.CurMod()], modeNames[p.PrvMod()],
		boolToDigit(p.FPD()), boolToDigit(p.TP()), boolToDigit(p.CM()))
}

// ShowMemory prints physical memory size and, once VMINIT has established
// page tables, per-region P0/P1/S0 accounting.
func (m *Machine) ShowMemory() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	size := m.Mem.Size()
	pages := size / 512

	megaText := strings.TrimSpace(fmt.Sprintf("%4.2fMb", float64(size)/(1024*1024)))

	m.Printf("\n    Physical Memory (%s)\n        %08X (%d decimal) pages\n        Addresses  00000000 - %08X\n\n",
		megaText, pages, pages, size-1)

	state := disabledState
	if m.CPU.PR(registers.MAPEN) != 0 {
		state = enabledState
	}

	m.Printf("    Virtual Memory (currently %s)\n", state)

	// If VMINIT has never been issued, this port has no console-specific
	// knowledge of the memory layout, so it can't report on it.
	if !m.VMInitValid {
		m.Printf("        Virtual memory configuration is unknown.\n")

		return nil
	}

	mapped := m.Mem.MappedPages()
	m.Printf("    There are %d physical pages mapped, %d free.\n\n", mapped, int(pages)-mapped)

	if db := m.Mem.Paging(); db != nil {
		m.showFrames(db.Counts())

		if m.processOne != nil {
			m.showPaging(m.processOne.PageCounts)
			m.showPageFile(m.processOne.PageFileReport())
		}
	}

	bases := [3]uint32{m.CPU.PR(registers.P0BR), m.CPU.PR(registers.P1BR), m.CPU.PR(registers.SBR)}
	lens := [3]uint32{m.CPU.PR(registers.P0LR), m.CPU.PR(registers.P1LR), m.CPU.PR(registers.SLR)}

	for n, r := range m.Regions {
		plural := "s"
		if r.pteCount == 1 {
			plural = " "
		}

		m.Printf("        %s Region\n            Size = %08X (%5d decimal) pages\n            PFN database = %d page%s",
			r.name, r.size, r.size, r.pteCount, plural)

		m.Printf("  %sBR = %08X    %sLR = %08X\n", r.name, bases[n], r.name, lens[n])

		m.Printf("            Physical addresses = %08X-%08X\n            Virtual addresses  = %08X-%08X\n\n",
			r.pStart, r.pEnd-1, r.vStart, r.vEnd-1)
	}

	return nil
}

// PagingStatistics is what SHOW MEMORY shows of paging, gathered for
// govax's --stats report at exit (docs/PHASE-64 - page file.md, subtask
// 10): the page frames by place, the system's page faults and the
// writer's and reclaimer's work, and the page file.
type PagingStatistics struct {
	Frames   paging.Counts
	Pages    corevms.PageCounts
	PageFile corevms.PageFileReport
}

// PagingStatistics returns the paging figures SHOW MEMORY shows, and
// false when paging is off (or VMINIT hasn't made process 1), when
// there are none.
func (m *Machine) PagingStatistics() (PagingStatistics, bool) {
	db := m.Mem.Paging()
	if db == nil || m.processOne == nil {
		return PagingStatistics{}, false
	}

	return PagingStatistics{
		Frames:   db.Counts(),
		Pages:    m.processOne.PageCounts,
		PageFile: m.processOne.PageFileReport(),
	}, true
}

// showFrames prints, in paging mode (VMINIT/PAGING), how many page frames
// are in each place the PFN database keeps them: the free list, the
// modified list (pages waiting to be written to the page file), and in
// use, by the kind of page each holds.
func (m *Machine) showFrames(c paging.Counts) {
	m.Printf("    Page frames (paging on)\n")
	m.Printf("        Free list          %8d\n", c.Free)
	m.Printf("        Modified list      %8d\n", c.Modified)
	m.Printf("        Active             %8d\n", c.Active())
	m.Printf("            Process pages  %8d\n", c.Process)
	m.Printf("            Global pages   %8d\n", c.Global)
	m.Printf("            System pages   %8d\n\n", c.System)
}

// showPaging prints, in paging mode, the system's page faults by how
// each was resolved (soft faults taking a frame back off the free or
// modified list, or finding a global page in memory; demand-zero faults;
// hard faults reading a file, and of those the page file), the pages
// the modified page writer has written, and the reclaimer's runs and the
// pages it has trimmed from working sets (internal/corevms's accounting.go). VMS shows its fault
// rates in MONITOR PAGE, not SHOW MEMORY; these lines are govax's own.
func (m *Machine) showPaging(c corevms.PageCounts) {
	f := c.Faults

	m.Printf("    Page faults        %12d\n", f.Total())
	m.Printf("        Free list          %8d\n", f.FreeList)
	m.Printf("        Modified list      %8d\n", f.ModifiedList)
	m.Printf("        Global valid       %8d\n", f.GlobalValid)
	m.Printf("        Demand zero        %8d\n", f.DemandZero)
	m.Printf("        Read               %8d\n", f.Read)
	m.Printf("            Page file      %8d\n", c.PageFileReads)
	m.Printf("    Pages written      %12d\n", c.Written)
	m.Printf("        Write I/Os         %8d\n", c.WriteIOs)
	m.Printf("    Reclaimer runs     %12d\n", c.Reclaims)
	m.Printf("        Pages trimmed      %8d\n\n", c.Trimmed)
}

// showPageFile prints, in paging mode, the page file's section (Phase
// 64, subtask 7; govax's own layout, VMS's SHOW MEMORY/FILES being
// another): its name and state, and when it's open, its size, its free
// and used blocks (each holds one page), the run of blocks its map
// looks for now, and whether it has passed the PAGEFRAG and PAGECRIT
// thresholds or the writer last found it full. Committed is the pages
// charged against every job's PGFLQUOTA, which the file need not hold
// yet: a page is charged when it's made, and takes a block only when
// the modified page writer writes it.
func (m *Machine) showPageFile(r corevms.PageFileReport) {
	switch r.Status {
	case corevms.PageFileNone:
		m.Printf("    Page file: none\n")
	case corevms.PageFileNotOpened:
		m.Printf("    Page file: %s (not opened yet)\n", r.Name)
	case corevms.PageFileUnavailable:
		m.Printf("    Page file: %s (not available)\n", r.Name)
	case corevms.PageFileInUse:
		m.Printf("    Page file: %s\n", r.Name)
		m.Printf("        Size               %8d\n", r.Size)
		m.Printf("        Free               %8d\n", r.Free)
		m.Printf("        In use             %8d\n", r.Used)
		m.Printf("        Allocation size    %8d\n", r.AllocationSize)

		var state []string

		switch r.Warned {
		case paging.PageFrag:
			state = append(state, "past PAGEFRAG (65%)")
		case paging.PageCrit:
			state = append(state, "past PAGECRIT (90%)")
		}

		if r.Full {
			state = append(state, "full")
		}

		if len(state) > 0 {
			m.Printf("        State              %s\n", strings.Join(state, ", "))
		}
	}

	m.Printf("    Page file committed%12d\n\n", r.Charged)
}

// ShowMemoryFiles is SHOW MEMORY/FILES, VMS's report of the page and
// swap files, in VMS 7.1's VAX layout (testdata/pg64): its title, then a
// line of headings and, for the page file, its name on a line of its own
// and its free, reservable, and total pages. With full (/FILES/FULL) each
// file has a block of its details instead. govax has one page file and
// no swap file; the page file's number is VMS's (corevms.PageFileIndex),
// its reservable pages are those not committed to a job's PGFLQUOTA, and
// it's listed only while it's open (with paging off, deferred, or
// unavailable, the report has no files).
func (m *Machine) ShowMemoryFiles(full bool) error {
	if err := m.requireInit(); err != nil {
		return err
	}

	if m.processOne == nil {
		return nil
	}

	r := m.processOne.PageFileReport()
	open := r.Status == corevms.PageFileInUse
	reservable := int64(r.Size) - int64(r.Charged)

	m.Printf("%s\n\n", m.processOne.MemoryResourcesTitle())

	if !full {
		m.Printf("%-26s%26s%12s%12s\n", "Paging File Usage (pages):", "Free", "Reservable", "Total")

		if open {
			m.Printf("  %-78s\n%52d%12d%12d\n", r.FilesName, r.Free, reservable, r.Size)
		}

		return nil
	}

	if !open {
		return nil
	}

	// Each line is two halves of 38 columns, a label and its value
	// right-aligned at the half's end.
	half := func(label string, n int64) string {
		return fmt.Sprintf("    %s%*d", label, 34-len(label), n)
	}

	m.Printf("%-80s\n", r.FilesName)
	m.Printf("%s%s\n", half("Free Blocks", int64(r.Free)), half("Reservable Blocks", reservable))
	m.Printf("%s%s\n", half("Total Size (blocks)", int64(r.Size)), half("Paging File Number", corevms.PageFileIndex))
	m.Printf("%s%s\n", half("Swap Usage (processes)", 0), half("Paging Usage (processes)", int64(r.Processes)))
	m.Printf("  This file can be used for either paging or swapping.\n")

	return nil
}

// ShowSymbols prints every defined symbol whose name matches pattern,
// for the debugger's SHOW SYMBOL/ALL [pattern]: VMS wildcards, "*" for any run of
// characters and "%" for exactly one; an empty pattern matches every
// name. The assembler's predefined system symbols are listed too, as the
// C source's one shared symbol table listed them.
func (m *Machine) ShowSymbols(pattern string) error {
	return m.showSymbolList(pattern, false)
}

// showSymbolList prints the symbols listSymbols selects, one per line, or
// reports CLI_UNDEFSYM when a pattern matched nothing.
func (m *Machine) showSymbolList(pattern string, systemOnly bool) error {
	if err := m.requireInit(); err != nil {
		return err
	}

	syms := m.listSymbols(pattern, systemOnly)
	if len(syms) == 0 && strings.TrimSpace(pattern) != "" {
		return vmserrors.New(vmserrors.CLI_UNDEFSYM, strings.TrimSpace(pattern))
	}

	for _, s := range syms {
		m.Printf("%-31s = %08X  (%s)\n", s.Name, s.Value, symbolKindLabel(s))
	}

	return nil
}

// listSymbols returns, sorted by name, every console symbol and every
// predefined symbol (asm.BuiltinSymbols) not shadowed by a console symbol
// of the same name, whose name matches pattern (VMS wildcards; empty
// matches all). systemOnly leaves out user symbols.
func (m *Machine) listSymbols(pattern string, systemOnly bool) []*Symbol {
	pattern = strings.ToUpper(strings.TrimSpace(pattern))
	match := func(name string) bool { return pattern == "" || lnm.Match(pattern, name) }

	var out []*Symbol

	seen := map[string]bool{}

	for _, s := range m.Symbols.All() {
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
func (m *Machine) ShowRadix() error {
	m.Printf("Radix = %d\n", m.Radix)

	return nil
}

// ShowVersion is SHOW VERSION: which microkernel govax's console is.
func (m *Machine) ShowVersion() error {
	m.Printf("govax Console Microkernel 2.0\nBy Tom Cole\n")

	return nil
}

// ShowBase prints the console's current EXAMINE/DEPOSIT cursor.
func (m *Machine) ShowBase() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	m.printBase()

	return nil
}

// printBase is ShowBase's body, factored out so ShowCPU can reproduce
// SHOW CPU followed by SHOW BASE without duplicating the format string.
func (m *Machine) printBase() {
	m.Printf("\n        Next storage address is %08X\n", m.DepositAddr)
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
// handlefault.go's `SetPR(registers.PrivReg(curMod), GPR(SP))` before a fault
// switches to a new mode) for where that save happens; this only reads
// the result.
func (m *Machine) stackPointerFor(kind StackKind) uint32 {
	psl := m.CPU.PSL()

	if kind == StackISP {
		if psl.IS() {
			return m.CPU.GPR(registers.SP)
		}

		return m.CPU.PR(registers.ISP)
	}

	var (
		mode registers.AccessMode
		reg  registers.PrivReg
	)

	switch kind {
	case StackESP:
		mode, reg = 1, registers.ESP
	case StackSSP:
		mode, reg = 2, registers.SSP
	case StackUSP:
		mode, reg = 3, registers.USP
	default:
		mode, reg = 0, registers.KSP
	}

	if !psl.IS() && psl.CurMod() == mode {
		return m.CPU.GPR(registers.SP)
	}

	return m.CPU.PR(reg)
}

// ShowStack dumps live memory from a privileged mode's stack pointer
// upward (one longword per line, formatted in the console's current
// radix).
func (m *Machine) ShowStack(kind StackKind, current bool, count uint32, all bool) error {
	if err := m.requireInit(); err != nil {
		return err
	}

	psl := m.CPU.PSL()

	var (
		sp       uint32
		modeName string
	)

	if current {
		sp = m.CPU.GPR(registers.SP)

		modeIdx := int(psl.CurMod())
		if psl.IS() {
			modeIdx = 4
		}

		modeName = modeNames[modeIdx]
	} else {
		sp = m.stackPointerFor(kind)
		modeName = kind.String()
	}

	m.Printf("    %s MODE SP = %08X:\n", modeName, sp)

	if !all && (count < 1 || count > 255) {
		count = 1
	}

	mapen := m.CPU.PR(registers.MAPEN)

	for n := uint32(0); all || n < count*4; n += 4 {
		addr := sp + n

		if (psl.CurMod() == 3 && mapen != 0 && addr >= 0x7FFFFFFF) ||
			(mapen == 0 && addr >= m.Mem.Size()) {
			m.Printf("      <end of memory>\n")

			break
		}

		v, err := m.Mem.LoadLongword(m.CPU, addr)
		if err != nil {
			m.Printf("      <end of stack>\n")

			break
		}

		if m.Radix == 16 {
			m.Printf("      SP+%04X [%08X]:  %08X\n", n, addr, v)
		} else {
			m.Printf("      SP+%04d [%08X]:  %d\n", n, addr, int32(v))
		}
	}

	return nil
}

// privRegDisplay lists the privileged registers SHOW CPU's own privileged-
// register block.
var privRegDisplay = []struct {
	name string
	reg  registers.PrivReg
}{
	{"P0BR", registers.P0BR}, {"P0LR", registers.P0LR}, {"P1BR", registers.P1BR}, {"P1LR", registers.P1LR},
	{"SBR", registers.SBR}, {"SLR", registers.SLR},
	{"PCBB", registers.PCBB}, {"SCBB", registers.SCBB}, {"IPL", registers.IPL}, {"ASTLVL", registers.ASTLVL},
	{"SIRR", registers.SIRR}, {"SISR", registers.SISR},
	{"ICCS", registers.ICCS}, {"NICR", registers.NICR}, {"ICR", registers.ICR}, {"TODR", registers.TODR},
	{"RXCS", registers.RXCS}, {"RXDB", registers.RXDB}, {"TXCS", registers.TXCS}, {"TXDB", registers.TXDB}, {"TBDR", registers.TBDR},
	{"MAPEN", registers.MAPEN}, {"TBIA", registers.TBIA}, {"TBIS", registers.TBIS},
	{"PMR", registers.PMR}, {"SID", registers.SID}, {"TBCHK", registers.TBCHK},
}

// ShowCPU prints the registers/PSL, mode stack pointers, and privileged
// register block, then falls through to SHOW BASE's "next storage
// address" line.
func (m *Machine) ShowCPU() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	m.dumpRegisters()

	m.Printf("\n    Stack pointers:\n\n")
	m.Printf("    USP: %08X   SSP: %08X   ESP: %08X   KSP: %08X\n    ISP: %08X\n\n",
		m.CPU.PR(registers.USP), m.CPU.PR(registers.SSP), m.CPU.PR(registers.ESP), m.CPU.PR(registers.KSP),
		m.CPU.PR(registers.ISP))

	m.Printf("    Privileged registers:\n\n")

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
		buff += fmt.Sprintf("    %-6s:  %08X", e.name, m.Engine.ReadPR(e.reg))

		if len(buff) > 60 {
			m.Printf("%s\n", buff)

			buff = "    "
		}
	}

	if len(buff) > 4 {
		m.Printf("%s\n", buff)
	}

	m.printBase()

	return nil
}

// ShowRegisterOrPrivReg implements the plain register/privileged-register
// name shortcuts of SHOW (e.g. "SHOW R0", "SHOW PC", "SHOW P0BR").
func (m *Machine) ShowRegisterOrPrivReg(name string) error {
	if err := m.requireInit(); err != nil {
		return err
	}

	name = strings.ToUpper(name)
	if r, ok := registerNames[name]; ok {
		m.Printf("%-4s = %08X\n", name, m.CPU.GPR(r))

		return nil
	}

	if pr, ok := privRegNames[name]; ok {
		// ReadPR, not the register file: TODR is computed when read.
		m.Printf("%-6s = %08X\n", name, m.Engine.ReadPR(pr))

		return nil
	}

	return vmserrors.New(vmserrors.CLI_NOTIMPL, name)
}

// ShowMode prints the current privileged access mode, matching SHOW MODE.
func (m *Machine) ShowMode() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	m.Printf("Current MODE is %s\n", modeNames[m.CPU.PSL().CurMod()])

	return nil
}

// ShowNVRAM prints the loaded NVRAM image's file name and extent, matching
// SHOW NVRAM.
func (m *Machine) ShowNVRAM() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	if len(m.Engine.Memory().NVRAM) == 0 {
		m.Printf("No NVRAM initialized\n")

		return nil
	}

	size := m.Engine.Memory().NVRAMEnd + 1 - m.Engine.Memory().NVRAMBase
	m.Printf("    NVRAM  FILE=%q\n", m.Engine.Memory().NVRAMFile)
	m.Printf("        CONSOLE$NVRAM_BASE = %08X\n", m.Engine.Memory().NVRAMBase)
	m.Printf("        CONSOLE$NVRAM_END  = %08X\n", m.Engine.Memory().NVRAMEnd)
	m.Printf("        CONSOLE$NVRAM_SIZE = %08X (%dK)\n", size, size/1024)

	return nil
}

// ShowROM prints the loaded ROM image's file name and extent, matching
// SHOW ROM.
func (m *Machine) ShowROM() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	if len(m.Engine.Memory().ROM) == 0 {
		m.Printf("No ROM loaded\n")

		return nil
	}

	size := m.Engine.Memory().ROMEnd + 1 - m.Engine.Memory().ROMBase
	m.Printf("    ROM FILE=%q\n", m.Engine.Memory().ROMFile)
	m.Printf("        CONSOLE$ROM_BASE  = %08X\n", m.Engine.Memory().ROMBase)
	m.Printf("        CONSOLE$ROM_END   = %08X\n", m.Engine.Memory().ROMEnd)
	m.Printf("        CONSOLE$ROM_SIZE  = %08X (%dK)\n", size, size/1024)

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
func (m *Machine) ShowShim() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	if !m.shimsReady {
		m.Printf("No RTL shims defined.\n")

		return nil
	}

	m.Printf("RTL SHIMS:\n")

	for _, e := range ShimTable {
		name := fmt.Sprintf("SHIM$%s_%08X", e.Library, e.Offset)

		addr, _ := m.Symbols.Get(name)

		status := fmt.Sprintf("resolved to %s", e.Name)
		if e.Code != 0 {
			status = "unimplemented"

			if m.processOne.HasShim(e.Code) {
				status = "live"
			}
		}

		m.Printf("    %-32s = %08X  code=%-3d %s\n", name, addr, e.Code, status)
	}

	return nil
}

// ShowString walks kernel.asm's CONSOLE$STRINGPOOL linked list of VAX
// string descriptors. Requires a booted microkernel.
func (m *Machine) ShowString() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	base, ok := m.Symbols.Get("CONSOLE$STRINGPOOL_BASE")
	if !ok {
		return vmserrors.New(vmserrors.CLI_NOSTRINGPOOL)
	}

	size, ok := m.Symbols.Get("CONSOLE$STRINGPOOL_SIZE")
	if !ok {
		return vmserrors.New(vmserrors.CLI_NOPOOLSIZE)
	}

	current, ok := m.Symbols.Get("CONSOLE$STRINGPOOL")
	if !ok {
		return vmserrors.New(vmserrors.CLI_NOPOOL)
	}

	m.Printf("STRING POOL\n")
	m.Printf("    CONSOLE$STRINGPOOL_BASE       %08X\n", base)
	m.Printf("    CONSOLE$STRINGPOOL_SIZE       %08X\n", size)
	m.Printf("    CONSOLE$STRINGPOOL (Current)  %08X\n", current)

	addr, err := m.Mem.LoadLongword(m.CPU, base)
	if err != nil {
		return err
	}

	if addr != 0 {
		m.Printf("\n")
	}

	for addr != 0 {
		lenRaw, err := m.Mem.LoadLongword(m.CPU, addr)
		if err != nil {
			return err
		}

		descLen := lenRaw & 0xFFFF

		descAddr, err := m.Mem.LoadLongword(m.CPU, addr+4)
		if err != nil {
			return err
		}

		desc := addr

		next, err := m.Mem.LoadLongword(m.CPU, addr+8)
		if err != nil {
			return err
		}

		n := descLen

		truncated := n > 250
		if truncated {
			n = 250
		}

		buf := make([]byte, n)
		if err := m.Mem.Load(m.CPU, descAddr, buf); err != nil {
			return err
		}

		text := string(buf)
		if truncated {
			text += "..."
		}

		m.Printf("    Descriptor %08X [%3d bytes]  \"%s\"\n", desc, descLen, text)

		addr = next
	}

	m.Printf("\n")

	return nil
}

// ShowPage reports one virtual address's page table entry (VALID/PROT/M/
// OWNER/S/PFN, plus the physical address it resolves to and whether the
// given access would be permitted) - a read-only diagnostic that reports
// a PTE's raw contents even for a page a real access would refuse,
// unlike EXAMINE/DEPOSIT which go through the enforcing
// internal/vm.Memory.Translate. The write flag selects /WRITE (the default
// is /READ, matching tracevm's own mode==0 default).
func (m *Machine) ShowPage(addrExpr string, write bool) error {
	if err := m.requireInit(); err != nil {
		return err
	}

	if m.CPU.PR(registers.MAPEN) == 0 {
		m.Printf("Virtual memory is disabled.\n")

		return nil
	}

	v, _, err := m.Evaluator().Eval(strings.TrimSpace(addrExpr))
	if err != nil {
		return err
	}

	region, pteAddr, pte, err := m.Mem.LookupPTE(m.CPU, v)
	if err != nil {
		m.Printf("ACCVIO, page table region %d length violation\n", region)

		return nil
	}

	byteOffset := v & 0x1FF

	m.Printf("    Virtual address:   %08X\n", v)
	m.Printf("    Region:            %02d\n", region)
	m.Printf("    PTE Address:       %08X\n", pteAddr)
	m.Printf("    PTE Longword:      %08X\n", uint32(pte))
	m.Printf("    PTE Form:          %s\n", pteForm(pte))
	m.Printf("        VALID: %s\n", boolToDigit(pte.Valid()))
	m.Printf("        PROT:  %02X (PTE$K_%s)\n", uint8(pte.Protection()), pte.Protection())
	m.Printf("        M:     %s\n", boolToDigit(pte.Modified()))
	m.Printf("        OWNER: %X\n", pte.Owner())
	m.Printf("        S:     %X\n", pte.Software())
	m.Printf("        PFN:   %08X\n", pte.PFN())
	m.Printf("    Physical address:  %08X\n", pte.PFN()<<9+byteOffset)

	access, label := vm.AccessRead, "VM_READ"
	if write {
		access, label = vm.AccessWrite, "VM_WRITE"
	}

	m.Printf("    Access:      %s\n", label)

	if !pte.Protection().Allows(m.CPU.PSL().CurMod(), access) {
		m.Printf("ACCVIO, protection violation\n")

		return nil
	}

	if !pte.Valid() {
		m.Printf("TNV, valid bit not set\n")
	}

	return nil
}

// pteForm names the form a PTE takes (internal/vm's pager.go; *VAX/VMS
// Internals and Data Structures*, figure 14-3) and the number its form
// holds. An invalid PTE's type bits say where the page is: a process
// section, a global section, the page file, a frame on the free or
// modified list (in transition), or nowhere yet (demand zero). The forms
// are named by their bits alone; with paging off, govax never writes the
// page file or transition forms, and treats any invalid page that isn't a
// section or global page as demand zero.
func pteForm(pte vm.PTE) string {
	if pte.Valid() {
		return fmt.Sprintf("valid, PFN %08X", pte.PFN())
	}

	if i, ok := pte.SectionIndex(); ok {
		return fmt.Sprintf("process section, PSTX %X", i)
	}

	if i, ok := pte.GlobalIndex(); ok {
		return fmt.Sprintf("global page, GPTX %X", i)
	}

	if b, ok := pte.PageFileBlock(); ok {
		if b == 0 {
			return "page file, block not yet allocated"
		}

		return fmt.Sprintf("page file, block %X", b)
	}

	if pfn, ok := pte.Transition(); ok {
		return fmt.Sprintf("transition, PFN %08X", pfn)
	}

	return "demand zero"
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
func (m *Machine) ShowSCB() error { return m.showSCB(false) }

// ShowSCBAll implements SHOW SCB/ALL — see ShowSCB.
func (m *Machine) ShowSCBAll() error { return m.showSCB(true) }

func (m *Machine) showSCB(all bool) error {
	if err := m.requireInit(); err != nil {
		return err
	}

	scbb := m.CPU.PR(registers.SCBB)
	if scbb == 0 {
		m.Printf("No SCB established (SCBB=0)\n")

		return nil
	}

	m.Printf("SYSTEM CONTROL BLOCK\n    SCBB register = %08X\n", scbb)

	savedMapen := m.CPU.PR(registers.MAPEN)
	m.CPU.SetPR(registers.MAPEN, 0)

	defer m.CPU.SetPR(registers.MAPEN, savedMapen)

	count := 0

	for n := range uint32(64) {
		// Calculate offset into SCB vector slot
		offset := n << 2

		vector, err := m.Mem.LoadLongword(m.CPU, scbb+offset)
		if err != nil {
			return err
		}

		if vector == 0 && !all {
			continue
		}

		if count == 0 {
			m.Printf("    Slot  Name            Address  ISP?  Label\n")
		}

		count++
		code := cpu.Exception(offset) // Convert SCB offset to exception code

		// Get exception code name, or fall back to default name
		name := fmt.Sprintf("RESERVED%0x2", code)
		if item, found := scbVectorNames[code]; found {
			name = item.name
		}

		if vector == 0xFFFFFFFF {
			m.Printf("     %02X   EXC$%-10s  <emul>  \n", code, name)

			continue
		}

		stack := vector & 0x3
		addr := vector &^ 0x3

		label := ""
		if name, ok := m.Symbols.FindByValue(addr); ok {
			label = name
		}

		ispFlag := "     "
		if stack != 0 {
			ispFlag = "(ISP)"
		}

		m.Printf("     %02X   EXC$%-10s  %08X %s %s\n", code, name, addr, ispFlag, label)
	}

	if count == 0 {
		m.Printf("No exception vector entries defined.\n")
	}

	return nil
}

// ShowCallFrames walks the CALLS/CALLG frame chain from FP, printing
// handler/mask/SPA/calltype/saved-AP/saved-FP/saved-PC/argument list per
// frame. Decodes the mask
// longword using this port's own real-VAX-architecture bit layout (spa
// bits 30-31, calltype bit 29, mask bits 16-27, psw bits 0-15 — see
// internal/cpu/call.go's emulRet, the authoritative decode this mirrors).
func (m *Machine) ShowCallFrames(countExpr string) error {
	return m.ShowCalls(countExpr, false)
}

// ShowCalls implements SHOW CALLS[/[NO]SYMBOLIC] [count]. With symbolic,
// and the PC in a module of a loaded image's debug symbol table, it's the
// VMS debugger's table of frames (showDebugCalls; docs/PHASE-41.md,
// subtask 12), every frame when no count is given, as the debugger shows
// them. Otherwise it's the console's own dump of the frames at FP
// (ShowCallFrames), one frame when no count is given.
func (m *Machine) ShowCalls(countExpr string, symbolic bool) error {
	if err := m.requireInit(); err != nil {
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

		if shown, err := m.showDebugCalls(debugCount); shown || err != nil {
			return err
		}
	}

	fp := m.CPU.GPR(registers.FP)
	ap := m.CPU.GPR(registers.AP)

	if fp == 0 || ap == 0 {
		return vmserrors.New(vmserrors.CLI_NOFRAMES)
	}

	for ; count > 0; count-- {
		m.Printf("    FRAME: %08X", fp)

		addr := fp

		handler, err := m.Mem.LoadLongword(m.CPU, addr)
		if err != nil {
			return err
		}

		if handler != 0 {
			m.Printf(", Handler: %08X", handler)
		}

		addr += 4

		maskWord, err := m.Mem.LoadLongword(m.CPU, addr)
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

		m.Printf(", SPA: %1X, %s, MASK: %04X, PSW: %04X\n", spa, callType, mask, psw)

		addr += 4

		savedAP, err := m.Mem.LoadLongword(m.CPU, addr)
		if err != nil {
			return err
		}

		m.Printf("        Saved AP : %08X\n", savedAP)

		addr += 4

		savedFP, err := m.Mem.LoadLongword(m.CPU, addr)
		if err != nil {
			return err
		}

		if savedFP == cpu.SentinelReturn {
			m.Printf("        Saved FP : <console>\n")
		} else {
			m.Printf("        Saved FP : %08X\n", savedFP)
		}

		addr += 4

		savedPC, err := m.Mem.LoadLongword(m.CPU, addr)
		if err != nil {
			return err
		}

		if savedPC == cpu.SentinelReturn {
			m.Printf("        Saved PC : <console>\n")
		} else {
			m.Printf("        Saved PC : %08X\n", savedPC)
		}

		addr += 4

		for n := uint32(0); n <= 11; n++ {
			if mask&(1<<n) == 0 {
				continue
			}

			v, err := m.Mem.LoadLongword(m.CPU, addr)
			if err != nil {
				return err
			}

			addr += 4

			m.Printf("        Saved R%-2d: %08X\n", n, v)
		}

		var argc uint32

		if ap >= 0x200 && ap < 0x7FFFFFFC {
			argc, err = m.Mem.LoadLongword(m.CPU, ap)
			if err != nil {
				return err
			}
		} else {
			m.Printf("        (AP) is not an argument list\n")
		}

		switch {
		case argc > 255:
			m.Printf("        (AP) is not an argument list\n")

		case argc > 0:
			word, plural := "is", ""
			if argc != 1 {
				word, plural = "are", "s"
			}

			m.Printf("        There %s %d argument%s:\n", word, argc, plural)

			a := ap

			for n := uint32(1); n <= argc; n++ {
				if n > 15 {
					m.Printf("            ...and %d more...\n", argc-n)

					break
				}

				a += 4

				v, err := m.Mem.LoadLongword(m.CPU, a)
				if err != nil {
					return err
				}

				m.Printf("            [%2d]: %08X\n", n, v)
			}

			if argc > 64 {
				m.Printf("            Note: AP is probably bogus for this call frame\n")
			}
		}

		ap = savedAP
		fp = savedFP

		if fp == 0 || fp == cpu.SentinelReturn {
			break
		}

		m.Printf("\n")
	}

	return nil
}

// ShowSharePrefix reports the sharable-image search prefix, matching SHOW
// SHARE_PREFIX.
func (m *Machine) ShowSharePrefix() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	switch {
	case m.SharePrefix != "" && m.SharePrefix[0] == 0x1B:
		m.Printf("Recursive sharable image loading disabled.\n")

	case m.SharePrefix == "":
		m.Printf("No sharable image prefix defined\n")

	default:
		m.Printf("Sharable images loaded from \"%s\"\n", m.SharePrefix)
	}

	return nil
}

// ShowImages lists every loaded image (main and sharable dependencies).
// The full flag adds each image's transfer address(es) and dependency list.
func (m *Machine) ShowImages(full bool) error {
	if err := m.requireInit(); err != nil {
		return err
	}

	if len(m.images().ICBList) == 0 {
		m.Printf("No VMS images loaded in memory.\n")

		return nil
	}

	m.Printf("ACTIVE IMAGES IN MEMORY:\n")

	for _, icb := range m.images().ICBList {
		m.Printf("    %-39s  %08X  %08X%s\n", icb.Name, icb.Base, icb.End, debugKind(icb))

		if !full {
			continue
		}

		if icb.DebugErr != nil {
			m.Printf("        debug symbol table unreadable: %v\n", icb.DebugErr)
		}

		if icb.Transfer[0] == 0 {
			m.Printf("        no transfer addresses\n")
		} else {
			suffix := ""
			if icb.Transfer[1] != 0 {
				suffix = "es"
			}

			m.Printf("        transfer address%s ", suffix)

			for _, t := range icb.Transfer {
				if t != 0 {
					m.Printf("%08X  ", t)
				}
			}

			m.Printf("\n")
		}

		if len(icb.SHRList) > 1 {
			m.Printf("        depends on ")

			first := true

			for _, shr := range icb.SHRList {
				if shr.ID == 0 {
					continue // the self-reference SHR entry
				}

				if !first {
					m.Printf(", ")
				}

				first = false

				m.Printf("%s", shr.Name)
			}

			m.Printf("\n")
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
func (m *Machine) ShowSymbol(name string) error {
	if err := m.requireInit(); err != nil {
		return err
	}

	name = strings.TrimSpace(name)

	if lnm.HasWildcards(name) {
		return m.showSymbolList(name, false)
	}

	sym, ok := m.Symbols.Find(name)
	if !ok {
		v, found := asm.BuiltinSymbol(name)
		if !found {
			return vmserrors.New(vmserrors.CLI_UNDEFSYM, name)
		}

		sym = &Symbol{Name: strings.ToUpper(name), Value: v, Flags: symtab.System | symtab.Builtin}
	}

	m.Printf("    %s = %08X (hex)   %12d (dec)  (%s)\n", sym.Name, sym.Value, int32(sym.Value), symbolKindLabel(sym))

	return nil
}

// ShowSymbolsSystem prints every system (as opposed to user-defined)
// symbol whose name matches pattern (see ShowSymbols), for the
// debugger's SHOW SYMBOL/SYSTEM [pattern].
func (m *Machine) ShowSymbolsSystem(pattern string) error {
	return m.showSymbolList(pattern, true)
}

// ShowQuantum reports the interrupt-admission quantum countdown, matching
// SHOW QUANTUM.
func (m *Machine) ShowQuantum() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	current, initial := m.Engine.Quantum()

	if m.Engine.HardwareClock() {
		m.Printf("QUANTUM\n    INTERRUPTS      Disabled, Initial=%d\n", initial)
		m.Printf("    HARDWARE CLOCK  Enabled\n")
	} else {
		m.Printf("QUANTUM\n    INTERRUPTS      Initial=%d  Current=%d\n", initial, current)
		m.Printf("    HARDWARE CLOCK  Disabled\n")
	}

	return nil
}

// ShowClock reports the interval-clock/quantum state.  "clock_running" is exactly
// ICCS bit 0 (vax.h: "Copy of ICCS<0>"), so this reads ICCS directly rather than
// needing a separate mirrored field.
func (m *Machine) ShowClock() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	// 1. Get the number of 10 ms ticks since January 1st from the TODR
	// register, which the engine computes from its clock when read.
	var ticks int64 = int64(m.Engine.TODR())

	// 2. Define the target year's starting point (January 1st at midnight)
	currentYear := time.Now().Year()
	baseTime := time.Date(currentYear, time.January, 1, 0, 0, 0, 0, time.UTC)

	// 3. Convert ticks to a Go time.Duration
	// 10 milliseconds * number of ticks
	duration := time.Duration(ticks) * 10 * time.Millisecond

	// 4. Add the duration to the baseline date and format a value for the TODR
	resultingTime := baseTime.Add(duration)
	todr := resultingTime.Format("Jan-02 15:04:05")

	running := m.CPU.PR(registers.ICCS)&1 != 0
	runWord := "not "

	if running {
		runWord = ""
	}

	hwClock := "configured"
	if !m.Engine.HardwareClock() {
		hwClock = "emulated"
	}

	m.Printf("   CPU CLOCK STATES:\n")
	m.Printf("      HARDWARE CLOCK=%s, clock is currently %srunning\n", hwClock, runWord)
	m.Printf("      ICR  = %08X (%4d)  [Current clock value]\n", m.CPU.PR(registers.ICR), m.CPU.PR(registers.ICR))
	m.Printf("      NICR = %08X (%4d)  [Reload  clock value]\n", m.CPU.PR(registers.NICR), m.CPU.PR(registers.NICR))
	m.Printf("      TODR = %08X (%s)\n", ticks, strings.ToUpper(todr))

	// If we are using the quantum clock mechanism, report on that now.
	if !m.Engine.HardwareClock() {
		current, initial := m.Engine.Quantum()
		m.Printf("  QUANTUM\n    INITIAL=%d\n    CURRENT=%d\n", initial, current)
	}

	return nil
}

// ShowFault reports the fault/exception event history and pending device/
// software interrupts.
func (m *Machine) ShowFault() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	history := m.Engine.FaultHistory()

	if len(history) == 0 {
		m.Printf("    No exceptions or interrupts have occurred yet\n")
	} else {
		m.Printf("FAULT/EXCEPTION EVENT HISTORY:\n")

		for _, fr := range history {
			m.Printf("%02X %-40s SEQ(%d)  PC(%08X) PSL(%08X) ",
				uint8(fr.Code), exceptionDesc(fr.Code), fr.Seq, fr.PC, uint32(fr.PSL))

			if len(fr.Args) > 0 {
				m.Printf(" ARGS(")

				for i, a := range fr.Args {
					if i > 0 {
						m.Printf(",")
					}

					m.Printf("%08X", a)
				}

				m.Printf(") ")
			}

			m.Printf("\n")
		}
	}

	pending, queued := m.Engine.PendingInterrupts()

	if pending != nil {
		m.Printf("    There is a pending interrupt code %s at ipl %02X\n", exceptionName(pending.Code), pending.IPL)
	}

	if len(queued) > 0 {
		m.Printf("Pending interrupts:\n")
		m.Printf("    IPL  Age Code\n")

		for _, q := range queued {
			m.Printf("    %02X   %4d  %02X %s\n", q.IPL, q.Age, uint8(q.Code), exceptionName(q.Code))
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
func (m *Machine) ShowTB() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	stcTries, stcHits := m.Mem.STCStats()

	m.Printf("Sequential Translation Cache\n")
	m.Printf("    Tries=%d    Hits=%d    Misses=%d    Ratio = %d%%\n",
		stcTries, stcHits, stcTries-stcHits, ratioPercent(stcTries, stcHits))

	enabled := enabledState
	if m.CPU.PR(registers.TBDR) != 0 {
		enabled = disabledState
	}

	m.Printf("\nTranslation buffer caching is %s\n", enabled)

	tries, hits, flushes, pflushes := m.Mem.TBStats()

	m.Printf("    Tries=%d    Hits=%d    Misses=%d    Ratio = %d%%\n",
		tries, hits, tries-hits, ratioPercent(tries, hits))
	m.Printf("    Flushes=%d   PFlushes=%d\n", flushes, pflushes)

	// Each entry's MODE= says what a hit on it lets the CPU's current
	// access mode do: "KERNEL WRITE" (reads and writes), "KERNEL READ"
	// (reads; a write walks the page table, to set the page's modify bit
	// or to fault), or "-NONE-" (every access walks).
	curMod := m.CPU.PSL().CurMod()

	for _, e := range m.Mem.TBSnapshot() {
		mode := "-NONE-"
		if access, ok := e.Permits(curMod); ok {
			mode = modeNames[curMod] + " " + tbModeNames[access]
		}

		m.Printf("    TB(%02X)  VA=%08X  PA=%08X  PROT=%-4s  MODE=%s\n",
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
func (m *Machine) ShowInstructions(modes, profile, unimplemented, all bool, opcodeExpr string) error {
	if err := m.requireInit(); err != nil {
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
		return m.showInstructionsGrid(table, unimplemented)
	}

	return m.showInstructionsDetail(table, opmatch)
}

func (m *Machine) showInstructionsGrid(table *cpu.Table, unimplemented bool) error {
	m.Printf("INSTRUCTION OPCODES:\n\n")

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
			m.Printf("%s\n", line.String())
			line.Reset()

			n = 0
		}
	}

	if n > 0 {
		m.Printf("%s\n", line.String())
	}

	m.Printf("\n%d Instructions.\n", count)

	return nil
}

func (m *Machine) showInstructionsDetail(table *cpu.Table, opmatch int32) error {
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

		m.Printf("    %s\n", line.String())

		count++
	}

	if opmatch < 0 && count > 1 {
		m.Printf("%d instructions\n", count)
	}

	if opmatch >= 0 && !found {
		m.Printf("No implemented instruction for opcode %02X\n", opmatch)
	}

	return nil
}

// debugShowEntry is one row of ShowDebug's display.
type debugShowEntry struct {
	flag registers.DebugFlags
	name string
	desc string
}

var debugShowEntries = []debugShowEntry{
	{registers.DebugCHM, "CHM", "Debug change-mode operations"},
	{registers.DebugExpand, "COMMAND", "Display command line expansions"},
	{registers.DebugDCL, "DCL", "Debug DCL parsing"},
	{registers.DebugNative, "DEBUG", "Invoke native debugger"},
	{registers.DebugDevices, "DEVICES", "Debug device operations"},
	{registers.DebugExceptions, "EXCEPTIONS", "Debug exception handling"},
	{registers.DebugFullDisasm, "FULLDISASM", "Display operand values on disasm"},
	{registers.DebugImages, "IMAGES", "Display image info on RUN command"},
	{registers.DebugInterrupts, "INTERRUPTS", "Debug interrupt handling"},
	{registers.DebugKeyboard, "KEYBOARD", "Debug console keyboard input"},
	{registers.DebugLibinit, "LIBINIT", "Invoke LIB$INITIALIZE for images"},
	{registers.DebugLogicals, "LOGICALS", "Debug logical name operations"},
	{registers.DebugProcess, "PROCESSES", "Debug process operations"},
	{registers.DebugRegisters, "REGISTERS", "Display changed registers on STEP"},
	{registers.DebugRMS, "RMS", "Debug RMS operations"},
	{registers.DebugServices, "SERVICES", "Debug P1 system service calls"},
	{registers.DebugSymbols, "SYMBOLS", "Debug symbol table handling"},
	{registers.DebugTB, "TB", "Debug translation buffer caching"},
	{registers.DebugUserHalt, "USERHALT", "HALT in user mode halts CPU"},
	{registers.DebugUserStep, "USERSTEP", "Step only affects USER mode"},
	{registers.DebugVM, "VM", "Debug virtual memory translations"},
}

// ShowDebug implements SHOW DEBUG a bit that's set prints its plain name,
// a clear bit prints "NO"+name, both followed by the flag's description.
func (m *Machine) ShowDebug() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	debug := m.CPU.Debug()

	m.Printf("DEBUG SETTINGS:\n")

	for _, e := range debugShowEntries {
		name := e.name
		if debug&e.flag == 0 {
			name = "NO" + name
		}

		m.Printf("    %-20s    %s\n", name, e.desc)
	}

	return nil
}

// ShowTrace implements SHOW TRACE (also reached via the DISASSEMBLY
// keyword).
func (m *Machine) ShowTrace() error {
	if err := m.requireInit(); err != nil {
		return err
	}

	state := disabledState
	if m.Trace {
		state = enabledState
	}

	m.Printf("    Execution trace disassembly is %s\n", state)

	if m.Trace {
		regState := disabledState
		if m.CPU.DebugEnabled(registers.DebugRegisters) {
			regState = enabledState
		}

		m.Printf("    Register tracking is %s\n", regState)
	}

	return nil
}

// ShowRegionsOf implements SHOW REGIONS for env's process: the
// high-water marks of its P0, P1, and S0 regions (corevms.Environment's
// RegionSize: $EXPREG and image activation's bookkeeping).
func (m *Machine) ShowRegionsOf(env *corevms.Environment) error {
	if err := m.requireInit(); err != nil {
		return err
	}

	names := [3]string{"P0", "P1", "S0"}

	m.Printf("REGION LIMITS:\n")

	for i, name := range names {
		m.Printf("\t%s   %08X\n", name, env.RegionSize[i])
	}

	m.Printf("\n")

	return nil
}
