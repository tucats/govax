package rms

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/isam"
)

// The tests in this file are an indexed $CREATE's and the services around it
// that the probe's run doesn't reach (Phase 88, subtask 7): its own cases are
// internal/console's TestIdx88CreateOracle, and VMS's files opened are
// TestIndexedOpenAsVMS. The record services' are indexedrecords_test.go.

// Where these tests put their XABs, and a key name.
const (
	xabKey0Addr uint32 = 0x6000
	xabKey1Addr uint32 = 0x6100
	xabAll0Addr uint32 = 0x6200
	xabAll1Addr uint32 = 0x6300
	xabSumAddr  uint32 = 0x6400
	keyNameAddr uint32 = 0x6500
)

// putXABKey lays out a XABKEY at addr of one segment, next in the chain.
func putXABKey(t *testing.T, ctx *Context, addr, next uint32, ref int, dtp byte, pos uint16, siz byte, flg byte) {
	t.Helper()

	for i := range uint32(vmsConst("XAB$C_KEYLEN")) {
		putByte(t, ctx, addr+i, 0)
	}

	putByte(t, ctx, addr+xabCOD, xabKEY)
	putByte(t, ctx, addr+xabBLN, byte(vmsConst("XAB$C_KEYLEN")))
	putLongwordAt(t, ctx, addr+xabNXT, next)
	putByte(t, ctx, addr+xabKeyREF, byte(ref))
	putByte(t, ctx, addr+xabKeyDTP, dtp)
	putByte(t, ctx, addr+xabKeyFLG, flg)
	putWord(t, ctx, addr+xabKeyPOS, pos)
	putByte(t, ctx, addr+xabKeySIZ, siz)
}

// putXABAll lays out a XABALL at addr.
func putXABAll(t *testing.T, ctx *Context, addr, next uint32, aid, bkz byte, alq uint32) {
	t.Helper()

	for i := range uint32(vmsConst("XAB$C_ALLLEN")) {
		putByte(t, ctx, addr+i, 0)
	}

	putByte(t, ctx, addr+xabCOD, xabALL)
	putByte(t, ctx, addr+xabBLN, byte(vmsConst("XAB$C_ALLLEN")))
	putLongwordAt(t, ctx, addr+xabNXT, next)
	putByte(t, ctx, addr+xabAllAID, aid)
	putByte(t, ctx, addr+xabAllBKZ, bkz)
	putLongwordAt(t, ctx, addr+xabALQ, alq)
}

// putXABSum lays out a XABSUM at addr.
func putXABSum(t *testing.T, ctx *Context, addr, next uint32) {
	t.Helper()

	for i := range uint32(vmsConst("XAB$C_SUMLEN")) {
		putByte(t, ctx, addr+i, 0)
	}

	putByte(t, ctx, addr+xabCOD, xabSUM)
	putByte(t, ctx, addr+xabBLN, byte(vmsConst("XAB$C_SUMLEN")))
	putLongwordAt(t, ctx, addr+xabNXT, next)
}

// indexedFAB lays out a FAB at testFabAddr for an indexed file of variable
// records of up to mrs bytes, with the XAB chain at xab.
func indexedFAB(t *testing.T, ctx *Context, spec string, mrs uint16, xab uint32) {
	t.Helper()

	for i := range uint32(vmsConst("FAB$C_BLN")) {
		putByte(t, ctx, testFabAddr+i, 0)
	}

	newFAB(t, ctx, spec)
	putByte(t, ctx, testFabAddr+fabFAC, facPut|facGet|facUpd)
	putByte(t, ctx, testFabAddr+fabORG, orgIdx)
	putByte(t, ctx, testFabAddr+fabRFM, 2) // FAB$C_VAR
	putWord(t, ctx, testFabAddr+fabMRS, mrs)
	putLongwordAt(t, ctx, testFabAddr+fabOffset("XAB"), xab)
}

// run calls an RMS service on testFabAddr, failing on a Go error.
func run(t *testing.T, ctx *Context, service func(*Context, []uint32) (uint32, error), addr uint32) uint32 {
	t.Helper()

	r0, err := service(ctx, []uint32{addr})
	if err != nil {
		t.Fatal(err)
	}

	return r0
}

// twoKeys makes DUA0:TWO.IDX: key 0 a string at 0, key 1 a string at 8 with
// a name, two areas, the alternate's in area 1. It's left open.
func twoKeys(t *testing.T, f *createFixture) {
	t.Helper()

	putXABKey(t, f.ctx, xabKey0Addr, xabKey1Addr, 0, 0, 0, 8, 0)
	putXABKey(t, f.ctx, xabKey1Addr, xabAll0Addr, 1, 0, 8, 6, xabFlagDUP)
	putByte(t, f.ctx, xabKey1Addr+xabKeyDAN, 1)
	putByte(t, f.ctx, xabKey1Addr+xabKeyIAN, 1)
	putByte(t, f.ctx, xabKey1Addr+xabKeyLAN, 1)
	putLongwordAt(t, f.ctx, xabKey1Addr+xabKeyKNM, keyNameAddr)

	for i, c := range []byte(strings.Repeat(" ", keyNameSize)) {
		putByte(t, f.ctx, keyNameAddr+uint32(i), c)
	}

	for i, c := range []byte("BY NAME") {
		putByte(t, f.ctx, keyNameAddr+uint32(i), c)
	}

	putXABAll(t, f.ctx, xabAll0Addr, xabAll1Addr, 0, 2, 6)
	putXABAll(t, f.ctx, xabAll1Addr, 0, 1, 1, 4)
	indexedFAB(t, f.ctx, "DUA0:TWO.IDX", 100, xabKey0Addr)

	if r0 := run(t, f.ctx, SysCreate, testFabAddr); r0 != rmsNormal {
		t.Fatalf("$CREATE: %s", StatusName(r0))
	}
}

// TestIndexedCreateAndOpen makes a file of two keys and two areas, then opens
// it with XABKEYs, XABALLs, and a XABSUM in the chain: each is filled as
// $DISPLAY would, the key name too.
func TestIndexedCreateAndOpen(t *testing.T) {
	f := newCreateFixture(t, true)
	twoKeys(t, f)

	// $CREATE leaves the XABKEYs, as it does the XABALLs.
	if got := readByte(t, f.ctx, xabKey1Addr+xabKeyTKS); got != 0 {
		t.Errorf("$CREATE filled the XABKEY: TKS %d", got)
	}

	run(t, f.ctx, SysClose, testFabAddr)

	putXABSum(t, f.ctx, xabSumAddr, xabKey1Addr)
	putXABKey(t, f.ctx, xabKey1Addr, xabAll1Addr, 1, 0, 0, 0, 0)
	putLongwordAt(t, f.ctx, xabKey1Addr+xabKeyKNM, keyNameAddr)
	putXABAll(t, f.ctx, xabAll1Addr, 0, 1, 0, 0)

	for i := range uint32(keyNameSize) {
		putByte(t, f.ctx, keyNameAddr+i, 0)
	}

	indexedFAB(t, f.ctx, "DUA0:TWO.IDX", 0, xabSumAddr)
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)

	if r0 := run(t, f.ctx, SysOpen, testFabAddr); r0 != rmsNormal {
		t.Fatalf("$OPEN: %s", StatusName(r0))
	}

	for _, c := range []struct {
		what      string
		addr      uint32
		got, want uint32
	}{
		{"NOK", xabSumAddr + xabNOK, uint32(readByte(t, f.ctx, xabSumAddr+xabNOK)), 2},
		{"NOA", xabSumAddr + xabNOA, uint32(readByte(t, f.ctx, xabSumAddr+xabNOA)), 2},
		{"PVN", xabSumAddr + xabPVN, uint32(readWord(t, f.ctx, xabSumAddr+xabPVN)), 3},
		{"TKS", 0, uint32(readByte(t, f.ctx, xabKey1Addr+xabKeyTKS)), 6},
		{"POS", 0, uint32(readWord(t, f.ctx, xabKey1Addr+xabKeyPOS)), 8},
		{"MRL", 0, uint32(readWord(t, f.ctx, xabKey1Addr+xabKeyMRL)), 14},
		{"DAN", 0, uint32(readByte(t, f.ctx, xabKey1Addr+xabKeyDAN)), 1},
		{"DBS", 0, uint32(readByte(t, f.ctx, xabKey1Addr+xabKeyDBS)), 1},
		// DUP, no compression of the alternate's records, and no index yet.
		{"FLG", 0, uint32(readByte(t, f.ctx, xabKey1Addr+xabKeyFLG)), 0x91},
		{"PROLOG", 0, uint32(readByte(t, f.ctx, xabKey1Addr+xabKeyPROLOG)), 3},
		{"BKZ", 0, uint32(readByte(t, f.ctx, xabAll1Addr+xabAllBKZ)), 1},
		{"area 1's ALQ", 0, readLongword(t, f.ctx, xabAll1Addr+xabALQ), 4},
		{"FAB's BKS", 0, uint32(readByte(t, f.ctx, testFabAddr+fabOffset("BKS"))), 2},
	} {
		if c.got != c.want {
			t.Errorf("%s = %#x, want %#x", c.what, c.got, c.want)
		}
	}

	name, _ := f.ctx.loadFixedString(keyNameAddr, keyNameSize)
	if name != "BY NAME"+strings.Repeat(" ", keyNameSize-7) {
		t.Errorf("KNM = %q", name)
	}

	// A key past the file's: RMS$_REF, STV the key.
	putByte(t, f.ctx, xabKey1Addr+xabKeyREF, 5)
	putLongwordAt(t, f.ctx, testFabAddr+fabOffset("XAB"), xabKey1Addr)
	putLongwordAt(t, f.ctx, xabKey1Addr+xabNXT, 0)

	if r0 := run(t, f.ctx, SysDisplay, testFabAddr); r0 != rmsKeyRef || readLongword(t, f.ctx, testFabAddr+fabSTV) != 5 {
		t.Errorf("$DISPLAY of key 5: %s, STV %d", StatusName(r0), readLongword(t, f.ctx, testFabAddr+fabSTV))
	}

	// An area past the file's: RMS$_AID, STV the area.
	putByte(t, f.ctx, xabAll1Addr+xabAllAID, 2)
	putLongwordAt(t, f.ctx, testFabAddr+fabOffset("XAB"), xabAll1Addr)

	if r0 := run(t, f.ctx, SysDisplay, testFabAddr); r0 != rmsAreaID || readLongword(t, f.ctx, testFabAddr+fabSTV) != 2 {
		t.Errorf("$DISPLAY of area 2: %s", StatusName(r0))
	}

	// FAB$L_XAB is left at the XABALL: $CLOSE has nothing it reads there.
	run(t, f.ctx, SysClose, testFabAddr)

	attrs, err := NewSession(f.ctx.Mounts).FileAttributes("DUA0:TWO.IDX")
	if err != nil {
		t.Fatal(err)
	}

	if attrs.Keys != 2 || attrs.Areas != 2 || attrs.Prolog != 3 || attrs.Organization != 2 {
		t.Errorf("FileAttributes: keys %d, areas %d, prolog %d, organization %d", attrs.Keys, attrs.Areas, attrs.Prolog, attrs.Organization)
	}
}

// TestIndexedCreateRefusals has the refusals the probe has no case of.
func TestIndexedCreateRefusals(t *testing.T) {
	for _, c := range []struct {
		name  string
		setup func(t *testing.T, ctx *Context)
		mrs   uint16
		sts   uint32
		stv   uint32
	}{
		{"an alternate past MRS", func(t *testing.T, ctx *Context) {
			putXABKey(t, ctx, xabKey0Addr, xabKey1Addr, 0, 0, 0, 8, 0)
			putXABKey(t, ctx, xabKey1Addr, 0, 1, 0, 96, 8, 0)
		}, 100, rmsKeyPosition, 1},
		{"a key the index bucket can't hold twice", func(t *testing.T, ctx *Context) {
			putXABKey(t, ctx, xabKey0Addr, 0, 0, 0, 0, 255, 0)
			putByte(t, ctx, testFabAddr+fabOffset("BKS"), 1)
		}, 300, rmsKeyRoom, 0},
		{"a collating key (Decision 7)", func(t *testing.T, ctx *Context) {
			putXABKey(t, ctx, xabKey0Addr, 0, 0, byte(vmsConst("XAB$C_COL")), 0, 8, 0)
		}, 100, rmsKeyType, 0},
		{"overlapping segments in an alternate are taken", func(t *testing.T, ctx *Context) {
			putXABKey(t, ctx, xabKey0Addr, xabKey1Addr, 0, 0, 0, 8, 0)
			putXABKey(t, ctx, xabKey1Addr, 0, 1, 0, 0, 4, 0)
			putWord(t, ctx, xabKey1Addr+xabKeyPOS+2, 2)
			putByte(t, ctx, xabKey1Addr+xabKeySIZ+1, 4)
		}, 100, rmsNormal, 0},
		{"RFM STM", func(t *testing.T, ctx *Context) {
			putXABKey(t, ctx, xabKey0Addr, 0, 0, 0, 0, 8, 0)
			putByte(t, ctx, testFabAddr+fabRFM, 4)
		}, 100, rmsInvalidRFM, 0},
	} {
		f := newCreateFixture(t, true)
		indexedFAB(t, f.ctx, "DUA0:E.IDX", c.mrs, xabKey0Addr)
		c.setup(t, f.ctx)

		r0 := run(t, f.ctx, SysCreate, testFabAddr)
		if stv := readLongword(t, f.ctx, testFabAddr+fabSTV); r0 != c.sts || stv != c.stv {
			t.Errorf("%s: %s, STV %d; want %s, STV %d", c.name, StatusName(r0), stv, StatusName(c.sts), c.stv)
		}
	}
}

// TestIndexedBadKeyName is VMS's E85: a key name RMS can't read is RMS$_KNM,
// found as the prolog is written, so the file stays with an empty prolog,
// which $OPEN then calls RMS$_PLG.
func TestIndexedBadKeyName(t *testing.T) {
	f := newCreateFixture(t, true)
	putXABKey(t, f.ctx, xabKey0Addr, 0, 0, 0, 0, 8, 0)
	putLongwordAt(t, f.ctx, xabKey0Addr+xabKeyKNM, 0xFFFFFFF0)
	indexedFAB(t, f.ctx, "DUA0:E85.IDX", 100, xabKey0Addr)

	if r0 := run(t, f.ctx, SysCreate, testFabAddr); r0 != rmsKeyName {
		t.Fatalf("$CREATE: %s, want RMS$_KNM", StatusName(r0))
	}

	indexedFAB(t, f.ctx, "DUA0:E85.IDX", 0, 0)
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)

	if r0 := run(t, f.ctx, SysOpen, testFabAddr); r0 != rmsPrologError {
		t.Errorf("$OPEN: %s, want RMS$_PLG", StatusName(r0))
	}
}

// TestExtend is $EXTEND: an indexed file's area 0 by the FAB's ALQ, an area by
// a XABALL, the statuses, and a sequential file.
func TestExtend(t *testing.T) {
	f := newCreateFixture(t, true)
	twoKeys(t, f)

	h, _ := f.ctx.Files.Lookup(readWord(t, f.ctx, testFabAddr+fabIFI))
	before := h.File.Blocks()

	// Area 0 by the FAB: ALQ is what was added, STV too.
	putLongwordAt(t, f.ctx, testFabAddr+fabOffset("XAB"), 0)
	putLongwordAt(t, f.ctx, testFabAddr+fabOffset("ALQ"), 5)

	if r0 := run(t, f.ctx, SysExtend, testFabAddr); r0 != rmsNormal {
		t.Fatalf("$EXTEND: %s", StatusName(r0))
	}

	if got := readLongword(t, f.ctx, testFabAddr+fabOffset("ALQ")); got != 5 || h.File.Blocks() != before+5 {
		t.Errorf("ALQ %d, blocks %d, want 5 more than %d", got, h.File.Blocks(), before)
	}

	// Area 1 by a XABALL.
	putXABAll(t, f.ctx, xabAll1Addr, 0, 1, 0, 3)
	putLongwordAt(t, f.ctx, testFabAddr+fabOffset("XAB"), xabAll1Addr)

	if r0 := run(t, f.ctx, SysExtend, testFabAddr); r0 != rmsNormal || readLongword(t, f.ctx, testFabAddr+fabSTV) != 3 {
		t.Fatalf("$EXTEND of area 1: %s", StatusName(r0))
	}

	file, err := openIndexed(h.File, false)
	if err != nil {
		t.Fatal(err)
	}

	a0, a1 := file.Areas[0], file.Areas[1]
	if a0.NextExtentSize != 5 || a0.TotalAllocation != 11 || a1.NextExtentSize != 3 || a1.TotalAllocation != 7 {
		t.Errorf("areas after $EXTEND: %+v, %+v", a0, a1)
	}

	if h.File.Header.RecordAttributes.EndOfFileBlock != h.File.Blocks()+1 {
		t.Errorf("end of file %d over %d blocks", h.File.Header.RecordAttributes.EndOfFileBlock, h.File.Blocks())
	}

	for _, c := range []struct {
		what string
		set  func()
		sts  uint32
	}{
		{"an area past the file's", func() { putByte(t, f.ctx, xabAll1Addr+xabAllAID, 7) }, rmsAreaID},
		{"an ALQ of 0", func() { putByte(t, f.ctx, xabAll1Addr+xabAllAID, 1); putLongwordAt(t, f.ctx, xabAll1Addr+xabALQ, 0) }, rmsInvalidALQ},
	} {
		c.set()

		if r0 := run(t, f.ctx, SysExtend, testFabAddr); r0 != c.sts {
			t.Errorf("%s: %s, want %s", c.what, StatusName(r0), StatusName(c.sts))
		}
	}

	run(t, f.ctx, SysClose, testFabAddr)

	if r0 := run(t, f.ctx, SysExtend, testFabAddr); r0 != rmsInvalidIFI {
		t.Errorf("a closed file: %s", StatusName(r0))
	}

	// A sequential file grows; one opened only to read can't.
	newFAB(t, f.ctx, "DUA0:SEQ.DAT")
	putLongwordAt(t, f.ctx, testFabAddr+fabOffset("XAB"), 0)

	if r0 := run(t, f.ctx, SysCreate, testFabAddr); r0 != rmsNormal {
		t.Fatalf("$CREATE: %s", StatusName(r0))
	}

	putLongwordAt(t, f.ctx, testFabAddr+fabOffset("ALQ"), 4)

	if r0 := run(t, f.ctx, SysExtend, testFabAddr); r0 != rmsNormal || readLongword(t, f.ctx, testFabAddr+fabOffset("ALQ")) != 4 {
		t.Errorf("$EXTEND of a sequential file: %s", StatusName(r0))
	}

	run(t, f.ctx, SysClose, testFabAddr)
	newFAB(t, f.ctx, "DUA0:SEQ.DAT")
	putByte(t, f.ctx, testFabAddr+fabFAC, facGet)
	run(t, f.ctx, SysOpen, testFabAddr)

	if r0 := run(t, f.ctx, SysExtend, testFabAddr); r0 != rmsFACNotAllowed {
		t.Errorf("$EXTEND of a file open to read: %s", StatusName(r0))
	}
}

// TestTraceIndexed is SET DEBUG RMS's line for an indexed $CREATE: the XABKEY's
// and XABALL's fields.
func TestTraceIndexed(t *testing.T) {
	f := newCreateFixture(t, true)
	twoKeys(t, f)

	line := Trace(f.ctx, "SYS$CREATE", testFabAddr, rmsNormal)

	for _, want := range []string{"ORG 20", "[KEY REF 0 DTP 00 FLG 00 0:8 KEY REF 1 DTP 00 FLG 01 8:6 ALL AID 0 BKZ 2 ALQ 6 DEQ 0 ALL AID 1 BKZ 1 ALQ 4 DEQ 0]"} {
		if !strings.Contains(line, want) {
			t.Errorf("no %q in %s", want, line)
		}
	}
}

// An isam.Type is a byte of XAB$B_DTP: the two must agree.
var _ = isam.Type(vmsConst("XAB$C_PAC"))
