package console

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/vmserrors"
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
