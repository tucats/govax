package console

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/vax"
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
// SET MKVALID/NOMK, SET PTE, SET DEBUG/ASM flags, SET [NO]EXPAND/SHARE, SET
// FAULT) are not implemented — see docs/PHASE-08.md's progress log; most
// are device/assembler/debug-tracing state with no consumer yet in this
// port.
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

// SetRadix implements SET RADIX <8|10|16>.
func (c *Console) SetRadix(radix int) error {
	if radix != 8 && radix != 10 && radix != 16 {
		return fmt.Errorf("console: invalid radix %d (must be 8, 10, or 16)", radix)
	}
	c.Radix = radix
	return nil
}
