package cpu

import "fmt"

// Exception is a VAX machine-check/fault/interrupt exception code, matching
// vax.h's EXC_* constants. It doubles as the byte offset into the System
// Control Block (SCBB + code) where the exception's vector is found.
type Exception uint8

const (
	ExcMachineCheck  Exception = 0x04
	ExcKernelStackNV Exception = 0x08
	ExcPowerFail     Exception = 0x0C
	ExcPrivileged    Exception = 0x10 // Privileged or reserved instruction
	ExcCustomer      Exception = 0x14 // Customer-reserved instruction
	ExcReservedOp    Exception = 0x18 // Reserved operand fault
	ExcReservedAddr  Exception = 0x1C // Reserved addressing mode fault
	ExcAccessViol    Exception = 0x20 // Access control violation
	ExcTranslationNV Exception = 0x24 // Translation not valid
	ExcTracePending  Exception = 0x28
	ExcBreakpoint    Exception = 0x2C
	ExcCompatibility Exception = 0x30 // PDP-11 compatibility mode fault
	ExcArithmetic    Exception = 0x34
	ExcChangeModeK   Exception = 0x40
	ExcChangeModeE   Exception = 0x44
	ExcChangeModeS   Exception = 0x48
	ExcChangeModeU   Exception = 0x4C
	ExcSoftware1     Exception = 0x84 // Base of the 15 SIRR-requestable software interrupts (IPL 1-15)

	// ExcInterval/ExcConRead/ExcConWrite are eVAX's own device-interrupt
	// vector assignments (not real VAX architectural exceptions -- there is
	// no single standard SCB layout for a simulated console/clock device),
	// matching internal/asm/builtins.go's EXC$INTERVAL/EXC$CONREAD/
	// EXC$CONWRITE symbols and interrupt.c's own call sites (vax.c's
	// interval-clock tick, emul_procreg.c's RXCS/TXCS/TXDB cases). See
	// docs/PHASE-14.md.
	ExcInterval Exception = 0xC0
	ExcConRead  Exception = 0xF8
	ExcConWrite Exception = 0xFC
)

// Fault reports a VAX exception: what happened, and the data
// Engine.HandleFault (internal/cpu/handlefault.go) needs to build the
// signal-argument stack frame. This is the Go equivalent of interrupt.c's
// set_fault, minus
// the parts of struct FAULT that are console/history bookkeeping
// (history_id, the fault-history linked list) rather than exception state.
type Fault struct {
	Code Exception
	// Args holds this exception's signal arguments (0-6 of them), matching
	// set_fault's variadic Count/... parameters.
	Args []uint32
}

func (f *Fault) Error() string {
	return fmt.Sprintf("cpu: exception %#02x, args=%v", f.Code, f.Args)
}
