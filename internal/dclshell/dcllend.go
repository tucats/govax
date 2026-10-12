package dclshell

import (
	"time"

	"github.com/tucats/govax/internal/registers"
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
// DIRECTORY, and logged out, by the next prompt.
//
// Then, while the prompt waits for the line (Phase 51, subtask 4),
// process 1 goes on lending the CPU (LendWhileReading): cmd/govax reads
// the terminal on a goroutine of its own, and the others run until the
// line comes, with no time limit. When each of them waits, the host
// waits a little (promptPoll, HostIdle's time, nobody's) and the
// scheduler looks again, since a wait may end by the host's clock (a
// subprocess's WAIT) or a timer. With no other process, the prompt just
// waits for the line, as it always has.
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
func (c *Interpreter) commandBoundary() {
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

// LendWhileIdle is process 1's step after a command at its command level
// 0 (see above): it lends the CPU, waiting, until no other process wants
// it, or promptLendLimit has passed, or an attention key is typed.
func (c *Interpreter) LendWhileIdle() {
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
		return idle() || !time.Now().Before(limit) || c.attentionPending()
	}, c.machineDeadline(promptLendLimit))
}

// promptPoll is how long LendWhileReading waits in the host, when every
// other process waits, before the scheduler looks at their waits again.
const promptPoll = 10 * time.Millisecond

// LendWhileReading is process 1's DCL waiting for a line at command
// level 0 (see above): the front end reads the line meanwhile, and line
// is closed when the read is over (a line typed, Ctrl/C, Ctrl/Z, or the
// end of the input). Until then the other processes run. It returns
// once line is closed; when it can't lend the CPU (no other process, the
// debugger's prompt, the interactive assembler's), or a key has stopped
// the machine, or a stop in another process (a HALT, a run limit)
// ended a lending, it just waits for line.
func (c *Interpreter) LendWhileReading(line <-chan struct{}) {
	read := func() bool {
		select {
		case <-line:
			return true
		default:
			return false
		}
	}

	// Process 1 waits while it lends the CPU: the others are idle when
	// no process is current or computable.
	idle := func() bool {
		s := c.RTL.Scheduler()
		_, current := s.Current()

		return !current && len(s.Computable()) == 0
	}

	for !read() {
		if !c.Running() || !c.canLend() || c.attentionPending() {
			break
		}

		ran, stopped := c.lendCPU(func() bool { return read() || idle() || c.attentionPending() }, 0)

		switch {
		case stopped:
			<-line

			return

		case ran:
			continue
		}

		// Nothing ran: each of the others waits, or the scheduler chose
		// process 1 first. The host waits a little, ending at once if
		// the line comes.
		c.RTL.HostIdle(c.machine.Engine, func() {
			select {
			case <-line:
			case <-time.After(promptPoll):
			}
		})
	}

	<-line
}

// attentionPending reports whether Ctrl/C or Ctrl/Y is waiting for the
// machine's attention.
func (c *Interpreter) attentionPending() bool {
	return c.machine.Engine != nil && isAttentionKey(c.machine.Engine.PendingAttention())
}

// canLend reports whether process 1's DCL can lend the CPU now (see
// above).
func (c *Interpreter) canLend() bool {
	switch {
	case c.lending, c.consoleCommands > 0, c.subCLI != nil:
		return false
	case c.machine.Engine == nil, c.RTL == nil, c.RTL != c.machine.ProcessOne():
		return false
	case c.machine.Engine.SchedulingFrozen(), c.machine.InDebugger(), c.machine.InAssemblerMode():
		return false
	case c.machine.RunningProcess() != c.RTL:
		return false
	}

	return len(c.RTL.Processes()) > 1
}

// lendCPU lends the CPU to the other processes (see above) until the
// scheduler chooses process 1 again: process 1 yields, if over is nil,
// or else waits until over reports true (deadline being corevms's
// LendCPU's). It reports whether another process ran, and whether a
// stop in another process (a HALT, a limit) ended the lending; the stop
// is reported as a run's would be. Process 1 has the CPU back at the
// end, whatever happened.
func (c *Interpreter) lendCPU(over func() bool, deadline uint64) (ran, stopped bool) {
	if !c.canLend() || c.attentionPending() {
		return false, false
	}

	if !c.RTL.LendCPU(c.RTL, over, deadline) {
		return false, false
	}

	c.lending = true

	defer func() {
		c.lending = false

		if err := c.RTL.ReclaimCPU(c.machine.Engine, c.RTL); err != nil {
			c.Printf("%%%s\n", err)
		}
	}()

	c.machine.Engine.BeginRun()

	for {
		// A key typed meanwhile is process 1's: DCL acts on it after
		// the command (dclcontroly.go).
		if c.attentionPending() {
			return ran, false
		}

		err := c.machine.BeginStep()

		// The scheduler has chosen process 1: it gets the CPU back
		// before it runs an instruction.
		if err == nil && c.machine.RunningProcess() == c.RTL {
			return ran, false
		}

		if err == nil {
			ran = true
			finish := c.machine.TraceStep(c.machine.CPU.GPR(registers.PC), false)

			if err = c.machine.StepMachine(); err == nil {
				finish()

				continue
			}
		}

		if err := c.machine.ReportStop(err); err != nil {
			c.Printf("%%%s\n", err)
		}

		return ran, true
	}
}

// machineDeadline is the system time a wait of d from now ends at, for
// the scheduler's idling (corevms's WaitUntil): with the hardware clock,
// the system time is the host's, and the scheduler, with nothing to run,
// sleeps until then; otherwise 0, since the system time counts
// instructions and would jump ahead of the host's.
func (c *Interpreter) machineDeadline(d time.Duration) uint64 {
	if c.machine.Engine == nil || !c.machine.Engine.HardwareClock() {
		return 0
	}

	return c.machine.Engine.SystemTime() + uint64(d/(time.Second/time.Duration(vmsdef.TicksPerSecond)))
}
