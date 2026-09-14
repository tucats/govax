package console

import (
	"fmt"
	"io"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// minPhysMemory/physMemAlign match alloc_vax's own minimum (8K) and
// rounding (512-byte page) rules for the INIT command's requested memory
// size.
const (
	minPhysMemory = 8192
	physMemAlign  = 512
)

// Console is the govax interactive monitor's machine-facing state: the
// instantiated CPU/Memory/Engine (nil until an INIT command creates them,
// matching the C source's vax_init flag), the symbol table, the console's
// own radix/deposit-address/verbosity settings, and breakpoints — the Go
// equivalent of vax.console's fields in reference/eVAX/eVAX/Headers/vax.h,
// minus everything owned by a later phase (device state, RTL/microkernel
// validity, the assembler's own settings).
type Console struct {
	Engine *cpu.Engine
	CPU    *vax.CPU
	Mem    *vm.Memory

	Symbols *SymbolTable

	Radix       int // 8, 10, or 16
	DepositAddr uint32
	Verbose     bool
	Verify      bool

	Breakpoints []*Breakpoint

	// ROM/NVRAM are separate byte buffers outside vm.Memory's main RAM,
	// matching the C source's own rom/nvram globals (reference/eVAX/eVAX/
	// Source/CPU/vm.c) — see internal/vm/memory.go's doc comment: physical
	// resolution beyond RAM belongs to this phase (the file format; see
	// rom.go) and Phase 09 (mapping them into the address space actually
	// translated by internal/vm.Translate, not done here).
	ROM     []byte
	ROMBase uint32
	ROMEnd  uint32

	NVRAM     []byte
	NVRAMBase uint32
	NVRAMEnd  uint32

	VMInitValid bool // set by VMINIT (vminit.go); cleared by INIT/ZERO

	quit bool // set by Quit (misc.go); read via Running

	Out io.Writer
}

// New returns a Console with no machine allocated yet (vax_init == 0 in the
// C source's terms) — an INIT command (see init.go) must run before most
// other commands will accept.
func New(out io.Writer) *Console {
	return &Console{
		Symbols: NewSymbolTable(),
		Radix:   16, // alloc_vax's own default
		Out:     out,
	}
}

// Initialized reports whether an INIT command has allocated a machine yet
// (matching the C source's vax_init flag).
func (c *Console) Initialized() bool { return c.Engine != nil }

// requireInit is the common guard nearly every command runs first, matching
// every console_*.c handler's own "if (!vax_init) return VAX_NOVAX;" check.
func (c *Console) requireInit() error {
	if !c.Initialized() {
		return fmt.Errorf("console: no VAX processor allocated (use INIT first)")
	}
	return nil
}

// requireKernelMode matches the handful of commands (VMINIT, INIT/ROM, ...)
// that additionally refuse to run outside kernel mode.
func (c *Console) requireKernelMode() error {
	if c.CPU.PSL().CurMod() != vax.Kernel {
		return fmt.Errorf("console: not permitted outside kernel mode")
	}
	return nil
}

// Evaluator returns an Evaluator bound to this console's symbol table,
// radix, and current deposit address.
func (c *Console) Evaluator() *Evaluator {
	return &Evaluator{Symbols: c.Symbols, Radix: c.Radix, Here: c.DepositAddr}
}

// Printf writes to the console's output stream, matching the C source's
// direct printf calls.
func (c *Console) Printf(format string, args ...any) {
	fmt.Fprintf(c.Out, format, args...)
}

// allocPhysMemory returns bytes normalized to alloc_vax's own minimum-size
// and 512-byte-alignment rules.
func allocPhysMemory(bytes uint32) uint32 {
	aligned := bytes &^ (physMemAlign - 1)
	if aligned < minPhysMemory {
		aligned = minPhysMemory
	}
	if aligned != bytes {
		aligned += physMemAlign
	}
	return aligned
}
