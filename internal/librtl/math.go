package librtl

import "github.com/tucats/govax/internal/rtl"

// libAdawi is LIB$ADAWI (ported from eVAX's librtl_math.c):
//
//	LIB$ADAWI add ,sum ,sign
//
// It adds the longword at add to the longword at sum and stores the sign of
// the result (-1, 0, or 1) at sign. Like eVAX's, it doesn't model the ADAWI
// instruction's interlock, and it returns 1.
func libAdawi(env *rtl.Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()
	sumAddr, baseAddr, signAddr := arg(argv, 0), arg(argv, 1), arg(argv, 2)

	sum, err := mem.LoadLongword(cpu, sumAddr)
	if err != nil {
		return 0, err
	}

	base, err := mem.LoadLongword(cpu, baseAddr)
	if err != nil {
		return 0, err
	}

	base += sum

	var sign uint32

	switch {
	case int32(base) < 0:
		sign = 0xFFFFFFFF // -1
	case base == 0:
		sign = 0
	default:
		sign = 1
	}

	if err := mem.StoreLongword(cpu, signAddr, sign); err != nil {
		return 0, err
	}

	return 1, nil
}
