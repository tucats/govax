package console

import (
	"fmt"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// RunOptions controls RUN's optional qualifier, matching console_run's own
// single-leading-qualifier parsing: exactly one of /NOINIT, /INIT,
// /BREAK|/DEBUG|/STEP, or /NOEXECUTE may appear (not a combinable list),
// unrecognized-4-character-prefix rules and all (see dispatch.go's
// parseRunQualifier, the DCL-facing counterpart of this struct).
type RunOptions struct {
	RunInits  bool // /INIT (run each dependency's LIB$INITIALIZE); /NOINIT is the same as the zero value
	Step      bool // /BREAK, /DEBUG, /STEP: single-step the main call
	NoExecute bool // /NOEXECUTE: load and fix up, but don't transfer control
}

// Run implements the RUN <filename> command: loads fn and its sharable-
// image dependencies (imageLoad), fixes them up (imageFixup), and
// transfers control to the main image's own entry point -- optionally
// calling each dependency's LIB$INITIALIZE entry point first -- via a small
// synthesized IMAGE$INIT driver procedure run through Console.Call.
// Matches console_run.c's own strategy; see docs/PHASE-13.md's "Key
// finding" for why none of this needs a real assembler, just the same
// CALLS/PUSHL opcode bytes a real assembler would produce, written
// directly.
func (c *Console) Run(fn string, opts RunOptions) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	savedMode := c.CPU.PSL().CurMod()
	c.Engine.SetModeStack(vax.Kernel, false)
	defer c.Engine.SetModeStack(savedMode, false) // safety net on an early-error return

	c.resetICBList()
	if err := c.ensureShims(); err != nil {
		return err
	}

	main, err := c.imageLoad(fn, icbMain)
	if err != nil {
		return vmserrors.Wrap(vmserrors.CLI_ACTIVATE, err, fn)
	}
	if c.CPU.DebugEnabled(vax.DebugImages) {
		c.Printf("Main image is %s\n", main.Name)
	}

	for _, dep := range c.ICBList {
		if err := c.imageFixup(dep); err != nil {
			return vmserrors.Wrap(vmserrors.CLI_FIXUP, err, dep.Name)
		}
	}

	// console_run.c restores the caller's mode here -- before building and
	// running IMAGE$INIT, not after -- so the loaded program executes in
	// whatever mode RUN itself was invoked from.
	c.Engine.SetModeStack(savedMode, false)

	driverAddr, ok, err := c.buildImageInitDriver(main, opts.RunInits)
	if err != nil {
		return err
	}
	if !ok {
		c.Printf("No transfer address!\n")
		return nil
	}
	if opts.NoExecute {
		return nil
	}

	return c.Call(driverAddr, opts.Step)
}

// DefaultRunInits reports RUN's own default for whether to invoke each
// dependency's LIB$INITIALIZE before any /INIT or /NOINIT qualifier
// overrides it, matching console_run.c:208's `run_inits = vax.debug &
// DBG_LIBINIT` -- DebugLibinit defaults on (vax.DebugDefault), so
// LIB$INITIALIZE runs by default, not only when /INIT is given explicitly.
// See dispatch.go's parseRunQualifier, the DCL-facing counterpart.
func (c *Console) DefaultRunInits() bool {
	return c.CPU != nil && c.CPU.DebugEnabled(vax.DebugLibinit)
}

// mainTransferAddress selects icb's own user-mode entry point, matching
// console_run's own transfer[0]-then-[1]-then-[2] fallback: the first
// transfer-vector entry that looks like a plausible P0/P1 user-space
// address (nonzero, below 0x3FFFFFFF) -- transfer[0] is often a debugger/
// LIB$INITIALIZE-style entry point outside that range, hence the fallback.
func mainTransferAddress(icb *ICB) (uint32, bool) {
	for _, addr := range icb.Transfer[:3] {
		if addr > 0 && addr < 0x3FFFFFFF {
			return addr, true
		}
	}
	return 0, false
}

// buildImageInitDriver writes a small procedure at CONSOLE$SCRATCH+8 (an
// empty entry mask, then one PUSHL/PUSHL/PUSHL/CALLS sequence per
// dependency's LIB$INITIALIZE entry point if runInits, then a final CALLS
// to main's own entry point and a RET) and returns its address -- the Go
// equivalent of console_run's own assemble_direct-built IMAGE$INIT
// procedure. ok is false if main has no usable transfer address, matching
// console_run's own "No transfer address!" case: no driver is written at
// all, since a procedure with LIB$INITIALIZE calls but no final RET would
// have nothing safe to fall through to. c.ICBList[1:] is main's dependency
// list in load order (id-0 self-references live only in each ICB's own
// SHRList, never in ICBList itself, so no filtering is needed here).
func (c *Console) buildImageInitDriver(main *ICB, runInits bool) (uint32, bool, error) {
	addr, ok := mainTransferAddress(main)
	if !ok {
		return 0, false, nil
	}

	base, found := c.Symbols.Get("CONSOLE$SCRATCH")
	if !found {
		return 0, false, vmserrors.New(vmserrors.CLI_NOSCRATCH)
	}
	driverAddr := base + 8

	code := []byte{0x00, 0x00} // entry mask: no registers saved

	if runInits {
		for _, dep := range c.ICBList[1:] {
			if dep.Transfer[0] == 0 {
				continue
			}
			initAddr, ok := c.Symbols.Get(fmt.Sprintf("SHARE$%s_INITIALIZE", dep.Name))
			if !ok {
				continue
			}
			if c.CPU.DebugEnabled(vax.DebugImages) {
				c.Printf("Preparing call to LIB$INITIALIZE entry %08X for image %s\n", initAddr, dep.Name)
			}
			// LIBRTL's LIB$INITIALIZE apparently wants 100 as a special
			// flag argument -- console_run.c's own comment says as much
			// without explaining why; every other dependency gets 0
			// (matching the C source's own hardcoded-0 third argument,
			// not the commented-out "icb->transfer[0]" it never actually
			// used). Replicated as-is.
			flag := uint32(0)
			if dep.Name == "LIBRTL" {
				flag = 100
			}
			
			code = append(code, encodePushl(0)...)
			code = append(code, encodePushl(0)...)
			code = append(code, encodePushl(flag)...)
			code = append(code, encodeCalls(3, initAddr)...)
		}
	}

	code = append(code, encodeCalls(0, addr)...)
	code = append(code, 0x04) // RET

	if err := c.storeBytes(driverAddr, code); err != nil {
		return 0, false, err
	}
	return driverAddr, true, nil
}

// encodePushl returns PUSHL #v's raw opcode bytes (0xDD, general-operand
// immediate-longword mode).
func encodePushl(v uint32) []byte {
	return []byte{0xDD, 0x8F, byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
}

// encodeCalls returns CALLS #argc,@#target's raw opcode bytes (0xFB, an
// immediate-longword argument count, and an absolute (@#) target address).
func encodeCalls(argc, target uint32) []byte {
	return []byte{
		0xFB,
		0x8F, byte(argc), byte(argc >> 8), byte(argc >> 16), byte(argc >> 24),
		0x9F, byte(target), byte(target >> 8), byte(target >> 16), byte(target >> 24),
	}
}
