package rms

import (
	"bytes"
	"strings"
	"testing"
)

// testKeyAddr is where the keyed tests keep RAB$L_KBF's record number.
const testKeyAddr = 0x7000

// createRecords creates name with records (each one $PUT), of record
// format rfm (1 fixed, 2 variable) and, for fixed, size mrs.
func createRecords(t *testing.T, ctx *Context, name string, rfm byte, mrs uint16, records ...[]byte) {
	t.Helper()

	newFAB(t, ctx, "DUA0:"+name)
	putByte(t, ctx, testFabAddr+fabRFM, rfm)
	putWord(t, ctx, testFabAddr+fabMRS, mrs)

	if _, err := SysCreate(ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysCreate: %v", err)
	}

	connectRAB(t, ctx, testFabAddr)
	putByte(t, ctx, testRabAddr+rabRAC, racSeq)

	for _, record := range records {
		putRecord(t, ctx, testRabAddr, record)

		if r0, err := SysPut(ctx, []uint32{testRabAddr}); err != nil || r0 != rmsNormal {
			t.Fatalf("SysPut = %#x, %v", r0, err)
		}
	}

	if _, err := SysClose(ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysClose: %v", err)
	}
}

// getByKey does a $GET by relative record number n, with key size ksz,
// and returns its status and the record read.
func getByKey(t *testing.T, ctx *Context, n uint32, ksz byte) (uint32, string) {
	t.Helper()

	putByte(t, ctx, testRabAddr+rabRAC, racKey)
	putLongwordAt(t, ctx, testRabAddr+rabKBF, testKeyAddr)
	putByte(t, ctx, testRabAddr+rabKSZ, ksz)
	putLongwordAt(t, ctx, testKeyAddr, n)
	putLongwordAt(t, ctx, testRabAddr+rabUBF, testUserBufAddr)
	putWord(t, ctx, testRabAddr+rabUSZ, 512)

	r0, err := SysGet(ctx, []uint32{testRabAddr})
	if err != nil {
		t.Fatalf("SysGet: %v", err)
	}

	if r0&1 == 0 {
		return r0, ""
	}

	rsz := readWord(t, ctx, testRabAddr+rabRSZ)

	text, err := ctx.loadFixedString(testUserBufAddr, int(rsz))
	if err != nil {
		t.Fatalf("loadFixedString: %v", err)
	}

	return r0, text
}

// TestSysGetByRelativeRecordNumber: a fixed-length sequential file is
// read at random by record number (RAB$C_KEY), in any order, and a
// sequential $GET after one reads the record that follows it.
func TestSysGetByRelativeRecordNumber(t *testing.T) {
	f := newCreateFixture(t, true)

	records := [][]byte{[]byte("ONE.."), []byte("TWO.."), []byte("THREE")}
	createRecords(t, f.ctx, "RRN.DAT", 1, 5, records...)
	openAndConnectForRead(t, f.ctx, "RRN.DAT")

	for _, c := range []struct {
		n    uint32
		ksz  byte
		want string
	}{{3, 4, "THREE"}, {1, 0, "ONE.."}, {2, 4, "TWO.."}} {
		r0, got := getByKey(t, f.ctx, c.n, c.ksz)
		if r0 != rmsNormal || got != c.want {
			t.Errorf("record %d: %#x %q, want RMS$_NORMAL %q", c.n, r0, got, c.want)
		}
	}

	// Sequential from there: the record after record 2.
	putByte(t, f.ctx, testRabAddr+rabRAC, racSeq)

	if r0, err := SysGet(f.ctx, []uint32{testRabAddr}); err != nil || r0 != rmsNormal {
		t.Fatalf("sequential SysGet = %#x, %v", r0, err)
	}

	if got, _ := f.ctx.loadFixedString(testUserBufAddr, 5); got != "THREE" {
		t.Errorf("sequential $GET after record 2 read %q, want THREE", got)
	}
}

// TestSysGetByKeyStatuses: record 0, a record past the end, a key size
// other than 0 or 4, and keyed access to a file of variable-length
// records.
func TestSysGetByKeyStatuses(t *testing.T) {
	f := newCreateFixture(t, true)

	createRecords(t, f.ctx, "FIX.DAT", 1, 4, []byte("AAAA"), []byte("BBBB"))
	openAndConnectForRead(t, f.ctx, "FIX.DAT")

	for _, c := range []struct {
		name string
		n    uint32
		ksz  byte
		want uint32
	}{
		{"record 0", 0, 4, rmsInvalidKey},
		{"past the end", 3, 4, rmsEOF},
		{"key size 2", 1, 2, rmsInvalidKeySize},
	} {
		if r0, _ := getByKey(t, f.ctx, c.n, c.ksz); r0 != c.want {
			t.Errorf("%s: r0 = %#x, want %#x", c.name, r0, c.want)
		}

		if sts := readLongword(t, f.ctx, testRabAddr+rabSTS); sts != c.want {
			t.Errorf("%s: RAB$L_STS = %#x, want %#x", c.name, sts, c.want)
		}
	}

	if _, err := SysClose(f.ctx, []uint32{testFabAddr}); err != nil {
		t.Fatalf("SysClose: %v", err)
	}

	createRecords(t, f.ctx, "VAR.DAT", 2, 0, []byte("short"), []byte("longer one"))
	openAndConnectForRead(t, f.ctx, "VAR.DAT")

	if r0, _ := getByKey(t, f.ctx, 1, 4); r0 != rmsInvalidRAC {
		t.Errorf("variable-length file: r0 = %#x, want RMS$_RAC (%#x)", r0, rmsInvalidRAC)
	}
}

// TestTraceShowsKeyedGet: SET DEBUG RMS's line for a keyed $GET names the
// stream, the record number, the record, and the status by name.
func TestTraceShowsKeyedGet(t *testing.T) {
	f := newCreateFixture(t, true)

	createRecords(t, f.ctx, "TRC.DAT", 1, 4, []byte("WXYZ"))
	openAndConnectForRead(t, f.ctx, "TRC.DAT")

	r0, _ := getByKey(t, f.ctx, 1, 4)
	putLongwordAt(t, f.ctx, testRabAddr+rabRBF, testUserBufAddr)

	line := Trace(f.ctx, "SYS$GET", testRabAddr, r0)
	for _, want := range []string{"SYS$GET 00004000", "(file, access 02)", "RAC 1", "record 1", `"WXYZ"`, "RMS$_NORMAL/S"} {
		if !strings.Contains(line, want) {
			t.Errorf("Trace = %q, missing %q", line, want)
		}
	}

	if got := StatusName(rmsFACNotAllowed); got != "RMS$_FAC/F (00018514)" {
		t.Errorf("StatusName(RMS$_FAC) = %q", got)
	}
}

// TestFTNTerminalText: each FORTRAN carriage control character, with the
// line open or not.
func TestFTNTerminalText(t *testing.T) {
	for _, c := range []struct {
		record   string
		open     bool
		want     string
		wantOpen bool
	}{
		{" line", false, "line\n", false},
		{" line", true, "\nline\n", false},
		{"0after a blank", false, "\nafter a blank\n", false},
		{"1page", false, "\fpage\n", false},
		{"$>", false, ">", true},
		{"$>", true, "\n>", true},
		{"+ more", true, " more\n", false},
		{"+over", false, "over\n", false},
		{"\x00raw", true, "raw", true},
		{"Xodd", false, "odd\n", false},
		{"", false, "\n", false},
	} {
		got, open := ftnTerminalText([]byte(c.record), c.open)
		if string(got) != c.want || open != c.wantOpen {
			t.Errorf("ftnTerminalText(%q, %v) = %q, %v; want %q, %v", c.record, c.open, got, open, c.want, c.wantOpen)
		}
	}
}

// TestTerminalTextJoinsPromptAndReply: an FTN accessor's "$" prompt
// leaves the line open for a "+" record, a terminal read closes it, and
// another stream's record ends an open line first.
func TestTerminalTextJoinsPromptAndReply(t *testing.T) {
	ctx := &Context{Files: NewFileTable(nil)}
	ftn := indirectWord(1, ratFTN)

	var out bytes.Buffer

	out.Write(ctx.terminalText(ftn, []byte("$Your score is")))
	out.Write(ctx.terminalText(ftn, []byte("+   0 points.")))
	out.Write(ctx.terminalText(ftn, []byte("$>")))
	ctx.terminalRead()
	out.Write(ctx.terminalText(ftn, []byte(" Taken.")))
	out.Write(ctx.terminalText(ftn, []byte("$>")))
	out.Write(ctx.terminalText(0, []byte("%SYSTEM-F-ABORT")))

	want := "Your score is   0 points.\n>Taken.\n>\n%SYSTEM-F-ABORT\n"
	if out.String() != want {
		t.Errorf("terminal got %q, want %q", out.String(), want)
	}
}
