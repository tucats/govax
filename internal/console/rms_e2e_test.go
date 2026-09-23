package console

import (
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/ods2/diskimage"
	"github.com/tucats/ods2/volume"
)

// The literal P1-vector addresses (internal/rtl/p1vector.go) that
// testdata/asm/rms_roundtrip.asm's own ".set /perm sys$xxx ..." lines are
// built from, duplicated here as plain constants for the same reason
// internal/rtl/rms_test.go duplicates them: this test can't import
// internal/rtl's unexported table, and re-deriving them from anywhere else
// would just be indirection around six well-known, fixed VMS constants.
const (
	rmsE2ESysCreateAddr  = 0x7FFEE1C8
	rmsE2ESysConnectAddr = 0x7FFEE1C0
	rmsE2ESysPutAddr     = 0x7FFEE188
	rmsE2ESysCloseAddr   = 0x7FFEE1B8
	rmsE2ESysOpenAddr    = 0x7FFEE208
	rmsE2ESysGetAddr     = 0x7FFEE180
)

// depositP1VectorTrampolines writes a real, CALLS-compatible P1-vector stub
// at each of the six real VMS system-service-vector addresses
// rms_roundtrip.asm calls through CALLS/@#, matching the real reference
// tool's own p1_vector.c p1_init byte for byte: a 2-byte zero procedure-
// entry mask (so CALLS's own frame-build logic -- internal/cpu/call.go's
// buildCallFrame, which every CALLS unconditionally reads a mask through,
// real VAX ISA behavior -- finds a valid, empty mask rather than misreading
// arbitrary bytes as one), then the 2-byte "XFC #XFC$P1VECTOR" instruction
// itself (opcode 0xFC, selector byte 0x7A -- internal/cpu/xfc.go's
// xfcP1Vector), then a RET (opcode 0x04) to unwind the CALLS frame that XFC's
// own handler never touches -- without it, execution falls straight through
// into whatever memory follows the stub once SystemService returns.
// internal/cpu's XFC$P1VECTOR handler (emulXfcP1Vector) computes the
// dispatch address as PC-4 specifically to land back on this stub's own base
// address once both the mask-skip and the XFC fetch have advanced PC past
// them (see that function's own doc comment) -- so the mask word here isn't
// just CALLS bookkeeping, it's required for the address arithmetic to land
// on the real, well-known SYS$xxx address at all.
//
// Nothing in internal/asm or internal/console deposits this trampoline
// today: internal/asm/pseudo.go's own ".P1VECTOR" pseudo-op is a
// deliberate no-op (its own doc comment explains why: building it for real
// would need internal/asm to import internal/rtl's service table,
// deferred), and internal/console/asm.go's depositAsmImage only ever
// copies the assembler's own P0/S0 byte ranges into memory, with no path
// for an arbitrary absolute P1 address like these. So this test does by
// hand, once, exactly what a future ".P1VECTOR" implementation would do as
// part of microkernel bootstrap.
func depositP1VectorTrampolines(t *testing.T, c *Console) {
	t.Helper()

	addrs := []uint32{
		rmsE2ESysCreateAddr,
		rmsE2ESysConnectAddr,
		rmsE2ESysPutAddr,
		rmsE2ESysCloseAddr,
		rmsE2ESysOpenAddr,
		rmsE2ESysGetAddr,
	}

	for _, addr := range addrs {
		if err := c.storeBytes(addr, []byte{0x00, 0x00, 0xFC, 0x7A, 0x04}); err != nil {
			t.Fatalf("depositing P1-vector stub at %#x: %v", addr, err)
		}
	}
}

// mountFreshRMSVolume builds a throwaway ODS-2 container via ods2's own
// diskimage.Create + volume.Initialize (docs/PHASE-22.md's "Test fixtures"
// design decision: generated on the fly in t.TempDir(), never a committed
// binary blob) and mounts it read/write on device name "DUA0" -- the same
// pattern internal/rms/mount_test.go's newTestVolumeFile and
// internal/rtl/rms_test.go's newMountedVolumeFixture already use, just at
// the Console level (c.Mounts) rather than a bare *rms.MountTable or
// *rtl.Environment.
func mountFreshRMSVolume(t *testing.T, c *Console) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "test.dsk")

	container, err := diskimage.Create(path, 400)
	if err != nil {
		t.Fatalf("diskimage.Create: %v", err)
	}

	if err := volume.Initialize(container, volume.InitializeOptions{Label: "TESTVOL"}); err != nil {
		_ = container.Close()
		t.Fatalf("volume.Initialize: %v", err)
	}

	if err := container.Close(); err != nil {
		t.Fatalf("closing freshly initialized container: %v", err)
	}

	if err := c.Mounts.Mount("DUA0", path, true); err != nil {
		t.Fatalf("Mounts.Mount: %v", err)
	}
}

// TestRMSRoundTrip_assembledProgram is docs/PHASE-22.md's subtask 14
// acceptance test: a real, assembled and executed MACRO-32 program
// (testdata/asm/rms_roundtrip.asm) drives SYS$CREATE -> SYS$CONNECT ->
// SYS$PUT (x3) -> SYS$CLOSE -> SYS$OPEN -> SYS$CONNECT -> SYS$GET (x3) ->
// SYS$CLOSE against a mounted ODS-2 disk device, through real CALLS
// instructions reaching real fetched/executed XFC$P1VECTOR trampolines.
// Every other RMS test in this project (internal/rms/*_test.go,
// internal/rtl/rms_test.go) either calls the internal/rms handlers
// directly or drives them through env.SystemService(pc) from Go -- never
// through genuinely assembled and executed VAX instructions, which is
// exactly the gap this subtask closes. The assembled program itself
// verifies each of the three records read back matches what was written
// (CMPL/BNEQ to a FAIL label on any mismatch or non-success RMS status);
// this test only has to confirm the program actually reached that success
// path, via R0.
func TestRMSRoundTrip_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)

	mountFreshRMSVolume(t, c)
	depositP1VectorTrampolines(t, c)

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "rms_roundtrip.asm"))
	if err != nil {
		t.Fatalf("Assemble(rms_roundtrip.asm): %v", err)
	}

	if !hasEntry {
		t.Fatal("expected rms_roundtrip.asm's \".end main\" to report an entry address")
	}

	runErr, hitCap := callBounded(t, c, addr, 100_000)
	if runErr != nil {
		t.Fatalf("running rms_roundtrip.asm: %v", runErr)
	}

	if hitCap {
		t.Fatal("rms_roundtrip.asm did not reach a HALT/return within 100,000 steps")
	}

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Errorf("R0 = %d, want 1 (all three records round-tripped correctly)", got)
	}
}
