package rtl

// Port of librtl_math.c.

// shimLibAdawi is LIB$ADAWI (shim code 1): sum,base -> base+sum, plus the
// sign of the new base value. Matches lib_adawi's own use of native (not
// interlocked) load/store — this port doesn't model the ADAWI instruction's
// atomicity here either, same as the C source.
func shimLibAdawi(env *Environment, argv []uint32) (uint32, error) {
	sumAddr, baseAddr, signAddr := argv[0], argv[1], argv[2]

	sum, err := env.mem.LoadLongword(env.cpu, sumAddr)
	if err != nil {
		return 0, err
	}

	base, err := env.mem.LoadLongword(env.cpu, baseAddr)
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

	if err := env.mem.StoreLongword(env.cpu, signAddr, sign); err != nil {
		return 0, err
	}
	
	return 1, nil
}

func registerMathShims(t *ShimTable) {
	t.Register(1, "LIB$ADAWI", shimLibAdawi)
}
