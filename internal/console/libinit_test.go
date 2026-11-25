package console

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/testproc"
)

// libinitProgram lists three initialization routines for LIB$INITIALIZE, whose
// effect on COUNT says whether they ran and in which order, and returns
// COUNT+1 as its status: %X111 when FIRST and then SECOND ran, %X101 the other
// way, 1 when none did. The second, LINKED, calls its init-coroutine (the VMS
// RTL manual's 7.5), so SECOND and the main program run inside it, and adds
// %X1000 to the main program's status before it returns it: %X1111.
const libinitProgram = `	.TITLE	LIBINIT
	.PSECT	DATA,NOEXE,WRT,LONG
COUNT:	.LONG	0
	.PSECT	LIB$INITIALIZE, NOPIC,USR,CON,REL,GBL,NOSHR,NOEXE,RD,NOWRT,LONG
	.ADDRESS	FIRST, LINKED, SECOND
	.EXTERNAL	LIB$INITIALIZE
	.PSECT	CODE,EXE,NOWRT
	.ENTRY	FIRST,^M<>
	MOVL	#^X100,COUNT
	RET
	.ENTRY	LINKED,^M<>
	CMPL	(AP),#6
	BNEQ	10$
	CALLS	#0,@4(AP)
	ADDL2	#^X1000,R0
10$:	RET
	.ENTRY	SECOND,^M<>
	ADDL2	#^X10,COUNT
	RET
	.ENTRY	START,^M<>
	ADDL3	#1,COUNT,R0
	RET
	.END	START
`

// TestLibInitialize links a program with initialization routines against
// govax's LIB$INITIALIZE module and runs it (Phase 113, subtask 6): the routines
// run before the program, in the order listed, each with six arguments (VMS
// 7.1's count, testdata/err113), one calling back to run the rest; under
// traceback and without it; RUN/NOINIT leaves them out.
func TestLibInitialize(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	d, _, buf := dungeonConsole(t, false)

	src := filepath.Join(t.TempDir(), "libinit.mar")
	if err := os.WriteFile(src, []byte(libinitProgram), 0o644); err != nil {
		t.Fatal(err)
	}

	shareFindRun(t, d, `COPY/QUIET "`+src+`"/HOST LIBINIT.MAR`)
	shareFindRun(t, d, "MACRO LIBINIT")

	status := func(run string) string {
		t.Helper()

		buf.Reset()
		_ = d.Dispatch(run)
		shareFindRun(t, d, "SHOW SYMBOL $STATUS")

		return strings.TrimSpace(buf.String())
	}

	for _, tc := range []struct{ link, run, want string }{
		{"LINK LIBINIT", "RUN LIBINIT", `$STATUS == "%X00001111"`},
		{"LINK/NOTRACEBACK LIBINIT", "RUN LIBINIT", `$STATUS == "%X00001111"`},
		{"LINK LIBINIT", "RUN/NOINIT LIBINIT", `$STATUS == "%X00000001"`},
	} {
		shareFindRun(t, d, tc.link)

		if got := status(tc.run); got != tc.want {
			t.Errorf("%s, %s: %q, want %q", tc.link, tc.run, got, tc.want)
		}
	}
}

// libinitShareable is a shareable image with an initialization routine of its
// own, which sets COUNTER, a universal symbol.
const libinitShareable = `	.TITLE	SHRINIT
	.PSECT	SHRDATA, NOEXE,WRT,NOSHR,LONG
COUNTER::	.LONG	0
	.PSECT	LIB$INITIALIZE, NOPIC,USR,CON,REL,GBL,NOSHR,NOEXE,RD,NOWRT,LONG
	.ADDRESS	SETUP
	.EXTERNAL	LIB$INITIALIZE
	.PSECT	SHRCODE, EXE,NOWRT,SHR,PIC
	.ENTRY	SETUP,^M<>
	MOVL	#^X20,COUNTER
	RET
	.END
`

// libinitUser is a program that returns its shareable image's COUNTER plus 1.
const libinitUser = `	.TITLE	USEINIT
	.PSECT	CODE,EXE,NOWRT
	.ENTRY	START,^M<>
	ADDL3	#1,G^COUNTER,R0
	RET
	.END	START
`

// TestLibInitializeShareable runs a program using a shareable image whose own
// LIB$INITIALIZE psect lists a routine (Phase 113, subtask 6): LINK makes
// LIB$INITIALIZE the image's initialization list, IHD$V_INISHR and
// IHA$L_INISHR, as VMS 7.1's DECC$SHR has it (subtask 10), and govax calls it
// before the program (how VMS calls it is the probe round's question).
// RUN/NOINIT leaves it out.
func TestLibInitializeShareable(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	d, _, buf := dungeonConsole(t, false)
	dir := t.TempDir()

	for name, text := range map[string]string{"shrinit.mar": libinitShareable, "useinit.mar": libinitUser} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}

		shareFindRun(t, d, `COPY/QUIET "`+path+`"/HOST `+strings.ToUpper(name))
	}

	proc := writeProcedure(t, dir, "build.com",
		"$ MACRO SHRINIT",
		"$ MACRO USEINIT",
		"$ LINK/SHAREABLE=SHRINIT/NOMAP SHRINIT,SYS$INPUT/OPTIONS",
		"UNIVERSAL=COUNTER",
		"$ DEFINE SHRINIT SYS$DISK:[]SHRINIT.EXE",
		"$ LINK/NOMAP USEINIT,SYS$INPUT/OPTIONS",
		"SHRINIT/SHAREABLE",
		"$ SHOW SYMBOL $STATUS",
	)

	buf.Reset()

	if err := d.Dispatch("@" + proc); err != nil {
		t.Fatalf("@build: %v\n%s", err, buf.String())
	}

	if got := buf.String(); got != "  $STATUS == \"%X10000001\"\n" {
		t.Fatalf("building said %q", got)
	}

	buf.Reset()
	shareFindRun(t, d, "ANALYZE/IMAGE/OUTPUT=SHRINIT.ANI SHRINIT.EXE")
	shareFindRun(t, d, "TYPE SHRINIT.ANI")

	for _, want := range []string{"IHD$V_INISHR     1", "shareable image initialization list:"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("SHRINIT's header lacks %q:\n%s", want, buf.String())
		}
	}

	for run, want := range map[string]string{
		"RUN USEINIT":        `$STATUS == "%X00000021"`,
		"RUN/NOINIT USEINIT": `$STATUS == "%X00000001"`,
	} {
		buf.Reset()
		_ = d.Dispatch(run)
		shareFindRun(t, d, "SHOW SYMBOL $STATUS")

		if got := strings.TrimSpace(buf.String()); got != want {
			t.Errorf("%s: %q, want %q", run, got, want)
		}
	}
}

// libinitStray is a shareable image with a transfer address and no
// initialization list, as VMS 7.1's LIBRTL.EXE is: its .END names LIB$EMODD,
// which LINK put in the transfer address array. STRAY, the transfer address
// here, reads its first argument's longword, as LIB$EMODD does, and sets
// COUNTER.
const libinitStray = `	.TITLE	SHRSTRAY
	.PSECT	SHRDATA, NOEXE,WRT,NOSHR,LONG
COUNTER::	.LONG	0
	.PSECT	SHRCODE, EXE,NOWRT,SHR,PIC
	.ENTRY	STRAY,^M<R2>
	MOVL	@4(AP),R2
	MOVL	#^X40,COUNTER
	RET
	.END	STRAY
`

// TestShareableTransferNotCalled is Phase 113's subtask 10: a shareable
// image's transfer addresses are never called, only its initialization list.
// Before it govax called VMS's LIBRTL's, LIB$EMODD, with no arguments to read,
// and faulted at SHARE$LIBRTL+0CE1.
func TestShareableTransferNotCalled(t *testing.T) {
	if testproc.Isolate(t) {
		return
	}
	d, _, buf := dungeonConsole(t, false)
	dir := t.TempDir()

	for name, text := range map[string]string{"shrstray.mar": libinitStray, "useinit.mar": libinitUser} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}

		shareFindRun(t, d, `COPY/QUIET "`+path+`"/HOST `+strings.ToUpper(name))
	}

	proc := writeProcedure(t, dir, "build.com",
		"$ MACRO SHRSTRAY",
		"$ MACRO USEINIT",
		"$ LINK/SHAREABLE=SHRSTRAY/NOMAP/NOTRACEBACK SHRSTRAY,SYS$INPUT/OPTIONS",
		"UNIVERSAL=COUNTER",
		"$ DEFINE SHRSTRAY SYS$DISK:[]SHRSTRAY.EXE",
		"$ LINK/NOMAP USEINIT,SYS$INPUT/OPTIONS",
		"SHRSTRAY/SHAREABLE",
		"$ SHOW SYMBOL $STATUS",
		"$ ANALYZE/IMAGE/OUTPUT=SHRSTRAY.ANI SHRSTRAY.EXE",
	)

	buf.Reset()

	if err := d.Dispatch("@" + proc); err != nil {
		t.Fatalf("@build: %v\n%s", err, buf.String())
	}

	if got := buf.String(); !strings.HasPrefix(got, "  $STATUS == \"%X10000001\"\n") {
		t.Fatalf("building said %q", got)
	}

	buf.Reset()
	shareFindRun(t, d, "TYPE SHRSTRAY.ANI")

	for _, want := range []string{"IHD$V_INISHR     0", "first transfer address:  %X'00000"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("SHRSTRAY's header lacks %q:\n%s", want, buf.String())
		}
	}

	if strings.Contains(buf.String(), "first transfer address:  %X'00000000'") {
		t.Errorf("SHRSTRAY has no transfer address:\n%s", buf.String())
	}

	buf.Reset()
	_ = d.Dispatch("RUN USEINIT")
	shareFindRun(t, d, "SHOW SYMBOL $STATUS")

	if got, want := strings.TrimSpace(buf.String()), `$STATUS == "%X00000001"`; got != want {
		t.Errorf("RUN USEINIT: %q, want %q", got, want)
	}

	// RUN of the shareable image itself is still NOTFR.
	buf.Reset()

	err := d.Dispatch("RUN SHRSTRAY")
	if got := fmt.Sprint(err) + buf.String(); !strings.Contains(got, "no transfer address") {
		t.Errorf("RUN SHRSTRAY: %q, want NOTFR", got)
	}
}
