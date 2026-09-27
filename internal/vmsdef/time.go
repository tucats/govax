package vmsdef

import "time"

// UnixEpoch is the Unix epoch (1-Jan-1970 00:00 UTC) in VMS 64-bit system
// time: 100ns units since the VMS base date, 17-Nov-1858 00:00 UTC — the
// 40587 days between the two.
const UnixEpoch = 40587 * 86400 * 10_000_000

// Time converts a Go time to VMS 64-bit system time.
func Time(t time.Time) uint64 {
	return uint64(t.UnixNano()/100) + UnixEpoch
}
