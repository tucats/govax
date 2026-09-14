package rtl

import "time"

// Port of librtl_time.c.

// shimDeccTime is DECC$TIME (shim code 32): the C library time() call —
// seconds since the Unix epoch, optionally also stored at argv[0].
func shimDeccTime(env *Environment, argv []uint32) (uint32, error) {
	now := uint32(time.Now().Unix())

	if len(argv) >= 1 && argv[0] != 0 {
		if err := env.mem.StoreLongword(env.cpu, argv[0], now); err != nil {
			return 0, err
		}
	}
	return now, nil
}

func registerTimeShims(t *ShimTable) {
	t.Register(32, "DECC$TIME", shimDeccTime)
}
