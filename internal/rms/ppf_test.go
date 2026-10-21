package rms

import (
	"bytes"
	"testing"

	"github.com/tucats/govax/internal/lnm"
)

// These tests are Phase 71's (docs/PHASE-71 - process-permanent files.md,
// subtask 4): RMS's table of process-permanent files and an image's
// indirect access to them, checked against what VMS 7.1 showed in
// testdata/ppf71.

// ppfFixture is one process with a process-permanent file: its sharer
// (an RMS Context with a FAB at testFabAddr and a RAB at testRabAddr),
// and the entry.
type ppfFixture struct {
	*sharer
	mounts *MountTable
	entry  *PermanentEntry
}

// newPPFFixture makes DUA0:PPF.LOG, a VFC print file like the /OUTPUT
// file of DCL's @, with the records recs, opens it with access fac as a
// process-permanent file, and defines name as its logical name.
func newPPFFixture(t *testing.T, name string, fac byte, recs ...string) *ppfFixture {
	t.Helper()

	p, mounts := newSharers(t, 1)
	s := p[0]
	s.ctx.NodeName = "GOVAX"

	newFAB(t, s.ctx, "DUA0:PPF.LOG")
	putByte(t, s.ctx, testFabAddr+fabRFM, rfmVFC)
	putByte(t, s.ctx, testFabAddr+fabRAT, ratPRN)
	putByte(t, s.ctx, testFabAddr+fabOffset("FSZ"), 2)
	putWord(t, s.ctx, testFabAddr+fabMRS, 0)

	if r0, err := SysCreate(s.ctx, []uint32{testFabAddr}); err != nil || r0&1 == 0 {
		t.Fatalf("$CREATE: %#x, %v", r0, err)
	}

	connectRAB(t, s.ctx, testFabAddr)

	for _, r := range recs {
		s.put("\x01\x8D" + r)
	}

	s.close()

	newFAB(t, s.ctx, "DUA0:PPF.LOG")
	putByte(t, s.ctx, testFabAddr+fabFAC, fac)
	putByte(t, s.ctx, testFabAddr+fabSHR, shrGet)

	if r0, err := SysOpen(s.ctx, []uint32{testFabAddr}); err != nil || r0&1 == 0 {
		t.Fatalf("$OPEN: %#x, %v", r0, err)
	}

	e, err := s.ctx.Files.MakePermanent(readWord(t, s.ctx, testFabAddr+fabIFI), PermanentOptions{
		Device: "_GOVAX$DUA0:", RAT: ratPRN, SQO: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	defineTestLogicalAttrs(t, s.ctx.Logicals, name, lnm.AttrTerminal, e.EquivalenceString())

	return &ppfFixture{sharer: s, mounts: mounts, entry: e}
}

// openFAB lays out a FAB for spec, with RAT=CR and FAC=fac, and a NAM,
// and runs service on it, returning the status.
func (f *ppfFixture) openFAB(service func(*Context, []uint32) (uint32, error), spec string, fac byte, fop uint32) uint32 {
	f.t.Helper()

	ctx := f.ctx
	newFAB(f.t, ctx, spec)
	putByte(f.t, ctx, testFabAddr+fabBID, fabBIDValue)
	putByte(f.t, ctx, testFabAddr+fabBLN, fabBLNValue)
	putByte(f.t, ctx, testFabAddr+fabFAC, fac)
	putByte(f.t, ctx, testFabAddr+fabRFM, rfmVAR)
	putByte(f.t, ctx, testFabAddr+fabRAT, ratCR)
	putWord(f.t, ctx, testFabAddr+fabIFI, 0)
	putLongwordAt(f.t, ctx, testFabAddr+fabFOP, fop)
	putLongwordAt(f.t, ctx, testFabAddr+fabNAM, testNamAddr)

	for i := range uint32(0x300) {
		putByte(f.t, ctx, testNamAddr+i, 0)
	}

	putByte(f.t, ctx, testNamAddr+namBID, namBIDValue)
	putByte(f.t, ctx, testNamAddr+namBLN, namBLNValue)
	putLongwordAt(f.t, ctx, testNamAddr+namESA, testESAAddr)
	putByte(f.t, ctx, testNamAddr+namESS, 255)
	putLongwordAt(f.t, ctx, testNamAddr+namRSA, testRSAAddr)
	putByte(f.t, ctx, testNamAddr+namRSS, 255)

	r0, err := service(ctx, []uint32{testFabAddr})
	if err != nil {
		f.t.Fatal(err)
	}

	return r0
}

// connect runs $CONNECT on the FAB, returning the status.
func (f *ppfFixture) connect() uint32 {
	f.t.Helper()

	putRAB(f.t, f.ctx, testFabAddr)
	putByte(f.t, f.ctx, testRabAddr+rabRAC, racSeq)

	r0, err := SysConnect(f.ctx, []uint32{testRabAddr})
	if err != nil {
		f.t.Fatal(err)
	}

	return r0
}

// rabService runs a RAB service, returning its status.
func (f *ppfFixture) rabService(service func(*Context, []uint32) (uint32, error)) uint32 {
	f.t.Helper()

	r0, err := service(f.ctx, []uint32{testRabAddr})
	if err != nil {
		f.t.Fatal(err)
	}

	return r0
}

// namString reads a NAM string: its length at lenOff, its text at addr.
func (f *ppfFixture) namString(lenOff, addr uint32) string {
	n := readByte(f.t, f.ctx, testNamAddr+lenOff)
	b := make([]byte, n)

	for i := range b {
		b[i] = readByte(f.t, f.ctx, addr+uint32(i))
	}

	return string(b)
}

// dvi reads NAM$T_DVI, a counted string.
func (f *ppfFixture) dvi() string {
	n := readByte(f.t, f.ctx, testNamAddr+namDVI)
	b := make([]byte, n)

	for i := range b {
		b[i] = readByte(f.t, f.ctx, testNamAddr+namDVI+1+uint32(i))
	}

	return string(b)
}

// want compares a value read back with what VMS 7.1 showed.
func want[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()

	if got != want {
		t.Errorf("%s = %#v, want %#v", what, got, want)
	}
}

// TestPermanentWords: the IFI word in an equivalence string, and an
// indirect accessor's IFI and ISI, as VMS 7.1 numbered them (Part A's
// table: %X8101 for the terminal's SYS$INPUT, %X8005 for OPEN/READ's
// file, %XC084 for an open of SYS$OUTPUT with RAT=CR, entry 4).
func TestPermanentWords(t *testing.T) {
	want(t, "ppfWord(1, PRN)", ppfWord(1, ratPRN), 0x8101)
	want(t, "ppfWord(5, 0)", ppfWord(5, 0), 0x8005)
	want(t, "ppfWord(6, PRN)", ppfWord(6, ratPRN), 0x8106)
	want(t, "indirectWord(4, CR)", indirectWord(4, ratCR), 0xC084)
	want(t, "wordRAT(C084)", wordRAT(0xC084), byte(ratCR))
	want(t, "wordRAT(8101)", wordRAT(0x8101), byte(ratPRN))
	want(t, "isIndirect(C084)", isIndirect(0xC084), true)
	want(t, "isIndirect(8104)", isIndirect(0x8104), false)
	want(t, "isIndirect(4)", isIndirect(4), false)
}

// TestPermanent_outputIndirect follows testdata/ppf71's O1 to O4: an
// image's $OPEN and $CREATE of a name for the /OUTPUT file.
func TestPermanent_outputIndirect(t *testing.T) {
	f := newPPFFixture(t, "TEST$OUT", facPut, "from DCL")
	ctx := f.ctx

	// O1: $OPEN reaches the file indirectly.
	want(t, "$OPEN", f.openFAB(SysOpen, "TEST$OUT", facPut, 0), rmsNormal)

	ifi := readWord(t, ctx, testFabAddr+fabIFI)
	want(t, "IFI", ifi, indirectWord(f.entry.index, ratCR))
	want(t, "FOP", readLongword(t, ctx, testFabAddr+fabFOP), 0)
	want(t, "DEV", readLongword(t, ctx, testFabAddr+fabDEV), 0x08CD0103)
	want(t, "SDC", readLongword(t, ctx, testFabAddr+fabSDC), 0x18CD4108)
	want(t, "FAC", readByte(t, ctx, testFabAddr+fabFAC), facPut)
	want(t, "RFM", readByte(t, ctx, testFabAddr+fabRFM), byte(rfmVFC))
	want(t, "RAT", readByte(t, ctx, testFabAddr+fabRAT), byte(ratPRN))
	want(t, "FSZ", readByte(t, ctx, testFabAddr+fabOffset("FSZ")), 2)
	want(t, "ESA", f.namString(namESL, testESAAddr), "TEST$OUT:.;")
	want(t, "RSA", f.namString(namRSL, testRSAAddr), "TEST$OUT:.;")
	want(t, "FNB", readLongword(t, ctx, testNamAddr+namFNB), 0x00010080)
	want(t, "DVI", f.dvi(), "TEST$OUT")
	want(t, "FID", readWord(t, ctx, testNamAddr+namFID), 0)

	want(t, "$CONNECT", f.connect(), rmsNormal)
	want(t, "ISI", readWord(t, ctx, testRabAddr+rabISI), indirectWord(f.entry.stream, ratCR))

	putRecord(t, ctx, testRabAddr, []byte("O1 a record"))
	want(t, "$PUT", f.rabService(SysPut), rmsNormal)

	// $DISPLAY shows the file as the open that made it: its access, its
	// resultant string, its device.
	want(t, "$DISPLAY", f.rabFAB(SysDisplay), rmsNormal)
	want(t, "$DISPLAY STV", readLongword(t, ctx, testFabAddr+fabSTV), ppfDisplaySTV|uint32(f.entry.stream))
	want(t, "$DISPLAY FAC", readByte(t, ctx, testFabAddr+fabFAC), facPut)
	want(t, "$DISPLAY ESA", f.namString(namESL, testESAAddr), "TEST$OUT:.;")
	want(t, "$DISPLAY RSA", f.namString(namRSL, testRSAAddr), "DUA0:[000000]PPF.LOG;1")
	want(t, "$DISPLAY DVI", f.dvi(), "_GOVAX$DUA0")

	// What an indirect accessor may not do.
	want(t, "$REWIND", f.rabService(SysRewind), rmsIOP)
	want(t, "$FIND", f.rabService(SysFind), rmsFACNotAllowed)
	want(t, "$GET", f.rabService(SysGet), rmsFACNotAllowed)
	want(t, "$ERASE", f.rabFAB(SysErase), rmsInvalidIFI)

	want(t, "$CLOSE", f.rabFAB(SysClose), rmsNormal)
	want(t, "IFI after $CLOSE", readWord(t, ctx, testFabAddr+fabIFI), 0)

	// O2: $CREATE is $OPEN.
	want(t, "$CREATE", f.openFAB(SysCreate, "TEST$OUT", facPut, 0), rmsNormal)
	want(t, "$CREATE IFI", readWord(t, ctx, testFabAddr+fabIFI), ifi)
	f.connect()
	putRecord(t, ctx, testRabAddr, []byte("O2 a record"))
	want(t, "$PUT after $CREATE", f.rabService(SysPut), rmsNormal)
	f.rabFAB(SysClose)

	// O4: FAC is ignored; the original open's access decides.
	want(t, "$OPEN FAC=GET", f.openFAB(SysOpen, "TEST$OUT", facGet, 0), rmsNormal)
	f.connect()
	putRecord(t, ctx, testRabAddr, []byte("O4 a record"))
	want(t, "$PUT with FAC=GET", f.rabService(SysPut), rmsNormal)
	f.rabFAB(SysClose)

	// Image rundown ends indirect accesses and leaves the file open.
	if n, err := ctx.Files.Rundown(); n != 0 || err != nil {
		t.Errorf("Rundown closed %d files, %v; want none", n, err)
	}

	want(t, "$OPEN after rundown", f.openFAB(SysOpen, "TEST$OUT", facPut, 0), rmsNormal)

	if n, err := ctx.Files.ClosePermanentFiles(); n != 1 || err != nil {
		t.Fatalf("ClosePermanentFiles closed %d, %v; want 1", n, err)
	}

	// The records, each with the print file's carriage control.
	wantRecords(t, records(t, f.mounts, "PPF.LOG"),
		"\x01\x8Dfrom DCL", "\x01\x8DO1 a record", "\x01\x8DO2 a record", "\x01\x8DO4 a record")
}

// rabFAB runs a FAB service on the fixture's FAB, returning its status.
func (f *ppfFixture) rabFAB(service func(*Context, []uint32) (uint32, error)) uint32 {
	f.t.Helper()

	r0, err := service(f.ctx, []uint32{testFabAddr})
	if err != nil {
		f.t.Fatal(err)
	}

	return r0
}

// TestPermanent_twoOpensOneStream follows O3 and I1 to I8: two indirect
// opens share the file's one position.
func TestPermanent_twoOpensOneStream(t *testing.T) {
	f := newPPFFixture(t, "TEST$IN", facGet, "rec 1", "rec 2", "rec 3", "rec 4")
	ctx := f.ctx

	const secondFAB, secondRAB = 0x7000, 0x7400

	want(t, "first $OPEN", f.openFAB(SysOpen, "TEST$IN", facGet, 0), rmsNormal)
	f.connect()

	// A second FAB and RAB: copies of the first.
	for i := range uint32(0x50) {
		putByte(t, ctx, secondFAB+i, readByte(t, ctx, testFabAddr+i))
		putByte(t, ctx, secondRAB+i, readByte(t, ctx, testRabAddr+i))
	}

	putLongwordAt(t, ctx, secondRAB+rabFAB, secondFAB)

	get := func(rab uint32) (string, uint32) {
		t.Helper()

		putLongwordAt(t, ctx, rab+rabUBF, testRecordAddr)
		putWord(t, ctx, rab+rabUSZ, 1024)

		r0, err := SysGet(ctx, []uint32{rab})
		if err != nil {
			t.Fatal(err)
		}

		n := readWord(t, ctx, rab+rabRSZ)
		b := make([]byte, n)

		for i := range b {
			b[i] = readByte(t, ctx, testRecordAddr+uint32(i))
		}

		return string(bytes.TrimPrefix(b, []byte("\x01\x8D"))), r0
	}

	for i, step := range []struct {
		rab  uint32
		want string
	}{{testRabAddr, "rec 1"}, {secondRAB, "rec 2"}, {testRabAddr, "rec 3"}, {secondRAB, "rec 4"}} {
		rec, sts := get(step.rab)
		if sts != rmsNormal || rec != step.want {
			t.Errorf("get %d: %q, %#x; want %q", i+1, rec, sts, step.want)
		}
	}

	if _, sts := get(testRabAddr); sts != rmsEOF {
		t.Errorf("get at the end: %#x, want RMS$_EOF", sts)
	}

	// I9: an input stream refuses $PUT.
	putRecord(t, ctx, testRabAddr, []byte("no"))
	want(t, "$PUT to an input stream", f.rabService(SysPut), rmsFACNotAllowed)

	// An input stream's device has no output.
	want(t, "SDC", readLongword(t, ctx, testFabAddr+fabSDC), 0x14CD4108)
	want(t, "DEV", readLongword(t, ctx, testFabAddr+fabDEV), 0x04CD0103)
}

// TestPermanent_names follows O5 to O12: FOP's refusals, a file name
// after the colon, an alias, $PARSE, and $SEARCH.
func TestPermanent_names(t *testing.T) {
	f := newPPFFixture(t, "TEST$OUT", facPut)
	ctx := f.ctx

	// O5: PPF is refused before the name is processed.
	want(t, "FOP=PPF", f.openFAB(SysOpen, "TEST$OUT", facPut, fopPPF), rmsPrivilegeViolation)
	want(t, "FOP=PPF ESA", f.namString(namESL, testESAAddr), "")

	// O6, O7: UFO and NFS after it.
	for _, fop := range []uint32{fopUFO, fopNFS} {
		want(t, "FOP UFO/NFS", f.openFAB(SysOpen, "TEST$OUT", facPut, fop), rmsFileOptionError)
		want(t, "FOP UFO/NFS ESA", f.namString(namESL, testESAAddr), "TEST$OUT:.;")
		want(t, "FOP UFO/NFS RSA", f.namString(namRSL, testRSAAddr), "")
		want(t, "FOP UFO/NFS IFI", readWord(t, ctx, testFabAddr+fabIFI), 0)
	}

	// O8, O9: the colon, and a name after it, kept and ignored.
	want(t, "$OPEN with colon", f.openFAB(SysOpen, "TEST$OUT:", facPut, 0), rmsNormal)
	want(t, "colon ESA", f.namString(namESL, testESAAddr), "TEST$OUT:.;")
	f.rabFAB(SysClose)

	want(t, "$OPEN with a name", f.openFAB(SysOpen, "TEST$OUT:PPF71X.DAT", facPut, 0), rmsNormal)
	want(t, "name ESA", f.namString(namESL, testESAAddr), "TEST$OUT:PPF71X.DAT;")
	want(t, "name RSA", f.namString(namRSL, testRSAAddr), "TEST$OUT:PPF71X.DAT;")
	want(t, "name FNB", readLongword(t, ctx, testNamAddr+namFNB), 0x00010086)
	f.rabFAB(SysClose)

	// O11: through an alias, the process-permanent name.
	defineTestLogical(t, ctx.Logicals, "TEST$ALIAS", "TEST$OUT")
	want(t, "$OPEN of the alias", f.openFAB(SysOpen, "TEST$ALIAS", facPut, 0), rmsNormal)
	want(t, "alias ESA", f.namString(namESL, testESAAddr), "TEST$OUT:.;")
	want(t, "alias DVI", f.dvi(), "TEST$OUT")
	f.rabFAB(SysClose)

	// O12: $PARSE succeeds; $SEARCH finds no device.
	want(t, "$PARSE", f.openFAB(SysParse, "TEST$OUT", facPut, 0), rmsNormal)
	want(t, "$PARSE ESA", f.namString(namESL, testESAAddr), "TEST$OUT:.;")
	want(t, "$PARSE FNB", readLongword(t, ctx, testNamAddr+namFNB), 0x00010080)
	want(t, "$PARSE DVI", f.dvi(), "TEST$OUT")
	want(t, "$PARSE DEV", readLongword(t, ctx, testFabAddr+fabDEV), 0x18CD4108)
	want(t, "$PARSE SDC", readLongword(t, ctx, testFabAddr+fabSDC), 0x18CD4108)
	want(t, "$SEARCH", f.rabFAB(SysSearch), rmsDeviceError)
	want(t, "$SEARCH STV", readLongword(t, ctx, testFabAddr+fabSTV), ssNoSuchDevice)

	// A default name's device leads there too, without EXP_DEV.
	newFAB(t, ctx, "X.DAT")
	putLongwordAt(t, ctx, testFabAddr+fabNAM, testNamAddr)
	putLongwordAt(t, ctx, testFabAddr+fabDNA, 0x2800)

	for i, c := range []byte("TEST$OUT:") {
		putByte(t, ctx, 0x2800+uint32(i), c)
	}

	putByte(t, ctx, testFabAddr+fabDNS, byte(len("TEST$OUT:")))
	want(t, "$PARSE with a default", f.rabFAB(SysParse), rmsNormal)
	want(t, "default ESA", f.namString(namESL, testESAAddr), "TEST$OUT:X.DAT;")
	want(t, "default FNB", readLongword(t, ctx, testNamAddr+namFNB), 0x00010006)

	// $CRELNM's copy of the string is a name for the same file (O13).
	defineTestLogical(t, ctx.Logicals, "TEST$COPY", f.entry.EquivalenceString())
	want(t, "$OPEN of the copy", f.openFAB(SysOpen, "TEST$COPY", facPut, 0), rmsNormal)
	want(t, "copy ESA", f.namString(namESL, testESAAddr), "TEST$COPY:.;")
	want(t, "copy IFI", readWord(t, ctx, testFabAddr+fabIFI), indirectWord(f.entry.index, ratCR))
	f.rabFAB(SysClose)

	// A name whose file has been closed.
	if ok, err := ctx.Files.ClosePermanent(f.entry.IFI()); !ok || err != nil {
		t.Fatalf("ClosePermanent: %v, %v", ok, err)
	}

	want(t, "$OPEN of a closed file", f.openFAB(SysOpen, "TEST$OUT", facPut, 0), rmsInvalidIFI)
}

// TestPermanent_terminal follows the terminal run's O1: the terminal's
// own SYS$OUTPUT, opened for reading and writing.
func TestPermanent_terminal(t *testing.T) {
	p, _ := newSharers(t, 1)
	s := p[0]

	console := &bytes.Buffer{}

	e, err := s.ctx.Files.AddPermanent(&FileHandle{Console: console, Access: facGet | facPut},
		PermanentOptions{Device: "_GOVAX$TTA0:", RAT: ratPRN, SQO: true})
	if err != nil {
		t.Fatal(err)
	}

	defineTestLogicalAttrs(t, s.ctx.Logicals, "TEST$TERM", lnm.AttrTerminal, e.EquivalenceString())

	f := &ppfFixture{sharer: s, entry: e}
	ctx := s.ctx

	want(t, "$OPEN", f.openFAB(SysOpen, "TEST$TERM", facPut, 0), rmsNormal)
	want(t, "DEV", readLongword(t, ctx, testFabAddr+fabDEV), 0x0C040007)
	want(t, "SDC", readLongword(t, ctx, testFabAddr+fabSDC), 0x0C040007)
	want(t, "RFM", readByte(t, ctx, testFabAddr+fabRFM), byte(rfmVAR))
	want(t, "RAT", readByte(t, ctx, testFabAddr+fabRAT), byte(ratCR))
	want(t, "FSZ", readByte(t, ctx, testFabAddr+fabOffset("FSZ")), 2)

	f.connect()
	putRecord(t, ctx, testRabAddr, []byte("to the terminal"))
	want(t, "$PUT", f.rabService(SysPut), rmsNormal)
	want(t, "console", console.String(), "to the terminal\n")

	want(t, "$DISPLAY", f.rabFAB(SysDisplay), rmsNormal)
	want(t, "$DISPLAY FAC", readByte(t, ctx, testFabAddr+fabFAC), facGet|facPut)
	want(t, "$DISPLAY RFM", readByte(t, ctx, testFabAddr+fabRFM), byte(rfmVFC))
	want(t, "$DISPLAY RAT", readByte(t, ctx, testFabAddr+fabRAT), byte(ratPRN))
	want(t, "$DISPLAY DVI", f.dvi(), "_GOVAX$TTA0")

	want(t, "string", lnm.DisplayValue(e.EquivalenceString()), "_GOVAX$TTA0:")

	if pf, ok := lnm.ParsePermanentFile(e.EquivalenceString()); !ok || pf.IFI != 0x8101 {
		t.Errorf("string's IFI %#x, %v; want %#x", pf.IFI, ok, 0x8101)
	}
}
