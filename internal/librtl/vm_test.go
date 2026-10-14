package librtl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmheap"
)

// Cells the heap tests pass by reference.
const (
	vmSizeCell = 0x1000 // number-of-bytes
	vmAddrCell = 0x1004 // base-address
	vmZoneCell = 0x1008 // zone-id
	vmArgCells = 0x1100 // CREATE_VM_ZONE's arguments 2 to 10
	vmNameDesc = 0x1200 // its zone-name descriptor
	vmNameText = 0x1210
)

// omit, as one of createZone's values, passes that argument as omitted.
const omit = int64(-1) << 40

// createZone calls LIB$CREATE_VM_ZONE with arguments 2 on (algorithm,
// algorithm-argument, flags, extend-size, initial-size, block-size,
// alignment, page-limit, smallest-block-size) as values, each passed by
// reference unless it's omit, and no zone name. It returns the status
// and the zone ID written.
func createZone(t *testing.T, env *corevms.Environment, values ...int64) (uint32, uint32) {
	t.Helper()

	argv := []uint32{vmZoneCell}

	for i, v := range values {
		if v == omit {
			argv = append(argv, 0)

			continue
		}

		putLongword(t, env, vmArgCells+uint32(4*i), uint32(v))
		argv = append(argv, vmArgCells+uint32(4*i))
	}

	putLongword(t, env, vmZoneCell, 0xDEADBEEF)
	r0 := call(t, env, "LIB$CREATE_VM_ZONE", argv...)

	return r0, longword(t, env, vmZoneCell)
}

// mustCreateZone is createZone that fails the test unless it succeeds.
func mustCreateZone(t *testing.T, env *corevms.Environment, values ...int64) uint32 {
	t.Helper()

	r0, id := createZone(t, env, values...)
	if r0 != ssNormal {
		t.Fatalf("LIB$CREATE_VM_ZONE%v = %#x, want SS$_NORMAL", values, r0)
	}

	return id
}

// getVM calls LIB$GET_VM for size bytes in zone (0 passes zone-id as
// omitted) and returns the status and the address stored.
func getVM(t *testing.T, env *corevms.Environment, size, zone uint32) (uint32, uint32) {
	t.Helper()

	putLongword(t, env, vmSizeCell, size)
	putLongword(t, env, vmAddrCell, 0xDEADBEEF)
	putLongword(t, env, vmZoneCell, zone)

	r0 := call(t, env, "LIB$GET_VM", vmSizeCell, vmAddrCell, zoneArg(zone))

	return r0, longword(t, env, vmAddrCell)
}

func mustGetVM(t *testing.T, env *corevms.Environment, size, zone uint32) uint32 {
	t.Helper()

	r0, addr := getVM(t, env, size, zone)
	if r0 != ssNormal {
		t.Fatalf("LIB$GET_VM(%d, zone %X) = %#x, want SS$_NORMAL", size, zone, r0)
	}

	return addr
}

// freeVM calls LIB$FREE_VM for the block at addr in zone, with size
// passed by reference, or omitted when it's 0.
func freeVM(t *testing.T, env *corevms.Environment, size, addr, zone uint32) uint32 {
	t.Helper()

	sizeArg := uint32(0)
	if size != 0 {
		putLongword(t, env, vmSizeCell, size)
		sizeArg = vmSizeCell
	}

	putLongword(t, env, vmAddrCell, addr)
	putLongword(t, env, vmZoneCell, zone)

	return call(t, env, "LIB$FREE_VM", sizeArg, vmAddrCell, zoneArg(zone))
}

// zoneArg is zone-id's address: omitted for the default zone.
func zoneArg(zone uint32) uint32 {
	if zone == 0 {
		return 0
	}

	return vmZoneCell
}

// zoneCall calls LIB$DELETE_VM_ZONE or LIB$RESET_VM_ZONE of zone.
func zoneCall(t *testing.T, env *corevms.Environment, name string, zone uint32) uint32 {
	t.Helper()

	putLongword(t, env, vmZoneCell, zone)

	return call(t, env, name, vmZoneCell)
}

// allBytes reports whether every byte of b is v.
func allBytes(b string, v byte) bool {
	for i := range len(b) {
		if b[i] != v {
			return false
		}
	}

	return true
}

// The default zone: zone ID 0 or none, its first block at its first
// area's start, the size LIB$FREE_VM is given trusted (VMS 7.1's way,
// testdata/lib59), and a second free refused.
func TestLibGetFreeVM(t *testing.T) {
	env := pageFixture(t)

	addr := mustGetVM(t, env, 100, 0)

	// The P0 high-water mark's first page holds the RTL's zones' control
	// blocks; the page pool starts after it, and the default zone's
	// initial area is its first pages, its first block at its start (an
	// area's control block is kept apart).
	if addr != 0x4200 {
		t.Errorf("allocated at %#x, want 0x4200", addr)
	}

	for _, size := range []uint32{0, 0xFFFFFFFF, 0x80000000} {
		if r0, got := getVM(t, env, size, 0); r0 != libBadBlockSize || got != 0xDEADBEEF {
			t.Errorf("LIB$GET_VM(%#x) = %#x, address %#x; want LIB$_BADBLOSIZ, unchanged", size, r0, got)
		}
	}

	if r0 := freeVM(t, env, 0xFFFFFFFF, addr, 0); r0 != libBadBlockSize {
		t.Errorf("LIB$FREE_VM(-1) = %#x, want LIB$_BADBLOSIZ", r0)
	}

	// 96 bytes of the 104-byte block free 96, leaving its last 8 the
	// program's; freed again, they're LIB$_BADBLOADR.
	if r0 := freeVM(t, env, 96, addr, 0); r0 != ssNormal {
		t.Errorf("LIB$FREE_VM(96) = %#x, want SS$_NORMAL", r0)
	}

	if r0 := freeVM(t, env, 96, addr, 0); r0 != libBadBlockAddress {
		t.Errorf("LIB$FREE_VM again = %#x, want LIB$_BADBLOADR", r0)
	}

	if r0 := freeVM(t, env, 8, addr+96, 0); r0 != ssNormal {
		t.Errorf("LIB$FREE_VM of the rest = %#x, want SS$_NORMAL", r0)
	}

	// Without a size, an untagged zone's block's size is read from the
	// longword at 0, as VMS 7.1 reads it (no-access there, so an access
	// violation; this fixture's page 0 holds 0, LIB$_BADBLOSIZ). zone-id
	// holding 0 is the default zone too.
	b := mustGetVM(t, env, 8, 0)
	putLongword(t, env, vmAddrCell, b)
	putLongword(t, env, vmZoneCell, 0)

	if r0 := call(t, env, "LIB$FREE_VM", 0, vmAddrCell, vmZoneCell); r0 != libBadBlockSize && r0 != ssAccVio {
		t.Errorf("LIB$FREE_VM without a size = %#x, want SS$_ACCVIO or LIB$_BADBLOSIZ", r0)
	}
}

// The RTL's own zones (Phase 59, subtask 5): the default zone and the
// string zone, made together at the image's first allocation, each with
// a control block whose address is its ID. Zone 0 is the default zone
// by its ID too; neither can be reset or deleted; dynamic strings are
// the string zone's blocks, not the default zone's.
func TestLibSystemZones(t *testing.T) {
	env := pageFixture(t)

	def, str := env.DefaultZoneID(), env.StringZoneID()
	if def == 0 || str <= def || longword(t, env, def) != def || longword(t, env, str) != str {
		t.Fatalf("default zone %#x, string zone %#x; want two IDs, in order, each its control block's mark", def, str)
	}

	// Their attributes are VMS 7.1's (testdata/lib59).
	for _, c := range []struct {
		id      uint32
		name    string
		extend  uint32
		initial uint32
		tags    bool
	}{{0, "DEFAULT_ZONE", 128, 124, false}, {def, "DEFAULT_ZONE", 128, 124, false}, {str, "STRING_ZONE", 32, 4, true}} {
		s, ok := env.ZoneSettingsOf(c.id)
		if !ok || s.Name != c.name || s.BlockSize != 8 || s.ExtendPages != c.extend || s.InitialPages != c.initial ||
			s.BoundaryTags != c.tags || !s.ExtendArea {
			t.Errorf("zone %#x: %+v, %v; want %s, VMS 7.1's attributes", c.id, s, ok, c.name)
		}
	}

	// The default zone's own ID and 0 name one zone.
	a := mustGetVM(t, env, 40, def)
	if r0 := freeVM(t, env, 40, a, 0); r0 != ssNormal {
		t.Errorf("a block of zone %#x freed in zone 0 = %#x, want SS$_NORMAL", def, r0)
	}

	for _, routine := range []string{"LIB$RESET_VM_ZONE", "LIB$DELETE_VM_ZONE"} {
		for _, id := range []uint32{0, def, str} {
			putLongword(t, env, vmZoneCell, id)

			if r0 := call(t, env, routine, vmZoneCell); r0 != libBadBlockAddress {
				t.Errorf("%s of zone %#x = %#x, want LIB$_BADBLOADR", routine, id, r0)
			}
		}
	}

	// A dynamic string's text is in the string zone.
	putDesc(t, env, dynDesc, 0, dscClassD, 0)
	putLongword(t, env, dynDesc+16, 30)
	call(t, env, "LIB$SGET1_DD", dynDesc+16, dynDesc)

	_, _, text := descriptorAt(t, env, dynDesc)

	if _, ok := env.StringSize(text); !ok {
		t.Fatalf("the dynamic string's text at %#x isn't a string zone block", text)
	}

	if _, ok := env.BlockSize(text); ok {
		t.Errorf("the dynamic string's text at %#x is a default zone block", text)
	}

	if r0 := freeVM(t, env, 30, text, 0); r0 != libBadBlockAddress {
		t.Errorf("the text freed in the default zone = %#x, want LIB$_BADBLOADR", r0)
	}

	if got := call(t, env, "LIB$SFREE1_DD", dynDesc); got != ssNormal {
		t.Errorf("LIB$SFREE1_DD = %#x, want SS$_NORMAL", got)
	}

	// A block from LIB$GET_VM isn't a dynamic string's to free.
	b := mustGetVM(t, env, 16, 0)
	putDesc(t, env, dynDesc, 16, dscClassD, b)

	if got := call(t, env, "LIB$SFREE1_DD", dynDesc); got != strErrFreDyn {
		t.Errorf("LIB$SFREE1_DD of a LIB$GET_VM block = %#x, want STR$_ERRFREDYN", got)
	}
}

// A zone with every default: its ID is its control block's address,
// which holds the ID; blocks are its own, and each zone frees only its
// own blocks.
func TestLibCreateVMZone(t *testing.T) {
	env := pageFixture(t)

	id := mustCreateZone(t, env)

	if id == 0 || longword(t, env, id) != id {
		t.Fatalf("zone ID %#x; its control block holds %#x", id, longword(t, env, id))
	}

	s, ok := env.ZoneSettingsOf(id)
	if !ok || s.Algorithm != vmheap.FirstFit || s.ExtendPages != 16 || s.BlockSize != 8 || s.Alignment != 8 ||
		s.InitialPages != 0 || s.PageLimit != 0 || s.BoundaryTags || s.ExtendArea {
		t.Errorf("settings %+v, %v; want the manual's defaults", s, ok)
	}

	inZone := mustGetVM(t, env, 24, id)
	inDefault := mustGetVM(t, env, 24, 0)

	if r0 := freeVM(t, env, 24, inZone, 0); r0 != libBadBlockAddress {
		t.Errorf("the zone's block freed in the default zone = %#x, want LIB$_BADBLOADR", r0)
	}

	if r0 := freeVM(t, env, 24, inDefault, id); r0 != libBadBlockAddress {
		t.Errorf("the default zone's block freed in the zone = %#x, want LIB$_BADBLOADR", r0)
	}

	if r0 := freeVM(t, env, 24, inZone, id); r0 != ssNormal {
		t.Errorf("LIB$FREE_VM in its zone = %#x", r0)
	}

	// Not a zone's ID.
	for _, bad := range []uint32{id + 8, env.DefaultZoneID() + 4} {
		if r0, _ := getVM(t, env, 8, bad); r0 != libBadBlockAddress {
			t.Errorf("LIB$GET_VM in zone %#x = %#x, want LIB$_BADBLOADR", bad, r0)
		}
	}

	// The name, of any class.
	putDescriptor(t, env, vmNameDesc, vmNameText, "MY_ZONE")

	if r0 := call(t, env, "LIB$CREATE_VM_ZONE", vmZoneCell, 0, 0, 0, 0, 0, 0, 0, 0, 0, vmNameDesc); r0 != ssNormal {
		t.Fatalf("LIB$CREATE_VM_ZONE with a name = %#x", r0)
	}

	if s, _ := env.ZoneSettingsOf(longword(t, env, vmZoneCell)); s.Name != "MY_ZONE" {
		t.Errorf("zone name %q, want MY_ZONE", s.Name)
	}

	putLongword(t, env, vmNameDesc, 0x7F000005) // class 127

	if r0 := call(t, env, "LIB$CREATE_VM_ZONE", vmZoneCell, 0, 0, 0, 0, 0, 0, 0, 0, 0, vmNameDesc); r0 != libInvStrDes {
		t.Errorf("a bad name descriptor = %#x, want LIB$_INVSTRDES", r0)
	}
}

// Each argument out of range is LIB$_INVARG, and no zone is made.
func TestLibCreateVMZoneBadArguments(t *testing.T) {
	const (
		tags   = 0x01
		fill0  = 0x02
		fill1  = 0x04
		free0  = 0x08
		free1  = 0x10
		noExt  = 0x40
		badBit = 0x200
	)

	for _, c := range []struct {
		name   string
		values []int64
	}{
		{"algorithm 0", []int64{0}},
		{"algorithm 5", []int64{5}},
		{"quick fit without lists", []int64{2}},
		{"quick fit, 0 lists", []int64{2, 0}},
		{"quick fit, 129 lists", []int64{2, 129}},
		{"frequent sizes, 17 lists", []int64{3, 17}},
		{"fixed size 0", []int64{4, 0}},
		{"fixed size -8", []int64{4, -8}},
		{"flag bit 9", []int64{1, omit, badBit}},
		{"extend 0", []int64{1, omit, 0, 0}},
		{"initial -1", []int64{1, omit, 0, 16, -1}},
		{"block size 4", []int64{1, omit, 0, 16, 0, 4}},
		{"block size 24", []int64{1, omit, 0, 16, 0, 24}},
		{"block size 1024", []int64{1, omit, 0, 16, 0, 1024}},
		{"alignment 2", []int64{1, omit, 0, 16, 0, 8, 2}},
		{"alignment 12", []int64{1, omit, 0, 16, 0, 8, 12}},
		{"page limit -1", []int64{1, omit, 0, 16, 1, 8, 8, -1}},
		{"page limit 32768", []int64{1, omit, 0, 16, 1, 8, 8, 32768}},
		{"smallest 0", []int64{2, 4, 0, 16, 0, 8, 8, 0, 0}},
	} {
		if r0, id := createZone(t, env(t), c.values...); r0 != libInvArg || id != 0xDEADBEEF {
			t.Errorf("%s: %#x, zone-id %#x; want LIB$_INVARG, unchanged", c.name, r0, id)
		}
	}

	// In range, as VMS 7.1 takes them (testdata/lib59): flag bit 8,
	// both fills of a pair (the fill of 0 wins), an extend size past
	// 1,024, a page limit or NO_EXTEND without an initial size; the
	// algorithms' bounds.
	e := pageFixture(t)
	for _, values := range [][]int64{
		{1, omit, 0x100},
		{1, omit, fill0 | fill1},
		{1, omit, free0 | free1 | tags},
		{1, omit, 0, 1025},
		{1, omit, 0, 16, 0, 8, 8, 10},
		{1, omit, noExt},
		{2, 128},
		{3, 16},
		{4, 1},
		{1, 99, 0, 1024, 1, 512, 512, 32767, omit},
	} {
		mustCreateZone(t, e, values...)
	}

	if s, _ := e.ZoneSettingsOf(mustCreateZone(t, e, 1, omit, fill0|fill1)); s.GetFill != corevms.FillZero {
		t.Errorf("both get fills: %v, want the fill of 0", s.GetFill)
	}

	// An initial size past the page limit is LIB$_PAGLIMEXC, with
	// zone-id written, though no zone is made.
	if r0, id := createZone(t, e, 1, omit, 0, 16, 11, 8, 8, 10); r0 != libPageLimit || id == 0xDEADBEEF {
		t.Errorf("initial past the limit: %#x, zone-id %#x; want LIB$_PAGLIMEXC, written", r0, id)
	} else if _, ok := e.ZoneSettingsOf(id); ok {
		t.Errorf("initial past the limit made zone %#x", id)
	}
}

// env is a fresh page fixture (for tables of cases).
func env(t *testing.T) *corevms.Environment {
	t.Helper()

	return pageFixture(t)
}

// Fills: GET_FILL1 fills a new block with FF, FREE_FILL0 a freed one
// with 0; RESET and DELETE fill the blocks they free.
func TestLibVMFills(t *testing.T) {
	e := pageFixture(t)
	id := mustCreateZone(t, e, 1, omit, 0x04|0x08) // GET_FILL1, FREE_FILL0

	a := mustGetVM(t, e, 20, id)
	if b := bytesAt(t, e, a, 24); !allBytes(b, 0xFF) {
		t.Errorf("a new block holds % X, want FF", b)
	}

	if r0 := freeVM(t, e, 20, a, id); r0 != ssNormal {
		t.Fatalf("LIB$FREE_VM = %#x", r0)
	}

	// Its first 8 bytes are the free space's control words (its size,
	// then a link), as VMS 7.1 keeps them; the rest hold the fill.
	if b := bytesAt(t, e, a+8, 16); !allBytes(b, 0) || longword(t, e, a) != 24 {
		t.Errorf("a freed block holds size %d, then % X; want 24, then 00", longword(t, e, a), b)
	}

	for _, op := range []string{"LIB$RESET_VM_ZONE", "LIB$DELETE_VM_ZONE"} {
		z := mustCreateZone(t, e, 1, omit, 0x02|0x10) // GET_FILL0, FREE_FILL1
		x := mustGetVM(t, e, 13, z)

		if b := bytesAt(t, e, x, 16); !allBytes(b, 0) {
			t.Errorf("a new block holds % X, want 00", b)
		}

		if r0 := zoneCall(t, e, op, z); r0 != ssNormal {
			t.Fatalf("%s = %#x", op, r0)
		}

		if b := bytesAt(t, e, x, 16); !allBytes(b, 0xFF) {
			t.Errorf("%s left the block holding % X, want FF", op, b)
		}
	}
}

// Boundary tags, as VMS 7.1 writes them (testdata/lib59): the first
// holds the block's size with bit 30 and 2 (allocated), the last its
// zone; a block frees without a size; freed again, or with bit 30 of its
// first tag cleared, it's LIB$_BADTAGVAL, the block kept; a changed last
// tag isn't checked.
func TestLibVMBoundaryTags(t *testing.T) {
	e := pageFixture(t)
	id := mustCreateZone(t, e, 1, omit, 0x01, 16, 0, 8, 4) // tags, block size 8, alignment 4

	a := mustGetVM(t, e, 8, id)
	b := mustGetVM(t, e, 8, id)
	c := mustGetVM(t, e, 8, id)

	if b-a != 16 || longword(t, e, a-4) != 0x40000012 || longword(t, e, a+8) != id {
		t.Errorf("blocks %d apart, tags %#x and %#x; want 16 apart, tags 0x40000012 and %#x",
			b-a, longword(t, e, a-4), longword(t, e, a+8), id)
	}

	if r0 := freeVM(t, e, 0, a, id); r0 != ssNormal {
		t.Errorf("LIB$FREE_VM without a size = %#x", r0)
	}

	if r0 := freeVM(t, e, 0, a, id); r0 != libBadTagValue {
		t.Errorf("LIB$FREE_VM again = %#x, want LIB$_BADTAGVAL", r0)
	}

	putLongword(t, e, b+8, 0x12345678)

	if r0 := freeVM(t, e, 0, b, id); r0 != ssNormal {
		t.Errorf("LIB$FREE_VM with a changed last tag = %#x, want SS$_NORMAL", r0)
	}

	putLongword(t, e, c-4, 0x12)

	if r0 := freeVM(t, e, 0, c, id); r0 != libBadTagValue {
		t.Errorf("LIB$FREE_VM with a changed first tag = %#x, want LIB$_BADTAGVAL", r0)
	}

	putLongword(t, e, c-4, 0x40000012)

	if r0 := freeVM(t, e, 8, c, id); r0 != ssNormal {
		t.Errorf("LIB$FREE_VM with the tag put back = %#x", r0)
	}
}

// A fixed-size zone: requests up to its size get its size, larger ones
// are LIB$_BADBLOSIZ; and a lookaside block freed twice is SS$_NORMAL,
// its list then giving out that block from then on, as VMS 7.1's does
// (testdata/lib59), where a first fit block freed twice is
// LIB$_BADBLOADR.
func TestLibVMFixedSizeAndDoubleFrees(t *testing.T) {
	e := pageFixture(t)
	id := mustCreateZone(t, e, 4, 20)

	if r0, _ := getVM(t, e, 21, id); r0 != libBadBlockSize {
		t.Errorf("LIB$GET_VM(21) = %#x, want LIB$_BADBLOSIZ", r0)
	}

	a := mustGetVM(t, e, 3, id)
	b := mustGetVM(t, e, 20, id)

	if b-a != 24 {
		t.Errorf("fixed blocks %d apart, want 24", b-a)
	}

	if r0 := freeVM(t, e, 21, a, id); r0 != libBadBlockSize {
		t.Errorf("LIB$FREE_VM(21) = %#x, want LIB$_BADBLOSIZ", r0)
	}

	for range 2 {
		if r0 := freeVM(t, e, 5, a, id); r0 != ssNormal {
			t.Errorf("LIB$FREE_VM(5) = %#x, want SS$_NORMAL", r0)
		}
	}

	// The double free linked the block to itself.
	for range 2 {
		if x := mustGetVM(t, e, 1, id); x != a {
			t.Errorf("next block %X, want the freed %X", x, a)
		}
	}

	putLongword(t, e, vmZoneCell, id)

	if r0 := call(t, e, "LIB$VERIFY_VM_ZONE", vmZoneCell); r0 != libBadZone {
		t.Errorf("LIB$VERIFY_VM_ZONE of the damaged zone = %#x, want LIB$_BADZONE", r0)
	}

	first := mustCreateZone(t, e)
	c := mustGetVM(t, e, 8, first)
	freeVM(t, e, 8, c, first)

	if r0 := freeVM(t, e, 8, c, first); r0 != libBadBlockAddress {
		t.Errorf("first fit double free = %#x, want LIB$_BADBLOADR", r0)
	}
}

// The page limit is LIB$_PAGLIMEXC, and so is NO_EXTEND past the
// initial area, as VMS 7.1 answers (testdata/lib59).
func TestLibVMPageLimit(t *testing.T) {
	e := pageFixture(t)
	limited := mustCreateZone(t, e, 1, omit, 0, 16, 1, 8, 8, 2)

	if r0, addr := getVM(t, e, 2*512, limited); r0 != libPageLimit || addr != 0xDEADBEEF {
		t.Errorf("past the page limit: %#x, address %#x; want LIB$_PAGLIMEXC, unchanged", r0, addr)
	}

	mustGetVM(t, e, 400, limited)

	fixed := mustCreateZone(t, e, 1, omit, 0x40, omit, 1)
	mustGetVM(t, e, 400, fixed)

	if r0, _ := getVM(t, e, 200, fixed); r0 != libPageLimit {
		t.Errorf("past a NO_EXTEND zone's area: %#x, want LIB$_PAGLIMEXC", r0)
	}
}

// RESET frees every block and keeps the pages; DELETE gives the pages
// back, after which the ID is invalid. Zone 0 can't be either.
func TestLibResetDeleteVMZone(t *testing.T) {
	e := pageFixture(t)

	// The zone's initial area is the pool's two pages after the control
	// zone's first area, four pages from the pool's start.
	id := mustCreateZone(t, e, 1, omit, 0, 16, 2)
	a := mustGetVM(t, e, 100, id)
	mustGetVM(t, e, 200, id)

	if a != 0x4A00 {
		t.Errorf("the zone's first block at %#x, want 0x4A00", a)
	}

	end := e.RegionSize[0]

	if r0 := zoneCall(t, e, "LIB$RESET_VM_ZONE", id); r0 != ssNormal {
		t.Fatalf("LIB$RESET_VM_ZONE = %#x", r0)
	}

	if x := mustGetVM(t, e, 8, id); x != a || e.RegionSize[0] != end {
		t.Errorf("after the reset, block at %#x (want %#x), P0 ends at %#x (want %#x)", x, a, e.RegionSize[0], end)
	}

	if r0 := freeVM(t, e, 200, a+104, id); r0 != libBadBlockAddress {
		t.Errorf("freeing a block from before the reset = %#x, want LIB$_BADBLOADR", r0)
	}

	if r0 := zoneCall(t, e, "LIB$DELETE_VM_ZONE", id); r0 != ssNormal {
		t.Fatalf("LIB$DELETE_VM_ZONE = %#x", r0)
	}

	if longword(t, e, id) != 0 {
		t.Errorf("the deleted zone's control block still holds %#x", longword(t, e, id))
	}

	// The ID is no zone's now.
	if r0, _ := getVM(t, e, 8, id); r0 != libBadBlockAddress {
		t.Errorf("LIB$GET_VM in a deleted zone = %#x, want LIB$_BADBLOADR", r0)
	}

	for _, op := range []string{"LIB$DELETE_VM_ZONE", "LIB$RESET_VM_ZONE"} {
		if r0 := zoneCall(t, e, op, id); r0 != libBadBlockAddress {
			t.Errorf("%s of a deleted zone = %#x, want LIB$_BADBLOADR", op, r0)
		}
	}

	// Its pages are the pool's again.
	if p := mustGetPages(t, e, 2); p != 0x4A00 {
		t.Errorf("2 pages at %#x, want the deleted zone's 0x4A00", p)
	}

	// The default zone can't be reset or deleted, and its blocks
	// survive the attempt.
	block := mustGetVM(t, e, 16, 0)

	for _, op := range []string{"LIB$DELETE_VM_ZONE", "LIB$RESET_VM_ZONE"} {
		if r0 := zoneCall(t, e, op, 0); r0 != libBadBlockAddress {
			t.Errorf("%s of zone 0 = %#x, want LIB$_BADBLOADR", op, r0)
		}
	}

	if r0 := freeVM(t, e, 16, block, 0); r0 != ssNormal {
		t.Errorf("the default zone's block didn't survive: %#x", r0)
	}
}
