package console

import (
	"strings"
	"time"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vmserrors"
)

// CommandLineString is any "left over" text from govax's own command line,
// put here by cmd/govax when it parses its options: the one-shot command,
// which RunCommandLine runs once vax.init is done. Empty, there is none.
var CommandLineString string

// RunCommandLine is the text after the image's file name in govax's own
// "run" command line (cmd/govax): the one-shot command's RUN gives it to
// the image as its command text (RunOptions.CommandLine), as a foreign
// command's parameters are given. The host's shell has already parsed it,
// so it's passed as it is, without DCL's uppercasing.
var RunCommandLine string

// Print implements the PRINT/ECHO console command: each item is a
// double-quoted literal string, printed as it is, or an expression,
// printed in the console's current radix: PRINT is silent whenever
// SET NOVERBOSE has turned Console.Verbose off (see set.go's
// SetVerbose/SetNoVerbose). The items are the DCL grammar's
// list of $expression values (docs/PHASE-37.md), quotes kept.
func (c *Console) Print(items []string) error {
	if !c.Verbose {
		return nil
	}

	ev := c.Evaluator()

	for _, item := range items {
		// A quoted string by itself is text to print; one inside a
		// larger expression is the evaluator's string literal.
		if strings.HasPrefix(item, `"`) {
			end := strings.IndexByte(item[1:], '"')
			if end < 0 {
				return vmserrors.New(vmserrors.CLI_UNTERMSTR)
			}

			if end+2 == len(item) {
				c.Printf("%s", item[1:end+1])

				continue
			}
		}

		v, rest, err := ev.Eval(item)
		if err != nil {
			return err
		}

		if extra := strings.TrimSpace(rest); extra != "" {
			return vmserrors.New(vmserrors.CLI_EXTRAPARAMETER, extra)
		}

		if c.Radix == 10 {
			c.Printf("%d", int32(v))
		} else {
			c.Printf("%08X", v)
		}
	}

	c.Printf("\n")

	return nil
}

// Running reports whether the console should keep reading commands,
// matching vax.console.running (cleared by QUIT/EXIT.
func (c *Console) Running() bool { return !c.quit }

// CommandLineErr returns the failure of the one-shot command given on
// govax's command line, or nil.
func (c *Console) CommandLineErr() error { return c.commandLineErr }

// Quit implements QUIT/EXIT: stops the command loop.
func (c *Console) Quit() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.quit = true

	return nil
}

// Time runs one command (via dispatch) and prints how long it took.
func (c *Console) Time(cmd string, dispatch func(string) error) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if strings.TrimSpace(cmd) == "" {
		c.Printf("Time is %s\n", time.Now().Format(time.RFC1123))

		return nil
	}

	start := time.Now()
	err := dispatch(cmd)
	elapsed := time.Since(start)

	c.Printf("Elapsed time: %f seconds\n", elapsed.Seconds())

	return err
}

// SetRunLimits records govax's --instruction-limit and --time-limit
// options, the most instructions and the longest host time one run of the
// VAX may take (0 is no limit). They don't take effect at once: see
// ApplyRunLimits, and the fields' comment in machine.go on why.
func (c *Console) SetRunLimits(instructions int, duration time.Duration) {
	c.instructionLimit = instructions
	c.timeLimit = duration
}

// ApplyRunLimits puts the limits SetRunLimits recorded onto the Engine,
// so every run from now on is held to them. govax calls it once the boot
// script is done, before the interactive prompt; RunCommandLine calls
// it before the one-shot command.
func (c *Console) ApplyRunLimits() {
	if c.Engine != nil {
		c.Engine.SetLimits(c.instructionLimit, c.timeLimit)
	}
}

// RunCommandLine runs the one-shot command: the text left on govax's own
// command line once its options are parsed (CommandLineString). cmd/govax
// calls it once vax.init has run, as a login command procedure runs before
// the command interpreter reads its input (docs/PHASE-50 - DCL command
// procedures.md): the command is level 0's input, in place of the
// terminal, and the session ends when it's done. It reports whether there
// was a command to run; its failure is CommandLineErr's.
func (c *Console) RunCommandLine(dispatch func(string) error) bool {
	if CommandLineString == "" {
		return false
	}

	text := CommandLineString
	CommandLineString = ""
	c.runCommandLine, RunCommandLine = RunCommandLine, ""

	// The command is the user's program, not boot: hold it to the
	// --instruction-limit and --time-limit options.
	c.ApplyRunLimits()

	// The command ends the session either way: a failed one-shot command
	// shouldn't leave the user at a prompt. run (cmd/govax) reports its
	// failure and exits nonzero.
	c.limitStop = nil

	defer func() { c.quit = true }()

	if status := dispatch(text); status != nil {
		c.commandLineErr = status
	} else if c.limitStop != nil {
		// The program was stopped by --instruction-limit or --time-limit.
		// ReportStop has shown the message and let the command finish
		// normally, as it should at the prompt; but a one-shot command
		// that didn't run to its end has failed. The message is marked as
		// shown already, so govax doesn't print it a second time.
		c.commandLineErr = vmserrors.InhibitMessage(c.limitStop)
	}

	return true
}

// ClearSymbol implements the debugger's CANCEL (or CLEAR) SYMBOL: a
// specific name, or every user symbol (/ALL). Its /TEMPORARY distinction
// is ClearSymbolTemporary, below.
func (c *Console) ClearSymbol(name string, all bool) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if all {
		c.Symbols.ClearAll()

		return nil
	}

	c.Symbols.Delete(name)

	return nil
}

// ClearSymbolTemporary implements the debugger's CANCEL SYMBOL/TEMPORARY.
func (c *Console) ClearSymbolTemporary() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Symbols.ClearTemporary()

	return nil
}

// ClearString implements CLEAR STRINGS: resets CONSOLE$STRINGPOOL
// back to CONSOLE$STRINGPOOL_BASE and zeroes the pool's backing
// storage — the write side of ShowString, requiring the
// same booted-microkernel symbols.
func (c *Console) ClearString() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	base, ok := c.Symbols.Get("CONSOLE$STRINGPOOL_BASE")
	if !ok {
		return vmserrors.New(vmserrors.CLI_NOSTRINGPOOL)
	}

	size, ok := c.Symbols.Get("CONSOLE$STRINGPOOL_SIZE")
	if !ok {
		return vmserrors.New(vmserrors.CLI_NOPOOLSIZE)
	}

	c.Symbols.Set("CONSOLE$STRINGPOOL", base, SymbolSystem)

	return c.Mem.Store(c.CPU, base, make([]byte, size))
}

// ClearTB implements CLEAR TB: a full translation-buffer flush plus a
// reset of its tries/hits/pflushes counters — tb_flush itself and the
// sequential translation cache's own try/hit counters are deliberately
// left alone.
func (c *Console) ClearTB() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Mem.InvalidateTB()
	c.Mem.ResetTBCounters()

	return nil
}

// ClearMemory implements CLEAR MEMORY,  which simply calls console_zero(),
// the same routine the ZERO command itself runs.
func (c *Console) ClearMemory() error {
	return c.Zero()
}

// ClearInterrupt implements CLEAR INTERRUPT <id>. It removes every 
// queued interrupt whose code matches id.
func (c *Console) ClearInterrupt(code uint32) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	n := c.Engine.ClearInterrupt(cpu.Exception(code))

	plural := "s"
	if n == 1 {
		plural = ""
	}

	c.Printf("\tCleared %d pending interrupt%s\n", n, plural)

	return nil
}

// ClearAllInterrupts implements CLEAR INTERRUPT/ALL: empties the 
// interrupt queue and cancels any immediately-pending interrupt.
func (c *Console) ClearAllInterrupts() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	n := c.Engine.ClearAllInterrupts()

	plural := "s"
	if n == 1 {
		plural = ""
	}

	c.Printf("\tCleared %d pending interrupt%s\n", n, plural)

	return nil
}
