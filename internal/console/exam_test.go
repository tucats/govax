package console

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func newTestConsole(t *testing.T) (*Console, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer

	c := New(&buf)
	if err := c.Init(64 * 1024); err != nil {
		t.Fatalf("Init: %v", err)
	}

	return c, &buf
}

func TestInit_allocatesAndZeroes(t *testing.T) {
	c, _ := newTestConsole(t)
	if !c.Initialized() {
		t.Fatal("expected Initialized() true after Init")
	}

	if c.Mem.Size() != 64*1024 {
		t.Errorf("Mem.Size() = %d, want 65536", c.Mem.Size())
	}
	
	if c.DepositAddr != 0x200 {
		t.Errorf("DepositAddr = %#x, want 0x200", c.DepositAddr)
	}
	
	wantSP := c.Mem.Size() - 4
	if c.CPU.GPR(vax.SP) != wantSP {
		t.Errorf("SP = %#x, want %#x", c.CPU.GPR(vax.SP), wantSP)
	}
}

func TestInit_roundsMemorySize(t *testing.T) {
	c := New(&bytes.Buffer{})
	// alloc_vax's own rounding: below-minimum sizes clamp to 8192, and
	// since that clamped value differs from the raw request, a further
	// 512-byte round-up is added on top (see allocPhysMemory's doc
	// comment) — this matches the C source exactly, quirky as it looks.
	if err := c.Init(100); err != nil {
		t.Fatalf("Init: %v", err)
	}
	
	if c.Mem.Size() != minPhysMemory+physMemAlign {
		t.Errorf("Mem.Size() = %d, want %d", c.Mem.Size(), minPhysMemory+physMemAlign)
	}
}

func TestAllocPhysMemory_alreadyAligned(t *testing.T) {
	// A request that's already an exact, sufficiently large multiple of
	// 512 round-trips unchanged.
	if got := allocPhysMemory(64 * 1024); got != 64*1024 {
		t.Errorf("allocPhysMemory(64K) = %d, want 65536", got)
	}
}

func TestExamineDeposit_registerRoundTrip(t *testing.T) {
	c, buf := newTestConsole(t)
	if err := c.Deposit("R3", 0, SizeLongword, 0xDEADBEEF); err != nil {
		t.Fatalf("Deposit: %v", err)
	}
	
	buf.Reset()
	
	if err := c.Examine("R3", 0, 1, SizeLongword); err != nil {
		t.Fatalf("Examine: %v", err)
	}
	
	if !strings.Contains(buf.String(), "DEADBEEF") {
		t.Errorf("output = %q, want it to contain DEADBEEF", buf.String())
	}
}

func TestExamineDeposit_memoryRoundTrip(t *testing.T) {
	c, buf := newTestConsole(t)
	if err := c.Deposit("", 0x1000, SizeLongword, 0x12345678); err != nil {
		t.Fatalf("Deposit: %v", err)
	}
	
	if c.DepositAddr != 0x1004 {
		t.Errorf("DepositAddr after Deposit = %#x, want 0x1004", c.DepositAddr)
	}

	buf.Reset()
	
	if err := c.Examine("", 0x1000, 1, SizeLongword); err != nil {
		t.Fatalf("Examine: %v", err)
	}
	
	got := buf.String()
	if !strings.Contains(got, "00001000:") || !strings.Contains(got, "12345678") {
		t.Errorf("output = %q, want address+value", got)
	}
	
	if c.DepositAddr != 0x1004 {
		t.Errorf("DepositAddr after Examine = %#x, want 0x1004", c.DepositAddr)
	}
}

func TestExamine_byteAndWordSizes(t *testing.T) {
	c, buf := newTestConsole(t)
	if err := c.Deposit("", 0x2000, SizeWord, 0xABCD); err != nil {
		t.Fatalf("Deposit: %v", err)
	}
	
	buf.Reset()
	
	if err := c.Examine("", 0x2000, 1, SizeWord); err != nil {
		t.Fatalf("Examine: %v", err)
	}
	
	if !strings.Contains(buf.String(), "ABCD") {
		t.Errorf("output = %q, want ABCD", buf.String())
	}
}

func TestExamine_ascii(t *testing.T) {
	c, buf := newTestConsole(t)
	msg := "HI"
	
	for i, ch := range []byte(msg) {
		if err := c.Deposit("", 0x3000+uint32(i), SizeByte, uint32(ch)); err != nil {
			t.Fatalf("Deposit: %v", err)
		}
	}
	
	buf.Reset()

	if err := c.Examine("", 0x3000, 2, SizeASCII); err != nil {
		t.Fatalf("Examine: %v", err)
	}
	
	got := buf.String()
	if !strings.Contains(got, "H") || !strings.Contains(got, "I") {
		t.Errorf("output = %q, want to contain H and I", got)
	}
}

func TestExamine_unknownRegister(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.Examine("R99", 0, 1, SizeLongword); err == nil {
		t.Error("expected error for unknown register")
	}
}

func TestExamine_requiresInit(t *testing.T) {
	c := New(&bytes.Buffer{})
	if err := c.Examine("", 0, 1, SizeLongword); err == nil {
		t.Error("expected error before Init")
	}
}

func TestZero_clearsMemoryAndSymbols(t *testing.T) {
	c, _ := newTestConsole(t)
	if err := c.Deposit("", 0x1000, SizeLongword, 0xFFFFFFFF); err != nil {
		t.Fatalf("Deposit: %v", err)
	}
	
	c.Symbols.Set("FOO", 42, SymbolUser)

	if err := c.Zero(); err != nil {
		t.Fatalf("Zero: %v", err)
	}

	v, err := c.Mem.LoadLongword(c.CPU, 0x1000)
	if err != nil {
		t.Fatalf("LoadLongword: %v", err)
	}

	if v != 0 {
		t.Errorf("memory not zeroed: %#x", v)
	}
	
	if _, ok := c.Symbols.Get("FOO"); ok {
		t.Error("expected FOO symbol to be cleared")
	}
}
