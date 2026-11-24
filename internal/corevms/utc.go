package corevms

import (
	"strconv"
	"strings"
	"time"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// The UTC services (docs/planning/PHASE-113 - Fortran errata.md, subtask 8):
// $GETUTC, $ASCUTC, $BINUTC, $NUMUTC, and $TIMCON, VMS V6's companions of
// $GETTIM, $ASCTIM, $BINTIM, and $NUMTIM (vmstime.go) for times in
// Coordinated Universal Time, written from VSI's System Services Reference
// Manual.
//
// A VMS system time (a "Smithsonian" time, counted from the Smithsonian
// Institution's base date, 17-Nov-1858) is local time, and says nothing of
// where it was taken. A UTC time is 128 bits, an octaword, that carry their
// own time zone:
//
//	bytes 0-7    the time: 100-nanosecond ticks since 15-Oct-1582 00:00 UTC,
//	             the first day of the Gregorian calendar
//	bytes 8-13   its inaccuracy, 48 bits of ticks; all ones is infinite
//	bytes 14-15  the time differential factor (TDF), the local time's
//	             distance from UTC in minutes, a signed 12-bit number, and
//	             above it a 4-bit version number, 1
//
// No manual we may use gives the layout: it is the DCE time service's, whose
// form VMS took for these services, and it is unconfirmed until Phase 113's
// probe round (testdata/svc113) shows VMS's octawords.
//
// govax keeps no inaccuracy: every time it makes says it is infinite, as a
// system that runs no time service has no bound on its clock's error
// (unconfirmed). The local time zone is SYS$TIMEZONE_DIFFERENTIAL's, the
// logical name VMS V6 and later define, which govax defines from the host's
// offset (internal/lnm's timezone.go).

// UTC's numbers.
const (
	// utcBaseTicks is the ticks from UTC's base date, 15-Oct-1582, to the
	// Smithsonian base date, 17-Nov-1858: 100,840 days.
	utcBaseTicks = 100_840 * ticksPerDay

	// utcInfinite is the 48-bit inaccuracy that means "infinite".
	utcInfinite = 1<<48 - 1

	// utcVersion is the version number in the top four bits.
	utcVersion = 1

	// ticksPerMinute converts the TDF.
	ticksPerMinute = 60 * vmsdef.TicksPerSecond

	// utcSize is an octaword's bytes.
	utcSize = 16
)

// utcTime is a 128-bit UTC time taken apart.
type utcTime struct {
	ticks uint64 // since 15-Oct-1582 00:00 UTC
	inacc uint64 // 48 bits; utcInfinite is infinite
	tdf   int    // minutes east of Greenwich, -2048 to 2047
	vers  int    // the version number, 1
}

// bytes is u's octaword.
func (u utcTime) bytes() [utcSize]byte {
	var b [utcSize]byte

	for i := 0; i < 8; i++ {
		b[i] = byte(u.ticks >> (8 * i))
	}

	for i := 0; i < 6; i++ {
		b[8+i] = byte(u.inacc >> (8 * i))
	}

	w := uint16(u.vers)<<12 | uint16(u.tdf)&0xFFF
	b[14], b[15] = byte(w), byte(w>>8)

	return b
}

// utcFromBytes takes an octaword apart.
func utcFromBytes(b [utcSize]byte) utcTime {
	var u utcTime

	for i := 7; i >= 0; i-- {
		u.ticks = u.ticks<<8 | uint64(b[i])
	}

	for i := 13; i >= 8; i-- {
		u.inacc = u.inacc<<8 | uint64(b[i])
	}

	w := uint16(b[14]) | uint16(b[15])<<8
	u.vers = int(w >> 12)

	// The TDF's 12 bits, sign-extended.
	u.tdf = int(int16(w<<4) >> 4)

	return u
}

// utcFromSystem is the UTC time for the VMS system (local) time v, taken
// in a zone tdf minutes east of Greenwich. ok is false for a delta time,
// which has no place in UTC (SS$_IVTIME).
func utcFromSystem(v uint64, tdf int) (u utcTime, ok bool) {
	if int64(v) < 0 {
		return u, false
	}

	ticks := int64(v) + utcBaseTicks - int64(tdf)*ticksPerMinute

	return utcTime{ticks: uint64(ticks), inacc: utcInfinite, tdf: tdf, vers: utcVersion}, true
}

// system is the VMS system time u stands for, in u's own zone (its TDF).
// ok is false for a version other than 1 or a time that falls before the
// Smithsonian base date, which a system time can't hold (both
// unconfirmed: the manual's SS$_IVTIME says "too small to be represented
// as a Smithsonian Time, or ... not valid").
func (u utcTime) system() (v uint64, ok bool) {
	if u.vers != utcVersion || int64(u.ticks) < 0 {
		return 0, false
	}

	local := int64(u.ticks) - utcBaseTicks + int64(u.tdf)*ticksPerMinute
	if local < 0 {
		return 0, false
	}

	return uint64(local), true
}

// localTDF is the local time zone's difference from UTC in minutes:
// SYS$TIMEZONE_DIFFERENTIAL's seconds, rounded to the nearest minute, or 0
// when the name isn't defined or doesn't hold a number.
func (env *Environment) localTDF() int {
	if env.Logicals == nil {
		return 0
	}

	e, err := env.Logicals.Translate(lnm.SystemTableName, lnm.TimeZoneName, lnm.User, 0)
	if err != nil || e == nil || len(e.Equivalences) == 0 {
		return 0
	}

	seconds, err := strconv.Atoi(strings.TrimSpace(e.Equivalences[0].Value))
	if err != nil {
		return 0
	}

	if seconds < 0 {
		return -((-seconds + 30) / 60)
	}

	return (seconds + 30) / 60
}

// nowUTC is the current time in UTC, in the local zone.
func (env *Environment) nowUTC() utcTime {
	u, _ := utcFromSystem(env.Clock(), env.localTDF())

	return u
}

// loadUTC reads the octaword at addr.
func (env *Environment) loadUTC(addr uint32) (u utcTime, ok bool) {
	var b [utcSize]byte
	if addr == 0 || env.mem.Load(env.cpu, addr, b[:]) != nil {
		return u, false
	}

	return utcFromBytes(b), true
}

// storeUTC writes u's octaword at addr.
func (env *Environment) storeUTC(addr uint32, u utcTime) bool {
	if addr == 0 || !env.accessible(addr, utcSize, vm.AccessWrite) {
		return false
	}

	b := u.bytes()

	return env.mem.Store(env.cpu, addr, b[:]) == nil
}

// utcArgument is the time a service's optional utcadr argument names: the
// octaword there, or the current time when utcadr is 0.
func (env *Environment) utcArgument(utcadr uint32) (utcTime, bool) {
	if utcadr == 0 {
		return env.nowUTC(), true
	}

	return env.loadUTC(utcadr)
}

// serviceSysGetutc is SYS$GETUTC:
//
//	SYS$GETUTC utcadr
//
// It stores the current time, in UTC with the local TDF, in the octaword
// at utcadr; 0 or an unwritable address is SS$_ACCVIO.
func serviceSysGetutc(env *Environment, argv []uint32) (uint32, error) {
	if !env.storeUTC(optArg(argv, 0), env.nowUTC()) {
		return ssAccVio, nil
	}

	return ssNormal, nil
}

// serviceSysAscutc is SYS$ASCUTC:
//
//	SYS$ASCUTC [timlen] ,timbuf ,[utcadr] ,[cvtflg]
//
// $ASCTIM for a UTC time: the text of the local time the octaword at utcadr
// stands for in its own zone (the current time when utcadr is 0), with
// $ASCTIM's forms and cvtflg. A time a system time can't hold is
// SS$_IVTIME. A buffer too short for the text gets as much as fits, with
// SS$_BUFFEROVF if the date is cut; a buffer that holds the date and the
// blank after it is a success (VMS 7.1's 12 bytes; its 5 were BUFFEROVF;
// between them unconfirmed).
func serviceSysAscutc(env *Environment, argv []uint32) (uint32, error) {
	timlen, timbuf, utcadr := optArg(argv, 0), optArg(argv, 1), optArg(argv, 2)
	timeOnly := optArg(argv, 3)&1 != 0

	u, ok := env.utcArgument(utcadr)
	if !ok {
		return ssAccVio, nil
	}

	v, ok := u.system()
	if !ok {
		return ssIvTime, nil
	}

	text, ok := formatVMSTime(v, timeOnly)
	if !ok {
		return ssIvTime, nil
	}

	if timbuf == 0 {
		return ssAccVio, nil
	}

	n, truncated, err := storeDescriptor(env, timbuf, text)
	if err != nil {
		return ssAccVio, nil
	}

	if timlen != 0 {
		if err := env.mem.StoreWord(env.cpu, timlen, n); err != nil {
			return ssAccVio, nil
		}
	}

	if truncated && (timeOnly || n < ascutcDate) {
		return ssBufferOvf, nil
	}

	return ssNormal, nil
}

// ascutcDate is the date's length in $ASCUTC's text, with the blank after it.
const ascutcDate = 12

// serviceSysBinutc is SYS$BINUTC:
//
//	SYS$BINUTC timbuf ,utcadr
//
// $BINTIM for a UTC time: the text is a local absolute time, with $BINTIM's
// rules for omitted fields, made a UTC time in the local zone. A delta time
// is SS$_IVTIME, after the octaword's inaccuracy is written (infinite, as
// VMS 7.1's), and so is text $BINTIM refuses, nothing written.
func serviceSysBinutc(env *Environment, argv []uint32) (uint32, error) {
	timbuf, utcadr := optArg(argv, 0), optArg(argv, 1)
	if timbuf == 0 || utcadr == 0 {
		return ssAccVio, nil
	}

	text, fits, err := strGet(env, timbuf, maxTimeString)
	if err != nil {
		return ssAccVio, nil
	}

	if !fits {
		return ssIvTime, nil
	}

	v, ok := parseVMSTime(text, env.Clock())
	if !ok {
		return ssIvTime, nil
	}

	if int64(v) < 0 {
		infinite := []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
		if err := env.mem.Store(env.cpu, utcadr+8, infinite); err != nil {
			return ssAccVio, nil
		}

		return ssIvTime, nil
	}

	u, ok := utcFromSystem(v, env.localTDF())
	if !ok {
		return ssIvTime, nil
	}

	if !env.storeUTC(utcadr, u) {
		return ssAccVio, nil
	}

	return ssNormal, nil
}

// numutcWords is $NUMUTC's buffer: 13 words.
const numutcWords = 13

// serviceSysNumutc is SYS$NUMUTC:
//
//	SYS$NUMUTC timbuf ,[utcadr]
//
// $NUMTIM for a UTC time: 13 words at timbuf, the local time in the
// octaword's own zone (year, month, day, hour, minute, second, hundredths,
// as $NUMTIM's), then its inaccuracy (days, hours, minutes, seconds,
// hundredths; each -1 for an infinite one, the manual's "negative values"),
// then the TDF in minutes. The time is the current one when utcadr is 0.
// The layout is VMS 7.1's (testdata/svc113). An inaccuracy of 0 is days 17
// and the rest 0, as VMS's: it converts the inaccuracy as a delta time, and
// a zero quadword is the absolute time 17-NOV-1858.
func serviceSysNumutc(env *Environment, argv []uint32) (uint32, error) {
	timbuf, utcadr := optArg(argv, 0), optArg(argv, 1)

	u, ok := env.utcArgument(utcadr)
	if !ok {
		return ssAccVio, nil
	}

	v, ok := u.system()
	if !ok {
		return ssIvTime, nil
	}

	fields, ok := numericTime(v)
	if !ok {
		return ssIvTime, nil
	}

	var words [numutcWords]uint16

	copy(words[:], fields[:])

	switch u.inacc {
	case utcInfinite:
		for i := 7; i < 12; i++ {
			words[i] = 0xFFFF
		}
	case 0:
		words[7] = 17
	default:
		ticks := u.inacc
		words[7] = uint16(ticks / ticksPerDay)
		ticks %= ticksPerDay
		secs := ticks / vmsdef.TicksPerSecond
		words[8], words[9], words[10] = uint16(secs/3600), uint16(secs/60%60), uint16(secs%60)
		words[11] = uint16(ticks % vmsdef.TicksPerSecond / ticksPerHundredth)
	}

	words[12] = uint16(int16(u.tdf))

	const size = numutcWords * 2

	if timbuf == 0 || !env.accessible(timbuf, size, vm.AccessWrite) {
		return ssAccVio, nil
	}

	var data [size]byte
	for i, w := range words {
		data[2*i], data[2*i+1] = byte(w), byte(w>>8)
	}

	if err := env.mem.Store(env.cpu, timbuf, data[:]); err != nil {
		return ssAccVio, nil
	}

	return ssNormal, nil
}

// serviceSysTimcon is SYS$TIMCON:
//
//	SYS$TIMCON [smnadr] ,[utcadr] ,cvtflg
//
// With cvtflg 0 it converts the UTC octaword at utcadr to the system time
// in its own zone, stored in the quadword at smnadr; with any other cvtflg
// (VMS 7.1's 2 and 3 as its 1) the system time at smnadr to UTC in the local
// zone, stored at utcadr. A delta time, or a UTC time a system time can't
// hold, is SS$_IVTIME. An address it can't reach, 0 included, is an access
// violation signaled at that address, as VMS's (its $TIMCON reaches them
// from the caller's mode): the read first, then the write.
func serviceSysTimcon(env *Environment, argv []uint32) (uint32, error) {
	smnadr, utcadr, cvtflg := optArg(argv, 0), optArg(argv, 1), optArg(argv, 2)

	if cvtflg == 0 {
		u, ok := env.loadUTC(utcadr)
		if !ok || utcadr == 0 {
			return env.timconFault(utcadr, 0)
		}

		v, ok := u.system()
		if !ok {
			return ssIvTime, nil
		}

		if smnadr == 0 || !env.storeQuad(smnadr, v) {
			return env.timconFault(smnadr, uint32(vm.MaskWrite))
		}

		return ssNormal, nil
	}

	v, ok := env.loadQuad(smnadr)
	if !ok || smnadr == 0 {
		return env.timconFault(smnadr, 0)
	}

	u, ok := utcFromSystem(v, env.localTDF())
	if !ok {
		return ssIvTime, nil
	}

	if utcadr == 0 || !env.storeUTC(utcadr, u) {
		return env.timconFault(utcadr, uint32(vm.MaskWrite))
	}

	return ssNormal, nil
}

// timconFault signals the access violation $TIMCON's own access of va would
// have, its reason mask's write bit set for a write.
func (env *Environment) timconFault(va, reason uint32) (uint32, error) {
	return env.SignalFailure([]uint32{ssAccVio, reason, va}, ssAccVio)
}

// utcBaseCheck is the base date's distance worked out by the calendar, for
// the tests: it must equal utcBaseTicks.
func utcBaseCheck() uint64 {
	from := time.Date(1582, time.October, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(1858, time.November, 17, 0, 0, 0, 0, time.UTC)

	return uint64(to.Sub(from).Hours()/24) * ticksPerDay
}

func registerUTCServices(t *ServiceTable) {
	t.Register("SYS$GETUTC", serviceSysGetutc)
	t.Register("SYS$ASCUTC", serviceSysAscutc)
	t.Register("SYS$BINUTC", serviceSysBinutc)
	t.Register("SYS$NUMUTC", serviceSysNumutc)
	t.Register("SYS$TIMCON", serviceSysTimcon)
}
