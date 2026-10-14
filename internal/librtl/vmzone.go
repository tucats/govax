package librtl

import (
	"errors"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmheap"
	"github.com/tucats/govax/internal/vmsdef"
)

// LIB$CREATE_VM_ZONE: a new zone, a heap of its own with the attributes
// the caller gives (5.0 LIB$ manual, section 5.3, and the routine's
// entry), made by corevms's CreateZone over internal/vmheap's Zone.

// The algorithm codes and flag bits, from VMS's definitions.
var (
	vmFirstFit  = vmsdef.LibrarySymbols["LIB$K_VM_FIRST_FIT"]
	vmFixedSize = vmsdef.LibrarySymbols["LIB$K_VM_FIXED"]

	vmBoundaryTags = vmsdef.LibrarySymbols["LIB$M_VM_BOUNDARY_TAGS"]
	vmGetFill0     = vmsdef.LibrarySymbols["LIB$M_VM_GET_FILL0"]
	vmGetFill1     = vmsdef.LibrarySymbols["LIB$M_VM_GET_FILL1"]
	vmFreeFill0    = vmsdef.LibrarySymbols["LIB$M_VM_FREE_FILL0"]
	vmFreeFill1    = vmsdef.LibrarySymbols["LIB$M_VM_FREE_FILL1"]
	vmExtendArea   = vmsdef.LibrarySymbols["LIB$M_VM_EXTEND_AREA"]
	vmNoExtend     = vmsdef.LibrarySymbols["LIB$M_VM_NO_EXTEND"]
	vmTailLarge    = vmsdef.LibrarySymbols["LIB$M_VM_TAIL_LARGE"]
)

// vmFlagsDefined are the flag bits VMS 7.3 defines (the 5.0 manual
// stops at EXTEND_AREA, bit 5; the VSI manual describes NO_EXTEND and
// TAIL_LARGE, bits 6 and 7); any other bit is LIB$_INVARG.
var vmFlagsDefined = vmBoundaryTags | vmGetFill0 | vmGetFill1 | vmFreeFill0 | vmFreeFill1 |
	vmExtendArea | vmNoExtend | vmTailLarge

// algorithms are the manual's algorithm codes, each with the bounds of
// its algorithm-argument (0 and 0: none, ignored).
var algorithms = map[uint32]struct {
	kind     vmheap.Algorithm
	min, max int32
}{
	vmFirstFit: {vmheap.FirstFit, 0, 0},
	vmsdef.LibrarySymbols["LIB$K_VM_QUICK_FIT"]:  {vmheap.QuickFit, 1, 128},
	vmsdef.LibrarySymbols["LIB$K_VM_FREQ_SIZES"]: {vmheap.FrequentSizes, 1, 16},
	vmFixedSize: {vmheap.FixedSize, 1, 0x7FFFFFFF},
}

// errBadZoneArgument is an argument of LIB$CREATE_VM_ZONE out of its
// range: LIB$_INVARG.
var errBadZoneArgument = errors.New("zone attribute out of range")

// libCreateVMZone is LIB$CREATE_VM_ZONE:
//
//	LIB$CREATE_VM_ZONE zone-id [,algorithm] [,algorithm-argument]
//	    [,flags] [,extend-size] [,initial-size] [,block-size]
//	    [,alignment] [,page-limit] [,smallest-block-size] [,zone-name]
//	    [,number-of-areas] [,get-page] [,free-page]
//
// It makes a zone and writes its ID to zone-id. Every argument but
// zone-id is optional, a longword by reference but for zone-name (a
// string of any class). Defaults and ranges, the manual's: algorithm 1
// to 4, first fit by default; algorithm-argument required for the other
// three (quick fit's lists 1 to 128, frequent sizes' 1 to 16, a fixed
// size above 0); extend-size 1 to 1,024 pages, 16 by default, unused
// with NO_EXTEND; initial-size 0 or more pages, 0 (no area until the
// first LIB$GET_VM) by default; block-size a power of 2 from 8 to 512,
// alignment from 4 to 512, both 8 by default; page-limit 0 (none) to
// 32,767 pages; smallest-block-size above 0, the block size by default.
// Anything out of range is LIB$_INVARG, and so are, by govax's choice
// (unconfirmed): a flag bit VMS doesn't define, both fills of a pair, a
// page limit or NO_EXTEND without an initial size, and an initial size
// past the page limit. No room for the initial area is LIB$_INSVIRMEM.
// After an error no zone is made.
//
// number-of-areas (the 5.0 manual's; nothing says what it does) is
// ignored, and so, until user zones' page routines are done (Phase 59's
// subtask 6), are get-page and free-page; which of arguments 12 to 14
// VMS 7.3 reads them from is a probe question.
func libCreateVMZone(env *corevms.Environment, argv []uint32) (uint32, error) {
	settings, err := zoneSettings(env, argv)

	switch {
	case errors.Is(err, errBadZoneArgument):
		return libInvArg, nil
	case err != nil:
		return stringFailure(env, err)
	}

	id, err := env.CreateZone(settings)
	if err != nil {
		return vmFailure(env, err)
	}

	if err := env.Memory().StoreLongword(env.CPU(), arg(argv, 0), id); err != nil {
		_ = env.DeleteZone(id)

		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// zoneSettings reads LIB$CREATE_VM_ZONE's arguments 2 to 11 into a
// zone's settings, checking each against its range. The error is
// errBadZoneArgument, or a memory or string error.
func zoneSettings(env *corevms.Environment, argv []uint32) (corevms.ZoneSettings, error) {
	var s corevms.ZoneSettings

	// Each longword argument, read in order: given or not, and its value
	// (the default when not).
	type longArg struct {
		given bool
		value int32
	}

	longs := make([]longArg, 10)

	for i := range longs {
		addr := arg(argv, i+1)
		if i == 9 || addr == 0 { // argument 11 is the zone's name
			continue
		}

		v, err := env.Memory().LoadLongword(env.CPU(), addr)
		if err != nil {
			return s, err
		}

		longs[i] = longArg{true, int32(v)}
	}

	value := func(i int, fallback int32) int32 {
		if longs[i].given {
			return longs[i].value
		}

		return fallback
	}

	algorithm, ok := algorithms[uint32(value(0, int32(vmFirstFit)))]
	if !ok {
		return s, errBadZoneArgument
	}

	s.Algorithm = algorithm.kind

	if algorithm.kind != vmheap.FirstFit {
		n := value(1, 0)
		if !longs[1].given || n < algorithm.min || n > algorithm.max {
			return s, errBadZoneArgument
		}

		if algorithm.kind == vmheap.FixedSize {
			s.FixedSize = uint32(n)
		} else {
			s.Lists = uint32(n)
		}
	}

	flags := uint32(value(2, 0))
	if flags&^vmFlagsDefined != 0 ||
		flags&(vmGetFill0|vmGetFill1) == vmGetFill0|vmGetFill1 ||
		flags&(vmFreeFill0|vmFreeFill1) == vmFreeFill0|vmFreeFill1 {
		return s, errBadZoneArgument
	}

	s.BoundaryTags = flags&vmBoundaryTags != 0
	s.ExtendArea = flags&vmExtendArea != 0
	s.NoExtend = flags&vmNoExtend != 0
	s.TailLarge = flags&vmTailLarge != 0
	s.GetFill = fillOf(flags, vmGetFill0, vmGetFill1)
	s.FreeFill = fillOf(flags, vmFreeFill0, vmFreeFill1)

	extend, initial := value(3, 16), value(4, 0)
	blockSize, alignment := value(5, 8), value(6, 8)
	limit, smallest := value(7, 0), value(8, blockSize)

	if !s.NoExtend && (extend < 1 || extend > 1024) ||
		initial < 0 ||
		!powerOf2Within(blockSize, 8, 512) || !powerOf2Within(alignment, 4, 512) ||
		limit < 0 || limit > 32767 || limit > 0 && (initial == 0 || initial > limit) ||
		s.NoExtend && initial == 0 ||
		smallest <= 0 {
		return s, errBadZoneArgument
	}

	if !s.NoExtend {
		s.ExtendPages = uint32(extend)
	}

	s.InitialPages, s.PageLimit = uint32(initial), uint32(limit)
	s.BlockSize, s.Alignment = uint32(blockSize), uint32(alignment)

	if s.Algorithm == vmheap.QuickFit {
		s.Smallest = uint32(smallest)
	}

	name, _, err := optionalName(env, arg(argv, 10))
	if err != nil {
		return s, err
	}

	s.Name = name

	return s, nil
}

// fillOf is the fill flags choose of a pair's two bits: bytes of 0 for
// zero, of FF for ones, or none.
func fillOf(flags, zero, ones uint32) corevms.Fill {
	switch {
	case flags&zero != 0:
		return corevms.FillZero
	case flags&ones != 0:
		return corevms.FillOnes
	default:
		return corevms.NoFill
	}
}

// powerOf2Within reports whether n is a power of 2 from lo to hi.
func powerOf2Within(n, lo, hi int32) bool {
	return n >= lo && n <= hi && n&(n-1) == 0
}
