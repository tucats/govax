package debugger

import (
	"slices"
	"strings"
	"sync"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file decides, before each instruction runs, whether one of the
// debugger's breakpoints (Debugger.Breakpoints) stops the program, and
// says so the way the VMS debugger does. The commands that make and
// remove breakpoints are in breakcmd.go.
//
// A few VAX/VMS terms for readers new to them:
//
//   - A *routine* entered with CALLS or CALLG starts with a two-byte
//     *entry mask* (which registers it saves). The mask is data, not an
//     instruction, so "at routine X" means the address just after it.
//   - *BSB* and *JSB* call a subroutine and *RSB* returns from it; they
//     keep no call frame. *CALLS* and *CALLG* build one and *RET* tears
//     it down. The VMS debugger's "calls" class is all of these.
//   - A *branch* is any instruction that may change the PC to somewhere
//     other than the next instruction without calling anything: a
//     conditional branch (BEQL, BLBS), a loop (SOBGTR, AOBLSS), a bit test
//     (BBS), a jump (JMP, BRB), or a CASE.

// The instruction classes SET BREAK/CALL and SET BREAK/BRANCH stop at, in
// the order SHOW BREAK lists them, which is the VMS debugger's own
// (testdata/dbgcmd/vax/brkcls.dlg).
var (
	callNames = []string{"BSBB", "BSBW", "CALLG", "CALLS", "JSB", "RET", "RSB"}

	branchNames = []string{
		"ACBB", "ACBD", "ACBF", "ACBG", "ACBH", "ACBL", "ACBW", "AOBLEQ",
		"AOBLSS", "BBC", "BBCC", "BBCCI", "BBCS", "BBS", "BBSC", "BBSS",
		"BBSSI", "BCC", "BCS", "BEQL", "BEQLU", "BGEQ", "BGEQU", "BGTR",
		"BGTRU", "BLBC", "BLBS", "BLEQ", "BLEQU", "BLSS", "BLSSU", "BNEQ",
		"BNEQU", "BRB", "BRW", "BVC", "BVS", "CASEB", "CASEL", "CASEW",
		"JMP", "SOBGEQ", "SOBGTR",
	}
)

// classOps resolves a list of mnemonics to the instruction table's
// entries. A name the table doesn't have under that spelling (BCC and
// BCS are the same opcodes as BGEQU and BLSSU, and the table has one name
// for each opcode) is skipped: its opcode is in the set under its other
// name.
func classOps(names []string) map[*cpu.Instruction]bool {
	set := make(map[*cpu.Instruction]bool, len(names))

	for _, name := range names {
		if inst := lookupInstruction(name); inst != nil {
			set[inst] = true
		}
	}

	return set
}

// callOps and branchOps are the two classes as sets, built the first time
// a breakpoint needs them.
var (
	callOps   = sync.OnceValue(func() map[*cpu.Instruction]bool { return classOps(callNames) })
	branchOps = sync.OnceValue(func() map[*cpu.Instruction]bool { return classOps(branchNames) })
)

// breakpointHit checks every breakpoint against the instruction about to
// run at pc. A breakpoint that is reached counts toward its /AFTER, and
// stops the program only when the count is up and its WHEN condition, if
// it has one, holds. Every breakpoint reached counts, even when another
// at the same place has already stopped the program, and each one-shot
// breakpoint that stops it is removed.
//
// It prints the stop message of the first breakpoint that stops, and
// queues that breakpoint's DO commands for Start to run once the
// debugger is back at its prompt. It reports whether the program stops.
func (d *Debugger) breakpointHit(pc uint32) bool {
	// The instruction at pc, read only if a breakpoint wants to know
	// what it is (most runs have none that do).
	var (
		inst   *cpu.Instruction
		peeked bool
	)

	peek := func() *cpu.Instruction {
		if !peeked {
			peeked = true
			inst, _ = d.Console.Engine.PeekInstruction()
		}

		return inst
	}

	var stopped *Breakpoint

	// Work from a copy: a one-shot breakpoint removes itself from the
	// list as it stops the program.
	for _, bp := range slices.Clone(d.Breakpoints) {
		if !d.reached(bp, pc, peek) {
			continue
		}

		if bp.After > 0 {
			bp.hits++

			if bp.hits < bp.After {
				continue
			}
		}

		if bp.When != "" && !d.whenHolds(bp) {
			continue
		}

		if bp.Temporary {
			d.removeBreakpointPtr(bp)
		}

		if stopped == nil {
			stopped = bp
		}
	}

	if stopped == nil {
		return false
	}

	if text := d.stopMessage(stopped, pc); text != "" {
		d.Console.Printf("%s\n", text)
	}

	d.pendingDo = stopped.Do

	return true
}

// reached reports whether the instruction about to run at pc is one bp
// stops at (before its count and condition are considered). peek returns
// that instruction, or nil when it can't be read.
func (d *Debugger) reached(bp *Breakpoint, pc uint32, peek func() *cpu.Instruction) bool {
	switch bp.Kind {
	case BreakAddress:
		return bp.Addr == pc

	case BreakCall:
		return callOps()[peek()]

	case BreakBranch:
		return branchOps()[peek()]

	case BreakLine:
		return d.Console.LineStart(pc)

	case BreakInstruction:
		inst := peek()

		return inst != nil && slices.Contains(bp.Ops, inst)

	case BreakAnyInstruction:
		return true

	case BreakReturn:
		inst := peek()

		return inst != nil && inst.Name == "RET" && pc >= bp.Start && pc-bp.Start < bp.Size
	}

	// BreakException stops when a condition is signaled (signalBreak), not
	// at an instruction.
	return false
}

// stopMessage is what the debugger says when bp stops the program at pc,
// in the VMS debugger's words:
//
//	break at DBGCMD\FACT\BACK
//	break at routine DBGCMD\FACT
//	break on calls at DBGCMD\FACT\%LINE 60
//	break on return from routine DBGCMD\FACT at DBGCMD\FACT\%LINE 58
//
// It is empty for the debugger's own silent breakpoints.
func (d *Debugger) stopMessage(bp *Breakpoint, pc uint32) string {
	where := d.Console.LocationText(pc)

	switch bp.Kind {
	case BreakCall:
		return "break on calls at " + where

	case BreakBranch:
		return "break on branches at " + where

	case BreakLine:
		return "break on lines at " + where

	case BreakInstruction:
		return "break on instruction(s) at " + where

	case BreakAnyInstruction:
		return "break on instruction at " + where

	case BreakReturn:
		return "break on return from routine " + bp.Name + " at " + where
	}

	switch {
	case bp.Quiet:
		return ""

	case bp.Step:
		return "Stepped to " + where

	case bp.Routine:
		return "break at routine " + bp.Name
	}

	return "break at " + where
}

// whenHolds evaluates bp's WHEN condition. A condition that can't be
// evaluated (an unreadable address, a name that isn't defined) shows its
// error and counts as true, so the breakpoint stops: the VMS debugger
// stops rather than run past a breakpoint it couldn't judge
// (testdata/dbgcmd/vax/break.dlg).
func (d *Debugger) whenHolds(bp *Breakpoint) bool {
	ok, err := d.evalCondition(bp.When)
	if err != nil {
		d.Console.Printf("%%%s\n", err)

		return true
	}

	return ok
}

// onSignal is the console's report that a condition has been signaled
// and is about to be offered to the program's handlers (the
// console.Console.OnSignal hook). If SET BREAK/EXCEPTION is in effect it
// is held for the run loop, which stops the program at the next
// instruction boundary (signalBreak).
func (d *Debugger) onSignal(u console.UnhandledException) {
	for _, bp := range d.Breakpoints {
		if bp.Kind != BreakException {
			continue
		}

		d.signalled = &u
		d.signalBP = bp

		return
	}
}

// signalBreak reports whether the instruction just executed signaled a
// condition that a SET BREAK/EXCEPTION breakpoint stops at, and if so
// says so, as unhandledBreak does for a condition nobody handled:
//
//	break on exception preceding DBGCMD\CATCH\%LINE 71
//
// The program is paused with the condition's dispatch set up; GO sends it
// on to the handlers. The breakpoint's /AFTER, WHEN, and DO apply as they
// do for any other.
func (d *Debugger) signalBreak() bool {
	u, bp := d.signalled, d.signalBP
	if u == nil {
		return false
	}

	d.signalled, d.signalBP = nil, nil

	if bp.After > 0 {
		bp.hits++

		if bp.hits < bp.After {
			return false
		}
	}

	if bp.When != "" && !d.whenHolds(bp) {
		return false
	}

	where := "at"
	if u.Preceding {
		where = "preceding"
	}

	// VMS shows the condition's message before the break, as it does for
	// an unhandled one (where the catch-all shows it).
	if text := d.Console.StatusText(u.Condition); text != "" {
		d.Console.Printf("%s\n", text)
	}

	d.Console.Printf("break on exception %s %s\n", where, d.Console.LocationText(u.PC))

	if bp.Temporary {
		d.removeBreakpointPtr(bp)
	}

	d.pendingDo = bp.Do

	return true
}

// runDo runs the commands of the DO clause of the breakpoint that last
// stopped the program, the way they would run if typed at the DBG>
// prompt. The clause is "(command; command; ...)". A command that fails
// shows its message, and the ones after it still run.
func (d *Debugger) runDo() {
	text := d.pendingDo
	d.pendingDo = ""

	if text == "" {
		return
	}

	for _, command := range splitTop(unparenthesize(text), ';') {
		if err := d.Dispatch(command); err != nil && !vmserrors.MessageInhibited(err) {
			d.Console.Printf("%%%s\n", err)
		}
	}
}

// unparenthesize removes the parentheses around text, if it is wrapped in
// one matching pair, and the blanks inside them.
func unparenthesize(text string) string {
	text = strings.TrimSpace(text)

	if strings.HasPrefix(text, "(") && matchingParen(text, 0) == len(text)-1 {
		return strings.TrimSpace(text[1 : len(text)-1])
	}

	return text
}

// matchingParen returns the index of the parenthesis that closes the one
// at text[open], or -1 if it isn't closed. Parentheses inside quotes
// don't count.
func matchingParen(text string, open int) int {
	depth := 0

	var quote byte

	for i := open; i < len(text); i++ {
		ch := text[i]

		switch {
		case quote != 0:
			if ch == quote {
				quote = 0
			}

		case ch == '"' || ch == '\'':
			quote = ch

		case ch == '(':
			depth++

		case ch == ')':
			depth--

			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// splitTop splits text at each sep that isn't inside parentheses or
// quotes, trimming blanks and dropping empty pieces.
func splitTop(text string, sep byte) []string {
	var (
		parts []string
		start int
		depth int
		quote byte
	)

	for i := 0; i < len(text); i++ {
		ch := text[i]

		switch {
		case quote != 0:
			if ch == quote {
				quote = 0
			}

		case ch == '"' || ch == '\'':
			quote = ch

		case ch == '(':
			depth++

		case ch == ')':
			depth--

		case ch == sep && depth == 0:
			parts = append(parts, text[start:i])
			start = i + 1
		}
	}

	parts = append(parts, text[start:])

	out := parts[:0]

	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}

	return out
}
