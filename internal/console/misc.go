package console

import (
	"fmt"
	"strings"
	"time"
)

// Print implements the PRINT/ECHO console command: a comma-separated list
// of double-quoted literal strings and/or expressions (printed in the
// console's current radix), matching console_print.c — minus its
// SET [NO]VERBOSE gate (CONSOLE_VERBOSE), which this port always treats as
// on (SET [NO]VERBOSE isn't implemented, see doc.go).
func (c *Console) Print(text string) error {
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
				return fmt.Errorf("console: unterminated quoted string")
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
// case, minus its /TEMPORARY distinction (this port's SymbolTable doesn't
// track a separate temporary category — see symbols.go).
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

// ClearBreakpoint implements CLEAR BREAKPOINT: a specific address, or
// every breakpoint (CLEAR BREAKPOINT/ALL) — matching console_clear.c's
// clear_breakpoint case for address breakpoints (its /FAULT and
// /INSTRUCTION sub-forms aren't implemented, since fault/opcode
// breakpoints themselves aren't — see run.go).
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
