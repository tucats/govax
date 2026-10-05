package console

import (
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// stepDebugger is a minimal stand-in for internal/debugger (which imports
// this package, so these tests can't use the real one). Many tests here
// use RUN/STEP or CALL/STEP only to leave an image stopped at its first
// instruction, to test the console's symbols, disassembly, or SHOW
// commands there. This does that and nothing else: one instruction, with
// STEP's USERSTEP rule, no breakpoints, no messages. The debugger's own
// tests (internal/debugger) cover the stepping itself.
type stepDebugger struct {
	c *Console
}

func (s *stepDebugger) Active() bool { return false }

func (s *stepDebugger) Dispatch(string) error { return vmserrors.New(vmserrors.DBG_NOTAVAILABLE) }

func (s *stepDebugger) Start(a Activation) error {
	c := s.c

	if a.Kind == ActivateCall {
		if err := c.Engine.CallEntry(*a.Addr, a.Args...); err != nil {
			return err
		}

		c.Engine.BeginRun()

		if !a.Step {
			return c.runPlain()
		}
	}

	// RUN under the debugger: run the image's driver up to the main
	// routine's first instruction, as the real debugger does.
	if a.Kind == ActivateImage {
		if err := c.Engine.CallEntry(*a.Addr); err != nil {
			return err
		}

		c.Engine.BeginRun()

		for c.CPU.GPR(vax.PC) != *a.StopAt {
			if err := c.Engine.Step(); err != nil {
				return c.ReportStop(err)
			}
		}

		return nil
	}

	if a.Kind == ActivateGo {
		if a.Addr != nil {
			c.CPU.SetGPR(vax.PC, *a.Addr)
		}

		c.Engine.BeginRun()

		return c.runPlain()
	}

	userStep := c.Engine.CPU().DebugEnabled(vax.DebugUserStep)

	for {
		if err := c.Engine.Step(); err != nil {
			return c.ReportStop(err)
		}

		if userStep && c.Engine.CPU().PSL().CurMod() < 0b11 {
			continue
		}

		return nil
	}
}

// The breakpoint and step-mode operations aren't supported here.
func (s *stepDebugger) AddBreakpoint(uint32)          {}
func (s *stepDebugger) AddTemporaryBreakpoint(uint32) {}
func (s *stepDebugger) ClearBreakpoint(uint32, bool) error {
	return vmserrors.New(vmserrors.DBG_NOTAVAILABLE)
}
func (s *stepDebugger) AddInstructionBreakpoint(string) error {
	return vmserrors.New(vmserrors.DBG_NOTAVAILABLE)
}
func (s *stepDebugger) RemoveInstructionBreakpoint(string) error {
	return vmserrors.New(vmserrors.DBG_NOTAVAILABLE)
}
func (s *stepDebugger) ClearAllInstructionBreakpoints() error {
	return vmserrors.New(vmserrors.DBG_NOTAVAILABLE)
}
func (s *stepDebugger) ShowInstructionBreakpoints() error {
	return vmserrors.New(vmserrors.DBG_NOTAVAILABLE)
}
func (s *stepDebugger) AddFaultBreakpoint(string) error {
	return vmserrors.New(vmserrors.DBG_NOTAVAILABLE)
}
func (s *stepDebugger) RemoveFaultBreakpoint(string) error {
	return vmserrors.New(vmserrors.DBG_NOTAVAILABLE)
}
func (s *stepDebugger) ClearAllFaultBreakpoints() error {
	return vmserrors.New(vmserrors.DBG_NOTAVAILABLE)
}
func (s *stepDebugger) ShowBreakpoints() error { return vmserrors.New(vmserrors.DBG_NOTAVAILABLE) }
func (s *stepDebugger) SetStepMode(string) error {
	return vmserrors.New(vmserrors.DBG_NOTAVAILABLE)
}
func (s *stepDebugger) ShowStepMode() error { return vmserrors.New(vmserrors.DBG_NOTAVAILABLE) }
