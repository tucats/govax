package console

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestRun_imageOnVolume assembles and links a program onto a mounted
// volume, then runs it from there by the file name rules MACRO and LINK
// follow: a full specification, one without a type (EXE), and a bare name
// once the default directory is on the volume.
func TestRun_imageOnVolume(t *testing.T) {
	c := newBootableConsole(t)
	mountFreshContainer(t, c, "DUA0")

	dir := t.TempDir()
	writeHostFile(t, filepath.Join(dir, "main.mar"), "\t.PSECT\tCODE,NOWRT,EXE\n"+
		"\t.ENTRY\tSTART,^M<>\n\tMOVL\t#42,R0\n\tRET\n\t.END\tSTART\n")

	if err := c.Macro(MacroOptions{Source: filepath.Join(dir, "main.mar"), Object: "DUA0:[000000]MAIN.OBJ"}); err != nil {
		t.Fatal(err)
	}

	if err := c.Link(LinkOptions{Objects: []string{"DUA0:[000000]MAIN"}}); err != nil {
		t.Fatal(err)
	}

	if got := runImage(t, c, "DUA0:[000000]MAIN.EXE"); got != 42 {
		t.Errorf("R0 = %d, want 42", got)
	}

	// The same image, named without its type, and by its bare name once the
	// default directory is on the volume. (A console runs its kernel's
	// setup once, so these are read rather than run again.)
	want, err := c.readImage("DUA0:[000000]MAIN.EXE", true)
	if err != nil {
		t.Fatal(err)
	}

	if got, err := c.readImage("DUA0:[000000]MAIN", true); err != nil || !bytes.Equal(got, want) {
		t.Errorf("DUA0:[000000]MAIN: %d bytes, %v", len(got), err)
	}

	if err := c.SetDefault("DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	if got, err := c.readImage("MAIN", true); err != nil || !bytes.Equal(got, want) {
		t.Errorf("MAIN: %d bytes, %v", len(got), err)
	}

	if _, err := c.readImage("NOSUCH", true); !errors.Is(err, vmserrors.New(vmserrors.RMS_IMAGENOTFOUND)) {
		t.Errorf("NOSUCH: err = %v, want IMAGENOTFOUND", err)
	}
}

// TestDispatch_runHost runs a host image with /HOST after its name while
// the default directory is on a volume, where the same bare name would
// otherwise be looked for.
func TestDispatch_runHost(t *testing.T) {
	c := newBootableConsole(t)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)
	mountFreshContainer(t, c, "DUA0")

	// RUN needs the kernel's shims, as runImage sets them up.
	if _, _, err := c.Assemble(kernelPath(t)); err != nil {
		t.Fatal(err)
	}

	runKernelInitialize(t, c)

	dir := t.TempDir()
	assembleFixture(t, c, "entry", dir)

	exe := filepath.Join(dir, "entry.exe")
	
	if err := c.Link(LinkOptions{Objects: []string{filepath.Join(dir, "entry")}}); err != nil {
		t.Fatal(err)
	}

	if err := c.SetDefault("DUA0:[000000]"); err != nil {
		t.Fatal(err)
	}

	if _, err := c.readImage("entry", true); err == nil {
		t.Fatal("a bare name was found on the host while the default is on a volume")
	}

	if err := d.Dispatch(`RUN/NOEXECUTE "` + exe + `"/HOST`); err != nil {
		t.Errorf("RUN .../HOST: %v", err)
	}
}

// TestRun_recordUpdates runs testdata/rms49/update.mar, a MACRO-32
// program that uses $FIND, $UPDATE, $TRUNCATE, and $REWIND on a file on
// a volume (docs/PHASE-49.md); it returns 999, or the step that failed.
func TestRun_recordUpdates(t *testing.T) {
	c := newBootableConsole(t)
	mountFreshContainer(t, c, "DUA0")

	if err := c.Macro(MacroOptions{
		Source: filepath.Join("..", "..", "testdata", "rms49", "update.mar"), Object: "DUA0:[000000]UPDATE.OBJ",
	}); err != nil {
		t.Fatal(err)
	}

	if err := c.Link(LinkOptions{Objects: []string{"DUA0:[000000]UPDATE"}}); err != nil {
		t.Fatal(err)
	}

	if got := runImageBounded(t, c, "DUA0:[000000]UPDATE.EXE", 1_000_000); got != 999 {
		t.Errorf("R0 = %d, want 999 (another value is the step that failed)", got)
	}
}
