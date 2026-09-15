package console

import (
	"errors"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// Call implements the console CALL command's core mechanism (see
// docs/PHASE-13.md): invokes the procedure at addr with the given arguments
// (pushed right-to-left, matching a real CALLS instruction -- see
// cpu.Engine.CallEntry), then runs Engine.Step in a loop until it returns
// (cpu.CallEntry/ErrConsoleCallReturned) or halts (cpu.ErrHalted). If step
// is true, each instruction executed is traced with its resulting PC,
// matching console_run.c's /STEP qualifier on RUN.
func (c *Console) Call(addr uint32, step bool, args ...uint32) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if err := c.Engine.CallEntry(addr, args...); err != nil {
		return err
	}

	c.Engine.BeginRun()

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

		return c.reportStopReason(err)
	}
}
