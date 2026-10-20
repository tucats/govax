package console

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/govax/internal/vmsimage"
)

// link68Vax is the probe's directory of VMS's files (testdata/link68).
var link68Vax = filepath.Join("..", "..", "testdata", "link68", "vax")

// TestLinkShareable runs the probe's LINK/SHAREABLE of SHR4 through DCL,
// its options from SYS$INPUT, with verification on (docs/PHASE-68's
// subtask 5). As in VMS's log, ISD_MAX= is ignored with OPTIGNSHR after
// its line, the link succeeds without USRTFR, and the image is VMS's but
// for the name, time, and linker ID in its header and global symbol
// table. /SYMBOL_TABLE writes the global symbol table (subtask 6) as an
// object file. LINK's status is VMS's, for a clean link and for one with
// warnings.
func TestLinkShareable(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	dir := t.TempDir()

	for _, name := range []string{"xfr1", "shr1"} {
		src := filepath.Join("..", "..", "testdata", "link68", name+".mar")
		if err := c.cmd().Macro(dclcmd.MacroOptions{Source: src, Object: filepath.Join(dir, strings.ToUpper(name)+".OBJ")}); err != nil {
			t.Fatalf("MACRO %s: %v", name, err)
		}
	}

	exe, stb := filepath.Join(dir, "SHR4.EXE"), filepath.Join(dir, "SHR4.STB")
	link := `$ LINK/SHAREABLE="` + exe + `"/NOMAP/SYMBOL_TABLE="` + stb + `" SYS$INPUT/OPTIONS`
	// The probe's E2: the same objects linked as an executable, which has
	// no transfer address, a warning.
	e2 := `$ LINK/NOMAP/EXECUTABLE="` + filepath.Join(dir, "E2.EXE") + `" "` + filepath.Join(dir, "XFR1.OBJ") + `","` + filepath.Join(dir, "SHR1.OBJ") + `"`

	proc := writeProcedure(t, dir, "link.com",
		"$ SET VERIFY",
		link,
		`CLUSTER=FIRST,,,"`+filepath.Join(dir, "XFR1.OBJ")+`"`,
		"COLLECT=FIRST,SHR1_RO",
		`"`+filepath.Join(dir, "SHR1.OBJ")+`"`,
		"PSECT_ATTR=SHR1_RO,NOSHR",
		"DZRO_MIN=1",
		"ISD_MAX=20",
		"GSMATCH=ALWAYS,0,0",
		"UNIVERSAL=SHR1_COUNT,SHR1_TABLE,SHR1_ORIGIN",
		"$ SHOW SYMBOL $STATUS",
		e2,
		"$ SHOW SYMBOL $STATUS",
		"$ SET NOVERIFY",
	)

	if err := d.Dispatch("@" + proc); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@link: %v", err)
	}

	got := strings.ReplaceAll(buf.String(), dir+"/", "")
	want := strings.Join([]string{
		strings.ReplaceAll(link, dir+"/", ""),
		`CLUSTER=FIRST,,,"XFR1.OBJ"`,
		"COLLECT=FIRST,SHR1_RO",
		`"SHR1.OBJ"`,
		"PSECT_ATTR=SHR1_RO,NOSHR",
		"DZRO_MIN=1",
		"ISD_MAX=20",
		"%LINK-I-OPTIGNSHR, option ignored for shareable image",
		"GSMATCH=ALWAYS,0,0",
		"UNIVERSAL=SHR1_COUNT,SHR1_TABLE,SHR1_ORIGIN",
		"$ SHOW SYMBOL $STATUS",
		`  $STATUS == "%X10000001"`,
		strings.ReplaceAll(e2, dir+"/", ""),
		"%LINK-W-USRTFR, image E2.EXE has no user transfer address",
		"$ SHOW SYMBOL $STATUS",
		`  $STATUS == "%X10648268"`,
		"$ SET NOVERIFY",
		"",
	}, "\n")

	if got != want {
		t.Errorf("output:\n%s\nwant:\n%s", got, want)
	}

	image, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}

	vms, err := os.ReadFile(filepath.Join(link68Vax, "shr4.exe"))
	if err != nil {
		t.Fatal(err)
	}

	// The image's name, link time, and linker ID are its own; its ident,
	// V1.0, is the modules'. So are the global symbol table's: its main
	// header's name and dates, and its linker name. The table starts at
	// the block the IHS names, and is padded to a whole block.
	const ihs, ihi = 0x44, 0x58

	gst := (binary.LittleEndian.Uint32(vms[ihs+4:]) - 1) * 512
	vms = bytes.Clone(vms[:gst])
	copy(vms[ihi:ihi+40], image[ihi:ihi+40])
	copy(vms[ihi+56:ihi+80], image[ihi+56:ihi+80])

	if !bytes.Equal(image[:min(len(image), len(vms))], vms) {
		t.Errorf("SHR4.EXE differs from VMS's (%d bytes, want %d)", len(image), len(vms))

		for i := 0; i < min(len(image), len(vms)); i++ {
			if image[i] != vms[i] {
				t.Errorf("first difference at %#x: %02X, want %02X", i, image[i], vms[i])

				break
			}
		}
	}

	vmsTable := link68Table(t, filepath.Join(link68Vax, "shr4.exe"), gst)
	table := link68Table(t, exe, gst)

	if file, err := os.ReadFile(stb); err != nil {
		t.Error(err)
	} else if !bytes.Equal(file, table) {
		t.Error("SHR4.STB isn't the image's global symbol table")
	}

	gotRecords, err := obj.ReadRecords(bytes.NewReader(table))
	if err != nil {
		t.Fatal(err)
	}

	wantRecords, err := obj.ReadRecords(bytes.NewReader(vmsTable))
	if err != nil {
		t.Fatal(err)
	}

	// Past the main header and the linker's name, every record is VMS's.
	if len(gotRecords) != len(wantRecords) {
		t.Fatalf("%d global symbol table records, want %d", len(gotRecords), len(wantRecords))
	}

	for i := 2; i < len(wantRecords); i++ {
		if !bytes.Equal(gotRecords[i], wantRecords[i]) {
			t.Errorf("global symbol table record %d is %x, want %x", i+1, gotRecords[i], wantRecords[i])
		}
	}
}

// link68Table returns the global symbol table of image file path, which
// starts at byte offset at: its records, in an object file's layout, to
// the end of module record.
func link68Table(t *testing.T, path string, at uint32) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	records, err := obj.ReadRecords(bytes.NewReader(data[at:]))
	if err != nil {
		t.Fatal(err)
	}

	// Padding reads as empty records after the end of module.
	for i, r := range records {
		if len(r) > 0 && r[0] == 3 {
			records = records[:i+1]

			break
		}
	}

	var buf bytes.Buffer
	if err := obj.WriteRecords(&buf, records); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

// TestLinkShareableUsingAnother runs the probe's LINK/SHAREABLE of SHR2
// through DCL, linked against VMS's SHR1.EXE, which a SYS$INPUT options
// file names (docs/PHASE-68's subtask 7). The image is VMS's but for the
// name, time, and linker ID in its header: its ident is SHR1's, V1.0, the
// first module LINK read; its shareable image list and global section
// ISDs name LIBRTL, which SHR1 uses; and its cells for SHR1 are in
// descending name order.
func TestLinkShareableUsingAnother(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	dir := t.TempDir()

	object := filepath.Join(dir, "SHR2.OBJ")
	if err := c.cmd().Macro(dclcmd.MacroOptions{Source: filepath.Join("..", "..", "testdata", "link68", "shr2.mar"), Object: object}); err != nil {
		t.Fatalf("MACRO: %v", err)
	}

	shr1, err := os.ReadFile(filepath.Join(link68Vax, "shr1.exe"))
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "SHR1.EXE"), shr1, 0o644); err != nil {
		t.Fatal(err)
	}

	exe := filepath.Join(dir, "SHR2.EXE")
	proc := writeProcedure(t, dir, "link.com",
		`$ LINK/SHAREABLE="`+exe+`"/NOMAP "`+object+`",SYS$INPUT/OPTIONS`,
		`"`+filepath.Join(dir, "SHR1.EXE")+`"/SHAREABLE`,
		"UNIVERSAL=SHR2_TWICE,SHR2_COUNT,SHR2_ADDR",
		"GSMATCH=EQUAL,2,5",
		"$ SHOW SYMBOL $STATUS",
	)

	if err := d.Dispatch("@" + proc); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@link: %v", err)
	}

	if got := buf.String(); got != "  $STATUS == \"%X10000001\"\n" {
		t.Errorf("output %q", got)
	}

	image, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}

	vms, err := os.ReadFile(filepath.Join(link68Vax, "shr2.exe"))
	if err != nil {
		t.Fatal(err)
	}

	// The header's name, link time, and linker ID are the image's own,
	// and the global symbol table, from the block the IHS names, has them
	// too (TestLinkShareable checks one).
	const ihs, ihi = 0x44, 0x58

	gst := (binary.LittleEndian.Uint32(vms[ihs+4:]) - 1) * 512
	vms = bytes.Clone(vms[:gst])
	copy(vms[ihi:ihi+40], image[ihi:ihi+40])
	copy(vms[ihi+56:ihi+80], image[ihi+56:ihi+80])

	if len(image) < len(vms) || !bytes.Equal(image[:len(vms)], vms) {
		t.Errorf("SHR2.EXE differs from VMS's (%d bytes, want at least %d)", len(image), len(vms))
	}
}

// TestLinkShareableFullMap links SHR2 against VMS's SHR1.EXE with
// /MAP/FULL: the map is a full one (docs/PHASE-68, subtask 8), with the
// image section synopsis and its SHR1 sections, the module relocatable
// reference synopsis, the symbols by value, and the image synopsis's
// SHAREABLE line. /FULL with /BRIEF is DCL's CONFLICT, and links nothing.
func TestLinkShareableFullMap(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	dir := t.TempDir()

	object := filepath.Join(dir, "SHR2.OBJ")
	if err := c.cmd().Macro(dclcmd.MacroOptions{Source: filepath.Join("..", "..", "testdata", "link68", "shr2.mar"), Object: object}); err != nil {
		t.Fatalf("MACRO: %v", err)
	}

	shr1, err := os.ReadFile(filepath.Join(link68Vax, "shr1.exe"))
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "SHR1.EXE"), shr1, 0o644); err != nil {
		t.Fatal(err)
	}

	exe, mapFile := filepath.Join(dir, "SHR2.EXE"), filepath.Join(dir, "SHR2.MAP")
	proc := writeProcedure(t, dir, "link.com",
		`$ LINK/SHAREABLE="`+exe+`"/MAP="`+mapFile+`"/FULL "`+object+`",SYS$INPUT/OPTIONS`,
		`"`+filepath.Join(dir, "SHR1.EXE")+`"/SHAREABLE`,
		"UNIVERSAL=SHR2_TWICE,SHR2_COUNT,SHR2_ADDR",
		"GSMATCH=EQUAL,2,5",
		"$ SHOW SYMBOL $STATUS",
		`$ LINK/NOEXECUTABLE/MAP/FULL/BRIEF "`+object+`"`,
	)

	if err := d.Dispatch("@" + proc); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@link: %v", err)
	}

	if got := buf.String(); !strings.HasPrefix(got, "  $STATUS == \"%X10000001\"\n") || !strings.Contains(got, "CONFLICT") {
		t.Errorf("output %q", got)
	}

	data, err := os.ReadFile(mapFile)
	if err != nil {
		t.Fatal(err)
	}

	text := string(data)

	for _, want := range []string{
		"! Module Relocatable Reference Synopsis !",
		"SHR2                                    6  ",
		"! Image Section Synopsis !",
		"SHR1               3     1    00000000-R        0   0 READ ONLY                  SHR1_001         LESS/EQUAL        1      1000",
		"                   2     1    00000400-R        4   0 READ WRITE   FIXUP VECTORS",
		"SHR2_TWICE      00000000-RU",
		"! Symbols By Value !",
		"00000448     RX-SHR1_COUNT",
		"PIC, SHAREABLE. Global Section Match=EQUAL, Ident, Major=2, Minor=5",
		"FULL in file",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the map has no %q", want)
		}
	}
}

// TestLinkAgainstGovaxShareableImages runs the probe's shareable image
// cycle through DCL on a volume, all of it govax's (docs/PHASE-68's
// subtask 9): MACRO of the sources, LINK/SHAREABLE of SHR1 and of SHR2,
// which uses SHR1, then MAIN2 linked against SHR2. SHR1 is found through
// its logical name, as the probe's VAX found it, for MAIN2's global
// section ISD for SHR1. MAIN2's header, its image sections and their
// global section names, idents, match controls, and pages, is VMS's but
// for its name, time, and linker ID, and so is its shareable image list.
func TestLinkAgainstGovaxShareableImages(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	mountFreshContainer(t, c, "DUA1:")
	dir := t.TempDir()

	for _, name := range []string{"xfr1", "shr1", "shr2", "main2"} {
		src := filepath.Join("..", "..", "testdata", "link68", name+".mar")
		if err := c.cmd().Macro(dclcmd.MacroOptions{Source: src, Object: "DUA1:[000000]" + strings.ToUpper(name) + ".OBJ"}); err != nil {
			t.Fatalf("MACRO %s: %v", name, err)
		}
	}

	proc := writeProcedure(t, dir, "link.com",
		"$ SET DEFAULT DUA1:[000000]",
		"$ LINK/SHAREABLE=SHR1/NOMAP SYS$INPUT/OPTIONS",
		"CLUSTER=SHR1_VECTORS,,,XFR1.OBJ",
		"SHR1.OBJ",
		"GSMATCH=LEQUAL,1,1000",
		"UNIVERSAL=SHR1_COUNT,SHR1_TABLE,SHR1_ORIGIN",
		"$ SHOW SYMBOL $STATUS",
		"$ DEFINE SHR1 DUA1:[000000]SHR1.EXE",
		"$ LINK/SHAREABLE=SHR2/NOMAP SHR2.OBJ,SYS$INPUT/OPTIONS",
		"SHR1/SHAREABLE",
		"UNIVERSAL=SHR2_TWICE,SHR2_COUNT,SHR2_ADDR",
		"GSMATCH=EQUAL,2,5",
		"$ SHOW SYMBOL $STATUS",
		"$ LINK/NOMAP MAIN2.OBJ,SYS$INPUT/OPTIONS",
		"SHR2/SHAREABLE",
		"$ SHOW SYMBOL $STATUS",
	)

	if err := d.Dispatch("@" + proc); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("@link: %v", err)
	}

	if got, want := buf.String(), strings.Repeat("  $STATUS == \"%X10000001\"\n", 3); got != want {
		t.Errorf("output %q, want %q", got, want)
	}

	image, _, err := c.ContainerSession.ReadRawFile(rms.FileLocation{Name: "DUA1:[000000]MAIN2.EXE"})
	if err != nil {
		t.Fatal(err)
	}

	vms, err := os.ReadFile(filepath.Join(link68Vax, "main2.exe"))
	if err != nil {
		t.Fatal(err)
	}

	// The header block, with the image's own name, link time, and linker
	// ID, and its ident, which is from the link time.
	const ident, ihi = 0x24, 0x58

	header := bytes.Clone(vms[:512])
	copy(header[ident:ident+4], image[ident:ident+4])
	copy(header[ihi:ihi+40], image[ihi:ihi+40])
	copy(header[ihi+56:ihi+80], image[ihi+56:ihi+80])

	if !bytes.Equal(image[:512], header) {
		for i := range header {
			if image[i] != header[i] {
				t.Errorf("MAIN2.EXE's header differs from VMS's at %#x: %02X, want %02X", i, image[i], header[i])

				break
			}
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
		t.Errorf("shareable image list %q, want %q", got.Fixups.Shared, want.Fixups.Shared)
	}
}
