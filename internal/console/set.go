package console

import (
	"strings"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmserrors"
)

// privRegNames table (the subset with an architected name in 
// internal/vax/registers.go's own PrivReg constants).
var privRegNames = map[string]vax.PrivReg{
	"KSP": vax.KSP, "ESP": vax.ESP, "SSP": vax.SSP, "USP": vax.USP, "ISP": vax.ISP,
	"P0BR": vax.P0BR, "P0LR": vax.P0LR, "P1BR": vax.P1BR, "P1LR": vax.P1LR,
	"SBR": vax.SBR, "SLR": vax.SLR, "PCBB": vax.PCBB, "SCBB": vax.SCBB,
	"IPL": vax.IPL, "ASTLVL": vax.ASTLVL, "SIRR": vax.SIRR, "SISR": vax.SISR,
	"ICCS": vax.ICCS, "NICR": vax.NICR, "ICR": vax.ICR, "TODR": vax.TODR,
	"RXCS": vax.RXCS, "RXDB": vax.RXDB, "TXCS": vax.TXCS, "TXDB": vax.TXDB,
	"TBDR": vax.TBDR, "MAPEN": vax.MAPEN, "TBIA": vax.TBIA, "TBIS": vax.TBIS,
	"PMR": vax.PMR, "SID": vax.SID, "TBCHK": vax.TBCHK,
}

// SetSymbol implements the "SET <name>=<value>" family of the SET command: 
// a general register, a privileged register (by name), the whole PSL 
// (SET PSL=value), or — if none of those match — a plain symbol
// definition. Matches console_set's own dispatch order (register, then
// privileged-register-or-PSL name, then symbol). Equivalent to
// SetSymbolQualified with every qualifier false — see that function for the
// /PERMANENT, /ENTRY, /LABEL qualified form.
//
// The remaining unimplemented SET subcommands (SET MKVALID/NOMK, SET
// ASSEMBLER flags, SET WATCH, SET FAULT/HISTORY, SET BREAK/FAULT) each need
// state this port doesn't model yet, or a design decision flagged in
// docs/PHASE-16.md — see that document's own sub-phase 3 for the current
// status of each.
func (c *Console) SetSymbol(name string, value uint32) error {
	return c.SetSymbolQualified(name, value, false, false, false)
}

// SetSymbolQualified is SetSymbol with /PERMANENT,
// /ENTRY, /LABEL qualifier scan applied (its own qualifier-token loop ahead
// of the NAME=value parse — see setcommand.go's SET_SYMBOL) — see
// symbols.go's Symbol.Permanent/IsEntry/IsLabel.
func (c *Console) SetSymbolQualified(name string, value uint32, permanent, entry, label bool) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	name = strings.ToUpper(name)

	if r, ok := registerNames[name]; ok {
		c.CPU.SetGPR(r, value)

		return nil
	}

	if pr, ok := privRegNames[name]; ok {
		// TODR isn't stored in the register file; the engine computes it
		// from its clock, and a write sets where it counts from.
		if pr == vax.TODR {
			c.Engine.SetTODR(value)

			return nil
		}

		c.CPU.SetPR(pr, value)

		return nil
	}

	if name == "PSL" {
		oldMode := c.CPU.PSL().CurMod()
		c.CPU.SetPSL(vax.PSL(value))

		// "SET PSL=value" case: a bare
		// register overwrite calls read_psl_bits() right after, which
		// invalidates cached TB protection state if CurMod changed. See
		// docs/PHASE-21.md.
		if newMode := c.CPU.PSL().CurMod(); newMode != oldMode {
			c.Mem.InvalidateProtection()
		}

		return nil
	}

	c.Symbols.SetQualified(name, value, permanent, entry, label)

	return nil
}

// pslFieldNames are the PSL field name. CUR_MOD (handled
// separately in SetPSLField, below) also accepts the bare "MODE" spelling
// as a convenience alias; unlike the C source, setting CUR_MOD here does
// not check for/deliver a pending AST itself. govax delivers ASTs from the
// RTL, which checks the current mode at every instruction boundary
// (docs/PHASE-26.md subtask 15), so the next step sees the new mode.
var pslFieldNames = map[string]func(p *vax.PSL, v uint32) error{
	"CM":  func(p *vax.PSL, v uint32) error { return setPSLBit(p.SetCM, v) },
	"TP":  func(p *vax.PSL, v uint32) error { return setPSLBit(p.SetTP, v) },
	"FPD": func(p *vax.PSL, v uint32) error { return setPSLBit(p.SetFPD, v) },
	"IS":  func(p *vax.PSL, v uint32) error { return setPSLBit(p.SetIS, v) },
	"DV":  func(p *vax.PSL, v uint32) error { return setPSLBit(p.SetDV, v) },
	"FU":  func(p *vax.PSL, v uint32) error { return setPSLBit(p.SetFU, v) },
	"IV":  func(p *vax.PSL, v uint32) error { return setPSLBit(p.SetIV, v) },
	"T":   func(p *vax.PSL, v uint32) error { return setPSLBit(p.SetT, v) },
	"N":   func(p *vax.PSL, v uint32) error { return setPSLBit(p.SetN, v) },
	"Z":   func(p *vax.PSL, v uint32) error { return setPSLBit(p.SetZ, v) },
	"V":   func(p *vax.PSL, v uint32) error { return setPSLBit(p.SetV, v) },
	"C":   func(p *vax.PSL, v uint32) error { return setPSLBit(p.SetC, v) },
	"IPL": func(p *vax.PSL, v uint32) error { return setPSLRange(p.SetIPL, v, 31) },
	"PRV_MOD": func(p *vax.PSL, v uint32) error {
		return setPSLRange(func(m uint32) { p.SetPrvMod(vax.AccessMode(m)) }, v, 3)
	},
}

func setPSLBit(set func(bool), v uint32) error {
	if v > 1 {
		return vmserrors.New(vmserrors.CLI_INVSETPSL, v)
	}

	set(v != 0)

	return nil
}

func setPSLRange(set func(uint32), v, maxValue uint32) error {
	if v > maxValue {
		return vmserrors.New(vmserrors.CLI_INVSETPSL, v)
	}

	set(v)

	return nil
}

// SetPSLField implements one "<field>=<value>" clause of SET PSL (setcommand.go
// loops over a comma-separated list of these. 
// 
// CM/TP/FPD/IS/DV/FU/IV/T/N/Z/V/C are boolean bits (0 or 1);
// IPL is 0-31; PRV_MOD is 0-3; CUR_MOD/MODE switches the active mode/stack
// via SetMode.
func (c *Console) SetPSLField(field string, value uint32) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	field = strings.ToUpper(field)
	if field == "CUR_MOD" || field == "MODE" {
		return c.setModeStackByValue(value)
	}

	setter, ok := pslFieldNames[field]
	if !ok {
		return vmserrors.New(vmserrors.CLI_INVSETPSL, field)
	}

	psl := c.CPU.PSL()
	if err := setter(&psl, value); err != nil {
		return err
	}

	c.CPU.SetPSL(psl)

	return nil
}

// debugFlagNames (the 25 names SET DEBUG accepts) — see docs/PHASE-17.md.
var debugFlagNames = map[string]vax.DebugFlags{
	"RMS":        vax.DebugRMS,
	"FULLDISASM": vax.DebugFullDisasm,
	"DEBUG":      vax.DebugNative,
	"KEYBOARD":   vax.DebugKeyboard,
	"DEVICES":    vax.DebugDevices,
	"VM":         vax.DebugVM,
	"TB":         vax.DebugTB,
	"MEMORY":     vax.DebugMemory,
	"SYMBOLS":    vax.DebugSymbols,
	"INTERRUPTS": vax.DebugInterrupts,
	"EXCEPTIONS": vax.DebugExceptions,
	"P1":         vax.DebugP1,
	"P2":         vax.DebugP2,
	"P3":         vax.DebugP3,
	"P4":         vax.DebugP4,
	"REGISTERS":  vax.DebugRegisters,
	"IMAGES":     vax.DebugImages,
	"USERHALT":   vax.DebugUserHalt,
	"SERVICES":   vax.DebugServices,
	"DCL":        vax.DebugDCL,
	"CHM":        vax.DebugCHM,
	"COMMAND":    vax.DebugExpand,
	"LOGICALS":   vax.DebugLogicals,
	"LIBINIT":    vax.DebugLibinit,
	"PROCESS":    vax.DebugProcess,
	"PROCESSES":  vax.DebugProcess,
	"USERSTEP":   vax.DebugUserStep,
}

// SetDebug implements SET DEBUG [name[,name...]] (alias SET DBG).
// Each name may be prefixed NO to clear that one bit
// instead of setting it; a bare SET DEBUG with no names sets the DEBUG
// (native-debugger) bit alone.
func (c *Console) SetDebug(names []string) error {
	if len(names) == 0 {
		c.CPU.SetDebug(c.CPU.Debug() | vax.DebugNative)

		return nil
	}

	flags := c.CPU.Debug()

	for _, name := range names {
		name = strings.ToUpper(strings.TrimSpace(name))
		clearFlag := strings.HasPrefix(name, "NO")
		lookup := name

		if clearFlag {
			lookup = name[2:]
		}

		flag, ok := debugFlagNames[lookup]
		if !ok {
			return vmserrors.New(vmserrors.CLI_BADDEBUGFLAG, name)
		}

		if clearFlag {
			flags &^= flag
		} else {
			flags |= flag
		}
	}

	c.CPU.SetDebug(flags)

	return nil
}

// setModeNames SET MODE sub-switch keywords, spelled
// out in full (see this file's own full-keyword convention). "INTERRUPT"
// has no AccessMode of its own (set_mode_stack's mode value 4 means "switch
// to the interrupt stack", not a fourth access mode) — SetMode handles it
// as a separate case rather than adding an entry here.
var setModeNames = map[string]vax.AccessMode{
	"KERNEL": vax.Kernel, "EXEC": vax.Executive, "EXECUTIVE": vax.Executive,
	"SUPER": vax.Supervisor, "SUPERVISOR": vax.Supervisor, "USER": vax.User,
}

// SetMode implements SET MODE <KERNEL|EXEC|SUPER|USER|INTERRUPT>.
// Like SetPSLField's CUR_MOD case, this does not check for/deliver a
// pending AST (vax.pslw.cur_mod >= vax.ASTLVL) itself: the RTL's AST 
// delivery sees the new mode at the next instruction boundary (see 
// pslFieldNames' doc comment).
func (c *Console) SetMode(name string) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	name = strings.ToUpper(strings.TrimSpace(name))
	if name == "INTERRUPT" {
		c.Engine.SetModeStack(vax.Kernel, true)

		return nil
	}

	mode, ok := setModeNames[name]
	if !ok {
		return vmserrors.New(vmserrors.CLI_BADMODE, name)
	}

	c.Engine.SetModeStack(mode, false)

	return nil
}

// setModeStackByValue implements SET PSL CUR_MOD=<n>, which — unlike SET
// MODE's keyword form — takes CUR_MOD's raw 0-3 numeric encoding (matching
// AccessMode's own iota values) with no "INTERRUPT" case (SET PSL has no
// numeric encoding for the interrupt stack.
func (c *Console) setModeStackByValue(value uint32) error {
	if value > uint32(vax.User) {
		return vmserrors.New(vmserrors.CLI_INVSETPSL, value)
	}

	c.Engine.SetModeStack(vax.AccessMode(value), false)

	return nil
}

// SetTrace implements SET TRACE/SET DISASSEMBLY (enable) and SET NOTRACE/
// SET NODISASSEMBLE (disable).
func (c *Console) SetTrace(on bool) {
	c.Trace = on
}

// SetRadix implements SET RADIX <8|10|16>.
func (c *Console) SetRadix(radix int) error {
	if radix != 8 && radix != 10 && radix != 16 {
		return vmserrors.New(vmserrors.CLI_BADRADIX, radix)
	}

	c.Radix = radix

	return nil
}

// SetVM implements SET VM/SET MAPEN (on) and SET NOVM/SET NOMAPEN (off).
// Sets/clears the MAPEN privileged register directly (the same register
// the generic "SET MAPEN=<value>" form already reaches via 
// SetSymbol/privRegNames, this being only the bare on/off
// keyword spelling acepted). Requires kernel mode.
func (c *Console) SetVM(on bool) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if err := c.requireKernelMode(); err != nil {
		return err
	}

	v := uint32(0)
	if on {
		v = 1
	}

	c.CPU.SetPR(vax.MAPEN, v)

	return nil
}

// SetBase implements SET BASE <addr>, sets the EXAMINE/DEPOSIT cursor
// directly (Console.DepositAddr), the same cursor SHOW BASE reports 
// (see docs/PHASE-16.md sub-phase 1d's ShowBase fix) and EXAMINE/DEPOSIT
// themselves advance after every access.
func (c *Console) SetBase(addr uint32) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.DepositAddr = addr

	return nil
}

// SetVerbose implements SET VERBOSE command, sets
// CONSOLE_VERBOSE; unlike SET VERIFY it doesn't touch Verify).
func (c *Console) SetVerbose() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Verbose = true

	return nil
}

// SetVerify implements SET VERIFY (sets vax.console.verify; 
// unlike SET VERBOSE it doesn't touch Verbose).
func (c *Console) SetVerify() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Verify = true

	return nil
}

// SetNoVerbose implements SET NOVERBOSE - clears *both* Verbose and Verify.
func (c *Console) SetNoVerbose() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Verbose = false
	c.Verify = false

	return nil
}

// SetQuantum implements SET QUANTUM <n>, resets Engine's interrupt-admission
// quantum counter (both its initial reload value and its current countdown).
func (c *Console) SetQuantum(n int) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	_, initial := c.Engine.Quantum()

	switch {
	case initial == 0 && n > 0:
		c.Printf("%%VAX-I-INTERRUPTS, interrupt delivery resumed (quantum>0)\n")

	case initial > 0 && n == 0:
		c.Printf("%%VAX-I-NOINTERRUPTS, interrupt delivery suspended (quantum=0)\n")
	}

	c.Engine.SetQuantum(n)

	return nil
}

// SetUIQuantum implements SET UIQUANTUM <n>.
func (c *Console) SetUIQuantum(n int) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Printf("UI time slicing is not modeled by this port (no cooperative host-UI polling loop); value accepted but has no effect.\n")

	return nil
}

// SetFaultHistory implements SET FAULT/SET HIST <n>.
func (c *Console) SetFaultHistory(n int) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Engine.SetFaultHistorySize(n)

	return nil
}

// pteFieldNames.
var pteFieldNames = map[string]func(pte *vm.PTE, v uint32){
	"V":     func(pte *vm.PTE, v uint32) { pte.SetValid(v != 0) },
	"VALID": func(pte *vm.PTE, v uint32) { pte.SetValid(v != 0) },
	"PROT":  func(pte *vm.PTE, v uint32) { pte.SetProtection(vm.Protection(v)) },
	"M":     func(pte *vm.PTE, v uint32) { pte.SetModified(v != 0) },
	"MODIFY": func(pte *vm.PTE, v uint32) {
		pte.SetModified(v != 0)
	},
	"OWN":   func(pte *vm.PTE, v uint32) { pte.SetOwner(uint8(v)) },
	"OWNER": func(pte *vm.PTE, v uint32) { pte.SetOwner(uint8(v)) },
	"S":     func(pte *vm.PTE, v uint32) { pte.SetSoftware(uint8(v)) },
	"SOFT":  func(pte *vm.PTE, v uint32) { pte.SetSoftware(uint8(v)) },
	"PFN":   func(pte *vm.PTE, v uint32) { pte.SetPFN(v) },
	"PAGE":  func(pte *vm.PTE, v uint32) { pte.SetPFN(v) },
}

// SetPTE implements one "<addr> <field>=<value>[,<field>=<value>...]" SET
// PTE/SET PAGE command (setPTECommand parses the field=value list and calls this
// once per field, the write-side counterpart to ShowPage. Reports
// "Cannot SET PAGE when virtual memory is disabled" (not an error, matching
// setpte's own VAX_OK-with-printf behavior) rather than failing if MAPEN is
// off.
func (c *Console) SetPTE(addr uint32, field string, value uint32) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if c.CPU.PR(vax.MAPEN) == 0 {
		c.Printf("Cannot SET PAGE when virtual memory is disabled.\n")

		return nil
	}

	setter, ok := pteFieldNames[strings.ToUpper(field)]
	if !ok {
		return vmserrors.New(vmserrors.CLI_BADPTEFIELD, field)
	}

	_, _, pte, err := c.Mem.LookupPTE(c.CPU, addr)
	if err != nil {
		c.Printf("ACCVIO, page table length violation\n")

		return nil
	}

	setter(&pte, value)

	return c.Mem.StorePTE(c.CPU, addr, pte)
}
