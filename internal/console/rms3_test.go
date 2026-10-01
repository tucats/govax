package console

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/ods2/ondisk"
)

// The Phase 33 oracle's probes (testdata/mar/rms3), run under govax.

// probeRecord is one record a probe wrote: its tag, case number, the
// service called, and the bytes.
type probeRecord struct {
	Tag  string
	Step uint32
	Op   uint32
	Data []byte
}

func (r probeRecord) String() string {
	return fmt.Sprintf("%s %d op%d % X", r.Tag, r.Step, r.Op, r.Data)
}

// decodeProbe splits a probe's records.
func decodeProbe(t *testing.T, records [][]byte) []probeRecord {
	t.Helper()

	out := make([]probeRecord, 0, len(records))

	for _, r := range records {
		if len(r) < 12 {
			t.Fatalf("probe record of %d bytes", len(r))
		}

		out = append(out, probeRecord{
			Tag:  string(r[:4]),
			Step: binary.LittleEndian.Uint32(r[4:]),
			Op:   binary.LittleEndian.Uint32(r[8:]),
			Data: r[12:],
		})
	}

	return out
}

// buildRMS3Tree makes BUILD.COM's test tree on DUA0 with govax: the
// directories, and the files with their versions.
func buildRMS3Tree(t *testing.T, c *Console) {
	t.Helper()

	vol, ok := c.Mounts.Lookup("DUA0")
	if !ok {
		t.Fatal("DUA0 isn't mounted")
	}

	mfd, err := vol.OpenDirectory(ondisk.MasterFileDirectoryFid)
	if err != nil {
		t.Fatal(err)
	}

	bm, err := mfd.Device.Bitmap()
	if err != nil {
		t.Fatal(err)
	}

	ib, err := mfd.Device.IndexBitmap()
	if err != nil {
		t.Fatal(err)
	}

	test, err := vol.CreateDirectory(mfd, "TEST.DIR", 0, bm, ib)
	if err != nil {
		t.Fatal(err)
	}

	for _, d := range []string{"SUB.DIR", "EMPTY.DIR"} {
		if _, err := vol.CreateDirectory(test, d, 0, bm, ib); err != nil {
			t.Fatal(err)
		}
	}

	for _, d := range []string{"OUT.DIR", "CRE.DIR"} {
		if _, err := vol.CreateDirectory(mfd, d, 0, bm, ib); err != nil {
			t.Fatal(err)
		}
	}

	for _, f := range []struct{ name, text string }{
		{"[TEST]A.DAT", "A.DAT, the first version"},
		{"[TEST]A.DAT", "A.DAT, the second version\nwith two lines"},
		{"[TEST]A.DAT", "A.DAT, the third version"},
		{"[TEST]B.TXT", "B.TXT holds several lines,\nso that its end-of-file block and"},
		{"[TEST]AB.DAT", "AB.DAT"},
		{"[TEST]C.DAT", "[TEST]C.DAT"},
		{"[TEST.SUB]C.DAT", "[TEST.SUB]C.DAT"},
		{"[TEST.SUB]D.DAT", "[TEST.SUB]D.DAT"},
		{"[TEST]F.DAT", ""},
	} {
		var records [][]byte
		for _, l := range strings.Split(f.text, "\n") {
			if l != "" {
				records = append(records, []byte(l))
			}
		}

		if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA0:" + f.name}, rms.VariableRecords, records); err != nil {
			t.Fatalf("creating %s: %v", f.name, err)
		}
	}
}

// runRMS3Probe assembles, links, and runs a probe with DUA0:[000000] as
// the default directory and RUN.COM's logical names, and returns what it
// wrote.
func runRMS3Probe(t *testing.T, c *Console, name string) []probeRecord {
	t.Helper()

	src := filepath.Join("..", "..", "testdata", "mar", "rms3", name+".mar")
	dir := t.TempDir()

	if err := c.Macro(MacroOptions{Source: src, Object: filepath.Join(dir, name+".obj")}); err != nil {
		t.Fatalf("MACRO: %v", err)
	}

	if err := c.Link(LinkOptions{Objects: []string{filepath.Join(dir, name)}}); err != nil {
		t.Fatalf("LINK: %v", err)
	}

	if err := c.SetDefault("DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	for n, eqv := range map[string][]string{
		"TST": {"DUA0:[TEST]"},
		"TSL": {"DUA0:[TEST.SUB]", "DUA0:[TEST]"},
	} {
		var e []lnm.Equivalence
		for _, v := range eqv {
			e = append(e, lnm.Equivalence{Value: v})
		}

		if _, err := c.Logicals.Define(lnm.ProcessTableName, n, lnm.Supervisor, 0, e); err != nil {
			t.Fatal(err)
		}
	}

	if r0 := runImage(t, c, filepath.Join(dir, name+".exe")); r0&1 != 1 {
		t.Fatalf("R0 = %#x, want a success status", r0)
	}

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA0:[OUT]" + strings.ToUpper(name) + ".DMP"}, rms.VariableRecords)
	if err != nil {
		t.Fatal(err)
	}

	return decodeProbe(t, records)
}

// expandedStrings returns each case's status and expanded string, from a
// probe's STAT, NAM_, and ESA_ records.
func expandedStrings(recs []probeRecord) map[uint32]string {
	out := map[uint32]string{}

	var sts uint32

	esl := 0

	for _, r := range recs {
		switch r.Tag {
		case "STAT":
			sts = binary.LittleEndian.Uint32(r.Data)
		case "NAM_":
			esl = int(r.Data[0xb])
		case "ESA_":
			out[r.Step] = fmt.Sprintf("%08X %s", sts, r.Data[:esl])
		}
	}

	return out
}

// TestRMS3Parse_govaxTree runs the PARSE probe against a govax-built copy
// of the oracle's tree and checks the expanded strings name processing
// gives.
func TestRMS3Parse_govaxTree(t *testing.T) {
	c := newBootableConsole(t)
	c.HostLibrary = t.TempDir()

	mountFreshRMSVolume(t, c)
	buildRMS3Tree(t, c)

	got := expandedStrings(runRMS3Probe(t, c, "parse"))

	for step, want := range map[uint32]string{
		1:  "00010001 DUA0:[TEST]A.DAT;",
		2:  "00010001 DUA0:[TEST]A.DAT;",
		3:  "00010001 DUA0:[TEST]*.DAT;*",
		4:  "00010001 DUA0:[TEST...]*.*;",
		5:  "00010001 DUA0:[TEST.SUB]C.DAT;",
		11: "00010001 DUA0:[TEST]A.DAT;",
		12: "00010001 DUA0:[TEST.SUB]C.DAT;",
		15: "00010001 DUA0:[TEST]A.DAT;",
		16: "00010001 DUA0:[TEST]A.DAT;2",
		20: "00010001 DUA0:[TEST]A.DAT;",
		21: "00010001 DUA0:[TEST.SUB]C.DAT;",
		25: "00010001 DUA0:[TEST].;",
		26: "00010001 DUA0:[000000]A.DAT;",
	} {
		if !strings.HasPrefix(got[step], want) {
			t.Errorf("case %d: got %q, want %q", step, got[step], want)
		}
	}

	for step := uint32(1); step <= 30; step++ {
		t.Logf("%2d: %s", step, got[step])
	}
}

// resultantStrings returns each $SEARCH call's status and resultant
// string, in order, from a probe's STAT, NAM_, and RSA_ records.
func resultantStrings(recs []probeRecord) []string {
	var (
		out []string
		sts uint32
		rsl int
	)

	for _, r := range recs {
		switch r.Tag {
		case "STAT":
			sts = binary.LittleEndian.Uint32(r.Data)
		case "NAM_":
			rsl = int(r.Data[3])
		case "RSA_":
			if r.Op == 2 {
				s := ""
				if sts&1 == 1 {
					s = string(r.Data[:rsl])
				}

				out = append(out, fmt.Sprintf("%d %08X %s", r.Step, sts, s))
			}
		}
	}

	return out
}

// TestRMS3Search_govaxTree runs the SEARCH probe against a govax-built
// copy of the oracle's tree and checks what each search returns.
func TestRMS3Search_govaxTree(t *testing.T) {
	c := newBootableConsole(t)
	c.HostLibrary = t.TempDir()

	mountFreshRMSVolume(t, c)
	buildRMS3Tree(t, c)

	got := resultantStrings(runRMS3Probe(t, c, "search"))

	want := map[string]bool{
		"101 00010001 DUA0:[TEST]A.DAT;3":     true,
		"102 00010001 DUA0:[TEST]A.DAT;2":     true,
		"201 00010001 DUA0:[TEST]A.DAT;3":     true,
		"202 000182CA ":                       true,
		"401 00018292 ":                       true,
		"601 00010001 DUA0:[TEST.SUB]C.DAT;1": true,
		"1001 00010001 DUA0:[TEST]A.DAT;2":    true,
		"1101 00010001 DUA0:[TEST]A.DAT;3":    true,
	}

	seen := map[string]bool{}
	for _, g := range got {
		seen[g] = true

		t.Log(g)
	}

	for w := range want {
		if !seen[w] {
			t.Errorf("no %q among the searches", w)
		}
	}
}
