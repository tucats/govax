package console

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/dclcmd"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmserrors"
)

// docs/PHASE-68 - link shareable images.md, subtask 11: RUN activates a
// program's shareable images (their placement, their own fixups, their
// idents), checked against testdata/link68's runs R1 and R5 to R11 and
// E7. These run VMS's own images; TestActivateGovaxShareableImages runs
// the ones govax's LINK makes.

// mainOneRun is what MAIN1 writes, run against SHR1 (testdata/link68's
// R1). Where SHR1 lands differs from VMS's, which put LIBRTL's 279 pages
// before it: govax's shims stand for LIBRTL. So the first line's
// address is masked (maskAddresses).
const mainOneRun = `MAIN1: SHR1_ORIGIN ########, MAIN1 00000400
MAIN1: SHR1_ADD(3,4) 7, SHR1_COUNT 1
MAIN1: SHR1_JSB(5) 11
MAIN1: G^SHR1_ADD origin+FFFFFC08, .ADDRESS origin+FFFFFC08
MAIN1: G^SHR1_TABLE origin+FFFFFE00 holds 11111111, .ADDRESS origin+FFFFFE00
MAIN1: hello, through SHR1_SHOW
SHR1: the .ADDRESS of its own table is right
MAIN1: G^SHR1_COUNT origin+00000200 holds 3
  $STATUS == "%X00000001"
`

// identMismatch is a run whose shareable image's ident doesn't match
// (R5), the image and its file in place of %s.
const identMismatch = `%%DCL-W-ACTIMAGE, error activating image %s
-CLI-E-IMGNAME, image file %s
-SYSTEM-F-SHRIDMISMAT, ident mismatch with shareable image
  $STATUS == "%%X100020BC"
`

// shareImageAddress is an address that depends on where an image landed:
// SHR1's origin, and SHR3P's data.
var shareImageAddress = regexp.MustCompile(`(SHR1_ORIGIN|SHR3_GET\(\)|SHR3_DATA) [0-9A-F]{8}`)

// maskAddresses masks the addresses shareImageAddress finds in out.
func maskAddresses(out string) string {
	return shareImageAddress.ReplaceAllStringFunc(out, func(s string) string {
		return s[:len(s)-8] + "########"
	})
}

// shareActivateConsole is shareFindConsole's console with the images
// names, VMS's, in DUA1:[000000], each found through a logical name of
// its own, as the probe defined them.
func shareActivateConsole(t *testing.T, paging bool, names ...string) (*Dispatcher, *Console, *bytes.Buffer) {
	t.Helper()

	d, c, buf := shareFindConsole(t, paging)

	for _, name := range names {
		upper := strings.ToUpper(name)
		shareFindCopy(t, d, filepath.Join(link68Vax, name+".exe"), "DUA1:[000000]"+upper+".EXE")

		if strings.HasPrefix(upper, "SHR") {
			shareFindRun(t, d, "DEFINE "+upper+" DUA1:[000000]"+upper+".EXE")
		}
	}

	buf.Reset()

	return d, c, buf
}

// shareRun runs image with RUN, and returns what it and SHOW SYMBOL
// $STATUS wrote.
func shareRun(t *testing.T, d *Dispatcher, buf *bytes.Buffer, image string) string {
	t.Helper()

	buf.Reset()

	if err := d.Dispatch("RUN " + image); err != nil && !vmserrors.MessageInhibited(err) {
		fmt.Fprintf(buf, "%%%s\n", err) // as cmd/govax shows it
	}

	shareFindRun(t, d, "SHOW SYMBOL $STATUS")

	out := buf.String()
	buf.Reset()

	return out
}

// eachLoader runs test in both paging modes: the copying loader and the
// mapping one.
func eachLoader(t *testing.T, test func(t *testing.T, paging bool)) {
	for _, paging := range []bool{false, true} {
		t.Run(map[bool]string{false: "copying", true: "mapping"}[paging], func(t *testing.T) {
			test(t, paging)
		})
	}
}

// TestActivateVMSShareableImages runs VMS's programs against VMS's
// shareable images: MAIN1 (R1), MAIN2 through SHR2 and SHR1 (R7), MAIN3
// against the based SHR3 and MAIN3P against SHR3P (R8), MAIN4 against
// SHR4 and its ALWAYS relink (R9), MAIN6 (R10), EXE1 (R11), MAIN1A
// against SHR1A and its relink (R6), and RUN of SHR1 itself (E7).
func TestActivateVMSShareableImages(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, _, buf := shareActivateConsole(t, paging,
			"main2", "shr2", "main3", "shr3", "main3p", "shr3p", "main4", "shr4",
			"main6", "shr6", "exe1", "main1a", "shr1a")
		shareFindRun(t, d, "DEFINE SHR1 DUA1:[000000]SHR1.EXE")

		runs := []struct{ image, want string }{
			{"MAIN1", mainOneRun},
			{"MAIN2", "MAIN2: SHR2_TWICE(21) 42\nMAIN2: SHR2_COUNT() 1\nMAIN2: SHR2_ADDR() 0\n  $STATUS == \"%X00000001\"\n"},
			{"MAIN3", "MAIN3: SHR3_GET() 00300200, G^SHR3_DATA 00300200 holds 00003333\n  $STATUS == \"%X00000001\"\n"},
			{"MAIN3P", "MAIN3: SHR3_GET() ########, G^SHR3_DATA ######## holds 00003333\n  $STATUS == \"%X00000001\"\n"},
			{"MAIN4", mainOneRun},
			{"MAIN6", "MAIN6: SHR6_GET() 0\n  $STATUS == \"%X00000001\"\n"},
			{"EXE1", "EXE1: EXE1_DATA 00010000, EXE1_RO 00040000, EXE1_ZERO 00010200\n" +
				"EXE1: EXE1 00010C00, EXE1B_ROUT 00040400, EXE1B_DATA 00040200\n  $STATUS == \"%X00000001\"\n"},
			{"MAIN1A", strings.ReplaceAll(mainOneRun, "FFFFFC08", "00000007")},
			{"SHR1.EXE", "%DCL-E-NOTFR, no transfer address\n  $STATUS == \"%X0003889A\"\n"},
		}

		for _, r := range runs {
			// MAIN3's addresses are SHR3's own, where it was based.
			got := shareRun(t, d, buf, r.image)
			if r.image != "MAIN3" {
				got = maskAddresses(got)
			}

			if got != r.want {
				t.Errorf("RUN %s:\n%s\nwant:\n%s", r.image, got, r.want)
			}
		}

		// MAIN4 runs against SHR4 relinked ALWAYS,7,7 (VMS's second link).
		shareFindCopy(t, d, filepath.Join(link68Vax, "shr4-2.exe"), "DUA1:[000000]SHR4.EXE")

		if got := maskAddresses(shareRun(t, d, buf, "MAIN4")); got != mainOneRun {
			t.Errorf("RUN MAIN4 against SHR4;2:\n%s\nwant:\n%s", got, mainOneRun)
		}

		// MAIN1A doesn't run against SHR1A relinked: its default ident,
		// from the link time, is another.
		shareFindCopy(t, d, filepath.Join(link68Vax, "shr1a-2.exe"), "DUA1:[000000]SHR1A.EXE")

		if got, want := shareRun(t, d, buf, "MAIN1A"), fmt.Sprintf(identMismatch, "SHR1A", "DUA1:[000000]SHR1A.EXE;2"); got != want {
			t.Errorf("RUN MAIN1A against SHR1A;2:\n%s\nwant:\n%s", got, want)
		}
	})
}

// govaxShareConsole is shareFindConsole's console with the probe's
// images made by govax's MACRO and LINK in DUA2:[000000], now the
// default directory, as testdata/link68's LINK68.COM makes them, each
// shareable image found through a logical name of its own. It also
// returns the directory of the test's procedures.
func govaxShareConsole(t *testing.T, paging bool) (*Dispatcher, *Console, *bytes.Buffer, string) {
	t.Helper()

	d, c, buf := shareFindConsole(t, paging)
	dir := t.TempDir()

	for _, name := range []string{"xfr1", "shr1", "shr2", "shr3", "shr6", "main1", "main2", "main3", "main6", "exe1", "exe1b"} {
		src := filepath.Join("..", "..", "testdata", "link68", name+".mar")
		if err := c.cmd().Macro(dclcmd.MacroOptions{Source: src, Object: "DUA2:[000000]" + strings.ToUpper(name) + ".OBJ"}); err != nil {
			t.Fatalf("MACRO %s: %v", name, err)
		}
	}

	shareFindRun(t, d, `COPY "`+filepath.Join("..", "..", "testdata", "link68", "shr1.opt")+`"/HOST DUA2:[000000]SHR1.OPT`)

	proc := writeProcedure(t, dir, "build.com",
		"$ SET DEFAULT DUA2:[000000]",
		"$ @"+relinkProcedure(t, dir)+` "LEQUAL,1,1000"`,
		"$ DEFINE SHR1 DUA2:[000000]SHR1.EXE",
		"$ LINK/NOMAP MAIN1.OBJ,SYS$INPUT/OPTIONS",
		"SHR1/SHAREABLE",
		"$ LINK/SHAREABLE=SHR2/NOMAP SHR2.OBJ,SYS$INPUT/OPTIONS",
		"SHR1/SHAREABLE",
		"UNIVERSAL=SHR2_TWICE,SHR2_COUNT,SHR2_ADDR",
		"GSMATCH=EQUAL,2,5",
		"$ DEFINE SHR2 DUA2:[000000]SHR2.EXE",
		"$ LINK/NOMAP MAIN2.OBJ,SYS$INPUT/OPTIONS",
		"SHR2/SHAREABLE",
		"$ LINK/SHAREABLE=SHR3/NOMAP SHR3.OBJ,SYS$INPUT/OPTIONS",
		"BASE=%X300000",
		"UNIVERSAL=SHR3_GET,SHR3_DATA",
		"$ LINK/SHAREABLE=SHR3P/NOMAP SHR3.OBJ,SYS$INPUT/OPTIONS",
		"UNIVERSAL=SHR3_GET,SHR3_DATA",
		"$ DEFINE SHR3 DUA2:[000000]SHR3.EXE",
		"$ DEFINE SHR3P DUA2:[000000]SHR3P.EXE",
		"$ LINK/NOMAP MAIN3.OBJ,SYS$INPUT/OPTIONS",
		"SHR3/SHAREABLE",
		"$ LINK/NOMAP/EXECUTABLE=MAIN3P MAIN3.OBJ,SYS$INPUT/OPTIONS",
		"SHR3P/SHAREABLE",
		"$ LINK/SHAREABLE=SHR6/NOMAP SHR6.OBJ,SYS$INPUT/OPTIONS",
		"UNIVERSAL=SHR6_GET",
		"$ DEFINE SHR6 DUA2:[000000]SHR6.EXE",
		"$ LINK/NOMAP MAIN6.OBJ,SYS$INPUT/OPTIONS",
		"SHR6/SHAREABLE",
		"$ LINK/NOMAP EXE1.OBJ,SYS$INPUT/OPTIONS",
		"BASE=%X10000",
		"CLUSTER=SECOND,%X40000,,EXE1B.OBJ",
		"COLLECT=SECOND,EXE1_RO",
		"PSECT_ATTR=EXE1_DATA,NOWRT",
		"IOSEGMENT=40",
		"DZRO_MIN=2",
		"ISD_MAX=40",
		"$ SHOW SYMBOL $STATUS",
	)

	buf.Reset()

	if err := d.Dispatch("@" + proc); err != nil {
		t.Fatalf("@build: %v\n%s", err, buf)
	}

	if got, want := buf.String(), "  $STATUS == \"%X10000001\"\n"; got != want {
		t.Fatalf("the links: %q, want %q", got, want)
	}

	buf.Reset()

	return d, c, buf, dir
}

// relinkProcedure writes RELINK.COM, which links SHR1 with P1 as its
// GSMATCH=, as LINK68.COM's RELINK1 does, and returns its name.
func relinkProcedure(t *testing.T, dir string) string {
	t.Helper()

	return writeProcedure(t, dir, "relink.com",
		"$ OPEN/WRITE OPT GSM.OPT",
		`$ WRITE OPT "GSMATCH=''P1'"`,
		"$ CLOSE OPT",
		"$ LINK/SHAREABLE=SHR1.EXE/NOMAP SHR1.OPT/OPTIONS,GSM.OPT/OPTIONS",
	)
}

// TestActivateGovaxShareableImages runs the programs govax's LINK made
// against the shareable images it made, as TestActivateVMSShareableImages
// runs VMS's: their output is VMS's.
func TestActivateGovaxShareableImages(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf, _ := govaxShareConsole(t, paging)

		runs := []struct{ image, want string }{
			{"MAIN1", mainOneRun},
			{"MAIN2", "MAIN2: SHR2_TWICE(21) 42\nMAIN2: SHR2_COUNT() 1\nMAIN2: SHR2_ADDR() 0\n  $STATUS == \"%X00000001\"\n"},
			{"MAIN3", "MAIN3: SHR3_GET() 00300200, G^SHR3_DATA 00300200 holds 00003333\n  $STATUS == \"%X00000001\"\n"},
			{"MAIN3P", "MAIN3: SHR3_GET() ########, G^SHR3_DATA ######## holds 00003333\n  $STATUS == \"%X00000001\"\n"},
			{"MAIN6", "MAIN6: SHR6_GET() 0\n  $STATUS == \"%X00000001\"\n"},
			{"EXE1", "EXE1: EXE1_DATA 00010000, EXE1_RO 00040000, EXE1_ZERO 00010200\n" +
				"EXE1: EXE1 00010C00, EXE1B_ROUT 00040400, EXE1B_DATA 00040200\n  $STATUS == \"%X00000001\"\n"},
			{"SHR1.EXE", "%DCL-E-NOTFR, no transfer address\n  $STATUS == \"%X0003889A\"\n"},
		}

		for _, r := range runs {
			got := shareRun(t, d, buf, r.image)
			if r.image != "MAIN3" {
				got = maskAddresses(got)
			}

			if got != r.want {
				t.Errorf("RUN %s:\n%s\nwant:\n%s", r.image, got, r.want)
			}

			// EXE1's LIBRTL is where its link placed it, a vector into
			// the shims (EXE1.MAP: LIBRTL_001 at ^X10E00).
			if r.image == "EXE1" {
				placed := false

				for _, icb := range c.ImageState(nil).ICBList {
					placed = placed || icb.Name == "LIBRTL" && icb.Base == 0x10E00
				}

				if !placed {
					t.Error("EXE1's LIBRTL wasn't placed at ^X10E00")
				}
			}
		}
	})
}

// TestActivateIdentMatch relinks SHR1 with each GSMATCH= of
// testdata/link68's R5, running MAIN1 (linked against LEQUAL,1,1000)
// after each, and MAIN1N (linked against EQUAL,1,1001) where VMS ran it:
// the match controls must agree, and the linked-with one decides.
func TestActivateIdentMatch(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, _, buf, dir := govaxShareConsole(t, paging)
		relink := relinkProcedure(t, dir)

		mismatch := func(version int) string {
			return fmt.Sprintf(identMismatch, "SHR1", fmt.Sprintf("DUA2:[000000]SHR1.EXE;%d", version))
		}

		// A step relinks SHR1 with its GSMATCH=, or else links MAIN1N
		// against SHR1 as it is (link), then runs its image.
		steps := []struct {
			gsmatch     string
			link        bool
			image, want string
		}{
			{"LEQUAL,1,1001", false, "MAIN1", mainOneRun},
			{"LEQUAL,1,999", false, "MAIN1", mismatch(3)},
			{"LEQUAL,2,1000", false, "MAIN1", mismatch(4)},
			{"EQUAL,1,1000", false, "MAIN1", mismatch(5)},
			{"EQUAL,1,1001", false, "MAIN1", mismatch(6)},
			{"NEVER,1,1000", false, "MAIN1", mismatch(6)}, // the link fails: SHR1.EXE;6 stays
			{"", true, "MAIN1N", mainOneRun},
			{"ALWAYS,3,3", false, "MAIN1", mismatch(7)},
			{"", false, "MAIN1N", mismatch(7)},
			{"LEQUAL,1,1000", false, "MAIN1", mainOneRun},
		}

		for _, s := range steps {
			switch {
			case s.gsmatch != "":
				_ = d.Dispatch(`@` + relink + ` "` + s.gsmatch + `"`)
			case s.link:
				if err := d.Dispatch("@" + writeProcedure(t, dir, "main1n.com",
					"$ LINK/NOMAP/EXECUTABLE=MAIN1N MAIN1.OBJ,SYS$INPUT/OPTIONS", "SHR1/SHAREABLE")); err != nil {
					t.Fatalf("LINK MAIN1N: %v", err)
				}
			}

			if got := maskAddresses(shareRun(t, d, buf, s.image)); got != s.want {
				t.Errorf("after GSMATCH=%s, RUN %s:\n%s\nwant:\n%s", s.gsmatch, s.image, got, s.want)
			}
		}
	})
}

// TestActivateProtectionChanges activates MAIN6 (RUN/NOEXECUTE) against
// the SHR6 govax's LINK made: SHR6's first page, read-only but holding a
// .ADDRESS of its own, is fixed up and then made UR, and its fixup
// section's page UREW, by its change-protection list (SHR6.ANI). (That
// the fixup was right is MAIN6's run, in the tests above.)
func TestActivateProtectionChanges(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, _, _ := govaxShareConsole(t, paging)
		shareFindRun(t, d, "RUN/NOEXECUTE MAIN6")

		var shr *machine.ICB

		for _, icb := range c.ImageState(nil).ICBList {
			if icb.Name == "SHR6" {
				shr = icb
			}
		}

		if shr == nil {
			t.Fatal("SHR6 wasn't activated")
		}

		space := c.ProcessOne().Space.AddressSpace

		for _, page := range []struct {
			offset uint32
			want   vm.Protection
		}{{0, vm.ProtUR}, {0x400, vm.ProtUREW}} {
			pte, err := c.Mem.LookupPTEIn(c.CPU, space, shr.Base+page.offset)
			if err != nil {
				t.Fatal(err)
			}

			if got := pte.Protection(); got != page.want {
				t.Errorf("SHR6's page at +%X has protection %v, want %v", page.offset, got, page.want)
			}
		}
	})
}
