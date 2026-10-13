package librtl

import (
	"math"

	"github.com/tucats/govax/internal/corevms"
)

// The simple date routines (docs/PHASE-56, subtask 4): LIB$DATE_TIME, the
// current time as text, and LIB$DAY and LIB$DAY_OF_WEEK, a time's day
// number and day of the week. Unlike the arithmetic and the conversions,
// they return SS$_NORMAL, as their entries list it.
//
// LIB$DAY and LIB$DAY_OF_WEEK take an optional time by reference. When it
// is omitted, or 0 by value (the argument's address is 0), they use the
// current time; the address of a zero quadword is the base date itself,
// 17-Nov-1858, day 0, a Wednesday. They don't check the time: a delta
// (negative) gives a negative day count.

// ticksPerHundredth is one hundredth of a second, the unit of LIB$DAY's
// day-time: 100,000 ticks of 100 ns.
const ticksPerHundredth = 100_000

// dayOverflows reports whether LIB$DAY can't count the time v
// (SS$_INTOVF). The manual puts the limit at "a date past the year 8600",
// which fits a VAX routine that divides the time by 10^9 ticks
// (100-second units) with EDIV, whose quotient must fit a longword:
// 2^31 x 10^9 ticks is 25-Dec-8663 (unconfirmed). A delta whose quotient
// is below -2^31 overflows the same way.
func dayOverflows(v uint64) bool {
	q := int64(v) / 1_000_000_000

	return q > math.MaxInt32 || q < math.MinInt32
}

// optionalTime reads the time an optional argument names: the quadword at
// addr, or the current time when addr is 0.
func optionalTime(env *corevms.Environment, addr uint32) (uint64, error) {
	if addr == 0 {
		return env.Clock(), nil
	}

	return env.Memory().LoadQuadword(env.CPU(), addr)
}

// dayNumber is the number of whole days in the time v, and the hundredths
// of a second since that day's midnight. Both are divided as EDIV divides
// (unconfirmed for a delta): truncated toward zero, so a delta's day count
// is its whole days, negated, and its day-time the rest, negated too.
func dayNumber(v uint64) (days, hundredths int64) {
	ticks := int64(v)

	return ticks / ticksPerDay, ticks % ticksPerDay / ticksPerHundredth
}

// libDateTime is LIB$DATE_TIME (LIB-80): the current date and time as
// $ASCTIM writes them, 23 characters with a blank-padded day
// (" 9-OCT-1988 10:15:43.21"), written by the output's class. A string
// too short for them is LIB$_STRTRU; a bad descriptor LIB$_INVSTRDES, and
// a dynamic string with no memory LIB$_INSVIRMEM.
//
//	LIB$DATE_TIME date-time-string
func libDateTime(env *corevms.Environment, argv []uint32) (uint32, error) {
	// The clock's times are always ones $ASCTIM can write.
	text, _ := corevms.FormatTime(env.Clock(), false)

	_, truncated, err := writeString(env, arg(argv, 0), text)

	switch {
	case err != nil:
		return stringFailure(env, err)
	case truncated:
		return libStrTru, nil
	}

	return ssNormal, nil
}

// libDay is LIB$DAY (LIB-82): the number of days since 17-Nov-1858 in a
// time, and optionally the hundredths of a second since that day's
// midnight (dayNumber). A time of 25-Dec-8663 or later is SS$_INTOVF
// (dayOverflows), with nothing stored (unconfirmed).
//
//	LIB$DAY number-of-days [,user-time] [,day-time]
func libDay(env *corevms.Environment, argv []uint32) (uint32, error) {
	v, err := optionalTime(env, arg(argv, 1))
	if err != nil {
		return memoryFault(env, err)
	}

	if dayOverflows(v) {
		return ssIntOvf, nil
	}

	days, hundredths := dayNumber(v)

	if err := env.Memory().StoreLongword(env.CPU(), arg(argv, 0), uint32(days)); err != nil {
		return memoryFault(env, writing(err))
	}

	if addr := arg(argv, 2); addr != 0 {
		if err := env.Memory().StoreLongword(env.CPU(), addr, uint32(hundredths)); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	return ssNormal, nil
}

// libDayOfWeek is LIB$DAY_OF_WEEK (LIB-84): the day of the week of a
// time, 1 for Monday to 7 for Sunday. It counts from the time's day
// number (dayNumber), day 0 being a Wednesday, so a delta's day is as
// many days before that Wednesday as the delta is long (unconfirmed).
//
//	LIB$DAY_OF_WEEK [user-time] ,day-number
func libDayOfWeek(env *corevms.Environment, argv []uint32) (uint32, error) {
	v, err := optionalTime(env, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	days, _ := dayNumber(v)

	// Wednesday is 3, so day 0 is 2 past Monday. The remainder is made
	// 0-6 for a negative day count too.
	weekDay := ((days+2)%7+7)%7 + 1

	if err := env.Memory().StoreLongword(env.CPU(), arg(argv, 1), uint32(weekDay)); err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}
