package console

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/asm"
)

// Assemble implements the batch form of the ASM <filename> command
// (console_include.c's console_asm, called through console_include with an
// implicit "/ASM"): reads path and assembles it into the console's
// persistent ASM session (c.asmSession, created fresh on first use after
// each VMINIT — see machine.go's doc comment on why multiple ASM commands
// in a row must share one assembler: a later file can reference an earlier
// one's labels, e.g. hello.asm's "@#lib$put_output" resolving to
// kernel.asm's own .ENTRY lib$put_output once kernel.asm has been ASMed
// first). Deposits the resulting P0 and S0 bytes into live memory (requires
// INIT/VMINIT, exactly like RUN's own precondition), and merges the
// session's own symbols so far (asm.Assembler.Symbols -- labels, .ENTRY
// points, .SET values, excluding the fixed builtin set every assembly
// starts with) into Console.Symbols, the same table EXAMINE/DEPOSIT/CALL's
// expression evaluator consults, so a freshly assembled program's own
// labels are immediately usable by name.
//
// Unlike console_asm.c's own interactive, line-at-a-time "assembler mode"
// (turned on by a bare ASM with no filename, then fed one line per
// subsequent console command), this only implements the batch "ASM
// <filename>" form Phase 11's whole-program Assembler is built for -- see
// docs/PHASE-11.md's own note that wiring the interactive REPL mode was
// left as follow-up work.
//
// Returns the assembler's own Entry() result, so cmdAssemble can replicate
// console.c's "if ASM_ENTRY, push_command(CALL __ENTRY)" behavior: a
// program whose .END named an explicit start address is invoked
// immediately afterward, with no arguments, exactly as the reference
// tool's own post-command hook does.
func (c *Console) Assemble(path string) (entryAddr uint32, hasEntry bool, err error) {
	if err := c.requireInit(); err != nil {
		return 0, false, err
	}

	src, err := os.ReadFile(path)
	if err != nil {
		return 0, false, err
	}

	if c.asmSession == nil {
		c.asmSession = asm.New()
		// See s0Free's doc comment: the assembler's literal default S0
		// origin (0x80000000) collides with this live VM's own S0 page
		// table, which is mapped starting at that exact virtual address.
		c.asmSession.SetS0Origin(c.s0Free)
	}
	a := c.asmSession

	dir := filepath.Dir(path)
	a.SetIncludeResolver(func(name string) (string, error) {
		b, err := os.ReadFile(filepath.Join(dir, name))
		return string(b), err
	})

	if _, err := a.Assemble(string(src)); err != nil {
		return 0, false, fmt.Errorf("console: assembling %s: %w", path, err)
	}

	if p0 := a.Bytes(); len(p0) > 0 {
		if err := c.storeBytes(a.Origin(), p0); err != nil {
			return 0, false, fmt.Errorf("console: depositing %s: %w", path, err)
		}
	}
	if s0 := a.BytesRange(a.S0Origin(), a.S0End()); len(s0) > 0 {
		if err := c.storeBytes(a.S0Origin(), s0); err != nil {
			return 0, false, fmt.Errorf("console: depositing %s: %w", path, err)
		}
	}

	for name, value := range a.Symbols() {
		kind := SymbolUser
		if strings.ContainsRune(name, '$') {
			kind = SymbolSystem
		}
		c.Symbols.Set(name, value, kind)
	}

	addr, ok := a.TakeEntry()
	return addr, ok, nil
}
