package debugger

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// SHOW STACK (docs/PHASE-42.md, subtask 11, Decision 6), which describes
// each call frame on the stack in full, as the VMS debugger does
// (testdata/dbgcmd/vax/exam.dlg):
//
//	stack frame 0 (7FED5314)
//
//	    condition handler: 00000000
//	       SPA:            0
//	       S:              1
//	       mask:           ^M<R2>
//	       PSW:            0000 (hexadecimal)
//	    saved AP:          7FED534C
//	    saved FP:          7FED5334
//	    saved PC:          DBGCMD\FACT\BACK
//	    saved R2:          00000005
//	    argument list:(1)  00000004
//
// A *call frame* is what the VAX's CALLS and CALLG instructions build on
// the stack: the address FP points at holds, in order,
//
//	FP+0   the address of a condition handler (0 if none)
//	FP+4   a longword packing: bits 31:30 are SPA (how many bytes were
//	       skipped to align the stack), bit 29 is S (1 for CALLS, 0 for
//	       CALLG), bits 27:16 are the mask of the registers R0-R11 the
//	       routine saved, and bits 15:0 are the saved PSW
//	FP+8   the caller's AP (argument pointer)
//	FP+12  the caller's FP, which links the frames into a chain
//	FP+16  the return address (the caller's PC)
//	FP+20  each register in the mask, lowest register first
//
// The frame's argument list is what AP pointed to in that routine: a
// longword count and then that many argument longwords.

// stackValueColumn is the column the values start in; the labels are
// padded out to it.
const stackValueColumn = 23

// maxStackFrames bounds how far SHOW STACK follows the chain, should a
// corrupt stack link its frames into a loop (govax's choice).
const maxStackFrames = 1000

// stackField prints one "label: value" line of a frame, with the value in
// the column all of them share.
func (d *Debugger) stackField(label, value string) {
	d.Console.Printf("%-*s%s\n", stackValueColumn, label, value)
}

// showStack runs SHOW STACK [count]: the frames from the current one out,
// count of them if a count is given, else every one. Each is preceded by a
// blank line, and the whole listing by another, as the debugger prints it.
func (d *Debugger) showStack(countText string) error {
	c := d.Console

	if err := c.RequireInit(); err != nil {
		return err
	}

	limit := uint32(maxStackFrames)

	if text := strings.TrimSpace(countText); text != "" {
		n, err := d.evalWhole(text)
		if err != nil {
			return err
		}

		limit = min(max(n, 1), maxStackFrames)
	}

	fp := c.CPU.GPR(vax.FP)
	ap := c.CPU.GPR(vax.AP)

	if d.imageExited || fp == 0 || fp == cpu.SentinelReturn {
		return vmserrors.New(vmserrors.DBG_NOCALLS)
	}

	c.Printf("\n")

	for n := uint32(0); n < limit && fp != 0 && fp != cpu.SentinelReturn; n++ {
		c.Printf("\n")

		next, nextAP, err := d.showFrame(n, fp, ap)
		if err != nil {
			return err
		}

		// The stack grows down, so a caller's frame is above its
		// callee's: anything else is a broken chain.
		if next <= fp {
			break
		}

		fp, ap = next, nextAP
	}

	return nil
}

// showFrame prints frame number n, whose frame pointer is fp and whose
// routine's argument pointer is ap, and returns the caller's FP and AP
// for the next frame.
func (d *Debugger) showFrame(n, fp, ap uint32) (savedFP, savedAP uint32, err error) {
	c := d.Console

	// The frame is five longwords, then the saved registers.
	var words [5]uint32

	for i := range words {
		if words[i], err = c.Mem.LoadLongword(c.CPU, fp+uint32(i)*4); err != nil {
			return 0, 0, err
		}
	}

	handler, packed, savedAP, savedFP, savedPC := words[0], words[1], words[2], words[3], words[4]

	spa := packed >> 30
	callType := (packed >> 29) & 1
	mask := (packed >> 16) & 0x0FFF
	psw := packed & 0xFFFF

	c.Printf("stack frame %d (%08X)\n\n", n, fp)

	d.stackField("    condition handler:", fmt.Sprintf("%08X", handler))
	d.stackField("       SPA:", fmt.Sprintf("%d", spa))
	d.stackField("       S:", fmt.Sprintf("%d", callType))
	d.stackField("       mask:", maskText(mask))
	d.stackField("       PSW:", fmt.Sprintf("%s (%s)", formatRadix(uint64(psw), 2, d.outputRadix), radixWord(d.outputRadix)))
	d.stackField("    saved AP:", fmt.Sprintf("%08X", savedAP))
	d.stackField("    saved FP:", fmt.Sprintf("%08X", savedFP))
	d.stackField("    saved PC:", d.framePC(savedPC))

	addr := fp + 20

	for r := 0; r <= 11; r++ {
		if mask&(1<<r) == 0 {
			continue
		}

		v, err := c.Mem.LoadLongword(c.CPU, addr)
		if err != nil {
			return 0, 0, err
		}

		addr += 4

		d.stackField(fmt.Sprintf("    saved R%d:", r), fmt.Sprintf("%08X", v))
	}

	d.showArguments(ap)

	return savedFP, savedAP, nil
}

// framePC is a saved PC as the frame shows it: named from the program's
// symbols where it can be, else the address; or <console> for the
// sentinel return address govax gives a routine the console called.
func (d *Debugger) framePC(pc uint32) string {
	if pc == cpu.SentinelReturn {
		return "<console>"
	}

	return d.Console.LocationText(pc)
}

// maskText is a register save mask the way MACRO writes it: ^M<R2,R3>.
func maskText(mask uint32) string {
	var regs []string

	for r := 0; r <= 11; r++ {
		if mask&(1<<r) != 0 {
			regs = append(regs, fmt.Sprintf("R%d", r))
		}
	}

	return "^M<" + strings.Join(regs, ",") + ">"
}

// showArguments prints the frame's argument list: its count in
// parentheses, then each argument in 8 hex digits, the first beside the
// label and the rest under it. An AP that doesn't lead to a plausible
// list (a count over 255, an unreadable address) shows nothing, as the
// debugger's frame for its own caller does. After the list comes a line
// of blanks, as in the probe's logs. The layout of the lines after the
// first is unconfirmed (every probe frame had one argument).
func (d *Debugger) showArguments(ap uint32) {
	c := d.Console

	if ap < 0x200 {
		return
	}

	argc, err := c.Mem.LoadLongword(c.CPU, ap)
	if err != nil || argc > 255 {
		return
	}

	label := fmt.Sprintf("    argument list:(%d)", argc)

	for i := uint32(1); i <= argc; i++ {
		v, err := c.Mem.LoadLongword(c.CPU, ap+i*4)
		if err != nil {
			break
		}

		d.stackField(label, fmt.Sprintf("%08X", v))

		label = ""
	}

	// With no arguments the label still has to be shown.
	if argc == 0 {
		d.stackField(label, "")
	}

	c.Printf("%s\n", strings.Repeat(" ", 18))
}
