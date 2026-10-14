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

// vmFlagsAccepted are the flag bits VMS 7.1 accepts (testdata/lib59):
// bits 0 to 8. Bits 6 and 7 are NO_EXTEND and TAIL_LARGE (the VSI
// manual's; the 5.0 manual stops at EXTEND_AREA, bit 5), which act but
// aren't shown by LIB$SHOW_VM_ZONE; bit 8 does nothing govax knows of.
// Any other bit is LIB$_INVARG.
const vmFlagsAccepted = 0x1FF

// vmMaxCreateArgs is the most arguments LIB$CREATE_VM_ZONE takes on VMS
// 7.1: thirteen, get-page twelfth and free-page thirteenth, as the VSI
// manual has them (testdata/lib59: fourteen is LIB$_WRONUMARG).
const vmMaxCreateArgs = 13

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
//	    [,get-page] [,free-page]
//
// It makes a zone and writes its ID to zone-id. Every argument but
// zone-id is optional, a longword by reference but for zone-name (a
// string of any class) and the procedures. The rules are VMS 7.1's
// (testdata/lib59): algorithm 1 to 4, first fit by default;
// algorithm-argument required for the other three (quick fit's lists 1
// to 128, frequent sizes' 1 to 16, a fixed size above 0) and ignored for
// first fit; flags bits 0 to 8, both fills of a pair being the fill of
// 0; extend-size above 0 pages, 16 by default; initial-size 0 or more
// pages, none by default (no area until the first LIB$GET_VM);
// block-size a power of 2 from 8 to 512, alignment from 4 to 512, both 8
// by default; page-limit 0 (none) to 32,767 pages, an initial size past
// it LIB$_PAGLIMEXC (zone-id written, though no zone is made);
// smallest-block-size above 0, the block size by default, rounded up to
// a multiple of it. Anything out of range is LIB$_INVARG; more than 13
// arguments LIB$_WRONUMARG. No room for the initial area is
// LIB$_INSVIRMEM. After an error no zone is made.
//
// get-page and free-page are procedures (entry masks' addresses, by
// value) the zone calls instead of LIB$GET_VM_PAGE and LIB$FREE_VM_PAGE,
// with their arguments; 0 or omitted is those routines (corevms's
// ZoneSettings). A zone with a get-page routine calls it here for its
// control block's page and its initial area; a failure status it
// returns is LIB$CREATE_VM_ZONE's.
func libCreateVMZone(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) > vmMaxCreateArgs {
		return libWroNumArg, nil
	}

	settings, err := zoneSettings(env, argv)

	switch {
	case errors.Is(err, errBadZoneArgument):
		return libInvArg, nil
	case err != nil:
		return stringFailure(env, err)
	}

	settings.GetPage, settings.FreePage = arg(argv, 11), arg(argv, 12)

	// Its control block, initial area, or deletion after an error below
	// calls the program's routines: that needs a goroutine of the
	// routine's own.
	if (settings.GetPage != 0 || settings.FreePage != 0) && !env.CallingBack() {
		return env.RunCallingBack("LIB$CREATE_VM_ZONE", libCreateVMZone, argv)
	}

	id, err := env.CreateZone(settings)
	if errors.Is(err, vmheap.ErrPageLimit) && id != 0 {
		if err := env.Memory().StoreLongword(env.CPU(), arg(argv, 0), id); err != nil {
			return memoryFault(env, writing(err))
		}

		return libPageLimit, nil
	}

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
	if flags&^vmFlagsAccepted != 0 {
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

	if extend < 1 || initial < 0 ||
		!powerOf2Within(blockSize, 8, 512) || !powerOf2Within(alignment, 4, 512) ||
		limit < 0 || limit > 32767 || smallest <= 0 {
		return s, errBadZoneArgument
	}

	s.ExtendPages = uint32(extend)

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
