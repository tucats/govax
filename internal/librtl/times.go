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
// The 5.0 manual says "delta times must be less than 10,000 days" (the
// limit $ASCTIM and $NUMTIM keep), but VMS V7.1's routines take longer
// ones and make them (testdata/lib56): a delta's only limit is the
// quadword's, and a result past it is LIB$_IVTIME. The routines return
// LIB$_NORMAL, not SS$_NORMAL, as their entries list it. On an error
// nothing is stored.

var (
	libIvTime     = vmsdef.LibrarySymbols["LIB$_IVTIME"]
	libOneDelTim  = vmsdef.LibrarySymbols["LIB$_ONEDELTIM"]
	libNegTim     = vmsdef.LibrarySymbols["LIB$_NEGTIM"]
	libInvArgOrd  = vmsdef.LibrarySymbols["LIB$_INVARGORD"]
	ticksPerDay   = int64(86400 * vmsdef.TicksPerSecond)
	maxDeltaTicks = int64(math.MaxInt64) // the first delta too long to store
)

// isDelta reports whether the quadword time v is a delta time.
func isDelta(v uint64) bool { return int64(v) < 0 }

// deltaLength is the length in ticks of the delta time v: its magnitude.
// ok is false for the most negative quadword, which has no positive
// twin.
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
// of LIB$ADD_TIMES and LIB$SUB_TIMES. ok is false for a delta with no
// length (deltaLength).
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
// Two absolute times are LIB$_ONEDELTIM. A sum too big for a quadword
// wraps, as on VMS V7.1
// (an absolute time past the year 9999 is accepted; unconfirmed). Any
// count of arguments but three is LIB$_WRONUMARG.
//
//	LIB$ADD_TIMES time1 ,time2 ,resultant-time
func libAddTimes(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) != 3 {
		return libWroNumArg, nil
	}

	t1, t2, _, err := loadTimes(env, argv)
	if err != nil {
		return memoryFault(env, err)
	}

	var sum uint64

	// The sums are quadword arithmetic, wrapping as VMS V7.1's do (two
	// deltas of 2^62 ticks make the most negative quadword:
	// testdata/lib56).
	switch {
	case !isDelta(t1) && !isDelta(t2):
		return libOneDelTim, nil

	case isDelta(t1) && isDelta(t2):
		// Stored negated, two deltas add as they are.
		sum = t1 + t2

	default:
		// A delta, stored negated, moves an absolute time later by its
		// length: the absolute time minus the stored delta.
		absolute, delta := t1, t2
		if isDelta(t1) {
			absolute, delta = t2, t1
		}

		sum = absolute - delta
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
// be zero"; VMS V7.1 agrees). Two absolute times any distance apart make
// a delta, 10,000 days or more included. Any count of arguments but three
// is LIB$_WRONUMARG.
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
// ticks. ok is false for an absolute time (zero included): LIB$_IVTIME.
func loadDelta(env *corevms.Environment, argv []uint32) (ticks int64, ok bool, err error) {
	v, err := env.Memory().LoadQuadword(env.CPU(), arg(argv, 1))
	if err != nil || !isDelta(v) {
		return 0, false, err
	}

	ticks, ok = deltaLength(v)

	return ticks, ok, nil
}

// libMultDeltaTime is LIB$MULT_DELTA_TIME (LIB-297): the delta time
// multiplied, in place, by the absolute value of a longword. A multiplier
// of 0 is LIB$_IVTIME, as on VMS (a zero "delta" would be the base date),
// and so is a product too big for a quadword. Any count of arguments but
// two is LIB$_WRONUMARG.
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

	if multiplier == 0 || ticks > (maxDeltaTicks-1)/multiplier {
		return libIvTime, nil
	}

	product, _ := deltaTime(ticks * multiplier)

	return storeTime(env, arg(argv, 1), product)
}

// libMultfDeltaTime is LIB$MULTF_DELTA_TIME (LIB-298): LIB$MULT_DELTA_TIME
// with an F-floating multiplier. The product is exact, then made a whole
// number of ticks by rounding its length up, as VMS V7.1 does
// (testdata/lib56: one tick times 0.5, 1.4, 1.5, and 2.5 is one, two,
// two, and three ticks, and an hour times F_floating's 1/3 is
// 12,000,000,358 ticks, not ...357.6). A multiplier of 0 is LIB$_IVTIME.
// A reserved operand signals SS$_ROPRAND, as on VMS.
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
	case !ok, m.Sign() == 0:
		return libIvTime, nil
	}

	length, ok := ceilTicks(new(big.Rat).Mul(m.Abs().Rat(), new(big.Rat).SetInt64(ticks)))
	if !ok {
		return libIvTime, nil
	}

	product, ok := deltaTime(length)
	if !ok {
		return libIvTime, nil
	}

	return storeTime(env, arg(argv, 1), product)
}

// ceilTicks rounds the nonnegative exact value r up to a whole number of
// ticks. ok is false when the result is too big for a quadword, so that
// a huge product is refused before it is made an int64.
func ceilTicks(r *big.Rat) (ticks int64, ok bool) {
	q, rem := new(big.Int).QuoRem(r.Num(), r.Denom(), new(big.Int))
	if rem.Sign() > 0 {
		q.Add(q, big.NewInt(1))
	}

	if q.Cmp(big.NewInt(maxDeltaTicks)) >= 0 {
		return 0, false
	}

	return q.Int64(), true
}
