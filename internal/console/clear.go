package console

import (
	"strings"
	"time"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vmserrors"
)

// Print implements the PRINT/ECHO console command: each item is a
// double-quoted literal string, printed as it is, or an expression,
// printed in the console's current radix: PRINT is silent whenever
// SET NOVERBOSE has turned Console.Verbose off (see set.go's
// SetVerbose/SetNoVerbose). The items are the DCL grammar's
// list of $expression values (docs/PHASE-37.md), quotes kept.
func (c *Machine) Print(items []string) error {
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

// SetRunLimits records govax's --instruction-limit and --time-limit
// options, the most instructions and the longest host time one run of the
// VAX may take (0 is no limit). They don't take effect at once: see
// ApplyRunLimits, and the fields' comment in machine.go on why.
func (c *Machine) SetRunLimits(instructions int, duration time.Duration) {
	c.instructionLimit = instructions
	c.timeLimit = duration
}

// ApplyRunLimits puts the limits SetRunLimits recorded onto the Engine,
// so every run from now on is held to them. govax calls it once the boot
// script is done, before the interactive prompt; RunCommandLine calls
// it before the one-shot command.
func (c *Machine) ApplyRunLimits() {
	if c.Engine != nil {
		c.Engine.SetLimits(c.instructionLimit, c.timeLimit)
	}
}

// ClearSymbol implements the debugger's CANCEL (or CLEAR) SYMBOL: a
// specific name, or every user symbol (/ALL). Its /TEMPORARY distinction
// is ClearSymbolTemporary, below.
func (c *Machine) ClearSymbol(name string, all bool) error {
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
func (c *Machine) ClearSymbolTemporary() error {
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
func (c *Machine) ClearString() error {
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
func (c *Machine) ClearTB() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Mem.InvalidateTB()
	c.Mem.ResetTBCounters()

	return nil
}

// ClearMemory implements CLEAR MEMORY,  which simply calls console_zero(),
// the same routine the ZERO command itself runs.
func (c *Machine) ClearMemory() error {
	return c.Zero()
}

// ClearInterrupt implements CLEAR INTERRUPT <id>. It removes every
// queued interrupt whose code matches id.
func (c *Machine) ClearInterrupt(code uint32) error {
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
func (c *Machine) ClearAllInterrupts() error {
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
