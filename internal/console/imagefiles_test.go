package console_test

import (
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
)

// A process 1 image's files are closed when the image ends, as RMS's
// rundown closes them on VMS: when it exits; when it was stopped and
// never resumed, at the next RUN; and whatever it was doing, when the
// machine is rebuilt (ZERO) or govax exits (EndSession). Each test's
// image writes a record without closing its file; the record is only on
// the volume once the file is closed.

// leftOpenFile is the file the writers write.
const leftOpenFile = "DUA0:[000000]LEFTOPEN.DAT"

// spinningWriterSource is writerSource, but it loops forever instead of
// returning once its record is written.
const spinningWriterSource = `	.title	spinner
	.psect	data,noexe,wrt
fab:	$fab	fnm=<DUA0:[000000]LEFTOPEN.DAT>,rfm=var,rat=cr,fac=put
rab:	$rab	fab=fab,rbf=rec,rsz=reclen
rec:	.ascii	/written, never closed/
reclen = .-rec
	.psect	code,exe,nowrt
	.entry	start,^m<>
	$create	fab=fab
	blbc	r0,spin
	$connect rab=rab
	blbc	r0,spin
	$put	rab=rab
spin:	brb	spin
	.end	start
`

// writerConsole boots a console with a fresh volume on DUA0, and returns
// it with a dispatcher for its commands.
func writerConsole(t *testing.T) (*console.Console, *console.Dispatcher) {
	t.Helper()

	c, _ := scheduledConsole(t, longQuantum, brbSelf)

	path := filepath.Join(t.TempDir(), "work.dsk")
	if err := c.InitializeContainer(path, 400, "WORK", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	return c, console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
}

// runImage runs the image at host path exe with RUN.
func runImage(t *testing.T, d *console.Dispatcher, exe string) {
	t.Helper()

	if err := d.Dispatch(`RUN "` + exe + `"`); err != nil {
		t.Fatalf("RUN %s: %v", exe, err)
	}
}

// wantRecords checks how many records the writers' file holds.
func wantRecords(t *testing.T, c *console.Console, want int, when string) {
	t.Helper()

	lines, _, err := c.ContainerSession.ReadRecordFile(rms.FileLocation{Name: leftOpenFile}, rms.TextRecords)
	if err != nil {
		t.Fatalf("%s: reading the file: %v", when, err)
	}

	if len(lines) != want || (want == 1 && string(lines[0]) != "written, never closed") {
		t.Errorf("%s: the file holds %q; want %d record(s)", when, lines, want)
	}
}

// startSpinner runs the spinning writer until the instruction limit
// stops it, its record written and its file open.
func startSpinner(t *testing.T, c *console.Console, d *console.Dispatcher) {
	t.Helper()

	c.Engine.SetLimits(100_000, 0)
	runImage(t, d, buildImage(t, c, "spinner", spinningWriterSource))
	c.Engine.SetLimits(0, 0)

	if !c.ImageActive() {
		t.Fatal("the spinner isn't stopped in its image")
	}

	wantRecords(t, c, 0, "while the spinner is stopped")
}

// TestImageExit_closesFiles: an image that returns has its files closed.
func TestImageExit_closesFiles(t *testing.T) {
	c, d := writerConsole(t)

	runImage(t, d, buildImage(t, c, "writer", writerSource))
	wantRecords(t, c, 1, "after the image exits")
}

// TestAbandonedImage_nextRun: an image that was stopped and never
// resumed is run down when RUN starts the next one. (The next one is
// only loaded, /NOEXECUTE, so that its own exit doesn't close the file.)
func TestAbandonedImage_nextRun(t *testing.T) {
	c, d := writerConsole(t)
	startSpinner(t, c, d)

	exe := buildChildImage(t, c)
	if err := d.Dispatch(`RUN/NOEXECUTE "` + exe + `"`); err != nil {
		t.Fatalf("RUN/NOEXECUTE: %v", err)
	}

	wantRecords(t, c, 1, "after the next RUN")

	if c.ImageActive() {
		t.Error("the spinner's image is still active")
	}
}

// TestAbandonedImage_zero: rebuilding the machine closes process 1's
// files. (ZERO is a kernel-mode command, and the spinner stopped in user
// mode.)
func TestAbandonedImage_zero(t *testing.T) {
	c, d := writerConsole(t)
	startSpinner(t, c, d)

	c.Engine.SetModeStack(vax.Kernel, false)

	if err := c.Zero(); err != nil {
		t.Fatal(err)
	}

	wantRecords(t, c, 1, "after ZERO")
}

// TestAbandonedImage_endSession: govax's exit closes process 1's files
// before it dismounts the volumes.
func TestAbandonedImage_endSession(t *testing.T) {
	c, d := writerConsole(t)
	startSpinner(t, c, d)

	c.EndSession()
	wantRecords(t, c, 1, "after EndSession")

	if c.ImageActive() {
		t.Error("the spinner's image is still active")
	}
}
