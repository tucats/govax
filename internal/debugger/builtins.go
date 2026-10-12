package debugger

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/vmserrors"
)

// The debugger's built-in symbols (docs/PHASE-42.md, subtask 20; the VMS
// 5.5 Debugger Manual's appendix D). They stand for things the debugger
// knows rather than names the program defines:
//
//	%CURLOC  .   the current location: the one EXAMINE, DEPOSIT, or
//	             EVALUATE/ADDRESS last referred to
//	%NEXTLOC     the location after it (EXAMINE with no location is the
//	             same: the manual's "Return key")
//	%PREVLOC ^   the location before it
//	%CURVAL  \   the value EXAMINE or EVALUATE last showed, or DEPOSIT
//	             last stored
//	%DECWINDOWS  0: govax has only the command interface
//	%EXC_NUMBER  the current exception's condition value, and
//	%EXC_SEVERITY its severity (the low three bits); %EXC_NAME and
//	%EXC_FACILITY are its message's name and facility, as strings, which
//	EVALUATE shows and a WHEN clause may compare with EQL or NEQ
//
// The "current exception" is the one an exception break (SET
// BREAK/EXCEPTION, or an unhandled condition) last stopped the program
// for, until the program runs again.
//
// The console's expression evaluator does the rest of appendix D itself:
// %NAME 'string', the radix operators %BIN, %DEC, %HEX, and %OCT, and a
// leading backslash for a global name (\X). The registers' %R0 forms were
// there already. %PARCNT needs DECLARE, which govax hasn't; the scope
// entries (%CURRENT_SCOPE_ENTRY ...) come with SET SCOPE, and %ADDR, %REF,
// %VAL, and %DESCR with CALL's argument forms.

// currentValue is %CURVAL: the last value shown or stored, and whether
// there has been one.
type currentValue struct {
	v   uint32
	set bool
}

// setCurrentValue records data, the bytes of a value just shown or
// stored, as %CURVAL: its first longword, least significant byte first.
func (d *Debugger) setCurrentValue(data []byte) {
	var v uint32

	for i := min(len(data), 4) - 1; i >= 0; i-- {
		v = v<<8 | uint32(data[i])
	}

	d.curval = currentValue{v: v, set: true}
}

// currentLocation is %CURLOC's address and the size of what is there: the
// last location examined or deposited, or, before any, the console's
// current address.
func (d *Debugger) currentLocation() examineState {
	if d.examined.set {
		return d.examined
	}

	return examineState{addr: d.Console.DepositAddr, size: defaultSize, set: true}
}

// builtinSymbol is the console evaluator's hook for the built-in symbols
// (console.Console.DebugBuiltins). ok is false for a name that isn't one.
func (d *Debugger) builtinSymbol(name string) (uint32, bool, error) {
	cur := d.currentLocation()

	switch name {
	case ".", "%CURLOC":
		return cur.addr, true, nil

	case "%NEXTLOC":
		return cur.addr + cur.size, true, nil

	case "^", "%PREVLOC":
		return cur.addr - cur.size, true, nil

	case "\\", "%CURVAL":
		if !d.curval.set {
			return 0, true, vmserrors.New(vmserrors.DBG_NOCURVAL)
		}

		return d.curval.v, true, nil

	case "%DECWINDOWS":
		return 0, true, nil

	case "%EXC_NUMBER", "%EXC_SEVERITY", "%EXC_NAME", "%EXC_FACILITY":
		if d.exception == nil {
			return 0, true, vmserrors.New(vmserrors.DBG_NOEXC)
		}

		switch name {
		case "%EXC_NUMBER":
			return d.exception.Condition, true, nil
		case "%EXC_SEVERITY":
			return d.exception.Condition & 7, true, nil
		}

		// A name or facility is a string, which no number stands for.
		return 0, true, vmserrors.New(vmserrors.DBG_NOTNUMERIC, name)
	}

	return 0, false, nil
}

// exceptionString is the value of %EXC_NAME or %EXC_FACILITY, the
// built-in symbols whose values are strings: the current exception's
// message name (ENDOFFILE) or facility (SYSTEM), from its message line,
// "%SYSTEM-W-ENDOFFILE, end of file". ok is false for any other text.
func (d *Debugger) exceptionString(text string) (value string, ok bool, err error) {
	name := strings.ToUpper(strings.TrimSpace(text))
	if name != "%EXC_NAME" && name != "%EXC_FACILITY" {
		return "", false, nil
	}

	if d.exception == nil {
		return "", true, vmserrors.New(vmserrors.DBG_NOEXC)
	}

	facility, ident := messageParts(d.Console.StatusText(d.exception.Condition))
	if name == "%EXC_NAME" {
		return ident, true, nil
	}

	return facility, true, nil
}

// messageParts splits a message line, "%FACILITY-S-IDENT, text", into
// its facility and its identifier.
func messageParts(line string) (facility, ident string) {
	head, _, _ := strings.Cut(strings.TrimPrefix(line, "%"), ",")
	parts := strings.Split(head, "-")

	if len(parts) < 3 {
		return "", head
	}

	return parts[0], parts[len(parts)-1]
}

// stringComparison evaluates a WHEN condition's comparison of a string
// built-in symbol with a quoted string, %EXC_NAME EQL "ENDOFFILE" (the
// manual's example of a condition on the current exception). ok is false
// when the comparison isn't one.
func (d *Debugger) stringComparison(op, left, right string) (result, ok bool, err error) {
	if op != "EQL" && op != "NEQ" {
		return false, false, nil
	}

	value, isString, err := d.exceptionString(left)
	other := right

	if !isString && err == nil {
		value, isString, err = d.exceptionString(right)
		other = left
	}

	if err != nil {
		return false, true, err
	}

	if !isString || !quoted(other) {
		return false, false, nil
	}

	equal := strings.EqualFold(value, unquote(other))

	return equal == (op == "EQL"), true, nil
}

// setException records the exception the program has just stopped for,
// as the current exception the %EXC_ symbols describe.
func (d *Debugger) setException(u *machine.UnhandledException) {
	copied := *u
	d.exception = &copied
}

// quotedString is a string value as EVALUATE shows one: in quotes.
func quotedString(s string) string { return fmt.Sprintf("%q", s) }
