package console

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmsimage"
)

// Phase 113's probe round 2 (docs/planning/PHASE-113 - Fortran errata.md,
// subtask 14a; testdata/err113's README, "Round 2"): ERR113B.COM asks VMS what
// round 1 and subtask 13 left unconfirmed (a shareable image's .LONG of its
// own address, the blocks LINK gives a shareable image's debug symbol table,
// a register name after an operator in MACRO's data, other commands' file
// parameters, a full volume's last block, and the index file past its header
// on a fragmented volume). TestErr113BProcedure rehearses it on govax.

// runErr113B makes round 2's volume as exchangeb.cmd does and runs ERR113B.COM
// with it on DUA1 and an RD51 scratch volume on DUA3, made as INITIALIZE
// /HEADERS=16 makes it and used as it is (P3 KEEP: govax has no INITIALIZE of
// a device). It returns the log.
func runErr113B(t *testing.T) string {
	t.Helper()

	d, c, buf := shareFindConsole(t, false)
	dir := t.TempDir()
	exchange := filepath.Join(dir, "err113b.dsk")
	scratch := filepath.Join(dir, "scratch.dsk")

	shareFindRun(t, d, `INITIALIZE/CONTAINER "`+exchange+`" /DEVICE=RD54 ERR113B`)
	shareFindRun(t, d, `INITIALIZE/CONTAINER "`+scratch+`" /DEVICE=RD51 /HEADERS=16 SCR113`)

	if err := c.cmd().Mount("DUA5:", exchange, true); err != nil {
		t.Fatal(err)
	}

	t.Chdir(filepath.Join("..", ".."))

	script, err := os.ReadFile(filepath.Join("testdata", "err113", "exchangeb.cmd"))
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
	shareFindRun(t, d, `@ERR113B/OUTPUT=ERR113B.LOG DUA3: "" KEEP`)

	host := filepath.Join(t.TempDir(), "err113b.log")
	shareFindRun(t, d, `COPY/QUIET DUA1:[000000]ERR113B.LOG "`+host+`"/HOST`)

	data, err := os.ReadFile(host)
	if err != nil {
		t.Fatal(err)
	}

	return strings.ReplaceAll(string(data), "\r", "")
}

func TestErr113BProcedure(t *testing.T) {
	if testing.Short() {
		t.Skip("the round's procedure fills a volume with files")
	}

	log := runErr113B(t)

	if path := os.Getenv("ERR113B_WRITE"); path != "" {
		if err := os.WriteFile(path, []byte(log), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	sections := vms93Sections(log)

	for _, name := range []string{"LNK113L", "LNK113D", "MAC113F", "DCL2", "FULL2", "HDRFULL", "END"} {
		if _, ok := sections[name]; !ok {
			t.Errorf("no section %s in the log:\n%s", name, log)
		}
	}

	vmsLog, err := os.ReadFile(filepath.Join("testdata", "err113", "vax", "round2", "err113b.log"))
	if err != nil {
		t.Fatal(err)
	}

	vms := vms93Sections(strings.ReplaceAll(string(vmsLog), "\r", ""))

	for name, diffs := range err113BDifferences {
		want := vms[name]

		for _, d := range diffs {
			if !strings.Contains(want, d.vms) {
				t.Errorf("%s: VMS's section has no %q (err113BDifferences: %s)", name, d.vms, d.why)
			}

			want = strings.Replace(want, d.vms, d.govax, 1)
		}

		if got := sections[name]; got != want {
			t.Errorf("%s is\n%s\nVMS's (with err113BDifferences)\n%s", name, got, want)
		}
	}
}

// err113BDifferences are the round's sections TestErr113BProcedure compares
// with VMS's log, each with where govax's differs and why.
var err113BDifferences = map[string][]err113Difference{
	// Every command's file specification with a directory left open is
	// CLI$_DIRECT before the command runs (the grammar's $FILE; @ its own).
	"DCL2": {{
		vms: "%DCL-W-DIRECT, invalid directory syntax - check brackets and other delimiters\n" +
			" \\DCL[X.TXT\\\n  $STATUS == \"%X00038030\"\n%DCL-W-DIRECT, invalid directory syntax - check brackets and other delimiters\n" +
			" \\DCL[X.COM\\\n",
		govax: "%DCL-E-UNDEFSYM, Undefined symbol \"DCL\"\n  $STATUS == \"%X00200072\"\n" +
			"%DCL-W-DIRECT, invalid directory syntax - check brackets and other delimiters\n" +
			" \\DCL[X.COM\\\n",
		why: "govax's PRINT is the machine console's, printing an expression, not VMS's queue command",
	}},
}

// TestErr113BLink links round 2's LNK113L from VMS's own object
// (testdata/err113/vax/round2) and holds the image and map to VMS's: a
// shareable image's .LONG of its own address is LINK-W-NONPICREF, and the
// image is based where it was linked (LINK-I-BASDUERRS), with no fixups of
// its own addresses.
func TestErr113BLink(t *testing.T) {
	d, _, buf := shareFindConsole(t, false)
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "err113"))
	if err != nil {
		t.Fatal(err)
	}

	vax := filepath.Join(dir, "vax", "round2")

	shareFindRun(t, d, "CREATE/DIRECTORY DUA1:[ERR113]")
	shareFindRun(t, d, "SET DEFAULT DUA1:[ERR113]")
	shareFindRun(t, d, `COPY/BINARY "`+filepath.Join(vax, "lnk113l.obj")+`"/HOST LNK113L.OBJ`)
	shareFindRun(t, d, `COPY "`+filepath.Join(dir, "lnk113l.opt")+`"/HOST LNK113L.OPT`)

	buf.Reset()
	_ = d.Dispatch("LINK/SHAREABLE/MAP=LNK113L/FULL LNK113L,LNK113L/OPTIONS")

	if said := buf.String(); !strings.HasPrefix(said, "%LINK-I-BASDUERRS, basing image due to errors in relocatable references\n") ||
		strings.Count(said, "%LINK-W-NONPICREF, failure to use .ADDRESS for relocatable data \n") != 3 {
		t.Errorf("LINK said:\n%s", said)
	}

	out := t.TempDir()
	shareFindRun(t, d, `COPY/BINARY LNK113L.EXE "`+filepath.Join(out, "lnk113l.exe")+`"/HOST`)
	shareFindRun(t, d, `COPY LNK113L.MAP "`+filepath.Join(out, "lnk113l.map")+`"/HOST`)

	link68CompareImage(t, "lnk113l", link68ReadBytes(t, filepath.Join(vax, "lnk113l.exe")), link68ReadBytes(t, filepath.Join(out, "lnk113l.exe")))

	link68CompareLines(t, "lnk113l.map", link68MaskText(link68Read(t, filepath.Join(out, "lnk113l.map")), "lnk113l.map", "lnk113l"),
		link68MaskText(link68Read(t, filepath.Join(vax, "lnk113l.map")), "lnk113l.map", "lnk113l"), link68TextDifferences)
}

// TestErr113BDSTBlocks links round 2's LNK113D sweep (testdata/err113/gen.go)
// with govax's MACRO and LINK and holds each image's debug symbol table to
// VMS's size in blocks: its traceback records' block, then one for fixup
// records of 110 bytes, though the two total 505 to 520 (link's
// withDSTFixups).
func TestErr113BDSTBlocks(t *testing.T) {
	d, _, _ := shareFindConsole(t, false)
	dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", "err113"))
	if err != nil {
		t.Fatal(err)
	}

	out := t.TempDir()

	shareFindRun(t, d, "CREATE/DIRECTORY DUA1:[ERR113]")
	shareFindRun(t, d, "SET DEFAULT DUA1:[ERR113]")

	for i := range 16 {
		name := fmt.Sprintf("lnk113d%02d", i)
		upper := strings.ToUpper(name)

		shareFindRun(t, d, `COPY "`+filepath.Join(dir, name+".mar")+`"/HOST `+upper+".MAR")
		shareFindRun(t, d, "MACRO/NOLIST "+upper)
		shareFindRun(t, d, "LINK/NOMAP/SHAREABLE "+upper)
		shareFindRun(t, d, "COPY/BINARY "+upper+`.EXE "`+filepath.Join(out, name+".exe")+`"/HOST`)

		got, err := vmsimage.ReadImage(link68ReadBytes(t, filepath.Join(out, name+".exe")))
		if err != nil {
			t.Fatal(err)
		}

		want, err := vmsimage.ReadImage(link68ReadBytes(t, filepath.Join(dir, "vax", "round2", name+".exe")))
		if err != nil {
			t.Fatal(err)
		}

		if got.DSTBlockCount() != want.DSTBlockCount() {
			t.Errorf("%s: a DST of %d blocks, VMS's %d", upper, got.DSTBlockCount(), want.DSTBlockCount())
		}
	}
}
