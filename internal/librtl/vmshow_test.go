package librtl

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
)

// Cells the zone debugging tests pass by reference.
const (
	zoneScanCell  = 0x1400 // LIB$FIND_VM_ZONE's context
	zoneFoundCell = 0x1404 // its zone-id
	showZone      = 0x1408 // LIB$SHOW_VM_ZONE's and LIB$VERIFY_VM_ZONE's zone-id
	showDetail    = 0x140C // detail-level
)

// findZones runs LIB$FIND_VM_ZONE's scan from a context of 0 to its end
// and returns the IDs it found, failing the test unless each call is
// SS$_NORMAL and the last LIB$_NOTFOU, leaving zone-id alone.
func findZones(t *testing.T, env *corevms.Environment) []uint32 {
	t.Helper()

	var ids []uint32

	putLongword(t, env, zoneScanCell, 0)

	for range 100 {
		putLongword(t, env, zoneFoundCell, 0xDEADBEEF)

		switch r0 := call(t, env, "LIB$FIND_VM_ZONE", zoneScanCell, zoneFoundCell); r0 {
		case ssNormal:
			ids = append(ids, longword(t, env, zoneFoundCell))
		case libNotFou:
			if v := longword(t, env, zoneFoundCell); v != 0xDEADBEEF {
				t.Errorf("LIB$FIND_VM_ZONE's end wrote %#x", v)
			}

			return ids
		default:
			t.Fatalf("LIB$FIND_VM_ZONE = %#x after %X", r0, ids)
		}
	}

	t.Fatal("LIB$FIND_VM_ZONE never ended")

	return nil
}

// verifyZone calls LIB$VERIFY_VM_ZONE of zone.
func verifyZone(t *testing.T, env *corevms.Environment, zone uint32) uint32 {
	t.Helper()

	putLongword(t, env, showZone, zone)

	return call(t, env, "LIB$VERIFY_VM_ZONE", showZone)
}

// showZoneLines calls LIB$SHOW_VM_ZONE of zone at detail and returns its
// status and the lines it wrote to SYS$OUTPUT (out).
func showZoneLines(t *testing.T, env *corevms.Environment, out fmt.Stringer, zone, detail uint32) (uint32, []string) {
	t.Helper()

	before := len(out.String())

	putLongword(t, env, showZone, zone)
	putLongword(t, env, showDetail, detail)

	r0 := call(t, env, "LIB$SHOW_VM_ZONE", showZone, showDetail)
	text := strings.TrimSuffix(out.String()[before:], "\n")

	if text == "" {
		return r0, nil
	}

	return r0, strings.Split(text, "\n")
}

// wantLines fails the test unless got is want, showing both.
func wantLines(t *testing.T, step string, got []string, want ...string) {
	t.Helper()

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s:\n%s\nwant:\n%s", step, strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// LIB$FIND_VM_ZONE: the default zone, the string zone, then the
// program's zones, user-defined too, in the order they were made; a
// deleted zone isn't found, and a scan in progress goes on past it; the
// argument count.
func TestLibFindVMZone(t *testing.T) {
	e := userFixture(t)

	if ids := findZones(t, e); len(ids) != 2 || ids[0] != e.DefaultZoneID() || ids[1] != e.StringZoneID() {
		t.Fatalf("a fresh image's zones: %X, want the default zone %X and the string zone %X",
			ids, e.DefaultZoneID(), e.StringZoneID())
	}

	a := mustCreateZone(t, e, 1)
	u := createUserZone(t, e, 7)
	b := mustCreateZone(t, e, 2, 4)

	want := []uint32{e.DefaultZoneID(), e.StringZoneID(), a, u, b}
	if ids := findZones(t, e); fmt.Sprint(ids) != fmt.Sprint(want) {
		t.Errorf("zones %X, want %X", ids, want)
	}

	// A scan that has found the user zone goes on to b when a and the
	// user zone are deleted, and a zone made since comes after b.
	putLongword(t, e, zoneScanCell, 0)

	for range 4 {
		if r0 := call(t, e, "LIB$FIND_VM_ZONE", zoneScanCell, zoneFoundCell); r0 != ssNormal {
			t.Fatalf("LIB$FIND_VM_ZONE = %#x", r0)
		}
	}

	if r0 := zoneCall(t, e, "LIB$DELETE_VM_ZONE", a); r0 != ssNormal {
		t.Fatalf("LIB$DELETE_VM_ZONE = %#x", r0)
	}

	c := mustCreateZone(t, e, 1)

	for _, wantID := range []uint32{b, c} {
		if r0 := call(t, e, "LIB$FIND_VM_ZONE", zoneScanCell, zoneFoundCell); r0 != ssNormal || longword(t, e, zoneFoundCell) != wantID {
			t.Errorf("LIB$FIND_VM_ZONE = %#x, %X; want %X", r0, longword(t, e, zoneFoundCell), wantID)
		}
	}

	if r0 := call(t, e, "LIB$FIND_VM_ZONE", zoneScanCell); r0 != libWroNumArg {
		t.Errorf("LIB$FIND_VM_ZONE with one argument = %#x, want LIB$_WRONUMARG", r0)
	}
}

// LIB$FIND_VM_ZONE's first call verifies the zones: a zone whose control
// block the program changed is LIB$_BADZONE, nothing written.
func TestLibFindVMZoneDamaged(t *testing.T) {
	e := pageFixture(t)
	id := mustCreateZone(t, e, 1)

	putLongword(t, e, id, 0)
	putLongword(t, e, zoneScanCell, 0)
	putLongword(t, e, zoneFoundCell, 0xDEADBEEF)

	if r0 := call(t, e, "LIB$FIND_VM_ZONE", zoneScanCell, zoneFoundCell); r0 != libBadZone || longword(t, e, zoneFoundCell) != 0xDEADBEEF {
		t.Errorf("LIB$FIND_VM_ZONE of a damaged database = %#x, %#x; want LIB$_BADZONE, nothing written", r0, longword(t, e, zoneFoundCell))
	}

	if r0 := verifyZone(t, e, id); r0 != libBadZone {
		t.Errorf("LIB$VERIFY_VM_ZONE of the zone = %#x, want LIB$_BADZONE", r0)
	}

	putLongword(t, e, id, id)

	if r0 := verifyZone(t, e, id); r0 != ssNormal {
		t.Errorf("LIB$VERIFY_VM_ZONE with its mark put back = %#x", r0)
	}
}

// LIB$VERIFY_VM_ZONE: sound zones (the default zone as 0, a user zone,
// zones with a free fill through frees that merge, tags, and alignment
// padding) are SS$_NORMAL; a freed block the program wrote in, on a free
// list or a lookaside list, is LIB$_BADZONE; a bad ID, a null argument,
// and the argument count.
func TestLibVerifyVMZone(t *testing.T) {
	e := userFixture(t)

	if r0 := verifyZone(t, e, 0); r0 != ssNormal {
		t.Errorf("LIB$VERIFY_VM_ZONE of the default zone = %#x", r0)
	}

	if r0 := verifyZone(t, e, createUserZone(t, e, 7)); r0 != ssNormal {
		t.Errorf("LIB$VERIFY_VM_ZONE of a user zone = %#x", r0)
	}

	for _, c := range []struct {
		name   string
		values []int64
	}{
		{"first fit, FREE_FILL1", []int64{1, omit, 0x10}},
		{"first fit, FREE_FILL0, tags", []int64{1, omit, 0x08 | 0x01}},
		{"tags, block size 8, alignment 16, FREE_FILL1", []int64{1, omit, 0x10 | 0x01, 16, 0, 8, 16}},
		{"quick fit, FREE_FILL1", []int64{2, 4, 0x10}},
	} {
		id := mustCreateZone(t, e, c.values...)

		var blocks []uint32
		for i := range 12 {
			blocks = append(blocks, mustGetVM(t, e, uint32(8+8*(i%5)), id))
		}

		// Free every other block, then the rest, so that freed blocks
		// merge with neighbors on both sides.
		for _, step := range []int{0, 1} {
			for i := step; i < len(blocks); i += 2 {
				if r0 := freeVM(t, e, 0, blocks[i], id); r0 != ssNormal {
					t.Fatalf("%s: LIB$FREE_VM = %#x", c.name, r0)
				}

				if r0 := verifyZone(t, e, id); r0 != ssNormal {
					t.Fatalf("%s: LIB$VERIFY_VM_ZONE after freeing block %d = %#x", c.name, i, r0)
				}
			}
		}

		// Allocate again (from the freed space), free one, and write in it.
		x := mustGetVM(t, e, 8, id)
		mustGetVM(t, e, 8, id)

		if r0 := freeVM(t, e, 0, x, id); r0 != ssNormal {
			t.Fatalf("%s: LIB$FREE_VM = %#x", c.name, r0)
		}

		if r0 := verifyZone(t, e, id); r0 != ssNormal {
			t.Errorf("%s: LIB$VERIFY_VM_ZONE = %#x", c.name, r0)
		}

		putLongword(t, e, x, 0x12345678)

		if r0 := verifyZone(t, e, id); r0 != libBadZone {
			t.Errorf("%s: LIB$VERIFY_VM_ZONE with a freed block written in = %#x, want LIB$_BADZONE", c.name, r0)
		}
	}

	if r0 := verifyZone(t, e, 0x12340); r0 != libBadZone {
		t.Errorf("LIB$VERIFY_VM_ZONE of no zone = %#x, want LIB$_BADZONE", r0)
	}

	if r0 := call(t, e, "LIB$VERIFY_VM_ZONE", 0); r0 != libInvArg {
		t.Errorf("LIB$VERIFY_VM_ZONE of a null argument = %#x, want LIB$_INVARG", r0)
	}

	for _, argv := range [][]uint32{{}, {showZone, showZone}} {
		if r0 := call(t, e, "LIB$VERIFY_VM_ZONE", argv...); r0 != libWroNumArg {
			t.Errorf("LIB$VERIFY_VM_ZONE with %d arguments = %#x, want LIB$_WRONUMARG", len(argv), r0)
		}
	}
}

// LIB$SHOW_VM_ZONE of the default zone at levels 0 and 1, as the
// manual's sample lays level 1 out, before and after its first block.
func TestLibShowVMZoneDefault(t *testing.T) {
	e, out := statFixture(t)
	id := e.DefaultZoneID()

	r0, lines := showZoneLines(t, e, out, 0, 0)
	if r0 != ssNormal {
		t.Errorf("LIB$SHOW_VM_ZONE = %#x", r0)
	}

	head := fmt.Sprintf(`Zone Id = %08X,  Zone name = "DEFAULT_ZONE"`, id)
	wantLines(t, "level 0", lines, head)

	_, lines = showZoneLines(t, e, out, id, 1)
	wantLines(t, "level 1", lines,
		head,
		"    Algorithm = LIB$K_VM_FIRST_FIT",
		"    Flags = 00000020",
		"                 LIB$M_VM_EXTEND_AREA",
		"    Initial size =       0 pages     Current size = 0 pages in 0 areas",
		"    Extend size  =     128 pages     Page limit   = None",
		"    Requests are rounded up to a multiple of 8 bytes,",
		"    naturally aligned on 8 byte boundaries",
		"    0 bytes have been freed and not yet reallocated",
		"    128 bytes are used for zone and area control blocks, or 100.0% overhead")

	a := mustGetVM(t, e, 100, 0)
	mustGetVM(t, e, 20, 0)

	if r0 := freeVM(t, e, 0, a, 0); r0 != ssNormal {
		t.Fatalf("LIB$FREE_VM = %#x", r0)
	}

	_, lines = showZoneLines(t, e, out, 0, 3)
	wantLines(t, "level 3", lines[4:],
		"    Initial size =       0 pages     Current size = 128 pages in 1 area",
		"    Extend size  =     128 pages     Page limit   = None",
		"    Requests are rounded up to a multiple of 8 bytes,",
		"    naturally aligned on 8 byte boundaries",
		"    104 bytes have been freed and not yet reallocated",
		"    192 bytes are used for zone and area control blocks, or 0.3% overhead",
		"    Area Summary:",
		"          First     Last       Pages    Bytes not yet",
		"         address   address    assigned    allocated",
		"        --------  --------  ----------  -------------",
		fmt.Sprintf("        %08X  %08X         128          65344", a-64, a-64+128*512-1),
		fmt.Sprintf("    Scanning Free List for Area at %08X", a-64),
		"      Number of blocks = 1, Min blocksize = 104, Max blocksize = 104")
}

// LIB$SHOW_VM_ZONE of a quick fit zone with a free fill at levels 2 and
// 3 (the manual's second sample's form), and a freed block written in.
func TestLibShowVMZoneQuickFit(t *testing.T) {
	e, out := statFixture(t)

	putDescriptor(t, e, vmNameDesc, vmNameText, "Mix")

	argv := []uint32{vmZoneCell}
	for i, v := range []uint32{2, 4, 0x08 | 0x20, 16, 16} { // QUICK_FIT, 4 lists, FREE_FILL0 and EXTEND_AREA, 16 pages
		putLongword(t, e, vmArgCells+uint32(4*i), v)
		argv = append(argv, vmArgCells+uint32(4*i))
	}

	argv = append(argv, 0, 0, 0, 0, vmNameDesc)
	if r0 := call(t, e, "LIB$CREATE_VM_ZONE", argv...); r0 != ssNormal {
		t.Fatalf("LIB$CREATE_VM_ZONE = %#x", r0)
	}

	id := longword(t, e, vmZoneCell)

	var blocks []uint32
	for _, size := range []uint32{16, 16, 24, 200, 300, 8} {
		blocks = append(blocks, mustGetVM(t, e, size, id))
	}

	for _, i := range []int{0, 1, 2, 3} {
		if r0 := freeVM(t, e, 0, blocks[i], id); r0 != ssNormal {
			t.Fatalf("LIB$FREE_VM = %#x", r0)
		}
	}

	area := blocks[0] - 64

	_, lines := showZoneLines(t, e, out, id, 3)
	wantLines(t, "level 3", lines,
		fmt.Sprintf(`Zone Id = %08X,  Zone name = "Mix"`, id),
		"    Algorithm = LIB$K_VM_QUICK_FIT  with 4 Lookaside Lists ranging from",
		"                a minimum blocksize of 8, to a maximum blocksize of 32",
		"    Flags = 00000028",
		"                 LIB$M_VM_FREE_FILL0",
		"                 LIB$M_VM_EXTEND_AREA",
		"    Initial size =      16 pages     Current size = 16 pages in 1 area",
		"    Extend size  =      16 pages     Page limit   = None",
		"    Requests are rounded up to a multiple of 8 bytes,",
		"    naturally aligned on 8 byte boundaries",
		"    256 bytes have been freed and not yet reallocated",
		"    192 bytes are used for zone and area control blocks, or 2.3% overhead",
		"    Quick Fit Lookaside List Summary:",
		"         List        Block   Number of",
		"        number        size     blocks",
		"        ------  ----------  ----------",
		"             2          16           2",
		"             3          24           1",
		"    Area Summary:",
		"          First     Last       Pages    Bytes not yet",
		"         address   address    assigned    allocated",
		"        --------  --------  ----------  -------------",
		fmt.Sprintf("        %08X  %08X          16           7560", area, area+16*512-1),
		"    Scanning Lookaside Lists in Zone Control Block",
		fmt.Sprintf("    Scanning Free List for Area at %08X", area),
		"      Number of blocks = 1, Min blocksize = 200, Max blocksize = 200")

	// A block on a lookaside list written in shows as an error with its
	// contents; the zone fails verification.
	putLongword(t, e, blocks[2]+4, 0x6B6E754A) // "Junk"

	_, lines = showZoneLines(t, e, out, id, 3)
	wantLines(t, "damaged", lines[23:],
		"    Scanning Lookaside Lists in Zone Control Block",
		"    **** ERROR -- free-fill mismatch ****",
		"    Block size = 24",
		"    Block contents:",
		fmt.Sprintf("      00000000 00000000 6B6E754A 00000000  ....Junk........ 00000 %08X", blocks[2]),
		fmt.Sprintf("                        00000000 00000000  ........         00010 %08X", blocks[2]+16),
		fmt.Sprintf("    Scanning Free List for Area at %08X", area),
		"      Number of blocks = 1, Min blocksize = 200, Max blocksize = 200")

	if r0 := verifyZone(t, e, id); r0 != libBadZone {
		t.Errorf("LIB$VERIFY_VM_ZONE = %#x, want LIB$_BADZONE", r0)
	}

	// Level 2 stops after the area summary.
	_, lines = showZoneLines(t, e, out, id, 2)
	if len(lines) != 23 {
		t.Errorf("level 2: %d lines, want 23:\n%s", len(lines), strings.Join(lines, "\n"))
	}
}

// LIB$SHOW_VM_ZONE's other algorithms' lines, a user zone (its first line
// only), and its failures: a bad ID, a bad detail level, the argument
// count.
func TestLibShowVMZoneOthers(t *testing.T) {
	e, out := statFixture(t)

	fixed := mustCreateZone(t, e, 4, 64, 0x01|0x40, 16, 4, 8, 4, 8) // FIXED 64, tags, NO_EXTEND, 4 pages, limit 8
	_, lines := showZoneLines(t, e, out, fixed, 1)
	wantLines(t, "fixed", lines[1:],
		"    Algorithm = LIB$K_VM_FIXED  with a blocksize of 64",
		"    Flags = 00000041",
		"                 LIB$M_VM_BOUNDARY_TAGS",
		"                 LIB$M_VM_NO_EXTEND",
		"    Initial size =       4 pages     Current size = 4 pages in 1 area",
		"    Extend size  =       0 pages     Page limit   = 8 pages",
		"    Requests are rounded up to a multiple of 8 bytes,",
		"    aligned on 4 byte boundaries",
		"    0 bytes have been freed and not yet reallocated",
		"    192 bytes are used for zone and area control blocks, or 8.6% overhead")

	freq := mustCreateZone(t, e, 3, 2)
	_, lines = showZoneLines(t, e, out, freq, 1)

	if lines[1] != "    Algorithm = LIB$K_VM_FREQ_SIZES  with 2 Lookaside Lists" {
		t.Errorf("frequent sizes: %q", lines[1])
	}

	u := createUserZone(t, e, 7)
	_, lines = showZoneLines(t, e, out, u, 3)
	wantLines(t, "user zone", lines, fmt.Sprintf(`Zone Id = %08X,  Zone name = ""`, u))

	if r0, lines := showZoneLines(t, e, out, 0x12340, 0); r0 != libBadZone || lines != nil {
		t.Errorf("LIB$SHOW_VM_ZONE of no zone = %#x, %q; want LIB$_BADZONE, nothing", r0, lines)
	}

	for _, detail := range []uint32{4, 0xFFFFFFFF} {
		if r0, lines := showZoneLines(t, e, out, 0, detail); r0 != libInvArg || lines != nil {
			t.Errorf("LIB$SHOW_VM_ZONE at level %d = %#x, %q; want LIB$_INVARG, nothing", int32(detail), r0, lines)
		}
	}

	// Omitted zone-id and detail-level: the default zone at level 0.
	before := out.Len()
	if r0 := call(t, e, "LIB$SHOW_VM_ZONE", 0); r0 != ssNormal ||
		out.String()[before:] != fmt.Sprintf("Zone Id = %08X,  Zone name = \"DEFAULT_ZONE\"\n", e.DefaultZoneID()) {
		t.Errorf("LIB$SHOW_VM_ZONE() = %#x, %q", r0, out.String()[before:])
	}

	for _, argv := range [][]uint32{{}, {showZone, 0, 0, 0, 0}} {
		if r0 := call(t, e, "LIB$SHOW_VM_ZONE", argv...); r0 != libWroNumArg {
			t.Errorf("LIB$SHOW_VM_ZONE with %d arguments = %#x, want LIB$_WRONUMARG", len(argv), r0)
		}
	}
}
