package corevms

import (
	"errors"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/registers"
)

// EngineHooks is the process side of the engine's hooks: the parts of
// cpu.SystemServices (SYS$ services and shims), cpu.ASTSource, and
// cpu.ExceptionDispatcher that belong to a process, each passed to the
// process whose context the CPU holds. Whoever implements the rest of
// cpu.SystemServices (the console's I/O and command selectors) embeds an
// EngineHooks to get these.
type EngineHooks struct {
	// Home returns the process that runs when the System has no current
	// process: process 1. Called each time, since the System (and so
	// process 1) is replaced by each INIT and VMINIT. It may return nil
	// before the first, and a nil Home is the same.
	Home func() *Environment
}

// Running is the process the engine's hooks reach: the System's current
// process, the one whose context the CPU holds (docs/PHASE-44.md), or
// Home's when there's none. nil before the first System exists.
func (h EngineHooks) Running() *Environment {
	if h.Home == nil {
		return nil
	}

	home := h.Home()
	if home == nil {
		return nil
	}

	if cur := home.Current(); cur != nil {
		return cur
	}

	return home
}

// SystemService passes XFC$P1VECTOR (a SYS$ service's dispatch) to the
// running process.
func (h EngineHooks) SystemService(pc uint32) (uint32, bool, error) {
	r0, handled, err := h.Running().SystemService(pc)

	return r0, handled, EngineError(err)
}

// Shim passes XFC$SHIM (a LIB$ or C library routine) to the running
// process.
func (h EngineHooks) Shim(code uint32) (uint32, bool, error) {
	r0, handled, err := h.Running().Shim(code)

	return r0, handled, EngineError(err)
}

// NextAST makes EngineHooks a cpu.ASTSource: the engine asks at every
// instruction boundary whether an AST is due, and the running process,
// which owns the AST queues, answers (docs/PHASE-26.md subtask 15).
func (h EngineHooks) NextAST() (cpu.ASTCall, bool, error) {
	routine, argList, returnPC, ok, err := h.Running().NextAST()

	return cpu.ASTCall{Routine: routine, ArgList: argList, ReturnPC: returnPC}, ok, err
}

// DispatchException makes EngineHooks a cpu.ExceptionDispatcher: an
// exception kernel.asm's SCB sends to console$handler is offered to the
// running program's condition handlers before the console reports it
// (docs/PHASE-26.md subtask 31).
func (h EngineHooks) DispatchException(code uint32, params []uint32, pc uint32, psl registers.PSL) (bool, error) {
	env := h.Running()
	if env == nil {
		return false, nil
	}

	return env.DispatchException(code, params, pc, psl)
}

// EngineError turns this package's signals into the engine's: ErrHalt (a
// service or shim asking the machine to halt, e.g. an unrecognized
// SYS$CLI request) into cpu.ErrHalted, ErrWait (a service waiting for an
// event flag, docs/PHASE-26.md) into cpu.ErrServiceWait, ErrExit (the
// image has exited) into cpu.ErrImageExit, and $EXIT's CallRequest (call
// an exit handler) into a cpu.ServiceCall. Any other error is unchanged.
func EngineError(err error) error {
	var call *CallRequest

	switch {
	case errors.Is(err, ErrHalt):
		return cpu.ErrHalted
	case errors.Is(err, ErrWait):
		return cpu.ErrServiceWait
	case errors.Is(err, ErrExit):
		return cpu.ErrImageExit
	case errors.As(err, &call):
		return &cpu.ServiceCall{Routine: call.Routine, ArgList: call.ArgList, Image: call.Image}
	}

	return err
}
