package console

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/vmserrors"
)

// link68Vax is the probe's directory of VMS's files (testdata/link68).
var link68Vax = filepath.Join("..", "..", "testdata", "link68", "vax")

// TestLinkShareable runs the probe's LINK/SHAREABLE of SHR4 through DCL,
// its options from SYS$INPUT, with verification on (docs/PHASE-68's
// subtask 5). As in VMS's log, ISD_MAX= is ignored with OPTIGNSHR after
// its line, the link succeeds without USRTFR, and the image is VMS's but
// for its global symbol table (subtask 6) and the header's name, time,
// and linker ID. LINK's status is VMS's, for a clean link and for one
// with warnings.
func TestLinkShareable(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	dir := t.TempDir()

	for _, name := range []string{"xfr1", "shr1"} {
		src := filepath.Join("..", "..", "testdata", "link68", name+".mar")
		if err := c.cmd().Macro(dclcmd.MacroOptions{Source: src, Object: filepath.Join(dir, strings.ToUpper(name)+".OBJ")}); err != nil {
			t.Fatalf("MACRO %s: %v", name, err)
		}
	}

	exe := filepath.Join(dir, "SHR4.EXE")
	link := `$ LINK/SHAREABLE="` + exe + `"/NOMAP SYS$INPUT/OPTIONS`
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

	// VMS's image less its global symbol table: the file ends at the
	// table's block, which the IHS names (its VBN, and its record count
	// as a word and a longword).
	const ihs, ihi = 0x44, 0x58

	le := binary.LittleEndian
	vms = bytes.Clone(vms[:(le.Uint32(vms[ihs+4:])-1)*512])
	le.PutUint32(vms[ihs+4:], 0)
	le.PutUint16(vms[ihs+10:], 0)
	le.PutUint32(vms[ihs+24:], 0)

	// The image's name, link time, and linker ID are its own; its ident,
	// V1.0, is the modules'.
	copy(vms[ihi:ihi+40], image[ihi:ihi+40])
	copy(vms[ihi+56:ihi+80], image[ihi+56:ihi+80])

	if !bytes.Equal(image, vms) {
		t.Errorf("SHR4.EXE differs from VMS's (%d bytes, want %d)", len(image), len(vms))

		for i := 0; i < min(len(image), len(vms)); i++ {
			if image[i] != vms[i] {
				t.Errorf("first difference at %#x: %02X, want %02X", i, image[i], vms[i])

				break
			}
		}
	}
}
