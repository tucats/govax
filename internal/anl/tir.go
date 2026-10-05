package anl

import (
	"fmt"

	"github.com/tucats/govax/internal/obj"
)

// commandColumn is the width a TIR command's heading is padded to before
// its stack depth.
const commandColumn = 56

// commandField is one line a TIR command's description shows after its
// heading.
type commandField func(a *objectAnalyzer, c obj.Command)

// The fields TIR commands show.
var (
	fieldSymbol = func(a *objectAnalyzer, c obj.Command) {
		a.line("\t\tsymbol: " + quote(c.Name))
	}
	fieldPsect = func(a *objectAnalyzer, c obj.Command) {
		a.line(fmt.Sprintf("\t\tpsect: %d", c.Psect))
	}
	fieldValue = func(a *objectAnalyzer, c obj.Command) {
		a.line("\t\tvalue: " + signedValue(c.StackedValue()))
	}
	fieldUnsigned = func(a *objectAnalyzer, c obj.Command) {
		a.line("\t\tunsigned value: " + unsignedValue(c.StackedValue()))
	}
	fieldEnvironment = func(a *objectAnalyzer, c obj.Command) {
		a.line(fmt.Sprintf("\t\tenvironment: %d", c.Env))
	}
	fieldLiteral = func(a *objectAnalyzer, c obj.Command) {
		a.line(fmt.Sprintf("\t\tliteral index: %d", c.Index))
	}
	fieldArgument = func(a *objectAnalyzer, c obj.Command) {
		a.line(fmt.Sprintf("\t\targument index: %d", c.Index))
		a.hexDump("\t\t", c.Data)
	}
	fieldBitField = func(a *objectAnalyzer, c obj.Command) {
		a.line(fmt.Sprintf("\t\tbit position: %d", c.Pos))
		a.line(fmt.Sprintf("\t\tfield size: %d", c.Size))
	}
	fieldBytes = func(a *objectAnalyzer, c obj.Command) {
		a.line(fmt.Sprintf("\t\tbyte count: %d", len(c.Data)))
		a.hexDump("\t\t", c.Data)
	}
)

// commandFields are the fields each TIR command shows, by its TIR$C_
// name without the prefix; a command not listed shows none. Real
// ANALYZE's output settles STA_GBL, STA_EPM, STA_SB, STA_SW, STA_LW,
// STA_PB, STA_PW, STA_PL, STA_UB, STA_UW, and CTL_AUGRB; the rest follow
// their pattern (unconfirmed).
var commandFields = map[string][]commandField{
	"STA_GBL":   {fieldSymbol},
	"STA_EPM":   {fieldSymbol},
	"OPR_REDEF": {fieldSymbol},
	"STA_SB":    {fieldValue},
	"STA_SW":    {fieldValue},
	"STA_LW":    {fieldValue},
	"CTL_AUGRB": {fieldValue},
	"STA_UB":    {fieldUnsigned},
	"STA_UW":    {fieldUnsigned},
	"STA_PB":    {fieldPsect, fieldValue},
	"STA_PW":    {fieldPsect, fieldValue},
	"STA_PL":    {fieldPsect, fieldValue},
	"STA_WPB":   {fieldPsect, fieldValue},
	"STA_WPW":   {fieldPsect, fieldValue},
	"STA_WPL":   {fieldPsect, fieldValue},
	"STA_LSY":   {fieldEnvironment, fieldSymbol},
	"STA_LEPM":  {fieldEnvironment, fieldSymbol},
	"STA_LIT":   {fieldLiteral},
	"OPR_DFLIT": {fieldLiteral},
	"STA_CKARG": {fieldSymbol, fieldArgument},
	"STO_VPS":   {fieldBitField},
	"OPR_INSV":  {fieldBitField},
	"STO_RIVB":  {fieldBytes},
}

// commandNames are the TIR commands ANALYZE names differently from VMS
// 7.3's objfmt.sdl, by their SDL names: ANALYZE calls TIR$C_STO_L (22)
// TIR$C_STO_LW.
var commandNames = map[string]string{
	"STO_L": "STO_LW",
}

// commandName is a TIR command's name as ANALYZE shows it, without the
// TIR$C_ prefix.
func commandName(op obj.Op) string {
	if name, ok := commandNames[op.String()]; ok {
		return name
	}

	return op.String()
}

// command describes one TIR command, numbered k within its record, and
// follows its effect on the linker's stack whether it's shown or not.
func (a *objectAnalyzer) command(show bool, k int, c obj.Command) {
	if c.Op == obj.OpStoreImmediate {
		if show {
			n := len(c.Data)
			a.line(fmt.Sprintf("\t%d)  Store Immediate, %d byte%s:", k, n, plural(n)))
			a.hexDump("\t\t", c.Data)
		}

		return
	}

	pop, push := c.Op.StackEffect()
	a.depth += push - pop

	if !show {
		return
	}

	heading := fmt.Sprintf("\t%d)  TIR$C_%s (%d, %%X'%02X')", k, commandName(c.Op), int(c.Op), int(c.Op))
	if pop != push {
		heading = fmt.Sprintf("%-*sstack depth: %d", commandColumn+1, heading, a.depth)
	}

	a.line(heading)

	for _, f := range commandFields[c.Op.String()] {
		f(a, c)
	}
}

// quote puts a name or text in double quotes as it is, without escaping
// anything (ANALYZE shows the bytes).
func quote(s string) string {
	return `"` + s + `"`
}
