package console

import "github.com/tucats/govax/internal/corevms"

// Interpreter is the console's DCL, every process's: each process that
// runs DCL has a Shell of its own (process 1's is home, the others'
// their CLIs'), and the one whose commands run now is embedded, so its
// fields read as the interpreter's (c.Out, c.levels). Console embeds the
// Interpreter.
type Interpreter struct {
	// Shell is the DCL whose commands run now (shell.go): process 1's
	// (home), or a subprocess's while its interpreter runs.
	*Shell

	// control is SET [NO]CONTROL's (dclcontroly.go).
	control controlKeys

	// home is process 1's DCL: the Shell embedded here, but for the while
	// a subprocess's DCL runs, when that is the subprocess's (subcli.go).
	home *Shell

	// clis are the command interpreters of the processes that run one
	// (subcli.go: LIB$SPAWN's, and $CREPRC's of LOGINOUT), by process.
	clis map[*corevms.Environment]*subprocessCLI

	// spawnPage is the S0 pool page SPAWN writes its call to LIB$SPAWN
	// in (spawncmd.go), allocated on first use on each machine.
	spawnPage uint32

	// lending is set while process 1 lends the CPU to the other
	// processes between its DCL commands, and consoleCommands counts the
	// XFC$CONSOLE_CMD calls in progress, during which the machine is
	// mid-instruction and can't run others (dcllend.go).
	lending         bool
	consoleCommands int
}
