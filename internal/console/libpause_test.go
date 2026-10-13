package console_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/vmserrors"
)

// Phase 57, subtask 8: LIB$PAUSE reads DCL's Ctrl/Y level with the
// image paused (internal/dclshell/dclpause.go).

// pauseSource is an image that declares an exit handler (which writes
// "exit handler"), writes "before", calls LIB$PAUSE, and writes "after",
// returning LIB$PAUSE's status.
const pauseSource = `	.title	pause
	.psect	data,noexe,wrt
exh:	.long	0
	.address handler
	.long	1
	.address exsts
exsts:	.long	0
hmsg:	.ascid	/exit handler/
bmsg:	.ascid	/before/
amsg:	.ascid	/after/
	.psect	code,exe,nowrt
	.entry	start,^m<r2>
	pushal	exh
	calls	#1,g^sys$dclexh
	pushaq	bmsg
	calls	#1,g^lib$put_output
	calls	#0,g^lib$pause
	movl	r0,r2
	pushaq	amsg
	calls	#1,g^lib$put_output
	movl	r2,r0
	ret
	.entry	handler,^m<>
	pushaq	hmsg
	calls	#1,g^lib$put_output
	ret
	.end	start
`

// In the outputs wanted, "P> " stands for the console's prompt.

// pauseConsole is a scheduled console with PAUSE, a foreign command for
// pauseSource's image, and X defined, with typed as the terminal's
// input.
func pauseConsole(t *testing.T, typed string) (*console.Console, func() string, *console.Dispatcher) {
	t.Helper()

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	image := buildImage(t, c, "pause", pauseSource)

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	for _, line := range []string{"PAUSE :== $" + image, `X = "paused"`} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	c.In = strings.NewReader(typed)
	out.Reset()

	return c, func() string { return strings.Join(programLines(out.String()), "|") }, d
}

// TestLibPause_levelZero: at command level 0, DCL reads commands with
// the image paused: CONTINUE resumes it (LIB$PAUSE returns SS$_NORMAL);
// STOP and EXIT end it, without its exit handler, $STATUS SS$_CONTROLY,
// and govax goes on; a command that runs an image (a procedure, here)
// ends it and then runs.
func TestLibPause_levelZero(t *testing.T) {
	proc := writeCommands(t, `$ WRITE SYS$OUTPUT "procedure"`)

	for _, tc := range []struct {
		name, typed, want string
		status            uint32
	}{
		{"CONTINUE", "SHOW SYMBOL X\nCONTINUE\n", `before|P>   X = "paused"|P> after|exit handler`, 1},
		{"STOP", "STOP\n", "before|P> ", ssControlY},
		{"EXIT", "EXIT\n", "before|P> ", ssControlY},
		{"end of input", "", "before|P> ", ssControlY},
		{"procedure", `@"` + proc + "\"\n", "before|P> procedure", 0x10001}, // WRITE's status
	} {
		c, output, d := pauseConsole(t, tc.typed)

		if err := d.Dispatch("PAUSE"); err != nil {
			t.Errorf("%s: PAUSE: %v", tc.name, err)
		}

		if got, want := output(), strings.ReplaceAll(tc.want, "P> ", c.Prompt()); got != want {
			t.Errorf("%s: output\n%q\nwant\n%q", tc.name, got, want)
		}

		if c.Status() != tc.status {
			t.Errorf("%s: $STATUS %08X, want %08X", tc.name, c.Status(), tc.status)
		}

		if !c.Running() {
			t.Errorf("%s: govax is ending", tc.name)
		}
	}
}

// TestLibPause_procedure: in a procedure, the pause is the Ctrl/Y
// level's: CONTINUE resumes the image and the procedure goes on; STOP
// ends the image and the procedure.
func TestLibPause_procedure(t *testing.T) {
	for _, tc := range []struct {
		name, typed, want string
	}{
		{"CONTINUE", "WRITE SYS$OUTPUT X\nCONTINUE\n", "before|P> paused|P> after|exit handler|next"},
		{"STOP", "STOP\n", "before|P> "},
	} {
		c, output, d := pauseConsole(t, tc.typed)
		proc := writeCommands(t, `$ PAUSE`, `$ WRITE SYS$OUTPUT "next"`)

		if err := d.Dispatch(`@"` + proc + `"`); err != nil && !vmserrors.MessageInhibited(err) {
			t.Errorf("%s: @: %v", tc.name, err)
		}

		if got, want := output(), strings.ReplaceAll(tc.want, "P> ", c.Prompt()); got != want {
			t.Errorf("%s: output\n%q\nwant\n%q", tc.name, got, want)
		}

		if c.CommandLevel() != 0 {
			t.Errorf("%s: command level %d after the procedure", tc.name, c.CommandLevel())
		}
	}
}

// TestLibPause_subprocess: a spawned subprocess's image pauses beneath
// its own DCL, which reads the terminal with its prompt; CONTINUE lets
// the image finish, and STOP ends it, SPAWN's status SS$_CONTROLY.
func TestLibPause_subprocess(t *testing.T) {
	for _, tc := range []struct {
		name, typed, want string
	}{
		{"CONTINUE", "SHOW SYMBOL X\nCONTINUE\n", `before|$   X = "paused"|$ after|exit handler|`},
		{"STOP", "STOP\n", "before|$ "},
	} {
		c, output, d := pauseConsole(t, tc.typed)

		if err := d.Dispatch("SPAWN PAUSE"); err != nil && !vmserrors.MessageInhibited(err) {
			t.Errorf("%s: SPAWN: %v", tc.name, err)
		}

		if want := spawnedOne + tc.want + returnedOne; output() != want {
			t.Errorf("%s: output\n%q\nwant\n%q", tc.name, output(), want)
		}

		if n := len(c.RTL.Processes()); n != 1 {
			t.Errorf("%s: %d processes after SPAWN", tc.name, n)
		}
	}
}

// TestLibPause_batch: in a LOGINOUT job reading a command file, which
// isn't interactive, LIB$PAUSE does nothing and returns SS$_NORMAL.
func TestLibPause_batch(t *testing.T) {
	c, output, d := pauseConsole(t, "")
	pause, _ := c.DCLSymbol("PAUSE")

	if err := d.Dispatch("SHOW SYMBOL X"); err != nil {
		t.Fatal(err)
	}

	loginout(t, c, writeCommands(t, `$ PAUSE :== `+pause, `$ PAUSE`))

	got, _ := splitLogoutReport(strings.Split(output(), "|"))
	want := []string{`  X = "paused"`, `$ PAUSE :== ` + pause, `$ PAUSE`, "before", "after", "exit handler"}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
