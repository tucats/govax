package console

import (
	"bytes"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func exeFixturePath(t *testing.T, name string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "exe", name)
}

// newRunnableConsole returns a Console with enough physical/virtual memory
// set up (INIT + VMINIT) and running in kernel mode to exercise imageLoad,
// matching console_run's own "switch to kernel mode" precondition.
func newRunnableConsole(t testing.TB) *Console {
	t.Helper()

	c := New(&bytes.Buffer{})
	if err := c.Init(4096 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(2000, 100, 0, 4, 4, 4, 4, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	return c
}

// TestImageLoad_simpleExe checks imageLoad against the real testdata/exe/
// simple.exe fixture's header, cross-checked by hand against a raw hexdump
// of the file (see docs/PHASE-13.md's progress log for the derivation):
// IHD.offset_ident (@2 bytes 0x0030->wait see below) locates the IHI block
// at file offset 0x60, whose IMGNAML/IMGNAM read "SIMPLE" -- confirmed
// directly in the raw bytes (offset 0x60 is 0x06 followed by ASCII
// "SIMPLE") -- and the transfer array's first two longwords are
// 0x7FFEDF68 (a P1-space address, out of the <0x3FFFFFFF main-image range)
// and 0x00000200 (the real P0 entry point selected by console_run's own
// transfer[0]-then-transfer[1] fallback).
func TestImageLoad_simpleExe(t *testing.T) {
	c := newRunnableConsole(t)
	c.Engine.SetModeStack(vax.Kernel, false)

	icb, err := c.imageLoad(exeFixturePath(t, "simple.exe"), icbMain)
	if err != nil {
		t.Fatalf("imageLoad: %v", err)
	}

	if icb.Transfer[0] != 0x7FFEDF68 {
		t.Errorf("Transfer[0] = %#x, want 0x7ffedf68", icb.Transfer[0])
	}

	if icb.Transfer[1] != 0x00000200 {
		t.Errorf("Transfer[1] = %#x, want 0x200", icb.Transfer[1])
	}

	if icb.Base != 0 {
		t.Errorf("Base = %#x, want 0 (first image loaded)", icb.Base)
	}

	if len(icb.ISDList) == 0 {
		t.Fatal("expected at least one ISD")
	}

	if icb.Flags&icbIncomplete != 0 {
		t.Error("expected ICB_INCOMPLETE cleared after a successful load")
	}

	if got := c.findMainICB(); got != icb {
		t.Errorf("findMainICB = %v, want the loaded ICB", got)
	}

	// simple.exe (per docs/PHASE-13.md's fixture notes) has exactly one
	// FIXUPVEC section and depends on four sharable images (LIBRTL,
	// DECC$SHR, MTHRTL, CMA$TIS_SHR) plus its own self-reference (id 0) --
	// five SHR entries in total, none of which are present as real
	// loadable files under testdata/, so each secondary imageLoad is
	// expected to fail and be tolerated (VAX_FNF), leaving each
	// dependency's Icb/Base unset.
	if icb.FixupISD == nil {
		t.Fatal("expected a FIXUPVEC section")
	}

	if len(icb.SHRList) != 5 {
		t.Fatalf("SHRList = %d entries, want 5 (self + 4 dependencies)", len(icb.SHRList))
	}

	if icb.SHRList[0].ID != 0 || icb.SHRList[0].Icb != icb {
		t.Errorf("SHRList[0] should be the self-reference entry, got %+v", icb.SHRList[0])
	}
	// These are the IAF sharable-image list's own (short, unversioned)
	// names -- matching kernel.asm's own `.shim` library field spellings
	// ("LIBRTL", "DECC$SHR", ...), which is exactly what SHIM$<name>_
	// <offset> fixup resolution needs. Distinct from -- and not to be
	// confused with -- the fuller "DECC$SHR_001"-style names that appear
	// as several ISDs' own NAME field (a separate, GBL-section-type-
	// specific naming convention this port doesn't otherwise consume).
	gotNames := map[string]bool{}
	for _, shr := range icb.SHRList[1:] {
		gotNames[shr.Name] = true
	}
	
	for _, want := range []string{"LIBRTL", "DECC$SHR", "MTHRTL", "CMA$TIS_SHR"} {
		if !gotNames[want] {
			t.Errorf("SHR dependency %q not found among %v", want, gotNames)
		}
	}
}

// TestImageLoad_alreadyLoadedIsNoOp matches image_load's own "already
// loaded" short-circuit: a second call with the same name returns the same
// ICB without re-parsing the file or growing the P0 high-water mark.
func TestImageLoad_alreadyLoadedIsNoOp(t *testing.T) {
	c := newRunnableConsole(t)
	c.Engine.SetModeStack(vax.Kernel, false)

	path := exeFixturePath(t, "put.exe")

	first, err := c.imageLoad(path, icbSecondary)
	if err != nil {
		t.Fatalf("imageLoad: %v", err)
	}
	
	highWater := c.RTL.RegionSize[0]

	second, err := c.imageLoad(path, icbSecondary)
	if err != nil {
		t.Fatalf("imageLoad (again): %v", err)
	}

	if first != second {
		t.Error("expected the same *ICB on a repeat load")
	}

	if c.RTL.RegionSize[0] != highWater {
		t.Errorf("high-water mark changed on a repeat load: %#x -> %#x", highWater, c.RTL.RegionSize[0])
	}
}

// TestImageFixup_simpleExe loads simple.exe and its (recursively loaded --
// tolerantly, since none of its dependencies are present as real files
// under testdata/) sharable-image dependencies, then fixes it up: every G^
// fixup must resolve through ensureShims's synthesized SHIM$ symbols, since
// none of DECC$SHR/MTHRTL/LIBRTL/CMA$TIS_SHR is an actually-loaded ICB.
func TestImageFixup_simpleExe(t *testing.T) {
	c := newRunnableConsole(t)
	c.Engine.SetModeStack(vax.Kernel, false)

	if err := c.ensureShims(); err != nil {
		t.Fatalf("ensureShims: %v", err)
	}

	icb, err := c.imageLoad(exeFixturePath(t, "simple.exe"), icbMain)
	if err != nil {
		t.Fatalf("imageLoad: %v", err)
	}

	if err := c.imageFixup(icb); err != nil {
		t.Fatalf("imageFixup: %v", err)
	}

	if icb.Flags&icbFixed == 0 {
		t.Error("expected ICB_FIXED set after a successful fixup")
	}

	// A second fixup pass must be a no-op (already fixed), not an error or
	// a re-application of the same rewrite.
	if err := c.imageFixup(icb); err != nil {
		t.Fatalf("imageFixup (again): %v", err)
	}
}

// TestImageFixup_everyRealFixtureFixesUp runs imageLoad+imageFixup across
// every real fixture (mirroring TestImageLoad_everyRealFixtureLoads),
// reporting whichever SHIM$ symbol (if any) is missing so a gap in
// shimTable is easy to see, matching docs/PHASE-13.md's own bar: an
// unresolved reference should be reported clearly, not silently ignored.
func TestImageFixup_everyRealFixtureFixesUp(t *testing.T) {
	for _, name := range []string{"cli.exe", "dbl.exe", "getvm.exe", "put.exe", "putc.exe", "sieve.exe", "simple.exe"} {
		t.Run(name, func(t *testing.T) {
			c := newRunnableConsole(t)
			c.Engine.SetModeStack(vax.Kernel, false)

			if err := c.ensureShims(); err != nil {
				t.Fatalf("ensureShims: %v", err)
			}

			icb, err := c.imageLoad(exeFixturePath(t, name), icbMain)
			if err != nil {
				t.Fatalf("imageLoad(%s): %v", name, err)
			}

			if err := c.imageFixup(icb); err != nil {
				t.Errorf("imageFixup(%s): %v", name, err)
			}
		})
	}
}

// TestImageLoad_everyRealFixtureLoads is a smoke test: imageLoad must
// succeed against every real fixture in testdata/exe/ (put1.exe is a
// zero-byte file, excluded per docs/PHASE-12.md's own scope note), each in
// a fresh Console so one fixture's high-water mark/ICB list can't affect
// another's.
func TestImageLoad_everyRealFixtureLoads(t *testing.T) {
	for _, name := range []string{"cli.exe", "dbl.exe", "getvm.exe", "put.exe", "putc.exe", "sieve.exe", "simple.exe"} {
		t.Run(name, func(t *testing.T) {
			c := newRunnableConsole(t)
			c.Engine.SetModeStack(vax.Kernel, false)

			icb, err := c.imageLoad(exeFixturePath(t, name), icbMain)
			if err != nil {
				t.Fatalf("imageLoad(%s): %v", name, err)
			}

			if icb.Flags&icbIncomplete != 0 {
				t.Errorf("%s: ICB_INCOMPLETE still set after load", name)
			}

			if len(icb.ISDList) == 0 {
				t.Errorf("%s: expected at least one ISD", name)
			}
		})
	}
}

// TestImageLoad_missingFileReportsError matches image_load's VAX_FNF path
// for a top-level load (as opposed to the tolerated secondary-image case
// exercised inside TestImageLoad_simpleExe).
func TestImageLoad_missingFileReportsError(t *testing.T) {
	c := newRunnableConsole(t)
	c.Engine.SetModeStack(vax.Kernel, false)

	if _, err := c.imageLoad("no-such-image", icbMain); err == nil {
		t.Error("expected an error for a nonexistent image file")
	}
}
