package debugger

import (
	"strings"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file is the debugger's EXAMINE command, so far in its
// /INSTRUCTION form (docs/PHASE-42.md, subtask 9; the data forms are
// subtask 10's), and the display modes and radix it follows.
//
// The VMS debugger's EXAMINE shows what is at a location. With
// /INSTRUCTION that is the machine instruction there, laid out as
//
//	DBGDIS\START\%LINE 47:  MOVL     L^DBGDIS\COUNT,R0
//
// the location (named from the program's debug symbols), a colon, and the
// instruction with its operands named. The same layout is what govax's
// console DISASSEMBLE gives, so this command asks the console to print it.

// radixNames maps each radix keyword SET RADIX takes to its value. The
// VMS debugger accepts the whole word or any abbreviation of it.
var radixNames = []struct {
	name  string
	radix int
}{
	{"DECIMAL", 10},
	{"HEXADECIMAL", 16},
	{"OCTAL", 8},
	{"BINARY", 2},
}

// parseRadix turns a SET RADIX keyword (DECIMAL, DEC, HEXADECIMAL, HEX, OCTAL,
// BINARY, or one of their abbreviations; or 2, 8, 10, 16 as govax's console
// takes them) into a radix, or 0 if it isn't one.
func parseRadix(word string) int {
	word = strings.ToUpper(strings.TrimSpace(word))

	switch word {
	case "2":
		return 2
	case "8":
		return 8
	case "10":
		return 10
	case "16":
		return 16
	}

	if word == "" {
		return 0
	}

	for _, r := range radixNames {
		if strings.HasPrefix(r.name, word) {
			return r.radix
		}
	}

	return 0
}

// evalWhole evaluates text, one whole address or value expression, in the
// debugger's input radix. A number with no prefix (^D, ^X, ^O, ^B) is read
// in it, so "SET RADIX DECIMAL" makes "EXAMINE 100" decimal.
func (d *Debugger) evalWhole(text string) (uint32, error) {
	return d.Console.EvalWholeIn(d.inputRadix, text)
}

// symbolRadix is the radix the offsets in a symbolic name are written in
// (GLIMIT+589 in decimal, GLIMIT+24D in hexadecimal): the debugger's
// output radix when it is decimal, and hexadecimal otherwise.
func (d *Debugger) symbolRadix() int {
	if d.outputRadix == 10 {
		return 10
	}

	return 16
}

// bindExamine binds EXAMINE, and SET MODE, SET RADIX, and CANCEL RADIX,
// which change how it shows things.
func (d *Dispatcher) bindExamine() {
	g := d.Grammar

	g.Bind("EXAMINE", func(id int64, r *dcl.Result) error { return d.examine(r) })
	g.Bind("SET_MODE", func(id int64, r *dcl.Result) error { return d.Debugger.setMode(r.String("WORDS")) })
	g.Bind("SET_RADIX", func(id int64, r *dcl.Result) error { return d.setRadix(r) })
	g.Bind("CANCEL_RADIX", func(id int64, r *dcl.Result) error {
		d.Debugger.inputRadix, d.Debugger.outputRadix = 16, 16

		return nil
	})
}

// examine runs EXAMINE[/qualifiers] [location[,location...]]. Only
// /INSTRUCTION is implemented so far; asking for the data forms is an
// error until subtask 10.
func (d *Dispatcher) examine(r *dcl.Result) error {
	dbg := d.Debugger

	if err := dbg.Console.RequireInit(); err != nil {
		return err
	}

	// /OPERANDS describes an instruction's operands, so it implies
	// /INSTRUCTION (dbgdis.dlg: EXAMINE/OPERANDS .PC).
	operandsGiven := r.Present("OPERANDS") && !r.Defaulted("OPERANDS")

	if !r.Present("INSTRUCTION") && !operandsGiven {
		return vmserrors.New(vmserrors.DBG_NOTAVAILABLE)
	}

	opts := console.DisassembleOptions{
		Symbolic:  dbg.symbolic,
		Constants: r.Present("CONSTANTS"),
		Shareable: r.Present("SHAREABLE"),
		Operands:  dbg.operands,
		Radix:     dbg.symbolRadix(),
	}

	// /OPERANDS[=FULL] for this command, /NOOPERANDS to turn the session's
	// mode off for it.
	// (A qualifier with a default is always "present"; Defaulted says it was
	// left out.)
	if operandsGiven {
		switch {
		case r.Negated("OPERANDS"):
			opts.Operands = console.OperandsOff
		case strings.HasPrefix("FULL", strings.ToUpper(r.String("OPERANDS"))) && r.String("OPERANDS") != "":
			opts.Operands = console.OperandsFull
		default:
			opts.Operands = console.OperandsBrief
		}
	}

	locations := strings.TrimSpace(r.String("LOCATION"))
	if locations == "" {
		// With no location, the instruction at the current location.
		return dbg.Console.DisassembleWith(dbg.Console.DepositAddr, dbg.Console.DepositAddr, opts)
	}

	items, err := splitList(locations)
	if err != nil {
		return err
	}

	for _, item := range items {
		start, end, isRange := item, "", false

		if lo, hi, ok := splitRange(item); ok {
			start, end, isRange = lo, hi, true
		}

		first, err := dbg.evalWhole(start)
		if err != nil {
			return err
		}

		last := first

		if isRange {
			if last, err = dbg.evalWhole(end); err != nil {
				return err
			}
		}

		// A range's start typed as a line (%LINE 85) names the first
		// instruction by that line, as the debugger does.
		opts.StartLine = strings.Contains(strings.ToUpper(start), "%LINE")

		if err := dbg.Console.DisassembleWith(first, last, opts); err != nil {
			return err
		}
	}

	return nil
}

// splitList splits a comma-separated list of locations at the commas that
// aren't inside parentheses or quotes. A list of one is returned as itself.
func splitList(text string) ([]string, error) {
	var (
		items []string
		depth int
		quote bool
		start int
	)

	for i := 0; i < len(text); i++ {
		switch ch := text[i]; {
		case ch == '"':
			quote = !quote
		case quote:
		case ch == '(' || ch == '[':
			depth++
		case ch == ')' || ch == ']':
			depth--
		case ch == ',' && depth == 0:
			items = append(items, strings.TrimSpace(text[start:i]))
			start = i + 1
		}
	}

	items = append(items, strings.TrimSpace(text[start:]))

	for _, item := range items {
		if item == "" {
			return nil, vmserrors.New(vmserrors.CLI_NEEDEXPR)
		}
	}

	return items, nil
}

// splitRange splits "start:end" at its colon, one that isn't inside
// parentheses or quotes. ok is false for an item that isn't a range.
func splitRange(item string) (start, end string, ok bool) {
	depth := 0
	quote := false

	for i := 0; i < len(item); i++ {
		switch ch := item[i]; {
		case ch == '"':
			quote = !quote
		case quote:
		case ch == '(' || ch == '[':
			depth++
		case ch == ')' || ch == ']':
			depth--
		case ch == ':' && depth == 0:
			return strings.TrimSpace(item[:i]), strings.TrimSpace(item[i+1:]), true
		}
	}

	return "", "", false
}

// setMode runs SET MODE keyword[,keyword...] for the keywords the debugger
// has so far: [NO]SYMBOLIC, which chooses whether EXAMINE/INSTRUCTION names
// addresses from the program's debug symbols (the default), and
// [NO]OPERANDS[=FULL], which adds a line for each operand. The other modes
// (and govax's access mode) come with subtask 11.
func (d *Debugger) setMode(words string) error {
	for _, word := range strings.Split(words, ",") {
		word = strings.ToUpper(strings.TrimSpace(word))
		name, value, _ := strings.Cut(word, "=")

		switch {
		case isKeyword(name, "SYMBOLIC", 3) && value == "":
			d.symbolic = true
		case isKeyword(name, "NOSYMBOLIC", 5) && value == "":
			d.symbolic = false
		case isKeyword(name, "OPERANDS", 3):
			switch {
			case value == "":
				d.operands = console.OperandsBrief
			case isKeyword(value, "FULL", 1):
				d.operands = console.OperandsFull
			case isKeyword(value, "BRIEF", 1):
				d.operands = console.OperandsBrief
			default:
				return vmserrors.New(vmserrors.DBG_SYNTAX, word)
			}
		case isKeyword(name, "NOOPERANDS", 5) && value == "":
			d.operands = console.OperandsOff
		default:
			return vmserrors.New(vmserrors.DBG_SYNTAX, word)
		}
	}

	return nil
}

// isKeyword reports whether word is keyword or an abbreviation of it at
// least min letters long, as VMS commands allow.
func isKeyword(word, keyword string, min int) bool {
	return len(word) >= min && strings.HasPrefix(keyword, word)
}

// setRadix runs SET RADIX [/INPUT|/OUTPUT] radix. With no qualifier it sets
// both the radix numbers are typed in and the one they are shown in. (govax
// shows only the offsets in symbolic names in the output radix so far.)
func (d *Dispatcher) setRadix(r *dcl.Result) error {
	radix := parseRadix(r.String("RADIX"))
	if radix == 0 {
		return vmserrors.New(vmserrors.DBG_SYNTAX, strings.ToUpper(strings.TrimSpace(r.String("RADIX"))))
	}

	both := !r.Present("INPUT") && !r.Present("OUTPUT")

	if both || r.Present("INPUT") {
		d.Debugger.inputRadix = radix
	}

	if both || r.Present("OUTPUT") {
		d.Debugger.outputRadix = radix
	}

	return nil
}
