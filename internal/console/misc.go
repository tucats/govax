package console

import (
	"strings"
	"time"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vmserrors"
)

// Print implements the PRINT/ECHO console command: a comma-separated list
// of double-quoted literal strings and/or expressions (printed in the
// console's current radix), matching console_print.c — including its
// CONSOLE_VERBOSE gate (console_print's own leading check): PRINT is
// silent whenever SET NOVERBOSE has turned Console.Verbose off (see
// set.go's SetVerbose/SetNoVerbose).
func (c *Console) Print(text string) error {
	if !c.Verbose {
		return nil
	}

	ev := c.Evaluator()
	pos := text

	for {
		pos = strings.TrimLeft(pos, " \t")
		if pos == "" {
			break
		}

		if pos[0] == ',' {
			pos = pos[1:]

			continue
		}

		if pos[0] == '"' {
			end := strings.IndexByte(pos[1:], '"')
			if end < 0 {
				return vmserrors.New(vmserrors.CLI_UNTERMSTR)
			}

			c.Printf("%s", pos[1:end+1])
			pos = pos[end+2:]

			continue
		}

		v, rest, err := ev.Eval(pos)
		if err != nil {
			return err
		}

		if c.Radix == 10 {
			c.Printf("%d", int32(v))
		} else {
			c.Printf("%08X", v)
		}

		pos = rest
	}

	c.Printf("\n")

	return nil
}

// Running reports whether the console should keep reading commands,
// matching vax.console.running (cleared by QUIT/EXIT — see
// console_quit.c's console_exit_dcl).
func (c *Console) Running() bool { return !c.quit }

// Quit implements QUIT/EXIT: stops the command loop.
func (c *Console) Quit() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.quit = true

	return nil
}

// Time runs one command (via dispatch) and prints how long it took,
// matching console_time.c — minus its Mac-only instruction-count/MIPS
// reporting (vax.console.instruction_count has no equivalent counter in
// internal/cpu.Engine to read).
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

// Include reads path line by line, calling dispatch for each non-blank,
// non-comment ("!"-prefixed) line — a simplified stand-in for
// console_include.c's push_include/INCLUDE-stack machinery (which supports
// nested includes via a file stack, /VERIFY echoing, and an ASM-mode
// variant): this port just runs straight through one file, recursively,
// since INCLUDE's only in-scope consumer right now is loading a startup
// script like vax.init (see cmd/govax/main.go). path is resolved through
// c.Paths (docs/PHASE-15.md), so an unqualified name like "vax.init" is
// found via the configured search path / embedded fallback, not just a
// literal relative-to-cwd read.
func (c *Console) Include(path string, dispatch func(string) error) error {
	b, err := c.Paths.ReadFile(path)
	if err != nil {
		return err
	}

	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "!") {
			continue
		}

		if err := dispatch(line); err != nil {
			c.Printf("%s: %v\n", path, err)
		}
	}

	return nil
}

// ClearSymbol implements CLEAR SYMBOL: a specific name, or every user
// symbol (CLEAR SYMBOL/ALL) — matching console_clear.c's clear_symbols
// case. Its /TEMPORARY distinction is ClearSymbolTemporary, below.
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

// ClearSymbolTemporary implements CLEAR SYMBOL/TEMPORARY, matching
// console_clear.c's clear_temp_symbols (case 115) — see
// SymbolTable.ClearTemporary.
func (c *Console) ClearSymbolTemporary() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Symbols.ClearTemporary()

	return nil
}

// ClearString implements CLEAR STRINGS, matching console_clear.c's case
// 105: resets CONSOLE$STRINGPOOL back to CONSOLE$STRINGPOOL_BASE and zeroes
// the pool's backing storage — the write side of ShowString, requiring the
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

// ClearTB reports that this port has no translation-cache state to reset,
// matching ShowTB's own "not applicable to this port" stance (see
// docs/PHASE-16.md sub-phase 1f: internal/vm.Memory.Translate does an
// uncached page-table walk, so console_clear.c's CLEAR TB — which resets
// tb_hit/tb_try/cached_page_hit/cached_page_try counters this port never
// maintains — has nothing to do here).
func (c *Console) ClearTB() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Printf("Translation buffer statistics are not modeled by this port (uncached page-table walk).\n")

	return nil
}

// ClearMemory implements CLEAR MEMORY, matching console_clear.c's case 103
// — which simply calls console_zero(), the same routine the ZERO command
// itself runs (see console_clear.c's own module comment: "CLEAR MEMORY is
// mapped to the ZERO command").
func (c *Console) ClearMemory() error {
	return c.Zero()
}

// ClearMemoryStatistics implements CLEAR MEMORY/STATISTICS, matching
// console_clear.c's case 114 — which resets allocator byte-counters
// (total_allocated/count_allocated/total_freed/count_freed) this port has
// no equivalent of (internal/vm.Memory is a fixed-size byte slice, not a
// tracked heap allocator).
func (c *Console) ClearMemoryStatistics() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.Printf("Memory allocation statistics are not modeled by this port.\n")

	return nil
}

// ClearInterrupt implements CLEAR INTERRUPT <id>, matching console_clear.c's
// case 102: removes every queued interrupt whose code matches id.
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

// ClearAllInterrupts implements CLEAR INTERRUPT/ALL, matching
// console_clear.c's case 110: empties the interrupt queue and cancels any
// immediately-pending interrupt.
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

// ClearBreakpoint implements CLEAR BREAKPOINT: a specific address, or
// every breakpoint (CLEAR BREAKPOINT/ALL) — matching console_clear.c's
// clear_breakpoint case for address breakpoints. Its /FAULT sub-form isn't
// implemented, since fault breakpoints themselves aren't (see execute.go's
// BreakKind doc comment); /INSTRUCTION is implemented, but as an entirely
// separate command path (its own DCL syntax, dispatched straight to
// RemoveInstructionBreakpoint/ClearAllInstructionBreakpoints in
// instbreak.go) rather than through this function — matching the C
// source's own instruction[].debugdata mechanism, never part of
// clear_breakpoint's address-oriented breakpoint_list walk either. See
// docs/PHASE-18.md.
func (c *Console) ClearBreakpoint(addr uint32, all bool) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if all {
		c.ClearAllBreakpoints()

		return nil
	}

	c.RemoveBreakpoint(addr)

	return nil
}
