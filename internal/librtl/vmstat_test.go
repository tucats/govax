package librtl

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/tucats/govax/internal/corevms"
)

// Cells the statistics tests pass by reference.
const (
	statCode  = 0x1300 // code
	statValue = 0x1304 // value-argument
)

// statFixture is an RTL environment with a stack, whose P0 ends at
// 0x4000, and whose SYS$OUTPUT is the buffer returned.
func statFixture(t *testing.T) (*corevms.Environment, *bytes.Buffer) {
	t.Helper()

	env, out, _ := timerFixture(t)
	env.RegionSize[0] = 0x4000

	return env, out
}

// statVM calls LIB$STAT_VM for code and returns the status and the
// value written (0xDEADBEEF when none was).
func statVM(t *testing.T, env *corevms.Environment, code uint32) (uint32, uint32) {
	t.Helper()

	putLongword(t, env, statCode, code)
	putLongword(t, env, statValue, 0xDEADBEEF)

	r0 := call(t, env, "LIB$STAT_VM", statCode, statValue)

	return r0, longword(t, env, statValue)
}

// wantStats fails the test unless LIB$STAT_VM's codes 1 to 3 and 5 to 7
// return want (in that order).
func wantStats(t *testing.T, env *corevms.Environment, step string, want [6]uint32) {
	t.Helper()

	for i, code := range []uint32{1, 2, 3, 5, 6, 7} {
		if r0, v := statVM(t, env, code); r0 != ssNormal || v != want[i] {
			t.Errorf("%s: STAT_VM(%d) = %#x, %d; want SS$_NORMAL, %d", step, code, r0, int32(v), int32(want[i]))
		}
	}
}

// TestLibStatVM: the heap's counts follow successful GET_VM and FREE_VM
// calls, in the default zone and the program's own, with FREE_VM's size
// or, omitted, the size GET_VM asked; failed calls and a user zone's
// calls don't count, nor do RESET's frees. The page counts follow the
// pool, a zone's areas among them.
func TestLibStatVM(t *testing.T) {
	env, _ := statFixture(t)

	wantStats(t, env, "fresh", [6]uint32{})

	a := mustGetVM(t, env, 10, 0)
	b := mustGetVM(t, env, 100, 0)

	gets, frees, pages := env.PageStatistics()
	if gets == 0 || frees != 0 || pages == 0 {
		t.Fatalf("PageStatistics after the default zone's first block: %d, %d, %d", gets, frees, pages)
	}

	wantStats(t, env, "two blocks", [6]uint32{2, 0, 110, gets, 0, pages})

	// Failures count nowhere.
	if r0, _ := getVM(t, env, 0, 0); r0 != libBadBlockSize {
		t.Errorf("GET_VM(0) = %#x", r0)
	}

	if r0 := freeVM(t, env, 0, a+4, 0); r0 != libBadBlockAddress {
		t.Errorf("FREE_VM of an interior address = %#x", r0)
	}

	wantStats(t, env, "failures", [6]uint32{2, 0, 110, gets, 0, pages})

	if r0 := freeVM(t, env, 10, a, 0); r0 != ssNormal {
		t.Fatalf("FREE_VM(10) = %#x", r0)
	}

	if r0 := freeVM(t, env, 0, b, 0); r0 != ssNormal {
		t.Fatalf("FREE_VM without a size = %#x", r0)
	}

	wantStats(t, env, "both freed", [6]uint32{2, 2, 0, gets, 0, pages})

	// The program's pages, and a zone's area: one more request each.
	p := mustGetPages(t, env, 3)
	wantStats(t, env, "three pages", [6]uint32{2, 2, 0, gets + 1, 0, pages + 3})

	if r0 := freePages(t, env, 1, p+512); r0 != ssNormal {
		t.Fatalf("FREE_VM_PAGE = %#x", r0)
	}

	wantStats(t, env, "a page freed", [6]uint32{2, 2, 0, gets + 1, 1, pages + 2})

	zone := mustCreateZone(t, env, 1, omit, omit, 2) // first fit, 2-page areas
	g, _, n := env.PageStatistics()
	c := mustGetVM(t, env, 24, zone)

	wantStats(t, env, "a zone's block", [6]uint32{3, 2, 24, g + 1, 1, n + 2})

	// RESET frees the block, but not by FREE_VM; DELETE gives the area
	// back to the pool.
	if r0 := zoneCall(t, env, "LIB$RESET_VM_ZONE", zone); r0 != ssNormal {
		t.Fatalf("RESET_VM_ZONE = %#x", r0)
	}

	if r0 := freeVM(t, env, 24, c, zone); r0 != libBadBlockAddress {
		t.Errorf("FREE_VM after RESET = %#x, want LIB$_BADBLOADR", r0)
	}

	if r0 := zoneCall(t, env, "LIB$DELETE_VM_ZONE", zone); r0 != ssNormal {
		t.Fatalf("DELETE_VM_ZONE = %#x", r0)
	}

	wantStats(t, env, "the zone deleted", [6]uint32{3, 2, 24, g + 1, 2, n})

	// A user zone's GET_VM is its routine's, uncounted.
	user := createUserZone(t, env, 7, 0x2000, 0x2100)
	putLongword(t, env, vmSizeCell, 16)
	putLongword(t, env, vmZoneCell, user)
	_ = tailCallOf(t, env, "LIB$GET_VM", vmSizeCell, vmAddrCell, vmZoneCell)

	if r0, v := statVM(t, env, 1); r0 != ssNormal || v != 3 {
		t.Errorf("STAT_VM(1) after a user zone's GET_VM = %#x, %d; want 3", r0, v)
	}

	// Image rundown starts the counts again.
	env.ImageRundown()
	wantStats(t, env, "after rundown", [6]uint32{})
}

// TestLibStatVMBadCodes: codes 0 and 4, those outside 1 to 7, and an
// omitted code are LIB$_INVARG, with nothing written.
func TestLibStatVMBadCodes(t *testing.T) {
	env, _ := statFixture(t)

	for _, code := range []uint32{0, 4, 8, 0xFFFFFFFF} {
		if r0, v := statVM(t, env, code); r0 != libInvArg || v != 0xDEADBEEF {
			t.Errorf("STAT_VM(%d) = %#x, wrote %#x; want LIB$_INVARG, nothing", int32(code), r0, v)
		}
	}

	if r0 := call(t, env, "LIB$STAT_VM", 0, statValue); r0 != libInvArg {
		t.Errorf("STAT_VM without a code = %#x, want LIB$_INVARG", r0)
	}
}

// TestLibShowVM: the manual's line for code 0 (or none), the page
// routines' for 4, each single statistic's, and LIB$_INVARG for other
// codes, each line written to SYS$OUTPUT with a blank first.
func TestLibShowVM(t *testing.T) {
	env, out := statFixture(t)

	if r0 := call(t, env, "LIB$SHOW_VM"); r0 != ssNormal ||
		out.String() != " 0 calls to LIB$GET_VM, 0 calls to LIB$FREE_VM, 0 bytes still allocated\n" {
		t.Errorf("SHOW_VM, fresh: %#x, %q", r0, out.String())
	}

	a := mustGetVM(t, env, 40, 0)
	mustGetVM(t, env, 2, 0)

	if r0 := freeVM(t, env, 40, a, 0); r0 != ssNormal {
		t.Fatalf("FREE_VM = %#x", r0)
	}

	p := mustGetPages(t, env, 5)
	if r0 := freePages(t, env, 2, p); r0 != ssNormal {
		t.Fatalf("FREE_VM_PAGE = %#x", r0)
	}

	gets, frees, pages := env.PageStatistics()

	for code, want := range []string{
		0: " 2 calls to LIB$GET_VM, 1 calls to LIB$FREE_VM, 2 bytes still allocated\n",
		1: " 2 calls to LIB$GET_VM\n",
		2: " 1 calls to LIB$FREE_VM\n",
		3: " 2 bytes still allocated\n",
		4: " " + itoa(gets) + " calls to LIB$GET_VM_PAGE, " + itoa(frees) + " calls to LIB$FREE_VM_PAGE, " +
			itoa(pages) + " pages still allocated\n",
		5: " " + itoa(gets) + " calls to LIB$GET_VM_PAGE\n",
		6: " " + itoa(frees) + " calls to LIB$FREE_VM_PAGE\n",
		7: " " + itoa(pages) + " pages still allocated\n",
	} {
		out.Reset()
		putLongword(t, env, statCode, uint32(code))

		if r0 := call(t, env, "LIB$SHOW_VM", statCode); r0 != ssNormal || out.String() != want {
			t.Errorf("SHOW_VM(%d): %#x, %q; want %q", code, r0, out.String(), want)
		}
	}

	out.Reset()

	for _, code := range []uint32{8, 0xFFFFFFFF} {
		putLongword(t, env, statCode, code)

		if r0 := call(t, env, "LIB$SHOW_VM", statCode); r0 != libInvArg {
			t.Errorf("SHOW_VM(%d) = %#x, want LIB$_INVARG", int32(code), r0)
		}
	}

	if out.Len() != 0 {
		t.Errorf("the INVARG calls wrote %q", out.String())
	}
}

// TestLibShowVMAction: with an action routine, SHOW_VM writes nothing
// and asks for a tail call of it with a class S descriptor of the line
// and the user argument, 0 when omitted.
func TestLibShowVMAction(t *testing.T) {
	env, out := statFixture(t)

	mustGetVM(t, env, 5, 0)

	const action = 0x2000

	for _, c := range []struct {
		argv  []uint32
		value uint32
	}{
		{[]uint32{0, action, 42}, 42},
		{[]uint32{0, action}, 0},
	} {
		req := tailCallOf(t, env, "LIB$SHOW_VM", c.argv...)

		if req.Routine != action || longword(t, env, req.ArgList) != 2 || longword(t, env, req.ArgList+8) != c.value {
			t.Errorf("SHOW_VM%v: routine %#x, %d arguments, value %d", c.argv, req.Routine,
				longword(t, env, req.ArgList), longword(t, env, req.ArgList+8))
		}

		desc := longword(t, env, req.ArgList+4)

		line, err := readString(env, desc)
		if err != nil || line != " 1 calls to LIB$GET_VM, 0 calls to LIB$FREE_VM, 5 bytes still allocated" {
			t.Errorf("the routine's line: %q, %v", line, err)
		}

		if longword(t, env, desc)>>16 != 0x010E {
			t.Errorf("descriptor %#x, want class S, type T", longword(t, env, desc))
		}
	}

	if out.Len() != 0 {
		t.Errorf("SHOW_VM wrote %q", out.String())
	}
}

// itoa is n in decimal.
func itoa(n uint32) string {
	return strconv.FormatUint(uint64(n), 10)
}
