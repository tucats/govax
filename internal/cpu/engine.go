package cpu

import (
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// Engine composes a vax.CPU and vm.Memory with the decode/execute-only state
// the C source keeps on the global vax struct but Phase 01 deliberately left
// out of vax.CPU (registers/PSL only) and Phase 02 left out of vm.Memory
// (owns RAM, not CPU-loop state): the PC of the instruction currently being
// decoded/executed, and whether the machine has halted. See docs/PHASE-03.md's
// design notes on why this is a new type rather than vax.CPU growing these
// fields.
type Engine struct {
	cpu   *vax.CPU
	mem   *vm.Memory
	table *Table

	// instructionPC is the PC at the start of the instruction currently
	// being decoded/executed — the C source's vax.instruction_PC. Faults
	// raised during decode or execution are reported against this address,
	// not wherever decode got to before faulting; see decode.go's doc
	// comment on decodeInstruction.
	instructionPC uint32
	halted        bool
}

// NewEngine returns an Engine driving cpu and mem, using the built-in VAX
// instruction table.
func NewEngine(cpu *vax.CPU, mem *vm.Memory) *Engine {
	return &Engine{cpu: cpu, mem: mem, table: instructionTable}
}

// CPU returns the engine's CPU.
func (e *Engine) CPU() *vax.CPU { return e.cpu }

// Memory returns the engine's memory.
func (e *Engine) Memory() *vm.Memory { return e.mem }

// Halted reports whether the machine has executed a HALT instruction (or
// otherwise been asked to stop; see ErrHalted).
func (e *Engine) Halted() bool { return e.halted }
