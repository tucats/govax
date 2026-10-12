package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/vmserrors"
)

// exitHandlerSource declares a user-mode exit handler that writes
// "handler", then loops forever.
const exitHandlerSource = `	.title	exh
	.psect	data,noexe,wrt
said:	.ascid	/handler/
desblk:	.long	0
	.address handler
	.long	1
	.address status
status:	.blkl	1
	.psect	code,exe,nowrt
	.entry	handler,^m<>
	pushaq	said
	calls	#1,g^lib$put_output
	ret
	.entry	start,^m<>
	$dclexh_s desblk=desblk
forever: brb	forever
	.end	start
`

// TestStopImage: STOP/IMAGE makes a subprocess's image exit, its exit
// handler called, and its DCL goes on; STOP/EXIT=USER_MODE does the
// same and then deletes the process; STOP/NOEXIT deletes it at once,
// no handler called (Phase 51).
func TestStopImage(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	exh := buildImage(t, c, "exh", exitHandlerSource)
	child := buildChildImage(t, c)

	proc := filepath.Join(t.TempDir(), "spin.com")
	if err := os.WriteFile(proc, []byte(strings.Join([]string{
		"$ SET NOON",
		`$ RUN "` + exh + `"`,
		`$ WRITE SYS$OUTPUT "after ''$STATUS'"`,
		`$ RUN "` + exh + `"`,
		`$ WRITE SYS$OUTPUT "not reached"`,
	}, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	run := func() {
		t.Helper()

		if err := c.Run(child, machine.RunOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	if err := d.Dispatch(`SPAWN/NOWAIT/PROCESS=SPIN @"` + proc + `"`); err != nil {
		t.Fatal(err)
	}

	run()
	out.Reset()

	if err := d.Dispatch("STOP/IMAGE SPIN"); err != nil {
		t.Fatalf("STOP/IMAGE: %v", err)
	}

	run()

	if got := out.String(); !strings.Contains(got, "handler\n") || !strings.Contains(got, "after %X") {
		t.Errorf("after STOP/IMAGE:\n%s\nwant the handler's line and the procedure going on", got)
	}

	if n := len(c.RTL.Processes()); n != 2 {
		t.Fatalf("%d processes after STOP/IMAGE, want SPIN still there", n)
	}

	out.Reset()

	if err := d.Dispatch("STOP/EXIT=BOGUS SPIN"); !hasStatusCode(err, vmserrors.CLI_IVKEYW) {
		t.Errorf("STOP/EXIT=BOGUS: %v, want IVKEYW", err)
	}

	if err := d.Dispatch("STOP/EXIT=USER SPIN"); err != nil {
		t.Fatalf("STOP/EXIT=USER: %v", err)
	}

	run()

	if got := out.String(); !strings.Contains(got, "handler\n") || strings.Contains(got, "not reached") {
		t.Errorf("after STOP/EXIT=USER:\n%s\nwant the handler's line, and the procedure ended", got)
	}

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes after STOP/EXIT=USER, want only process 1", n)
	}

	// /NOEXIT: deleted at once, no handler.
	if err := d.Dispatch(`SPAWN/NOWAIT/PROCESS=SPIN2 RUN "` + exh + `"`); err != nil {
		t.Fatal(err)
	}

	run()
	out.Reset()

	if err := d.Dispatch("STOP/NOEXIT SPIN2"); err != nil {
		t.Fatalf("STOP/NOEXIT: %v", err)
	}

	if n := len(c.RTL.Processes()); n != 1 || strings.Contains(out.String(), "handler") {
		t.Errorf("after STOP/NOEXIT: %d processes, output %q", n, out.String())
	}
}

// hasStatusCode reports whether err is govax's status code.
func hasStatusCode(err error, code uint32) bool {
	return err != nil && strings.Contains(err.Error(), vmserrors.New(code).Error())
}
