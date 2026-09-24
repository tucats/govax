package io

import "strings"

// DeviceClass identifies a device's VMS device class (DVI$_DEVCLASS),
// matching devices.c's dev_class_map and testdata/dcl/evax.dcl's dev_class
// keyword type (whose keyword IDs are exactly these class numbers).
type DeviceClass int32

// Device classes dev_class_map actually names; every other class number
// (testdata/dcl/evax.dcl's dev_class type defines many more, e.g. tape=2,
// workstation=70) is legal to set but has no display name of its own.
const (
	DeviceClassNone DeviceClass = 0
	DeviceClassDisk DeviceClass = 1
	DeviceClassTT   DeviceClass = 66
)

var deviceClassNames = map[DeviceClass]string{
	DeviceClassTT:   "terminal",
	DeviceClassDisk: "disk",
	DeviceClassNone: "none",
}

// DeviceClassName returns class's display name, or "<unknown>" for any
// class not in the small table above — matching get_dev_class_name's own
// fallback, minus its bug: the C function loops "for (n = 0; n < 100; n++)"
// over dev_class_map's 3 actual entries, reading 97 elements past the end
// of the array. That's a plain out-of-bounds loop bound, not an ISA
// judgment call (this file is RTL tooling, not emulated VAX behavior — see
// doc.go), so it's fixed here rather than replicated.
func DeviceClassName(class DeviceClass) string {
	if name, ok := deviceClassNames[class]; ok {
		return name
	}

	return "<unknown>"
}

// deviceTypeNames mirrors testdata/dcl/evax.dcl's dev_type keyword
// table id-for-id (rk06=1, rk07=2, ... vt100=96) -- the same numbers the
// DCL grammar already assigns to DEFINE/DEVICE/DEVTYPE=, restated here so
// Go code can turn a stored DevType back into its display name (e.g.
// SHOW DEVICE/FULL's "device type RA81" clause) without depending on the
// grammar package.
var deviceTypeNames = map[uint32]string{
	1:  "RK06",
	2:  "RK07",
	3:  "RP04",
	4:  "RP05",
	5:  "RP06",
	6:  "RM03",
	7:  "RP07",
	8:  "RP07HT",
	9:  "RL01",
	10: "RL02",
	11: "RX02",
	12: "RX04",
	13: "RM80",
	14: "TU58",
	15: "RM05",
	16: "RX01",
	17: "ML11",
	18: "RB02",
	19: "RB80",
	20: "RA80",
	21: "RA81",
	22: "RA60",
	23: "RZ01",
	25: "RD51",
	26: "RX50",
	27: "RX33",
	28: "RD31",
	29: "RD52",
	30: "RD32",
	31: "RD33",
	32: "RD53",
	33: "RD54",
	34: "RA70",
	35: "RA82",
	36: "RA71",
	37: "RA72",
	38: "RA90",
	39: "RA92",
	40: "RA73",
	96: "VT100",
}

// DeviceTypeName returns t's display name and true, or "", false if t
// isn't one of the known dev_type IDs above. Unlike DeviceClassName,
// there's no "<unknown>" fallback string: a caller like SHOW DEVICE/FULL
// wants to omit an unknown device type's clause entirely rather than
// print a placeholder, since — unlike device class, which every device
// genuinely has — most devices this emulator creates (in particular,
// MOUNT's own auto-created disk devices) never have a DevType set at
// all.
func DeviceTypeName(t uint32) (string, bool) {
	name, ok := deviceTypeNames[t]

	return name, ok
}

// Device is one DEFINE/DEVICE-created device record, the Go equivalent of
// devices.c's struct DEVICE. Field names follow the qualifier names in
// testdata/dcl/evax.dcl's define_device syntax rather than the C struct's
// exact spelling (e.g. DevBufSize not devbufsiz) — this is the emulator's
// own RTL bookkeeping data, not emulated VAX ISA state, so there's no
// fidelity reason to keep the C spelling. PID/OwnUIC are left for a caller
// to fill in (define_device.c reads OwnUIC from a qualifier and otherwise
// leaves PID at 0 until SYS$ASSIGN sets it from the calling process — no
// process/PID concept exists yet in this port; see doc.go's Phase 10 note).
type Device struct {
	Name string

	ACPPID     uint32
	Cluster    uint32
	Cylinders  uint32
	DevBufSize uint32
	DevChar    uint32
	DevChar2   uint32
	DevClass   DeviceClass
	DevDepend  uint32
	DevDepend2 uint32
	DevSts     uint32
	DevType    uint32
	ErrCnt     uint32
	FreeBlocks uint32
	LockID     uint32
	MaxBlock   uint32
	MaxFiles   uint32
	MountCount uint32
	OpCnt      uint32
	OwnUIC     uint32
	PID        uint32
	RecSize    uint32
	RefCnt     uint32
	Sectors    uint32
	Serial     uint32
	STS        uint32

	VolName     string
	MediaName   string
	MediaType   string
	RootDevName string
}

// DeviceOptions carries define_device's own settable fields — everything
// define_device.c's GET_INT_FIELD/GET_STR_FIELD macros read from a DCL
// qualifier, each defaulting to zero/empty when the qualifier is absent
// (matching those macros' own else-branch). A plain data struct so this
// package doesn't need to know anything about internal/console/dcl —
// internal/console/device.go (this phase's console wiring) builds one from
// a dcl.Result.
type DeviceOptions struct {
	Cluster, Cylinders, DevBufSize, DevChar, DevChar2 uint32
	DevClass                                          DeviceClass
	DevDepend, DevDepend2                             uint32
	DevType                                           uint32
	FreeBlocks, LockID, MaxBlock, MaxFiles, OwnUIC    uint32
	RecSize, Sectors, Serial                          uint32
	VolName, MediaName, MediaType, RootDevName        string
}

// DeviceTable is the Go equivalent of devices.c's devices linked list.
type DeviceTable struct {
	// devices holds Defined devices most-recent-first, matching
	// find_device's own traversal order (new devices are prepended to the
	// C source's linked list, so a name defined twice resolves to its
	// most recent definition).
	devices []*Device
}

// KnownDeviceOptions is the default-geometry dictionary Define falls back to for
// well-known disk device types when a DEFINE/DEVICE qualifier didn't
// specify Cylinders/Sectors/MaxBlock explicitly. Values come from
// reference/vms/disk-devices.md (sec/cyl/LBNs columns -> Sectors/Cylinders/
// MaxBlock); that chart's "surf" column has no equivalent field here.
var KnownDeviceOptions = map[string]DeviceOptions{
	"RX50": {
		Cylinders: 80,
		Sectors:   10,
		MaxBlock:  800,
		DevClass:  DeviceClassDisk,
	},
	"RX33": {
		Sectors:   15,
		Cylinders: 80,
		MaxBlock:  2400,
		DevClass:  DeviceClassDisk,
	},
	"RD51": {
		Sectors:   18,
		Cylinders: 306,
		MaxBlock:  21600,
		DevClass:  DeviceClassDisk,
	},
	"RD31": {
		Sectors:   17,
		Cylinders: 615,
		MaxBlock:  41560,
		DevClass:  DeviceClassDisk,
	},
	"RD52": {
		Sectors:   17,
		Cylinders: 512,
		MaxBlock:  60480,
		DevClass:  DeviceClassDisk,
	},
	"RD32": {
		Sectors:   17,
		Cylinders: 820,
		MaxBlock:  83204,
		DevClass:  DeviceClassDisk,
	},
	"RD33": {
		Sectors:   17,
		Cylinders: 1170,
		MaxBlock:  138565,
		DevClass:  DeviceClassDisk,
	},
	"RD53": {
		Sectors:   17,
		Cylinders: 1024,
		MaxBlock:  138672,
		DevClass:  DeviceClassDisk,
	},
	"RD54": {
		Sectors:   17,
		Cylinders: 1225,
		MaxBlock:  311200,
		DevClass:  DeviceClassDisk,
	},
	"RA60": {
		Sectors:   42,
		Cylinders: 1600,
		MaxBlock:  400176,
		DevClass:  DeviceClassDisk,
	},
	"RA70": {
		Sectors:   33,
		Cylinders: 1507,
		MaxBlock:  547041,
		DevClass:  DeviceClassDisk,
	},
	"RA80": {
		Sectors:   31,
		Cylinders: 546,
		MaxBlock:  237212,
		DevClass:  DeviceClassDisk,
	},
	"RA81": {
		Sectors:   51,
		Cylinders: 1258,
		MaxBlock:  891072,
		DevClass:  DeviceClassDisk,
	},
	"RA82": {
		Sectors:   57,
		Cylinders: 1435,
		MaxBlock:  1216665,
		DevClass:  DeviceClassDisk,
	},
	"RA71": {
		Sectors:   51,
		Cylinders: 1921,
		MaxBlock:  1367310,
		DevClass:  DeviceClassDisk,
	},
	"RA72": {
		Sectors:   51,
		Cylinders: 1921,
		MaxBlock:  1953300,
		DevClass:  DeviceClassDisk,
	},
	"RA90": {
		Sectors:   69,
		Cylinders: 2656,
		MaxBlock:  2376153,
		DevClass:  DeviceClassDisk,
	},
	"RA92": {
		Sectors:   73,
		Cylinders: 3101,
		MaxBlock:  2940951,
		DevClass:  DeviceClassDisk,
	},
	"RA73": {
		Sectors:   70,
		Cylinders: 2667,
		MaxBlock:  3920490,
		DevClass:  DeviceClassDisk,
	},
}

// NewDeviceTable returns an empty device table (matching devices == 0L).
func NewDeviceTable() *DeviceTable { return &DeviceTable{} }

// normalizeDeviceName upcases name and strips one trailing ':', matching
// find_device's own dname preparation.
func normalizeDeviceName(name string) string {
	return strings.TrimSuffix(strings.ToUpper(name), ":")
}

// Define creates and registers a new device named name, matching
// define_device — which always succeeds once its (C) allocation succeeds;
// a Go allocation always does, so this never fails. define_device.c's
// "fill in stuff we know without being told" fields (pid/devsts/errcnt/
// acppid/refcnt/mountcount/opcnt/sts) all start at 0, which is already
// this struct's zero value, so there's nothing to set explicitly for them.
func (t *DeviceTable) Define(name string, opts DeviceOptions) *Device {
	d := &Device{
		Name:        strings.ToUpper(name),
		Cluster:     opts.Cluster,
		Cylinders:   opts.Cylinders,
		DevBufSize:  opts.DevBufSize,
		DevChar:     opts.DevChar,
		DevChar2:    opts.DevChar2,
		DevClass:    opts.DevClass,
		DevDepend:   opts.DevDepend,
		DevDepend2:  opts.DevDepend2,
		DevType:     opts.DevType,
		FreeBlocks:  opts.FreeBlocks,
		LockID:      opts.LockID,
		MaxBlock:    opts.MaxBlock,
		MaxFiles:    opts.MaxFiles,
		OwnUIC:      opts.OwnUIC,
		RecSize:     opts.RecSize,
		Sectors:     opts.Sectors,
		Serial:      opts.Serial,
		VolName:     opts.VolName,
		MediaName:   opts.MediaName,
		MediaType:   opts.MediaType,
		RootDevName: opts.RootDevName,
	}

	// Before we quit, if the user didn't specify something explicit
	// and it's a well-known device, set it's physical attributes from
	// default device dictionary.
	if defs, ok := KnownDeviceOptions[strings.ToUpper(name)]; ok {
		if d.DevClass == 0 {
			d.DevClass = defs.DevClass
		}

		if d.Cylinders == 0 {
			d.Cylinders = defs.Cylinders
		}

		if d.MaxBlock == 0 {
			d.MaxBlock = defs.MaxBlock
		}

		if d.Sectors == 0 {
			d.Sectors = defs.Sectors
		}
	}

	t.devices = append([]*Device{d}, t.devices...)

	return d
}

// Find looks up a device by name, matching find_device: case-insensitive,
// with one trailing ':' stripped if present (so "TTA0" and "TTA0:" both
// resolve the same device).
func (t *DeviceTable) Find(name string) (*Device, bool) {
	name = normalizeDeviceName(name)
	for _, d := range t.devices {
		if d.Name == name {
			return d, true
		}
	}

	return nil, false
}

// All returns every defined device, most-recently-Defined first (matching
// Find's own traversal order) — the enumeration show_device (SHOW DEVICE)
// walks.
func (t *DeviceTable) All() []*Device {
	out := make([]*Device, len(t.devices))
	copy(out, t.devices)

	return out
}
