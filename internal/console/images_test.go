package console

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vm"
)

// newSecondProcess builds a second process on a booted console: an
// Environment in the process table and an address space of its own. It
// isn't current: the CPU's registers still describe process 1.
func newSecondProcess(t *testing.T, c *Console) *corevms.Environment {
	t.Helper()

	env, err := corevms.NewEnvironment(c.RTL.System, c.Logicals, nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	if env.Space, err = c.RTL.BuildAddressSpace(env.Process.PID, 1024, 256); err != nil {
		t.Fatal(err)
	}

	return env
}

// TestActivateImageInAnotherProcess activates an image in a process that
// isn't the current one (bugs 1 and 5 of docs/PHASE-43.md): it lands in
// that process's P0, its ICB list and symbols are that process's, and its
// IMAGE$INIT driver is in a page of its own, while process 1's P0, ICB
// list, symbols, and driver are untouched.
func TestActivateImageInAnotherProcess(t *testing.T) {
	c := bootedConsole(t)

	if err := c.ensureShims(); err != nil {
		t.Fatal(err)
	}

	env := newSecondProcess(t, c)
	p := c.imagesOf(env)
	one := c.RTL.Space.AddressSpace

	path := exeFixturePath(t, "simple.exe")

	main, err := p.activateImage(path)
	if err != nil {
		t.Fatalf("activateImage: %v", err)
	}

	// The image's first section is in process 2's P0, where its ISD
	// says.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	isd := main.ISDList[0]
	file := data[(isd.VBN-1)*512:][:64]
	va := main.Base + uint32(isd.VPN)<<9

	got := make([]byte, 64)
	if err := c.Mem.LoadIn(c.CPU, env.Space.AddressSpace, va, got); err != nil || !bytes.Equal(got, file) {
		t.Errorf("process 2's %08X = % x (%v), want the section's % x", va, got, err, file)
	}

	// Process 1's P0 page there was never touched.
	if pte, err := c.Mem.LookupPTEIn(c.CPU, one, va); err != nil || pte.Valid() {
		t.Errorf("process 1's %08X PTE %#x (%v), want it still invalid", va, uint32(pte), err)
	}

	if len(p.ICBList) == 0 || p.ICBList[0] != main || len(c.images().ICBList) != 0 {
		t.Errorf("ICB lists: process 2 %d, process 1 %d", len(p.ICBList), len(c.images().ICBList))
	}

	if env.RegionSize[0] == 0 || c.RTL.RegionSize[0] != 0 {
		t.Errorf("P0 high-water marks: process 2 %#x, process 1 %#x", env.RegionSize[0], c.RTL.RegionSize[0])
	}

	if _, ok := p.symbols["MAIN"]; !ok {
		t.Error("process 2 has no MAIN symbol")
	}

	if v, ok := c.Symbols.Get("MAIN"); ok {
		t.Errorf("the console's MAIN is %08X, want process 2's activation to leave it undefined", v)
	}

	// The driver: in a pool page charged to process 2, not at
	// CONSOLE$SCRATCH+8.
	scratch, _ := c.Symbols.Get("CONSOLE$SCRATCH")

	before := make([]byte, 64)
	if err := c.Mem.LoadIn(c.CPU, one, scratch+8, before); err != nil {
		t.Fatal(err)
	}

	driver, ok, err := p.buildImageInitDriver(main, false)
	if err != nil || !ok {
		t.Fatalf("buildImageInitDriver: %v, %v", ok, err)
	}

	owned := false

	for _, a := range c.RTL.S0Pool().Allocations() {
		if a.Addr == driver && a.PID == env.Process.PID {
			owned = true
		}
	}

	if !owned {
		t.Errorf("driver at %08X isn't a pool page of process 2's", driver)
	}

	after := make([]byte, 64)
	if err := c.Mem.LoadIn(c.CPU, one, scratch+8, after); err != nil || !bytes.Equal(before, after) {
		t.Error("process 2's driver changed CONSOLE$SCRATCH")
	}

	// It ends with PUSHL R0, CALLS #1 to SYS$EXIT, and RET (the P1
	// vector is mapped in process 2 too).
	code := make([]byte, 64)
	if err := c.Mem.LoadIn(c.CPU, vm.AddressSpace{}, driver, code); err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(code, []byte{0xDD, 0x50, 0xFB}) {
		t.Errorf("driver % x has no $EXIT call", code)
	}

	// A second driver for the same process reuses its page.
	if again, _, _ := p.buildImageInitDriver(main, false); again != driver {
		t.Errorf("second driver at %08X, want %08X", again, driver)
	}

	// Removing the process gives the driver's page back with the rest.
	c.RTL.RemoveProcess(env)

	for _, a := range c.RTL.S0Pool().Allocations() {
		if a.PID == env.Process.PID {
			t.Errorf("process 2's %s still allocated after RemoveProcess", a.Purpose)
		}
	}
}

// TestProcessOneImagesSurviveInit: process 1's image state is the
// console's, so an image list survives an INIT, as it always has.
func TestProcessOneImagesSurviveInit(t *testing.T) {
	c := bootedConsole(t)
	c.images().ICBList = []*ICB{{Name: "<MAIN>"}}

	if c.imagesOf(c.RTL) != c.images() || c.imagesOf(nil) != c.images() {
		t.Error("imagesOf(process 1) isn't process 1's image state")
	}

	if err := c.Init(c.Mem.Size()); err != nil {
		t.Fatal(err)
	}

	if len(c.images().ICBList) != 1 || c.imagesOf(c.RTL) != c.images() {
		t.Error("process 1's image list was lost at INIT")
	}
}

// TestOtherImagesDroppedWithSystem: a new System (INIT) forgets other
// processes' image state.
func TestOtherImagesDroppedWithSystem(t *testing.T) {
	c := bootedConsole(t)
	env := newSecondProcess(t, c)
	c.imagesOf(env).ICBList = []*ICB{{Name: "<MAIN>"}}

	if err := c.Init(c.Mem.Size()); err != nil {
		t.Fatal(err)
	}

	if len(c.otherImages) != 0 {
		t.Errorf("%d other processes' image states kept across INIT", len(c.otherImages))
	}
}
