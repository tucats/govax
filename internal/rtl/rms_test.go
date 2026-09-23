package rtl

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/tucats/ods2/diskimage"
	"github.com/tucats/ods2/volume"
)

// This file tests registerRMSServices (rms.go) and the Environment/Console
// wiring docs/PHASE-22.md's subtask 11 added — NOT internal/rms's own field-
// layout/status-code/business-logic correctness, which that package's own,
// much larger test suite (internal/rms/*_test.go) already covers
// exhaustively. What's new and worth testing here is purely the glue: does
// a SYS$CREATE/SYS$CONNECT/SYS$PUT/SYS$CLOSE/SYS$OPEN/SYS$GET call, made
// exactly the way a real running VAX program would make it (a CALLS/CALLG
// through SystemService, not a direct Go function call into internal/rms),
// actually reach internal/rms's real handlers with this Environment's own
// console output and MountTable correctly threaded through?
//
// The FAB$.../RAB$... byte offsets used below duplicate a handful of
// internal/rms/fab.go's and rab.go's own unexported constants, since this
// package can't see those (different Go package) and has no reason to
// import internal/rms just to read them — internal/rms/fab_test.go and
// rab_test.go already independently verify every one of these offsets is
// correct; this file just needs literal, known-good values to build a
// realistic FAB/RAB in test memory.
const (
	testFabFAC = 22 // FAB$B_FAC
	testFabIFI = 2  // FAB$W_IFI
	testFabORG = 29 // FAB$B_ORG
	testFabRAT = 30 // FAB$B_RAT
	testFabRFM = 31 // FAB$B_RFM
	testFabFNA = 44 // FAB$L_FNA
	testFabFNS = 52 // FAB$B_FNS
	testFabMRS = 54 // FAB$W_MRS

	testRabFAB = 60 // RAB$L_FAB
	testRabRAC = 30 // RAB$B_RAC
	testRabRSZ = 34 // RAB$W_RSZ
	testRabRBF = 40 // RAB$L_RBF

	testFacPut = 0x01 // FAB$V_PUT
	testFacGet = 0x02 // FAB$V_GET

	testOrgSeq   = 0 // FAB$C_SEQ
	testRfmFixed = 1 // FAB$C_FIX
)

// The six p1Vector addresses these tests dispatch through SystemService,
// copied from p1vector.go's own table (this file intentionally calls
// SystemService the same way a real CALLS/CALLG instruction would resolve
// one of these addresses, rather than reaching into p1Vector directly).
const (
	testPCSysCreate  = 0x7FFEE1C8
	testPCSysConnect = 0x7FFEE1C0
	testPCSysPut     = 0x7FFEE188
	testPCSysClose   = 0x7FFEE1B8
	testPCSysOpen    = 0x7FFEE208
	testPCSysGet     = 0x7FFEE180
)

// putByte/putBytes/readBytes round out this package's existing put*/read*
// test helpers (rtl_test.go's putLongword/putString, core_test.go's
// putWord) with the byte-level ones this file's FAB/RAB/record fixtures
// need.
func putByte(t *testing.T, env *Environment, addr uint32, v byte) {
	t.Helper()

	if err := env.mem.StoreByte(env.cpu, addr, v); err != nil {
		t.Fatalf("StoreByte(%#x): %v", addr, err)
	}
}

func putBytes(t *testing.T, env *Environment, addr uint32, data []byte) {
	t.Helper()

	for i, b := range data {
		putByte(t, env, addr+uint32(i), b)
	}
}

func readBytes(t *testing.T, env *Environment, addr uint32, n int) []byte {
	t.Helper()

	buf := make([]byte, n)

	for i := range buf {
		b, err := env.mem.LoadByte(env.cpu, addr+uint32(i))
		if err != nil {
			t.Fatalf("LoadByte(%#x): %v", addr+uint32(i), err)
		}

		buf[i] = b
	}

	return buf
}

func readWordEnv(t *testing.T, env *Environment, addr uint32) uint16 {
	t.Helper()

	v, err := env.mem.LoadWord(env.cpu, addr)
	if err != nil {
		t.Fatalf("LoadWord(%#x): %v", addr, err)
	}

	return v
}

// callService dispatches pc through env.SystemService exactly the way
// cpu.SystemServices' XFC$P1VECTOR handler does for a real CALLS/CALLG,
// with argv already laid out at ap via putArgs — this is what makes these
// tests exercise the real dispatch path (ServiceTable.Lookup by p1Vector
// name, callHandler's panic recovery, and so on) rather than calling an
// internal/rms function directly.
func callService(t *testing.T, env *Environment, pc uint32, ap uint32, argv []uint32) uint32 {
	t.Helper()

	putArgs(t, env, ap, argv)

	r0, handled, err := env.SystemService(pc)
	if err != nil {
		t.Fatalf("SystemService(%#x): %v", pc, err)
	}

	if !handled {
		t.Fatalf("SystemService(%#x) not handled, want a registered RMS handler", pc)
	}

	return r0
}

// rmsSuccess reports whether r0's low bit (STS$K_SUCCESS) is set — the
// same BLBC-style check a real compiled VAX program makes on an RMS
// completion code, and internal/rms/status_test.go's own convention for
// checking this without needing every individual rms* status constant
// (which, like the offsets above, are unexported and belong to a
// different package).
func rmsSuccess(r0 uint32) bool {
	return r0&1 == 1
}

// newConsoleFAB lays out a minimal FAB at fabAddr naming the TTA0: console
// pseudo-device — just enough for SYS$CREATE/SYS$OPEN's own console
// special case (create.go/open.go), which never inspects ORG/RAT/RFM/MRS
// at all.
func newConsoleFAB(t *testing.T, env *Environment, fabAddr, fnaAddr uint32, fac byte) {
	t.Helper()

	spec := "TTA0:"

	putByte(t, env, fabAddr+testFabFAC, fac)
	putLongword(t, env, fabAddr+testFabFNA, fnaAddr)
	putByte(t, env, fabAddr+testFabFNS, byte(len(spec)))
	putBytes(t, env, fnaAddr, []byte(spec))
}

// newDiskFAB lays out a FAB at fabAddr naming a real file on a mounted
// ODS-2 volume ("DUA0:TEST.DAT"), sequential/Fixed-80, matching
// internal/rms/create_test.go's own newFAB fixture.
func newDiskFAB(t *testing.T, env *Environment, fabAddr, fnaAddr uint32, fac byte) {
	t.Helper()

	spec := "DUA0:TEST.DAT"

	putByte(t, env, fabAddr+testFabFAC, fac)
	putByte(t, env, fabAddr+testFabORG, testOrgSeq)
	putByte(t, env, fabAddr+testFabRFM, testRfmFixed)
	putByte(t, env, fabAddr+testFabRAT, 0)
	putWord(t, env, fabAddr+testFabMRS, 80)
	putLongword(t, env, fabAddr+testFabFNA, fnaAddr)
	putByte(t, env, fabAddr+testFabFNS, byte(len(spec)))
	putBytes(t, env, fnaAddr, []byte(spec))
}

// newMountedVolumeFixture returns a path to a freshly initialized, empty
// ODS-2 container (via the sibling ods2 module's own diskimage.Create +
// volume.Initialize — the same two calls docs/PHASE-22.md's "Test
// fixtures" design decision calls for, and internal/rms's own
// mount_test.go uses under a different, unexported helper name this
// package can't reach) and mounts it read/write on env.Mounts under
// "DUA0". t.TempDir() cleans the underlying file up automatically.
func newMountedVolumeFixture(t *testing.T, env *Environment) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.dsk")

	c, err := diskimage.Create(path, 400)
	if err != nil {
		t.Fatalf("diskimage.Create: %v", err)
	}

	if err := volume.Initialize(c, volume.InitializeOptions{Label: "TESTVOL"}); err != nil {
		_ = c.Close()
		t.Fatalf("volume.Initialize: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("closing freshly initialized container: %v", err)
	}

	if err := env.Mounts.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mounts.Mount: %v", err)
	}
}

// TestRMSServices_consoleRoundTrip drives SYS$CREATE -> SYS$CONNECT ->
// SYS$PUT -> SYS$CLOSE against the TTA0: console pseudo-device entirely
// through env.SystemService, confirming the record bytes (plus SYS$PUT's
// own trailing newline) land in this Environment's real console output
// stream (out) -- proving rmsContext correctly threads env's consoleOut
// into the rms.Context every wrapper closure in rms.go builds.
func TestRMSServices_consoleRoundTrip(t *testing.T) {
	env, out := fixture()

	const (
		fabAddr = 0x1000
		fnaAddr = 0x1100
		rabAddr = 0x2000
		recAddr = 0x3000
		ap      = 0x8000
	)

	newConsoleFAB(t, env, fabAddr, fnaAddr, testFacPut)

	r0 := callService(t, env, testPCSysCreate, ap, []uint32{fabAddr})
	if !rmsSuccess(r0) {
		t.Fatalf("SYS$CREATE r0 = %#x, want success", r0)
	}

	ifi := readWordEnv(t, env, fabAddr+testFabIFI)
	if ifi == 0 {
		t.Fatal("FAB$W_IFI left at 0 after a successful SYS$CREATE")
	}

	putLongword(t, env, rabAddr+testRabFAB, fabAddr)

	r0 = callService(t, env, testPCSysConnect, ap, []uint32{rabAddr})
	if !rmsSuccess(r0) {
		t.Fatalf("SYS$CONNECT r0 = %#x, want success", r0)
	}

	putByte(t, env, rabAddr+testRabRAC, racSeqTest)
	putBytes(t, env, recAddr, []byte("hello from rtl wiring test"))
	putLongword(t, env, rabAddr+testRabRBF, recAddr)
	putWord(t, env, rabAddr+testRabRSZ, uint16(len("hello from rtl wiring test")))

	r0 = callService(t, env, testPCSysPut, ap, []uint32{rabAddr})
	if !rmsSuccess(r0) {
		t.Fatalf("SYS$PUT r0 = %#x, want success", r0)
	}

	r0 = callService(t, env, testPCSysClose, ap, []uint32{fabAddr})
	if !rmsSuccess(r0) {
		t.Fatalf("SYS$CLOSE r0 = %#x, want success", r0)
	}

	want := "hello from rtl wiring test\n"
	if out.String() != want {
		t.Errorf("console output = %q, want %q", out.String(), want)
	}
}

// racSeqTest mirrors internal/rms/rab.go's unexported racSeq (RAB$C_SEQ =
// 0) — named separately here only so this file's test bodies read as
// self-documenting as internal/rms's own tests do, without importing that
// package's unexported constant (which, again, this package can't do at
// all).
const racSeqTest = 0

// TestRMSServices_diskRoundTrip drives a full write-then-reopen-then-read
// round trip against a real, freshly mounted ODS-2 volume entirely through
// env.SystemService: SYS$CREATE -> SYS$CONNECT -> SYS$PUT -> SYS$CLOSE
// writes one 80-byte Fixed-format record, then SYS$OPEN -> SYS$CONNECT ->
// SYS$GET -> SYS$CLOSE reads it back through a second, independent
// FAB/RAB pair -- confirming env.Mounts (Console.Mounts, injected by
// NewEnvironment) is the same MountTable a MOUNT command would populate,
// and that a real ODS-2-backed file survives round-tripping through this
// Environment's dispatch layer, not just internal/rms's own direct-call
// tests.
func TestRMSServices_diskRoundTrip(t *testing.T) {
	env, _ := fixture()
	newMountedVolumeFixture(t, env)

	const (
		fabAddr   = 0x1000
		fnaAddr   = 0x1100
		rabAddr   = 0x2000
		writeAddr = 0x3000
		readAddr  = 0x4000
		ap        = 0x8000
	)

	record := make([]byte, 80)
	copy(record, "hello, ods2 disk file, via SystemService")

	// --- write side: CREATE -> CONNECT -> PUT -> CLOSE ---

	newDiskFAB(t, env, fabAddr, fnaAddr, testFacPut)

	r0 := callService(t, env, testPCSysCreate, ap, []uint32{fabAddr})
	if !rmsSuccess(r0) {
		t.Fatalf("SYS$CREATE r0 = %#x, want success", r0)
	}

	putLongword(t, env, rabAddr+testRabFAB, fabAddr)

	r0 = callService(t, env, testPCSysConnect, ap, []uint32{rabAddr})
	if !rmsSuccess(r0) {
		t.Fatalf("SYS$CONNECT (write) r0 = %#x, want success", r0)
	}

	putByte(t, env, rabAddr+testRabRAC, racSeqTest)
	putBytes(t, env, writeAddr, record)
	putLongword(t, env, rabAddr+testRabRBF, writeAddr)
	putWord(t, env, rabAddr+testRabRSZ, uint16(len(record)))

	r0 = callService(t, env, testPCSysPut, ap, []uint32{rabAddr})
	if !rmsSuccess(r0) {
		t.Fatalf("SYS$PUT r0 = %#x, want success", r0)
	}

	r0 = callService(t, env, testPCSysClose, ap, []uint32{fabAddr})
	if !rmsSuccess(r0) {
		t.Fatalf("SYS$CLOSE (write) r0 = %#x, want success", r0)
	}

	// --- read side: OPEN -> CONNECT -> GET -> CLOSE, a fresh FAB/RAB ---

	newDiskFAB(t, env, fabAddr, fnaAddr, testFacGet)

	r0 = callService(t, env, testPCSysOpen, ap, []uint32{fabAddr})
	if !rmsSuccess(r0) {
		t.Fatalf("SYS$OPEN r0 = %#x, want success", r0)
	}

	r0 = callService(t, env, testPCSysConnect, ap, []uint32{rabAddr})
	if !rmsSuccess(r0) {
		t.Fatalf("SYS$CONNECT (read) r0 = %#x, want success", r0)
	}

	putByte(t, env, rabAddr+testRabRAC, racSeqTest)
	// Point RAB$L_RBF at a fresh, still-zeroed buffer distinct from
	// writeAddr, so a SYS$GET that silently did nothing couldn't make
	// this test pass by accident on leftover memory contents.
	putLongword(t, env, rabAddr+testRabRBF, readAddr)
	putWord(t, env, rabAddr+testRabRSZ, 0)

	r0 = callService(t, env, testPCSysGet, ap, []uint32{rabAddr})
	if !rmsSuccess(r0) {
		t.Fatalf("SYS$GET r0 = %#x, want success", r0)
	}

	if gotLen := readWordEnv(t, env, rabAddr+testRabRSZ); gotLen != uint16(len(record)) {
		t.Errorf("RAB$W_RSZ after SYS$GET = %d, want %d", gotLen, len(record))
	}

	got := readBytes(t, env, readAddr, len(record))
	if !bytes.Equal(got, record) {
		t.Errorf("record read back = %q, want %q", got, record)
	}

	r0 = callService(t, env, testPCSysClose, ap, []uint32{fabAddr})
	if !rmsSuccess(r0) {
		t.Fatalf("SYS$CLOSE (read) r0 = %#x, want success", r0)
	}
}

// TestRMSServices_unmountedDeviceReachesEnvironmentMounts confirms a
// SYS$CREATE targeting a disk device with nothing mounted reaches
// internal/rms's real device-not-ready handling (rather than, say, a nil-
// pointer panic from an unwired Mounts field) -- the negative-path
// counterpart to TestRMSServices_diskRoundTrip's positive one, and a
// direct check that env.Mounts is the exact, empty-but-non-nil MountTable
// fixture() constructed, not left nil.
func TestRMSServices_unmountedDeviceReachesEnvironmentMounts(t *testing.T) {
	env, _ := fixture()

	const (
		fabAddr = 0x1000
		fnaAddr = 0x1100
		ap      = 0x8000
	)

	newDiskFAB(t, env, fabAddr, fnaAddr, testFacPut)

	r0 := callService(t, env, testPCSysCreate, ap, []uint32{fabAddr})
	if rmsSuccess(r0) {
		t.Fatalf("SYS$CREATE against an unmounted device r0 = %#x, want failure", r0)
	}

	if ifi := readWordEnv(t, env, fabAddr+testFabIFI); ifi != 0 {
		t.Errorf("FAB$W_IFI = %d after a failed SYS$CREATE, want 0 (untouched)", ifi)
	}
}
