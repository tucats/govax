package rms

import (
	"bytes"
	"testing"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/ods2/ondisk"
)

// createFixture bundles everything TestSysCreate_* needs: a Context wired
// to a small VAX memory/CPU pair (matching internal/rtl's own test
// fixture convention — see internal/rtl/rtl_test.go's fixture) plus a
// MountTable with one device, "DUA0", already mounted (writable per the
// writable argument) on a freshly initialized, empty ODS-2 volume.
type createFixture struct {
	ctx     *Context
	console *bytes.Buffer
}

func newCreateFixture(t *testing.T, writable bool) *createFixture {
	t.Helper()

	cpu := vax.New()
	mem := vm.NewMemory(1 << 20)

	logicals := iodev.NewLogicalNameTable()
	logicals.InitLogicals()

	mounts := NewMountTable()
	if err := mounts.Mount("DUA0", newTestVolumeFile(t, "TESTVOL"), writable); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	console := &bytes.Buffer{}

	return &createFixture{
		ctx: &Context{
			Mem:      mem,
			CPU:      cpu,
			Mounts:   mounts,
			Files:    NewFileTable(nil),
			Logicals: logicals,
			Console:  console,
		},
		console: console,
	}
}

// Addresses used to lay out a test FAB and its associated file-spec
// string in the fixture's otherwise-empty 1MB memory. Chosen far apart so
// a wrong offset constant reading past one region would land in
// obviously-unrelated (zeroed) memory rather than silently overlapping
// the other.
const (
	testFabAddr = 0x1000
	testFnaAddr = 0x2000
)

func putByte(t *testing.T, ctx *Context, addr uint32, v byte) {
	t.Helper()

	if err := ctx.storeByte(addr, v); err != nil {
		t.Fatalf("storeByte(%#x): %v", addr, err)
	}
}

func putWord(t *testing.T, ctx *Context, addr uint32, v uint16) {
	t.Helper()

	if err := ctx.storeWord(addr, v); err != nil {
		t.Fatalf("storeWord(%#x): %v", addr, err)
	}
}

func putLongwordAt(t *testing.T, ctx *Context, addr uint32, v uint32) {
	t.Helper()

	if err := ctx.storeLongword(addr, v); err != nil {
		t.Fatalf("storeLongword(%#x): %v", addr, err)
	}
}

// putFileSpec writes s verbatim (no NUL terminator — see fabFNS's own doc
// comment on why RMS file-spec strings aren't C strings) at testFnaAddr,
// and points a fresh FAB at testFabAddr's FAB$L_FNA/FAB$B_FNS fields at
// it.
func putFileSpec(t *testing.T, ctx *Context, s string) {
	t.Helper()

	for i := 0; i < len(s); i++ {
		putByte(t, ctx, testFnaAddr+uint32(i), s[i])
	}

	putLongwordAt(t, ctx, testFabAddr+fabFNA, testFnaAddr)
	putByte(t, ctx, testFabAddr+fabFNS, byte(len(s)))
}

// newFAB writes a complete, otherwise-valid FAB at testFabAddr for
// creating a fixed-format, 80-byte-max-record sequential file named by
// spec, ready for a test to override individual fields before calling
// SysCreate.
func newFAB(t *testing.T, ctx *Context, spec string) {
	t.Helper()

	putFileSpec(t, ctx, spec)
	putByte(t, ctx, testFabAddr+fabFAC, facPut)
	putByte(t, ctx, testFabAddr+fabORG, orgSeq)
	putByte(t, ctx, testFabAddr+fabRFM, 1) // ondisk.RecordFormatFixed
	putByte(t, ctx, testFabAddr+fabRAT, 0)
	putWord(t, ctx, testFabAddr+fabMRS, 80)
}

func readWord(t *testing.T, ctx *Context, addr uint32) uint16 {
	t.Helper()

	v, err := ctx.loadWord(addr)
	if err != nil {
		t.Fatalf("loadWord(%#x): %v", addr, err)
	}

	return v
}

func readLongword(t *testing.T, ctx *Context, addr uint32) uint32 {
	t.Helper()

	v, err := ctx.loadLongword(addr)
	if err != nil {
		t.Fatalf("loadLongword(%#x): %v", addr, err)
	}

	return v
}

// TestSysCreate_console confirms SYS$CREATE against "TTA0:" binds a fresh
// IFI to the console writer rather than touching the mounted volume at
// all — real VMS RMS's own terminal-device support (docs/PHASE-22.md's
// "Why this phase looks different"), carried forward from the deleted
// Phase 10 stopgap.
func TestSysCreate_console(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "TTA0:")

	r0, err := SysCreate(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	if sts := readLongword(t, f.ctx, testFabAddr+fabSTS); sts != rmsNormal {
		t.Errorf("FAB$L_STS = %d, want rmsNormal (%d)", sts, rmsNormal)
	}

	ifi := readWord(t, f.ctx, testFabAddr+fabIFI)

	h, ok := f.ctx.Files.Lookup(ifi)
	if !ok {
		t.Fatalf("Files.Lookup(%d) = not found, want the console handle just allocated", ifi)
	}

	if !h.IsConsole() {
		t.Fatal("allocated handle is not the console case")
	}

	if _, err := h.Console.Write([]byte("hi")); err != nil {
		t.Fatalf("writing through the allocated console handle: %v", err)
	}

	if got := f.console.String(); got != "hi" {
		t.Errorf("console output = %q, want %q", got, "hi")
	}
}

// TestSysCreate_diskFile confirms SYS$CREATE against a real mounted
// device actually creates a file on the ODS-2 volume — the acceptance
// criterion this whole phase exists for (docs/PHASE-22.md's goal) — and
// returns RMS$_CREATED (not plain RMS$_NORMAL — see status.go's own doc
// comment on the two being distinguishable) with an IFI whose FileHandle
// wraps the new *volume.File.
func TestSysCreate_diskFile(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:TEST.DAT")

	r0, err := SysCreate(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	if r0 != rmsCreated {
		t.Errorf("r0 = %d, want rmsCreated (%d)", r0, rmsCreated)
	}

	if sts := readLongword(t, f.ctx, testFabAddr+fabSTS); sts != rmsCreated {
		t.Errorf("FAB$L_STS = %d, want rmsCreated (%d)", sts, rmsCreated)
	}

	ifi := readWord(t, f.ctx, testFabAddr+fabIFI)
	if ifi < firstUserIFI {
		t.Fatalf("IFI = %d, want >= %d", ifi, firstUserIFI)
	}

	h, ok := f.ctx.Files.Lookup(ifi)
	if !ok {
		t.Fatalf("Files.Lookup(%d) = not found, want the handle just allocated", ifi)
	}

	if h.IsConsole() || h.File == nil {
		t.Fatal("allocated handle is not a real ODS-2 file")
	}

	// Confirm the file is genuinely visible on the volume itself, not
	// just recorded in this package's own IFI table — looking it up
	// independently through the same mounted *volume.Volume SysCreate
	// itself used.
	vol, _ := f.ctx.Mounts.Lookup("DUA0")

	dir, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		t.Fatalf("OpenDirectory(MFD): %v", err)
	}

	entry, err := dir.Lookup("TEST.DAT", 1)
	if err != nil {
		t.Fatalf("looking up TEST.DAT;1 in the MFD: %v", err)
	}

	if entry.Fid != h.File.Header.Fid {
		t.Errorf("directory entry's Fid = %v, want the created file's own Fid %v", entry.Fid, h.File.Header.Fid)
	}
}

// TestSysCreate_readOnlyMount confirms SYS$CREATE against a device
// mounted read-only fails with RMS$_PRV rather than attempting (and
// somehow succeeding or panicking) a write ods2 itself would refuse, and
// that no half-finished IFI is left behind.
func TestSysCreate_readOnlyMount(t *testing.T) {
	f := newCreateFixture(t, false)
	newFAB(t, f.ctx, "DUA0:TEST.DAT")
	putWord(t, f.ctx, testFabAddr+fabIFI, 0xFFFF) // sentinel: must stay untouched

	r0, err := SysCreate(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	if r0 != rmsPrivilegeViolation {
		t.Errorf("r0 = %d, want rmsPrivilegeViolation (%d)", r0, rmsPrivilegeViolation)
	}

	if ifi := readWord(t, f.ctx, testFabAddr+fabIFI); ifi != 0xFFFF {
		t.Errorf("FAB$W_IFI = %#x, want the untouched sentinel 0xFFFF", ifi)
	}
}

// TestSysCreate_deviceNotMounted confirms a file spec naming a device
// with nothing mounted on it at all fails with RMS$_DNR, not a Go-level
// panic or a misleading "file not found".
func TestSysCreate_deviceNotMounted(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUB0:TEST.DAT")

	r0, err := SysCreate(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	if r0 != rmsDeviceNotReady {
		t.Errorf("r0 = %d, want rmsDeviceNotReady (%d)", r0, rmsDeviceNotReady)
	}
}

// TestSysCreate_facWithoutPut confirms a FAB that never asked for PUT
// access (FAB$B_FAC missing facPut) is rejected before this package ever
// consults the mount table or ods2 at all — matching real RMS's own
// access-mode check.
func TestSysCreate_facWithoutPut(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:TEST.DAT")
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)

	r0, err := SysCreate(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	if r0 != rmsPrivilegeViolation {
		t.Errorf("r0 = %d, want rmsPrivilegeViolation (%d)", r0, rmsPrivilegeViolation)
	}
}

// TestSysCreate_unsupportedOrg confirms a FAB naming an organization
// other than sequential (fab.go's orgSeq) fails with RMS$_ORG, matching
// docs/PHASE-22.md's sequential-only scope.
func TestSysCreate_unsupportedOrg(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:TEST.DAT")
	putByte(t, f.ctx, testFabAddr+fabORG, orgRel)

	r0, err := SysCreate(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	if r0 != rmsInvalidOrg {
		t.Errorf("r0 = %d, want rmsInvalidOrg (%d)", r0, rmsInvalidOrg)
	}
}

// TestSysCreate_unsupportedRFM confirms a FAB naming a record format
// value outside the seven real FAB$C_* values fails with RMS$_RFM rather
// than being passed straight through to ods2 as a bogus
// ondisk.RecordFormat.
func TestSysCreate_unsupportedRFM(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:TEST.DAT")
	putByte(t, f.ctx, testFabAddr+fabRFM, 99)

	r0, err := SysCreate(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	if r0 != rmsInvalidRFM {
		t.Errorf("r0 = %d, want rmsInvalidRFM (%d)", r0, rmsInvalidRFM)
	}
}

// TestSysCreate_noNameGiven confirms a file spec naming only a device,
// with no file name at all, is rejected rather than reaching
// vol.CreateFile with an empty name.
func TestSysCreate_noNameGiven(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:")

	r0, err := SysCreate(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	if r0 != rmsFileNotFound {
		t.Errorf("r0 = %d, want rmsFileNotFound (%d)", r0, rmsFileNotFound)
	}
}

// TestSysCreate_directoryNotFound confirms a file spec naming a
// subdirectory that doesn't exist on the volume fails cleanly (RMS$_FNF)
// rather than panicking inside filespec.ResolveDirectory.
func TestSysCreate_directoryNotFound(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:[NOSUCHDIR]TEST.DAT")

	r0, err := SysCreate(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	if r0 != rmsFileNotFound {
		t.Errorf("r0 = %d, want rmsFileNotFound (%d)", r0, rmsFileNotFound)
	}
}

// TestSysCreate_logicalNameTranslation confirms a file spec that is
// itself a defined logical name is translated before being parsed —
// here, a made-up logical pointing at "TTA0:" should reach the same
// console path TestSysCreate_console exercises directly.
func TestSysCreate_logicalNameTranslation(t *testing.T) {
	f := newCreateFixture(t, true)
	f.ctx.Logicals.Set("LNM$FILE_DEV", "MYOUT", "TTA0:", 0)
	newFAB(t, f.ctx, "MYOUT")

	r0, err := SysCreate(f.ctx, []uint32{testFabAddr})
	if err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	ifi := readWord(t, f.ctx, testFabAddr+fabIFI)

	h, ok := f.ctx.Files.Lookup(ifi)
	if !ok || !h.IsConsole() {
		t.Fatal("logical-name-translated spec did not resolve to the console handle")
	}
}
