package console_test

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/machine"
)

// newStackTopPage is UserStackTop's P1 page number, where a new space's P1
// starts with paging on (Phase 67).
const newStackTopPage = (corevms.UserStackTop - 0x40000000) / 512

// TestVirtualPageCount: with paging on, $CREPRC gives a new process a
// table area of vax.sysgen.virtualpagecnt entries, P1 starting as
// process 1's does, and SYI$_VIRTUALPAGECNT reports it; unset, it's
// VMINIT's /P0 plus /P1.
func TestVirtualPageCount(t *testing.T) {
	setSetting(t, machine.PagingSetting, "true")

	code, _ := assembleAt(t, createChild)
	c, _ := scheduledConsole(t, longQuantum, code)
	one := c.RTL

	if data, ok := one.SYIItem("SYI$_VIRTUALPAGECNT"); !ok || corevms.ItemLongword(data) != 16384+8192 {
		t.Errorf("SYI$_VIRTUALPAGECNT %v unset, want /P0 + /P1", corevms.ItemLongword(data))
	}

	setSetting(t, "vax.sysgen.virtualpagecnt", "%X2400")

	c, _ = scheduledConsole(t, longQuantum, code)
	one = c.RTL

	if data, ok := one.SYIItem("SYI$_VIRTUALPAGECNT"); !ok || corevms.ItemLongword(data) != 0x2400 {
		t.Errorf("SYI$_VIRTUALPAGECNT %v, want 9216", corevms.ItemLongword(data))
	}

	if one.Space.Entries != 16384+8192 {
		t.Errorf("process 1's area has %d entries, want VMINIT's /P0 + /P1", one.Space.Entries)
	}

	// The program's data longwords, in a P0 page nothing has created.
	if err := one.EnsurePages(dataAddr, 16); err != nil {
		t.Fatal(err)
	}

	if r0 := stepUntilStored(t, c); r0 != 1 {
		t.Fatalf("$CREPRC returned %08X, want SS$_NORMAL", r0)
	}

	child, ok := one.FindProcess(longwordAt(t, c, one, dataAddr+4))
	if !ok {
		t.Fatal("no new process")
	}

	sp := child.Space

	// 0x2400 entries: 5120 for P0's table (40 pages) and 4096 for P1's
	// (32), P1 from the user stack's top up.
	if sp.Entries != 0x2400 || sp.P0LR != 0 || sp.P1LR != newStackTopPage || sp.P1BR+0x200000*4 != sp.Table+72*512 {
		t.Errorf("the new process's space %+v", sp)
	}

	backed := 0

	for va := sp.Table; va < sp.Table+72*512; va += 512 {
		if _, _, pte, err := c.Mem.LookupPTE(c.CPU, va); err == nil && pte.Valid() {
			backed++
		}
	}

	if backed != 32 {
		t.Errorf("the new process: %d of its 72 table pages backed, want P1's 32", backed)
	}

	if got := sp.P0Capacity(); got != 0x2400-4096 {
		t.Errorf("the new process's P0 can grow to %d pages, want %d", got, 0x2400-4096)
	}
}
