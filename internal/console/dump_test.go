package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/rms"
)

// TestDump: DUMP's block and record layouts (VMS 7.3's, testdata/dcl50's
// rounds 4 and 6, which the round's oracles compare whole), its ranges,
// and its failures (Phase 51).
func TestDump(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.cmd().SetDefault("DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	records := [][]byte{[]byte("first"), []byte(strings.Repeat("x", 40)), {}, []byte("last")}
	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "D.DAT"}, rms.TextRecords, records); err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	if err := d.Dispatch("DUMP/RECORD=(START=2,COUNT=2) D.DAT"); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(buf.String(), "\n")
	want := []string{
		"\f",
		"Dump of file DUA0:[000000]D.DAT;1 on ",
		"File ID",
		"",
		"Record number 2 (00000002), 40 (0028) bytes, RFA(0001,0000,0008)",
		"",
		" 78787878 78787878 78787878 78787878 78787878 78787878 78787878 78787878 xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx 000000",
		"                                                       78787878 78787878 xxxxxxxx........................ 000020",
		"",
		"Record number 3 (00000003), 0 (0000) bytes, RFA(0001,0000,0032)",
		"",
		"",
	}

	for i, w := range want {
		if i >= len(lines) || (lines[i] != w && !strings.HasPrefix(lines[i], w)) || (w == "" && lines[i] != "") {
			t.Fatalf("DUMP/RECORD line %d: %q, want %q\n%s", i, lines[min(i, len(lines)-1)], w, buf.String())
		}
	}

	buf.Reset()

	if err := d.Dispatch("DUMP D.DAT"); err != nil {
		t.Fatal(err)
	}

	if got := buf.String(); !strings.Contains(got, "\nVirtual block number 1 (00000001), 512 (0200) bytes\n\n 78787878 ") ||
		!strings.Contains(got, " 78780028 00747372 69660005 ..first.(.xxxxxxxxxxxxxxxxxxxxxx 000000\n") ||
		strings.Count(got, "\n 00000000 00000000") < 10 || strings.Contains(got, "block number 2") {
		t.Errorf("DUMP:\n%s", got)
	}

	for _, line := range []string{"DUMP NOSUCH.DAT", "DUMP *.DAT", "DUMP/BLOCK=(BOGUS=1) D.DAT", "DUMP/BLOCK=START=X D.DAT"} {
		if err := d.Dispatch(line); err == nil {
			t.Errorf("%s succeeded", line)
		}
	}

	if err := d.Dispatch("DUMP NOSUCH.DAT"); err == nil || !strings.HasPrefix(err.Error(), "DUMP-E-OPENIN, error opening DUA0:[000000]NOSUCH.DAT; as input\n-RMS-E-FNF") {
		t.Errorf("DUMP of a missing file: %v", err)
	}

	if r, err := dclcmd.ParseDumpRange("(START=3,END=9)"); err != nil || r != (dclcmd.DumpRange{Start: 3, End: 9}) {
		t.Errorf("ParseDumpRange = %+v, %v", r, err)
	}
}
