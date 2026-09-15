package console

import (
	"strings"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// privRegNames matches vax.c's pr_names[] table (the subset with an
// architected name in internal/vax/registers.go's own PrivReg constants).
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

// SetSymbol implements the "SET <name>=<value>" family of console_set.c's
// SET command: a general register, a privileged register (by name), the
// whole PSL (SET PSL=value), or — if none of those match — a plain symbol
// definition. Matches console_set's own dispatch order (register, then
// privileged-register-or-PSL name, then symbol).
//
// The richer SET subcommands (SET PSL <field>=value, SET MODE, SET STEP,
// SET MKVALID/NOMK, SET PTE, SET ASM flags, SET [NO]EXPAND/SHARE, SET
// FAULT) are not implemented — see docs/PHASE-08.md's progress log; most
// are device/assembler state with no consumer yet in this port. SET DEBUG
// (SetDebug, below) is implemented — see docs/PHASE-17.md.
func (c *Console) SetSymbol(name string, value uint32) error {
	if err := c.requireInit(); err != nil {
		return err
	}
	name = strings.ToUpper(name)

	if r, ok := registerNames[name]; ok {
		c.CPU.SetGPR(r, value)
		return nil
	}
	if pr, ok := privRegNames[name]; ok {
		c.CPU.SetPR(pr, value)
		return nil
	}
	if name == "PSL" {
		c.CPU.SetPSL(vax.PSL(value))
		return nil
	}

	c.Symbols.Set(name, value, SymbolUser)
	return nil
}

// debugFlagNames matches console_set.c:598-621's SETDBG table (the 25 names
// SET DEBUG accepts) — full keyword spellings rather than the C source's
// 4-character abbreviation matching, matching this port's other hand-parsed
// SET sub-verbs (RADIX, BREAKPOINT). FUNCTIONS/UNIMP have no SETDBG entry in
// the C source either, so they're deliberately absent here too — see
// docs/PHASE-17.md.
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
}

// SetDebug implements SET DEBUG [name[,name...]] (alias SET DBG):
// console_set.c:581-639. Each name may be prefixed NO to clear that one bit
// instead of setting it; a bare SET DEBUG with no names sets the DEBUG
// (native-debugger) bit alone, matching console_set.c's isend(*p) case.
func (c *Console) SetDebug(names []string) error {
	if len(names) == 0 {
		c.CPU.SetDebug(c.CPU.Debug() | vax.DebugNative)
		return nil
	}

	flags := c.CPU.Debug()
	for _, name := range names {
		name = strings.ToUpper(strings.TrimSpace(name))
		clear := strings.HasPrefix(name, "NO")
		lookup := name
		if clear {
			lookup = name[2:]
		}

		flag, ok := debugFlagNames[lookup]
		if !ok {
			return vmserrors.New(vmserrors.CLI_BADDEBUGFLAG, name)
		}

		if clear {
			flags &^= flag
		} else {
			flags |= flag
		}
	}
	c.CPU.SetDebug(flags)
	return nil
}

// SetRadix implements SET RADIX <8|10|16>.
func (c *Console) SetRadix(radix int) error {
	if radix != 8 && radix != 10 && radix != 16 {
		return vmserrors.New(vmserrors.CLI_BADRADIX, radix)
	}
	c.Radix = radix
	return nil
}
