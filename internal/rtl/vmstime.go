package rtl

// The VMS time services (docs/PHASE-26.md subtasks 12-13): $GETTIM reads
// the system clock.
//
// A VMS time is a signed 64-bit count of 100-nanosecond "ticks":
//
//   - A positive value is an *absolute* time: ticks since the VMS base
//     date, 17-Nov-1858 00:00, in local time (see vmsdef.Time).
//   - A negative value is a *delta* time: an interval, such as "wait
//     10 seconds", stored negated so services like $SETIMR can tell the
//     two kinds apart by the sign bit alone.
//
// The VAX has no 64-bit integer registers, so these values always travel
// by reference: a service argument is the address of a "quadword" (two
// longwords, low-order one first) holding the time.

// loadQuad reads the quadword at addr, low-order longword first.
// ok is false if either longword can't be read.
func (env *Environment) loadQuad(addr uint32) (v uint64, ok bool) {
	lo, err := env.mem.LoadLongword(env.cpu, addr)
	if err != nil {
		return 0, false
	}

	hi, err := env.mem.LoadLongword(env.cpu, addr+4)
	if err != nil {
		return 0, false
	}

	return uint64(hi)<<32 | uint64(lo), true
}

// storeQuad writes v to the quadword at addr, low-order longword first.
// ok is false if either longword can't be written.
func (env *Environment) storeQuad(addr uint32, v uint64) (ok bool) {
	if err := env.mem.StoreLongword(env.cpu, addr, uint32(v)); err != nil {
		return false
	}

	return env.mem.StoreLongword(env.cpu, addr+4, uint32(v>>32)) == nil
}

// serviceSysGettim is SYS$GETTIM:
//
//	SYS$GETTIM timadr
//
// It stores the current system time (Environment.Clock, the same clock
// $SETIMR's timers run on) in the quadword at timadr. timadr is
// required; 0 or an unwritable address is SS$_ACCVIO.
//
// VMS updates its clock every 10ms, so real VMS returns a multiple of
// 100,000 ticks. govax's clock advances 1ms per interval-clock tick and
// returns it unrounded: a program can't depend on the 10ms granularity,
// and a finer clock only makes elapsed-time measurements more accurate.
func serviceSysGettim(env *Environment, argv []uint32) (uint32, error) {
	timadr := optArg(argv, 0)
	if timadr == 0 { // page 0 is never accessible on VMS
		return ssAccVio, nil
	}

	if !env.storeQuad(timadr, env.Clock()) {
		return ssAccVio, nil
	}

	return ssNormal, nil
}

func registerTimeServices(t *ServiceTable) {
	t.Register("SYS$GETTIM", serviceSysGettim)
}
