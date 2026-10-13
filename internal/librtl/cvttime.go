package librtl

import (
	"math/big"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmsdef"
)

// The time conversion routines (docs/PHASE-56, subtask 3):
// LIB$CVT_FROM_INTERNAL_TIME and LIB$CVTF_FROM_INTERNAL_TIME, which turn a
// VMS time into a number of some unit, LIB$CVT_TO_INTERNAL_TIME and
// LIB$CVTF_TO_INTERNAL_TIME, which turn a number of units into a delta
// time, and LIB$CVT_VECTIM, which turns $NUMTIM's seven words back into
// a time.
//
// Each conversion names its unit by an operation code, LIB$K_xxx. Codes
// 1 to 20 measure an absolute time (its month of the year, its second of
// the minute, ...), codes 21 to 25 a delta time in whole units, and 26
// to 30 a delta time in F-floating units. Each routine accepts only its
// own group; any other code is LIB$_INVOPER. Like the arithmetic, they
// return LIB$_NORMAL, refuse a delta of 10,000 days or more
// (LIB$_IVTIME), and store nothing on an error.

var (
	libAbsTimReq = vmsdef.LibrarySymbols["LIB$_ABSTIMREQ"]
	libDelTimReq = vmsdef.LibrarySymbols["LIB$_DELTIMREQ"]
	libInvOper   = vmsdef.LibrarySymbols["LIB$_INVOPER"]
)

// calendar is an absolute time broken into the fields the absolute
// operations count from.
type calendar struct {
	month   int // 1-12
	yearDay int // 1-366
	day     int // of the month, 1-31
	weekDay int // 1 (Monday) to 7 (Sunday)
	seconds int // since midnight
	days    int // since the base date, 17-Nov-1858
}

// calendarOf breaks the absolute time v into a calendar.
func calendarOf(v uint64) calendar {
	t := vmsdef.GoTime(v)

	// Go counts the week from Sunday (0); VMS from Monday (1).
	weekDay := int(t.Weekday())
	if weekDay == 0 {
		weekDay = 7
	}

	return calendar{
		month:   int(t.Month()),
		yearDay: t.YearDay(),
		day:     t.Day(),
		weekDay: weekDay,
		seconds: t.Hour()*3600 + t.Minute()*60 + t.Second(),
		days:    int(v / uint64(ticksPerDay)),
	}
}

// within is the number of whole units of the given length (in seconds)
// that have passed in the period of a year, a month, or a week, counted
// from 1: the period began dayIndex days ago (0 on its first day), plus
// the part of today that has passed. So the first hour of the year is
// hour 1.
func (c calendar) within(dayIndex, unit int) uint32 {
	return uint32(dayIndex*(86400/unit) + c.seconds/unit + 1)
}

// ofDay is the number of whole units of the given length (in seconds)
// that have passed in the current period (a day, an hour, or a minute)
// of the given length, counted from 0: at 09:13:25 the minute of the
// hour is 13. The 5.0 manual's ranges for these operations start at 1,
// the VSI manual's at 0; govax follows VSI (unconfirmed).
func (c calendar) ofDay(period, unit int) uint32 {
	return uint32(c.seconds % period / unit)
}

// timeOperation is one LIB$K_ operation code's rule. An absolute
// operation has a field, the number it takes from a calendar; a delta
// operation has a unit, its length in ticks, and is floating or not.
type timeOperation struct {
	field    func(c calendar) uint32
	unit     int64
	floating bool
}

// timeOperations are the operation codes, by their values in VMS's
// definitions (LIB$K_MONTH_OF_YEAR is 1, ..., LIB$K_DELTA_SECONDS_F 30).
// For the year, month, and week, the counts start at 1: 00:30 on
// 1 January is hour 1 of the year (unconfirmed). LIB$K_JULIAN_DATE is
// the number of days since the base date, as the manual's footnote says
// (unconfirmed: "Julian date" also means the day of the year).
var timeOperations = map[uint32]timeOperation{
	1:  {field: func(c calendar) uint32 { return uint32(c.month) }},
	2:  {field: func(c calendar) uint32 { return uint32(c.yearDay) }},
	3:  {field: func(c calendar) uint32 { return c.within(c.yearDay-1, 3600) }},
	4:  {field: func(c calendar) uint32 { return c.within(c.yearDay-1, 60) }},
	5:  {field: func(c calendar) uint32 { return c.within(c.yearDay-1, 1) }},
	6:  {field: func(c calendar) uint32 { return uint32(c.day) }},
	7:  {field: func(c calendar) uint32 { return c.within(c.day-1, 3600) }},
	8:  {field: func(c calendar) uint32 { return c.within(c.day-1, 60) }},
	9:  {field: func(c calendar) uint32 { return c.within(c.day-1, 1) }},
	10: {field: func(c calendar) uint32 { return uint32(c.weekDay) }},
	11: {field: func(c calendar) uint32 { return c.within(c.weekDay-1, 3600) }},
	12: {field: func(c calendar) uint32 { return c.within(c.weekDay-1, 60) }},
	13: {field: func(c calendar) uint32 { return c.within(c.weekDay-1, 1) }},
	14: {field: func(c calendar) uint32 { return c.ofDay(86400, 3600) }},
	15: {field: func(c calendar) uint32 { return c.ofDay(86400, 60) }},
	16: {field: func(c calendar) uint32 { return c.ofDay(86400, 1) }},
	17: {field: func(c calendar) uint32 { return c.ofDay(3600, 60) }},
	18: {field: func(c calendar) uint32 { return c.ofDay(3600, 1) }},
	19: {field: func(c calendar) uint32 { return c.ofDay(60, 1) }},
	20: {field: func(c calendar) uint32 { return uint32(c.days) }},
	21: {unit: 7 * ticksPerDay},
	22: {unit: ticksPerDay},
	23: {unit: ticksPerDay / 24},
	24: {unit: vmsdef.TicksPerSecond * 60},
	25: {unit: vmsdef.TicksPerSecond},
	26: {unit: 7 * ticksPerDay, floating: true},
	27: {unit: ticksPerDay, floating: true},
	28: {unit: ticksPerDay / 24, floating: true},
	29: {unit: vmsdef.TicksPerSecond * 60, floating: true},
	30: {unit: vmsdef.TicksPerSecond, floating: true},
}

// operationKind tells which routines accept an operation.
type operationKind int

const (
	absoluteOperation operationKind = iota // 1-20
	deltaOperation                         // 21-25
	floatingOperation                      // 26-30
)

// loadOperation reads the operation code at argv[0] and finds its rule.
// ok is false for a code that isn't one of the kinds the routine accepts
// (LIB$_INVOPER).
func loadOperation(env *corevms.Environment, argv []uint32, kinds ...operationKind) (op timeOperation, ok bool, err error) {
	code, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 0))
	if err != nil {
		return op, false, err
	}

	op, ok = timeOperations[code]
	if !ok {
		return op, false, nil
	}

	kind := absoluteOperation

	switch {
	case op.floating:
		kind = floatingOperation
	case op.field == nil:
		kind = deltaOperation
	}

	for _, k := range kinds {
		if k == kind {
			return op, true, nil
		}
	}

	return op, false, nil
}

// libCvtFromInternalTime is LIB$CVT_FROM_INTERNAL_TIME (LIB-67): one
// field of a time, as a longword. An absolute operation (1-20) measures
// an absolute time (LIB$_ABSTIMREQ for a delta), and a delta operation
// (21-25) counts the whole units in a delta time, truncated
// (LIB$_DELTIMREQ for an absolute time). The floating operations are
// LIB$CVTF_FROM_INTERNAL_TIME's, LIB$_INVOPER here (unconfirmed). With
// input-time omitted, or 0 by value (unconfirmed), the time is the
// current time, read once. Two or three arguments, else LIB$_WRONUMARG.
//
//	LIB$CVT_FROM_INTERNAL_TIME operation ,resultant-time [,input-time]
func libCvtFromInternalTime(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) != 2 && len(argv) != 3 {
		return libWroNumArg, nil
	}

	op, ok, err := loadOperation(env, argv, absoluteOperation, deltaOperation)

	switch {
	case err != nil:
		return memoryFault(env, err)
	case !ok:
		return libInvOper, nil
	}

	v := env.Clock()

	if addr := arg(argv, 2); addr != 0 {
		if v, err = env.Memory().LoadQuadword(env.CPU(), addr); err != nil {
			return memoryFault(env, err)
		}
	}

	var result uint32

	if op.field != nil {
		if isDelta(v) {
			return libAbsTimReq, nil
		}

		result = op.field(calendarOf(v))
	} else {
		if !isDelta(v) {
			return libDelTimReq, nil
		}

		ticks, ok := deltaLength(v)
		if !ok {
			return libIvTime, nil
		}

		result = uint32(ticks / op.unit)
	}

	if err := env.Memory().StoreLongword(env.CPU(), arg(argv, 1), result); err != nil {
		return memoryFault(env, writing(err))
	}

	return libNormal, nil
}

// libCvtfFromInternalTime is LIB$CVTF_FROM_INTERNAL_TIME (LIB-70): a delta
// time as an F-floating number of units (operations 26-30, others
// LIB$_INVOPER): its exact length divided by the unit, rounded once to
// F_floating. input-time is required, and an absolute time is
// LIB$_DELTIMREQ. Exactly three arguments, else LIB$_WRONUMARG.
//
//	LIB$CVTF_FROM_INTERNAL_TIME operation ,resultant-time ,input-time
func libCvtfFromInternalTime(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) != 3 {
		return libWroNumArg, nil
	}

	op, ok, err := loadOperation(env, argv, floatingOperation)

	switch {
	case err != nil:
		return memoryFault(env, err)
	case !ok:
		return libInvOper, nil
	}

	v, err := env.Memory().LoadQuadword(env.CPU(), arg(argv, 2))

	switch {
	case err != nil:
		return memoryFault(env, err)
	case !isDelta(v):
		return libDelTimReq, nil
	}

	ticks, ok := deltaLength(v)
	if !ok {
		return libIvTime, nil
	}

	// At most 10,000 days of weeks or seconds: far inside F_floating's
	// range either way, so rounding can't fail.
	result, _ := vaxfloat.Round(vaxfloat.F, vaxfloat.FromRat(big.NewRat(ticks, op.unit)))

	if err := storeFloat(env, vaxfloat.F, arg(argv, 1), result); err != nil {
		return memoryFault(env, err)
	}

	return libNormal, nil
}

// libCvtToInternalTime is LIB$CVT_TO_INTERNAL_TIME (LIB-72): a delta time
// of a whole number of units (operations 21-25, others LIB$_INVOPER),
// given as a signed longword. A negative count, or one of 10,000 days or
// more, is LIB$_IVTIME. A count of 0 stores 0, as the 5.0 manual allows
// it (unconfirmed: the VSI manual asks for more than 0). Exactly three
// arguments, else LIB$_WRONUMARG.
//
//	LIB$CVT_TO_INTERNAL_TIME operation ,input-time ,resultant-time
func libCvtToInternalTime(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) != 3 {
		return libWroNumArg, nil
	}

	op, ok, err := loadOperation(env, argv, deltaOperation)

	switch {
	case err != nil:
		return memoryFault(env, err)
	case !ok:
		return libInvOper, nil
	}

	n, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 1))
	if err != nil {
		return memoryFault(env, err)
	}

	// Tested by division, before the product could overflow.
	count := int64(int32(n))
	if count < 0 || count > (maxDeltaTicks-1)/op.unit {
		return libIvTime, nil
	}

	v, _ := deltaTime(count * op.unit)

	return storeTime(env, arg(argv, 2), v)
}

// libCvtfToInternalTime is LIB$CVTF_TO_INTERNAL_TIME (LIB-74):
// LIB$CVT_TO_INTERNAL_TIME with an F-floating number of units
// (operations 26-30). The length is rounded to a whole tick, half away
// from zero (unconfirmed), as LIB$MULTF_DELTA_TIME's product is, and a
// reserved operand signals SS$_ROPRAND as there.
//
//	LIB$CVTF_TO_INTERNAL_TIME operation ,input-time ,resultant-time
func libCvtfToInternalTime(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) != 3 {
		return libWroNumArg, nil
	}

	op, ok, err := loadOperation(env, argv, floatingOperation)

	switch {
	case err != nil:
		return memoryFault(env, err)
	case !ok:
		return libInvOper, nil
	}

	units, err := loadFloat(env, vaxfloat.F, arg(argv, 1))
	if err != nil {
		return emodFailure(env, err)
	}

	if units.Sign() < 0 {
		return libIvTime, nil
	}

	ticks, ok := roundTicks(new(big.Rat).Mul(units.Rat(), new(big.Rat).SetInt64(op.unit)))
	if !ok {
		return libIvTime, nil
	}

	v, _ := deltaTime(ticks)

	return storeTime(env, arg(argv, 2), v)
}

// libCvtVectim is LIB$CVT_VECTIM (LIB-78): the time that a seven-word
// vector in $NUMTIM's layout (year, month, day, hour, minute, second,
// hundredths) stands for. A year and month both 0 make a delta time;
// anything else is absolute. A field out of range is LIB$_IVTIME (the
// limits are corevms.VectorTime's). Exactly two arguments, else
// LIB$_WRONUMARG.
//
//	LIB$CVT_VECTIM input-time ,resultant-time
func libCvtVectim(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) != 2 {
		return libWroNumArg, nil
	}

	var fields [7]uint16

	mem, c := env.Memory(), env.CPU()
	for i := range fields {
		w, err := mem.LoadWord(c, arg(argv, 0)+uint32(2*i))
		if err != nil {
			return memoryFault(env, err)
		}

		fields[i] = w
	}

	v, ok := corevms.VectorTime(fields)
	if !ok {
		return libIvTime, nil
	}

	return storeTime(env, arg(argv, 1), v)
}

