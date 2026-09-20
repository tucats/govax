package vax

import "io"

// DebugFlags is a bitmask of developer/tracing toggles, matching the C
// source's `vax.debug` field and vax.h's DBG_* #defines (vax.h:320-346).
// See docs/PHASE-17.md for which flags this port actually wires behavior
// for, and why several are deliberately settable/showable no-ops (matching
// the C reference having no consumer for them either).
type DebugFlags uint32

// DBG_* flag bits, named to match SHOW DEBUG's own display names where one
// exists (see ShowDebug in internal/console/show.go).
const (
	DebugSymbols    DebugFlags = 0x00000001 // DBG_SYMBOLS
	DebugNative     DebugFlags = 0x00000002 // DBG_DEBUG: invoke native debugger
	DebugMemory     DebugFlags = 0x00000004 // DBG_MEMORY
	DebugInterrupts DebugFlags = 0x00000008 // DBG_INTERRUPTS
	DebugExceptions DebugFlags = 0x00000010 // DBG_EXCEPTIONS
	DebugVM         DebugFlags = 0x00000020 // DBG_VM
	DebugTB         DebugFlags = 0x00000040 // DBG_TB
	DebugFunctions  DebugFlags = 0x00000080 // DBG_FUNCTIONS (no consumer, no command surface)
	DebugRegisters  DebugFlags = 0x00000100 // DBG_REGISTERS
	DebugImages     DebugFlags = 0x00000200 // DBG_IMAGES
	DebugUserHalt   DebugFlags = 0x00000400 // DBG_USERHALT
	DebugKeyboard   DebugFlags = 0x00000800 // DBG_KBD
	DebugServices   DebugFlags = 0x00001000 // DBG_SERVICES
	DebugDCL        DebugFlags = 0x00002000 // DBG_DCL
	DebugUnimp      DebugFlags = 0x00004000 // DBG_UNIMP (no consumer, no command surface)
	DebugCHM        DebugFlags = 0x00008000 // DBG_CHM
	DebugExpand     DebugFlags = 0x00010000 // DBG_EXPAND
	DebugLogicals   DebugFlags = 0x00020000 // DBG_LOGICALS
	DebugDevices    DebugFlags = 0x00040000 // DBG_DEVICES
	DebugLibinit    DebugFlags = 0x00080000 // DBG_LIBINIT
	DebugProcess    DebugFlags = 0x00100000 // DBG_PROCESS
	DebugFullDisasm DebugFlags = 0x00200000 // DBG_FULLDISASM
	DebugRMS        DebugFlags = 0x00400000 // DBG_RMS
	DebugP1         DebugFlags = 0x01000000 // DBG_P1 (no consumer, settable only)
	DebugP2         DebugFlags = 0x02000000 // DBG_P2 (no consumer, settable only)
	DebugP3         DebugFlags = 0x04000000 // DBG_P3 (no consumer, settable only)
	DebugP4         DebugFlags = 0x08000000 // DBG_P4 (no consumer, settable only)
	DebugUserStep   DebugFlags = 0x10000000 // DBG_USERSTEP

	// DebugDefault matches initialization.c's alloc_vax default assignment
	// (vax.debug = DBG_REGISTERS | DBG_USERHALT | DBG_LIBINIT).
	DebugDefault = DebugRegisters | DebugUserHalt | DebugLibinit | DebugUserStep
)

// Debug returns the current debug/tracing flag bitmask.
func (c *CPU) Debug() DebugFlags { return c.debug }

// SetDebug replaces the debug/tracing flag bitmask.
func (c *CPU) SetDebug(flags DebugFlags) { c.debug = flags }

// DebugEnabled reports whether every bit in flag is set in the current
// debug/tracing flag bitmask.
func (c *CPU) DebugEnabled(flag DebugFlags) bool { return c.debug&flag == flag }

// SetDebugWriter sets the destination for debug/tracing output. Passing nil
// discards all trace output (see DebugWriter).
func (c *CPU) SetDebugWriter(w io.Writer) { c.debugOut = w }

// DebugWriter returns the current debug/tracing output destination, or
// io.Discard if none has been set — so callers can unconditionally write to
// it behind a DebugEnabled check with no nil-writer special-casing.
func (c *CPU) DebugWriter() io.Writer {
	if c.debugOut == nil {
		return io.Discard
	}

	return c.debugOut
}
