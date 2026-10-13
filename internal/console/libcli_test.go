package console_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
)

// Phase 57: the LIB$ CLI routines reach the DCL of the process whose
// image calls them (internal/dclshell/libcli.go).

// symbolsSource is an image that reads GREETING with LIB$GET_SYMBOL
// (the name in lowercase, with a trailing blank) and writes its value,
// and "global" if it was a global symbol; then defines FROM_IMAGE (local)
// and G*LOBAL (global) with LIB$SET_SYMBOL, and deletes the local symbol
// DOOMED with LIB$DELETE_SYMBOL. It returns the first failure's status,
// or SS$_NORMAL.
const symbolsSource = `	.title	symbols
	.psect	data,noexe,wrt
greet:	.ascid	/greeting /
buf:	.blkb	80
desc:	.word	80
	.byte	14,1
	.address buf
len:	.word	0
table:	.long	0
out:	.word	0
	.byte	14,1
	.address buf
gmsg:	.ascid	/global/
fromim:	.ascid	/from_image/
fromval: .ascid	/set by the image/
glob:	.ascid	/G*LOBAL/
globval: .ascid	/global one/
two:	.long	2
doomed:	.ascid	/DOOMED/
	.psect	code,exe,nowrt
	.entry	start,^m<>
	pushal	table
	pushaw	len
	pushaq	desc
	pushaq	greet
	calls	#4,g^lib$get_symbol
	blbc	r0,90$
	movw	len,out
	pushaq	out
	calls	#1,g^lib$put_output
	cmpl	table,#2
	bneq	10$
	pushaq	gmsg
	calls	#1,g^lib$put_output
10$:	pushaq	fromval
	pushaq	fromim
	calls	#2,g^lib$set_symbol
	blbc	r0,90$
	pushal	two
	pushaq	globval
	pushaq	glob
	calls	#3,g^lib$set_symbol
	blbc	r0,90$
	pushaq	doomed
	calls	#1,g^lib$delete_symbol
90$:	ret
	.end	start
`

// TestLibCLI_symbols: process 1's image reads a global symbol of the
// console's DCL, defines a local and a global one, and deletes a local
// one; DCL sees each change once the image has ended.
func TestLibCLI_symbols(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	image := buildImage(t, c, "symbols", symbolsSource)

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	for _, line := range []string{`GREETING == "hello there"`, `DOOMED = 7`} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	out.Reset()

	for _, line := range []string{`RUN "` + image + `"`, "SHOW SYMBOL FROM_IMAGE", "SHOW SYMBOL GL"} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	if err := d.Dispatch("SHOW SYMBOL DOOMED"); err == nil || !strings.Contains(err.Error(), "UNDSYM") {
		t.Errorf("SHOW SYMBOL DOOMED: %v, want UNDSYM: the image deleted it", err)
	}

	want := []string{
		"hello there",
		"global",
		`  FROM_IMAGE = "set by the image"`,
		`  G*LOBAL == "global one"`,
	}

	if got := programLines(out.String()); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestLibCLI_subprocessSymbols: a subprocess's image works with its own
// DCL's symbols, not process 1's: GREETING is the subprocess's local
// symbol (table-type-indicator 1), and FROM_IMAGE is defined there.
func TestLibCLI_subprocessSymbols(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	image := buildImage(t, c, "symbols", symbolsSource)

	commands := writeCommands(t,
		`$ GREETING = "from the subprocess"`,
		`$ DOOMED = 1`,
		`$ RUN "`+image+`"`,
		`$ SHOW SYMBOL FROM_IMAGE`,
	)

	out.Reset()

	loginout(t, c, commands)

	got, _ := splitLogoutReport(programLines(out.String()))
	want := []string{
		`$ GREETING = "from the subprocess"`,
		`$ DOOMED = 1`,
		`$ RUN "` + image + `"`,
		"from the subprocess",
		`$ SHOW SYMBOL FROM_IMAGE`,
		`  FROM_IMAGE = "set by the image"`,
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	if _, ok := c.DCLSymbol("FROM_IMAGE"); ok {
		t.Error("the subprocess's image defined FROM_IMAGE in process 1's DCL")
	}
}

// logicalsSource is an image that defines FROM_IMAGE with
// LIB$SET_LOGICAL and deletes DOOMED with LIB$DELETE_LOGICAL, returning
// the first failure's status, or SS$_NORMAL.
const logicalsSource = `	.title	logicals
	.psect	data,noexe,wrt
name:	.ascid	/FROM_IMAGE/
value:	.ascid	/set by the image/
doomed:	.ascid	/DOOMED/
	.psect	code,exe,nowrt
	.entry	start,^m<>
	pushaq	value
	pushaq	name
	calls	#2,g^lib$set_logical
	blbc	r0,90$
	pushaq	doomed
	calls	#1,g^lib$delete_logical
90$:	ret
	.end	start
`

// TestLibCLI_logicals: the names an image defines and deletes with
// LIB$SET_LOGICAL and LIB$DELETE_LOGICAL are supervisor-mode names in
// LNM$PROCESS, so the changes outlast the image.
func TestLibCLI_logicals(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	image := buildImage(t, c, "logicals", logicalsSource)

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	if err := d.Dispatch("DEFINE DOOMED X"); err != nil {
		t.Fatal(err)
	}

	out.Reset()

	for _, line := range []string{`RUN "` + image + `"`, "SHOW LOGICAL/FULL FROM_IMAGE"} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	_ = d.Dispatch("SHOW LOGICAL DOOMED") // NOTRAN, a warning

	want := []string{
		`   "FROM_IMAGE" [super] = "set by the image" (LNM$PROCESS_TABLE)`,
		`%SHOW-S-NOTRAN, no translation for logical name DOOMED`,
	}

	if got := programLines(out.String()); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// commandSource is an image that reads a line with LIB$GET_COMMAND,
// prompting "Cmd: ", then one with LIB$GET_INPUT, and writes each,
// returning the first failure's status, or SS$_NORMAL.
const commandSource = `	.title	command
	.psect	data,noexe,wrt
prompt:	.ascid	/Cmd: /
buf:	.blkb	80
desc:	.word	80
	.byte	14,1
	.address buf
len:	.word	0
out:	.word	0
	.byte	14,1
	.address buf
	.psect	code,exe,nowrt
	.entry	start,^m<>
	pushaw	len
	pushaq	prompt
	pushaq	desc
	calls	#3,g^lib$get_command
	blbc	r0,90$
	movw	len,out
	pushaq	out
	calls	#1,g^lib$put_output
	pushaw	len
	clrl	-(sp)
	pushaq	desc
	calls	#3,g^lib$get_input
	blbc	r0,90$
	movw	len,out
	pushaq	out
	calls	#1,g^lib$put_output
90$:	ret
	.end	start
`

// TestLibCLI_getCommand: in a command procedure, LIB$GET_COMMAND reads
// the terminal (SYS$COMMAND), while LIB$GET_INPUT reads the procedure's
// data line (SYS$INPUT).
func TestLibCLI_getCommand(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	image := buildImage(t, c, "command", commandSource)
	proc := writeCommands(t,
		`$ RUN "`+image+`"`,
		`from the procedure`,
	)

	c.In = strings.NewReader("from the terminal\r\n")
	out.Reset()

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	if err := d.Dispatch(`@"` + proc + `"`); err != nil {
		t.Fatalf("@: %v", err)
	}

	want := []string{"Cmd: from the terminal", "from the procedure"}
	if got := programLines(out.String()); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// putCommonSource is an image that leaves text in the common area with
// LIB$PUT_COMMON, returning its status.
const putCommonSource = `	.title	putcommon
	.psect	data,noexe,wrt
text:	.ascid	/left by an earlier image/
	.psect	code,exe,nowrt
	.entry	start,^m<>
	pushaq	text
	calls	#1,g^lib$put_common
	ret
	.end	start
`

// getCommonSource is an image that reads the common area with
// LIB$GET_COMMON and writes "common: " and the text, returning the
// first failure's status, or SS$_NORMAL.
const getCommonSource = `	.title	getcommon
	.psect	data,noexe,wrt
buf:	.ascii	/common: /
text:	.blkb	252
desc:	.word	252
	.byte	14,1
	.address text
len:	.word	0
out:	.word	0
	.byte	14,1
	.address buf
	.psect	code,exe,nowrt
	.entry	start,^m<>
	pushaw	len
	pushaq	desc
	calls	#2,g^lib$get_common
	blbc	r0,90$
	addw3	#8,len,out
	pushaq	out
	calls	#1,g^lib$put_output
90$:	ret
	.end	start
`

// TestLibCLI_common: the text one image leaves with LIB$PUT_COMMON is
// there for a later image of the same process, DCL commands between; a
// subprocess's common area is its own, empty when it starts.
func TestLibCLI_common(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	put := buildImage(t, c, "putcommon", putCommonSource)
	get := buildImage(t, c, "getcommon", getCommonSource)

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	out.Reset()

	for _, line := range []string{`RUN "` + get + `"`, `RUN "` + put + `"`, "X = 1", `RUN "` + get + `"`} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	want := []string{"common: ", "common: left by an earlier image"}
	if got := programLines(out.String()); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("process 1's output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	commands := writeCommands(t,
		`$ RUN "`+get+`"`,
		`$ RUN "`+put+`"`,
		`$ RUN "`+get+`"`,
	)

	out.Reset()

	loginout(t, c, commands)

	got, _ := splitLogoutReport(programLines(out.String()))
	want = []string{
		`$ RUN "` + get + `"`,
		"common: ",
		`$ RUN "` + put + `"`,
		`$ RUN "` + get + `"`,
		"common: left by an earlier image",
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("the subprocess's output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// chainSource is an image that declares an exit handler, which writes
// "exit handler", and then passes its command text (LIB$GET_FOREIGN) to
// the chaining routine named by the %s: LIB$DO_COMMAND or
// LIB$RUN_PROGRAM. It returns the first failure's status.
const chainSource = `	.title	chain
	.psect	data,noexe,wrt
buf:	.blkb	255
desc:	.word	255
	.byte	14,1
	.address buf
len:	.word	0
text:	.word	0
	.byte	14,1
	.address buf
exh:	.long	0
	.address handler
	.long	1
	.address exsts
exsts:	.long	0
hmsg:	.ascid	/exit handler/
	.psect	code,exe,nowrt
	.entry	start,^m<>
	pushal	exh
	calls	#1,g^sys$dclexh
	blbc	r0,90$
	pushaw	len
	clrl	-(sp)
	pushaq	desc
	calls	#3,g^lib$get_foreign
	blbc	r0,90$
	movw	len,text
	pushaq	text
	calls	#1,g^%s
90$:	ret
	.entry	handler,^m<>
	pushaq	hmsg
	calls	#1,g^lib$put_output
	ret
	.end	start
`

// chainConsole is a scheduled console with DOCMD and RUNPROG, foreign
// commands for chainSource's two images, defined.
func chainConsole(t *testing.T) (*console.Console, *bytes.Buffer, *console.Dispatcher) {
	t.Helper()

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	docmd := buildImage(t, c, "docmd", fmt.Sprintf(chainSource, "lib$do_command"))
	runprog := buildImage(t, c, "runprog", fmt.Sprintf(chainSource, "lib$run_program"))

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	for _, line := range []string{"DOCMD :== $" + docmd, "RUNPROG :== $" + runprog, `CHAINED == "yes"`} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	out.Reset()

	return c, out, d
}

// TestLibCLI_doCommand: LIB$DO_COMMAND's command runs once the image's
// exit handler has, its status $STATUS; a chained image may chain again.
func TestLibCLI_doCommand(t *testing.T) {
	c, out, d := chainConsole(t)

	for _, line := range []string{"DOCMD SHOW SYMBOL CHAINED", "DOCMD DOCMD SHOW SYMBOL CHAINED"} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	want := []string{
		"exit handler",
		`  CHAINED == "yes"`,
		"exit handler",
		"exit handler",
		`  CHAINED == "yes"`,
	}

	if got := programLines(out.String()); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	if err := d.Dispatch("DOCMD SHOW SYMBOL NOSUCH"); err == nil || !strings.Contains(err.Error(), "UNDSYM") {
		t.Errorf("a chained command that fails: %v, want UNDSYM", err)
	}

	if status, _ := c.DCLSymbol("$STATUS"); status != `%X00038140` {
		t.Errorf("$STATUS = %s, want the chained command's, %%X00038140", status)
	}
}

// TestLibCLI_doCommandLevel: in a procedure, the chained command runs at
// the procedure's level: the local symbol it defines is gone once the
// procedure ends.
func TestLibCLI_doCommandLevel(t *testing.T) {
	c, out, d := chainConsole(t)

	proc := writeCommands(t,
		`$ DOCMD X = 5`,
		`$ SHOW SYMBOL/LOCAL X`,
	)

	if err := d.Dispatch(`@"` + proc + `"`); err != nil {
		t.Fatalf("@: %v", err)
	}

	want := []string{"exit handler", "  X = 5   Hex = 00000005  Octal = 00000000005"}
	if got := programLines(out.String()); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	if _, ok := c.DCLSymbol("X"); ok {
		t.Error("X outlived the procedure whose image chained its definition")
	}
}

// TestLibCLI_runProgram: LIB$RUN_PROGRAM's image runs next, the common
// area carrying text to it, and with no command text of its own; one
// that isn't there fails as RUN of it would.
func TestLibCLI_runProgram(t *testing.T) {
	c, out, d := chainConsole(t)
	put := buildImage(t, c, "putcommon", putCommonSource)
	get := buildImage(t, c, "getcommon", getCommonSource)

	for _, line := range []string{`RUN "` + put + `"`, `RUNPROG "` + get + `"`} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	want := []string{"exit handler", "common: left by an earlier image"}
	if got := programLines(out.String()); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	if err := d.Dispatch(`RUNPROG "` + filepath.Join(t.TempDir(), "missing.exe") + `"`); err == nil {
		t.Error("RUN_PROGRAM of a missing image: no error")
	}
}

// TestLibCLI_subprocessChain: a subprocess's image chains a command to
// its own DCL.
func TestLibCLI_subprocessChain(t *testing.T) {
	c, out, _ := chainConsole(t)
	docmd, _ := c.DCLSymbol("DOCMD")

	commands := writeCommands(t,
		`$ DOCMD :== `+docmd,
		`$ X = "in the subprocess"`,
		`$ DOCMD SHOW SYMBOL X`,
	)

	out.Reset()

	loginout(t, c, commands)

	got, _ := splitLogoutReport(programLines(out.String()))
	want := []string{
		`$ DOCMD :== ` + docmd,
		`$ X = "in the subprocess"`,
		`$ DOCMD SHOW SYMBOL X`,
		"exit handler",
		`  X = "in the subprocess"`,
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// fixedChainSource is an image that chains SHOW SYMBOL CHAINED with
// LIB$DO_COMMAND.
const fixedChainSource = `	.title	fixedchain
	.psect	data,noexe,wrt
text:	.ascid	/SHOW SYMBOL CHAINED/
	.psect	code,exe,nowrt
	.entry	start,^m<>
	pushaq	text
	calls	#1,g^lib$do_command
	ret
	.end	start
`

// TestLibCLI_chainUnderDebugger: an image run under the debugger chains
// a command, which DCL runs once the debugger's EXIT ends the session
// (the image's exit leaves the session open, as VMS's debugger does).
func TestLibCLI_chainUnderDebugger(t *testing.T) {
	c, out, d := chainConsole(t)
	consoletest.InstallDebugger(t, c, d)
	image := buildImage(t, c, "fixedchain", fixedChainSource)

	if err := d.Dispatch(`RUN/DEBUG "` + image + `"`); err != nil {
		t.Fatalf("RUN/DEBUG: %v", err)
	}

	if !c.InDebugger() {
		t.Fatal("no debugger session after RUN/DEBUG")
	}

	if err := d.Dispatch("GO"); err != nil {
		t.Fatalf("GO: %v", err)
	}

	out.Reset()

	if err := d.Dispatch("EXIT"); err != nil {
		t.Fatalf("EXIT: %v", err)
	}

	if c.InDebugger() {
		t.Fatal("the session outlived the debugger's EXIT")
	}

	if got := programLines(out.String()); strings.Join(got, "\n") != `  CHAINED == "yes"` {
		t.Errorf("output after EXIT:\n%s", out.String())
	}
}

// controlSource is an image that calls the control-key routine named by
// its format argument (LIB$DISABLE_CTRL or LIB$ENABLE_CTRL) for Ctrl/Y,
// and writes "old" and the keys old-mask says were enabled before.
const controlSource = `	.title	control
	.psect	data,noexe,wrt
mask:	.long	^x02000000
old:	.long	0
none:	.ascid	/old/
ymsg:	.ascid	/old Y/
tmsg:	.ascid	/old T/
	.psect	code,exe,nowrt
	.entry	start,^m<>
	pushal	old
	pushal	mask
	calls	#2,g^%s
	blbc	r0,90$
	pushaq	none
	bbc	#25,old,10$
	movaq	ymsg,(sp)
10$:	calls	#1,g^lib$put_output
	bbc	#20,old,20$
	pushaq	tmsg
	calls	#1,g^lib$put_output
20$:	movl	#1,r0
90$:	ret
	.end	start
`

// controlConsole is a scheduled console with NOCTRLY and CTRLY, foreign
// commands for controlSource's two images, defined; it returns their
// files too.
func controlConsole(t *testing.T) (*console.Console, *bytes.Buffer, *console.Dispatcher, string, string) {
	t.Helper()

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	disable := buildImage(t, c, "noctrly", fmt.Sprintf(controlSource, "lib$disable_ctrl"))
	enable := buildImage(t, c, "ctrly", fmt.Sprintf(controlSource, "lib$enable_ctrl"))

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	for _, line := range []string{"NOCTRLY :== $" + disable, "CTRLY :== $" + enable} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	out.Reset()

	return c, out, d, disable, enable
}

// TestLibCLI_controlKeys: LIB$DISABLE_CTRL's setting is the DCL's, so
// F$ENVIRONMENT shows it after the image and Ctrl/Y is off at the
// terminal; old-mask names the keys enabled before; LIB$ENABLE_CTRL
// turns Ctrl/Y back on. Disabling Ctrl/Y leaves the procedure's ON
// CONTROL_Y, unlike SET NOCONTROL=Y (VMS V7.1, testdata/lib57).
func TestLibCLI_controlKeys(t *testing.T) {
	c, out, d, _, _ := controlConsole(t)

	for _, line := range []string{
		"SET CONTROL=T",
		"NOCTRLY",
		`WRITE SYS$OUTPUT F$ENVIRONMENT("NOCONTROL")`,
	} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	if c.ControlYEnabled() {
		t.Error("Ctrl/Y is enabled after LIB$DISABLE_CTRL")
	}

	if err := d.Dispatch("CTRLY"); err != nil {
		t.Fatalf("CTRLY: %v", err)
	}

	if !c.ControlYEnabled() {
		t.Error("Ctrl/Y is disabled after LIB$ENABLE_CTRL")
	}

	proc := writeCommands(t,
		`$ ON CONTROL_Y THEN EXIT`,
		`$ NOCTRLY`,
		`$ WRITE SYS$OUTPUT F$ENVIRONMENT("ON_CONTROL_Y")`,
		`$ SET CONTROL=Y`,
	)

	if err := d.Dispatch(`@"` + proc + `"`); err != nil {
		t.Fatalf("@: %v", err)
	}

	want := []string{"old Y", "old T", "Y", "old", "old T", "old Y", "old T", "TRUE"}
	if got := programLines(out.String()); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestLibCLI_subprocessControlKeys: a subprocess's image changes its own
// DCL's setting, not process 1's.
func TestLibCLI_subprocessControlKeys(t *testing.T) {
	c, out, _, disable, _ := controlConsole(t)

	commands := writeCommands(t,
		`$ RUN "`+disable+`"`,
		`$ WRITE SYS$OUTPUT F$ENVIRONMENT("NOCONTROL")`,
	)

	loginout(t, c, commands)

	got, _ := splitLogoutReport(programLines(out.String()))
	want := []string{
		`$ RUN "` + disable + `"`,
		"old Y",
		`$ WRITE SYS$OUTPUT F$ENVIRONMENT("NOCONTROL")`,
		"T,Y",
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	if !c.ControlYEnabled() {
		t.Error("process 1's Ctrl/Y is disabled by a subprocess's LIB$DISABLE_CTRL")
	}

	if got, _, err := c.EvaluateExpression(`F$ENVIRONMENT("CONTROL")`); err != nil || got != "Y" {
		t.Errorf(`process 1's F$ENVIRONMENT("CONTROL") = %q (%v), want "Y"`, got, err)
	}
}
