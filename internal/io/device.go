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
		Name:        name,
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
