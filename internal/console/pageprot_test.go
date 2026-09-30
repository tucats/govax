package console

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// More status values the page services return.
var (
	ssIvProtect = vmsdef.Symbols["SS$_IVPROTECT"]
	ssLenVio    = vmsdef.Symbols["SS$_LENVIO"]
	ssWasClr    = vmsdef.Symbols["SS$_WASCLR"]
	ssWasSet    = vmsdef.Symbols["SS$_WASSET"]
)

const vaPrvprt = 0x8020

func TestSetprt(t *testing.T) {
	c := newServiceConsole(t)

	// Two ordinary P0 pages (UW since VMINIT), made user read-only.
	putRange(t, c, vaInadr, vaPages, vaPages+0x200)

	if err := c.storeLong(vaPages+0x208, 0x77); err != nil {
		t.Fatal(err)
	}

	if got := callService(t, c, "SYS$SETPRT", vaInadr, vaRetadr, 0, uint32(vm.ProtUR), vaPrvprt); got != ssNormal {
		t.Fatalf("$SETPRT = %#x", got)
	}

	if a, b := getRange(t, c, vaRetadr); a != vaPages || b != vaPages+0x3FF {
		t.Errorf("retadr %#x-%#x", a, b)
	}

	if p, _ := c.Mem.LoadByte(c.CPU, vaPrvprt); p != byte(vm.ProtUW) {
		t.Errorf("prvprt %d, want UW (%d)", p, vm.ProtUW)
	}

	pte := pteAt(t, c, vaPages+0x200)
	if pte.Protection() != vm.ProtUR || pte.Owner() != uint8(vax.User) || !pte.Valid() {
		t.Errorf("PTE %#x: want UR, still user-owned and valid", uint32(pte))
	}

	if v, err := c.Mem.LoadLongword(c.CPU, vaPages+0x208); err != nil || v != 0x77 {
		t.Errorf("read %#x, %v; want the data kept", v, err)
	}

	if err := c.Mem.StoreLongword(c.CPU, vaPages+0x208, 1); err == nil {
		t.Error("a user read-only page was written")
	}

	// 0 means kernel read-only; 1 is reserved.
	putRange(t, c, vaInadr, vaPages, vaPages)

	if got := callService(t, c, "SYS$SETPRT", vaInadr, 0, 0, 0); got != ssNormal || pteAt(t, c, vaPages).Protection() != vm.ProtKR {
		t.Errorf("$SETPRT 0 = %#x, protection %d; want KR", got, pteAt(t, c, vaPages).Protection())
	}

	if got := callService(t, c, "SYS$SETPRT", vaInadr, 0, 0, 1); got != ssIvProtect {
		t.Errorf("$SETPRT 1 = %#x, want SS$_IVPROTECT", got)
	}
}

func TestSetprt_errors(t *testing.T) {
	c := newServiceConsole(t)

	cases := []struct {
		name  string
		addr  uint32
		want  uint32
		setup func()
	}{
		{"system page", 0x80000000, ssNoPriv, nil},
		{"beyond P0", (c.CPU.PR(vax.P0LR) + 4) * 512, ssLenVio, nil},
		{"deleted page", vaPages, ssAccVio, func() {
			putRange(t, c, vaInadr, vaPages, vaPages)
			callService(t, c, "SYS$DELTVA", vaInadr)
		}},
	}

	for _, tc := range cases {
		if tc.setup != nil {
			tc.setup()
		}

		putRange(t, c, vaInadr, tc.addr, tc.addr)

		if got := callService(t, c, "SYS$SETPRT", vaInadr, vaRetadr, 0, uint32(vm.ProtUR)); got != tc.want {
			t.Errorf("%s: $SETPRT = %#x, want %#x", tc.name, got, tc.want)
		}

		if a, _ := getRange(t, c, vaRetadr); a != 0xFFFFFFFF {
			t.Errorf("%s: retadr %#x, want -1", tc.name, a)
		}
	}

	// User mode can't change a kernel page.
	putRange(t, c, vaInadr, vaPages+0x1000, vaPages+0x1000)
	callService(t, c, "SYS$CRETVA", vaInadr, 0, 0)
	c.Engine.SetModeStack(vax.User, false)

	if got := callService(t, c, "SYS$SETPRT", vaInadr, 0, 0, uint32(vm.ProtUR)); got != ssPagOwnVio {
		t.Errorf("user $SETPRT of a kernel page = %#x, want SS$_PAGOWNVIO", got)
	}
}

func TestLockServices(t *testing.T) {
	c := newServiceConsole(t)

	putRange(t, c, vaInadr, vaPages, vaPages+0x200)

	steps := []struct {
		service string
		want    uint32
	}{
		{"SYS$LKWSET", ssWasClr},
		{"SYS$LKWSET", ssWasSet},
		{"SYS$LCKPAG", ssWasClr}, // a different kind of lock
		{"SYS$ULWSET", ssWasSet},
		{"SYS$ULWSET", ssWasClr},
		{"SYS$ULKPAG", ssWasSet},
	}

	for i, st := range steps {
		if got := callService(t, c, st.service, vaInadr, vaRetadr); got != st.want {
			t.Errorf("step %d %s = %#x, want %#x", i, st.service, got, st.want)
		}
	}

	if a, b := getRange(t, c, vaRetadr); a != vaPages || b != vaPages+0x3FF {
		t.Errorf("retadr %#x-%#x", a, b)
	}

	// Deleting a page forgets its locks.
	callService(t, c, "SYS$LKWSET", vaInadr)
	callService(t, c, "SYS$DELTVA", vaInadr)
	callService(t, c, "SYS$CRETVA", vaInadr)

	if got := callService(t, c, "SYS$LKWSET", vaInadr); got != ssWasClr {
		t.Errorf("$LKWSET of recreated pages = %#x, want SS$_WASCLR", got)
	}

	// Image rundown unlocks everything.
	c.RTL.ImageRundown()

	if got := callService(t, c, "SYS$LKWSET", vaInadr); got != ssWasClr {
		t.Errorf("$LKWSET after rundown = %#x, want SS$_WASCLR", got)
	}

	// A page that doesn't exist, a system page.
	putRange(t, c, vaInadr, vaPages+0x2000, vaPages+0x2000)
	callService(t, c, "SYS$DELTVA", vaInadr)

	if got := callService(t, c, "SYS$LCKPAG", vaInadr); got != ssAccVio {
		t.Errorf("$LCKPAG of a deleted page = %#x, want SS$_ACCVIO", got)
	}

	putRange(t, c, vaInadr, 0x80000000, 0x80000000)

	if got := callService(t, c, "SYS$LKWSET", vaInadr); got != ssNoPriv {
		t.Errorf("$LKWSET of a system page = %#x, want SS$_NOPRIV", got)
	}
}

// TestPageProt_assembledProgram runs testdata/asm/pageprot.asm
// (docs/PHASE-26.md subtask 36): a page made read-only, written (an
// access violation), made writable again, then locked and unlocked.
func TestPageProt_assembledProgram(t *testing.T) {
	c := newBootableConsole(t)
	word := runFixture(t, c, "pageprot.asm")

	if got := c.CPU.GPR(vax.R0); got != 1 {
		t.Fatalf("R0 = %#x, want 1 (a $SETPRT failed?)", got)
	}

	checks := []struct {
		sym  string
		want uint32
	}{
		{"PRVPRT", uint32(vm.ProtUW)}, // one byte, over the ^XFF
		{"FAULTS", 1},
		{"VALUE", 0x22},
		{"LOCK1", ssWasClr},
		{"LOCK2", ssWasSet},
		{"UNLOCK", ssWasSet},
	}

	for _, ck := range checks {
		if got := word(ck.sym); got != ck.want {
			t.Errorf("%s = %#x, want %#x", ck.sym, got, ck.want)
		}
	}
}
