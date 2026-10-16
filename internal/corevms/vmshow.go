package corevms

import (
	"errors"
	"fmt"

	"github.com/tucats/govax/internal/vmheap"
)

// The zone database as the RTL's debugging routines see it
// (LIB$FIND_VM_ZONE, LIB$SHOW_VM_ZONE, LIB$VERIFY_VM_ZONE; the 5.0 LIB$
// manual's chapter 6 and the routines' entries; docs/PHASE-59, subtask
// 9): the image's zones in the order they were made, what each holds,
// and a check of each.
//
// A zone's bookkeeping is in Go (the phase doc's Decision 2), so what a
// program can damage, and a check can find, is what's in its memory: the
// validity longword at the start of the zone's control block, and the
// bytes of the free space, which a zone with a free fill keeps filled
// (ZoneFree fills a freed block whole, tags too, and alignment padding
// is filled as it's made). The Go side is checked too (vmheap's Check),
// which finds nothing unless govax itself is wrong.

// ErrZoneDamaged is a zone whose memory the program changed: its
// control block's validity longword, or free space that no longer holds
// the zone's free fill; or a lookaside list a double free linked to
// itself (LIB$_BADZONE).
var ErrZoneDamaged = errors.New("corevms: a zone is damaged")

// The contexts NextZone returns after the RTL's two zones; after any
// other zone, the context is its control zone entry's address.
const (
	afterDefaultZone = 1
	afterStringZone  = 2
)

// NextZone returns the ID of the zone after the one the context names
// (0: the first zone), and the context to pass for the zone after it,
// for LIB$FIND_VM_ZONE's scan; false when there are no more. The order
// is VMS 7.1's (testdata/lib59): the default zone, the string zone, then
// the others as their control blocks lie in the control zone, its areas
// in the order they're searched (the latest first) and each area's by
// address. The RTL's two zones are made first if they aren't yet (VMS's
// exist before a program runs). The error is vmheap.ErrNoMemory when P0
// has no room for them.
func (env *Environment) NextZone(context uint32) (id, next uint32, ok bool, err error) {
	if err := env.systemZones(); err != nil {
		return 0, 0, false, err
	}

	switch context {
	case 0:
		return env.defaultHeap.id, afterDefaultZone, true, nil
	case afterDefaultZone:
		return env.stringHeap.id, afterStringZone, true, nil
	}

	after := zonePlace{-1, 0}
	if context != afterStringZone {
		after = env.placeOf(context)
	}

	var (
		found *heapZone
		at    zonePlace
	)
	
	for _, z := range env.zones {
		if z.system {
			continue
		}

		p := env.placeOf(z.entry)
		if after.before(p) && (found == nil || p.before(at)) {
			found, at = z, p
		}
	}

	if found == nil {
		return 0, 0, false, nil
	}

	return found.id, found.entry, true, nil
}

// zonePlace is where a control block lies in the control zone: its
// area's place in the area list, and its address.
type zonePlace struct {
	area int
	addr uint32
}

func (p zonePlace) before(q zonePlace) bool {
	return p.area < q.area || p.area == q.area && p.addr < q.addr
}

// placeOf returns where the control block at addr lies.
func (env *Environment) placeOf(addr uint32) zonePlace {
	if env.zoneControl != nil {
		for i, a := range env.zoneControl.AreaReports() {
			if a.Base <= addr && addr < a.End {
				return zonePlace{i, addr}
			}
		}
	}

	return zonePlace{1 << 30, addr}
}

// VerifyZones checks every zone's control block mark, for
// LIB$FIND_VM_ZONE's first call, returning the first error. VMS 7.1's
// LIB$FIND_VM_ZONE doesn't find a zone damaged that LIB$VERIFY_VM_ZONE
// does (testdata/lib59: a free-fill mismatch, a lookaside list freed
// into twice), so neither does this.
func (env *Environment) VerifyZones() error {
	for _, z := range env.zones {
		if err := env.checkMark(z); err != nil {
			return err
		}
	}

	return nil
}

// checkMark checks the validity longword of z's control block.
func (env *Environment) checkMark(z *heapZone) error {
	if mark, err := env.mem.LoadLongword(env.cpu, z.id); err != nil || mark != z.id {
		return fmt.Errorf("%w: zone %08X's control block was changed", ErrZoneDamaged, z.id)
	}

	return nil
}

// knownZone returns the zone id names, a user-defined zone too (0, the
// default zone, made if it isn't yet).
func (env *Environment) knownZone(id uint32) (*heapZone, error) {
	if id == 0 {
		return env.defaultZone()
	}

	z, ok := env.zones[id]
	if !ok {
		return nil, ErrBadZone
	}

	return z, nil
}

// VerifyZone checks the zone id names (0, the default zone) for
// LIB$VERIFY_VM_ZONE: its control block's validity longword, its
// structures (a lookaside list a double free linked to itself is
// damage), and, for a zone with a free fill, the free fill in all of its
// free space, the lookaside lists' blocks too, but for the control words
// VMS keeps there (the 5.0 manual: "If the zone was created with
// LIB$M_VM_FREE_FILL0 or LIB$M_VM_FREE_FILL1, LIB$VERIFY_VM_ZONE also
// checks the contents of the free blocks"). The error is ErrBadZone for
// an ID that names no zone, and ErrZoneDamaged, saying what's wrong, for
// the first fault found.
func (env *Environment) VerifyZone(id uint32) error {
	z, err := env.knownZone(id)
	if err != nil {
		return err
	}

	if err := env.checkMark(z); err != nil {
		return err
	}

	if z.user != nil {
		return nil
	}

	if err := z.zone.Check(); err != nil {
		return fmt.Errorf("%w: %w", ErrZoneDamaged, err)
	}

	for _, s := range env.reportOf(z).FreeSpace() {
		if bad, err := env.FreeFillMismatch(id, s); err != nil || bad {
			return fmt.Errorf("%w: free-fill mismatch in %08X-%08X", ErrZoneDamaged, s.Start, s.End-1)
		}
	}

	return nil
}

// FreeFillMismatch reports whether free space s of the zone id names
// (0, the default zone) holds anything but the zone's free fill, its
// control words left out (always false for a zone without a free fill).
// The error is a byte that can't be read.
func (env *Environment) FreeFillMismatch(id uint32, s vmheap.Span) (bool, error) {
	z, err := env.knownZone(id)
	if err != nil || z.zone == nil {
		return false, err
	}

	from := s.Start

	for _, w := range append(controlWords(z, s), vmheap.Span{Start: s.End, End: s.End}) {
		if bad, err := env.FillMismatch(vmheap.Span{Start: from, End: w.Start}, z.settings.FreeFill); err != nil || bad {
			return bad, err
		}

		from = w.End
	}

	return false, nil
}

// ZoneReport is what LIB$SHOW_VM_ZONE shows of a zone: its ID and
// settings (User for a user-defined zone, which has nothing else), the
// size of its control block, its quantum and alignment (vmheap.Zone's
// Quantum and Alignment), how many pages its areas hold, the bytes in
// the program's blocks, and its areas and lookaside lists.
type ZoneReport struct {
	ID       uint32
	Settings ZoneSettings
	User     *UserZone
	Control  uint32
	Smallest uint32

	Quantum, Alignment uint32
	Pages              uint32
	InUse              uint32
	Freed              uint32
	Areas              []vmheap.AreaReport
	Lists              []vmheap.ListReport
}

// ZoneReportOf returns the report of the zone id names (0, the default
// zone). The error is ErrBadZone for an ID that names no zone.
func (env *Environment) ZoneReportOf(id uint32) (ZoneReport, error) {
	z, err := env.knownZone(id)
	if err != nil {
		return ZoneReport{}, err
	}

	return env.reportOf(z), nil
}

// reportOf is ZoneReportOf's report of z.
func (env *Environment) reportOf(z *heapZone) ZoneReport {
	r := ZoneReport{ID: z.id, Settings: z.settings, User: z.user, Control: z.cb}
	if z.user != nil {
		return r
	}

	r.Quantum, r.Alignment, r.Smallest = z.zone.Quantum(), z.zone.Alignment(), z.zone.Smallest()
	r.Pages, r.InUse, r.Freed = z.zone.Pages(), z.zone.BytesInUse(), z.zone.FreedBytes()
	r.Areas, r.Lists = z.zone.AreaReports(), z.zone.ListReports()

	return r
}

// ControlBytes is the bytes of the zone's control blocks: the zone's and
// each area's (vmheap.AreaHeader), as VMS 7.1 counts them.
func (r ZoneReport) ControlBytes() uint32 {
	return r.Control + vmheap.AreaHeader*uint32(len(r.Areas))
}

// FreeSpace is the zone's free space: each area's free spans, then the
// lookaside lists' blocks (whole, tags included).
func (r ZoneReport) FreeSpace() []vmheap.Span {
	var list []vmheap.Span

	for _, a := range r.Areas {
		list = append(list, a.Free...)
	}

	for _, l := range r.Lists {
		list = append(list, l.Blocks...)
	}

	return list
}

// FreeBytes is how many bytes have been freed and not yet allocated
// again, as VMS 7.1 counts them (vmheap.Zone.FreedBytes).
func (r ZoneReport) FreeBytes() uint32 {
	return r.Freed
}

// FillMismatch reports whether any byte of s doesn't hold fill (always
// false for NoFill). The error is a byte that can't be read.
func (env *Environment) FillMismatch(s vmheap.Span, fill Fill) (bool, error) {
	var want byte

	switch fill {
	case NoFill:
		return false, nil
	case FillOnes:
		want = 0xFF
	}

	for addr := s.Start; addr < s.End; addr++ {
		b, err := env.mem.LoadByte(env.cpu, addr)
		if err != nil {
			return false, err
		}

		if b != want {
			return true, nil
		}
	}

	return false, nil
}
