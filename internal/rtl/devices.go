package rtl

import (
	"errors"
	"sort"
	"strings"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// Port of devices.c's sys_assign — built on Phase 09's
// internal/io.DeviceTable, deferred to this phase per docs/PHASE-09.md's
// own open questions (it needs the RTL calling convention and a
// process/PID/UIC concept, neither of which existed yet) — and the
// allocation and channel services docs/PHASE-26.md adds. devices.c's
// other service, sys_getdviw, became the full $GETDVI in getdvi.go.

// channel is one SYS$ASSIGN-created channel, the Go equivalent of devices.c's
// struct CHAN. Channel numbers count up by 8 starting at 8 (nextChannel += 8
// before use), matching find_device's own convention.
type channel struct {
	Name    string
	Number  uint16
	Class   iodev.DeviceClass
	Flags   uint32
	Mode    uint32 // access mode assigned from ($ASSIGN's acmode, maximized)
	Mailbox string
	Device  *iodev.Device
}

// findChannel looks up a channel by number, matching sys_getdviw's own
// linear scan of the channels list.
func (env *Environment) findChannel(number uint32) (*channel, bool) {
	for _, c := range env.channels {
		if uint32(c.Number) == number {
			return c, true
		}
	}

	return nil, false
}

// serviceSysAssign is SYS$ASSIGN: given a device name, creates and returns a
// channel number bound to that device.
//
// devices.c's own sys_assign ignores str_get's "descriptor too large for a
// 64-byte buffer" outcome (str_get leaves its buffer untouched and reports
// failure only via an out-parameter the caller never checks), which would
// read name[-1] a few lines later — a plain out-of-bounds bug, not an ISA
// judgment call (this is RTL/emulator tooling, not VAX ISA behavior, same as
// Phase 09's internal/io — see its own doc.go), so it's fixed here by
// reporting SS_BADPARAM instead of replicating the out-of-bounds access.
func serviceSysAssign(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 2 {
		return ssInsfArg, nil
	}

	if len(argv) > 5 {
		return ssTooManyArgs, nil
	}

	if argv[0] == 0 || argv[1] == 0 {
		return ssInsfArg, nil
	}

	name, ok, err := strGet(env, argv[0], 64)
	if err != nil {
		return ssAccVio, nil
	}

	if !ok {
		return ssBadParam, nil
	}

	device, st := env.deviceName(name)
	if st != 0 {
		return st, nil
	}

	dp, found := env.Devices.Find(device)
	if !found {
		return ssIvDevNam, nil
	}

	if dp.Allocated() && dp.PID != env.Process.PID {
		return ssDevAlloc, nil
	}

	env.nextChannel += 8
	c := &channel{
		Name:   name,
		Number: uint16(env.nextChannel),
		Device: dp,
		Class:  dp.DevClass,
		Mode:   max(optArg(argv, 2)&3, uint32(env.cpu.PSL().CurMod())),
	}

	if err := env.mem.StoreWord(env.cpu, argv[1], c.Number); err != nil {
		return ssAccVio, nil
	}

	if len(argv) > 3 && argv[3] != 0 {
		mbx, ok, err := strGet(env, argv[3], 255)
		if err != nil {
			return ssAccVio, nil
		}

		if !ok {
			return ssBadParam, nil
		}

		c.Mailbox = mbx
	}

	if len(argv) > 4 {
		c.Flags = argv[4]
	}

	env.channels = append(env.channels, c)
	dp.RefCnt++
	dp.PID = env.Process.PID
	dp.OwnUIC = env.Process.UIC

	return ssNormal, nil
}

// Status codes and $DEVDEF bits $ALLOC uses (docs/PHASE-26.md).
var (
	ssDevAlloc    = vmsdef.SSConstants["SS$_DEVALLOC"]
	ssDevAlrAlloc = vmsdef.SSConstants["SS$_DEVALRALLOC"]
	ssDevMount    = vmsdef.SSConstants["SS$_DEVMOUNT"]
	ssIvStsFlg    = vmsdef.SSConstants["SS$_IVSTSFLG"]
	ssNoDevAvl    = vmsdef.SSConstants["SS$_NODEVAVL"]
	ssDevAssign   = vmsdef.SSConstants["SS$_DEVASSIGN"]
	ssDevNotAlloc = vmsdef.SSConstants["SS$_DEVNOTALLOC"]

	devMounted = vmsdef.DEVConstants["DEV$M_MNT"]
	devMailbox = vmsdef.DEVConstants["DEV$M_MBX"]
)

// allocGeneric is $ALLOC's one flags bit: devnam names a device type
// (RA81, TU58, ...) rather than a device, and the first available device
// of that type is allocated.
const allocGeneric = 1

// maxDeviceNameLength is the longest devnam $ALLOC accepts (SS$_IVLOGNAM
// beyond it).
const maxDeviceNameLength = 63

// serviceSysAlloc is SYS$ALLOC: allocates a device to the calling process
// (env.Process) for its exclusive use, marking it DEV$M_ALL with the
// process's PID and the allocation's access mode. devnam may be a logical
// name. The physical name ("_DUA0:") is returned through phylen/phybuf
// (both optional). acmode is maximized with the caller's mode. flags bit 0
// asks for the first available device of the type devnam names.
//
// A device already allocated to this process succeeds with
// SS$_DEVALRALLOC; one allocated to another PID fails with SS$_DEVALLOC.
// A mounted device or a mailbox fails with SS$_DEVMOUNT.
func serviceSysAlloc(env *Environment, argv []uint32) (uint32, error) {
	devnam, phylen, phybuf := optArg(argv, 0), optArg(argv, 1), optArg(argv, 2)
	acmode, flags := optArg(argv, 3), optArg(argv, 4)

	if flags&^allocGeneric != 0 {
		return ssIvStsFlg, nil
	}

	if devnam == 0 {
		return ssIvDevNam, nil
	}

	name, ok, err := strGet(env, devnam, maxDeviceNameLength)
	if err != nil {
		return ssAccVio, nil
	}

	if !ok || name == "" {
		return ssIvLogNam, nil
	}

	device, st := env.deviceName(name)
	if st != 0 {
		return st, nil
	}

	var dp *iodev.Device

	if flags&allocGeneric != 0 {
		if dp, st = env.genericDevice(device); st != 0 {
			return st, nil
		}
	} else {
		found := false
		if dp, found = env.Devices.Find(device); !found {
			return ssNoSuchDev, nil
		}

		if st := env.allocatable(dp); st != 0 && st != ssDevAlrAlloc {
			return st, nil
		}
	}

	status := uint32(ssNormal)
	if dp.Allocated() {
		status = ssDevAlrAlloc
	} else {
		dp.Allocate(env.Process.PID, max(acmode&3, uint32(env.cpu.PSL().CurMod())))
	}

	phyName := "_" + dp.Name + ":"

	if phybuf != 0 {
		n, truncated, err := storeDescriptor(env, phybuf, phyName)
		if err != nil {
			return ssAccVio, nil
		}

		if phylen != 0 {
			if err := env.mem.StoreWord(env.cpu, phylen, n); err != nil {
				return ssAccVio, nil
			}
		}

		if truncated && status == ssNormal {
			status = ssBufferOvf
		}
	}

	return status, nil
}

// allocatable reports why the calling process can't allocate dp (0 if it
// can): SS$_DEVALRALLOC if it already has, SS$_DEVALLOC if another
// process has, SS$_DEVMOUNT if it's mounted or a mailbox.
func (env *Environment) allocatable(dp *iodev.Device) uint32 {
	if dp.Allocated() {
		if dp.PID == env.Process.PID {
			return ssDevAlrAlloc
		}

		return ssDevAlloc
	}

	if dp.DevChar&(devMounted|devMailbox) != 0 {
		return ssDevMount
	}

	if env.Mounts != nil {
		if _, mounted := env.Mounts.Lookup(dp.Name); mounted {
			return ssDevMount
		}
	}

	return 0
}

// genericDevice picks the device a generic $ALLOC of type typeName gets:
// the first unallocated, unmounted one, by name. A device the process
// already holds doesn't count as available. SS$_NODEVAVL if devices of
// that type exist but none is available, SS$_NOSUCHDEV if there are none.
func (env *Environment) genericDevice(typeName string) (*iodev.Device, uint32) {
	var candidates []*iodev.Device

	for _, d := range env.Devices.All() {
		if t, ok := iodev.DeviceTypeName(d.DevType); ok && strings.EqualFold(t, typeName) {
			candidates = append(candidates, d)
		}
	}

	if len(candidates) == 0 {
		return nil, ssNoSuchDev
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Name < candidates[j].Name })

	for _, d := range candidates {
		if env.allocatable(d) == 0 {
			return d, 0
		}
	}

	return nil, ssNoDevAvl
}

// serviceSysDalloc is SYS$DALLOC: deallocates a device the calling
// process allocated. acmode is maximized with the caller's mode, and only
// an allocation made in that mode or a less privileged one may be
// released (SS$_NOPRIV otherwise). A device the process still has a
// channel to stays allocated (SS$_DEVASSIGN); deallocating a mailbox
// succeeds without doing anything.
//
// With devnam omitted, every device the process allocated in acmode or a
// less privileged mode is deallocated, silently skipping any it can't
// release, and the call succeeds.
func serviceSysDalloc(env *Environment, argv []uint32) (uint32, error) {
	devnam := optArg(argv, 0)
	mode := max(optArg(argv, 1)&3, uint32(env.cpu.PSL().CurMod()))

	if devnam == 0 {
		env.deallocateAll(mode)

		return ssNormal, nil
	}

	name, ok, err := strGet(env, devnam, maxDeviceNameLength)
	if err != nil {
		return ssAccVio, nil
	}

	if !ok || name == "" {
		return ssIvLogNam, nil
	}

	device, st := env.deviceName(name)
	if st != 0 {
		return st, nil
	}

	dp, found := env.Devices.Find(device)
	if !found {
		return ssNoSuchDev, nil
	}

	switch {
	case dp.DevChar&devMailbox != 0:
		return ssNormal, nil
	case !dp.Allocated() || dp.PID != env.Process.PID:
		return ssDevNotAlloc, nil
	case dp.AllocMode < mode:
		return ssNoPriv, nil
	case env.hasChannel(dp):
		return ssDevAssign, nil
	}

	dp.Deallocate()

	return ssNormal, nil
}

// serviceSysDassgn is SYS$DASSGN: releases a channel $ASSIGN created.
// Only the low word of chan counts; 0 is SS$_IVCHAN. A channel that isn't
// assigned, or was assigned from a more privileged access mode than the
// caller's, is SS$_NOPRIV, as the manual says. govax channels carry no
// I/O requests, open files, or network links, so releasing one only
// drops the device's reference count (and, once nothing references an
// unallocated device, its owner PID).
func serviceSysDassgn(env *Environment, argv []uint32) (uint32, error) {
	number := optArg(argv, 0) & 0xFFFF
	if number == 0 {
		return ssIvChan, nil
	}

	c, found := env.findChannel(number)
	if !found || c.Mode < uint32(env.cpu.PSL().CurMod()) {
		return ssNoPriv, nil
	}

	env.releaseChannel(c)

	return ssNormal, nil
}

// releaseChannel removes c from the process's channels and drops its
// device reference. Its CTRL/C and CTRL/Y ASTs are cancelled.
func (env *Environment) releaseChannel(c *channel) {
	env.disarmChannel(c.Number)

	for i, ch := range env.channels {
		if ch == c {
			env.channels = append(env.channels[:i], env.channels[i+1:]...)

			break
		}
	}

	d := c.Device
	if d.RefCnt > 0 {
		d.RefCnt--
	}

	if d.RefCnt == 0 && !d.Allocated() {
		d.PID = 0
	}
}

// deassignUserChannels is image rundown's channel step: VMS deassigns the
// channels an image assigned from user mode when it exits.
func (env *Environment) deassignUserChannels() {
	for _, c := range append([]*channel(nil), env.channels...) {
		if c.Mode == uint32(vax.User) {
			env.releaseChannel(c)
		}
	}
}

// hasChannel reports whether the process has a channel assigned to d.
func (env *Environment) hasChannel(d *iodev.Device) bool {
	for _, c := range env.channels {
		if c.Device == d {
			return true
		}
	}

	return false
}

// deallocateUserDevices is image rundown's device step: VMS deallocates
// the devices an image allocated in user mode when the image exits — the
// same as $DALLOC with no device name at user mode, so a device the
// process still has a (more privileged) channel to stays allocated.
func (env *Environment) deallocateUserDevices() {
	env.deallocateAll(uint32(vax.User))
}

// deallocateAll releases every device the process allocated in mode or a
// less privileged one, except those it still has a channel to.
func (env *Environment) deallocateAll(mode uint32) {
	for _, d := range env.Devices.All() {
		if d.Allocated() && d.PID == env.Process.PID && d.AllocMode >= mode && !env.hasChannel(d) {
			d.Deallocate()
		}
	}
}

func registerDeviceServices(t *ServiceTable) {
	t.Register("SYS$ASSIGN", serviceSysAssign)
	t.Register("SYS$ALLOC", serviceSysAlloc)
	t.Register("SYS$DALLOC", serviceSysDalloc)
	t.Register("SYS$DASSGN", serviceSysDassgn)
}

// deviceName translates a $ASSIGN/$GETDVI device name through its logical
// names (SYS$OUTPUT becomes TTA0, as on VMS) into a physical device name
// for Devices.Find; a leading "_" suppresses translation
// (docs/PHASE-25.md). A name that can't be translated is reported as its
// lnm status (SS$_TOOMANYLNAM), and one that names no device at all as
// SS$_IVDEVNAM.
func (env *Environment) deviceName(name string) (string, uint32) {
	device, err := rms.PhysicalDevice(env.Logicals, name)
	if err != nil {
		var lne *rms.LogicalNameError
		if errors.As(err, &lne) {
			st, _ := lnmStatus(lne.Err)

			return "", st
		}

		return "", ssIvDevNam
	}

	return device, 0
}
