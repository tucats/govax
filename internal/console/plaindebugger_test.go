package console

import (
	"strings"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// plainDebugger stands in for internal/debugger in this package's own
// tests, which can't import it (it imports this package). GO, CALL, and
// the console's other internal calls are the debugger's, so a test that
// runs vax.init needs something to be one. It runs each program to its
// end, with no breakpoints, as a bare Console does, and understands just
// "GO [address]" and "EXIT" as commands.
type plainDebugger struct {
	c      *Console
	active bool
}

// installPlainDebugger makes c's debugger a plainDebugger.
func installPlainDebugger(c *Console) *plainDebugger {
	p := &plainDebugger{c: c}
	c.Debugger = p

	return p
}

func (p *plainDebugger) Active() bool { return p.active }

func (p *plainDebugger) Start(a Activation) error {
	c := p.c

	if a.Kind == ActivateAttach {
		p.active = true

		return nil
	}

	if err := c.ReturnToProcessOne(); err != nil {
		return err
	}

	switch a.Kind {
	case ActivateGo:
		if a.Addr != nil {
			c.CPU.SetGPR(vax.PC, *a.Addr)
		}

	case ActivateCall:
		if err := c.Engine.CallEntry(*a.Addr, a.Args...); err != nil {
			return err
		}

	default:
		return vmserrors.New(vmserrors.DBG_NOTAVAILABLE)
	}

	c.Engine.BeginRun()

	return c.runPlain()
}

func (p *plainDebugger) Dispatch(line string) error {
	words := strings.Fields(line)
	if len(words) == 0 {
		return nil
	}

	switch strings.ToUpper(words[0]) {
	case "EXIT":
		p.active = false

		return nil

	case "GO":
		if len(words) == 1 {
			return p.Start(Activation{Kind: ActivateGo})
		}

		addr, err := p.c.EvalWhole(strings.Join(words[1:], " "))
		if err != nil {
			return err
		}

		return p.Start(Activation{Kind: ActivateGo, Addr: &addr})
	}

	return vmserrors.New(vmserrors.CLI_UNRECOGNIZED, "verb", words[0])
}
