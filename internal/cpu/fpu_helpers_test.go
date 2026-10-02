package cpu

import (
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vaxfloat"
)

// Test helpers from before Phase 35's floating core, kept so the F and D
// tests can still give values as float64s: fpuStore returns value's
// F_floating (size 4) or D_floating (size 8) bits as the CPU stores them,
// and fpuLoad reads them back. They now go through internal/vaxfloat.

func sizeFloatFormat(size int) vaxfloat.Format {
	if size == 8 {
		return vaxfloat.D
	}

	return vaxfloat.F
}

func fpuStore(cpu *vax.CPU, size int, value float64) (uint64, error) {
	bits, err := vaxfloat.Pack(sizeFloatFormat(size), vaxfloat.FromFloat64(value))
	if err != nil {
		if err = (&Engine{cpu: cpu}).floatException(err); err != nil {
			return 0, err
		}
	}

	return bits.Lo, nil
}

func fpuLoad(raw uint64, size int) (float64, error) {
	v, err := vaxfloat.Unpack(sizeFloatFormat(size), vaxfloat.Bits{Lo: raw})
	if err != nil {
		return 0, &Fault{Code: ExcReservedOp}
	}

	return v.Float64(), nil
}
