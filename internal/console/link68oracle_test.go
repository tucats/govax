package console

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/govax/internal/vmsimage"
)

// docs/PHASE-68 - link shareable images.md, subtask 13: the milestone.
// TestLink68Oracle runs testdata/link68's LINK68.COM on govax, as the
// author ran it on VMS V7.1: MACRO/LIST of every source, LINK/SHAREABLE
// and LINK with full maps, ANALYZE/IMAGE of every image, the runs R1 to
// R12, and the error links E1 to E9. Then it copies the results out as
// copyout.cmd copied VMS's, and holds each to VMS's: the log line for
// line, the maps, analyses, listings, and library listing line for line
// (times and the tools' names and versions masked), and the objects and
// images byte for byte but for their times and the tools' names. What
// still differs is listed: link68LogDifferences, link68TextDifferences,
// and the files compared only in part (link68FileNotes).

// link68Dir holds the probe.
var link68Dir = filepath.Join("..", "..", "testdata", "link68")

// link68TimeIdents are the probe's shareable images linked without
// GSMATCH=, whose idents come from the link time (and so differ from
// VMS's), and the programs linked against them, whose global section
// ISDs hold those idents. E1 is SHR1 with UNIVERSAL=NOSUCH and no
// GSMATCH=.
var link68TimeIdents = map[string]bool{
	"shr1a": true, "shr1a-2": true, "shr3": true, "shr3p": true, "shr5": true, "shr6": true, "e1": true,
	"main1a": true, "main3": true, "main3p": true, "main6": true,
}

// link68Difference is one expected difference in the log or a text
// file: the lines (VMS's, after masking) it covers, and why.
type link68Difference struct {
	line *regexp.Regexp
	why  string
}

// link68LogDifferences are the lines of VMS's log, masked
// (link68MaskLog), that govax's needn't match.
var link68LogDifferences = []link68Difference{
	{regexp.MustCompile(`^V7\.1 +VAX 8600$`), "F$GETSYI's VERSION and HW_NAME: govax says V7.3 and names itself"},
	{regexp.MustCompile(`^%LINK-|^\t(in module|which is|at %X)`), "E3's messages: govax refuses a shareable image named as an object (DCL-E-LINKING) where VMS's LINK reads its blocks as records (RECTYP, SEQNCE, NOEOM, ERRORS, NOIMGFIL, UDFSYM, ILLRECTYP)"},
	{regexp.MustCompile(`^  \$STATUS == "%X106482B2"$`), "E3's status, as its messages"},
}

// link68E3 is E3's LINK command line, whose messages differ.
const link68E3 = "$ LINK/NOMAP/EXECUTABLE=E3 MAIN2.OBJ,SHR2.EXE"

// link68Dates are the dates and times in the log and the text files.
var link68Dates = regexp.MustCompile(`[ 0-9]\d-[A-Z]{3}-\d{4}( \d\d:\d\d(:\d\d(\.\d\d)?)?)?`)

// link68Origins are the run's addresses that depend on where SHR1 and
// SHR3P landed. VMS put LIBRTL's 279 pages before them, where govax's
// shims take none; MAIN3's SHR3, based at ^X300000, is the same in both.
var link68Origins = regexp.MustCompile(`(SHR1_ORIGIN|SHR3_GET\(\)|SHR3_DATA) 000[0-2][0-9A-F]{4}`)

// link68MaskLog masks what differs in every run of the log, VMS's or
// govax's: dates and times (DIRECTORY's), and the addresses
// link68Origins finds. VMS's SYS$SHARE is SYS$SYSROOT:[SYSLIB] on a
// rooted SYS$SYSROOT, which govax doesn't have: the test's SYS$SYSROOT
// is DUA0:, so VMS's root becomes DUA0:.
func link68MaskLog(text string) []string {
	text = strings.ReplaceAll(text, "\r", "")
	text = strings.ReplaceAll(text, "SIMVAX$DUA0:[SYS0.SYSCOMMON.]", "DUA0:")
	text = link68Dates.ReplaceAllString(text, "<date>")
	text = link68Origins.ReplaceAllString(text, "$1 <address>")

	return strings.Split(strings.TrimRight(text, "\n"), "\n")
}

// link68Govax is a console with the probe's sources and LINK68.COM on
// an RD51 in DUA1, as the exchange volume had them (exchange.cmd), the
// default directory. SYS$SHARE is SYS$SYSROOT:[SYSLIB] in the system
// table, as on VMS, and SYS$SYSROOT is DUA0: (nothing mounted there).
func link68Govax(t *testing.T, paging bool) (*Dispatcher, *bytes.Buffer) {
	t.Helper()

	if paging {
		withSettings(t, map[string]string{machine.PagingSetting: "true"})
	}

	c, buf := newTestConsole(t)
	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)
	installPlainDebugger(c)

	if err := c.RunHostProcedure("vax.init", d.Dispatch); err != nil {
		t.Fatalf("vax.init: %v", err)
	}

	path := filepath.Join(t.TempDir(), "link68.dsk")
	if err := c.cmd().InitializeContainer(path, 21600, "LINK68", 0, "RD51"); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount("DUA1:", path, true); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{
		"xfr1.mar", "shr1.mar", "shr2.mar", "shr3.mar", "shr6.mar", "main1.mar", "main2.mar",
		"main3.mar", "main6.mar", "exe1.mar", "exe1b.mar", "shr1.opt", "link68.com",
	} {
		shareFindRun(t, d, `COPY "`+filepath.Join(link68Dir, name)+`"/HOST DUA1:[000000]`+strings.ToUpper(name))
	}

	shareFindRun(t, d, "SET DEFAULT DUA1:[000000]")
	shareFindRun(t, d, "DEFINE/SYSTEM/EXECUTIVE_MODE SYS$SYSROOT DUA0:")
	shareFindRun(t, d, "DEFINE/SYSTEM/EXECUTIVE_MODE SYS$SHARE SYS$SYSROOT:[SYSLIB]")
	buf.Reset()

	return d, buf
}

// link68CopyOut copies the run's results into a new directory as
// copyout.cmd copied VMS's into testdata/link68/vax, and returns it.
// The error links that made no image have nothing to copy.
func link68CopyOut(t *testing.T, d *Dispatcher) string {
	t.Helper()

	dir := t.TempDir()

	script, err := os.ReadFile(filepath.Join(link68Dir, "copyout.cmd"))
	if err != nil {
		t.Fatal(err)
	}

	for line := range strings.SplitSeq(string(script), "\n") {
		if strings.HasPrefix(line, "COPY ") {
			_ = d.Dispatch(strings.ReplaceAll(line, "testdata/link68/vax/", dir+string(filepath.Separator)))
		}
	}

	return dir
}

// TestLink68Oracle runs LINK68.COM on govax, in both paging modes, and
// holds the log and every file it makes to VMS's.
func TestLink68Oracle(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, buf := link68Govax(t, paging)

		if err := d.Dispatch("@LINK68/OUTPUT=LINK68.LOG"); err != nil && !vmserrors.MessageInhibited(err) {
			t.Fatalf("@LINK68: %v\n%s", err, buf)
		}

		out := link68CopyOut(t, d)

		// Nothing the run did leaves a file open: the failed activations
		// (R2, R3, R5's mismatches, E7's NOTFR) let their images go.
		shareFindRun(t, d, "DISMOUNT DUA1")

		t.Run("log", func(t *testing.T) { link68CompareLog(t, out) })

		names, err := os.ReadDir(filepath.Join(link68Dir, "vax"))
		if err != nil {
			t.Fatal(err)
		}

		for _, entry := range names {
			// round2 is the second round's (TestLink68Round2).
			if name := entry.Name(); name != "link68.log" && !entry.IsDir() {
				t.Run(name, func(t *testing.T) { link68CompareFile(t, out, name) })
			}
		}
	})
}

// link68CompareLog holds govax's LINK68.LOG to VMS's, line for line,
// masked by link68MaskLog, but for link68LogDifferences. govax's COPY
// says what it copied, which VMS's doesn't without /LOG: that line is
// left out.
func link68CompareLog(t *testing.T, out string) {
	want := link68MaskLog(link68Read(t, filepath.Join(link68Dir, "vax", "link68.log")))

	var got []string

	for _, line := range link68MaskLog(link68Read(t, filepath.Join(out, "link68.log"))) {
		if !strings.HasPrefix(line, "%COPY-S-COPIED, ") {
			got = append(got, line)
		}
	}

	// E3's messages are different lines: take them out of both, to
	// compare the rest.
	want, wantE3 := link68CutE3(want)
	got, gotE3 := link68CutE3(got)

	for _, line := range wantE3 {
		if link68Expected(link68LogDifferences, line) == "" {
			t.Errorf("E3: VMS's line %q isn't a listed difference", line)
		}
	}

	if len(gotE3) != 2 || !strings.HasPrefix(gotE3[0], "%DCL-E-LINKING, ") {
		t.Errorf("E3: govax's lines %q, want its message and $STATUS", gotE3)
	}

	link68CompareLines(t, "link68.log", got, want, link68LogDifferences)
}

// link68CutE3 takes out of lines what E3's LINK wrote, between its
// command line and its SHOW SYMBOL, and the $STATUS SHOW SYMBOL shows,
// returning the rest and those lines.
func link68CutE3(lines []string) (rest, e3 []string) {
	start := -1

	for i, line := range lines {
		switch {
		case line == link68E3:
			start = i + 1
		case start >= 0 && line == "$ SHOW SYMBOL $STATUS" && i+1 < len(lines):
			e3 = append(e3, lines[start:i]...)
			e3 = append(e3, lines[i+1])
			rest = append(rest, lines[:start]...)
			rest = append(rest, line)

			return append(rest, lines[i+2:]...), e3
		}
	}

	return lines, nil
}

// link68Read reads a file as text.
func link68Read(t *testing.T, path string) string {
	t.Helper()

	return string(link68ReadBytes(t, path))
}

// link68ReadBytes reads a file.
func link68ReadBytes(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

// link68Expected is the reason line is a listed difference, or "".
func link68Expected(diffs []link68Difference, line string) string {
	for _, d := range diffs {
		if d.line.MatchString(line) {
			return d.why
		}
	}

	return ""
}

// link68CompareLines holds got to want line for line, but for the lines
// of want diffs lists.
func link68CompareLines(t *testing.T, name string, got, want []string, diffs []link68Difference) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("%s: %d lines, VMS's %d\n%s", name, len(got), len(want), strings.Join(got, "\n"))

		return
	}

	listed := 0

	for i := range want {
		if got[i] == want[i] {
			continue
		}

		if link68Expected(diffs, want[i]) != "" {
			listed++

			continue
		}

		t.Errorf("%s line %d:\n got %q\nwant %q", name, i+1, got[i], want[i])
	}

	t.Logf("%s: %d lines, %d of them listed differences", name, len(want), listed)
}

// link68FileNotes are the files of VMS's run not compared, and why.
var link68FileNotes = map[string]string{
	"files.lst": "DIRECTORY/FULL's file IDs, dates, and owners are the volume's own; the log's DIRECTORY/SIZE/DATE checks the images' sizes",
	"myshr.olb": "the library holds govax's images' GSTs, whose headers hold their link times; myshr.lis checks its modules and symbols, and TestCreateShareableLibrary checks its layout byte for byte from VMS's images",
}

// link68CompareFile holds govax's copy of name, in the directory out,
// to VMS's.
func link68CompareFile(t *testing.T, out, name string) {
	if why, ok := link68FileNotes[name]; ok {
		t.Skip(why)
	}

	base := strings.TrimSuffix(name, filepath.Ext(name))
	vaxPath, govaxPath := filepath.Join(link68Dir, "vax", name), filepath.Join(out, name)

	if _, err := os.Stat(govaxPath); err != nil {
		t.Fatalf("govax's run made no %s", name)
	}

	switch filepath.Ext(name) {
	case ".exe":
		link68CompareImage(t, base, link68ReadBytes(t, vaxPath), link68ReadBytes(t, govaxPath))
	case ".obj", ".stb":
		link68CompareObject(t, link68ReadBytes(t, vaxPath), link68ReadBytes(t, govaxPath))
	default:
		link68CompareLines(t, name, link68MaskText(link68Read(t, govaxPath), name, base),
			link68MaskText(link68Read(t, vaxPath), name, base), link68TextDifferences)
	}
}

// link68CompareObject holds govax's object module (or symbol table
// file) to VMS's, record for record (link68CompareRecords).
func link68CompareObject(t *testing.T, vaxData, govaxData []byte) {
	want, err := obj.ReadRecords(bytes.NewReader(vaxData))
	if err != nil {
		t.Fatal(err)
	}

	got, err := obj.ReadRecords(bytes.NewReader(govaxData))
	if err != nil {
		t.Fatal(err)
	}

	link68CompareRecords(t, got, want)
}

// link68CompareRecords holds got's object records to want's, the main
// header's dates masked and the language processor header (record type
// 0, subtype 1), which names the tool, passed over.
func link68CompareRecords(t *testing.T, got, want [][]byte) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("%d records, VMS's %d", len(got), len(want))
	}

	for i := range want {
		g, w := got[i], want[i]

		if len(w) >= 2 && w[0] == 0 {
			switch w[1] {
			case 0: // the main header: its creation and patch times
				g = link68Dates.ReplaceAll(g, []byte("<date>"))
				w = link68Dates.ReplaceAll(w, []byte("<date>"))
			case 1: // the language processor's name
				continue
			}
		}

		if !bytes.Equal(g, w) {
			t.Errorf("record %d:\n got % x\nwant % x", i+1, g, w)
		}
	}
}

// link68CompareImage holds govax's image to VMS's, byte for byte
// through the last block before a shareable image's global symbol
// table, and the table record for record (real LINK ends the file at
// the table's end, govax at a block's; Phase 30's Decision 7), with
// link68ImageMask's bytes passed over.
func link68CompareImage(t *testing.T, base string, vaxData, govaxData []byte) {
	want, err := vmsimage.ReadImage(vaxData)
	if err != nil {
		t.Fatal(err)
	}

	got, err := vmsimage.ReadImage(govaxData)
	if err != nil {
		t.Fatal(err)
	}

	end := len(vaxData)

	switch {
	case want.GSTVBN != 0:
		end = int(want.GSTVBN-1) * vmsimage.BlockSize
		link68CompareRecords(t, got.GST, want.GST)
	case len(govaxData) != len(vaxData):
		t.Errorf("%d bytes, VMS's %d", len(govaxData), len(vaxData))
	}

	if len(govaxData) < end {
		t.Fatalf("%d bytes, VMS's %d before its symbol table", len(govaxData), end)
	}

	masked := link68ImageMask(want, base)
	differ := 0

	for i := range end {
		if govaxData[i] != vaxData[i] && !masked[i] {
			if differ++; differ <= 10 {
				t.Errorf("byte %#x: %#02x, VMS's %#02x", i, govaxData[i], vaxData[i])
			}
		}
	}

	if differ > 10 {
		t.Errorf("%d bytes differ", differ)
	}
}

// link68ImageMask is the offsets in img's file (VMS's) that
// link68CompareImage passes over: the identification block's link time
// and linker ID, and an ident that comes from the link time
// (link68TimeIdents): an executable's or such a shareable image's own,
// and a global section ISD's for such an image.
func link68ImageMask(img *vmsimage.Image, base string) map[int]bool {
	masked := map[int]bool{}

	mask := func(from, n int) {
		for i := from; i < from+n; i++ {
			masked[i] = true
		}
	}

	// The identification block's link time, at +0x38, and the linker's
	// ID, a counted string in 16 bytes, at +0x40.
	mask(img.ImgIDOffset+0x38, 0x18)

	if img.Type != vmsimage.TypeShareable || link68TimeIdents[base] {
		mask(vmsimage.IHDIdent, 4)
	}

	// The ISDs, which go on in the next block after an ISDContinue.
	at := int(binary.LittleEndian.Uint16(img.Header[vmsimage.IHDISDOffset:]))

	for at+vmsimage.ISDNameOffset < len(img.Header) {
		size := int(binary.LittleEndian.Uint16(img.Header[at:]))

		switch size {
		case 0:
			return masked
		case vmsimage.ISDContinue:
			at = (at/vmsimage.BlockSize + 1) * vmsimage.BlockSize

			continue
		}

		flags := binary.LittleEndian.Uint32(img.Header[at+vmsimage.ISDFlagsOffset:])
		if flags&vmsimage.ISDFlagGBL != 0 {
			n := int(img.Header[at+vmsimage.ISDNameOffset])
			name := string(img.Header[at+vmsimage.ISDNameOffset+1 : at+vmsimage.ISDNameOffset+1+n])

			if image, _, ok := strings.Cut(name, "_0"); ok && link68TimeIdents[strings.ToLower(image)] {
				mask(at+vmsimage.ISDIdentOffset, 4)
			}
		}

		at += size
	}

	return masked
}

// link68Mask is a text replacement link68MaskText applies.
type link68Mask struct {
	re   *regexp.Regexp
	with string
}

// link68Masks are the text files' masks, applied to VMS's and govax's
// lines alike: dates and times, and the tools' names and versions (the
// length of the language processor header and the header records'
// total bytes in ANALYZE/OBJECT, which follow from MACRO's name).
var link68Masks = []link68Mask{
	{link68Dates, "<date>"},
	{regexp.MustCompile(`(VAX|govax) MACRO V[^\s"]* *`), "<macro> "},
	{regexp.MustCompile(`Linker (V11-39|govax) *`), "<linker> "},
	{regexp.MustCompile(`"(V11-39|govax)"`), `"<linker>"`},
	{regexp.MustCompile(`Librarian T09-20|govax Librarian`), "<librarian>"},
	{regexp.MustCompile(`\(OBJ\$C_HDR_LNM\), \d+ bytes`), "(OBJ$$C_HDR_LNM), <n> bytes"},
	{regexp.MustCompile(`^(OBJ\$C_HDR|Totals)(\t+ +\d+\t +)\d+$`), "$1$2<n>"},
}

// link68IdentMasks mask the idents in the text files of
// link68TimeIdents: a map's image section lines and image type line,
// and ANALYZE/IMAGE's ident lines.
var link68IdentMasks = []link68Mask{
	{regexp.MustCompile(`(EQUAL +)\d+ +\d+$`), "$1<ident>"},
	{regexp.MustCompile(`Major=\d+, Minor=\d+`), "Major=<n>, Minor=<n>"},
	{regexp.MustCompile(`major id: %X'[0-9A-F]+', minor id: %X'[0-9A-F]+'`), "major id: <n>, minor id: <n>"},
}

// link68TextDifferences are the lines of VMS's text files, masked
// (link68MaskText), that govax's needn't match.
var link68TextDifferences = []link68Difference{
	{regexp.MustCompile(`^LIBRTL +V06-001 +0 SYS\$COMMON:\[SYSLIB\]LIBRTL\.EXE;1 `), "the map's LIBRTL: govax's tables stand for it, with no file, ident, date, or creator"},
	{regexp.MustCompile(`^Number of (files|modules|program sections): `), "VMS counts LIBRTL.EXE, STARLET.OLB, and its module SYS$P1_VECTOR with its psect, which govax's tables stand for"},
}

// link68MaskText is a text file's lines with link68Masks (and, for the
// files of link68TimeIdents, link68IdentMasks) applied, cut as its kind
// is compared: a map up to its Map format line, before the run
// statistics govax doesn't write, and without STARLET.OLB's module
// (govax's tables stand for it); a MACRO listing but for its
// performance indicators, which are MACRO's process's, and what follows
// them (where its last page begins among them) but its line of messages
// and its command line.
func link68MaskText(text, name, base string) []string {
	var lines []string

	for line := range strings.SplitSeq(strings.TrimRight(strings.ReplaceAll(text, "\r", ""), "\n"), "\n") {
		for _, m := range link68Masks {
			line = m.re.ReplaceAllString(line, m.with)
		}

		if link68TimeIdents[base] {
			for _, m := range link68IdentMasks {
				line = m.re.ReplaceAllString(line, m.with)
			}
		}

		lines = append(lines, line)
	}

	switch {
	case strings.HasSuffix(name, ".map"):
		var kept []string

		for _, line := range lines {
			if strings.Contains(line, "SYS$COMMON:[SYSLIB]STARLET.OLB") {
				continue
			}

			kept = append(kept, line)

			if strings.HasPrefix(line, "Map format:") {
				break
			}
		}

		return kept

	case strings.HasSuffix(name, ".lis") && name != "myshr.lis":
		for i, line := range lines {
			if !strings.Contains(line, "! Performance indicators !") {
				continue
			}

			kept := lines[:i+1:i+1]

			for _, rest := range lines[i:] {
				if link68Messages.MatchString(rest) {
					kept = append(kept, rest)
				}
			}

			// The command line ends the listing.
			return append(kept, lines[len(lines)-1])
		}
	}

	return lines
}

// link68Messages is a MACRO listing's line of messages.
var link68Messages = regexp.MustCompile(`^There (were|was) (no|\d+) (errors|error)`)

// TestLink68MaskLog checks the log's masks on lines of VMS's.
func TestLink68MaskLog(t *testing.T) {
	got := link68MaskLog("MAIN1: SHR1_ORIGIN 00023A00, MAIN1 00000400\r\n" +
		"MAIN3: SHR3_GET() 00300200, G^SHR3_DATA 00300200 holds 00003333\n" +
		"MAIN3: SHR3_GET() 00023800, G^SHR3_DATA 00023800 holds 00003333\n" +
		"-CLI-E-IMAGEFNF, image file not found SIMVAX$DUA0:[SYS0.SYSCOMMON.][SYSLIB]SHR1.EXE;\n" +
		"SHR1.EXE;8                17/17       19-OCT-2026 15:22:47.98\n")

	want := []string{
		"MAIN1: SHR1_ORIGIN <address>, MAIN1 00000400",
		"MAIN3: SHR3_GET() 00300200, G^SHR3_DATA 00300200 holds 00003333",
		"MAIN3: SHR3_GET() <address>, G^SHR3_DATA <address> holds 00003333",
		"-CLI-E-IMAGEFNF, image file not found DUA0:[SYSLIB]SHR1.EXE;",
		"SHR1.EXE;8                17/17       <date>",
	}

	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("masked:\n%q\nwant\n%q", got, want)
	}
}
