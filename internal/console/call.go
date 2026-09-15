package console

import (
	"errors"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// Call implements the console CALL command's core mechanism (see
// docs/PHASE-13.md): invokes the procedure at addr with zero arguments,
// then runs Engine.Step in a loop until it returns (cpu.CallEntry/
// ErrConsoleCallReturned) or halts (cpu.ErrHalted). If step is true, each
// instruction executed is traced with its resulting PC, matching
// console_run.c's /STEP qualifier on RUN.
//
// This is Phase 13's own primitive need (RUN's single call to its
// IMAGE$INIT driver procedure) rather than a full port of console_call.c's
// CALL verb: it has no argument-list syntax and isn't yet wired to a DCL
// command of its own. See cpu.Engine.CallEntry's doc comment for why an
// argument list isn't needed here.
func (c *Console) Call(addr uint32, step bool) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if err := c.Engine.CallEntry(addr); err != nil {
		return err
	}

	for {
		err := c.Engine.Step()
		if err == nil {
			if step {
				c.Printf("Stepped to %08X\n", c.CPU.GPR(vax.PC))
			}
			continue
		}
		if errors.Is(err, cpu.ErrConsoleCallReturned) {
			return nil
		}
		if errors.Is(err, cpu.ErrHalted) {
			c.Printf("HALT instruction executed at PC = %08X\n", c.CPU.GPR(vax.PC))
			return nil
		}
		return err
	}
}
