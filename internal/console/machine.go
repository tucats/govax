package console

import (
	"fmt"
	"io"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/cpu"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/rtl"
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

	// Devices/Logicals are Phase 09's device-abstraction/logical-name-table
	// state (internal/io) — separate from the vax_init-gated machine state
	// above, matching the C source's own devices/tables globals, which
	// exist independent of alloc_vax and are never reset by ZERO. Logicals
	// is seeded with the default tables at construction (see New), matching
	// init_symbols.c's init_system_symbols calling init_logicals once as
	// part of one-time process startup rather than per-INIT.
	Devices  *iodev.DeviceTable
	Logicals *iodev.LogicalNameTable

	// RTL is Phase 10's SYS$/LIB$ calling-convention environment, backing
	// this Console's cpu.SystemServices implementation (services.go) for
	// the XFC$P1VECTOR/XFC$SHIM selectors. Created fresh alongside the
	// Engine on every Init/Zero (see init.go), since it's addressed
	// through the same CPU/Memory pair.
	RTL *rtl.Environment

	// Dispatcher backs XFC$CONSOLE_CMD (a running VAX program asking the
	// console to execute a command line on its behalf). Unlike Engine/RTL,
	// this isn't created by Init/Zero — a Dispatcher needs the DCL grammar
	// and help text (cmd/govax's own startup sequence), which Console
	// itself knows nothing about — so it's nil until whoever constructs
	// both (see cmd/govax/main.go) assigns it explicitly. XFC$CONSOLE_CMD
	// reports failure if this is still nil.
	Dispatcher *Dispatcher

	quit bool // set by Quit (misc.go); read via Running

	// In/Out are the console's byte-level terminal streams: In backs
	// XFC$CONSOLE_READ and (shared with RTL) DECC$GETS/EXE$INPUT/EXE$READ's
	// fid-0 case; Out backs XFC$CONSOLE_WRITE and every other console
	// output path already using it. In is optional (nil is a legal "no
	// interactive input source" Console, matching every existing Out-only
	// caller/test) — reads against a nil In report EOF rather than panic.
	In  io.Reader
	Out io.Writer

	// shimBase/shimsReady back Phase 13's SHIM$ stub synthesis (shim.go):
	// shimBase is the dedicated S0 page VMInit reserves for these stubs,
	// and shimsReady guards ensureShims so the table (and the SHIM$
	// symbols pointing into it) is only ever built once per VMInit, since
	// a G^ fixup performed on one RUN must still resolve to the same
	// address on a later RUN.
	shimBase   uint32
	shimsReady bool

	// s0Free is the first S0 virtual address past every region VMInit
	// itself reserves -- see vminit.go's own doc comment on why a live ASM
	// session's S0 content must start here rather than at the assembler's
	// literal default origin.
	s0Free uint32

	// asmSession is the ASM command's persistent assembler state (Phase 12,
	// asm.go): matching the reference tool's own vax.assembler being a
	// single session-wide object, multiple "ASM <file>" commands in a row
	// share one location counter and one symbol table, so a later file can
	// reference an earlier one's labels (e.g. testdata/asm/hello.asm's
	// "@#lib$put_output" resolving to testdata/asm/kernel.asm's own
	// .ENTRY lib$put_output, once kernel.asm has been ASMed first in the
	// same session) exactly as vax.init's own boot sequence relies on.
	// Reset (nil, so the next ASM lazily creates a fresh one) by VMInit,
	// since its symbol table and deposit pointer would otherwise reference
	// a since-wiped address space.
	asmSession *asm.Assembler

	// ICBList is Phase 13's loaded-image list (console_run.c's icb_list),
	// in load order (main image first, each dependency appended as loaded
	// -- see image.go's doc comment on why this differs from, but is
	// equivalent to, the C source's own insert-at-front design intent).
	// Reset by RUN itself (matching reset_icb_list, called at the start of
	// every RUN) rather than by Zero/Init, since a loaded image's memory
	// survives independently of the ICB bookkeeping describing it.
	ICBList []*ICB

	// SharePrefix is a directory/filename prefix consulted when locating a
	// sharable image dependency that isn't found under its bare name,
	// matching vax.console.share_prefix (SET SHARE, not yet ported --
	// there is no console command that sets this field yet). A literal
	// ESC (0x1B) prefix means "don't search for sharable images at all",
	// matching find_image's own check; empty (the default) searches
	// alongside the loading image with no prefix.
	SharePrefix string
}

// New returns a Console with no machine allocated yet (vax_init == 0 in the
// C source's terms) — an INIT command (see init.go) must run before most
// other commands will accept.
func New(out io.Writer) *Console {
	logicals := iodev.NewLogicalNameTable()
	logicals.InitLogicals()
	return &Console{
		Symbols:  NewSymbolTable(),
		Radix:    16, // alloc_vax's own default
		Out:      out,
		Devices:  iodev.NewDeviceTable(),
		Logicals: logicals,
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
