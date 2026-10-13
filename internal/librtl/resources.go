package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// The processwide resource allocators (docs/PHASE-55, the survey's
// "Event flags and LUNs"; the 5.0 manual's section 2.4): LIB$GET_EF,
// LIB$FREE_EF, and LIB$RESERVE_EF hand out local event flags, and
// LIB$GET_LUN and LIB$FREE_LUN logical unit numbers, so the separately
// written routines of one program don't use the same flag or unit by
// accident. The routines only keep the books: GET_EF doesn't clear the
// flag it hands out, and FREE_EF doesn't touch its state.
//
// The pools are LIBRTL's own data, in the image's P0 space on VMS, so
// each image starts with the manual's initial reservations; govax keeps
// them per process in Go and forgets them at image rundown
// (corevms.Environment.ImageData).
//
// The statuses are returned, never signaled. They're all errors or
// worse: LIB$_INSEF and LIB$_LUNRESSYS are severe.

var (
	libInsEF     = vmsdef.LibrarySymbols["LIB$_INSEF"]
	libEFAlrFre  = vmsdef.LibrarySymbols["LIB$_EF_ALRFRE"]
	libEFAlrRes  = vmsdef.LibrarySymbols["LIB$_EF_ALRRES"]
	libEFResSys  = vmsdef.LibrarySymbols["LIB$_EF_RESSYS"]
	libInsLUN    = vmsdef.LibrarySymbols["LIB$_INSLUN"]
	libLUNAlrFre = vmsdef.LibrarySymbols["LIB$_LUNALRFRE"]
	libLUNResSys = vmsdef.LibrarySymbols["LIB$_LUNRESSYS"]
)

// The event flag pool (LIB$GET_EF's table in the 5.0 manual): flag 0 is
// never handed out; 1 to 23 are reserved when the image starts, but may
// be freed with LIB$FREE_EF and are then the routines' to give; 24 to 31
// are VMS's own; 32 to 63 start free. Flags 64 and up are common event
// flags, never the routines'.
const (
	firstLowEF  = 1  // the first of the initially reserved flags
	lastLowEF   = 23 // the last of them
	firstHighEF = 32 // the first of the initially free flags
	lastHighEF  = 63 // the last local event flag
)

// The LUN pool: 100 to 119, all free when the image starts.
const (
	firstLUN = 100
	lastLUN  = 119
)

// resourcePools is one image's allocation state: a bit set for each
// event flag (bit n for flag n) and each LUN (bit n for unit firstLUN+n)
// that's allocated.
type resourcePools struct {
	flags uint64
	luns  uint32
}

// poolsKey is the corevms.Environment.ImageData key of the image's
// resourcePools.
type poolsKey struct{}

// pools returns the running image's pools, the manual's initial
// reservations (flags 1 to 23) the first time an image asks.
func pools(env *corevms.Environment) *resourcePools {
	return env.ImageData(poolsKey{}, func() any {
		p := &resourcePools{}
		for ef := firstLowEF; ef <= lastLowEF; ef++ {
			p.flags |= 1 << ef
		}

		return p
	}).(*resourcePools)
}

// poolEF reports whether ef is one of the flags the routines manage:
// 1 to 23 or 32 to 63. The rest (0, VMS's 24 to 31, the common flags)
// are LIB$_EF_RESSYS to FREE_EF and RESERVE_EF.
func poolEF(ef int32) bool {
	return (ef >= firstLowEF && ef <= lastLowEF) || (ef >= firstHighEF && ef <= lastHighEF)
}

// readArg reads the longword argument passed by reference at addr.
func readArg(env *corevms.Environment, addr uint32) (int32, error) {
	v, err := env.Memory().LoadLongword(env.CPU(), addr)

	return int32(v), err
}

// writeResult writes v to the longword at addr, the routines' output.
func writeResult(env *corevms.Environment, addr uint32, v int32) error {
	return env.Memory().StoreLongword(env.CPU(), addr, uint32(v))
}

// allocate gives the lowest number in [first, last] whose bit in the set
// at bits is clear (bit n-base for number n), setting the bit, or -1 and
// false when they're all set. Which free number VMS hands out first
// isn't documented: lowest first is unconfirmed (the survey).
func allocate(bits *uint64, base, first, last int32) (int32, bool) {
	for n := first; n <= last; n++ {
		if bit := uint64(1) << (n - base); *bits&bit == 0 {
			*bits |= bit

			return n, true
		}
	}

	return -1, false
}

// nextEF allocates in the flag set at bits the flag GET_EF would give:
// the lowest free one of 1 to 23, else of 32 to 63; -1 and false when
// none is free.
func nextEF(bits *uint64) (int32, bool) {
	ef, ok := allocate(bits, 0, firstLowEF, lastLowEF)
	if !ok {
		ef, ok = allocate(bits, 0, firstHighEF, lastHighEF)
	}

	return ef, ok
}

// libGetEF is LIB$GET_EF:
//
//	LIB$GET_EF event-flag-number.wl.r
//
// It allocates a local event flag, the lowest free one (32 at image
// start, since 1 to 23 start reserved; unconfirmed), and writes its
// number; with none free it writes -1 and returns LIB$_INSEF.
func libGetEF(env *corevms.Environment, argv []uint32) (uint32, error) {
	p := pools(env)
	bits := p.flags
	ef, ok := nextEF(&bits)

	if err := writeResult(env, arg(argv, 0), ef); err != nil {
		return memoryFault(env, writing(err)) // p.flags unchanged: it stays free
	}

	p.flags = bits

	if !ok {
		return libInsEF, nil
	}

	return ssNormal, nil
}

// libFreeEF is LIB$FREE_EF:
//
//	LIB$FREE_EF event-flag-number.rlu.r
//
// It frees an event flag GET_EF or RESERVE_EF allocated, or one of the
// initially reserved 1 to 23. LIB$_EF_ALRFRE if it's free already, and
// LIB$_EF_RESSYS for a flag the routines don't manage (0, 24 to 31, 64
// and up).
func libFreeEF(env *corevms.Environment, argv []uint32) (uint32, error) {
	ef, err := readArg(env, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	if !poolEF(ef) {
		return libEFResSys, nil
	}

	p := pools(env)
	if p.flags&(1<<ef) == 0 {
		return libEFAlrFre, nil
	}

	p.flags &^= 1 << ef

	return ssNormal, nil
}

// libReserveEF is LIB$RESERVE_EF:
//
//	LIB$RESERVE_EF event-flag-number.rlu.r
//
// It allocates a particular event flag. LIB$_EF_ALRRES if it's
// allocated already (so any of 1 to 23 until FREE_EF frees it), and
// LIB$_EF_RESSYS for a flag the routines don't manage.
func libReserveEF(env *corevms.Environment, argv []uint32) (uint32, error) {
	ef, err := readArg(env, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	if !poolEF(ef) {
		return libEFResSys, nil
	}

	p := pools(env)
	if p.flags&(1<<ef) != 0 {
		return libEFAlrRes, nil
	}

	p.flags |= 1 << ef

	return ssNormal, nil
}

// libGetLUN is LIB$GET_LUN:
//
//	LIB$GET_LUN logical-unit-number.wl.r
//
// It allocates a logical unit number from 100 to 119, the lowest free
// one (unconfirmed), and writes it; with none free it writes -1 and
// returns LIB$_INSLUN.
func libGetLUN(env *corevms.Environment, argv []uint32) (uint32, error) {
	p := pools(env)
	bits := uint64(p.luns)

	lun, ok := allocate(&bits, firstLUN, firstLUN, lastLUN)

	if err := writeResult(env, arg(argv, 0), lun); err != nil {
		return memoryFault(env, writing(err)) // p.luns unchanged: it stays free
	}

	p.luns = uint32(bits)

	if !ok {
		return libInsLUN, nil
	}

	return ssNormal, nil
}

// libFreeLUN is LIB$FREE_LUN:
//
//	LIB$FREE_LUN logical-unit-number.rl.r
//
// It frees a logical unit number GET_LUN allocated. LIB$_LUNALRFRE if
// it's free already, and LIB$_LUNRESSYS for a number outside 100 to 119.
func libFreeLUN(env *corevms.Environment, argv []uint32) (uint32, error) {
	lun, err := readArg(env, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	if lun < firstLUN || lun > lastLUN {
		return libLUNResSys, nil
	}

	p := pools(env)
	bit := uint32(1) << (lun - firstLUN)

	if p.luns&bit == 0 {
		return libLUNAlrFre, nil
	}

	p.luns &^= bit

	return ssNormal, nil
}
