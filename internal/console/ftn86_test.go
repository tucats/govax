package console

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsimage"
)

// ftn86FileDifferences are the programs that don't print their name.out when FTN86.COM runs
// them, with why. A name.out is a terminal's output, and the procedure's runs write a file
// and read a file; each of these also runs with its N variant, compiled /NOOPTIMIZE, and its
// R variant, compiled /RECURSIVE.
var ftn86FileDifferences = map[string]string{
	"ASTS":     "writes a line by $QIO to SYS$OUTPUT, here a file and not a terminal",
	"DIALOG":   "a $ prompt's line isn't ended by a terminal read, so the next record starts its own",
	"FORMATS":  "a $ prompt's line isn't ended by a terminal read, so the next record starts its own",
	"INQUIRE":  "the directory is the program's own, not [000000]",
	"IOSTMT":   "writes unit 5, which only a terminal allows: RMS refuses it in a procedure",
	"SERVICES": "no prompt for a read of a file",
	"WHET":     "a kernel with no name.out",
}

// runFTN86 builds the volume as exchange.cmd does and runs FTN86.COM on it, returning the
// log and the console, whose default directory is the volume's. It's the procedure VMS
// runs, here as a rehearsal, FTNRTL built from its sources by it and every program linked
// against that.
func runFTN86(t *testing.T) (string, *Console, *Dispatcher) {
	t.Helper()

	c, buf := newTestConsole(t)
	c.Paths = respath.New(nil, bootdata.FS)
	d := NewDispatcher(c, loadEvaxGrammar(t))
	installPlainDebugger(c)

	if err := c.RunHostProcedure("startup.com", d.Dispatch); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "ftn86.dsk")
	if err := c.cmd().InitializeContainer(path, 60000, "FTN86", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.cmd().Mount("DUA5:", path, true); err != nil {
		t.Fatal(err)
	}

	t.Chdir(filepath.Join("..", ".."))

	script, err := os.ReadFile(filepath.Join("testdata", "ftn86", "exchange.cmd"))
	if err != nil {
		t.Fatal(err)
	}

	buf.Reset()

	for line := range strings.SplitSeq(string(script), "\n") {
		for _, skip := range []string{"!", "INITIALIZE/", "MOUNT/", "DISMOUNT", "QUIT"} {
			if strings.HasPrefix(line, skip) {
				line = ""
			}
		}

		if line == "" {
			continue
		}

		shareFindRun(t, d, line)

		if out := buf.String(); out != "" {
			t.Fatalf("%s said: %.300s", line, out)
		}
	}

	// The volume as VMS has it, on DUA1, so the names programs print are the same.
	shareFindRun(t, d, "SET DEFAULT DUA2:[000000]")
	shareFindRun(t, d, "DISMOUNT DUA5")

	if err := c.cmd().Mount("DUA1:", path, true); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, "SET DEFAULT DUA1:[000000]")
	buf.Reset()

	// As from an earlier run in the same session. This doesn't test what that did on
	// VMS: there LINK translates the name for FTNRTL/OPTIONS and looks for the image it
	// is to make, and govax's LINK doesn't translate it (the phase doc's log).
	shareFindRun(t, d, "DEFINE/NOLOG FTNRTL DUA1:[FTN86]FTNRTL.EXE")
	shareFindRun(t, d, "@FTN86/OUTPUT=FTN86.LOG")

	buf.Reset()

	return ftn86Log(t, d, "FTN86.LOG"), c, d
}

// ftn86Log is a log as COPY brings it to the host, as VMS's was: its records by their
// carriage control, so what a $ prompt left open goes on with the next record.
func ftn86Log(t *testing.T, d *Dispatcher, name string) string {
	t.Helper()

	host := filepath.Join(t.TempDir(), "log.txt")
	shareFindRun(t, d, "COPY/QUIET DUA1:[000000]"+name+` "`+host+`"/HOST`)

	text, err := os.ReadFile(host)
	if err != nil {
		t.Fatal(err)
	}

	return string(text)
}

// ftn86Sections is a log's sections by the name each starts with.
func ftn86Sections(log string) map[string]string {
	sections := map[string]string{}
	parts := regexp.MustCompile(`(?m)^==== (\S+)\n`).Split(log, -1)
	names := regexp.MustCompile(`(?m)^==== (\S+)\n`).FindAllStringSubmatch(log, -1)

	for i, name := range names {
		sections[name[1]] = parts[i+1]
	}

	return sections
}

// TestFTN86Procedure rehearses the VMS round (docs/PHASE-86 - FORTRAN milestone.md, subtask
// 4) on govax. FTNRTL builds from its sources with no message, and each program prints what
// the corpus says it should, but for ftn86FileDifferences, which it's held to as well: an
// entry no longer needed is reported. A program's R variant, compiled /RECURSIVE, isn't
// compared if recursiveDifferences lists it, since what it prints there may be the same by
// chance (TestFortranPrograms').
func TestFTN86Procedure(t *testing.T) {
	log, _, _ := runFTN86(t)
	sections := ftn86Sections(log)

	if _, ok := sections["END"]; !ok {
		t.Fatalf("the procedure didn't reach its end:\n%.2000s", log)
	}

	for line := range strings.SplitSeq(strings.TrimRight(sections["FTNRTL"], "\n"), "\n") {
		if line != `  $STATUS == "%X10000001"` {
			t.Errorf("building FTNRTL said: %s", line)
		}
	}

	var sources []string

	for _, pattern := range []string{"testdata/fortran/*.for", "testdata/opt84/kernels/*.for", "testdata/fortran/modules/*"} {
		found, err := filepath.Glob(filepath.FromSlash(pattern))
		if err != nil {
			t.Fatal(err)
		}

		sources = append(sources, found...)
	}

	checked := 0

	for _, source := range sources {
		base := strings.TrimSuffix(filepath.Base(source), ".for")
		upper := strings.ToUpper(base)
		want := filepath.Join(filepath.Dir(source), base+".out")
		directory := upper

		if !strings.HasSuffix(source, ".for") {
			want = filepath.Join(source, base+".out")
			directory = "M_" + upper
		}

		wanted, err := os.ReadFile(want)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}

		why, differs := ftn86FileDifferences[upper]
		_, recursiveDiffers := recursiveDifferences[base]

		for _, variant := range []string{directory, directory + "N", directory + "R"} {
			got, ok := sections[variant]
			if !ok {
				t.Errorf("%s isn't in the log", variant)

				continue
			}

			checked++

			if recursiveDiffers && variant == directory+"R" {
				continue
			}

			// As in TestFortranPrograms: /OPTIMIZE moves the code, so what a program
			// prints that says where its code is can't be the same.
			switch same := maskCodeAddresses(got) == maskCodeAddresses(string(wanted)); {
			case differs && same:
				t.Errorf("%s printed its name.out, though ftn86FileDifferences says: %s", variant, why)
			case !differs && !same:
				t.Errorf("%s printed:\n%s\nwant:\n%s", variant, got, wanted)
			}
		}
	}

	t.Logf("%d runs", checked)
}

// ftn86VMSDifferences are the sections of VMS 7.1's log (testdata/ftn86/vax/ftn86.log) that
// aren't what the same procedure prints on govax, with why. Each runs with its N and R
// variants; a name ending in R is the /RECURSIVE variant alone.
var ftn86VMSDifferences = map[string]string{
	"SERVICES":    "$GETCHN, a service govax lacks, which the program expects to be told so of",
	"SYSROUTINES": "SECNDS(T1+1.0) less than a second after T1 is a day more, and VMS's run got there in time",
	"MATHFNS":     "CSIN's last bit: MTHRTL's complex functions aren't always VMS's in it",
	"POWERS":      "OTS$POWRJ of a negative power: its last bit isn't always VMS's (the phase doc's round 2)",
	"POWBITS":     "the same, in exact bits",
	"RECURAST": "WAITED: on the VAX the 10 ms timer's AST can run SPIN six deep and end before " +
		"the wait loop's first test (a quantum end after $SETIMR, or simh's clock catching up), " +
		"as it did in two of the three builds; govax's clock is the instructions'",
	"ALIGNR":   "recursiveDifferences': MOD(%LOC(RC), 16) of a record in the frame, where VMS's stack put it on 16",
	"OPTFLOWR": "recursiveDifferences': ENTRY AGAIN reads an unsaved local, whatever the stack holds there",
}

// ftn86AwaitingVMS are corpus programs newer than VMS's last run of the procedure, which
// the next VMS round is to run. None are, since Phase 89's round (subtask 12).
var ftn86AwaitingVMS = []string{}

// ftn86AwaitingRecursive says VMS's last run had no R variants, each program compiled
// /RECURSIVE. It has since Phase 89's round.
const ftn86AwaitingRecursive = false

// ftn86Lines is a log's section as both sides can be held to it: without the commands SET
// VERIFY echoed on VMS, or the options files LINK echoes then. The two numbers a traceback
// prints for a frame in a shareable image are where the image was put.
func ftn86Lines(text string) string {
	var lines []string

	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "$ ") || strings.HasPrefix(line, "! ") || line == "FTNRTL/SHAREABLE" {
			continue
		}

		if ftn86Frame.MatchString(line) {
			line = "(a frame in FTNRTL)"
		}

		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// ftn86Statuses is a section's lines that show $STATUS.
func ftn86Statuses(text string) string {
	var lines []string

	for line := range strings.SplitSeq(text, "\n") {
		if strings.Contains(line, "$STATUS") {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

var ftn86Frame = regexp.MustCompile(`^ +[0-9A-F]{8}  [0-9A-F]{8}$`)

// TestFTN86Oracle holds the procedure's run on govax to VMS 7.1's: programs govax's compiler
// made, assembled and linked by VMS's MACRO and LINK against an FTNRTL built there from its
// sources, on a real VAX's instruction set and VMS's own LIBRTL, MTHRTL, and RMS.
func TestFTN86Oracle(t *testing.T) {
	vms, err := os.ReadFile(filepath.Join("..", "..", "testdata", "ftn86", "vax", "ftn86.log"))
	if err != nil {
		t.Fatal(err)
	}

	log, _, _ := runFTN86(t)
	got, want := ftn86Sections(log), ftn86Sections(ftn86Lines(string(vms)))

	for _, name := range ftn86AwaitingVMS {
		for _, variant := range []string{name, name + "N", name + "R"} {
			if _, ok := want[variant]; ok {
				t.Errorf("%s is in VMS's log: take it out of ftn86AwaitingVMS", variant)
			}

			delete(got, variant)
		}
	}

	if ftn86AwaitingRecursive {
		for name := range got {
			if _, ok := got[strings.TrimSuffix(name, "R")+"N"]; ok && strings.HasSuffix(name, "R") {
				if _, ok := want[name]; ok {
					t.Errorf("%s is in VMS's log: set ftn86AwaitingRecursive false", name)
				}

				delete(got, name)
			}
		}
	}

	if len(got) != len(want) {
		t.Errorf("%d sections, VMS's %d", len(got), len(want))
	}

	seen := map[string]bool{}

	// FTNRTL's section on VMS has its options file too, which LINK echoes under SET
	// VERIFY: what both say is the status of each command.
	if a, b := ftn86Statuses(got["FTNRTL"]), ftn86Statuses(want["FTNRTL"]); a != b {
		t.Errorf("building FTNRTL ended with:\n%s\nVMS's:\n%s", a, b)
	}

	delete(want, "FTNRTL")

	for name, text := range want {
		base := strings.TrimSuffix(strings.TrimSuffix(name, "N"), "R")
		if _, ok := ftn86VMSDifferences[name]; ok {
			base = name
		}

		_, expected := ftn86VMSDifferences[base]
		same := ftn86Lines(got[name]) == text

		switch {
		case same && expected:
			seen[base] = seen[base] || false
		case !same && expected:
			seen[base] = true
		case !same:
			t.Errorf("%s printed:\n%s\nVMS's:\n%s", name, ftn86Lines(got[name]), text)
		}
	}

	for name := range ftn86VMSDifferences {
		if !seen[name] {
			t.Errorf("%s now matches VMS: take it out of ftn86VMSDifferences", name)
		}
	}

	// Every positive power is exact, in all four formats.
	wantBits, gotBits := strings.Split(want["POWBITS"], "\n"), strings.Split(got["POWBITS"], "\n")

	for i, line := range wantBits {
		fields := strings.Fields(line)
		if len(fields) > 3 && !strings.HasPrefix(fields[0], "C") && !strings.HasPrefix(fields[2], "-") &&
			(i >= len(gotBits) || strings.TrimRight(gotBits[i], " ") != strings.TrimRight(line, " ")) {
			t.Errorf("a positive power's bits differ from VMS's: %q", line)
		}
	}
}

// The game's sections of FTN86D.COM on VMS 7.1: DUNGEON built there, its log local since it
// holds the game's text (testdata/disks/ftn86d-*.log), so its digests stand for it. Each
// section's R twin, DUNGEON compiled /RECURSIVE, is to print the same.
var ftn86DungeonSHA = map[string]string{
	"GAME":    "a324c46d657770e84b7aeade6af4110736fe20ee520eb741e804c450aae5e24c",
	"SAVE":    "c5fccbe574ec68dc7ee08fcda7641f175f6457732105ee464cf89c4c7aac0f22",
	"RESTORE": "c36a0a467b8a2968f3e500421c6ad09cefabd8ac17c364d161655d01d697d103",
}

// TestFTN86Dungeon runs FTN86D.COM on govax and holds what it prints to VMS's digests. It
// needs the kit, through the link README.md gives, and skips without it.
func TestFTN86Dungeon(t *testing.T) {
	dungeonSources(t)

	kit := filepath.Join("..", "..", "testdata", "disks", "dungeon-3.2A", "dungeon.f")
	if _, err := os.Stat(kit); err != nil {
		t.Skipf("no link to the kit: %v", err)
	}

	_, _, d := runFTN86(t)

	script, err := os.ReadFile(filepath.Join("testdata", "ftn86", "exchanged.cmd"))
	if err != nil {
		t.Fatal(err)
	}

	text := strings.ReplaceAll(string(script), "DUA5", "DUA1")

	for line := range strings.SplitSeq(text, "\n") {
		for _, skip := range []string{"!", "MOUNT/", "DISMOUNT", "QUIT"} {
			if strings.HasPrefix(line, skip) {
				line = ""
			}
		}

		if line != "" {
			shareFindRun(t, d, line)
		}
	}

	shareFindRun(t, d, "@FTN86D/OUTPUT=FTN86D.LOG")

	sections := ftn86Sections(ftn86Log(t, d, "FTN86D.LOG"))

	for name, want := range ftn86DungeonSHA {
		for _, section := range []string{name, name + "R"} {
			sum := sha256.Sum256([]byte(sections[section]))
			if got := hex.EncodeToString(sum[:]); got != want {
				t.Errorf("%s: %d bytes, SHA-256 %s, VMS's %s", section, len(sections[section]), got, want)
			}
		}
	}
}

// TestFTN86Objects holds govax's MACRO and LINK to VMS's on FTNRTL as VMS built it, from the
// copy of its sources in testdata/ftn86/vax/mar, so FTNRTL can go on changing: the objects
// record for record but for the three header records (time, tool, command), and the image's
// blocks but for its header and what starts with the debug symbol table (the link's time and
// the linker's name, whose length moves what follows).
func TestFTN86Objects(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "ftn86", "vax")

	sources, err := filepath.Glob(filepath.Join(dir, "mar", "*.mar"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("no sources: %v", err)
	}

	d, c, buf := dungeonConsole(t, false)

	opt, err := filepath.Abs(filepath.Join(dir, "mar", "ftnrtl.opt"))
	if err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, `COPY/QUIET "`+opt+`"/HOST FTNRTL.OPT`)

	for _, source := range sources {
		abs, err := filepath.Abs(source)
		if err != nil {
			t.Fatal(err)
		}

		name := strings.ToUpper(strings.TrimSuffix(filepath.Base(source), ".mar"))
		shareFindRun(t, d, `COPY/QUIET "`+abs+`"/HOST `+name+`.MAR`)
		shareFindRun(t, d, "MACRO/NOLIST "+name)

		if out := buf.String(); out != "" {
			t.Fatalf("MACRO %s said: %s", name, out)
		}

		got, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: name + ".OBJ"}, rms.VariableRecords)
		if err != nil {
			t.Fatal(err)
		}

		vmsObj, err := filepath.Abs(filepath.Join(dir, strings.ToLower(name)+".obj"))
		if err != nil {
			t.Fatal(err)
		}

		want, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Host: true, Name: vmsObj}, rms.VariableRecords)
		if err != nil {
			t.Fatal(err)
		}

		if len(got) != len(want) {
			t.Errorf("%s has %d records, VMS's %d", name, len(got), len(want))

			continue
		}

		for i := 3; i < len(got); i++ {
			if !bytes.Equal(got[i], want[i]) {
				t.Errorf("%s's record %d differs from VMS's", name, i)
			}
		}
	}

	shareFindRun(t, d, "LINK/SHAREABLE=FTNRTL FTNRTL/OPTIONS")

	if out := buf.String(); out != "" {
		t.Fatalf("LINK said: %s", out)
	}

	host := filepath.Join(t.TempDir(), "ftnrtl.exe")
	shareFindRun(t, d, `COPY/QUIET/BINARY FTNRTL.EXE "`+host+`"/HOST`)

	got, err := os.ReadFile(host)
	if err != nil {
		t.Fatal(err)
	}

	want, err := os.ReadFile(filepath.Join(dir, "ftnrtl.exe"))
	if err != nil {
		t.Fatal(err)
	}

	img, err := vmsimage.ReadImage(want)
	if err != nil {
		t.Fatal(err)
	}

	if img.DSTVBN < 2 {
		t.Fatalf("VMS's image has its debug symbol table at VBN %d", img.DSTVBN)
	}

	for block := 1; block < int(img.DSTVBN)-1; block++ {
		a, b := block*512, block*512+512
		if b > len(got) || !bytes.Equal(got[a:b], want[a:b]) {
			t.Errorf("block %d of the image differs from VMS's", block)
		}
	}
}
