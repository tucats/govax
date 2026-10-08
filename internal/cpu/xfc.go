package cpu

import (
	"errors"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// XFC (opcode 0xFC) selector codes, matching emul_xfc.c's case labels. Not a
// real VAX ISA instruction — eVAX's own escape hatch into console I/O, DCL
// parsing, and RTL system-service/shim dispatch. Deferred entirely by Phase
// 07 (docs/PHASE-07.md's open questions); implemented here per Phase 10's own
// scope note.
const (
	xfcConsoleWrite = 0x01
	xfcConsoleRead  = 0x02
	xfcConsoleCmd   = 0x03
	xfcConsolePut   = 0x04 // govax's own; not in eVAX (docs/PERFORMANCE.md, Study 1 R1)
	xfcQuitEmulator = 0x78
	xfcDCL          = 0x79
	xfcP1Vector     = 0x7A
	xfcHaltSilent   = 0x7B
	xfcHalt         = 0x7C
	xfcShim         = 0x7D
	xfcVMW          = 0x7E
	xfcVMR          = 0x7F
)

// ssNormal is VMS's "normal successful completion" status, SS$_NORMAL. A
// VMS status is a longword whose low bit set means success, and 1 is the
// plain success code. (internal/cpu doesn't import internal/vmsdef, which
// holds VMS's symbol tables, for this one value.)
const ssNormal = 1

// DCL subfunction codes for xfcDCL, selected by R0 — matching emul_xfc.c's
// nested switch on vax.R0 within case 0x79.
const (
	dclPresent    = 1
	dclGetKeyword = 2
	dclGetString  = 3
	dclGetInteger = 4
)

func init() {
	instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: 0xFC}), emulXfc)
}

// emulXfc is XFC, port of emul_xfc.c's emul_xfc. code is the instruction's
// implicit immediate byte operand.
func emulXfc(e *Engine, d *Decoded) error {
	raw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	code := uint32(raw)

	switch code {
	case xfcConsoleWrite:
		if e.services == nil {
			return &Fault{Code: ExcPrivileged}
		}

		e.services.ConsoleWriteByte(byte(e.cpu.GPR(vax.R0)))

		return nil

	case xfcConsoleRead:
		if e.services == nil {
			return &Fault{Code: ExcPrivileged}
		}

		ch := e.services.ConsoleReadByte()
		e.cpu.SetGPR(vax.R0, (e.cpu.GPR(vax.R0)&0xFFFFFF00)|uint32(ch))

		return nil

	case xfcConsoleCmd:
		return emulXfcConsoleCmd(e)

	case xfcConsolePut:
		return emulXfcConsolePut(e)

	case xfcQuitEmulator:
		if e.cpu.PSL().CurMod() != vax.Kernel {
			return &Fault{Code: ExcPrivileged}
		}

		// Matches emul_xfc.c's own vax.halted = VAX_USERHALT plus
		// vax.console.running = 0: this opcode stops the entire
		// emulator (driver.c's main() loop exits), not just the
		// running VAX program -- unlike xfcHalt/xfcHaltSilent below,
		// which only halt the CPU and return control to the console
		// prompt. RequestQuit carries the "stop the console too" half
		// up through Engine's SystemServices hook rather than an
		// os.Exit call here, which used to tear down the whole host
		// process from inside the emulation layer and made this path
		// untestable.
		if e.services != nil {
			e.services.RequestQuit()
		}

		return ErrHalted

	case xfcDCL:
		return emulXfcDCL(e)

	case xfcP1Vector:
		return emulXfcP1Vector(e)

	case xfcHaltSilent, xfcHalt:
		return ErrHalted

	case xfcShim:
		return emulXfcShim(e)

	case xfcVMW, xfcVMR:
		return emulXfcVM(e, code)

	default:
		return &Fault{Code: ExcReservedOp}
	}
}

// emulXfcConsoleCmd is XFC$CONSOLE_CMD: R0 addresses a counted string (a
// signed 16-bit length, then that many bytes, no terminator) to dispatch as a
// console command line. A non-positive length is silently ignored, matching
// emul_xfc.c's own "if (slen <= 0) break" guard against an empty or
// malformed string.
func emulXfcConsoleCmd(e *Engine) error {
	if e.services == nil {
		return &Fault{Code: ExcPrivileged}
	}
	
	addr := e.cpu.GPR(vax.R0)

	rawLen, err := e.mem.LoadWord(e.cpu, addr)
	if err != nil {
		return err
	}

	slen := int16(rawLen)
	if slen <= 0 {
		return nil
	}

	buf := make([]byte, slen)
	if err := e.mem.Load(e.cpu, addr+2, buf); err != nil {
		return err
	}

	e.cpu.SetGPR(vax.R0, e.services.ConsoleCommand(string(buf)))

	return nil
}

// emulXfcConsolePut is XFC$CONSOLE_PUT: write a whole string to the
// console in one exit to the emulator. R0 holds the address of a VMS string
// descriptor, the standard way VAX/VMS code passes a string: a 16-bit
// length (bytes 0-1), two type/class bytes this ignores (bytes 2-3), and the
// 32-bit address of the text (bytes 4-7). On return R0 is SS$_NORMAL (1).
//
// This replaced the microkernel's original console output, which emulated
// the console terminal's hardware one character at a time: a CHMK into
// kernel mode per character, an MTPR to the transmit data register (TXDB),
// and then a spin until the "transmitter ready" interrupt arrived and set a
// flag again. That interrupt could only be admitted on an interval-clock
// tick when another interrupt was in the way, so console output depended on
// the clock running. One XFC per string has no such dependency, and no
// privilege is needed: like XFC$CONSOLE_WRITE it can be used from any mode.
//
// The descriptor and the text are read with the current mode's access
// checks, so a program can only print memory it could read itself; a
// failed read is the ordinary access-violation fault. A zero length writes
// nothing.
func emulXfcConsolePut(e *Engine) error {
	if e.services == nil {
		return &Fault{Code: ExcPrivileged}
	}

	dsc := e.cpu.GPR(vax.R0)

	length, err := e.mem.LoadWord(e.cpu, dsc)
	if err != nil {
		return err
	}

	addr, err := e.mem.LoadLongword(e.cpu, dsc+4)
	if err != nil {
		return err
	}

	if length > 0 {
		buf := make([]byte, length)
		if err := e.mem.Load(e.cpu, addr, buf); err != nil {
			return err
		}

		e.services.ConsoleWrite(buf)
	}

	e.cpu.SetGPR(vax.R0, ssNormal)

	return nil
}

// emulXfcDCL is XFC$DCL: R0 selects one of four DCL-parse-callback
// subfunctions (R1-R3 carry that subfunction's own arguments), matching
// emul_xfc.c's case 0x79. An unrecognized subfunction is a reserved-operand
// fault — the C source instead prints a diagnostic and halts the machine
// unconditionally, but a Go fault is the mechanism this port uses everywhere
// else for "the program did something the microkernel doesn't support," so
// that's used here too rather than the C source's own ad hoc halt.
func emulXfcDCL(e *Engine) error {
	if e.services == nil {
		return &Fault{Code: ExcPrivileged}
	}

	r1, r2, r3 := e.cpu.GPR(vax.R1), e.cpu.GPR(vax.R2), e.cpu.GPR(vax.R3)

	switch e.cpu.GPR(vax.R0) {
	case dclPresent:
		e.cpu.SetGPR(vax.R0, e.services.DCLPresent(r1, r2))

	case dclGetKeyword:
		e.cpu.SetGPR(vax.R0, e.services.DCLGetKeyword(r1, r2, r3))

	case dclGetString:
		if addr, ok := e.services.DCLGetString(r1, r2); ok {
			e.cpu.SetGPR(vax.R0, addr)
		} else {
			e.cpu.SetGPR(vax.R0, 0)
		}

	case dclGetInteger:
		e.cpu.SetGPR(vax.R0, e.services.DCLGetInteger(r1, r2))

	default:
		return &Fault{Code: ExcReservedOp}
	}

	return nil
}

// emulXfcP1Vector is XFC$P1VECTOR: invokes a SYS$ system service, matching
// emul_xfc.c's `return call_service(vax.PC - 4)`. A real P1-vector entry
// reached via CALLS (p1_vector.c's p1_init: every entry except the one
// Jmp-style SYS$SRCHANDLER) is a genuine tiny procedure at its well-known
// address: a 2-byte zero procedure-entry mask (which CALLS's own frame-build
// logic reads and skips, landing PC two bytes past the entry address) then
// the 2-byte XFC instruction itself. By the time this Handler runs, PC has
// advanced past that whole 4-byte run (2-byte mask-skip + 2-byte XFC fetch),
// so the entry address call_service (and p1VectorByMatchAddr, built to match
// it -- see that table's own doc comment on the Jmp case's "Addr-2") expects
// is PC-4, not merely "PC minus the XFC instruction's own 2 bytes": that
// would land on the XFC opcode's address, not the CALLS target's, off by the
// mask's own 2 bytes for every non-Jmp entry.
//
// A handled call always sets R0 before returning, even when it also reports
// an error (e.g. a service that requests a halt) — except ErrServiceWait,
// which re-executes the XFC instead of completing, and a *ServiceCall,
// which calls a procedure that returns to the XFC (see exit.go); an
// ErrImageExit sets R0 and then ends the image — matching call_service's
// own "vax.R0 = rc" happening unconditionally after the native handler
// returns, before the caller's fetch loop next checks vax.halted.
func emulXfcP1Vector(e *Engine) error {
	if e.services == nil {
		return &Fault{Code: ExcPrivileged}
	}

	pc := e.cpu.GPR(vax.PC) - 4

	r0, handled, err := e.services.SystemService(pc)
	if !handled {
		if err != nil {
			return err
		}

		return &Fault{Code: ExcReservedOp}
	}

	if errors.Is(err, ErrServiceWait) {
		e.cpu.SetGPR(vax.PC, e.instructionPC) // run this XFC again next Step

		return nil
	}

	// The service needs a procedure called first ($EXIT's exit handlers):
	// call it, returning to this XFC. R0 is left alone.
	var call *ServiceCall
	if errors.As(err, &call) {
		return e.callForService(call)
	}

	e.cpu.SetGPR(vax.R0, r0)

	// The image has exited ($EXIT): r0 is its exit status.
	if errors.Is(err, ErrImageExit) {
		return e.exitImage()
	}

	return err
}

// emulXfcShim is XFC$SHIM: R0 selects a LIB$/CRTL shim routine by numeric
// code, matching emul_xfc.c's `return shim()`. See emulXfcP1Vector's doc
// comment on setting R0 even when a handled call also reports an error.
// A shim that waits (ErrServiceWait: LIB$PUT_OUTPUT to a full mailbox,
// docs/PHASE-46.md) runs again: the XFC is re-executed, with R0 still
// selecting it.
func emulXfcShim(e *Engine) error {
	if e.services == nil {
		return &Fault{Code: ExcPrivileged}
	}

	r0, handled, err := e.services.Shim(e.cpu.GPR(vax.R0))
	if !handled {
		if err != nil {
			return err
		}

		return &Fault{Code: ExcReservedOp}
	}

	if errors.Is(err, ErrServiceWait) {
		e.cpu.SetGPR(vax.PC, e.instructionPC) // run this XFC again next Step

		return nil
	}

	e.cpu.SetGPR(vax.R0, r0)

	return err
}

// emulXfcVM is XFC$VMW/XFC$VMR: translates the virtual address in R0 to a
// physical address (forcing MAPEN on regardless of its current state,
// matching emul_xfc.c's saved/restored vax.MAPEN dance), leaving the result
// in R0 on success or setting PSL<V> on a translation failure — no real
// fault either way, matching the C source's own VM_NOSIGNAL-style handling
// (see emulProbe's doc comment in misc.go for the same pattern).
func emulXfcVM(e *Engine, code uint32) error {
	access := vm.AccessRead
	if code == xfcVMW {
		access = vm.AccessWrite
	}

	addr := e.cpu.GPR(vax.R0)
	savedMapen := e.cpu.PR(vax.MAPEN)
	e.cpu.SetPR(vax.MAPEN, 1)
	paddr, err := e.mem.Translate(e.cpu, addr, access)
	e.cpu.SetPR(vax.MAPEN, savedMapen)

	psl := e.cpu.PSL()

	if err != nil {
		psl.SetV(true)
	} else {
		e.cpu.SetGPR(vax.R0, paddr)
	}

	e.cpu.SetPSL(psl)

	return nil
}
