package corevms

import (
	"errors"

	"github.com/tucats/govax/internal/vmheap"
)

// The image's zones: the heaps LIB$CREATE_VM_ZONE makes, each a
// vmheap.Zone over the image's page pool, which LIB$GET_VM and
// LIB$FREE_VM allocate from and LIB$RESET_VM_ZONE and LIB$DELETE_VM_ZONE
// empty and remove (5.0 LIB$ manual, chapter 5; docs/PHASE-59 - LIB VM
// zones.md).
//
// A zone's ID is the address of its control block in P0 (the phase
// doc's Decision 1), whose first longword holds the ID itself as a mark
// of validity. The zone's state is kept in Go, in env.zones, found by
// that address, so an ID that isn't a zone's is caught by the lookup,
// not by trusting memory the program can write. The control blocks are
// as large as VMS 7.1's (controlSize), and come, as VMS's do, from a
// zone of their own, first fit in 4-page areas (controlZone), whose
// layout LIB$FIND_VM_ZONE's order follows (testdata/lib59). A zone with
// a get-page routine has its control block in a page of its own, from
// that routine, and an entry in the control zone in its place.
//
// What the zone allocator doesn't do because it never touches memory is
// done here: the fills (LIB$M_VM_GET_FILL0/1, FREE_FILL0/1), the
// boundary tags' contents, and the control words VMS keeps at the start
// of free space (freeWords), which the program sees when it looks at
// memory it freed.

// Fill is what a zone fills blocks with as they're allocated or freed.
type Fill int

const (
	NoFill   Fill = iota // left as they are
	FillZero             // bytes of 0 (LIB$M_VM_GET_FILL0, FREE_FILL0)
	FillOnes             // bytes of FF (LIB$M_VM_GET_FILL1, FREE_FILL1)
)

// ZoneSettings are a zone's attributes as LIB$CREATE_VM_ZONE gives
// them: the allocator's, the fills, the zone's name, and the program's
// page routines.
//
// GetPage and FreePage are the entry masks' addresses of the get-page
// and free-page routines (0: LIB$GET_VM_PAGE's and LIB$FREE_VM_PAGE's
// code, the image's page pool), each called as those routines are
// (number-of-pages and base-address by reference) for the zone's areas:
// when it needs one, and for each when it's deleted (pageroutines.go).
// A zone with a get-page routine gets its control block's page from it
// too, first.
type ZoneSettings struct {
	vmheap.Attributes

	GetFill, FreeFill Fill
	Name              string
	GetPage, FreePage uint32
}

// InitialShown is the initial size LIB$SHOW_VM_ZONE shows: the extend
// size when the zone was made without one, as VMS 7.1 shows it.
func (s ZoneSettings) InitialShown() uint32 {
	if s.InitialPages == 0 {
		return s.ExtendPages
	}

	return s.InitialPages
}

// UserZone is a user-defined zone's settings, as
// LIB$CREATE_USER_VM_ZONE gives them: the user argument's value, the
// entry masks' addresses of the program's routines that LIB$GET_VM,
// LIB$FREE_VM, LIB$RESET_VM_ZONE, and LIB$DELETE_VM_ZONE call instead of
// doing the work themselves (0 for a routine not given), and the zone's
// name.
type UserZone struct {
	Argument                      uint32
	Allocate, Free, Reset, Delete uint32
	Name                          string
}

// heapZone is one zone of the image: its allocator, its settings, and
// its ID. A system zone is one the RTL made for itself (the default zone
// and the string zone), which the program can use but not reset or
// delete. A user-defined zone has user and no allocator: its blocks are
// the program's business.
//
// cb is the size of its control block, and entry its control block's
// address in the control zone (its ID, but for a zone with a get-page
// routine, whose control block is a page of its own): LIB$FIND_VM_ZONE
// finds the zones in their entries' order (NextZone).
type heapZone struct {
	id       uint32
	cb       uint32
	entry    uint32
	extra    uint32
	zone     *vmheap.Zone
	settings ZoneSettings
	system   bool
	user     *UserZone
}

// The sizes of VMS 7.1's control blocks (testdata/lib59, from
// LIB$SHOW_VM_ZONE's "bytes are used for zone and area control blocks"
// and the distances between zone IDs): a zone's, 80 bytes, with 8 for
// each quick fit lookaside list, 16 for each frequent sizes list, 8 for
// a fixed size zone's list, and its name as a counted string rounded up
// to a quadword; a user-defined zone's, 96 bytes and its name; and the
// control zone entry a zone with a get-page routine has.
const (
	zoneControlBase = 80
	userControlBase = 96
	pageZoneEntry   = 80
)

// tagFillExtra is the block of the control zone a zone with boundary
// tags and a free fill has besides its control block, put before it.
// testdata/lib59's one such zone shows 48 bytes of the control zone
// taken before its control block; the fixed size zone with a free fill
// made just before it could as well have taken them (unconfirmed). It
// isn't counted in LIB$STAT_VM's figures (the run's totals match without
// it).
const tagFillExtra = 48

// controlSize is the size of the control block of a zone with settings
// s.
func controlSize(s ZoneSettings) uint32 {
	n := uint32(zoneControlBase)

	switch s.Algorithm {
	case vmheap.QuickFit:
		n += 8 * s.Lists
	case vmheap.FrequentSizes:
		n += 16 * s.Lists
	case vmheap.FixedSize:
		n += 8
	}

	return n + nameSize(s.Name)
}

// nameSize is the bytes a zone's name takes in its control block.
func nameSize(name string) uint32 {
	if name == "" {
		return 0
	}

	return (uint32(len(name)) + 1 + 7) &^ 7
}

// userArgument is the offset in a user-defined zone's control block of
// the longword holding its user argument, which the program's routines
// are given by reference (testdata/lib59: VMS 7.1 passes the address 72
// bytes into the control block).
const userArgument = 72

var (
	// ErrBadZone is a zone ID that names no zone of the image, or names
	// the default zone where it can't be used (LIB$_BADBLOADR).
	ErrBadZone = errors.New("corevms: invalid zone ID")

	// ErrBadTag is a tagged block whose boundary tag says it isn't
	// allocated: freed already, or changed by the program
	// (LIB$_BADTAGVAL).
	ErrBadTag = errors.New("corevms: a block's boundary tag isn't an allocated block's")

	// ErrUserZone is a user-defined zone's ID given to an operation the
	// RTL does itself: the zone's own routine does it (UserZoneOf).
	ErrUserZone = errors.New("corevms: a user-defined zone's operation")
)

// controlZone is the zone the image's zones' control blocks are in, made
// at the first LIB$CREATE_VM_ZONE: first fit, 4-page areas, as VMS 7.1's
// (testdata/lib59).
var controlZone = vmheap.Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 4}

// The image's heap statistics, LIB$STAT_VM's codes 1 to 3, as VMS 7.1
// counts them (testdata/lib59): LIB$GET_VM's successful calls and
// LIB$FREE_VM's calls (failures among them, once the size is good), the
// RTL's allocations of control blocks among them; and the bytes in the
// blocks they and dynamic strings hold (each block's class, with a
// tagged block's tags: vmheap.Zone.Nominal), less what LIB$FREE_VM, a
// zone's reset or deletion, or a dynamic string's release gave back.
type VMCounts struct {
	Gets, Frees, Bytes uint32
}

// VMStatistics returns the image's heap statistics.
func (env *Environment) VMStatistics() VMCounts {
	return env.vmCounts
}

// CountVMCall counts a call of LIB$GET_VM (get) or LIB$FREE_VM that went
// to a user-defined zone's routine: the call counts, the bytes are the
// routine's (testdata/lib59).
func (env *Environment) CountVMCall(get bool) {
	if get {
		env.vmCounts.Gets++
	} else {
		env.vmCounts.Frees++
	}
}

// CreateZone makes a zone with settings s and returns its ID. The error
// is vmheap.ErrBadAttributes for attributes that break vmheap's rules,
// vmheap.ErrNoMemory when the control block or initial area can't be had,
// a get-page routine's failure (PageRoutineFailure), or
// vmheap.ErrPageLimit for an initial size past the page limit, which VMS
// 7.1 finds after making the control block: the ID is returned with it,
// though the zone is gone (testdata/lib59). No zone is made after an
// error.
//
// The RTL's own zones are made first if they aren't yet.
func (env *Environment) CreateZone(s ZoneSettings) (uint32, error) {
	if err := env.systemZones(); err != nil {
		return 0, err
	}

	cb := controlSize(s)

	if s.GetPage != 0 {
		return env.createPageZone(s, cb)
	}

	var extra uint32

	if s.BoundaryTags && s.FreeFill != NoFill {
		var err error
		if extra, err = env.controlSpace(tagFillExtra); err != nil {
			return 0, err
		}
	}

	id, err := env.controlBlock(cb)
	if err != nil {
		if extra != 0 {
			env.dropControlSpace(extra)
		}

		return 0, err
	}

	drop := func() {
		env.dropControlBlock(id, cb)

		if extra != 0 {
			env.dropControlSpace(extra)
		}
	}

	if s.PageLimit > 0 && s.InitialPages > s.PageLimit {
		drop()

		return id, vmheap.ErrPageLimit
	}

	z, err := env.newHeapZone(id, s)
	if err != nil {
		drop()

		return 0, err
	}

	z.cb, z.entry, z.extra = cb, id, extra

	return id, nil
}

// createPageZone makes a zone with a get-page routine: its entry in the
// control zone, then its control block's page from the routine, then the
// zone itself (whose initial area is the routine's too), as VMS 7.1 does
// (testdata/lib59: the routine's first call is for one page, and a
// zone's ID is page-aligned). A signal from the routine leaves the entry
// allocated, as on VMS.
func (env *Environment) createPageZone(s ZoneSettings, cb uint32) (uint32, error) {
	entry, err := env.controlBlock(pageZoneEntry)
	if err != nil {
		return 0, err
	}

	id, err := env.callPageRoutine(s.GetPage, 1, 0)
	if err != nil {
		env.dropControlBlock(entry, pageZoneEntry)

		return 0, err
	}

	if id == 0 || id%vmheap.PageSize != 0 {
		env.dropControlBlock(entry, pageZoneEntry)

		return 0, vmheap.ErrBadPages
	}

	release := func() {
		if s.FreePage != 0 {
			_, _ = env.callPageRoutine(s.FreePage, 1, id)
		} else {
			_ = env.pagePool().Free(1, id)
		}

		env.dropControlBlock(entry, pageZoneEntry)
	}

	if err := env.mem.StoreLongword(env.cpu, id, id); err != nil {
		release()

		return 0, err
	}

	if s.PageLimit > 0 && s.InitialPages > s.PageLimit {
		release()

		return id, vmheap.ErrPageLimit
	}

	z, err := env.newHeapZone(id, s)
	if err != nil {
		release()

		return 0, err
	}

	z.cb, z.entry = cb, entry

	return id, nil
}

// newHeapZone makes the zone with settings s whose control block is at
// id, and enters it in env.zones. A zone with page routines of the
// program's calls its get-page routine here for an initial area, so it
// must be made by a routine that may call back (CallingBack).
func (env *Environment) newHeapZone(id uint32, s ZoneSettings) (*heapZone, error) {
	zone, err := vmheap.NewZone(env.zonePages(s), s.Attributes)
	if err != nil {
		return nil, err
	}

	z := &heapZone{id: id, zone: zone, settings: s}
	env.watchFreeSpace(z)
	env.zones[id] = z

	return z, nil
}

// controlBlock allocates a control block of n bytes in the control zone
// (counted, as VMS 7.1 counts its own: testdata/lib59), writes its
// address in its first longword, and returns that address.
func (env *Environment) controlBlock(n uint32) (uint32, error) {
	id, err := env.controlSpace(n)
	if err == nil {
		env.vmCounts.Gets++
		env.vmCounts.Bytes += env.zoneControl.Nominal(n)
	}

	return id, err
}

// controlSpace is controlBlock without the count.
func (env *Environment) controlSpace(n uint32) (uint32, error) {
	if env.zoneControl == nil {
		var err error
		if env.zoneControl, err = vmheap.NewZone(env.pagePool(), controlZone); err != nil {
			panic(err) // controlZone's attributes are valid
		}
	}

	id, err := env.zoneControl.Allocate(n)
	if err != nil {
		return 0, err
	}

	if err := env.mem.StoreLongword(env.cpu, id, id); err != nil {
		_, _ = env.zoneControl.Free(id)

		return 0, err
	}

	if env.zones == nil {
		env.zones = map[uint32]*heapZone{}
	}

	return id, nil
}

// dropControlBlock frees a control block of n bytes, clearing its mark
// so the ID is invalid afterwards (until another zone's control block is
// put there), and forgets any zone whose ID it was.
func (env *Environment) dropControlBlock(id, n uint32) {
	if env.dropControlSpace(id) {
		env.vmCounts.Frees++
		env.vmCounts.Bytes -= env.zoneControl.Nominal(n)
	}
}

// dropControlSpace is dropControlBlock without the count, reporting
// whether there was a block to free.
func (env *Environment) dropControlSpace(id uint32) bool {
	delete(env.zones, id)
	_ = env.mem.StoreLongword(env.cpu, id, 0)

	_, err := env.zoneControl.Free(id)

	return err == nil
}

// forget forgets z and frees its control block (and a get-page zone's
// entry; its page is the caller's to give back).
func (env *Environment) forget(z *heapZone) {
	delete(env.zones, z.id)

	if z.extra != 0 {
		env.dropControlSpace(z.extra)
	}

	if z.entry == z.id {
		env.dropControlBlock(z.id, z.cb)

		return
	}

	_ = env.mem.StoreLongword(env.cpu, z.id, 0)
	env.dropControlBlock(z.entry, pageZoneEntry)
}

// CreateUserZone makes a user-defined zone (LIB$CREATE_USER_VM_ZONE)
// and returns its ID. Its control block holds the ID and, at
// userArgument, the user argument (UserZoneOf). The error is
// vmheap.ErrNoMemory when P0 has no room for the control block. The
// RTL's own zones are made first, as for CreateZone.
func (env *Environment) CreateUserZone(u UserZone) (uint32, error) {
	if err := env.systemZones(); err != nil {
		return 0, err
	}

	cb := userControlBase + nameSize(u.Name)

	id, err := env.controlBlock(cb)
	if err != nil {
		return 0, err
	}

	if err := env.mem.StoreLongword(env.cpu, id+userArgument, u.Argument); err != nil {
		env.dropControlBlock(id, cb)

		return 0, err
	}

	env.zones[id] = &heapZone{id: id, cb: cb, entry: id, user: &u, settings: ZoneSettings{Name: u.Name}}

	return id, nil
}

// UserZoneOf returns the settings of the user-defined zone id names, the
// address of the longword in its control block that holds its user
// argument (passed to its routines by reference), and whether id names
// a user-defined zone.
func (env *Environment) UserZoneOf(id uint32) (UserZone, uint32, bool) {
	z, ok := env.zones[id]
	if !ok || z.user == nil {
		return UserZone{}, 0, false
	}

	return *z.user, id + userArgument, true
}

// DeleteUserZone forgets the user-defined zone id names, freeing its
// control block, for LIB$DELETE_VM_ZONE, which then calls the zone's
// delete routine (or, without one, returns LIB$_INVOPEZON: the zone is
// gone either way, as on VMS 7.1, testdata/lib59). The error is
// ErrBadZone when id names no user-defined zone.
func (env *Environment) DeleteUserZone(id uint32) error {
	z, ok := env.zones[id]
	if !ok || z.user == nil {
		return ErrBadZone
	}

	env.forget(z)

	return nil
}

// ZoneCallsBack reports whether an allocation from the zone id names,
// or its deletion, may call the program's page routines (ZoneSettings'
// GetPage and FreePage): the routine doing it must be able to call back
// (CallingBack).
func (env *Environment) ZoneCallsBack(id uint32) bool {
	z, ok := env.zones[id]

	return ok && z.zone != nil && (z.settings.GetPage != 0 || z.settings.FreePage != 0)
}

// The RTL's two zones, with VMS 7.1's attributes (testdata/lib59's
// LIB$SHOW_VM_ZONE of each): the default zone, first fit, quadwords,
// EXTEND_AREA, an initial area of 124 pages made at its first
// allocation and 128-page areas after it, and an 80-byte control block;
// the string zone, quick fit with 17 lookaside lists from 16 bytes,
// boundary tags, EXTEND_AREA, an initial area of 4 pages made at its
// first allocation and 32-page areas after it, and a 216-byte control
// block. Their names are the manual's (LIB$FIND_VM_ZONE's example).
var (
	defaultZone = ZoneSettings{
		Attributes: vmheap.Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 128, InitialPages: 124,
			DeferInitial: true, ExtendArea: true},
		Name: "DEFAULT_ZONE",
	}

	stringZone = ZoneSettings{
		Attributes: vmheap.Attributes{Algorithm: vmheap.QuickFit, Lists: 17, Smallest: 16, BlockSize: 8,
			Alignment: 8, BoundaryTags: true, ExtendPages: 32, InitialPages: 4, DeferInitial: true,
			ExtendArea: true},
		Name: "STRING_ZONE",
	}
)

// The system zones' control block sizes: VMS 7.1's (its LIB$SHOW_VM_ZONE).
const (
	defaultZoneControl = 80
	stringZoneControl  = 216
)

// systemZones makes the RTL's two zones, the default zone and then the
// string zone, unless they're made already. Their control blocks are
// the RTL's own on VMS (the zones exist before a program runs), so here
// they're in a page of P0 of their own, apart from the pool and
// uncounted. They're made together at the image's first allocation, from
// either.
func (env *Environment) systemZones() error {
	if env.defaultHeap != nil {
		return nil
	}

	page, ok := env.expandP0(1)
	if !ok {
		return vmheap.ErrNoMemory
	}

	if env.zones == nil {
		env.zones = map[uint32]*heapZone{}
	}

	var made []*heapZone

	for _, sys := range []struct {
		s   ZoneSettings
		id  uint32
		cb  uint32
		ptr **heapZone
	}{
		{defaultZone, page, defaultZoneControl, &env.defaultHeap},
		{stringZone, page + 128, stringZoneControl, &env.stringHeap},
	} {
		if err := env.mem.StoreLongword(env.cpu, sys.id, sys.id); err != nil {
			return err
		}

		z, err := env.newHeapZone(sys.id, sys.s)
		if err != nil {
			for _, m := range made {
				delete(env.zones, m.id)
			}

			return err
		}

		z.cb, z.system = sys.cb, true
		made = append(made, z)
	}

	env.defaultHeap, env.stringHeap = made[0], made[1]

	return nil
}

// defaultZone returns the image's default zone, making the RTL's zones
// at the image's first allocation. The error is vmheap.ErrNoMemory when
// P0 has no room for them.
func (env *Environment) defaultZone() (*heapZone, error) {
	if err := env.systemZones(); err != nil {
		return nil, err
	}

	return env.defaultHeap, nil
}

// stringZone returns the image's string zone, as defaultZone does the
// default zone.
func (env *Environment) stringZone() (*heapZone, error) {
	if err := env.systemZones(); err != nil {
		return nil, err
	}

	return env.stringHeap, nil
}

// zoneFor returns the zone id names: 0 is the default zone, anything
// else a zone of env.zones (the default zone's own ID among them).
func (env *Environment) zoneFor(id uint32) (*heapZone, error) {
	if id == 0 {
		return env.defaultZone()
	}

	z, ok := env.zones[id]

	switch {
	case !ok:
		return nil, ErrBadZone
	case z.user != nil:
		return nil, ErrUserZone
	}

	return z, nil
}

// madeZone returns the zone id names, which must be one CreateZone made:
// the RTL's zones can't be reset or deleted (LIB$_BADBLOADR on VMS 7.1,
// testdata/lib59).
func (env *Environment) madeZone(id uint32) (*heapZone, error) {
	z, ok := env.zones[id]

	switch {
	case !ok || z.system:
		return nil, ErrBadZone
	case z.user != nil:
		return nil, ErrUserZone
	}

	return z, nil
}

// ZoneSettingsOf returns the settings of the zone id names (0, the
// default zone), and whether it names one.
func (env *Environment) ZoneSettingsOf(id uint32) (ZoneSettings, bool) {
	z, err := env.zoneFor(id)
	if err != nil {
		return ZoneSettings{}, false
	}

	return z.settings, true
}

// ZoneTagged reports whether the zone id names (0, the default zone) is
// one whose blocks have boundary tags, so that LIB$FREE_VM needn't be
// told a block's size; and whether id names a zone the RTL manages.
func (env *Environment) ZoneTagged(id uint32) (tagged, ok bool) {
	z, err := env.zoneFor(id)
	if err != nil {
		return false, false
	}

	return z.zone.Tagged(), true
}

// ZoneAllocate allocates size bytes from the zone id names (0, the
// default zone) for LIB$GET_VM, returning the block's address: tagged and
// filled as the zone's settings ask, and counted. The error is
// ErrBadZone, or vmheap's ErrBadSize, ErrNoMemory, or ErrPageLimit;
// nothing is allocated then.
func (env *Environment) ZoneAllocate(id, size uint32) (uint32, error) {
	z, err := env.zoneFor(id)
	if err != nil {
		return 0, err
	}

	addr, err := env.allocateIn(z, size)
	if err != nil {
		return 0, err
	}

	env.vmCounts.Gets++
	env.vmCounts.Bytes += z.zone.Nominal(size)

	return addr, nil
}

// allocateIn allocates size bytes from z, its tags written and its bytes
// filled as z's settings ask.
func (env *Environment) allocateIn(z *heapZone, size uint32) (uint32, error) {
	addr, err := z.zone.Allocate(size)
	if err != nil {
		return 0, err
	}

	if before, after, n, ok := z.zone.Tags(addr); ok {
		_ = env.mem.StoreLongword(env.cpu, before, allocatedTag|n|tagAllocated)
		_ = env.mem.StoreLongword(env.cpu, after, z.id)
	}

	n, _ := z.zone.BlockSize(addr)

	if err := env.fillBlock(addr, n, z.settings.GetFill); err != nil {
		_, _ = z.zone.Free(addr)

		return 0, err
	}

	return addr, nil
}

// A tagged block's first tag, as VMS 7.1 writes it (testdata/lib59): the
// bytes the block occupies, with bit 30 and a 2 in its low bits while
// it's allocated, and a 3 there when it's free. Its second tag holds its
// zone's ID while it's allocated (on VMS, a pointer to its area), and
// the first tag's copy when it's free.
const (
	allocatedTag = 0x40000000
	tagAllocated = 2
	tagFree      = 3
)

// ZoneFree frees the block at addr in the zone id names (0, the default
// zone) for LIB$FREE_VM, filling it as the zone's settings ask, and
// counts it: as a call when the bytes are the zone's (a double free
// too, as VMS 7.1 counts it, testdata/lib59), and its bytes when it
// succeeds.
//
// An untagged zone trusts size, the caller's figure (vmheap's
// FreeRange): the bytes from addr, size rounded, are freed, as on VMS
// 7.1. A tagged zone reads the block's first tag (sized or not): one
// that doesn't say the block is allocated is ErrBadTag. The errors:
// ErrBadZone; vmheap.ErrBadAddress for bytes that aren't the zone's
// blocks; vmheap.ErrBadSize for a size past a fixed size zone's;
// ErrBadTag. A block on a lookaside list freed again is no error, as on
// VMS, whose list then gives that block out forever (FreeAgain). Nothing
// is freed after an error.
func (env *Environment) ZoneFree(id, addr, size uint32, sized bool) error {
	z, err := env.zoneFor(id)
	if err != nil {
		return err
	}

	var freed uint32

	if z.zone.Tagged() {
		freed, err = env.freeTagged(z, addr)
	} else {
		if !sized {
			// The RTL's own callers only: LIB$FREE_VM reads the size.
			size, _ = z.zone.BlockSize(addr)
		}

		freed, err = z.zone.FreeRange(addr, size)
	}

	// VMS 7.1 counts a free that found the bytes in the zone, though
	// they were free already, not one outside it or misaligned, nor a
	// bad tag (testdata/lib59).
	switch {
	case err == nil, errors.Is(err, vmheap.ErrFreedAgain), errors.Is(err, vmheap.ErrDoubleFree):
		env.vmCounts.Frees++
	}

	if errors.Is(err, vmheap.ErrFreedAgain) {
		z.zone.FreeAgain(addr)

		return nil
	}

	if err != nil {
		return err
	}

	env.vmCounts.Bytes -= freed

	return nil
}

// freeTagged frees the tagged block at addr in z after checking its
// first tag, returning the bytes to count.
func (env *Environment) freeTagged(z *heapZone, addr uint32) (uint32, error) {
	before, _, _, ok := z.zone.Tags(addr)
	if !ok {
		if z.zone.Freed(addr) {
			return 0, ErrBadTag
		}

		return 0, vmheap.ErrBadAddress
	}

	tag, err := env.mem.LoadLongword(env.cpu, before)
	if err != nil {
		return 0, err
	}

	if tag&allocatedTag == 0 {
		return 0, ErrBadTag
	}

	class, listed := z.zone.Class(addr)
	if !listed {
		return 0, vmheap.ErrFreedAgain
	}

	if _, err := z.zone.Free(addr); err != nil {
		return 0, err
	}

	return class + 2*vmheap.TagSize, nil
}

// watchFreeSpace has z's allocator tell the RTL of the free space each
// free makes, so that it's filled (with z's free fill) and holds VMS's
// control words (freeWords).
func (env *Environment) watchFreeSpace(z *heapZone) {
	fill := z.settings.FreeFill
	tagged := z.zone.Tagged()

	z.zone.OnRelease(func(freed, merged vmheap.Span) {
		if freed.Size() > 0 {
			if tagged {
				// What was a span's tags and links is inside the merged
				// span now: refill all of it.
				_ = env.fillBlock(merged.Start, merged.Size(), fill)
			} else {
				_ = env.fillBlock(freed.Start, freed.Size(), fill)

				// The start of a span merged after it.
				if merged.End > freed.End {
					_ = env.fillBlock(freed.End, freeWordBytes, fill)
				}
			}
		}

		env.freeWords(z, merged)
	})

	z.zone.OnList(func(s vmheap.Span) {
		_ = env.fillBlock(s.Start, s.Size(), fill)
		env.freeWords(z, s)
	})
}

// freeWordBytes is how many bytes at the start of an untagged zone's
// free space hold control words, not the free fill.
const freeWordBytes = 8

// freeWords writes the control words VMS 7.1 keeps in free space s of z
// (testdata/lib59: the bytes that don't hold the free fill): untagged,
// the span's size and a link at its start; tagged, its first tag (its size
// and 3), two links, and a copy of the first tag at its end. The links
// are govax's, not VMS's: a free span's are its zone's ID.
func (env *Environment) freeWords(z *heapZone, s vmheap.Span) {
	if s.Size() < freeWordBytes {
		return
	}

	if !z.zone.Tagged() {
		_ = env.mem.StoreLongword(env.cpu, s.Start, s.Size())
		_ = env.mem.StoreLongword(env.cpu, s.Start+4, z.id)

		return
	}

	tag := s.Size() | tagFree

	_ = env.mem.StoreLongword(env.cpu, s.Start, tag)
	_ = env.mem.StoreLongword(env.cpu, s.Start+4, z.id)

	if s.Size() >= 16 {
		_ = env.mem.StoreLongword(env.cpu, s.Start+8, z.id)
		_ = env.mem.StoreLongword(env.cpu, s.End-4, tag)
	}
}

// controlWords returns the parts of free space s of z that hold control
// words rather than the free fill (freeWords), for VerifyZone.
func controlWords(z *heapZone, s vmheap.Span) []vmheap.Span {
	if s.Size() < freeWordBytes {
		return nil
	}

	if !z.zone.Tagged() {
		return []vmheap.Span{{Start: s.Start, End: s.Start + freeWordBytes}}
	}

	if s.Size() < 16 {
		return []vmheap.Span{{Start: s.Start, End: s.Start + 8}}
	}

	return []vmheap.Span{{Start: s.Start, End: s.Start + 12}, {Start: s.End - 4, End: s.End}}
}

// ResetZone frees every block of the zone id names, keeping its pages,
// for LIB$RESET_VM_ZONE: the areas are filled whole with the free fill,
// and the blocks' bytes come off the statistics. The error is ErrBadZone
// for an ID that names no zone CreateZone made.
func (env *Environment) ResetZone(id uint32) error {
	z, err := env.madeZone(id)
	if err != nil {
		return err
	}

	return env.reset(z)
}

// reset empties z (ResetZone).
func (env *Environment) reset(z *heapZone) error {
	areas := z.zone.AreaReports()

	env.vmCounts.Bytes -= env.liveBytes(z.zone.Reset(), z.zone.Tagged())

	for _, a := range areas {
		if err := env.fillBlock(a.Base, a.Top-a.Base, z.settings.FreeFill); err != nil {
			return err
		}
	}

	return nil
}

// liveBytes is the bytes the statistics count for blocks.
func (env *Environment) liveBytes(blocks []vmheap.Block, tagged bool) uint32 {
	var n uint32

	for _, b := range blocks {
		n += b.Class
		if tagged {
			n += 2 * vmheap.TagSize
		}
	}

	return n
}

// DeleteZone frees every block of the zone id names, gives its pages
// back to the page pool (or to the program's free-page routine: its
// areas, then its control block's page), and forgets it, for
// LIB$DELETE_VM_ZONE. Its control block is freed, its mark cleared, so
// the ID is invalid afterwards (until another zone's control block is
// put there). The error is ErrBadZone for an ID that names no zone
// CreateZone made; the zone is gone after any other (a fill that
// couldn't be written, or a PageRoutineFailure, after which, as on VMS
// 7.1, no more pages are given back).
func (env *Environment) DeleteZone(id uint32) error {
	z, err := env.madeZone(id)
	if err != nil {
		return err
	}

	// The zone goes first, so that the program's free-page routine,
	// called below, can't reach it.
	env.forget(z)

	err = env.reset(z)

	if _, perr := z.zone.Delete(); perr != nil {
		return perr
	}

	if z.entry != z.id {
		if z.settings.FreePage != 0 {
			if _, perr := env.callPageRoutine(z.settings.FreePage, 1, z.id); perr != nil {
				return perr
			}
		} else {
			_ = env.pagePool().Free(1, z.id)
		}
	}

	return err
}

// fillBlock fills the n bytes at addr with fill, by longwords where it
// can.
func (env *Environment) fillBlock(addr, n uint32, fill Fill) error {
	var b byte

	switch fill {
	case NoFill:
		return nil
	case FillOnes:
		b = 0xFF
	}

	word := uint32(b) * 0x01010101
	end := addr + n

	for addr < end {
		var err error

		if addr%4 == 0 && end-addr >= 4 {
			err = env.mem.StoreLongword(env.cpu, addr, word)
			addr += 4
		} else {
			err = env.mem.StoreByte(env.cpu, addr, b)
			addr++
		}

		if err != nil {
			return err
		}
	}

	return nil
}
