package console

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/rms"
)

// Phase 113's probe round (docs/planning/PHASE-113 - Fortran errata.md, subtask
// 12; testdata/err113's README): ERR113.COM asks VMS what the phase's earlier
// subtasks left unconfirmed (STR$'s arithmetic, the UTC and device information
// services, LIB$INITIALIZE, MACRO's questions, DCL's messages, TYPE without
// carriage control, a full volume, and the index file past its header).
// TestErr113Procedure runs it on govax and holds its log to VMS's,
// testdata/err113/vax/err113.log, section by section (subtask 13c), but for
// err113NotCompared and err113Differences, and the arguments' values
// (err113Masked).

// runErr113 makes the round's volume as exchange.cmd does, and a scratch volume as
// INITIALIZE/HEADERS=16 makes it on the VAX, and runs ERR113.COM with the volume on
// DUA1 and the scratch volume on DUA3. It returns the log and STR113.TXT.
func runErr113(t *testing.T) (string, string) {
	t.Helper()

	d, c, buf := shareFindConsole(t, false)
	dir := t.TempDir()
	exchange := filepath.Join(dir, "err113.dsk")
	scratch := filepath.Join(dir, "scratch.dsk")

	shareFindRun(t, d, `INITIALIZE/CONTAINER "`+exchange+`" /DEVICE=RD54 ERR113`)
	shareFindRun(t, d, `INITIALIZE/CONTAINER "`+scratch+`" /DEVICE=RD51 /HEADERS=16 SCR113`)

	if err := c.cmd().Mount("DUA5:", exchange, true); err != nil {
		t.Fatal(err)
	}

	t.Chdir(filepath.Join("..", ".."))

	script, err := os.ReadFile(filepath.Join("testdata", "err113", "exchange.cmd"))
	if err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	for line := range strings.SplitSeq(string(script), "\n") {
		if line == "" || strings.HasPrefix(line, "!") || strings.HasPrefix(line, "INITIALIZE/") ||
			strings.HasPrefix(line, "MOUNT/") || strings.HasPrefix(line, "DISMOUNT") {
			continue
		}

		err := d.Dispatch(line)
		if out := buf.String(); err != nil || out != "" {
			t.Fatalf("%s returned %v and said:\n%.600s", line, err, out)
		}
	}

	// On the VAX the volume is DUA1 and the scratch disk DUA3.
	shareFindRun(t, d, "SET DEFAULT DUA2:[000000]")
	shareFindRun(t, d, "DISMOUNT DUA5")
	shareFindRun(t, d, "DISMOUNT DUA1")

	if err := c.cmd().Mount("DUA1:", exchange, true); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount("DUA3:", scratch, true); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, "SET DEFAULT DUA1:[000000]")
	shareFindRun(t, d, "@ERR113/OUTPUT=ERR113.LOG")

	read := func(spec, name string) string {
		host := filepath.Join(t.TempDir(), name)
		shareFindRun(t, d, `COPY/QUIET `+spec+` "`+host+`"/HOST`)

		data, err := os.ReadFile(host)
		if err != nil {
			t.Fatal(err)
		}

		return strings.ReplaceAll(string(data), "\r", "")
	}

	return read("DUA1:[000000]ERR113.LOG", "err113.log"), read("DUA1:[ERR113]STR113.TXT", "str113.txt")
}

func TestErr113Procedure(t *testing.T) {
	if testing.Short() {
		t.Skip("the round's procedure makes some 3,000 files")
	}

	log, str113 := runErr113(t)
	sections := vms93Sections(log)

	for _, name := range []string{"STR113", "SVC113", "INI113", "INI113NT", "MAC113A", "MAC113B", "MAC113C",
		"MAC113D", "DCL", "NOCC", "FULL", "INDEX", "END"} {
		if _, ok := sections[name]; !ok {
			t.Errorf("no section %s in the log:\n%s", name, log)
		}
	}

	// STR113's output went to its own file, with the MATCH_WILD cases.
	if n := strings.Count(str113, "\nWILD "); n != 12 {
		t.Errorf("STR113.TXT has %d WILD cases, want 12:\n%s", n, str113)
	}

	vmsLog, err := os.ReadFile(filepath.Join("testdata", "err113", "vax", "err113.log"))
	if err != nil {
		t.Fatal(err)
	}

	vms := vms93Sections(strings.ReplaceAll(string(vmsLog), "\r", ""))

	for name, why := range err113NotCompared {
		if _, ok := vms[name]; !ok {
			t.Errorf("no section %s in VMS's log (err113NotCompared: %s)", name, why)
		}
	}

	// The index file grew as VMS's did (subtask 13d): 3,000 files and an
	// INDEXF.SYS of VMS's size, with the block it grew by in FULL for
	// MORE.TXT's header (REST.DAT left it free, subtask 14b3).
	if !strings.Contains(sections["INDEX"], "FILES 3000 STATUS %X00000001\n") {
		t.Errorf("INDEX didn't make 3,000 files:\n%s", sections["INDEX"])
	}

	if got, want := err113IndexBlocks(t, sections["INDEX"]), err113IndexBlocks(t, vms["INDEX"]); got != want {
		t.Errorf("INDEXF.SYS is %d blocks, want VMS's %d:\n%s", got, want, sections["INDEX"])
	}

	for name, want := range vms {
		if _, skip := err113NotCompared[name]; skip {
			continue
		}

		for _, d := range err113Differences[name] {
			if !strings.Contains(want, d.vms) {
				t.Errorf("%s: VMS's section has no %q (err113Differences: %s)", name, d.vms, d.why)
			}

			want = strings.Replace(want, d.vms, d.govax, 1)
		}

		got := err113Masked(sections[name])
		if want = err113Masked(want); got != want {
			t.Errorf("%s is\n%s\nVMS's (with err113Differences)\n%s", name, got, want)
		}
	}
}

// err113NotCompared are the sections of the round's log TestErr113Procedure
// leaves to other tests, with why.
var err113NotCompared = map[string]string{
	// govax's MACRO shows its messages its own way (DEVIATIONS.md), and its
	// status for errors is govax's ASMERRORS; TestErr113Macro holds the
	// listings, the order of the messages, and the objects to VMS's.
	"MAC113A": "MACRO's messages are govax's own, the listing TestErr113Macro's",
	"MAC113B": "MACRO's messages are govax's own, their order TestErr113Macro's",
	// govax has no DUMP/HEADER or ANALYZE/DISK_STRUCTURE; TestErr113Procedure
	// compares the index file's size, and ods2's TestIndexFileGrowsAsVMS its
	// extents.
	"INDEX": "no DUMP/HEADER or ANALYZE/DISK_STRUCTURE; the size compared alone",
}

// err113Difference is text of VMS's section and what govax shows in its place.
type err113Difference struct {
	vms, govax, why string
}

// err113Differences are where govax's log differs from VMS's, section by
// section, each with why.
var err113Differences = map[string][]err113Difference{
	// VMS's log is govax's copy of the VAX's file, in which TYPE's records
	// have no carriage control: govax's COPY to the host runs them and the
	// next line together. At the VAX's terminal ONE and TWO were one line
	// and AFTER a line of its own, as govax's TYPE shows them.
	"NOCC": {{vms: "ONETWOAFTER\n", govax: "ONETWO\nAFTER\n", why: "records with no carriage control"}},
	"FULL": {
		// The third pass initialized the scratch volume; the test mounts
		// govax's first.
		{vms: "%MOUNT-I-MOUNTED, SCR113 mounted on _SIMVAX$DUA3:\n", why: "the scratch volume made by the test"},
		// The third pass ran with SET VERIFY on, so CREATE's data line is
		// in VMS's log.
		{vms: "A line for a full volume.\n", why: "SET VERIFY's data line"},
	},
}

// err113IndexSize is DIRECTORY/SIZE=ALL's line for INDEXF.SYS.
var err113IndexSize = regexp.MustCompile(`INDEXF\.SYS;1 +([0-9]+)/`)

// err113IndexBlocks is INDEXF.SYS's size in an INDEX section.
func err113IndexBlocks(t *testing.T, section string) int {
	t.Helper()

	m := err113IndexSize.FindStringSubmatch(section)
	if m == nil {
		t.Fatalf("no INDEXF.SYS size in:\n%s", section)
	}

	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}

	return n
}

// err113Values are the values LIB$INITIALIZE's routines and the main program
// show of their arguments but the link flags (argument 5): addresses, and the
// CLI's coroutine and flags, which govax hasn't (0).
var err113Values = regexp.MustCompile(`(?m)^(  [1-46]: )[0-9A-F]{8}$`)

// err113Masked is a section of the round's log with err113Values masked.
func err113Masked(section string) string {
	return err113Values.ReplaceAllString(section, "${1}(value)")
}

// mac113Allowed are the lines of VMS's MAC113 listings govax's may differ in,
// each with why (docs/planning/PHASE-113 - Fortran errata.md, subtask 13c).
var mac113Allowed = map[string]map[string]string{
	// Round 2: the zero a register an operator left stores is marked
	// relocatable in VMS's listing, as MAC113A's R2's is; govax's is plain.
	"MAC113F": {
		"                           00000000' 0004       ": "R1's zero marked relocatable",
		"                           00000000' 0010       ": "R2's zero marked relocatable",
		"                               0000' 0016       ": "R3's zero marked relocatable",
	},
	"MAC113A": {
		// VMS's object stores R2's zero by an STO_L of nothing it stacked,
		// so its listing marks the zero relocatable; govax's is a plain 0.
		"                  00000003 00000000' 000F       ": "R2's zero marked relocatable",
		// .ADDRESS R3's two messages: VMS's "!"s stand under columns no rule
		// of its scanner explains (7 and 12); govax's under R3.
		"%MACRO-E-ADRLSTSYNX, Address list syntax error         !": "the column of .ADDRESS's ADRLSTSYNX",
		"%MACRO-E-DIRSYNX, Directive syntax error                    !": "the column of .ADDRESS's DIRSYNX",
	},
}

// mac113Pages is a listing's lines up to its performance indicators, without
// each page's two heading lines, which name the assembler and the times.
func mac113Pages(text string) []string {
	var out []string

	lines := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.Contains(line, "Performance indicators") {
			break
		}

		if strings.HasPrefix(line, "\f") || strings.Contains(line, "  Page ") {
			i++ // and the second heading line

			continue
		}

		out = append(out, line)
	}

	return out
}

// mac113Dates masks a DST$K_SRC_DECLFILE's creation date in an object's dump:
// the files are the same, made at other times.
var mac113Dates = regexp.MustCompile(`(01 2f 00 0[0-9] 00)( [0-9a-f]{2}){8}`)

// mac113Dump is an object's records as obj.Dump shows them, but for its
// header records and the source files' dates.
func mac113Dump(t *testing.T, records [][]byte) []string {
	t.Helper()

	m, err := obj.Decode(records)
	if err != nil {
		t.Fatal(err)
	}

	var b strings.Builder
	if err := obj.Dump(&b, m); err != nil {
		t.Fatal(err)
	}

	var out []string

	header := false

	for line := range strings.SplitSeq(b.String(), "\n") {
		if strings.Contains(line, ". HDR ") {
			header = true

			continue
		}

		if header && strings.HasPrefix(line, " ") {
			continue
		}

		header = false

		out = append(out, mac113Dates.ReplaceAllString(line, "$1 (date)"))
	}

	return out
}

// TestErr113Macro holds govax's MACRO to VMS 7.1's on the probe round's MAC113
// sections (testdata/err113; Phase 113, subtask 13c): register names as data
// (DIRSYNX, DATALSTSYN, ADRLSTSYNX), .WARN, .ERROR, and .PRINT shown in their
// lines' order, MACRO/DEBUG A+B (each file's lines numbered from 1, and each
// file declared to the debugger), and an .END before the last file. The
// listings are compared up to their performance indicators, but for their
// headings and mac113Allowed; the A+B objects record for record, but for
// their header records and the source files' dates.
func TestErr113Macro(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)
	dir := filepath.Join("..", "..", "testdata", "err113")

	shareFindRun(t, d, "CREATE/DIRECTORY DUA1:[ERR113]")
	shareFindRun(t, d, "SET DEFAULT DUA1:[ERR113]")

	for _, name := range []string{"mac113a", "mac113b", "mac113c1", "mac113c2", "mac113d1", "mac113d2", "mac113f"} {
		abs, err := filepath.Abs(filepath.Join(dir, name+".mar"))
		if err != nil {
			t.Fatal(err)
		}

		shareFindRun(t, d, `COPY/QUIET "`+abs+`"/HOST `+strings.ToUpper(name)+`.MAR`)
	}

	said := map[string]string{}

	for _, tc := range []struct{ name, command, vax string }{
		{"MAC113A", "MACRO/LIST MAC113A", "vax"},
		{"MAC113B", "MACRO/LIST MAC113B", "vax"},
		{"MAC113C", "MACRO/DEBUG/LIST=MAC113C/OBJECT=MAC113C MAC113C1+MAC113C2", "vax"},
		{"MAC113D", "MACRO/LIST=MAC113D/OBJECT=MAC113D MAC113D1+MAC113D2", "vax"},
		{"MAC113F", "MACRO/LIST MAC113F", filepath.Join("vax", "round2")}, // round 2
	} {
		buf.Reset()
		_ = d.Dispatch(tc.command)
		said[tc.name] = buf.String()

		listing, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: tc.name + ".LIS"}, rms.TextRecords)
		if err != nil {
			t.Fatalf("%s: no listing: %v", tc.name, err)
		}

		vms, err := os.ReadFile(filepath.Join(dir, tc.vax, strings.ToLower(tc.name)+".lis"))
		if err != nil {
			t.Fatal(err)
		}

		got, want := mac113Pages(string(joinRecords(listing))), mac113Pages(string(vms))
		if len(got) != len(want) {
			t.Errorf("%s: the listing has %d lines, VMS's %d:\n%s", tc.name, len(got), len(want), strings.Join(got, "\n"))

			continue
		}

		for i := range want {
			if got[i] != want[i] && mac113Allowed[tc.name][want[i]] == "" {
				t.Errorf("%s: listing line %d is\n %q\nVMS's\n %q", tc.name, i+1, got[i], want[i])
			}
		}
	}

	// .WARN, .ERROR, and .PRINT in their lines' order, as VMS's MACRO showed them.
	var order []string

	for line := range strings.SplitSeq(said["MAC113B"], "\n") {
		switch {
		case strings.Contains(line, "GENWRN"), strings.Contains(line, "GENERR"):
			order = append(order, line[strings.LastIndex(line, ": ")+2:])
		case strings.HasPrefix(line, "3  "):
			order = append(order, line)
		}
	}

	if want := []string{"1  a warning's text", "2  an error's text", "3  a printed line", " a warning with no expression"}; !slices.Equal(order, want) {
		t.Errorf("MAC113B said, in this order, %q, want %q:\n%s", order, want, said["MAC113B"])
	}

	for _, name := range []string{"MAC113C", "MAC113D"} {
		records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: name + ".OBJ"}, rms.VariableRecords)
		if err != nil {
			t.Fatal(err)
		}

		vmsObj, err := filepath.Abs(filepath.Join(dir, "vax", strings.ToLower(name)+".obj"))
		if err != nil {
			t.Fatal(err)
		}

		vmsRecords, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Host: true, Name: vmsObj}, rms.VariableRecords)
		if err != nil {
			t.Fatal(err)
		}

		if got, want := mac113Dump(t, records), mac113Dump(t, vmsRecords); !slices.Equal(got, want) {
			t.Errorf("%s's object isn't VMS's:\n%s\nVMS's:\n%s", name, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
}
