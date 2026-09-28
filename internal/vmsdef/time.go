package vmsdef

import "time"

// VMS 64-bit system time counts 100-nanosecond units since the VMS base
// date, 17-Nov-1858 00:00. Unlike Unix time, it is a *local* time: VMS
// keeps the system clock set to the wall-clock time where the machine is,
// with no time zone attached, so $GETTIM followed by $ASCTIM prints the
// time of day a user there would read off a clock. govax follows VMS
// (docs/PHASE-26.md subtask 12): Time converts using t's own zone offset.

// UnixEpoch is the Unix epoch (1-Jan-1970 00:00) in VMS 64-bit system
// time: the 40587 days between it and the VMS base date.
const UnixEpoch = 40587 * 86400 * 10_000_000

// TicksPerSecond is one second in VMS time units (100ns).
const TicksPerSecond = 10_000_000

// Time converts a Go time to VMS 64-bit system time, as the wall-clock
// reading in t's own location. For time.Now() that is the host's local
// time, as on VMS; a UTC time converts to the same reading in UTC.
func Time(t time.Time) uint64 {
	_, offset := t.Zone() // seconds east of UTC

	return uint64(t.UnixNano()/100) + uint64(int64(offset)*TicksPerSecond) + UnixEpoch
}

// GoTime is the inverse of Time for display: the wall-clock reading v
// stands for, as a Go time in UTC (UTC only because it has no offset of
// its own, so the fields — year, day, hour — are exactly v's). v is
// treated as an absolute time; a delta time (negative as a signed
// quadword) has no calendar reading.
func GoTime(v uint64) time.Time {
	ticks := int64(v - UnixEpoch) // may go negative: before 1970

	return time.Unix(0, 0).UTC().Add(time.Duration(ticks) * 100)
}
