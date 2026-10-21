package console

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// Tests of a command procedure's process-permanent files (docs/PHASE-71 -
// process-permanent files.md, subtask 6; internal/dclshell/dclppf.go).

// procedureName is one of the supervisor-mode SYS$ names at a point in a
// procedure: its process-permanent file, if it has a definition there.
type procedureName struct {
	file    lnm.PermanentFile
	defined bool
	attrs   uint32
	confine bool
}

// supervisorName returns name's supervisor-mode definition in c's process
// table, read as a process-permanent file string.
func supervisorName(t *testing.T, c *Console, name string) procedureName {
	t.Helper()

	e, err := c.Logicals.Translate(lnm.ProcessTableName, name, lnm.Supervisor, 0)
	if err != nil || e.Mode != lnm.Supervisor {
		return procedureName{}
	}

	pf, ok := lnm.ParsePermanentFile(e.Equivalences[0].Value)
	if !ok {
		t.Fatalf("%s is %q, not a process-permanent file string", name, e.Equivalences[0].Value)
	}

	return procedureName{file: pf, defined: true, attrs: e.Equivalences[0].Attrs, confine: e.Attrs&lnm.AttrConfine != 0}
}

// executiveOutput returns the executive-mode SYS$OUTPUT's value, the
// terminal's process-permanent file.
func executiveOutput(t *testing.T, c *Console) lnm.PermanentFile {
	t.Helper()

	e, err := c.Logicals.Translate(lnm.ProcessTableName, "SYS$OUTPUT", lnm.Executive, 0)
	if err != nil {
		t.Fatal(err)
	}

	pf, ok := lnm.ParsePermanentFile(e.Equivalences[0].Value)
	if !ok {
		t.Fatalf("executive SYS$OUTPUT is %q", e.Equivalences[0].Value)
	}

	return pf
}

// checkpoints runs the procedure the command line at runs, with each
// CHECK line it (or a procedure it runs) reaches recording SYS$INPUT's and
// SYS$OUTPUT's supervisor-mode names there, and returns them in order.
func checkpoints(t *testing.T, d *Dispatcher, c *Console, at string) [][2]procedureName {
	t.Helper()

	var seen [][2]procedureName

	route := func(line string) error {
		if line == "CHECK" {
			seen = append(seen, [2]procedureName{supervisorName(t, c, "SYS$INPUT"), supervisorName(t, c, "SYS$OUTPUT")})

			return nil
		}

		return d.Dispatch(line)
	}

	c.SetRoute(route)
	defer c.SetRoute(d.Dispatch)

	if err := route(at); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("%s: %v", at, err)
	}

	return seen
}

// TestProcedureFiles_names: DCL's supervisor-mode SYS$INPUT and
// SYS$OUTPUT at each level, as VMS 7.1 showed them (testdata/ppf71): at
// level 0 SYS$OUTPUT is the executive one's entry and there is no
// SYS$INPUT; a procedure's SYS$INPUT is one entry, kept for a nested
// procedure, naming the innermost procedure's file; each /OUTPUT file is
// an entry of its own; the names are put back as each level ends, and
// the files closed.
func TestProcedureFiles_names(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	dir := t.TempDir()

	inner := writeProcedure(t, dir, "inner.com", "$ CHECK", `$ WRITE SYS$OUTPUT "to the log"`)
	outer := writeProcedure(t, dir, "outer.com",
		"$ CHECK",
		"$ @"+inner+`/OUTPUT="`+filepath.Join(dir, "inner.log")+`"`,
		"$ CHECK")

	terminal := executiveOutput(t, c)

	level0 := supervisorName(t, c, "SYS$OUTPUT")
	if !level0.defined || level0.file != terminal || !level0.confine || level0.attrs != lnm.AttrConcealed|lnm.AttrTerminal {
		t.Errorf("level 0's SYS$OUTPUT = %+v, want the terminal's entry %+v, CONFINE, concealed and terminal", level0, terminal)
	}

	if in := supervisorName(t, c, "SYS$INPUT"); in.defined {
		t.Errorf("level 0 has a supervisor-mode SYS$INPUT: %+v", in)
	}

	buf.Reset()

	seen := checkpoints(t, d, c, "@"+outer)
	if len(seen) != 3 {
		t.Fatalf("%d checkpoints, want 3\n%s", len(seen), buf.String())
	}

	outerIn, innerIn := seen[0][0], seen[1][0]

	switch {
	case !outerIn.defined || !outerIn.confine || outerIn.attrs != lnm.AttrTerminal:
		t.Errorf("the outer procedure's SYS$INPUT = %+v, want CONFINE, terminal", outerIn)
	case !strings.HasSuffix(outerIn.file.Device, "outer.com"):
		t.Errorf("the outer procedure's SYS$INPUT names %q, want its file", outerIn.file.Device)
	case seen[0][1].file != terminal:
		t.Errorf("the outer procedure's SYS$OUTPUT = %+v, want the terminal's entry", seen[0][1])
	}

	switch {
	case innerIn.file.IFI != outerIn.file.IFI:
		t.Errorf("the nested procedure's SYS$INPUT is entry %04X, want the outer one's, %04X", innerIn.file.IFI, outerIn.file.IFI)
	case !strings.HasSuffix(innerIn.file.Device, "inner.com"):
		t.Errorf("the nested procedure's SYS$INPUT names %q, want its file", innerIn.file.Device)
	}

	log := seen[1][1]

	switch {
	case !log.defined || log.file.IFI == terminal.IFI || log.file.IFI == outerIn.file.IFI:
		t.Errorf("the /OUTPUT file's SYS$OUTPUT = %+v, want an entry of its own", log)
	case log.file.IFI&0x3FC0 != 0x0100:
		t.Errorf("the /OUTPUT file's word %04X doesn't carry PRN", log.file.IFI)
	case !strings.HasSuffix(log.file.Device, "inner.log"):
		t.Errorf("the /OUTPUT file's SYS$OUTPUT names %q, want the log", log.file.Device)
	}

	if seen[2][0] != outerIn || seen[2][1].file != terminal {
		t.Errorf("after the nested procedure: %+v, want the outer procedure's names again", seen[2])
	}

	if out := supervisorName(t, c, "SYS$OUTPUT"); out.file != terminal {
		t.Errorf("at level 0 again, SYS$OUTPUT = %+v", out)
	}

	if in := supervisorName(t, c, "SYS$INPUT"); in.defined {
		t.Errorf("at level 0 again, SYS$INPUT = %+v", in)
	}

	if strings.Contains(buf.String(), "to the log") {
		t.Errorf("the nested procedure's output reached the terminal:\n%s", buf.String())
	}

	// The procedure's entry and the log's are closed: a $CREATE of their
	// names' strings, copied, finds no file (RMS$_IFI at $OPEN, Phase
	// 71's subtask 4), and the entries were the RTL's.
	for _, pf := range []lnm.PermanentFile{outerIn.file, log.file} {
		if _, ok := c.RTL.PermanentEntryFor(pf.IFI); ok {
			t.Errorf("entry %04X is still open", pf.IFI)
		}
	}
}

// TestProcedureFiles_volume: a procedure on a volume, run with /OUTPUT to
// a volume file: SYS$INPUT names the volume's device as the node's
// physical name, with a colon, and its entry describes the procedure's
// file; the log is a VFC print file with the procedure's output, and an
// image's records (here TYPE SYS$INPUT's data lines) go there too.
func TestProcedureFiles_volume(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)
	dir := t.TempDir()

	mustDispatch(t, d,
		`INITIALIZE/CONTAINER "`+filepath.Join(dir, "v.dsk")+`" /SIZE=800 PPF`,
		`MOUNT/WRITE DUA1 "`+filepath.Join(dir, "v.dsk")+`"`)

	src := writeProcedure(t, dir, "vol.com", "$ CHECK", "$ TYPE SYS$INPUT", "data one", "data two", `$ WRITE SYS$OUTPUT "done"`)
	mustDispatch(t, d, "COPY "+src+"/HOST DUA1:[000000]VOL.COM")

	buf.Reset()

	seen := checkpoints(t, d, c, "@DUA1:[000000]VOL/OUTPUT=DUA1:[000000]VOL.LOG")
	if len(seen) != 1 {
		t.Fatalf("%d checkpoints, want 1\n%s", len(seen), buf.String())
	}

	if in := seen[0][0]; in.file.Device != "_"+c.RTL.NodeName+"$DUA1:" {
		t.Errorf("SYS$INPUT names %q, want the volume's device", in.file.Device)
	}

	if out := seen[0][1]; out.file.Device != "_"+c.RTL.NodeName+"$DUA1:" {
		t.Errorf("SYS$OUTPUT names %q, want the volume's device", out.file.Device)
	}

	if buf.Len() != 0 {
		t.Errorf("the terminal got:\n%s", buf.String())
	}

	records, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: "DUA1:[000000]VOL.LOG"}, rms.TextRecords)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := string(JoinRecords(records)), "data one\ndata two\ndone\n"; got != want {
		t.Errorf("VOL.LOG = %q, want %q", got, want)
	}
}

// TestProcedureFiles_vminit: VMINIT in a procedure (as vax.init's) makes a
// new process 1, which takes over DCL's procedure files: an image's
// SYS$INPUT after it is still the procedure's data lines.
func TestProcedureFiles_vminit(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)

	proc := writeProcedure(t, t.TempDir(), "init.com",
		"$ VMINIT", "$ CHECK", "$ TYPE SYS$INPUT", "after vminit", `$ PRINT "done"`)

	buf.Reset()

	var (
		checked bool
		open    bool
	)

	route := func(line string) error {
		if line != "CHECK" {
			return d.Dispatch(line)
		}

		in := supervisorName(t, c, "SYS$INPUT")
		_, open = c.RTL.PermanentEntryFor(in.file.IFI)
		checked = in.defined

		return nil
	}

	c.SetRoute(route)
	defer c.SetRoute(d.Dispatch)

	if err := route("@" + proc); err != nil {
		t.Fatalf("@init: %v\n%s", err, buf.String())
	}

	if !checked || !open {
		t.Errorf("after VMINIT, SYS$INPUT defined %v, its entry the new process 1's %v", checked, open)
	}

	if got := buf.String(); !strings.HasSuffix(got, "after vminit\ndone\n") {
		t.Errorf("output:\n%s", got)
	}
}
