package console_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/vmserrors"
)

// Phase 57, subtask 9: the terminal's holder, DCL's ATTACH, and
// LIB$ATTACH (internal/corevms/attach.go, internal/dclshell/attachcmd.go).

// attachSource is an image that finds its owner's PID ($GETJPIW,
// JPI$_OWNER), writes "attaching", gives its owner the terminal with
// LIB$ATTACH, and, when the terminal is back, writes "back", returning
// LIB$ATTACH's status.
const attachSource = `	.title	attach
	$jpidef
	.psect	data,noexe,wrt
items:	.word	4,jpi$_owner
	.address owner
	.long	0
	.long	0
owner:	.long	0
amsg:	.ascid	/attaching/
bmsg:	.ascid	/back/
	.psect	code,exe,nowrt
	.entry	start,^m<r2>
	$getjpiw_s itmlst=items
	pushaq	amsg
	calls	#1,g^lib$put_output
	pushal	owner
	calls	#1,g^lib$attach
	movl	r0,r2
	pushaq	bmsg
	calls	#1,g^lib$put_output
	movl	r2,r0
	ret
	.end	start
`

// attachConsole is a scheduled console with typed as the terminal's
// input, and a dispatch that returns the output since the last as
// programLines joined by "|", a failure's message among them.
func attachConsole(t *testing.T, typed string) (*console.Console, func(string) string) {
	t.Helper()

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	if image := buildImage(t, c, "attach", attachSource); d.Dispatch("ATT :== $"+image) != nil {
		t.Fatal("can't define ATT")
	}

	c.In = strings.NewReader(typed)

	return c, func(line string) string {
		t.Helper()

		out.Reset()

		if err := d.Dispatch(line); err != nil && !vmserrors.MessageInhibited(err) {
			out.WriteString("%" + err.Error() + "\n")
		}

		return strings.Join(programLines(out.String()), "|")
	}
}

// returnedSub is DCL's message when the terminal is back with SYSTEM_1.
const returnedSub = "%DCL-S-RETURNED, control returned to process SYSTEM_1"

// TestAttach_backAndForth: a spawned subprocess ATTACHes to process 1,
// which ends process 1's SPAWN (SS$_NORMAL), the subprocess still there;
// process 1 ATTACHes back to it, by name and then by PID, and the
// subprocess's ATTACH ends; when it logs out, process 1's ATTACH ends.
// Ctrl/Y's setting is the attached process's throughout.
func TestAttach_backAndForth(t *testing.T) {
	c, dispatch := attachConsole(t, strings.Join([]string{
		"SET NOCONTROL=Y",
		"ATTACH SYSTEM",
		`WRITE SYS$OUTPUT "again"`,
		"ATTACH SYSTEM",
		"LOGOUT",
	}, "\n")+"\n")

	if got, want := dispatch("SPAWN"), spawnedOne+"$ $ "+returnedOne; got != want {
		t.Errorf("SPAWN:\n%q\nwant\n%q", got, want)
	}

	if n := len(c.RTL.Processes()); n != 2 || c.Status() != 1 || !c.ControlYEnabled() {
		t.Errorf("after the subprocess's ATTACH: %d processes, $STATUS %08X, Ctrl/Y %t; want 2, 1, enabled",
			n, c.Status(), c.ControlYEnabled())
	}

	if got, want := dispatch("ATTACH SYSTEM_1"), returnedSub+"|$ again|$ "+returnedOne; got != want {
		t.Errorf("ATTACH SYSTEM_1:\n%q\nwant\n%q", got, want)
	}

	sub, ok := c.RTL.FindProcessName(c.RTL.Process.UICGroup(), "SYSTEM_1")
	if !ok {
		t.Fatal("SYSTEM_1 has gone")
	}

	pid := fmt.Sprintf("%X", sub.Process.PID) // leading zeros omitted

	if got, want := dispatch("ATTACH/IDENTIFICATION="+pid), returnedSub+"|$   Process SYSTEM_1 logged out at"; !strings.HasPrefix(got, want) || !strings.HasSuffix(got, returnedOne) {
		t.Errorf("ATTACH/IDENTIFICATION=%s:\n%q\nwant\n%q...%q", pid, got, want, returnedOne)
	}

	if n := len(c.RTL.Processes()); n != 1 || c.Status() != 1 || !c.ControlYEnabled() {
		t.Errorf("at the end: %d processes, $STATUS %08X, Ctrl/Y %t; want 1, 1, enabled", n, c.Status(), c.ControlYEnabled())
	}
}

// TestAttach_refused: ATTACH to the process typing it, to a SPAWN/NOWAIT
// subprocess is LIB$_ATTREQREF; to a name or
// PID no process has, SS$_NONEXPR; a name and /IDENTIFICATION both, or
// neither, is the command's error. $STATUS is each status.
func TestAttach_refused(t *testing.T) {
	c, dispatch := attachConsole(t, "")

	if got := dispatch("SPAWN/NOWAIT/PROCESS=BESIDE WAIT 00:00:30"); got != "%DCL-S-SPAWNED, process BESIDE spawned" {
		t.Fatalf("SPAWN/NOWAIT: %q", got)
	}

	const (
		attReqRef = "%LIB-F-ATTREQREF, attach request refused"
		nonExpr   = "%SYSTEM-W-NONEXPR, nonexistent process"
	)

	for _, tc := range []struct {
		line, want string
		status     uint32
	}{
		{"ATTACH SYSTEM", attReqRef, 0x158394},
		{"ATTACH BESIDE", attReqRef, 0x158394},
		{"ATTACH NOSUCH", nonExpr, 0x8e8},
		{"ATTACH/IDENTIFICATION=7F0123", nonExpr, 0x8e8},
		{"ATTACH/IDENTIFICATION=XYZ", "%DCL-W-IVVALU", 0},
		{"ATTACH/IDENTIFICATION=301 SYSTEM", "%DCL-W-CONFLICT", 0},
		{"ATTACH", "%DCL-W-INSFPRM", 0},
	} {
		if got := dispatch(tc.line); !strings.HasPrefix(got, tc.want) {
			t.Errorf("%s: %q, want %q", tc.line, got, tc.want)
		}

		if tc.status != 0 && c.Status() != tc.status {
			t.Errorf("%s: $STATUS %08X, want %08X", tc.line, c.Status(), tc.status)
		}
	}
}

// TestLibAttach_image: an image in a spawned subprocess gives its owner,
// process 1, the terminal with LIB$ATTACH, which ends process 1's SPAWN;
// process 1's ATTACH gives it back, LIB$ATTACH returns SS$_NORMAL, and
// the image goes on.
func TestLibAttach_image(t *testing.T) {
	c, dispatch := attachConsole(t, "ATT\nLOGOUT\n")

	if got, want := dispatch("SPAWN"), spawnedOne+"$ attaching|"+returnedOne; got != want {
		t.Errorf("SPAWN:\n%q\nwant\n%q", got, want)
	}

	if got, want := dispatch("ATTACH SYSTEM_1"), "back|$ "; !strings.HasPrefix(got, want) || !strings.HasSuffix(got, returnedOne) {
		t.Errorf("ATTACH SYSTEM_1:\n%q\nwant %q...%q", got, want, returnedOne)
	}

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes at the end", n)
	}
}

// TestAttach_nested: a subprocess's subprocess ATTACHes to process 1,
// ending process 1's SPAWN; process 1 ATTACHes to the subprocess between
// them, whose own SPAWN then ends, the innermost still there; when the
// middle one logs out, its subprocess goes with it, and the terminal
// comes back to process 1.
func TestAttach_nested(t *testing.T) {
	c, dispatch := attachConsole(t, strings.Join([]string{
		"SPAWN/PROCESS=INNER",
		"ATTACH SYSTEM",
		"LOGOUT",
	}, "\n")+"\n")

	want := spawnedOne + "$ %DCL-S-SPAWNED, process INNER spawned|%DCL-S-ATTACHED, terminal now attached to process INNER|$ " + returnedOne
	if got := dispatch("SPAWN"); got != want {
		t.Errorf("SPAWN:\n%q\nwant\n%q", got, want)
	}

	if n := len(c.RTL.Processes()); n != 3 {
		t.Errorf("%d processes after INNER's ATTACH, want 3", n)
	}

	got := dispatch("ATTACH SYSTEM_1")
	if want := returnedSub + "|$ "; !strings.HasPrefix(got, want) || !strings.HasSuffix(got, returnedOne) {
		t.Errorf("ATTACH SYSTEM_1:\n%q\nwant %q...%q", got, want, returnedOne)
	}

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes at the end, want 1", n)
	}
}

// TestAttach_pause: ATTACH typed at LIB$PAUSE's prompt in process 1
// ends the paused image first (without its exit handler), then runs.
func TestAttach_pause(t *testing.T) {
	c, dispatch := attachConsole(t, "ATTACH SYSTEM\nATTACH SYSTEM_1\nLOGOUT\n")

	if image := buildImage(t, c, "pause", pauseSource); dispatch("PAUSE :== $"+image) != "" {
		t.Fatal("can't define PAUSE")
	}

	if got, want := dispatch("SPAWN"), spawnedOne+"$ "+returnedOne; got != want {
		t.Errorf("SPAWN:\n%q\nwant\n%q", got, want)
	}

	got := dispatch("PAUSE")
	if want := "before|" + c.Prompt() + returnedSub + "|$ "; !strings.HasPrefix(got, want) || !strings.HasSuffix(got, returnedOne) {
		t.Errorf("PAUSE:\n%q\nwant %q...%q", got, want, returnedOne)
	}

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes at the end, want 1", n)
	}
}
