package cpu

import (
	"math"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// This is the Go port of fpu.c's fpu_load/fpu_store: VAX F_floating (4-byte)
// and D_floating (8-byte) conversion to/from a native float64. It is a
// from-scratch reimplementation of the conversion algorithm using direct bit
// arithmetic, not a port of fpu.c's byte-shuffle loops -- see
// docs/PHASE-05.md's design notes for why, and for how the layout below was
// derived and cross-checked against a standalone C harness built from the
// real fpu.c.

// Floating-exception signal-argument sub-codes, matching fpu.h's FAULT_FLT_*/
// TRAP_INT_OVF constants (the second signal argument passed to set_fault
// alongside EXC_ARITH/EXC_RESOP).
const (
	trapIntOvf  = 0x01 // TRAP_INT_OVF: integer overflow during a float->int conversion
	faultFltOvf = 0x08 // FAULT_FLT_OVF: floating overflow (store exponent > 127)
	faultFltUnd = 0x0A // FAULT_FLT_UND: floating underflow (store exponent < -127), PSL<FU> set
)

// Integer-overflow bounds for float->integer conversion (CVTFB/CVTFW/CVTFL/
// CVTRFL and their D-floating counterparts), per the VAX ISA manual's §8.3
// "Data Types" -- named constants per this phase's own deliverable, rather
// than re-derived ad hoc at each call site. These are the bounds
// reference/eVAX/AUDIT.md's N2 finding fixed in the C reference (already
// fixed there; replicated here, not a live bug).
const (
	byteMin = -128
	byteMax = 127
	wordMin = -32768
	wordMax = 32767
	longMin = -2147483648
	longMax = 2147483647
)

// wordSwap exchanges the upper and lower 16 bits of a 32-bit value -- VAX's
// floating-point "word-swapped" storage convention: a register or memory
// longword read as a plain integer has its two conceptual halves in the
// opposite order from a straightforward sign/exponent/fraction packing.
func wordSwap(v uint32) uint32 {
	return v<<16 | v>>16
}

// fpuStore converts value to VAX F_floating (size 4) or D_floating (size 8)
// bits, returning them zero-extended in a uint64 the same way Operand.Store
// expects (low 32 bits = the low-address/low-order longword, high 32 = the
// high-address/high-order longword, for size 8).
//
// Matches fpu_store's algorithm: a zero value short-circuits to all-zero
// bits; the exponent is checked against VAX's excess-128 8-bit range before
// biasing, faulting on overflow (always) or underflow (only when PSL<FU> is
// set -- otherwise flushed to zero, fixing fpu_store's dead-code underflow
// flush, see docs/PHASE-05.md's design notes); F_floating additionally
// rounds the 52-bit IEEE mantissa down to F_floating's 23 bits (round the
// first dropped bit, propagating carry into the exponent -- possibly turning
// a rounding carry into an overflow fault), matching fpu_store's explicit
// rounding block.
func fpuStore(cpu *vax.CPU, size int, value float64) (uint64, error) {
	bits, underflow, overflow := encodeFloatCore(size, value)
	if underflow {
		if cpu.PSL().FU() {
			return 0, &Fault{Code: ExcArithmetic, Args: []uint32{faultFltUnd}}
		}

		return 0, nil
	}

	if overflow {
		return 0, &Fault{Code: ExcArithmetic, Args: []uint32{faultFltOvf}}
	}

	return bits, nil
}

// EncodeFloat is fpuStore's no-CPU sibling, exported for internal/asm's
// assembler: it needs to encode F_FLOAT/D_FLOAT literals (.F_FLOAT/
// .D_FLOAT, and floating "#"/"I^#" immediate operands) with no live CPU/PSL
// to consult for the underflow trap. Underflow always flushes to zero (as
// it would with PSL<FU> clear); overflow is reported via the bool return
// instead of a machine fault.
func EncodeFloat(size int, value float64) (bits uint64, overflow bool) {
	bits, _, overflow = encodeFloatCore(size, value)

	return bits, overflow
}

// DecodeFloat converts VAX F_floating (size 4) or D_floating (size 8) bits
// to a float64, exported for internal/asm's disassembler. Unlike fpuLoad, a
// reserved (malformed) encoding decodes as 0 rather than reporting the
// reserved-operand fault fpuLoad raises on a live CPU — there's no fault to
// deliver when just formatting bytes for display, with no CPU at hand.
func DecodeFloat(bits uint64, size int) float64 {
	v, err := fpuLoad(bits, size)
	if err != nil {
		return 0
	}

	return v
}

// encodeFloatCore is the pure bit-arithmetic half of VAX F_floating/
// D_floating encoding, shared by fpuStore (which layers on the CPU-trap
// semantics for an out-of-range result) and EncodeFloat above. bits is only
// meaningful when both underflow and overflow are false.
func encodeFloatCore(size int, value float64) (bits uint64, underflow, overflow bool) {
	if value == 0 {
		return 0, false, false
	}

	raw := math.Float64bits(value)
	hi32 := uint32(raw >> 32)
	lo32 := uint32(raw)

	sign := hi32 >> 31
	ieeeExp := int((hi32 >> 20) & 0x7FF)
	vaxExp := ieeeExp - 1023 + 1

	if vaxExp < -127 {
		return 0, true, false
	}

	if vaxExp > 127 {
		return 0, false, true
	}

	biasedExp := uint32(vaxExp + 128)
	frac20 := hi32 & 0xFFFFF
	frac23 := frac20<<3 | lo32>>29

	if size == 4 {
		if lo32>>28&1 == 1 {
			frac23++
			if frac23 > 0x7FFFFF {
				frac23 = 0
				biasedExp++
			}

			if biasedExp > 255 {
				return 0, false, true
			}
		}

		natural := sign<<31 | biasedExp<<23 | frac23

		return uint64(wordSwap(natural)), false, false
	}

	natural := sign<<31 | biasedExp<<23 | frac23
	lowLong := wordSwap(natural)
	highLong := wordSwap(lo32 << 3)

	return uint64(lowLong) | uint64(highLong)<<32, false, false
}

// fpuLoad converts VAX F_floating (size 4) or D_floating (size 8) bits (in
// the same low/high-longword layout fpuStore produces) to a float64.
//
// Matches fpu_load's algorithm: a stored exponent field of zero means either
// 0.0 (every other bit clear) or, if the sign bit is set, a reserved-operand
// fault -- the VAX architecture's "negative zero is a reserved encoding
// unless every other bit is also zero" rule.
func fpuLoad(raw uint64, size int) (float64, error) {
	lowLong := wordSwap(uint32(raw))

	sign := lowLong >> 31
	biasedExp := lowLong >> 23 & 0xFF
	frac23 := lowLong & 0x7FFFFF

	if biasedExp == 0 {
		if lowLong&0x7FFFFFFF == 0 {
			return 0, nil
		}

		return 0, &Fault{Code: ExcReservedOp}
	}

	ieeeExp := uint32(int(biasedExp) - 129 + 1023)
	hi32 := sign<<31 | ieeeExp<<20 | frac23>>3

	var lo32 uint32

	if size == 8 {
		highLong := wordSwap(uint32(raw >> 32))
		lo32 = frac23&0x7<<29 | highLong>>3
	} else {
		lo32 = frac23 & 0x7 << 29
	}

	return math.Float64frombits(uint64(hi32)<<32 | uint64(lo32)), nil
}

// loadFloat reads op's value as a float64: an immediate (short-literal)
// operand already carries pre-converted IEEE double bits (decodeOperand's
// ShortLiteralFloat handling, internal/cpu/operand.go), while a register or
// memory operand carries real VAX F_floating/D_floating bits needing
// fpuLoad. See docs/PHASE-05.md's design notes.
func loadFloat(cpu *vax.CPU, mem *vm.Memory, op Operand) (float64, error) {
	raw, err := op.Load(cpu, mem)
	if err != nil {
		return 0, err
	}

	if op.Kind == OperandImmediate {
		return math.Float64frombits(raw), nil
	}

	return fpuLoad(raw, op.Size)
}

// storeFloat converts value to op's VAX F_floating/D_floating representation
// and stores it. Destinations are never immediate (decode already rejects a
// write to a short literal as a reserved-addressing-mode fault), so unlike
// loadFloat there is no short-literal case to special-case here.
func storeFloat(cpu *vax.CPU, mem *vm.Memory, op Operand, value float64) error {
	raw, err := fpuStore(cpu, op.Size, value)
	if err != nil {
		return err
	}
	return op.Store(cpu, mem, raw)
}
