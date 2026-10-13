package librtl

import (
	"math"
	"math/big"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmsdef"
)

// The time arithmetic routines (docs/PHASE-56, subtask 2): LIB$ADD_TIMES,
// LIB$SUB_TIMES, LIB$MULT_DELTA_TIME, and LIB$MULTF_DELTA_TIME.
//
// A VMS time is a quadword count of 100-nanosecond "ticks". A positive
// value (or zero, the base date) is an absolute time: ticks since
// 17-Nov-1858 00:00. A negative value is a delta time, an interval,
// stored negated so that its sign alone tells the two kinds apart. So a
// delta of one second is -10,000,000, and adding it to an absolute time
// moves that time one second later.
//
// The manual says "delta times must be less than 10,000 days" (the same
// limit $ASCTIM and $NUMTIM keep), and a delta at or past it, given or
// computed, is LIB$_IVTIME. The routines return LIB$_NORMAL, not
// SS$_NORMAL, as their entries list it. On an error nothing is stored
// (unconfirmed).

var (
	libIvTime     = vmsdef.LibrarySymbols["LIB$_IVTIME"]
	libOneDelTim  = vmsdef.LibrarySymbols["LIB$_ONEDELTIM"]
	libNegTim     = vmsdef.LibrarySymbols["LIB$_NEGTIM"]
	libInvArgOrd  = vmsdef.LibrarySymbols["LIB$_INVARGORD"]
	ticksPerDay   = int64(86400 * vmsdef.TicksPerSecond)
	maxDeltaTicks = 10_000 * ticksPerDay // the first delta that is too long
)

// isDelta reports whether the quadword time v is a delta time.
func isDelta(v uint64) bool { return int64(v) < 0 }

// deltaLength is the length in ticks of the delta time v: its magnitude.
// ok is false for a delta of 10,000 days or more. The most negative
// quadword has no positive twin, but it is far past the limit, so it
// never needs one.
func deltaLength(v uint64) (ticks int64, ok bool) {
	if int64(v) == math.MinInt64 {
		return 0, false
	}

	ticks = -int64(v)

	return ticks, ticks < maxDeltaTicks
}

// deltaTime is the quadword delta time of length ticks (stored negated),
// ok false when the delta is too long to be one.
func deltaTime(ticks int64) (v uint64, ok bool) {
	return uint64(-ticks), ticks < maxDeltaTicks
}

// loadTimes reads the quadword times at argv[0] and argv[1], the inputs
// of LIB$ADD_TIMES and LIB$SUB_TIMES. A delta of 10,000 days or more is
// reported as ok false.
func loadTimes(env *corevms.Environment, argv []uint32) (t1, t2 uint64, ok bool, err error) {
	mem, c := env.Memory(), env.CPU()

	if t1, err = mem.LoadQuadword(c, arg(argv, 0)); err != nil {
		return 0, 0, false, err
	}

	if t2, err = mem.LoadQuadword(c, arg(argv, 1)); err != nil {
		return 0, 0, false, err
	}

	for _, t := range []uint64{t1, t2} {
		if isDelta(t) {
			if _, ok := deltaLength(t); !ok {
				return t1, t2, false, nil
			}
		}
	}

	return t1, t2, true, nil
}

// storeTime writes the quadword time v at addr, returning the routine's
// result: LIB$_NORMAL, or the signaled access violation of an address
// that can't be written.
func storeTime(env *corevms.Environment, addr uint32, v uint64) (uint32, error) {
	if err := env.Memory().StoreQuadword(env.CPU(), addr, v); err != nil {
		return memoryFault(env, writing(err))
	}

	return libNormal, nil
}

// libAddTimes is LIB$ADD_TIMES (LIB-5): the sum of two times, at least
// one of them a delta. Two deltas add to a delta. A delta added to an
// absolute time, in either order, is the absolute time that much later.
// Two absolute times are LIB$_ONEDELTIM. A delta sum of 10,000 days or
// more is LIB$_IVTIME, and so is an absolute sum too big for a quadword
// (an absolute time past the year 9999 is accepted; unconfirmed). Any
// count of arguments but three is LIB$_WRONUMARG.
//
//	LIB$ADD_TIMES time1 ,time2 ,resultant-time
func libAddTimes(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) != 3 {
		return libWroNumArg, nil
	}

	t1, t2, ok, err := loadTimes(env, argv)

	switch {
	case err != nil:
		return memoryFault(env, err)
	case !ok:
		return libIvTime, nil
	}

	var sum uint64

	switch {
	case !isDelta(t1) && !isDelta(t2):
		return libOneDelTim, nil

	case isDelta(t1) && isDelta(t2):
		// Stored negated, two deltas add as they are; the length of the
		// sum is what must stay under the limit. Each is under 10,000
		// days, so the sum can't overflow.
		if sum, ok = deltaTime(-int64(t1) - int64(t2)); !ok {
			return libIvTime, nil
		}

	default:
		absolute, delta := t1, t2
		if isDelta(t1) {
			absolute, delta = t2, t1
		}

		length, _ := deltaLength(delta)
		if int64(absolute) > math.MaxInt64-length {
			return libIvTime, nil
		}

		sum = absolute + uint64(length)
	}

	return storeTime(env, arg(argv, 2), sum)
}

// libSubTimes is LIB$SUB_TIMES (LIB-397): time1 minus time2, in one of
// three ways:
//
//   - delta - delta is a delta, time1 the longer interval;
//   - absolute - absolute is a delta, time1 the later time;
//   - absolute - delta is the absolute time that much earlier (the
//     manual's table says only "time1-time2"; unconfirmed).
//
// A delta time1 with an absolute time2 is LIB$_INVARGORD. A negative
// result (time1 the shorter interval or the earlier time, or an absolute
// time before the base date) is LIB$_NEGTIM. Equal times give the
// smallest delta, one tick, as the VSI manual says ("a delta time can't
// be zero"; the 5.0 manual leaves equal times open; unconfirmed). A delta
// result of 10,000 days or more is LIB$_IVTIME (unconfirmed for two
// absolute times). Any count of arguments but three is LIB$_WRONUMARG.
//
//	LIB$SUB_TIMES time1 ,time2 ,resultant-time
func libSubTimes(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) != 3 {
		return libWroNumArg, nil
	}

	t1, t2, ok, err := loadTimes(env, argv)

	switch {
	case err != nil:
		return memoryFault(env, err)
	case !ok:
		return libIvTime, nil
	case isDelta(t1) && !isDelta(t2):
		return libInvArgOrd, nil
	}

	var difference uint64

	if !isDelta(t1) && isDelta(t2) {
		length, _ := deltaLength(t2)
		if int64(t1) < length {
			return libNegTim, nil
		}

		difference = t1 - uint64(length)
	} else {
		// Two of a kind, the result a delta. For absolute times the
		// interval is t1 - t2; for deltas (stored negated) it's the
		// lengths' difference, t2 - t1. Neither can overflow: absolute
		// times are positive, and the deltas are under the limit.
		ticks := int64(t1) - int64(t2)
		if isDelta(t1) {
			ticks = -ticks
		}

		switch {
		case ticks < 0:
			return libNegTim, nil
		case ticks == 0:
			ticks = 1
		}

		if difference, ok = deltaTime(ticks); !ok {
			return libIvTime, nil
		}
	}

	return storeTime(env, arg(argv, 2), difference)
}

// loadDelta reads the delta time LIB$MULT_DELTA_TIME and
// LIB$MULTF_DELTA_TIME multiply, at argv[1], returning its length in
// ticks. ok is false for an absolute time (zero included) or a delta of
// 10,000 days or more: LIB$_IVTIME (unconfirmed for an absolute time).
func loadDelta(env *corevms.Environment, argv []uint32) (ticks int64, ok bool, err error) {
	v, err := env.Memory().LoadQuadword(env.CPU(), arg(argv, 1))
	if err != nil || !isDelta(v) {
		return 0, false, err
	}

	ticks, ok = deltaLength(v)

	return ticks, ok, nil
}

// libMultDeltaTime is LIB$MULT_DELTA_TIME (LIB-297): the delta time
// multiplied, in place, by the absolute value of a longword. A product of
// 10,000 days or more is LIB$_IVTIME. A multiplier of 0 stores 0, which
// as a time is the base date rather than a delta (unconfirmed: VMS may
// refuse it). Any count of arguments but two is LIB$_WRONUMARG.
//
//	LIB$MULT_DELTA_TIME multiplier ,delta-time
func libMultDeltaTime(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) != 2 {
		return libWroNumArg, nil
	}

	m, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	ticks, ok, err := loadDelta(env, argv)

	switch {
	case err != nil:
		return memoryFault(env, err)
	case !ok:
		return libIvTime, nil
	}

	// The multiplier's absolute value, in 64 bits so that the most
	// negative longword has one. ticks is under 2^53 and the multiplier
	// at most 2^31, so the test against the limit is made by division,
	// before the product could overflow.
	multiplier := int64(int32(m))
	if multiplier < 0 {
		multiplier = -multiplier
	}

	if multiplier != 0 && ticks > (maxDeltaTicks-1)/multiplier {
		return libIvTime, nil
	}

	product, _ := deltaTime(ticks * multiplier)

	return storeTime(env, arg(argv, 1), product)
}

// libMultfDeltaTime is LIB$MULTF_DELTA_TIME (LIB-298): LIB$MULT_DELTA_TIME
// with an F-floating multiplier. The product is exact, then rounded to a
// whole tick, half away from zero (vaxfloat's rule; unconfirmed: VMS may
// truncate). A reserved operand signals SS$_ROPRAND, as the floating
// multiply reading it would (LIB$WAIT's rule on VMS; unconfirmed here).
//
//	LIB$MULTF_DELTA_TIME multiplier ,delta-time
func libMultfDeltaTime(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) != 2 {
		return libWroNumArg, nil
	}

	m, err := loadFloat(env, vaxfloat.F, arg(argv, 0))
	if err != nil {
		// emodFailure's rule: a reserved operand is signaled.
		return emodFailure(env, err)
	}

	ticks, ok, err := loadDelta(env, argv)

	switch {
	case err != nil:
		return memoryFault(env, err)
	case !ok:
		return libIvTime, nil
	}

	length, ok := roundTicks(new(big.Rat).Mul(m.Abs().Rat(), new(big.Rat).SetInt64(ticks)))
	if !ok {
		return libIvTime, nil
	}

	product, ok := deltaTime(length)
	if !ok {
		return libIvTime, nil
	}

	return storeTime(env, arg(argv, 1), product)
}

// roundTicks rounds the nonnegative exact value r to a whole number of
// ticks, half away from zero. ok is false when the result is 10,000 days
// or more, so that a huge product is refused before it is made an int64.
func roundTicks(r *big.Rat) (ticks int64, ok bool) {
	q, rem := new(big.Int).QuoRem(r.Num(), r.Denom(), new(big.Int))

	// Half or more of the next tick rounds up.
	if rem.Lsh(rem, 1).Cmp(r.Denom()) >= 0 {
		q.Add(q, big.NewInt(1))
	}

	if q.Cmp(big.NewInt(maxDeltaTicks)) >= 0 {
		return 0, false
	}

	return q.Int64(), true
}
