package cpu

import (
	"errors"

	"github.com/tucats/govax/internal/vm"
)

// Handler executes one decoded instruction against e. Phases 04-07 register
// one per opcode via Table.SetHandler.
type Handler func(e *Engine, d *Decoded) error

// unimplementedHandler is every Instruction's Handler until a real one is
// registered — the Go equivalent of init_emulators.c's default
// emul_unimplemented, which every table slot starts with before the
// explicitly-implemented ones are overridden. Since Phase 43 (LDPCTX and
// SVPCTX, context.go) only the reserved opcodes are left with it
// (TestEveryInstructionImplemented).
func unimplementedHandler(e *Engine, d *Decoded) error {
	return &Fault{Code: ExcPrivileged}
}

// SetHandler installs h as inst's dispatch Handler.
func (t *Table) SetHandler(inst *Instruction, h Handler) {
	if t.handlers == nil {
		t.handlers = make(map[*Instruction]Handler)
	}

	t.handlers[inst] = h
}

// HandlerFor returns inst's registered Handler, or unimplementedHandler if
// none has been registered.
func (t *Table) HandlerFor(inst *Instruction) Handler {
	if h, ok := t.handlers[inst]; ok {
		return h
	}

	return unimplementedHandler
}

// wrapMemError turns a raw *vm.TranslationFault or *vm.PhysicalAddressError
// — the errors internal/vm's Translate/Load/Store methods return — into the
// matching *Fault, mirroring how decode_opcode.c/decode_operand.c's own
// inlined fast-path memory fetches call set_fault(EXC_ACCVIO, ...) directly
// on a translation failure. Any other error (already a *Fault, or something
// else entirely) is returned unchanged.
func wrapMemError(err error) error {
	if err == nil {
		return nil
	}

	var tf *vm.TranslationFault

	if errors.As(err, &tf) {
		// The fault's parameters are the virtual address and the
		// architecture's reason mask (vm.MaskLength, MaskPTERef,
		// MaskWrite): a read protection violation's is 0, a write's 4, a
		// length violation's 1 or 5. (Before Phase 67 a protection
		// violation reported 2 and the write bit was never set; see
		// docs/DEVIATIONS.md.)
		code := ExcAccessViol
		if tf.Kind == vm.TranslationNotValid {
			code = ExcTranslationNV
		}

		return &Fault{Code: code, Args: []uint32{tf.Addr, uint32(tf.Mask)}}
	}

	var pe *vm.PhysicalAddressError
	if errors.As(err, &pe) {
		return &Fault{Code: ExcAccessViol, Args: []uint32{pe.Addr, 1}}
	}

	return err
}
