package dclcmd

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/tucats/gopackages/app-cli/settings"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file is DEFINE/DEVICE of an MD device, a RAM disk
// (docs/planning/PHASE-96 - RAM disk.md, Decision 5): an ODS-2 volume
// whose blocks are kept in govax's own memory, named as VMS's DECram
// disks are, MDA0, MDA1, and so on. RMS keeps each disk's memory
// (internal/rms's ramdisk.go); this file chooses the unit, its size, and
// holds the disks to their limit, and gives the device its record.
//
// A RAM disk's size counts against a limit on all of them together, so
// that a user can't use up the host's memory by mistake. The limit counts
// each disk's defined size, not what's on it now: the memory is taken only
// as blocks are written, but a disk can't grow past what was allowed when
// it was defined.

// The configuration keys for RAM disks (HELP CONFIGURATION KEYS); each is
// in cmd/govax's validConfigs.
const (
	// RAMDiskSizeSetting is a disk's size, in blocks, when DEFINE/DEVICE
	// gives no /MAXBLOCK.
	RAMDiskSizeSetting = "vax.ramdisk.size"

	// RAMDiskLimitSetting is the most blocks all RAM disks may total.
	RAMDiskLimitSetting = "vax.ramdisk.limit"
)

// The keys' defaults: a disk of 65,536 blocks (32 MB), and a total of
// 512,000 blocks (250 MB).
const (
	DefaultRAMDiskSize  = 65536
	DefaultRAMDiskLimit = 512000
)

// RAMDiskKeys returns the RAM disk keys, for cmd/govax's validConfigs and
// its audit, which checks each with ParseRAMDiskBlocks.
func RAMDiskKeys() []string {
	return []string{RAMDiskSizeSetting, RAMDiskLimitSetting}
}

// ParseRAMDiskBlocks reads a RAM disk key's value: a number of blocks, in
// decimal or as %X and hex digits, at least rms.MinimumMemoryBlocks. It
// reports false for anything else.
func ParseRAMDiskBlocks(text string) (uint32, bool) {
	text = strings.TrimSpace(text)
	base := 10

	if len(text) > 2 && strings.EqualFold(text[:2], "%x") {
		text, base = text[2:], 16
	}

	v, err := strconv.ParseUint(text, base, 32)
	if err != nil || v < rms.MinimumMemoryBlocks {
		return 0, false
	}

	return uint32(v), true
}

// ramDiskSetting is key's value, read now, or def when it isn't set or
// isn't a value ParseRAMDiskBlocks takes (which govax warned of at
// startup).
func ramDiskSetting(key string, def uint32) uint32 {
	if !settings.Exists(key) {
		return def
	}

	if v, ok := ParseRAMDiskBlocks(settings.Get(key)); ok {
		return v
	}

	return def
}

// ramDiskName matches an MD device's name: MDc, a controller, or MDcu, a
// unit, with any leading "_" and trailing ":" already taken off.
var ramDiskName = regexp.MustCompile(`^MD[A-Z][0-9]*$`)

// RAMDiskName reports whether name (as DEFINE/DEVICE or MOUNT was given
// it, perhaps "_MDA0:") names an MD device, and returns it in its plain
// form, "MDA0" or "MDA".
func RAMDiskName(name string) (string, bool) {
	name = strings.ToUpper(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(name), "_"), ":"))

	return name, ramDiskName.MatchString(name)
}

// ramDiskChar is an MD device's DEVCHAR: an ordinary disk's
// characteristics (file-oriented, directory-structured, random access,
// available, shareable, input and output). $GETDVI adds MNT, and SWL for a
// read-only mount, while a volume is mounted.
var ramDiskChar = func() uint32 {
	var c uint32

	for _, bit := range []string{"FOD", "DIR", "RND", "AVL", "SHR", "IDV", "ODV"} {
		c |= vmsdef.Symbols["DEV$M_"+bit]
	}

	return c
}()

// DefineDeviceCommand is the console's DEFINE/DEVICE: a RAM disk for an
// MD device's name (defineRAMDisk), and otherwise a device record with
// opts' settings, as it has always been (DefineDevice). log is false for
// /NOLOG, which only an MD device's definition reads.
func (c *Context) DefineDeviceCommand(name string, opts iodev.DeviceOptions, log bool) error {
	if plain, ok := RAMDiskName(name); ok {
		return c.defineRAMDisk(plain, opts, log)
	}

	c.DefineDevice(name, opts)

	return nil
}

// defineRAMDisk defines the MD device name (MDA, the controller's lowest
// unit not defined yet; MDA3, that unit) as a RAM disk:
//
//   - the process needs CMKRNL (%SYSTEM-F-NOPRIV);
//   - a /DEVCLASS or /DEVTYPE other than a disk and RAM_DISK conflicts
//     (%CLI-W-CONFLICT);
//   - its size is /MAXBLOCK, or vax.ramdisk.size, or 65,536 blocks, and
//     at least 100 (%VAX-E-RAMSIZE);
//   - it mustn't be mounted (%SYSTEM-F-DEVMOUNT);
//   - with it at that size, the RAM disks mustn't total more than
//     vax.ramdisk.limit blocks (%VAX-E-RAMLIMIT and -VAX-I-RAMKEY).
//
// A unit defined again loses the volume it held and gets its new size.
// Its record is a disk's, of type DT$_RAM_DISK, with /CLUSTER and
// /VOLNAME kept for its first mount; DEFINE/DEVICE's other qualifiers are
// kept as for any device. Unless /NOLOG, a verbose console says which unit
// was made:
//
//	%VAX-I-RAMDISK, _MDA0: defined, 20000 blocks
func (c *Context) defineRAMDisk(name string, opts iodev.DeviceOptions, log bool) error {
	if err := c.commandPrivilege("DEFINE_DEVICE_MD"); err != nil {
		return err
	}

	if (opts.DevClass != 0 && opts.DevClass != iodev.DeviceClassDisk) ||
		(opts.DevType != 0 && opts.DevType != iodev.DeviceTypeRAMDisk) {
		return vmserrors.New(vmserrors.CLI_CONFLICT)
	}

	unit := c.ramDiskUnit(name)

	size := opts.MaxBlock
	if size == 0 {
		size = ramDiskSetting(RAMDiskSizeSetting, DefaultRAMDiskSize)
	}

	if size < rms.MinimumMemoryBlocks {
		return vmserrors.New(vmserrors.VAX_RAMSIZE, uint32(rms.MinimumMemoryBlocks), size)
	}

	if _, mounted := c.Mounts.Lookup(unit); mounted {
		return vmserrors.New(vmserrors.SS_DEVMOUNT, unit)
	}

	discarded := c.Mounts.MemoryHasVolume(unit)
	limit := ramDiskSetting(RAMDiskLimitSetting, DefaultRAMDiskLimit)

	if err := c.Mounts.DefineMemory(unit, size, limit); err != nil {
		return c.ramDiskRefusal(err, unit)
	}

	opts.DevClass = iodev.DeviceClassDisk
	opts.DevType = iodev.DeviceTypeRAMDisk
	opts.DevChar |= ramDiskChar
	opts.MaxBlock = size
	c.Devices.Redefine(unit, opts)

	if log {
		tail := ""
		if discarded {
			tail = ", previous contents discarded"
		}

		c.info(vmserrors.New(vmserrors.VAX_RAMDISK, "_"+unit+":", size, tail))
	}

	return nil
}

// ramDiskUnit is the unit name names: itself if it has a unit number, and
// for a controller alone (MDA) the controller's lowest unit that isn't a
// device yet. Unlike MOUNT's ResolveUnit, which takes the lowest unit
// with nothing mounted, this takes one not defined, since a RAM disk must
// be defined before it's mounted.
func (c *Context) ramDiskUnit(name string) string {
	if last := name[len(name)-1]; last >= '0' && last <= '9' {
		return name
	}

	for n := 0; ; n++ {
		unit := fmt.Sprintf("%s%d", name, n)
		if _, found := c.Devices.Find(unit); !found {
			return unit
		}
	}
}

// ramDiskRefusal turns DefineMemory's failure into the command's. Over
// the limit, it shows the two messages itself, the second naming the key
// that sets the limit:
//
//	%VAX-E-RAMLIMIT, RAM disks would total 540000 blocks, over the limit of 512000
//	-VAX-I-RAMKEY, the limit is configuration key vax.ramdisk.limit
func (c *Context) ramDiskRefusal(err error, unit string) error {
	var over *rms.MemoryLimitError

	switch {
	case errors.As(err, &over):
		first := vmserrors.New(vmserrors.VAX_RAMLIMIT, over.Total, over.Limit)
		c.Printf("%%%s\n", first)
		c.Printf("-%s\n", vmserrors.New(vmserrors.VAX_RAMKEY, RAMDiskLimitSetting))

		return vmserrors.InhibitMessage(first)
	case errors.Is(err, rms.ErrMounted):
		return vmserrors.New(vmserrors.SS_DEVMOUNT, unit)
	}

	return vmserrors.Wrap(vmserrors.SS_NOMOUNT, err, unit)
}
