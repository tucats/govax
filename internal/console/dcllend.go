package console

import (
	"time"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// Letting the other processes run between DCL commands (docs/PHASE-50,
// subtask 21).
//
// DCL is Go, so a DCL command runs no instructions, and while process 1's
// DCL reads and runs commands the machine is stopped: a subprocess that
// SPAWN/NOWAIT made would run only at the next RUN, GO, or CALL. On VMS
// it runs beside its parent, the two taking turns at the CPU. So after
// each DCL command, at the terminal or in a command procedure, the
// command interpreter gives the CPU up (commandBoundary):
//
//   - Process 1's DCL lends it (lendCPU, corevms's LendCPU): process 1
//     yields, staying computable, and the machine runs the other
//     processes until the scheduler chooses process 1 again, by its
//     usual rules, and stops there, before process 1 runs anything. A
//     process of the same priority gets about a turn (a quantum, or one
//     DCL command of its own) before process 1 has its next command.
//   - A subprocess's DCL hands the CPU back to its shim (yieldCommand),
//     which yields (corevms's Yield): the process stays computable, and
//     its next command runs when the scheduler next chooses it.
//
// So a SPAWN/NOWAIT @FOO.COM run in BAR.COM interleaves the two
// procedures' commands. Process 1's WAIT lends the CPU for its whole
// time, waiting in the scheduler, and a subprocess's WAIT waits in the
// scheduler too (dclwait.go), so neither stops the other processes.
//
// A command at process 1's command level 0 (the terminal's, govax's
// one-shot command) is followed by its DCL reading the next one: on VMS,
// waiting for the terminal while the other processes run. So there
// process 1 lends the CPU waiting (lendWhileIdle), until no other process
// wants it (each waits, or has gone), or for promptLendLimit of the
// host's time at most, so that a process that never waits doesn't keep
// the prompt from coming back: SPAWN/NOWAIT DIRECTORY has run the
// DIRECTORY, and logged out, by the next prompt. (Only the time between
// commands is lent; what runs while the prompt waits for a line is
// govax's front end, not the machine.)
//
// Process 1 lends the CPU only from its DCL's own level, with the CPU in
// process 1 and the machine stopped: not inside a machine run (an image's
// XFC$CONSOLE_CMD, or a nested run with scheduling frozen), not while the
// debugger or the interactive assembler reads the input, and only when
// there is another process to run. A Ctrl/C or Ctrl/Y typed meanwhile
// ends the lending, and is left for DCL (dclcontroly.go).

// commandBoundary is the step after each DCL command, at command level 0
// and in a procedure: the CPU is given up for the other processes (see
// above).
func (c *Console) commandBoundary() {
	if !c.Running() {
		return
	}

	if cli := c.subCLI; cli != nil {
		cli.handBack(cliYield{kind: yieldCommand})

		return
	}

	c.lendCPU(nil, 0)
}

// promptLendLimit is the longest process 1 lends the CPU after a command
// at its command level 0 (lendWhileIdle).
const promptLendLimit = 50 * time.Millisecond

// lendWhileIdle is process 1's step after a command at its command level
// 0 (see above): it lends the CPU, waiting, until no other process wants
// it, or promptLendLimit has passed, or an attention key is typed.
func (c *Console) lendWhileIdle() {
	if !c.Running() || !c.canLend() {
		return
	}

	limit := time.Now().Add(promptLendLimit)
	s := c.RTL.Scheduler()

	// Process 1 is waiting meanwhile, so it is neither current nor
	// computable: the others are idle when no process is.
	idle := func() bool {
		_, current := s.Current()

		return !current && len(s.Computable()) == 0
	}

	c.lendCPU(func() bool {
		return idle() || !time.Now().Before(limit) || isAttentionKey(c.Engine.PendingAttention())
	}, c.machineDeadline(promptLendLimit))
}

// canLend reports whether process 1's DCL can lend the CPU now (see
// above).
func (c *Console) canLend() bool {
	switch {
	case c.lending, c.consoleCommands > 0, c.subCLI != nil:
		return false
	case c.Engine == nil, c.RTL == nil, c.RTL != c.processOne:
		return false
	case c.Engine.SchedulingFrozen(), c.InDebugger(), c.InAssemblerMode():
		return false
	case c.running() != c.RTL:
		return false
	}

	return len(c.RTL.Processes()) > 1
}

// lendCPU lends the CPU to the other processes (see above) until the
// scheduler chooses process 1 again: process 1 yields, if over is nil,
// or else waits until over reports true (deadline being corevms's
// LendCPU's). It reports whether another process ran. A stop in another
// process (a HALT, a limit) is reported as a run's would be, and ends
// the lending; process 1 has the CPU back at the end, whatever happened.
func (c *Console) lendCPU(over func() bool, deadline uint64) bool {
	if !c.canLend() || isAttentionKey(c.Engine.PendingAttention()) {
		return false
	}

	if !c.RTL.LendCPU(c.RTL, over, deadline) {
		return false
	}

	c.lending = true

	defer func() {
		c.lending = false

		if err := c.RTL.ReclaimCPU(c.Engine, c.RTL); err != nil {
			c.Printf("%%%s\n", err)
		}
	}()

	c.Engine.BeginRun()

	ran := false

	for {
		// A key typed meanwhile is process 1's: DCL acts on it after
		// the command (dclcontroly.go).
		if isAttentionKey(c.Engine.PendingAttention()) {
			return ran
		}

		err := c.BeginStep()

		// The scheduler has chosen process 1: it gets the CPU back
		// before it runs an instruction.
		if err == nil && c.running() == c.RTL {
			return ran
		}

		if err == nil {
			ran = true
			finish := c.traceStep(c.CPU.GPR(vax.PC), false)

			if err = c.StepMachine(); err == nil {
				finish()

				continue
			}
		}

		if err := c.ReportStop(err); err != nil {
			c.Printf("%%%s\n", err)
		}

		return ran
	}
}

// machineDeadline is the system time a wait of d from now ends at, for
// the scheduler's idling (corevms's WaitUntil): with the hardware clock,
// the system time is the host's, and the scheduler, with nothing to run,
// sleeps until then; otherwise 0, since the system time counts
// instructions and would jump ahead of the host's.
func (c *Console) machineDeadline(d time.Duration) uint64 {
	if c.Engine == nil || !c.Engine.HardwareClock() {
		return 0
	}

	return c.Engine.SystemTime() + uint64(d/(time.Second/time.Duration(vmsdef.TicksPerSecond)))
}
