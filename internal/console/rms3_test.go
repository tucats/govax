package console

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/ods2/ondisk"
	"github.com/tucats/ods2/volume"
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

	test, err := vol.CreateDirectory(mfd, "TEST.DIR", volume.DirectoryOptions{}, bm, ib)
	if err != nil {
		t.Fatal(err)
	}

	for _, d := range []string{"SUB.DIR", "EMPTY.DIR"} {
		if _, err := vol.CreateDirectory(test, d, volume.DirectoryOptions{}, bm, ib); err != nil {
			t.Fatal(err)
		}
	}

	for _, d := range []string{"OUT.DIR", "CRE.DIR"} {
		if _, err := vol.CreateDirectory(mfd, d, volume.DirectoryOptions{}, bm, ib); err != nil {
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
		// A relative directory applies to the process default, not the
		// default name (VMS 7.3).
		20: "000184CC ",
		21: "0001C04A DUA0:[000000.SUB]C.DAT;",
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

// probeSummary is one line per service call in a probe's records: the
// case, the op, the status, and the NAM's expanded and resultant strings
// and FNB.
func probeSummary(recs []probeRecord) []string {
	var (
		out      []string
		sts      uint32
		nam, esa []byte
	)

	for _, r := range recs {
		switch r.Tag {
		case "STAT":
			sts = binary.LittleEndian.Uint32(r.Data)
			if r.Op == 6 {
				out = append(out, fmt.Sprintf("%d close %08X", r.Step, sts))
			}
		case "NAM_":
			nam = r.Data
		case "ESA_":
			esa = r.Data
		case "RSA_":
			es, rs := "", ""
			if n := int(nam[0xb]); n <= len(esa) {
				es = string(esa[:n])
			}

			if n := int(nam[3]); n <= len(r.Data) {
				rs = string(r.Data[:n])
			}

			out = append(out, fmt.Sprintf("%d op%d %08X es=%q rs=%q fnb=%08X fid=% X did=% X",
				r.Step, r.Op, sts, es, rs, binary.LittleEndian.Uint32(nam[0x34:]), nam[0x24:0x2a], nam[0x2a:0x30]))
		}
	}

	return out
}

// TestRMS3OpenCreate_govaxTree runs the OPEN, NAMFID, and CREATE probes
// against a govax-built copy of the oracle's tree and checks the
// statuses and resultant strings that don't depend on VMS's details.
func TestRMS3OpenCreate_govaxTree(t *testing.T) {
	for _, tc := range []struct {
		probe string
		want  []string
	}{
		{"open", []string{
			`1 op3 00010001 es="DUA0:[TEST]A.DAT;" rs="DUA0:[TEST]A.DAT;3"`,
			`2 op3 00010001 es="DUA0:[TEST]A.DAT;1" rs="DUA0:[TEST]A.DAT;1"`,
			`4 op3 00018292`,
			`5 op3 0001C04A`,
			`6 op3 00018744`,
			`9 op3 00010001 es="DUA0:[TEST]A.DAT;-1" rs="DUA0:[TEST]A.DAT;2"`,
		}},
		{"namfid", []string{
			`2 op3 00010001 es="" rs="" fnb=00000000 fid=13 00 01 00 00 00`,
			`3 op3 00010001 es="" rs="_GOVAX$DUA0:[000000]B.TXT;1"`,
			`4 op3 00018292`,
			`5 op3 00018292`,
			`6 op3 00010001 es="" rs="_GOVAX$DUA0:[TEST.SUB]C.DAT;1"`,
			`8 op3 00010001 es="DUA0:[TEST]A.DAT;" rs="DUA0:[TEST]A.DAT;3"`,
			`10 op3 00018744`,
		}},
		{"create", []string{
			`1 op4 00010001 es="DUA0:[CRE]N1.DAT;" rs="DUA0:[CRE]N1.DAT;1" fnb=00000046`,
			`2 op4 00010001 es="DUA0:[CRE]N1.DAT;" rs="DUA0:[CRE]N1.DAT;2" fnb=00004046`,
			`3 op4 00018282`,
			`4 op4 00010001 es="DUA0:[CRE]N1.DAT;" rs="DUA0:[CRE]N1.DAT;2"`,
			`6 op4 00010001 es="DUA0:[CRE]N2.DAT;3" rs="DUA0:[CRE]N2.DAT;3" fnb=00008047`,
			`8 op4 0001C04A`,
			`9 op4 00018744`,
			`10 op4 00010001 es="DUA0:[CRE]N4.LIS;" rs="DUA0:[CRE]N4.LIS;1"`,
		}},
	} {
		c := newBootableConsole(t)
		c.HostLibrary = t.TempDir()

		mountFreshRMSVolume(t, c)
		buildRMS3Tree(t, c)

		got := probeSummary(runRMS3Probe(t, c, tc.probe))

		for _, w := range tc.want {
			found := false

			for _, g := range got {
				if strings.HasPrefix(g, w) {
					found = true
				}
			}

			if !found {
				t.Errorf("%s: no call like %s", tc.probe, w)
			}
		}

		for _, g := range got {
			t.Logf("%s %s", tc.probe, g)
		}
	}
}

// TestRMS3XAB_govaxTree runs the XAB probe against a govax-built copy of
// the oracle's tree, and the CREATE probe's read-back, and checks the
// XABs that don't depend on VMS's details.
func TestRMS3XAB_govaxTree(t *testing.T) {
	c := newBootableConsole(t)
	c.HostLibrary = t.TempDir()

	mountFreshRMSVolume(t, c)
	buildRMS3Tree(t, c)

	recs := runRMS3Probe(t, c, "xab")

	byTag := map[string][]byte{}

	for _, r := range recs {
		if r.Step == 1 || r.Step == 5 || r.Step == 7 {
			byTag[fmt.Sprintf("%d %s", r.Step, r.Tag)] = r.Data
		}

		t.Logf("%s", r)
	}

	if sts := binary.LittleEndian.Uint32(byTag["1 STAT"]); sts != 0x10001 {
		t.Errorf("case 1: status %#x, want RMS$_NORMAL", sts)
	}

	if fhc := byTag["1 XFHC"]; fhc[0x8] != 2 || binary.LittleEndian.Uint32(fhc[0x10:]) != 1 {
		t.Errorf("case 1: XABFHC RFO %d, EBK %d, want VAR and 1", fhc[0x8], binary.LittleEndian.Uint32(fhc[0x10:]))
	}

	if sts := binary.LittleEndian.Uint32(byTag["5 STAT"]); sts&1 == 1 {
		t.Errorf("case 5 (an unknown XAB code): status %#x, want a failure", sts)
	}

	// The CREATE probe's read-backs: case 1's XABPRO, XABALL, and
	// expiration date, and case 7's revision date and number and
	// protection from its $CLOSE.
	c = newBootableConsole(t)
	c.HostLibrary = t.TempDir()

	mountFreshRMSVolume(t, c)
	buildRMS3Tree(t, c)

	got := map[string][]byte{}

	for _, r := range runRMS3Probe(t, c, "create") {
		got[fmt.Sprintf("%d %s", r.Step, r.Tag)] = r.Data
	}

	vms := func(t time.Time) uint64 {
		return uint64(t.Sub(time.Date(1858, time.November, 17, 0, 0, 0, 0, time.UTC))/time.Second) * 10_000_000
	}

	for _, tc := range []struct {
		key  string
		off  int
		size int
		want uint64
	}{
		{"11 RPRO", 8, 2, 0xFA00},
		{"11 RALL", 0x14, 2, 3},
		{"11 RDAT", 0x1c, 8, vms(time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC))},
		{"107 RRDT", 0xc, 8, vms(time.Date(2031, time.February, 2, 12, 0, 0, 0, time.UTC))},
		{"107 RRDT", 8, 2, 9},
		{"107 RPRO", 8, 2, 0xFF00},
	} {
		b := make([]byte, 8)
		copy(b, got[tc.key][tc.off:tc.off+tc.size])

		if v := binary.LittleEndian.Uint64(b); v != tc.want {
			t.Errorf("CREATE %s at %#x = %#x, want %#x", tc.key, tc.off, v, tc.want)
		}
	}
}
