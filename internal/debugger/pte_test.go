package debugger_test

import (
	"bytes"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/debugger"
	"github.com/tucats/govax/internal/vm"
)

// newPagedConsole returns a console whose VMINIT has built real P0 and S0
// page tables (so a page table entry, a PTE, exists to change), with a
// debugger installed. A VAX divides memory into 512-byte pages, and a PTE is
// the longword that says whether a virtual page is valid, where it lives in
// physical memory (its PFN, page frame number), and who may access it.
func newPagedConsole(t *testing.T) *console.Console {
	t.Helper()

	var buf bytes.Buffer

	c := console.New(&buf)
	if err := c.Init(4096 * 512); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// 200 P1 pages, not 100: kernel.asm's ".p1vector" needs them.
	if err := c.VMInit(2000, 200, 0, 4, 4, 4, 4, 8); err != nil {
		t.Fatalf("VMInit: %v", err)
	}

	debugger.Install(c, consoletest.DebugGrammar(t), nil)

	return c
}

// TestSetPTEWithRange: SET PTE address TO address field=value changes every
// page from the first address's to the second's, inclusive, and no other.
func TestSetPTEWithRange(t *testing.T) {
	c := newPagedConsole(t)

	say(t, c, "SET PTE 200 TO 400 VALID=1,PFN=10")

	for _, addr := range []uint32{0x200, 0x400} {
		_, _, pte, err := c.Mem.LookupPTE(c.CPU, addr)
		if err != nil {
			t.Fatalf("LookupPTE(%#x): %v", addr, err)
		}

		if !pte.Valid() {
			t.Errorf("addr %#x: expected valid bit set", addr)
		}
	}

	// 0x000 (page 0) is below the range and must be untouched.
	if _, _, pte, err := c.Mem.LookupPTE(c.CPU, 0x000); err != nil {
		t.Fatalf("LookupPTE(0x0): %v", err)
	} else if pte.Valid() {
		t.Error("addr 0x0: expected the valid bit untouched (outside the TO range)")
	}
}

// TestSetPTEProtectionNames sets PROT= by the PTE$K_ name SHOW PTE prints
// for each of the 16 protection codes, so every name SHOW PTE shows can be
// typed back into SET PTE.
func TestSetPTEProtectionNames(t *testing.T) {
	c := newPagedConsole(t)

	say(t, c, "SET PTE 200 VALID=1,PFN=10")

	for code := vm.Protection(0); code < 16; code++ {
		cmd := "SET PTE 200 PROT=PTE$K_" + code.String()
		say(t, c, cmd)

		_, _, pte, err := c.Mem.LookupPTE(c.CPU, 0x200)
		if err != nil {
			t.Fatalf("LookupPTE: %v", err)
		}

		if got := pte.Protection(); got != code {
			t.Errorf("%s: PROT = %d, want %d", cmd, got, code)
		}
	}
}

// TestSetPSLFields: SET PSL sets several fields of the processor status
// longword at once (N and V are condition codes, IPL the interrupt priority
// level), reading the values in the debugger's hexadecimal radix, and an
// unknown field is an error.
func TestSetPSLFields(t *testing.T) {
	c, _ := newTestConsole(t)

	say(t, c, "SET PSL N=1,V=1,IPL=10")

	psl := c.CPU.PSL()
	if !psl.N() || !psl.V() || psl.IPL() != 16 {
		t.Errorf("PSL = %#x, want N/V set and IPL=16 (0x10)", uint32(psl))
	}

	if _, err := sayErr(c, "SET PSL BOGUS=1"); err == nil {
		t.Error("expected an error for an unknown PSL field")
	}
}
