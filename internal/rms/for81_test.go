package rms

import (
	"bytes"
	"testing"
)

// Tests of what Phase 81's groundwork added to RMS: keyed $PUT, block I/O,
// $DISCONNECT, FOP's bits, and $PARSE with no NAM. TestFor81RMSOracle
// (internal/console) holds most of the same to VMS 7.1's run; these also cover
// what that program couldn't reach on VMS, block I/O above all.

const (
	testBlockAddr = 0x8000
)

// newFileFAB lays out a FAB for name: FIX records of mrs bytes, access fac,
// options fop.
func newFileFAB(t *testing.T, ctx *Context, name string, fac byte, mrs uint16, fop uint32) {
	t.Helper()

	newFAB(t, ctx, name)
	putByte(t, ctx, testFabAddr+fabFAC, fac)
	putWord(t, ctx, testFabAddr+fabMRS, mrs)
	putLongwordAt(t, ctx, testFabAddr+fabFOP, fop)
}

// service runs one service on block, failing the test on a Go error.
func service(t *testing.T, ctx *Context, fn func(*Context, []uint32) (uint32, error), block uint32) uint32 {
	t.Helper()

	r0, err := fn(ctx, []uint32{block})
	if err != nil {
		t.Fatal(err)
	}

	return r0
}

// wantStatus fails the test unless a service's status is want.
func wantSts(t *testing.T, what string, got, want uint32) {
	t.Helper()

	if got != want {
		t.Errorf("%s: %s, want %s", what, StatusName(got), StatusName(want))
	}
}

// putKeyedRecord is a keyed $PUT of record number with RAB$L_ROP rop.
func putKeyedRecord(t *testing.T, ctx *Context, number uint32, record []byte, rop uint32) uint32 {
	t.Helper()

	putByte(t, ctx, testRabAddr+rabRAC, racKey)
	putLongwordAt(t, ctx, testRabAddr+rabKBF, testKeyAddr)
	putLongwordAt(t, ctx, testKeyAddr, number)
	putLongwordAt(t, ctx, testRabAddr+rabROP, rop)
	putRecord(t, ctx, testRabAddr, record)

	return service(t, ctx, SysPut, testRabAddr)
}

// getRecord is a $GET into a buffer, by RAB$B_RAC rac, returning the record.
func getRecord(t *testing.T, ctx *Context, rac byte, number uint32) ([]byte, uint32) {
	t.Helper()

	putByte(t, ctx, testRabAddr+rabRAC, rac)
	putLongwordAt(t, ctx, testKeyAddr, number)
	putLongwordAt(t, ctx, testRabAddr+rabUBF, testBlockAddr)
	putWord(t, ctx, testRabAddr+rabUSZ, 2048)

	r0 := service(t, ctx, SysGet, testRabAddr)
	record := make([]byte, readWord(t, ctx, testRabAddr+rabRSZ))

	for i := range record {
		record[i] = readByte(t, ctx, testBlockAddr+uint32(i))
	}

	return record, r0
}

func TestPutKeyed(t *testing.T) {
	f := newCreateFixture(t, true)
	newFileFAB(t, f.ctx, "DUA0:K.DAT", facGet|facPut, 16, 0)
	wantSts(t, "$CREATE", service(t, f.ctx, SysCreate, testFabAddr), rmsNormal)
	connectRAB(t, f.ctx, testFabAddr)

	a := bytes.Repeat([]byte{'A'}, 16)
	b := bytes.Repeat([]byte{'B'}, 16)

	wantSts(t, "record 1", putKeyedRecord(t, f.ctx, 1, a, 0), rmsNormal)
	wantSts(t, "record 1 again", putKeyedRecord(t, f.ctx, 1, a, 0), rmsNotAtEOF)
	wantSts(t, "record 1 with UIF", putKeyedRecord(t, f.ctx, 1, b, ropUIF), rmsNormal)
	wantSts(t, "record 4", putKeyedRecord(t, f.ctx, 4, a, 0), rmsNormal)
	wantSts(t, "record 0", putKeyedRecord(t, f.ctx, 0, a, ropUIF), rmsInvalidKey)
	wantSts(t, "a short one", putKeyedRecord(t, f.ctx, 3, a[:8], ropUIF), rmsRecordTooBig)

	cases := []struct {
		number uint32
		want   []byte
		sts    uint32
	}{
		{1, b, rmsNormal},
		{2, make([]byte, 16), rmsNormal},
		{3, make([]byte, 16), rmsNormal},
		{4, a, rmsNormal},
		{5, []byte{}, rmsEOF},
		{0, []byte{}, rmsInvalidKey},
	}

	for _, c := range cases {
		record, sts := getRecord(t, f.ctx, racKey, c.number)
		wantSts(t, "$GET", sts, c.sts)

		if !bytes.Equal(record, c.want) {
			t.Errorf("record %d = %q, want %q", c.number, record, c.want)
		}
	}
}

// TestPutKeyedThenSequential: a stream at the end of the file stays where the
// end was, so the next sequential $PUT goes there and ends the file (VMS 7.1's
// case "FIX 16, keyed then sequential").
func TestPutKeyedThenSequential(t *testing.T) {
	f := newCreateFixture(t, true)
	newFileFAB(t, f.ctx, "DUA0:K.DAT", facGet|facPut, 16, 0)
	service(t, f.ctx, SysCreate, testFabAddr)
	connectRAB(t, f.ctx, testFabAddr)

	a := bytes.Repeat([]byte{'A'}, 16)

	wantSts(t, "record 4", putKeyedRecord(t, f.ctx, 4, a, ropUIF), rmsNormal)

	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)
	putLongwordAt(t, f.ctx, testRabAddr+rabROP, 0)
	putRecord(t, f.ctx, testRabAddr, a)
	wantSts(t, "sequential", service(t, f.ctx, SysPut, testRabAddr), rmsNormal)

	if r := readWord(t, f.ctx, testRabAddr+rabRFA+4); r != 0 {
		t.Errorf("RFA's offset %d, want 0", r)
	}

	if _, sts := getRecord(t, f.ctx, racKey, 2); sts != rmsEOF {
		t.Errorf("record 2: %s, want RMS$_EOF", StatusName(sts))
	}
}

// TestPutKeyedVariable: a keyed $PUT needs fixed-length records.
func TestPutKeyedVariable(t *testing.T) {
	f := newCreateFixture(t, true)
	newFileFAB(t, f.ctx, "DUA0:V.DAT", facPut, 0, 0)
	putByte(t, f.ctx, testFabAddr+fabRFM, 2)
	service(t, f.ctx, SysCreate, testFabAddr)
	connectRAB(t, f.ctx, testFabAddr)

	wantSts(t, "$PUT", putKeyedRecord(t, f.ctx, 1, []byte("ABC"), ropUIF), rmsInvalidRAC)
}

func TestBlockIO(t *testing.T) {
	f := newCreateFixture(t, true)
	newFileFAB(t, f.ctx, "DUA0:B.DAT", facBIO|facPut, 512, 0)
	wantSts(t, "$CREATE", service(t, f.ctx, SysCreate, testFabAddr), rmsNormal)
	connectRAB(t, f.ctx, testFabAddr)

	data := make([]byte, 1000)
	for i := range data {
		data[i] = byte(i%250 + 1)
	}

	// Block 2 first: the file grows through it, and block 1 reads as zeros.
	putLongwordAt(t, f.ctx, testRabAddr+rabBKT, 2)
	putRecord(t, f.ctx, testRabAddr, data)
	wantSts(t, "$WRITE", service(t, f.ctx, SysWrite, testRabAddr), rmsNormal)

	if vbn := readLongword(t, f.ctx, testRabAddr+rabRFA) & 0xFFFF; vbn != 2 {
		t.Errorf("RFA's block %d, want 2", vbn)
	}

	wantSts(t, "$GET on a block stream", service(t, f.ctx, SysGet, testRabAddr), rmsIOP)
	wantSts(t, "$PUT on a block stream", service(t, f.ctx, SysPut, testRabAddr), rmsIOP)
	wantSts(t, "$CLOSE", service(t, f.ctx, SysClose, testFabAddr), rmsNormal)

	putByte(t, f.ctx, testFabAddr+fabFAC, facBIO)
	wantSts(t, "$OPEN", service(t, f.ctx, SysOpen, testFabAddr), rmsNormal)
	connectRAB(t, f.ctx, testFabAddr)

	read := func(vbn uint32, size uint16) ([]byte, uint32) {
		putLongwordAt(t, f.ctx, testRabAddr+rabBKT, vbn)
		putLongwordAt(t, f.ctx, testRabAddr+rabUBF, testBlockAddr)
		putWord(t, f.ctx, testRabAddr+rabUSZ, size)

		sts := service(t, f.ctx, SysRead, testRabAddr)
		out := make([]byte, readWord(t, f.ctx, testRabAddr+rabRSZ))

		for i := range out {
			out[i] = readByte(t, f.ctx, testBlockAddr+uint32(i))
		}

		return out, sts
	}

	got, sts := read(2, 1000)
	wantSts(t, "$READ", sts, rmsNormal)

	if !bytes.Equal(got, data) {
		t.Errorf("block 2 read back wrong: %d bytes", len(got))
	}

	// The next is block 4 (1000 bytes took two), past the end.
	_, sts = read(0, 512)
	wantSts(t, "$READ past the end", sts, rmsEOF)

	got, sts = read(1, 512)
	wantSts(t, "$READ of block 1", sts, rmsNormal)

	if !bytes.Equal(got, make([]byte, 512)) {
		t.Errorf("block 1 isn't zeros")
	}

	// 4096 asked for, and three blocks are there.
	got, sts = read(1, 4096)
	wantSts(t, "$READ of them all", sts, rmsNormal)

	if len(got) != 1536 {
		t.Errorf("%d bytes, want 1536", len(got))
	}

	_, sts = read(1, 0)
	wantSts(t, "$READ with no size", sts, rmsBadUserSize)

	putRecord(t, f.ctx, testRabAddr, data)
	wantSts(t, "$WRITE without write access", service(t, f.ctx, SysWrite, testRabAddr), rmsFACNotAllowed)
}

func TestBlockIOSharing(t *testing.T) {
	f := newCreateFixture(t, true)
	newFileFAB(t, f.ctx, "DUA0:B.DAT", facPut, 16, 0)
	service(t, f.ctx, SysCreate, testFabAddr)
	service(t, f.ctx, SysClose, testFabAddr)

	putByte(t, f.ctx, testFabAddr+fabFAC, facBIO|facGet)
	putByte(t, f.ctx, testFabAddr+fabSHR, shrGet|shrPut)
	wantSts(t, "shared writes", service(t, f.ctx, SysOpen, testFabAddr), rmsNeedsUPI)

	putByte(t, f.ctx, testFabAddr+fabSHR, shrGet|shrPut|shrUPI)
	wantSts(t, "with UPI", service(t, f.ctx, SysOpen, testFabAddr), rmsNormal)
}

// TestBlockOnRecordStream: a stream connected for records has no $READ, but FAB$V_BRO
// lets a $CONNECT choose.
func TestBlockOnRecordStream(t *testing.T) {
	f := newCreateFixture(t, true)
	newFileFAB(t, f.ctx, "DUA0:B.DAT", facPut, 16, 0)
	service(t, f.ctx, SysCreate, testFabAddr)
	connectRAB(t, f.ctx, testFabAddr)

	wantSts(t, "$READ", service(t, f.ctx, SysRead, testRabAddr), rmsIOP)
	service(t, f.ctx, SysClose, testFabAddr)

	putByte(t, f.ctx, testFabAddr+fabFAC, facBRO|facGet)
	service(t, f.ctx, SysOpen, testFabAddr)
	putRAB(t, f.ctx, testFabAddr)
	putLongwordAt(t, f.ctx, testRabAddr+rabROP, ropBIO)
	service(t, f.ctx, SysConnect, testRabAddr)
	putLongwordAt(t, f.ctx, testRabAddr+rabUBF, testBlockAddr)
	putWord(t, f.ctx, testRabAddr+rabUSZ, 512)
	putLongwordAt(t, f.ctx, testRabAddr+rabBKT, 1)
	wantSts(t, "$READ of an empty file", service(t, f.ctx, SysRead, testRabAddr), rmsEOF)
}

func TestDisconnect(t *testing.T) {
	f := newCreateFixture(t, true)
	newFileFAB(t, f.ctx, "DUA0:D.DAT", facGet|facPut, 4, 0)
	service(t, f.ctx, SysCreate, testFabAddr)
	connectRAB(t, f.ctx, testFabAddr)

	putRecord(t, f.ctx, testRabAddr, []byte("ABCD"))
	wantSts(t, "$PUT", service(t, f.ctx, SysPut, testRabAddr), rmsNormal)
	wantSts(t, "$DISCONNECT", service(t, f.ctx, SysDisconnect, testRabAddr), rmsNormal)

	if isi := readWord(t, f.ctx, testRabAddr+rabISI); isi != 0 {
		t.Errorf("RAB$W_ISI %d, want 0", isi)
	}

	wantSts(t, "$PUT disconnected", service(t, f.ctx, SysPut, testRabAddr), rmsInvalidIFI)
	wantSts(t, "$DISCONNECT again", service(t, f.ctx, SysDisconnect, testRabAddr), rmsInvalidISI)

	wantSts(t, "$CONNECT", service(t, f.ctx, SysConnect, testRabAddr), rmsNormal)

	if record, sts := getRecord(t, f.ctx, racSeq, 0); sts != rmsNormal || string(record) != "ABCD" {
		t.Errorf("$GET = %q, %s", record, StatusName(sts))
	}
}

func TestFOP(t *testing.T) {
	cases := []struct {
		name     string
		atCreate uint32
		atClose  uint32
		exists   bool
	}{
		{"nothing", 0, 0, true},
		{"TMP", fopTMP, fopTMP, false},
		{"TMD", fopTMD, fopTMD, false},
		{"DLT at $CREATE", fopDLT, fopDLT, false},
		{"DLT at $CLOSE", 0, fopDLT, false},
		{"DLT at $CREATE only", fopDLT, 0, true},
		{"SPL", 0, fopSPL, true},
		{"SPL and DLT", 0, fopSPL | fopDLT, true},
		{"SCF", 0, fopSCF, true},
		{"SCF and DLT", 0, fopSCF | fopDLT, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newCreateFixture(t, true)
			newFileFAB(t, f.ctx, "DUA0:F.DAT", facPut, 4, c.atCreate)
			wantSts(t, "$CREATE", service(t, f.ctx, SysCreate, testFabAddr), rmsNormal)
			connectRAB(t, f.ctx, testFabAddr)
			putRecord(t, f.ctx, testRabAddr, []byte("ABCD"))
			wantSts(t, "$PUT", service(t, f.ctx, SysPut, testRabAddr), rmsNormal)
			putLongwordAt(t, f.ctx, testFabAddr+fabFOP, c.atClose)
			wantSts(t, "$CLOSE", service(t, f.ctx, SysClose, testFabAddr), rmsNormal)

			want := rmsFileNotFound
			if c.exists {
				want = rmsNormal
			}

			putLongwordAt(t, f.ctx, testFabAddr+fabFOP, 0)
			putByte(t, f.ctx, testFabAddr+fabFAC, facGet)
			wantSts(t, "$OPEN", service(t, f.ctx, SysOpen, testFabAddr), want)
		})
	}
}

// TestParseNoNAM: with no NAM, $PARSE is the name's status alone.
func TestParseNoNAM(t *testing.T) {
	f := newCreateFixture(t, true)

	putByte(t, f.ctx, testFabAddr+fabBID, fabBIDValue)
	putByte(t, f.ctx, testFabAddr+fabBLN, fabBLNValue)

	newFAB(t, f.ctx, "DUA0:OK.DAT")
	wantSts(t, "a good name", service(t, f.ctx, SysParse, testFabAddr), rmsNormal)

	newFAB(t, f.ctx, "A[B")
	wantSts(t, "a bad name", service(t, f.ctx, SysParse, testFabAddr), rmsSyntaxError)
}

// TestCreateIfStatuses: FAB$V_CIF's RMS$_CREATED when it made the file, and the
// found file's attributes when it didn't.
func TestCreateIfStatuses(t *testing.T) {
	f := newCreateFixture(t, true)
	newFileFAB(t, f.ctx, "DUA0:C.DAT", facPut, 20, fopCIF)
	wantSts(t, "made", service(t, f.ctx, SysCreate, testFabAddr), rmsCreated)
	service(t, f.ctx, SysClose, testFabAddr)

	putWord(t, f.ctx, testFabAddr+fabMRS, 99)
	wantSts(t, "found", service(t, f.ctx, SysCreate, testFabAddr), rmsNormal)

	if mrs := readWord(t, f.ctx, testFabAddr+fabMRS); mrs != 20 {
		t.Errorf("FAB$W_MRS %d, want the file's 20", mrs)
	}
}

// TestCreateFixNoSize: a fixed-length file needs a record size.
func TestCreateFixNoSize(t *testing.T) {
	f := newCreateFixture(t, true)
	newFileFAB(t, f.ctx, "DUA0:Z.DAT", facPut, 0, 0)
	wantSts(t, "$CREATE", service(t, f.ctx, SysCreate, testFabAddr), rmsInvalidMRS)
}

// TestManyVersions: more versions of a name than one directory record holds.
func TestManyVersions(t *testing.T) {
	f := newCreateFixture(t, true)

	for range 80 {
		newFileFAB(t, f.ctx, "DUA0:MANY.DAT", facPut, 4, 0)
		wantSts(t, "$CREATE", service(t, f.ctx, SysCreate, testFabAddr), rmsNormal)
		wantSts(t, "$CLOSE", service(t, f.ctx, SysClose, testFabAddr), rmsNormal)
	}

	for _, name := range []string{"DUA0:MANY.DAT;80", "DUA0:MANY.DAT;64", "DUA0:MANY.DAT;1", "DUA0:MANY.DAT"} {
		newFileFAB(t, f.ctx, name, facGet, 4, 0)
		wantSts(t, name, service(t, f.ctx, SysOpen, testFabAddr), rmsNormal)
		service(t, f.ctx, SysClose, testFabAddr)
	}
}

// TestParseBadStrings: $PARSE of a string it can't read is a status and leaves
// the NAM's answers, which is how FORRTL's INQUIRE ends a search.
func TestParseBadStrings(t *testing.T) {
	f := newCreateFixture(t, true)
	ctx := f.ctx

	putByte(t, ctx, testFabAddr+fabBID, fabBIDValue)
	putByte(t, ctx, testFabAddr+fabBLN, fabBLNValue)
	newFAB(t, ctx, "DUA0:OK.DAT")
	putByte(t, ctx, testNamAddr+namBID, namBIDValue)
	putByte(t, ctx, testNamAddr+namBLN, namBLNValue)
	putLongwordAt(t, ctx, testNamAddr+namESA, testESAAddr)
	putByte(t, ctx, testNamAddr+namESS, 255)
	putLongwordAt(t, ctx, testFabAddr+fabNAM, testNamAddr)
	wantSts(t, "a good name", service(t, ctx, SysParse, testFabAddr), rmsNormal)

	esl := readByte(t, ctx, testNamAddr+namESL)
	if esl == 0 {
		t.Fatal("no expanded name from the first $PARSE")
	}

	// Past this test's memory, as page 0 is in a process's.
	const unreadable = 0x3FFFFFF0

	putLongwordAt(t, ctx, testFabAddr+fabDNA, unreadable)
	putByte(t, ctx, testFabAddr+fabDNS, 1)
	wantSts(t, "no default name", service(t, ctx, SysParse, testFabAddr), vmsConst("RMS$_DNA"))

	if got := readByte(t, ctx, testNamAddr+namESL); got != esl {
		t.Errorf("ESL is %d after it, want %d", got, esl)
	}

	putByte(t, ctx, testFabAddr+fabDNS, 0)
	putLongwordAt(t, ctx, testFabAddr+fabFNA, unreadable)
	wantSts(t, "no name", service(t, ctx, SysParse, testFabAddr), vmsConst("RMS$_FNA"))
}
