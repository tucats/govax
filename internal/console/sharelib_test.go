package console

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/govax/internal/vmsimage"
)

// docs/PHASE-68 - link shareable images.md, subtask 12: shareable image
// libraries, checked against testdata/link68's R12 and E9.

// listingDate is a date and time in a LIBRARY/LIST listing.
var listingDate = regexp.MustCompile(`\d{1,2}-[A-Z]{3}-\d{4} \d\d:\d\d:\d\d`)

// TestShareableImageLibrary runs R12 through DCL with govax's own images:
// LIBRARY/CREATE/SHARE of SHR1 and SHR2, LIBRARY/LIST of the library,
// MAIN1 linked through it by name (MAIN1L) and as a default user library
// (LNK$LIBRARY, MAIN1U), then without it (/NOUSERLIBRARY, E9). The
// listing is VMS's but for its times and creator; MAIN1L's and MAIN1U's
// headers and shareable image lists are VMS's but for their names, times,
// idents, and linker IDs; E9's messages and every status are VMS's; and
// both programs run as VMS's did. MAIN1L is linked from another default
// directory, so SHR1.EXE is found beside the library.
func TestShareableImageLibrary(t *testing.T) {
	d, c, buf := shareFindConsole(t, false)
	dir := t.TempDir()

	for _, name := range []string{"xfr1", "shr1", "shr2", "main1"} {
		src := filepath.Join("..", "..", "testdata", "link68", name+".mar")
		if err := c.cmd().Macro(dclcmd.MacroOptions{Source: src, Object: "DUA1:[000000]" + strings.ToUpper(name) + ".OBJ"}); err != nil {
			t.Fatalf("MACRO %s: %v", name, err)
		}
	}

	shareFindRun(t, d, `COPY "`+filepath.Join("..", "..", "testdata", "link68", "shr1.opt")+`"/HOST DUA1:[000000]SHR1.OPT`)

	proc := writeProcedure(t, dir, "r12.com",
		"$ SET DEFAULT DUA1:[000000]",
		"$ @"+relinkProcedure(t, dir)+` "LEQUAL,1,1000"`,
		"$ LINK/SHAREABLE=SHR2/NOMAP SHR2.OBJ,SYS$INPUT/OPTIONS",
		"SHR1/SHAREABLE",
		"UNIVERSAL=SHR2_TWICE,SHR2_COUNT,SHR2_ADDR",
		"GSMATCH=EQUAL,2,5",
		"$ LIBRARY/CREATE/SHARE MYSHR SHR1.EXE,SHR2.EXE",
		"$ SHOW SYMBOL $STATUS",
		"$ LIBRARY/LIST=MYSHR.LIS/FULL/NAMES/SHARE MYSHR",
		"$ SET DEFAULT DUA2:[000000]",
		"$ LINK/NOMAP/EXECUTABLE=DUA1:[000000]MAIN1L DUA1:[000000]MAIN1.OBJ,DUA1:[000000]MYSHR/LIBRARY",
		"$ SHOW SYMBOL $STATUS",
		"$ SET DEFAULT DUA1:[000000]",
		"$ DEFINE LNK$LIBRARY DUA1:[000000]MYSHR.OLB",
		"$ LINK/NOMAP/EXECUTABLE=MAIN1U MAIN1.OBJ",
		"$ SHOW SYMBOL $STATUS",
		"$ LINK/NOUSERLIBRARY/NOMAP/EXECUTABLE=E9 MAIN1.OBJ",
		"$ SHOW SYMBOL $STATUS",
		"$ DEASSIGN LNK$LIBRARY",
	)

	buf.Reset()

	if err := d.Dispatch("@" + proc); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@r12: %v\n%s", err, buf)
	}

	// E9's messages are the log's, between its LINK and its SHOW SYMBOL.
	logData, err := os.ReadFile(filepath.Join(link68Vax, "link68.log"))
	if err != nil {
		t.Fatal(err)
	}

	// VMS ended each line of its multi-line messages with a carriage
	// return too.
	log := strings.Split(strings.ReplaceAll(string(logData), "\r", ""), "\n")
	start := indexOf(t, log, "$ LINK/NOUSERLIBRARY/NOMAP/EXECUTABLE=E9 MAIN1.OBJ")
	end := start + indexOf(t, log[start:], "$ SHOW SYMBOL $STATUS")

	want := strings.Join([]string{
		`  $STATUS == "%X10000001"`, // LIBRARY
		`  $STATUS == "%X10000001"`, // MAIN1L
		`  $STATUS == "%X10000001"`, // MAIN1U
		strings.Join(log[start+1:end], "\n"),
		`  $STATUS == "%X10648268"`, // E9
		"",
	}, "\n")

	gotLines, wantLines := strings.Split(buf.String(), "\n"), strings.Split(want, "\n")

	for i := range max(len(gotLines), len(wantLines)) {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}

		if i < len(wantLines) {
			w = wantLines[i]
		}

		if g != w {
			t.Errorf("output line %d:\n got %q\nwant %q", i+1, g, w)
		}
	}

	// The listing, but for its times and its creator.
	listing, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]MYSHR.LIS"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	vmsListing, err := os.ReadFile(filepath.Join(link68Vax, "myshr.lis"))
	if err != nil {
		t.Fatal(err)
	}

	wantLines = strings.Split(strings.TrimSuffix(string(vmsListing), "\n"), "\n")

	for i, w := range wantLines {
		var g string
		if i < len(listing) {
			g = string(listing[i])
		}

		g, w = listingDate.ReplaceAllString(g, "<date>"), listingDate.ReplaceAllString(w, "<date>")
		g, w = strings.Replace(g, "govax Librarian", "<creator>", 1), strings.Replace(w, "Librarian T09-20", "<creator>", 1)

		if g != w {
			t.Errorf("MYSHR.LIS line %d:\n got %q\nwant %q", i+1, g, w)
		}
	}

	if len(listing) != len(wantLines) {
		t.Errorf("MYSHR.LIS has %d lines, want %d", len(listing), len(wantLines))
	}

	for _, name := range []string{"main1l", "main1u"} {
		checkProgramHeader(t, c, name)
	}

	shareFindRun(t, d, "DEFINE SHR1 DUA1:[000000]SHR1.EXE")

	for _, image := range []string{"MAIN1L", "MAIN1U"} {
		if got := maskAddresses(shareRun(t, d, buf, image)); got != mainOneRun {
			t.Errorf("RUN %s:\n%s\nwant:\n%s", image, got, mainOneRun)
		}
	}
}

// indexOf is the index of the first of lines that is line.
func indexOf(t *testing.T, lines []string, line string) int {
	t.Helper()

	for i, l := range lines {
		if l == line {
			return i
		}
	}

	t.Fatalf("no line %q", line)

	return 0
}

// checkProgramHeader checks that the program name govax linked in
// DUA1:[000000] has VMS's header block, but for its own name, link time,
// ident, and linker ID, and VMS's shareable image list.
func checkProgramHeader(t *testing.T, c *Console, name string) {
	t.Helper()

	image, _, err := c.ContainerSession.ReadRawFile(rms.FileLocation{Name: "DUA1:[000000]" + strings.ToUpper(name) + ".EXE"})
	if err != nil {
		t.Fatal(err)
	}

	vms, err := os.ReadFile(filepath.Join(link68Vax, name+".exe"))
	if err != nil {
		t.Fatal(err)
	}

	const ident, ihi = 0x24, 0x58

	header := bytes.Clone(vms[:512])
	copy(header[ident:ident+4], image[ident:ident+4])
	copy(header[ihi:ihi+40], image[ihi:ihi+40])
	copy(header[ihi+56:ihi+80], image[ihi+56:ihi+80])

	for i := range header {
		if image[i] != header[i] {
			t.Errorf("%s's header differs from VMS's at %#x: %02X, want %02X", name, i, image[i], header[i])

			break
		}
	}

	got, err := vmsimage.ReadImage(image)
	if err != nil {
		t.Fatal(err)
	}

	want, err := vmsimage.ReadImage(vms)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got.Fixups.Shared, want.Fixups.Shared) {
		t.Errorf("%s's shareable image list %q, want %q", name, got.Fixups.Shared, want.Fixups.Shared)
	}
}

// TestShareableImageLibraryHost links MAIN1 through VMS's MYSHR.OLB as a
// host file, with VMS's SHR1.EXE beside it: the library's images are
// looked for in its own directory on the host too. /INCLUDE=SHR1 takes
// the image into the link without a search; /USERLIBRARY=NONE and a
// keyword that isn't a table are checked too.
func TestShareableImageLibraryHost(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	dir := t.TempDir()

	for _, name := range []string{"myshr.olb", "shr1.exe"} {
		data, err := os.ReadFile(filepath.Join(link68Vax, name))
		if err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(dir, strings.ToUpper(name)), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	object := filepath.Join(dir, "MAIN1.OBJ")
	if err := c.cmd().Macro(dclcmd.MacroOptions{Source: filepath.Join("..", "..", "testdata", "link68", "main1.mar"), Object: object}); err != nil {
		t.Fatalf("MACRO: %v", err)
	}

	lib := filepath.Join(dir, "MYSHR.OLB")

	for _, line := range []string{
		`LINK/NOMAP/EXECUTABLE="` + filepath.Join(dir, "MAIN1L.EXE") + `" "` + object + `","` + lib + `"/LIBRARY`,
		`LINK/NOMAP/USERLIBRARY=NONE/EXECUTABLE="` + filepath.Join(dir, "MAIN1I.EXE") + `" "` + object + `","` + lib + `"/INCLUDE=SHR1`,
	} {
		buf.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			t.Fatalf("%s: %v", line, err)
		}

		if buf.Len() > 0 {
			t.Errorf("%s:\n%s", line, buf)
		}
	}

	for _, name := range []string{"MAIN1L.EXE", "MAIN1I.EXE"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}

		img, err := vmsimage.ReadImage(data)
		if err != nil {
			t.Fatal(err)
		}

		if got := img.Fixups.Shared; len(got) < 2 || got[1] != "SHR1" {
			t.Errorf("%s's shareable image list is %q", name, got)
		}
	}

	if err := d.Dispatch(`LINK/NOEXECUTABLE/USERLIBRARY=NOSUCH "` + object + `"`); err == nil || !strings.Contains(err.Error(), "IVKEYW") {
		t.Errorf("/USERLIBRARY=NOSUCH: %v", err)
	}
}
