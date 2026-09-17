package console

import (
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
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
// This is the batch "ASM <filename>" form. The bare, no-filename form that
// puts the console into interactive line-at-a-time assembler mode is
// AssembleBegin/AssembleInteractiveLine (docs/PHASE-19.md); both forms share
// the same persistent asmSession, ensureAsmSession/depositAsmImage/
// mergeAsmSymbols helpers below, so a file assembled in one form can be
// continued in the other.
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

	src, err := c.Paths.ReadFile(path)
	if err != nil {
		return 0, false, err
	}

	a := c.ensureAsmSession()

	// .INCLUDE names (e.g. kernel.asm's own "ssdef.asm") are resolved
	// alongside path's own directory first (WithDir), falling through to
	// c.Paths' own search directories/embedded fallback — see
	// docs/PHASE-15.md. Overrides ensureAsmSession's own directory-less
	// default resolver every call, so a session shared across several ASM
	// <file> commands always resolves .INCLUDE relative to whichever file
	// is currently being assembled.
	includePaths := c.Paths.WithDir(filepath.Dir(path))

	a.SetIncludeResolver(func(name string) (string, error) {
		b, err := includePaths.ReadFile(name)

		return string(b), err
	})

	if _, err := a.Assemble(string(src)); err != nil {
		return 0, false, vmserrors.Wrap(vmserrors.CLI_ASSEMBLING, err, path)
	}

	if err := c.depositAsmImage(a); err != nil {
		return 0, false, vmserrors.Wrap(vmserrors.CLI_DEPOSITING, err, path)
	}

	c.mergeAsmSymbols(a)

	addr, ok := a.TakeEntry()

	return addr, ok, nil
}

// ensureAsmSession returns the console's persistent ASM session, lazily
// creating one on first use after each VMINIT/ZERO — see asmSession's own
// doc comment (machine.go) for why one Assembler is shared across every
// "ASM <file>"/interactive-mode command in a row.
func (c *Console) ensureAsmSession() *asm.Assembler {
	if c.asmSession == nil {
		// Pass down the console's verbose flag to the assember
		c.asmSession = asm.New(c.Verbose)

		// vax.console.deposit is one shared "current address" register in
		// the reference tool -- EXAMINE/DEPOSIT/interactive-ASM all read
		// and advance the same field. This port tracks it as
		// Console.DepositAddr; seed a freshly created session from it
		// (rather than the assembler package's own hardcoded default)
		// so e.g. "DEP 4000" then a bare "ASM" starts assembling where the
		// user just pointed, matching the reference tool. Existing batch
		// "ASM <file>" behavior is unaffected when DepositAddr is still at
		// its post-VMINIT default, since that default already equals the
		// assembler's own (0x200).
		c.asmSession.SetOrigin(c.DepositAddr)
		// See s0Free's doc comment: the assembler's literal default S0
		// origin (0x80000000) collides with this live VM's own S0 page
		// table, which is mapped starting at that exact virtual address.
		c.asmSession.SetS0Origin(c.s0Free)
		// .SCB/.VECTOR compute their target address from the assembler's
		// own configured SCBB (pseudoSCB writes to 0x80000000+scbb+code),
		// which must match the live SCBB privileged register VMINIT set --
		// otherwise a kernel.asm .SCB entry lands at a physical address
		// the CPU's own exception dispatch never actually consults,
		// leaving every real exception vector reading whatever garbage
		// happens to be at the live SCBB instead.
		c.asmSession.SetSCBB(c.CPU.PR(vax.SCBB))
		// A directory-less default .INCLUDE resolver for interactive mode,
		// which has no "current file" of its own to bias the search with
		// (unlike Assemble's own per-call WithDir override above).
		c.asmSession.SetIncludeResolver(func(name string) (string, error) {
			b, err := c.Paths.ReadFile(name)

			return string(b), err
		})
	}

	return c.asmSession
}

// depositAsmImage stores everything a's assembled so far into live memory:
// the P0 program, any S0 data (.REGION), and the .SCB/.VECTOR page --
// shared by both the batch "ASM <file>" form and interactive-mode's own
// per-statement loop, since either can extend any of these regions.
func (c *Console) depositAsmImage(a *asm.Assembler) error {
	if p0 := a.Bytes(); len(p0) > 0 {
		if err := c.storeBytes(a.Origin(), p0); err != nil {
			return err
		}
	}

	if s0 := a.BytesRange(a.S0Origin(), a.S0End()); len(s0) > 0 {
		if err := c.storeBytes(a.S0Origin(), s0); err != nil {
			return err
		}
	}

	// .SCB/.VECTOR poke a longword directly at 0x80000000+SCBB+code (see
	// pseudoSCB), a fixed address in VMINIT's own dedicated SCB page --
	// deliberately *below* S0Origin (kernel.asm's own code starts past the
	// SCB page, not inside it), so it falls outside the BytesRange above
	// and would otherwise sit forever in the assembler's own image buffer,
	// never reaching live memory. Depositing this page on every call is
	// harmless (idempotent) even when nothing has written a .SCB entry.
	scbb := 0x80000000 + c.CPU.PR(vax.SCBB)
	if scb := a.BytesRange(scbb, scbb+512); len(scb) > 0 {
		if err := c.storeBytes(scbb, scb); err != nil {
			return err
		}
	}

	return nil
}

// mergeAsmSymbols merges every symbol a has defined so far into
// Console.Symbols (the table EXAMINE/DEPOSIT/CALL's expression evaluator
// consults), so a freshly assembled label is immediately usable by name --
// shared by the batch and interactive-mode forms alike.
func (c *Console) mergeAsmSymbols(a *asm.Assembler) {
	for name, info := range a.Symbols() {
		kind := SymbolUser
		if strings.ContainsRune(name, '$') {
			kind = SymbolSystem
		}

		if info.Entry {
			c.Symbols.SetEntry(name, info.Value, kind)
		} else {
			c.Symbols.Set(name, info.Value, kind)
		}
	}
}

// InAssemblerMode reports whether a bare "ASM" command has put the console
// into interactive assembler-mode (docs/PHASE-19.md) -- Dispatcher.Dispatch
// consults this ahead of normal verb-table lookup, and main.go's own
// readline prompt consults it to switch to "ASM> ", both matching the
// reference tool.
func (c *Console) InAssemblerMode() bool { return c.assemblerMode }

// AssembleBegin implements the bare "ASM" (no filename) command: puts the
// console into interactive assembler mode, matching console_asm's own
// isend(*p) branch. Every subsequent line reaching Dispatcher.Dispatch is
// handed to AssembleInteractiveLine instead of the normal command table
// until a bare or dotted END statement ends the mode.
func (c *Console) AssembleBegin() error {
	if err := c.requireInit(); err != nil {
		return err
	}

	c.ensureAsmSession().BeginInteractive()
	c.assemblerMode = true

	return nil
}

// AssembleInteractiveLine assembles one line typed while InAssemblerMode is
// true, matching assemble()'s own role as console_dispatch's direct hand-off
// target while vax.console.assembler_mode is set. A statement error is
// returned for display but does *not* exit assembler mode -- matching the
// reference tool, where a typo stays correctable rather than dropping back
// to the ordinary command prompt. Once END/.END stops assembly, assembler
// mode is turned back off and any unresolved forward references are warned
// about (ASM_WARNFORWARD, on by default); entryAddr/hasEntry then report
// whether an entry address was named, exactly like Assemble's own
// TakeEntry-based return, so the caller can issue the same one-shot
// "CALL __ENTRY" the reference tool's post-command hook does.
func (c *Console) AssembleInteractiveLine(line string) (done bool, entryAddr uint32, hasEntry bool, err error) {
	a := c.ensureAsmSession()

	done, err = a.AssembleLine(line)
	if err != nil {
		return false, 0, false, vmserrors.Wrap(vmserrors.CLI_ASSEMBLING, err, "ASM")
	}

	if err := c.depositAsmImage(a); err != nil {
		return false, 0, false, vmserrors.Wrap(vmserrors.CLI_DEPOSITING, err, "ASM")
	}

	c.mergeAsmSymbols(a)
	c.DepositAddr = a.Deposit()

	if !done {
		return false, 0, false, nil
	}

	c.assemblerMode = false

	if a.HasUnresolvedSymbols() {
		c.Printf("%%There are unresolved forward references\n")
	}

	entryAddr, hasEntry = a.TakeEntry()

	return true, entryAddr, hasEntry, nil
}
