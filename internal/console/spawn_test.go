package console_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 48, subtask 2: LIB$SPAWN (internal/librtl's libSpawn,
// corevms.Spawn). A subprocess runs the subprocess CLI (subcli.go) with
// its parent's symbols and logical names, and tells its parent when it
// ends.

// spawnWaitSource spawns "ECHO from the child" and waits for it, then
// reports what it found: the child's final status in
// completion-status-address.
const spawnWaitSource = `	.title	spawnwait
	.psect	data,noexe,wrt
cmd:	.ascid	/ECHO from the child/
done:	.ascid	/Parent: the child has ended/
okmsg:	.ascid	/Parent: its status is SS$_NORMAL/
badmsg:	.ascid	/Parent: wrong status/
status:	.long	0
	.psect	code,exe,nowrt
	.entry	start,^m<>
	pushal	status
	clrl	-(sp)
	clrl	-(sp)
	clrl	-(sp)
	clrl	-(sp)
	clrl	-(sp)
	pushaq	cmd
	calls	#7,g^lib$spawn
	blbc	r0,90$
	pushaq	done
	calls	#1,g^lib$put_output
	cmpl	status,#1
	bneq	80$
	pushaq	okmsg
	calls	#1,g^lib$put_output
	movl	#1,r0
	ret
80$:	pushaq	badmsg
	calls	#1,g^lib$put_output
90$:	ret
	.end	start
`

// spawnNoWaitSource spawns "ECHO from the child" with CLI$M_NOWAIT
// (flags 1), event flag 5, and an AST, says it goes on at once, then
// waits for the flag ($WAITFR).
const spawnNoWaitSource = `	.title	spawnnowait
	.psect	data,noexe,wrt
cmd:	.ascid	/ECHO from the child/
going:	.ascid	/Parent: going on/
astmsg:	.ascid	/Parent: AST/
flagmsg: .ascid	/Parent: event flag set/
flags:	.long	1
efn:	.byte	5
	.psect	code,exe,nowrt
	.entry	start,^m<>
	pushl	#0
	pushal	ast
	pushab	efn
	clrl	-(sp)
	clrl	-(sp)
	clrl	-(sp)
	pushal	flags
	clrl	-(sp)
	clrl	-(sp)
	pushaq	cmd
	calls	#10,g^lib$spawn
	blbc	r0,90$
	pushaq	going
	calls	#1,g^lib$put_output
	pushl	#5
	calls	#1,g^sys$waitfr
	pushaq	flagmsg
	calls	#1,g^lib$put_output
	movl	#1,r0
90$:	ret
	.entry	ast,^m<>
	pushaq	astmsg
	calls	#1,g^lib$put_output
	ret
	.end	start
`

// runParent builds source as name, runs it as process 1's image, and
// returns its output lines and those of the processes it started.
func runParent(t *testing.T, name, source string) []string {
	t.Helper()

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	echo := buildImage(t, c, "echo", echoSource)

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	if err := d.Dispatch(`ECHO :== "$` + echo + `"`); err != nil {
		t.Fatal(err)
	}

	parent := buildImage(t, c, name, source)

	out.Reset()

	if err := c.Run(parent, console.RunOptions{}); err != nil {
		t.Fatalf("RUN: %v\n%s", err, out.String())
	}

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes left, want only process 1", n)
	}

	return programLines(out.String())
}

// TestSpawn_wait: LIB$SPAWN with a command and no flags runs the command
// in a subprocess, which has the parent's ECHO symbol, and returns once
// the subprocess has ended, its final status (the command's) in
// completion-status-address.
func TestSpawn_wait(t *testing.T) {
	got := runParent(t, "spawnwait", spawnWaitSource)

	want := []string{"FROM THE CHILD", "Parent: the child has ended", "Parent: its status is SS$_NORMAL"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestSpawn_noWait: with CLI$M_NOWAIT, LIB$SPAWN doesn't wait; when the
// subprocess ends, the AST runs and the event flag is set. The
// subprocess, at its parent's priority, runs as soon as it's made: a
// process becoming computable at the current one's priority preempts it
// (internal/sched, the book's rule), so its line comes first.
func TestSpawn_noWait(t *testing.T) {
	got := runParent(t, "spawnnowait", spawnNoWaitSource)

	want := []string{"FROM THE CHILD", "Parent: AST", "Parent: going on", "Parent: event flag set"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestSpawn_rules: corevms.Spawn's checks and what the subprocess gets:
// the default name (the user name and "_n"), the parent's base priority,
// SYS$INPUT/SYS$OUTPUT, and process logical names (user and supervisor
// mode, not CONFINE, not executive mode); CLI$M_NOLOGNAM; LIB$_INVARG
// for an undefined flag; LIB$_NOCLI from a process with no CLI.
func TestSpawn_rules(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	one := c.RTL

	define := func(name string, mode lnm.Mode, attrs uint32) {
		t.Helper()

		if _, err := one.Logicals.Define(lnm.ProcessTableName, name, mode, attrs, []lnm.Equivalence{{Value: "X"}}); err != nil {
			t.Fatal(err)
		}
	}

	define("USERNAME_ONE", lnm.User, 0)
	define("SUPER_ONE", lnm.Supervisor, 0)
	define("EXEC_ONE", lnm.Executive, 0)
	define("CONFINED_ONE", lnm.User, lnm.AttrConfine)

	first, st := one.Spawn(corevms.SpawnRequest{Flags: 1, Output: "NL:"})
	if st != 1 {
		t.Fatalf("Spawn: status %08X", st)
	}

	second, st := one.Spawn(corevms.SpawnRequest{Flags: 1 | 4, Name: "OTHER"})
	if st != 1 {
		t.Fatalf("Spawn: status %08X", st)
	}

	if first.Process.Name != "SYSTEM_1" || second.Process.Name != "OTHER" {
		t.Errorf("names %q and %q, want SYSTEM_1 and OTHER", first.Process.Name, second.Process.Name)
	}

	if third, _ := one.Spawn(corevms.SpawnRequest{Flags: 1}); third == nil || third.Process.Name != "SYSTEM_2" {
		t.Errorf("a third subprocess's default name: want SYSTEM_2")
	}

	if first.Process.BasePriority != one.Process.BasePriority || first.Process.Owner != one.Process.PID {
		t.Errorf("base priority %d, owner %08X; want process 1's %d and its PID",
			first.Process.BasePriority, first.Process.Owner, one.Process.BasePriority)
	}

	if s := first.Startup; s == nil || s.CLI == nil || s.Output != "NL:" || s.Error != "NL:" || s.Input != "_TTA0:" {
		t.Errorf("startup %+v; want the CLI, output and error NL:, and process 1's input, _TTA0:", s)
	}

	has := func(env *corevms.Environment, name string) bool {
		_, err := env.Logicals.Translate(lnm.ProcessTableName, name, lnm.User, 0)

		return err == nil
	}

	for name, want := range map[string]bool{"USERNAME_ONE": true, "SUPER_ONE": true, "EXEC_ONE": false, "CONFINED_ONE": false} {
		if has(first, name) != want {
			t.Errorf("the subprocess has %s: %v, want %v", name, !want, want)
		}
	}

	if has(second, "USERNAME_ONE") {
		t.Error("CLI$M_NOLOGNAM: the subprocess has the parent's process logical names")
	}

	if _, st := one.Spawn(corevms.SpawnRequest{Flags: 0x200}); st != vmsdef.LibrarySymbols["LIB$_INVARG"] {
		t.Errorf("an undefined flag: status %08X, want LIB$_INVARG", st)
	}

	// A process $CREPRC made to run an image has no CLI.
	child, st := one.CreateProcess(corevms.CreateRequest{Image: "CHILD.EXE"})
	if st != 1 {
		t.Fatalf("CreateProcess: status %08X", st)
	}

	if _, st := child.Spawn(corevms.SpawnRequest{}); st != vmsdef.LibrarySymbols["LIB$_NOCLI"] {
		t.Errorf("from a process with no CLI: status %08X, want LIB$_NOCLI", st)
	}
}

// TestSpawn_schedulerOff: with vax.process.scheduler off, LIB$SPAWN is
// unsupported, as $CREPRC is.
func TestSpawn_schedulerOff(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	c.RTL.ProcessSettings.Scheduler = false

	if _, st := c.RTL.Spawn(corevms.SpawnRequest{}); st != vmsdef.Symbols["SS$_UNSUPPORTED"] {
		t.Errorf("status %08X, want SS$_UNSUPPORTED", st)
	}
}

// TestSpawnCommand: the console's SPAWN runs a command in a subprocess
// (here a foreign command the console defined) and waits for it, then
// says control has come back to process 1 (SYSTEM); SPAWN/NOWAIT makes
// one that runs only when the machine next runs, and says so; SPAWN/INPUT
// reads a file of commands.
func TestSpawnCommand(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	echo := buildImage(t, c, "echo", echoSource)
	child := buildChildImage(t, c)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	dispatch := func(line string) string {
		t.Helper()

		out.Reset()

		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v\n%s", line, err, out.String())
		}

		return strings.Join(programLines(out.String()), "|")
	}

	const returned = "%DCL-S-RETURNED, control returned to process SYSTEM"

	dispatch(`ECHO :== "$` + echo + `"`)

	if got := dispatch("SPAWN ECHO hello there"); got != "HELLO THERE|"+returned {
		t.Errorf("SPAWN ECHO: %q", got)
	}

	if got := dispatch("SPAWN/NOWAIT/PROCESS=LATER ECHO later"); got != "%DCL-S-SPAWNED, process LATER spawned" {
		t.Errorf("SPAWN/NOWAIT: %q", got)
	}

	if n := len(c.RTL.Processes()); n != 2 {
		t.Errorf("%d processes after SPAWN/NOWAIT, want 2", n)
	}

	// LATER runs when the machine next runs: during this RUN, if the
	// scheduler gives it the CPU before the child's image ends the run,
	// or else during the SPAWN after it.
	out.Reset()

	if err := c.Run(child, console.RunOptions{}); err != nil {
		t.Fatal(err)
	}

	ran := strings.Join(programLines(out.String()), "|")
	if !strings.Contains(ran, "Hello from the child") {
		t.Errorf("RUN: %q", ran)
	}

	commands := writeCommands(t, "ECHO one", "ECHO two")
	spawned := dispatch(`SPAWN/INPUT="` + commands + `"`)

	if !strings.HasSuffix(spawned, "ONE|TWO|"+returned) {
		t.Errorf("SPAWN/INPUT: %q", spawned)
	}

	if n := strings.Count(ran+"|"+spawned, "LATER"); n != 1 {
		t.Errorf("LATER's line appeared %d times in %q and %q, want once", n, ran, spawned)
	}

	if n := len(c.RTL.Processes()); n != 1 {
		t.Errorf("%d processes at the end, want only process 1", n)
	}
}

// TestSpawn_outputFile: a subprocess whose SYS$OUTPUT is a file
// (SPAWN/OUTPUT, LIB$SPAWN's output-file) writes its lines there, the
// CLI's messages and its images' LIB$PUT_OUTPUT alike, not on the
// terminal.
func TestSpawn_outputFile(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	echo := buildImage(t, c, "echo", echoSource)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	if err := d.Dispatch(`ECHO :== "$` + echo + `"`); err != nil {
		t.Fatal(err)
	}

	log := filepath.Join(t.TempDir(), "spawn.log")
	commands := writeCommands(t, "ECHO first", "FROBNICATE", "ECHO last")

	out.Reset()

	if err := d.Dispatch(`SPAWN/INPUT="` + commands + `"/OUTPUT="` + log + `"`); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(out.String(), "FIRST") {
		t.Errorf("the subprocess wrote on the terminal: %q", out.String())
	}

	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}

	want := "FIRST\n%DCL-W-IVVERB, unrecognized command verb - check validity and spelling\n \\FROBNICATE\\\nLAST\n"
	if string(b) != want {
		t.Errorf("the output file:\n%s\nwant:\n%s", b, want)
	}
}
