package rms

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

// openAndConnectForRead performs the SYS$OPEN (for GET access) + SYS$CONNECT
// sequence a real VAX program runs before its first SYS$GET against an
// already-existing file named name on the fixture's mounted "DUA0" device
// — the read-side counterpart to put_test.go's own connectRAB, which
// assumes the file was just SYS$CREATEd instead. Returns the IFI the
// underlying FAB was assigned.
func openAndConnectForRead(t *testing.T, ctx *Context, name string) uint16 {
	t.Helper()

	newFAB(t, ctx, "DUA0:"+name)
	putByte(t, ctx, testFabAddr+fabFAC, facGet)

	if r0, err := SysOpen(ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysOpen: %v", err)
	} else if r0 != rmsNormal {
		t.Fatalf("SysOpen r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	return connectRAB(t, ctx, testFabAddr)
}

// TestSysGet_diskFile confirms SYS$GET against a real, freshly written and
// reopened disk file reads its one record back correctly — the read-side
// half of docs/PHASE-22.md's whole acceptance criterion (subtask 14's own
// CREATE-then-PUT-then-CLOSE-then-OPEN-then-GET sequence) — and that
// RAB$W_RSZ is updated with the record's true length.
func TestSysGet_diskFile(t *testing.T) {
	f := newCreateFixture(t, true)

	record := bytes.Repeat([]byte{'G'}, 80)
	createAndCloseTestFile(t, f.ctx, "GET.DAT", record)

	openAndConnectForRead(t, f.ctx, "GET.DAT")

	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)
	putLongwordAt(t, f.ctx, testRabAddr+rabRBF, testRecordAddr)
	putWord(t, f.ctx, testRabAddr+rabRSZ, 0) // sentinel: SysGet must overwrite this

	r0, err := SysGet(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysGet: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	if sts := readLongword(t, f.ctx, testRabAddr+rabSTS); sts != rmsNormal {
		t.Errorf("RAB$L_STS = %d, want rmsNormal (%d)", sts, rmsNormal)
	}

	if rsz := readWord(t, f.ctx, testRabAddr+rabRSZ); rsz != uint16(len(record)) {
		t.Errorf("RAB$W_RSZ = %d, want %d", rsz, len(record))
	}

	got := make([]byte, len(record))
	for i := range got {
		got[i] = readByte(t, f.ctx, testRecordAddr+uint32(i))
	}

	if !bytes.Equal(got, record) {
		t.Errorf("record read into RAB$L_RBF = %q, want %q", got, record)
	}
}

// TestSysGet_multipleRecordsThenEOF confirms successive SYS$GET calls
// through the same RAB read a multi-record file back in order — the
// ordinary way a VAX program reads a whole file (one SYS$GET call per
// record, in a loop) — and that reading past the last record reports
// RMS$_EOF rather than an error or a bogus zero-length record.
func TestSysGet_multipleRecordsThenEOF(t *testing.T) {
	f := newCreateFixture(t, true)

	records := [][]byte{
		bytes.Repeat([]byte{'A'}, 80),
		bytes.Repeat([]byte{'B'}, 80),
		bytes.Repeat([]byte{'C'}, 80),
	}

	newFAB(t, f.ctx, "DUA0:MULTI.DAT")

	if _, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	connectRAB(t, f.ctx, testFabAddr)
	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)

	for _, record := range records {
		putRecord(t, f.ctx, testRabAddr, record)

		if _, err := SysPut(f.ctx, []uint32{testRabAddr}); err != nil {
			t.Fatalf("SysPut: %v", err)
		}
	}

	if _, err := SysClose(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysClose: %v", err)
	}

	openAndConnectForRead(t, f.ctx, "MULTI.DAT")
	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)
	putLongwordAt(t, f.ctx, testRabAddr+rabRBF, testRecordAddr)

	for i, want := range records {
		r0, err := SysGet(f.ctx, []uint32{testRabAddr})
		if err != nil {
			t.Fatalf("SysGet (record %d): %v", i, err)
		}

		if r0 != rmsNormal {
			t.Fatalf("SysGet (record %d) r0 = %d, want rmsNormal (%d)", i, r0, rmsNormal)
		}

		got := make([]byte, len(want))
		for j := range got {
			got[j] = readByte(t, f.ctx, testRecordAddr+uint32(j))
		}

		if !bytes.Equal(got, want) {
			t.Errorf("record %d = %q, want %q", i, got, want)
		}
	}

	// One more GET than the file has records: RMS$_EOF, not an error.
	r0, err := SysGet(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysGet (past EOF): %v", err)
	}

	if r0 != rmsEOF {
		t.Errorf("r0 = %d, want rmsEOF (%d)", r0, rmsEOF)
	}
}

// TestSysGet_userBuffer confirms SYS$GET honors a RAB that supplies a
// separate RAB$L_UBF/RAB$W_USZ "user buffer" pair: the record lands in
// UBF, not RBF, matching rab.go's own doc comment on the two fields.
func TestSysGet_userBuffer(t *testing.T) {
	f := newCreateFixture(t, true)

	record := bytes.Repeat([]byte{'U'}, 80)
	createAndCloseTestFile(t, f.ctx, "UBUF.DAT", record)

	openAndConnectForRead(t, f.ctx, "UBUF.DAT")

	// A sentinel written to the RBF destination before the call — distinct
	// from both the fixture file's own 'U' record bytes and from zero — so
	// that "RAB$L_RBF was left alone" can be checked by exact comparison
	// afterward, rather than assuming that unrelated memory happens to
	// start out zeroed.
	sentinel := bytes.Repeat([]byte{'.'}, len(record))
	for i, b := range sentinel {
		putByte(t, f.ctx, testRecordAddr+uint32(i), b)
	}

	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)
	putLongwordAt(t, f.ctx, testRabAddr+rabRBF, testRecordAddr)
	putLongwordAt(t, f.ctx, testRabAddr+rabUBF, testUserBufAddr)
	putWord(t, f.ctx, testRabAddr+rabUSZ, uint16(len(record)))

	r0, err := SysGet(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysGet: %v", err)
	}

	if r0 != rmsNormal {
		t.Errorf("r0 = %d, want rmsNormal (%d)", r0, rmsNormal)
	}

	got := make([]byte, len(record))
	for i := range got {
		got[i] = readByte(t, f.ctx, testUserBufAddr+uint32(i))
	}

	if !bytes.Equal(got, record) {
		t.Errorf("record read into RAB$L_UBF = %q, want %q", got, record)
	}

	// RAB$L_RBF was never the actual destination when UBF is supplied —
	// its sentinel bytes should be exactly as this test left them,
	// confirming SysGet really did prefer UBF rather than writing to both.
	afterward := make([]byte, len(record))
	for i := range afterward {
		afterward[i] = readByte(t, f.ctx, testRecordAddr+uint32(i))
	}

	if !bytes.Equal(afterward, sentinel) {
		t.Errorf("RAB$L_RBF's memory = %q, want the untouched sentinel %q", afterward, sentinel)
	}
}

// TestSysGet_userBufferTooSmall confirms a record too large for a
// caller-declared RAB$W_USZ user-buffer capacity fails with RMS$_RSZ,
// matching SysPut's own equivalent check for an outgoing record that
// doesn't fit a file's declared format.
func TestSysGet_userBufferTooSmall(t *testing.T) {
	f := newCreateFixture(t, true)

	record := bytes.Repeat([]byte{'S'}, 80)
	createAndCloseTestFile(t, f.ctx, "SMALL.DAT", record)

	openAndConnectForRead(t, f.ctx, "SMALL.DAT")

	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)
	putLongwordAt(t, f.ctx, testRabAddr+rabRBF, testRecordAddr)
	putLongwordAt(t, f.ctx, testRabAddr+rabUBF, testUserBufAddr)
	putWord(t, f.ctx, testRabAddr+rabUSZ, uint16(len(record)-1)) // one byte too small

	r0, err := SysGet(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysGet: %v", err)
	}

	if r0 != rmsRecordTooBig {
		t.Errorf("r0 = %d, want rmsRecordTooBig (%d)", r0, rmsRecordTooBig)
	}
}

// TestSysGet_invalidIFI confirms SYS$GET against a RAB that was never
// SYS$CONNECTed (RAB$W_ISI never set to a real IFI) fails with RMS$_IFI
// rather than panicking on a nil FileHandle, mirroring SysPut's own
// TestSysPut_invalidIFI.
func TestSysGet_invalidIFI(t *testing.T) {
	f := newCreateFixture(t, true)

	putRAB(t, f.ctx, testFabAddr)
	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)

	r0, err := SysGet(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysGet: %v", err)
	}

	if r0 != rmsInvalidIFI {
		t.Errorf("r0 = %d, want rmsInvalidIFI (%d)", r0, rmsInvalidIFI)
	}
}

// TestSysGet_invalidRAC confirms a RAB whose RAB$B_RAC names anything
// other than sequential access (rab.go's racSeq) fails with RMS$_RAC,
// mirroring SysPut's own TestSysPut_invalidRAC.
func TestSysGet_invalidRAC(t *testing.T) {
	f := newCreateFixture(t, true)

	createAndCloseTestFile(t, f.ctx, "RAC.DAT", bytes.Repeat([]byte{'R'}, 80))
	openAndConnectForRead(t, f.ctx, "RAC.DAT")

	putByte(t, f.ctx, testRabAddr+rabRAC, racKey) // not sequential

	r0, err := SysGet(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysGet: %v", err)
	}

	if r0 != rmsInvalidRAC {
		t.Errorf("r0 = %d, want rmsInvalidRAC (%d)", r0, rmsInvalidRAC)
	}
}

// consoleGetFixture creates TTA0: and connects a RAB to it, with typed
// as the terminal's input and the RAB's prompt (RAB$V_PMT) set to prompt
// when it isn't empty. Records go to the user buffer at testUserBufAddr,
// usz bytes long. It returns the fixture and what the console was sent.
func consoleGetFixture(t *testing.T, typed, prompt string, usz uint16) (*createFixture, *bytes.Buffer) {
	t.Helper()

	const testPromptAddr = 0x7000

	f := newCreateFixture(t, true)
	out := &bytes.Buffer{}
	f.ctx.Console = out
	f.ctx.ConsoleIn = bufio.NewReader(strings.NewReader(typed))
	newFAB(t, f.ctx, "TTA0:")

	if _, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	connectRAB(t, f.ctx, testFabAddr)
	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)
	putLongwordAt(t, f.ctx, testRabAddr+rabUBF, testUserBufAddr)
	putWord(t, f.ctx, testRabAddr+rabUSZ, usz)

	if prompt != "" {
		putLongwordAt(t, f.ctx, testRabAddr+rabROP, ropPMT)
		putLongwordAt(t, f.ctx, testRabAddr+rabPBF, testPromptAddr)
		putByte(t, f.ctx, testRabAddr+rabPSZ, byte(len(prompt)))

		for i := 0; i < len(prompt); i++ {
			putByte(t, f.ctx, testPromptAddr+uint32(i), prompt[i])
		}
	}

	return f, out
}

// testUserBufAddr is where the tests' RABs point RAB$L_UBF.
const testUserBufAddr = 0x6000

// consoleGet runs SYS$GET on consoleGetFixture's RAB, returning its
// status and the record it read.
func consoleGet(t *testing.T, f *createFixture) (uint32, string) {
	t.Helper()

	r0, err := SysGet(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysGet: %v", err)
	}

	if r0 != rmsNormal {
		return r0, ""
	}

	n := readWord(t, f.ctx, testRabAddr+rabRSZ)
	got := make([]byte, n)

	for i := range got {
		got[i] = readByte(t, f.ctx, testUserBufAddr+uint32(i))
	}

	return r0, string(got)
}

// TestSysGet_console reads lines typed at the terminal: each SYS$GET
// writes the prompt, then returns one line without its terminator (a
// "\r\n" pair is one), and the end of input is RMS$_EOF.
func TestSysGet_console(t *testing.T) {
	f, out := consoleGetFixture(t, "2 3 + .\r\nhalt\n\nlast", "> ", 80)

	for _, want := range []string{"2 3 + .", "halt", "", "last"} {
		if r0, got := consoleGet(t, f); r0 != rmsNormal || got != want {
			t.Errorf("SYS$GET = %#x, %q; want RMS$_NORMAL, %q", r0, got, want)
		}
	}

	if r0, _ := consoleGet(t, f); r0 != rmsEOF {
		t.Errorf("SYS$GET at end of input = %#x, want RMS$_EOF", r0)
	}

	if got, want := out.String(), strings.Repeat("> ", 5); got != want {
		t.Errorf("prompts = %q, want %q", got, want)
	}
}

// TestSysGet_consoleCtrlZ is end of file at a Ctrl/Z, and a line longer
// than the user buffer is read a buffer at a time.
func TestSysGet_consoleCtrlZ(t *testing.T) {
	f, out := consoleGetFixture(t, "abcdef\n\x1Amore\n", "", 4)

	for _, want := range []string{"abcd", "ef"} {
		if r0, got := consoleGet(t, f); r0 != rmsNormal || got != want {
			t.Errorf("SYS$GET = %#x, %q; want RMS$_NORMAL, %q", r0, got, want)
		}
	}

	if r0, _ := consoleGet(t, f); r0 != rmsEOF {
		t.Errorf("SYS$GET at Ctrl/Z = %#x, want RMS$_EOF", r0)
	}

	if out.Len() != 0 {
		t.Errorf("console output = %q, want none without RAB$V_PMT", out.String())
	}
}

// TestSysGet_consoleCtrlZAfterText: a Ctrl/Z typed after some text ends
// that text's record, and the second Ctrl/Z the front end sends after it
// is the next read's end of file.
func TestSysGet_consoleCtrlZAfterText(t *testing.T) {
	f, _ := consoleGetFixture(t, "show\x1A\x1Anext\n", "", 80)

	if r0, got := consoleGet(t, f); r0 != rmsNormal || got != "show" {
		t.Errorf("SYS$GET = %#x, %q; want RMS$_NORMAL, %q", r0, got, "show")
	}

	if r0, _ := consoleGet(t, f); r0 != rmsEOF {
		t.Errorf("SYS$GET after the record = %#x, want RMS$_EOF", r0)
	}

	if r0, got := consoleGet(t, f); r0 != rmsNormal || got != "next" {
		t.Errorf("SYS$GET after the end of file = %#x, %q; want RMS$_NORMAL, %q", r0, got, "next")
	}
}

// TestSysGet_noReadAccess confirms SYS$GET through a RAB that was
// SYS$CONNECTed for writing only (FAB$V_PUT, arming a Writer but no
// Reader) fails with RMS$_FAC (the RMS manual: a record operation the file
// access doesn't allow) rather than dereferencing a nil Reader —
// the mirror image of put_test.go's own TestSysPut_noWriteAccess.
func TestSysGet_noReadAccess(t *testing.T) {
	f := newCreateFixture(t, true)
	newFAB(t, f.ctx, "DUA0:WRITEONLY.DAT")

	if _, err := SysCreate(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	connectRAB(t, f.ctx, testFabAddr)
	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)

	r0, err := SysGet(f.ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysGet: %v", err)
	}

	if r0 != rmsFACNotAllowed {
		t.Errorf("r0 = %d, want RMS$_FAC (%d)", r0, rmsFACNotAllowed)
	}
}

// readByte is a small test helper matching create_test.go's own
// readWord/readLongword convention, added here because this is the first
// test file in this package that needs to read individual record bytes
// back out of VAX memory.
func readByte(t *testing.T, ctx *Context, addr uint32) byte {
	t.Helper()

	v, err := ctx.loadByte(addr)
	if err != nil {
		t.Fatalf("loadByte(%#x): %v", addr, err)
	}

	return v
}
