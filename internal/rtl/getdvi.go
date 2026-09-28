package rtl

import (
	"fmt"
	"strconv"
	"strings"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// $GETDVI and $GETDVIW (docs/PHASE-26.md subtask 28): information about a
// device.
//
// It is the $GETJPI/$GETSYI pattern (getjpi.go, getsyi.go) for a device:
// the program names the device by a channel or by name, and an item list
// says what to return — its name, class, characteristics, owner, and so
// on. The request completes during the call (event flag, IOSB, AST), so
// $GETDVI and $GETDVIW are the same service.
//
// Everything comes from the device's record (iodev.Device, the stand-in
// for VMS's unit control block, UCB) plus, for a disk, the volume mounted
// on it: its label, size, and free space come from the mounted ODS-2
// volume when there is one, as SHOW DEVICE/FULL reports them.
//
// This replaces eVAX's $GETDVIW, which knew three items (DEVCLASS,
// DEVTYPE, DEVBUFSIZ) and wrote them with the wrong sizes.

// dviItemFunc returns one item's value for device d.
type dviItemFunc func(env *Environment, d *iodev.Device) itemValue

// dviLong and dviString make the two common kinds of item: a longword
// field and a string.
func dviLong(field func(d *iodev.Device) uint32) dviItemFunc {
	return func(_ *Environment, d *iodev.Device) itemValue { return itemLong(field(d)) }
}

func dviString(field func(env *Environment, d *iodev.Device) string) dviItemFunc {
	return func(env *Environment, d *iodev.Device) itemValue { return itemString(field(env, d)) }
}

// dviItemsByName is the item-code registry, keyed by $DVIDEF name. init
// adds the Boolean items for each DEVCHAR bit and each terminal
// characteristic. Items not here are SS$_BADPARAM.
var dviItemsByName = map[string]dviItemFunc{
	// Device class, type, and characteristics.
	"DVI$_DEVCLASS":   dviLong(func(d *iodev.Device) uint32 { return uint32(d.DevClass) }),
	"DVI$_DEVTYPE":    dviLong(func(d *iodev.Device) uint32 { return d.DevType }),
	"DVI$_DEVBUFSIZ":  dviLong(func(d *iodev.Device) uint32 { return d.DevBufSize }),
	"DVI$_DEVCHAR":    func(env *Environment, d *iodev.Device) itemValue { return itemLong(env.devChar(d)) },
	"DVI$_DEVCHAR2":   dviLong(func(d *iodev.Device) uint32 { return d.DevChar2 }),
	"DVI$_DEVDEPEND":  dviLong(func(d *iodev.Device) uint32 { return d.DevDepend }),
	"DVI$_DEVDEPEND2": dviLong(func(d *iodev.Device) uint32 { return d.DevDepend2 }),
	"DVI$_DEVSTS":     dviLong(func(d *iodev.Device) uint32 { return d.DevSts }),
	"DVI$_STS":        dviLong(func(d *iodev.Device) uint32 { return d.STS }),
	"DVI$_UNIT":       dviLong(unitNumber),

	// Ownership and use.
	"DVI$_PID":       dviLong(func(d *iodev.Device) uint32 { return d.PID }),
	"DVI$_OWNUIC":    dviLong(func(d *iodev.Device) uint32 { return d.OwnUIC }),
	"DVI$_REFCNT":    dviLong(func(d *iodev.Device) uint32 { return d.RefCnt }),
	"DVI$_ERRCNT":    dviLong(func(d *iodev.Device) uint32 { return d.ErrCnt }),
	"DVI$_OPCNT":     dviLong(func(d *iodev.Device) uint32 { return d.OpCnt }),
	"DVI$_ACPPID":    dviLong(func(d *iodev.Device) uint32 { return d.ACPPID }),
	"DVI$_LOCKID":    dviLong(func(d *iodev.Device) uint32 { return d.LockID }),
	"DVI$_RECSIZ":    dviLong(func(d *iodev.Device) uint32 { return d.RecSize }),
	"DVI$_SERIALNUM": dviLong(func(d *iodev.Device) uint32 { return d.Serial }),

	// Names. A device name is written as VMS writes a physical one: a
	// leading "_" (so it isn't translated as a logical name) and a
	// trailing ":". The full and allocation-class names add the node.
	"DVI$_DEVNAM":     dviString(func(_ *Environment, d *iodev.Device) string { return physicalName(d) }),
	"DVI$_FULLDEVNAM": dviString(nodeDeviceName),
	"DVI$_ALLDEVNAM":  dviString(nodeDeviceName),
	"DVI$_ROOTDEVNAM": dviString(func(_ *Environment, d *iodev.Device) string { return d.RootDevName }),
	"DVI$_MEDIA_NAME": dviString(func(_ *Environment, d *iodev.Device) string { return d.MediaName }),
	"DVI$_MEDIA_TYPE": dviString(func(_ *Environment, d *iodev.Device) string { return d.MediaType }),
	"DVI$_TT_PHYDEVNAM": dviString(func(_ *Environment, d *iodev.Device) string {
		if d.DevClass == iodev.DeviceClassTT {
			return physicalName(d)
		}

		return ""
	}),

	// Disk geometry and the mounted volume (volumeInfo).
	"DVI$_CYLINDERS": dviLong(func(d *iodev.Device) uint32 { return d.Cylinders }),
	"DVI$_SECTORS":   dviLong(func(d *iodev.Device) uint32 { return d.Sectors }),
	"DVI$_MOUNTCNT":  dviLong(func(d *iodev.Device) uint32 { return d.MountCount }),
	"DVI$_VOLNAM":    func(env *Environment, d *iodev.Device) itemValue { return itemString(env.volumeInfo(d).label) },
	"DVI$_MAXBLOCK":  func(env *Environment, d *iodev.Device) itemValue { return itemLong(env.volumeInfo(d).maxBlock) },
	"DVI$_FREEBLOCKS": func(env *Environment, d *iodev.Device) itemValue {
		return itemLong(env.volumeInfo(d).freeBlocks)
	},
	"DVI$_CLUSTER":  func(env *Environment, d *iodev.Device) itemValue { return itemLong(env.volumeInfo(d).cluster) },
	"DVI$_MAXFILES": func(env *Environment, d *iodev.Device) itemValue { return itemLong(env.volumeInfo(d).maxFiles) },

	// Clusters: a single node, serving its own devices.
	"DVI$_ALLOCLASS":     func(*Environment, *iodev.Device) itemValue { return itemLong(0) },
	"DVI$_REMOTE_DEVICE": func(*Environment, *iodev.Device) itemValue { return itemLong(0) },
	"DVI$_SERVED_DEVICE": func(*Environment, *iodev.Device) itemValue { return itemLong(0) },
	"DVI$_VOLSETMEM":     func(*Environment, *iodev.Device) itemValue { return itemLong(0) },

	// The terminal's page length: the high byte of DEVDEPEND.
	"DVI$_TT_PAGE": dviLong(func(d *iodev.Device) uint32 { return d.DevDepend >> 24 }),
}

// dviDevCharItems are the Boolean items for DEVCHAR bits: DVI$_x is 1
// when the device has DEV$M_x.
var dviDevCharItems = []string{
	"REC", "CCL", "TRM", "DIR", "SDI", "SQD", "SPL", "OPR", "RCT", "NET",
	"FOD", "DUA", "SHR", "GEN", "AVL", "MNT", "MBX", "DMT", "ELG", "ALL",
	"FOR", "SWL", "IDV", "ODV", "RND", "RTM", "RCK", "WCK",
}

// init adds the generated Boolean items, then builds dviItems:
//
//   - each DEVCHAR bit in dviDevCharItems (DVI$_MNT: is it mounted?);
//   - each terminal characteristic, DVI$_TT_x, which is TT$M_x in the
//     terminal's DEVDEPEND or, if $TTDEF has no TT$M_x, TT2$M_x in its
//     DEVDEPEND2. (DVI$_TT_PAGE and DVI$_TT_PHYDEVNAM aren't bits and are
//     in the registry already.)
func init() {
	for _, name := range dviDevCharItems {
		mask, ok := vmsdef.DEVConstants["DEV$M_"+name]
		if !ok {
			panic("rtl: no $DEVDEF bit DEV$M_" + name)
		}

		dviItemsByName["DVI$_"+name] = func(env *Environment, d *iodev.Device) itemValue {
			return itemLong(b01(env.devChar(d)&mask != 0))
		}
	}

	for name := range vmsdef.DVIConstants {
		bit, ok := strings.CutPrefix(name, "DVI$_TT_")
		if !ok || dviItemsByName[name] != nil {
			continue
		}

		if mask, ok := vmsdef.TTConstants["TT$M_"+bit]; ok {
			dviItemsByName[name] = dviLong(func(d *iodev.Device) uint32 { return b01(d.DevDepend&mask != 0) })
		} else if mask, ok := vmsdef.TTConstants["TT2$M_"+bit]; ok {
			dviItemsByName[name] = dviLong(func(d *iodev.Device) uint32 { return b01(d.DevDepend2&mask != 0) })
		}
	}

	for name, fn := range dviItemsByName {
		code, ok := vmsdef.DVIConstants[name]
		if !ok {
			panic("rtl: no $DVIDEF item code " + name)
		}

		dviItems[uint16(code)] = fn
	}
}

// b01 is 1 for true, 0 for false.
func b01(b bool) uint32 {
	if b {
		return 1
	}

	return 0
}

// dviItems is dviItemsByName keyed by item code (built by init).
var dviItems = map[uint16]dviItemFunc{}

// dviItemFlags are the item-code bits that aren't part of the code:
// DVI$M_SECONDARY (bit 0, "the secondary device") and DVI$M_NOREDIRECT
// (bit 15).
var dviItemFlags = uint16(vmsdef.DVIConstants["DVI$M_SECONDARY"] | vmsdef.DVIConstants["DVI$M_NOREDIRECT"])

// dviItem returns the item for code, ignoring dviItemFlags: a govax
// device has no separate secondary device (VMS's is for spooled
// devices), and no redirection.
func dviItem(code uint16) (dviItemFunc, bool) {
	fn, ok := dviItems[code&^dviItemFlags]

	return fn, ok
}

// physicalName is d's name as VMS writes a physical device name:
// "_TTA0:".
func physicalName(d *iodev.Device) string { return "_" + d.Name + ":" }

// nodeDeviceName is d's name with the node's: "_GOVAX$TTA0:".
func nodeDeviceName(env *Environment, d *iodev.Device) string {
	return "_" + env.NodeName + "$" + d.Name + ":"
}

// unitNumber is the unit number in d's name: the digits it ends with
// (TTA0 is unit 0, DUA12 unit 12).
func unitNumber(d *iodev.Device) uint32 {
	i := len(d.Name)
	for i > 0 && d.Name[i-1] >= '0' && d.Name[i-1] <= '9' {
		i--
	}

	n, _ := strconv.ParseUint(d.Name[i:], 10, 32)

	return uint32(n)
}

// devChar is d's DEVCHAR, with DEV$M_MNT set while a volume is mounted
// on it and DEV$M_SWL (software write-locked) when that mount is read
// only: MOUNT (internal/rms's MountTable) doesn't set them on the record.
func (env *Environment) devChar(d *iodev.Device) uint32 {
	c := d.DevChar

	if env.Mounts != nil {
		if _, mounted := env.Mounts.Lookup(d.Name); mounted {
			c |= devMounted

			if !env.Mounts.Writable(d.Name) {
				c |= vmsdef.DEVConstants["DEV$M_SWL"]
			}
		}
	}

	return c
}

// volumeFacts are a disk's volume items.
type volumeFacts struct {
	label                                   string
	maxBlock, freeBlocks, cluster, maxFiles uint32
}

// volumeInfo returns d's volume items: from the volume mounted on it, if
// there is one and it can be read, as SHOW DEVICE/FULL does, and
// otherwise from the device record's fields.
func (env *Environment) volumeInfo(d *iodev.Device) volumeFacts {
	v := volumeFacts{label: d.VolName, maxBlock: d.MaxBlock, freeBlocks: d.FreeBlocks, cluster: d.Cluster, maxFiles: d.MaxFiles}

	if env.Mounts == nil {
		return v
	}

	stats, mounted, err := env.Mounts.VolumeStats(d.Name)
	if !mounted || err != nil {
		return v
	}

	v.maxBlock, v.freeBlocks = stats.TotalBlocks, stats.FreeBlocks
	v.cluster, v.maxFiles = uint32(stats.ClusterSize), stats.MaxFiles

	if label, ok := env.Mounts.VolumeLabel(d.Name); ok {
		v.label = label
	}

	return v
}

// serviceSysGetdvi is SYS$GETDVI and SYS$GETDVIW:
//
//	SYS$GETDVI[W] [efn] ,[chan] ,[devnam] ,itmlst [,iosb] [,astadr] [,astprm] [,nullarg]
//
// The device is the one channel chan (its low word) is assigned to, or,
// with chan 0, the one devnam names (see dviTarget). As for $GETJPI, the
// request completes at once: event flag efn (default 0) is cleared and
// then set, the IOSB gets the final status, and if astadr isn't 0 an AST
// is queued with astprm in the caller's mode. A request rejected before
// it starts (too few arguments, a bad event flag, an unwritable IOSB, no
// such device) completes nothing. An item code not in the registry ends
// the item list with SS$_BADPARAM, and the request still completes with
// that status.
//
// Not implemented: the ASTLM quota (SS$_EXASTLM); other nodes' devices
// (SS$_NONLOCAL); secondary devices.
func serviceSysGetdvi(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 4 {
		return ssInsfArg, nil
	}

	efn, chanNum, devnam, itmlst := argv[0], argv[1]&0xFFFF, argv[2], argv[3]
	iosb, astadr, astprm := optArg(argv, 4), optArg(argv, 5), optArg(argv, 6)

	flags, bit, st := env.eventFlagWord(efn)
	if st != 0 {
		return st, nil
	}

	*flags &^= 1 << bit

	if iosb != 0 && !env.storeQuad(iosb, 0) {
		return ssAccVio, nil
	}

	d, st := env.dviTarget(chanNum, devnam)
	if st != 0 {
		return st, nil
	}

	if env.cpu.DebugEnabled(vax.DebugDevices) {
		fmt.Fprintf(env.cpu.DebugWriter(), "DEBUG: SYS$GETDVI looks up device %s\n", d.Name)
	}

	status := env.walkItemList(itmlst, func(e itemListEntry) uint32 {
		item, ok := dviItem(e.ItemCode)
		if !ok {
			return ssBadParam
		}

		return env.storeItem(e, item(env, d))
	})
	if status == 0 {
		status = ssNormal
	}

	if iosb != 0 {
		if err := env.mem.StoreLongword(env.cpu, iosb, status); err != nil {
			return ssAccVio, nil
		}
	}

	*flags |= 1 << bit

	if astadr != 0 {
		env.queueAST(astadr, astprm, uint32(env.cpu.PSL().CurMod()))
	}

	return status, nil
}

// dviTarget finds the device a $GETDVI names, or the status saying why it
// can't (0 if it can):
//
//   - chan nonzero: the device the channel is assigned to; SS$_NOPRIV if
//     the channel isn't assigned, or was assigned from a more privileged
//     mode than the caller's.
//   - otherwise devnam, a device name or a logical name for one (a
//     leading "_" suppresses translation, and a colon and anything after
//     it is ignored): SS$_IVLOGNAM if it is empty or longer than 63
//     characters, SS$_NOSUCHDEV if there's no such device.
//   - neither: SS$_IVDEVNAM.
func (env *Environment) dviTarget(chanNum, devnam uint32) (*iodev.Device, uint32) {
	if chanNum != 0 {
		c, found := env.findChannel(chanNum)
		if !found || c.Mode < uint32(env.cpu.PSL().CurMod()) {
			return nil, ssNoPriv
		}

		return c.Device, 0
	}

	if devnam == 0 {
		return nil, ssIvDevNam
	}

	name, ok, err := strGet(env, devnam, maxDeviceNameLength)
	if err != nil {
		return nil, ssAccVio
	}

	if !ok || name == "" {
		return nil, ssIvLogNam
	}

	device, st := env.deviceName(name)
	if st != 0 {
		return nil, st
	}

	d, found := env.Devices.Find(device)
	if !found {
		return nil, ssNoSuchDev
	}

	return d, 0
}

func registerDVIServices(t *ServiceTable) {
	t.Register("SYS$GETDVI", serviceSysGetdvi)
	t.Register("SYS$GETDVIW", serviceSysGetdvi)
}
