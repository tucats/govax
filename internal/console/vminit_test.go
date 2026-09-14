package console

import (
	"bytes"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestVMInit_enablesTranslation(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.Init(128 * 512); err != nil { // 128 pages of physical RAM
		t.Fatalf("Init: %v", err)
	}

	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	if c.CPU.PR(vax.MAPEN) != 1 {
		t.Fatal("expected MAPEN enabled after VMInit")
	}
	if !c.VMInitValid {
		t.Error("expected VMInitValid true")
	}

	// A P0 virtual address (page 1, since page 0 is guarded) should
	// round-trip through the page table VMInit built.
	if err := c.Mem.StoreLongword(c.CPU, 0x200, 0xDEADBEEF); err != nil {
		t.Fatalf("StoreLongword through P0: %v", err)
	}
	v, err := c.Mem.LoadLongword(c.CPU, 0x200)
	if err != nil {
		t.Fatalf("LoadLongword through P0: %v", err)
	}
	if v != 0xDEADBEEF {
		t.Errorf("got %#x, want 0xdeadbeef", v)
	}
}

func TestVMInit_guardsFirstP0Page(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.Init(128 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2); err != nil {
		t.Fatalf("VMInit: %v", err)
	}
	if _, err := c.Mem.LoadLongword(c.CPU, 0x0); err == nil {
		t.Error("expected an access violation reading the guarded P0 page 0")
	}
}

func TestVMInit_stackPointersAreDistinctAndInS0(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.Init(128 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := c.VMInit(20, 20, 0, 2, 2, 2, 2); err != nil {
		t.Fatalf("VMInit: %v", err)
	}
	ksp := c.CPU.PR(vax.KSP)
	esp := c.CPU.PR(vax.ESP)
	ssp := c.CPU.PR(vax.SSP)
	isp := c.CPU.PR(vax.ISP)
	if ksp == esp || esp == ssp || ssp == isp || ksp == 0 {
		t.Errorf("expected distinct nonzero stacks: KSP=%#x ESP=%#x SSP=%#x ISP=%#x", ksp, esp, ssp, isp)
	}
	if c.CPU.GPR(vax.SP) != ksp {
		t.Errorf("SP = %#x, want KSP %#x", c.CPU.GPR(vax.SP), ksp)
	}
}

func TestVMInit_requiresInit(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.VMInit(1, 1, 0, 1, 1, 1, 1); err == nil {
		t.Error("expected error before Init")
	}
}

func TestVMInit_rejectsOversizedRequest(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.Init(128 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := c.VMInit(1000, 1000, 0, 1, 1, 1, 1); err == nil {
		t.Error("expected error for a VM request exceeding physical memory")
	}
}
