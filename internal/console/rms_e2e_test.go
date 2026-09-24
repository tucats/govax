package console

import (
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/ods2/diskimage"
	"github.com/tucats/ods2/volume"
)

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
// instructions reaching real fetched/executed XFC$P1VECTOR trampolines --
// built by the fixture's own ".microkernel"/".p1vector" statements
// (internal/asm/pseudo.go's pseudoP1Vector) rather than deposited by this
// test by hand, now that .P1VECTOR is a real implementation instead of a
// no-op. Every other RMS test in this project (internal/rms/*_test.go,
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

// TestRMSRoundTrip_afterKernelAlreadyP1VectoredIsIdempotent covers the real
// scenario a plain govax run's own boot sequence produces: vax.init's own
// "asm kernel.asm" already runs kernel.asm's ".p1vector" line once, in the
// same persistent console session (c.asmSession) any subsequent "ASM
// <file>" command shares -- so a file like rms_roundtrip.asm that carries
// its own ".p1vector" line (needed only because its own test above runs it
// standalone, without the full console boot kernel.asm would otherwise have
// given it for free) must not fail with a duplicate-symbol error the second
// time .P1VECTOR runs. See pseudoP1Vector's own doc comment on why re-
// running it is safe (matches p1_init()'s real, naturally idempotent
// set_symbol_direct calls).
func TestRMSRoundTrip_afterKernelAlreadyP1VectoredIsIdempotent(t *testing.T) {
	c := newBootableConsole(t)

	mountFreshRMSVolume(t, c)

	if _, _, err := c.Assemble(asmFixturePath(t, "kernel.asm")); err != nil {
		t.Fatalf("Assemble(kernel.asm): %v", err)
	}

	addr, hasEntry, err := c.Assemble(asmFixturePath(t, "rms_roundtrip.asm"))
	if err != nil {
		t.Fatalf("Assemble(rms_roundtrip.asm) after kernel.asm: %v", err)
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
