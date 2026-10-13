package console_test

import (
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
