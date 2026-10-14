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
// the zone's free fill (LIB$_BADZONE).
var ErrZoneDamaged = errors.New("corevms: a zone is damaged")

// NextZone returns the ID of the first of the image's zones made after
// the one numbered after (0: the first zone), and its number, for
// LIB$FIND_VM_ZONE's scan; false when there are no more. The RTL's two
// zones are made first if they aren't yet (VMS's exist before a program
// runs), so they're found first, the default zone and then the string
// zone, as the manual says. The error is vmheap.ErrNoMemory when P0 has
// no room for them.
func (env *Environment) NextZone(after uint32) (id, seq uint32, ok bool, err error) {
	if err := env.systemZones(); err != nil {
		return 0, 0, false, err
	}

	var next *heapZone

	for _, z := range env.zones {
		if z.seq > after && (next == nil || z.seq < next.seq) {
			next = z
		}
	}

	if next == nil {
		return 0, 0, false, nil
	}

	return next.id, next.seq, true, nil
}

// VerifyZones verifies every zone of the image (VerifyZone), for
// LIB$FIND_VM_ZONE's first call, returning the first error.
func (env *Environment) VerifyZones() error {
	for after := uint32(0); ; {
		id, seq, ok, err := env.NextZone(after)
		if err != nil || !ok {
			return err
		}

		if err := env.VerifyZone(id); err != nil {
			return err
		}

		after = seq
	}
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
// structures, and, for a zone with a free fill, the free fill in all of
// its free space, the lookaside lists' blocks too (the 5.0 manual: "If
// the zone was created with LIB$M_VM_FREE_FILL0 or LIB$M_VM_FREE_FILL1,
// LIB$VERIFY_VM_ZONE also checks the contents of the free blocks"). The
// error is ErrBadZone for an ID that names no zone, and ErrZoneDamaged,
// saying what's wrong, for the first fault found.
func (env *Environment) VerifyZone(id uint32) error {
	z, err := env.knownZone(id)
	if err != nil {
		return err
	}

	if mark, err := env.mem.LoadLongword(env.cpu, z.id); err != nil || mark != z.id {
		return fmt.Errorf("%w: zone %08X's control block was changed", ErrZoneDamaged, z.id)
	}

	if z.user != nil {
		return nil
	}

	if err := z.zone.Check(); err != nil {
		return fmt.Errorf("%w: %w", ErrZoneDamaged, err)
	}

	r := env.reportOf(z)

	for _, s := range r.FreeSpace() {
		bad, err := env.FillMismatch(s, z.settings.FreeFill)
		if err != nil || bad {
			return fmt.Errorf("%w: free-fill mismatch in %08X-%08X", ErrZoneDamaged, s.Start, s.End-1)
		}
	}

	return nil
}

// ZoneReport is what LIB$SHOW_VM_ZONE shows of a zone: its ID and
// settings (User for a user-defined zone, which has nothing else), its
// quantum and alignment (vmheap.Zone's Quantum and Alignment), how many
// pages its areas hold, the bytes in the program's blocks, and its areas
// and lookaside lists.
type ZoneReport struct {
	ID       uint32
	Settings ZoneSettings
	User     *UserZone

	Quantum, Alignment uint32
	Pages              uint32
	InUse              uint32
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
	r := ZoneReport{ID: z.id, Settings: z.settings, User: z.user}
	if z.user != nil {
		return r
	}

	r.Quantum, r.Alignment = z.zone.Quantum(), z.zone.Alignment()
	r.Pages, r.InUse = z.zone.Pages(), z.zone.BytesInUse()
	r.Areas, r.Lists = z.zone.AreaReports(), z.zone.ListReports()

	return r
}

// ControlBytes is the bytes of the zone's control blocks: the zone's
// (ZoneControlSize) and each area's (vmheap.AreaHeader). VMS's zone
// control block size isn't known, so neither is the figure
// (unconfirmed).
func (r ZoneReport) ControlBytes() uint32 {
	return ZoneControlSize + vmheap.AreaHeader*uint32(len(r.Areas))
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
// again: FreeSpace's bytes.
func (r ZoneReport) FreeBytes() uint32 {
	var n uint32

	for _, s := range r.FreeSpace() {
		n += s.Size()
	}

	return n
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
