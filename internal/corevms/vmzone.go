package corevms

import (
	"errors"

	"github.com/tucats/govax/internal/vmheap"
)

// The image's zones: the heaps LIB$CREATE_VM_ZONE makes, each a
// vmheap.Zone over the image's page pool, which LIB$GET_VM and
// LIB$FREE_VM allocate from and LIB$RESET_VM_ZONE and LIB$DELETE_VM_ZONE
// empty and remove (5.0 LIB$ manual, chapter 5; docs/PHASE-59 - LIB VM
// zones.md, subtask 4).
//
// A zone's ID is the address of its control block, ZoneControlSize bytes
// in P0 (the phase doc's Decision 1), whose first longword holds the ID
// itself as a mark of validity. The zone's state is kept in Go, in
// env.zones, found by that address, so an ID that isn't a zone's is
// caught by the lookup, not by trusting memory the program can write.
// The control blocks come from a zone of their own, which no zone ID
// names, so the program can't free one with LIB$FREE_VM.
//
// What the zone allocator doesn't do because it never touches memory is
// done here: the fills (LIB$M_VM_GET_FILL0/1, FREE_FILL0/1) and the
// boundary tags' contents.

// ZoneControlSize is the size of a zone's control block. VMS's size
// isn't documented; this is govax's (unconfirmed).
const ZoneControlSize = 128

// Fill is what a zone fills blocks with as they're allocated or freed.
type Fill int

const (
	NoFill   Fill = iota // left as they are
	FillZero             // bytes of 0 (LIB$M_VM_GET_FILL0, FREE_FILL0)
	FillOnes             // bytes of FF (LIB$M_VM_GET_FILL1, FREE_FILL1)
)

// ZoneSettings are a zone's attributes as LIB$CREATE_VM_ZONE gives
// them: the allocator's, the fills, and the zone's name.
type ZoneSettings struct {
	vmheap.Attributes

	GetFill, FreeFill Fill
	Name              string
}

// heapZone is one zone of the image: its allocator, its settings, and
// its ID (0 for the default zone).
type heapZone struct {
	id       uint32
	zone     *vmheap.Zone
	settings ZoneSettings
}

var (
	// ErrBadZone is a zone ID that names no zone of the image, or names
	// the default zone where it can't be used (LIB$_BADBLOADR).
	ErrBadZone = errors.New("corevms: invalid zone ID")

	// ErrBadTag is a block whose boundary tags the program changed
	// (LIB$_BADTAGVAL).
	ErrBadTag = errors.New("corevms: a block's boundary tag was changed")
)

// controlZone is the zone the image's zones' control blocks are in,
// made at the first LIB$CREATE_VM_ZONE.
var controlZone = vmheap.Attributes{BlockSize: 8, Alignment: 8, ExtendPages: 1, ExtendArea: true}

// CreateZone makes a zone with settings s and returns its ID. The error
// is vmheap.ErrBadAttributes for attributes that break vmheap's rules,
// or vmheap.ErrNoMemory (or ErrPageLimit) when the initial area or the
// control block can't be had; no zone is made then.
func (env *Environment) CreateZone(s ZoneSettings) (uint32, error) {
	zone, err := vmheap.NewZone(env.pagePool(), s.Attributes)
	if err != nil {
		return 0, err
	}

	if env.zoneControl == nil {
		if env.zoneControl, err = vmheap.NewZone(env.pagePool(), controlZone); err != nil {
			panic(err) // controlZone's attributes are valid
		}
	}

	id, err := env.zoneControl.Allocate(ZoneControlSize)
	if err == nil {
		err = env.mem.StoreLongword(env.cpu, id, id)
		if err != nil {
			_, _ = env.zoneControl.Free(id)
		}
	}

	if err != nil {
		zone.Delete()

		return 0, err
	}

	if env.zones == nil {
		env.zones = map[uint32]*heapZone{}
	}

	env.zones[id] = &heapZone{id: id, zone: zone, settings: s}

	return id, nil
}

// zoneFor returns the zone id names: 0 is the default zone (the image's
// heap), anything else a zone CreateZone made.
func (env *Environment) zoneFor(id uint32) (*heapZone, error) {
	if id == 0 {
		return &heapZone{zone: env.heap(), settings: ZoneSettings{Attributes: defaultZone}}, nil
	}

	z, ok := env.zones[id]
	if !ok {
		return nil, ErrBadZone
	}

	return z, nil
}

// madeZone returns the zone id names, which must be one CreateZone made:
// the default zone can't be reset or deleted (5.0 LIB$ manual, section
// 5.3.3).
func (env *Environment) madeZone(id uint32) (*heapZone, error) {
	if id == 0 {
		return nil, ErrBadZone
	}

	return env.zoneFor(id)
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

// ZoneAllocate allocates size bytes from the zone id names (0, the
// default zone) for LIB$GET_VM, returning the block's address: tagged and
// filled as the zone's settings ask. The error is ErrBadZone, or
// vmheap's ErrBadSize, ErrNoMemory, or ErrPageLimit; nothing is
// allocated then.
func (env *Environment) ZoneAllocate(id, size uint32) (uint32, error) {
	z, err := env.zoneFor(id)
	if err != nil {
		return 0, err
	}

	addr, err := z.zone.Allocate(size)
	if err != nil {
		return 0, err
	}

	if err := env.prepareBlock(z, addr); err != nil {
		_, _ = z.zone.Free(addr)

		return 0, err
	}

	return addr, nil
}

// prepareBlock writes a new block's boundary tags and fills its bytes,
// as its zone's settings ask.
func (env *Environment) prepareBlock(z *heapZone, addr uint32) error {
	if before, after, value, ok := z.zone.Tags(addr); ok {
		if err := env.mem.StoreLongword(env.cpu, before, value); err != nil {
			return err
		}

		if err := env.mem.StoreLongword(env.cpu, after, value); err != nil {
			return err
		}
	}

	n, _ := z.zone.BlockSize(addr)

	return env.fillBlock(addr, n, z.settings.GetFill)
}

// ZoneFree frees the block at addr in the zone id names (0, the default
// zone) for LIB$FREE_VM, filling it as the zone's settings ask. When
// sized, size is the caller's idea of the block's size, which must round
// to the block's (and be no more than a fixed-size zone's size); without
// it the zone's own record is used. The errors: ErrBadZone;
// vmheap.ErrBadAddress for no block at addr (or one freed already, but
// to a lookaside list, which is no error, as on VMS); vmheap.ErrBadSize
// for a size that doesn't match; ErrBadTag for a tag the program changed.
// Nothing is freed after an error.
func (env *Environment) ZoneFree(id, addr, size uint32, sized bool) error {
	z, err := env.zoneFor(id)
	if err != nil {
		return err
	}

	if n, ok := z.zone.BlockSize(addr); ok {
		attr := z.zone.Attributes()

		if sized && (z.zone.Usable(size) != n || attr.Algorithm == vmheap.FixedSize && size > attr.FixedSize) {
			return vmheap.ErrBadSize
		}

		if err := env.checkTags(z, addr); err != nil {
			return err
		}
	}

	n, err := z.zone.Free(addr)
	if errors.Is(err, vmheap.ErrFreedAgain) {
		return nil
	}

	if err != nil {
		return err
	}

	return env.fillBlock(addr, n, z.settings.FreeFill)
}

// checkTags reports ErrBadTag when either of the block at addr's
// boundary tags no longer holds the block's size.
func (env *Environment) checkTags(z *heapZone, addr uint32) error {
	before, after, value, ok := z.zone.Tags(addr)
	if !ok {
		return nil
	}

	for _, tag := range []uint32{before, after} {
		v, err := env.mem.LoadLongword(env.cpu, tag)
		if err != nil {
			return err
		}

		if v != value {
			return ErrBadTag
		}
	}

	return nil
}

// ResetZone frees every block of the zone id names, keeping its pages,
// for LIB$RESET_VM_ZONE. The error is ErrBadZone for an ID that names
// no zone CreateZone made.
func (env *Environment) ResetZone(id uint32) error {
	z, err := env.madeZone(id)
	if err != nil {
		return err
	}

	return env.fillBlocks(z.zone.Reset(), z.settings.FreeFill)
}

// DeleteZone frees every block of the zone id names, gives its pages
// back to the page pool, and forgets it, for LIB$DELETE_VM_ZONE. Its
// control block is freed, its mark cleared, so the ID is invalid
// afterwards (until another zone's control block is put there). The
// error is ErrBadZone for an ID that names no zone CreateZone made.
func (env *Environment) DeleteZone(id uint32) error {
	z, err := env.madeZone(id)
	if err != nil {
		return err
	}

	// The blocks are filled while the pages are still the zone's.
	err = env.fillBlocks(z.zone.Reset(), z.settings.FreeFill)
	z.zone.Delete()

	delete(env.zones, id)
	_ = env.mem.StoreLongword(env.cpu, id, 0)
	_, _ = env.zoneControl.Free(id)

	return err
}

// fillBlocks fills each of blocks with fill.
func (env *Environment) fillBlocks(blocks []vmheap.Block, fill Fill) error {
	for _, b := range blocks {
		if err := env.fillBlock(b.Addr, b.Size, fill); err != nil {
			return err
		}
	}

	return nil
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
