package console

import (
	"bytes"
	"io/fs"
	"path"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/ftnrtl"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// FTNRTL, the FORTRAN run-time library (docs/PHASE-74 - FORTRAN
// plan.md, subtask 4): a MACRO program calls its routines, links against
// it with no options file, and runs, with the image built into govax or
// one of the user's own, in both paging modes.

// ftnCaller calls FTN$VERSION with a string 13 characters long (the text
// is 11, so two blanks pad it), says whether its value is V1.0's, then
// ends with STOP's display.
const ftnCaller = `	.TITLE	FTNCALL
	.PSECT	DATA,WRT,NOEXE,LONG
TEXT:	.BLKB	13
DESC:	.WORD	13
	.BYTE	14,1
	.ADDRESS TEXT
OK:	.ASCID	/version ok/
BAD:	.ASCID	/version wrong/
DISP:	.ASCID	/END OF RUN/
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<R2>
	PUSHAQ	DESC
	CALLS	#1,G^FTN$VERSION
	MOVL	R0,R2
	PUSHAQ	DESC
	CALLS	#1,G^LIB$PUT_OUTPUT
	MOVAQ	OK,R0
	CMPL	R2,#^X00010000
	BEQL	10$
	MOVAQ	BAD,R0
10$:	PUSHL	R0
	CALLS	#1,G^LIB$PUT_OUTPUT
	PUSHAQ	DISP
	CALLS	#1,G^FTN$STOP
	MOVL	#4,R0
	RET
	.END	START
`

// ftnStopper calls FTN$STOP with no display, and once with an omitted
// argument would do the same.
const ftnStopper = `	.TITLE	FTNSTOP
	.PSECT	CODE,NOWRT,EXE,LONG
	.ENTRY	START,^M<>
	CLRL	-(SP)
	CALLS	#1,G^FTN$STOP
	MOVL	#4,R0
	RET
	.END	START
`

// ftnCallerRun is what FTNCALL writes, and $STATUS after it.
const ftnCallerRun = "FTNRTL V1.0  \nversion ok\nEND OF RUN\n  $STATUS == \"%X00000001\"\n"

func TestFTNRTL(t *testing.T) {
	eachLoader(t, func(t *testing.T, paging bool) {
		d, c, buf := shareFindConsole(t, paging)
		s := c.ContainerSession

		for name, text := range map[string]string{"FTNCALL.MAR": ftnCaller, "FTNSTOP.MAR": ftnStopper} {
			if _, err := s.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name}, rms.TextRecords, splitSource(text)); err != nil {
				t.Fatal(err)
			}
		}

		// SYS$SHARE leads nowhere that holds an FTNRTL, so the locator
		// gets to the built-in one.
		shareFindRun(t, d, "DEFINE SYS$SHARE DUA2:[000000]")

		for _, name := range []string{"FTNCALL", "FTNSTOP"} {
			shareFindRun(t, d, "MACRO "+name)
			shareFindRun(t, d, "LINK "+name)
		}

		if out := buf.String(); out != "" {
			t.Errorf("MACRO and LINK said:\n%s", out)
		}

		if got := shareRun(t, d, buf, "FTNCALL"); got != ftnCallerRun {
			t.Errorf("FTNCALL:\n%s\nwant:\n%s", got, ftnCallerRun)
		}

		if got, want := shareRun(t, d, buf, "FTNSTOP"), "FORTRAN STOP\n  $STATUS == \"%X00000001\"\n"; got != want {
			t.Errorf("FTNSTOP:\n%s\nwant:\n%s", got, want)
		}

		// The program's list of shareable images names FTNRTL, and the
		// image it ran with was govax's own.
		shareFindRun(t, d, "RUN/NOEXECUTE FTNCALL")

		if spec := ftnrtlSpec(c); spec != rms.BuiltinSpec(ftnrtl.Name) {
			t.Errorf("FTNRTL came from %q", spec)
		}

		// FTNRTL built from its sources with govax's commands, as
		// FTNRTL.OPT says, and named by a logical name, is used instead.
		ftnrtlBuild(t, d, c)
		buf.Reset()
		shareFindRun(t, d, "DEFINE FTNRTL DUA1:[000000]FTNRTL.EXE")

		if got := shareRun(t, d, buf, "FTNCALL"); got != ftnCallerRun {
			t.Errorf("FTNCALL with the user's FTNRTL:\n%s\nwant:\n%s", got, ftnCallerRun)
		}

		shareFindRun(t, d, "RUN/NOEXECUTE FTNCALL")

		if spec := ftnrtlSpec(c); !strings.HasPrefix(spec, "DUA1:[000000]FTNRTL.EXE") {
			t.Errorf("with FTNRTL defined, it came from %q", spec)
		}

		// LINK too reads the user's: a link with the logical name is the
		// same program.
		shareFindRun(t, d, "LINK/EXECUTABLE=FTNCALL2 FTNCALL")

		if got := shareRun(t, d, buf, "FTNCALL2"); got != ftnCallerRun {
			t.Errorf("FTNCALL2:\n%s", got)
		}
	})
}

// ftnrtlSpec is where the loaded program's FTNRTL came from.
func ftnrtlSpec(c *Console) string {
	for _, icb := range c.ImageState(nil).ICBList {
		if icb.Name == ftnrtl.Name {
			return icb.Spec.Name
		}
	}

	return ""
}

// ftnrtlBuild copies FTNRTL's sources to DUA1:[000000] and builds the
// image there with MACRO and LINK/SHAREABLE, as FTNRTL.OPT's comment
// says; the image is the built-in one but for the link time, the
// linker's name, and the modules' creation times.
func ftnrtlBuild(t *testing.T, d *Dispatcher, c *Console) {
	t.Helper()

	s := c.ContainerSession

	copyIn := func(file, name string) {
		data, err := fs.ReadFile(ftnrtl.Files, file)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := s.CreateRecordFile(rms.FileLocation{Name: "DUA1:[000000]" + name}, rms.TextRecords, splitSource(string(data))); err != nil {
			t.Fatal(err)
		}
	}

	copyIn(ftnrtl.OptionsFile, "FTNRTL.OPT")

	sources, err := fs.ReadDir(ftnrtl.Files, ftnrtl.SourceDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range sources {
		name := strings.ToUpper(e.Name())
		copyIn(path.Join(ftnrtl.SourceDir, e.Name()), name)
		shareFindRun(t, d, "MACRO "+strings.TrimSuffix(name, ".MAR"))
	}

	if err := d.Dispatch("LINK/SHAREABLE=FTNRTL/NOSYSLIB FTNRTL/OPTIONS"); err != nil && !vmserrors.MessageInhibited(err) {
		t.Fatalf("LINK/SHAREABLE: %v", err)
	}

	built, _, err := s.ReadRawFile(rms.FileLocation{Name: "DUA1:[000000]FTNRTL.EXE"})
	if err != nil {
		t.Fatal(err)
	}

	builtin, _ := ftnrtl.Image(ftnrtl.Name)
	if len(built) != len(builtin) {
		t.Fatalf("the commands' FTNRTL is %d bytes, the built-in one %d", len(built), len(builtin))
	}

	differ := 0

	for i := range built {
		if built[i] != builtin[i] {
			differ++
		}
	}

	// The link time (8 bytes), the linker's name (up to 15), and each
	// module's creation time in the debug symbol table account for the
	// differences; anything more would mean the commands build another
	// image.
	if differ > 32 {
		t.Errorf("the commands' FTNRTL differs from the built-in one in %d bytes", differ)
	}

	if !bytes.Contains(built, []byte("FTN$STOP")) {
		t.Error("the commands' FTNRTL has no FTN$STOP")
	}
}
