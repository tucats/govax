package asm

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// This file holds the assembler's source stack: where the lines being
// assembled come from.
//
// Assembly starts with one source, the program's own text. Three things
// push another source on top of it, whose lines are assembled before the
// assembler goes back to the line after the one that pushed it:
//
//   - .INCLUDE pushes the text of another file (sourceInclude).
//   - A macro call pushes the macro's expansion: its definition's lines
//     with the call's arguments substituted in (sourceMacro, see
//     macros.go).
//   - A repeat block pushes its repeated lines (docs/PHASE-28.md,
//     subtask 3).
//
// Each source is a sourceFrame on Assembler.sources, innermost last, and
// runSource is the loop that assembles one source's lines. The frames are
// what let an error say where it came from: an error on line 3 of a macro
// that line 20 of the program called is reported as
//
//	line 20: in expansion of macro NAME, line 3: <the error>
//
// # A note for a reader new to Go
//
// runSource calls assembleStatement for each line, and a statement (a
// macro call, a .INCLUDE) can call runSource again for the source it
// pushes: the Go call stack mirrors the source stack. The "defer"
// statements in runSource run when it returns, however it returns, which
// is how a frame is always popped and the line number always restored.

// sourceKind says where a source's lines came from.
type sourceKind int

const (
	// sourceFile is the program's own text (Assemble's argument).
	sourceFile sourceKind = iota
	// sourceInclude is a file .INCLUDE named.
	sourceInclude
	// sourceMacro is a macro's expansion.
	sourceMacro
)

// maxIncludeDepth is how deeply .INCLUDE files may nest. A deeper nest is
// almost certainly a file that includes itself.
const maxIncludeDepth = 64

// maxExpansionDepth is how deeply macro expansions may nest. A macro may
// call itself (with a conditional that ends the recursion), so the limit
// is generous; it stops a macro that never stops calling itself before
// the Go stack overflows.
const maxExpansionDepth = 1000

// sourceFrame is one source on the source stack.
type sourceFrame struct {
	kind sourceKind
	// name is the macro's name, for a sourceMacro frame.
	name string
	// callLine is the line, in the source below this one, that pushed
	// this one: the .INCLUDE statement or the macro call.
	callLine int
	// condBase is how many conditional blocks were open when the source
	// was pushed. A macro's expansion that ends early (.MEXIT) closes
	// the blocks it opened, and only those.
	condBase int
	// exit is set by .MEXIT: the rest of the expansion is skipped.
	exit bool
	// expansion is the macro's expansion state (arguments, for .NARG),
	// for a sourceMacro frame.
	expansion *expansion
}

// wrap puts a frame's location on err, an error at line of the frame's
// own lines. A file's location is just the line (*Error); an expansion's
// names the macro too (*ExpansionError).
func (f *sourceFrame) wrap(line int, err error) error {
	if f.kind == sourceMacro {
		return &ExpansionError{Macro: f.name, Line: line, Err: err}
	}

	return &Error{Line: line, Err: err}
}

// ExpansionError is an error on one line of a macro's expansion. The line
// number counts the expansion's lines from 1 (the first line of the
// macro's body). It is always wrapped in an *Error, or another
// *ExpansionError, naming the call.
type ExpansionError struct {
	Macro string
	Line  int
	Err   error
}

func (e *ExpansionError) Error() string {
	return fmt.Sprintf("in expansion of macro %s, line %d: %v", e.Macro, e.Line, e.Err)
}

// Unwrap returns the error itself, for errors.Is and errors.As.
func (e *ExpansionError) Unwrap() error { return e.Err }

// located wraps err, an error at the current line of the innermost
// source, in the location of every source on the stack, innermost first:
// the full "line 20: in expansion of macro NAME, line 3: ..." form. The
// MACRO dialect uses it to record an error and go on assembling (the
// console dialect instead returns the error up through each runSource,
// and each wraps its own frame's location on the way out).
func (a *Assembler) located(err error) error {
	line := a.line

	for k := len(a.sources) - 1; k >= 0; k-- {
		f := a.sources[k]
		err = f.wrap(line, err)
		line = f.callLine
	}

	return err
}

// count returns how many frames of kind are on the source stack.
func (a *Assembler) count(kind sourceKind) int {
	n := 0

	for _, f := range a.sources {
		if f.kind == kind {
			n++
		}
	}

	return n
}

// innermost returns the innermost frame of kind, or nil if there's none.
func (a *Assembler) innermost(kind sourceKind) *sourceFrame {
	for k := len(a.sources) - 1; k >= 0; k-- {
		if a.sources[k].kind == kind {
			return a.sources[k]
		}
	}

	return nil
}

// assembleLines assembles source as the text of a new file-level source:
// the program itself when the stack is empty, or a .INCLUDE file.
func (a *Assembler) assembleLines(source string) error {
	kind := sourceInclude
	if len(a.sources) == 0 {
		kind = sourceFile
	}

	if kind == sourceInclude && a.count(sourceInclude) >= maxIncludeDepth {
		return vmserrors.New(vmserrors.VAX_INCLUDEDEPTH)
	}

	return a.runSource(&sourceFrame{kind: kind}, strings.Split(source, "\n"))
}

// runSource pushes f and assembles lines as its text, then pops it.
//
// Each line goes, in order, to:
//
//  1. a macro definition being collected (collectDefinition), which
//     takes every line up to its .ENDM without assembling any of it;
//  2. statement, which joins continuation lines and preprocesses the
//     line (uppercasing, comment stripping);
//  3. assembleStatement.
//
// In the console dialect the first failing statement ends assembly, and
// the error is returned with this frame's location on it. In the MACRO
// dialect every error is recorded (with the whole stack's location) and
// assembly goes on.
func (a *Assembler) runSource(f *sourceFrame, lines []string) error {
	f.callLine = a.line
	f.condBase = len(a.cond)

	a.sources = append(a.sources, f)
	defer func() { a.sources = a.sources[:len(a.sources)-1] }()

	outerLine := a.line
	defer func() { a.line = outerLine }()

	// A macro definition started in this source has to end in it. (No
	// definition is being collected when a source is pushed, since a
	// definition's lines are collected, not assembled.)
	defer func() { a.defining = nil }()

	for i, raw := range lines {
		if a.stop || f.exit {
			break
		}

		a.line = i + 1

		if a.defining != nil {
			if err := a.collectDefinition(raw); err != nil {
				if a.dialect != DialectMACRO {
					return f.wrap(i+1, err)
				}

				a.errs = append(a.errs, a.located(err))
			}

			continue
		}

		// A macro's expansion evaluates its string operators (%LENGTH
		// and so on) as each line is reached (see stringOperators), except
		// in lines a conditional is leaving out.
		if f.kind == sourceMacro && !a.skipping() {
			expanded, err := a.stringOperators(raw)
			if err != nil {
				if a.dialect != DialectMACRO {
					return f.wrap(i+1, err)
				}

				a.errs = append(a.errs, a.located(err))

				continue
			}

			raw = expanded
		}

		line, ok := a.statement(raw)
		if !ok || line == "" {
			continue
		}

		if err := a.assembleStatement(line); err != nil {
			if a.dialect != DialectMACRO {
				return f.wrap(i+1, err)
			}

			a.errs = append(a.errs, a.located(err))
		}
	}

	// A continuation with no line to continue it.
	if a.continued != "" && !a.stop && !f.exit {
		line := a.continued
		a.continued = ""

		if err := a.assembleStatement(line); err != nil {
			return err
		}
	}

	// .MEXIT leaves the expansion from inside any conditional blocks it
	// opened; they end with it.
	if f.exit && len(a.cond) > f.condBase {
		a.cond = a.cond[:f.condBase]
	}

	// A macro definition still being collected when its source runs out.
	if a.defining != nil {
		name := a.defining.def.name

		return vmserrors.New(vmserrors.VAX_NOENDM, name)
	}

	return nil
}
