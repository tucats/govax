package console

import (
	"fmt"
	"io"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/cpu"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/rtl"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmserrors"
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

	// Trace matches vax.console.disasm (SET TRACE/NOTRACE, SHOW TRACE):
	// whether EXEC/GO/CALL/RUN disassemble each instruction as they execute
	// it. STEP's own tracing follows a different, per-mode rule instead of
	// this flag — see step.go's StepInto/StepOver/StepReturn doc comments
	// and docs/PHASE-17.md sub-phases 6-7, docs/PHASE-18.md.
	Trace bool

	// StepMode is STEP's default mode (SET STEP, SHOW STEP_MODE) — the Go
	// equivalent of vax.console.stepmode, initialized to StepInto (its zero
	// value) matching initialization.c's own startup default. See step.go.
	StepMode StepMode

	Breakpoints []*Breakpoint

	// InstructionBreakpoints holds every opcode currently flagged to break
	// on execution — SET BREAK/INSTRUCTION, the Go equivalent of vax.c's own
	// instruction[n].debugdata & OP_DBG_BREAK flag. Kept entirely separate
	// from Breakpoints/BreakKind because the C source itself never folds
	// this into its breakpoint_list either — see instbreak.go.
	InstructionBreakpoints map[*cpu.Instruction]bool

	VMInitValid bool // set by VMINIT (vminit.go); cleared by INIT/ZERO

	// Regions holds the P0/P1/S0 region bookkeeping VMInit computes (index
	// 0/1/2, matching vax.h's own struct VMREGION array) — kept around
	// purely so a later SHOW MEMORY can display it, matching that struct's
	// own doc comment ("stored in the virtual machine so a SHOW VM command
	// can display them"). Not to be confused with RTL's own RegionSize
	// (rtl.Environment.RegionSize): that's SYS$EXPREG/image-activation
	// high-water-mark bookkeeping for SHOW REGIONS, an unrelated concept
	// that happens to share a name in the C source (see ShowRegions' own
	// doc comment in show.go).
	Regions [3]vmRegion

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
	// and help text (main.go's own startup sequence), which Console
	// itself knows nothing about — so it's nil until whoever constructs
	// both (see main.go) assigns it explicitly. XFC$CONSOLE_CMD
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

	// assemblerMode is vax.console.assembler_mode (docs/PHASE-19.md): once a
	// bare "ASM" command sets this, Dispatcher.Dispatch hands every
	// subsequent line straight to Console.AssembleInteractiveLine instead of
	// the normal verb table/DCL grammar, matching console_dispatch.c's own
	// check ahead of read_verb. Reset alongside asmSession (see above).
	assemblerMode bool

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

	// Paths is the search-path policy (docs/PHASE-15.md) every file-reading
	// method on Console (Include, Assemble, imageLoad, LoadROM, LoadNVRAM)
	// resolves an unqualified name through: the name as given, then each of
	// Paths' own search directories, then its embedded fallback. A nil
	// Paths (the default for a Console built without main.go's own
	// startup wiring, e.g. most tests) makes every one of those methods
	// behave exactly as a plain os.ReadFile/os.Open — see
	// internal/respath's own doc comment.
	Paths *respath.Resolver
}

// New returns a Console with no machine allocated yet (vax_init == 0 in the
// C source's terms) — an INIT command (see init.go) must run before most
// other commands will accept.
func New(out io.Writer) *Console {
	logicals := iodev.NewLogicalNameTable()
	logicals.InitLogicals()

	return &Console{
		Symbols:  NewSymbolTable(),
		Radix:    16,   // alloc_vax's own default
		Verbose:  true, // initialization.c's own vax.console.flags = CONSOLE_EXPAND | CONSOLE_VERBOSE default
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
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	return nil
}

// requireKernelMode matches the handful of commands (VMINIT, INIT/ROM, ...)
// that additionally refuse to run outside kernel mode.
func (c *Console) requireKernelMode() error {
	if c.CPU.PSL().CurMod() != vax.Kernel {
		return vmserrors.New(vmserrors.CLI_NOTKERNEL)
	}

	return nil
}

// withKernelMode runs fn with the CPU temporarily forced into kernel mode,
// restoring whatever mode was active before -- matching RUN's own identical
// save/force/restore around image loading (run.go's own comment: "must run
// ... in kernel mode regardless of the mode the console happened to be in").
// Console-driven memory access (EXAMINE/DEPOSIT, ASM's own code deposit,
// image loading, SHIM$ stub synthesis) is conceptually the operator's
// console reaching into memory directly, not an access made by whatever
// program the CPU was last running -- on real VAX hardware this is
// privileged/physical console access, independent of the halted program's
// own PSL. Without this, any of those console operations that touch S0
// (kernel-write-only) pages fail with a protection violation the moment a
// program has ever legitimately dropped the CPU to a non-kernel mode and
// then halted there (see kernel.asm's own EXE$INITIALIZE, which does
// exactly that as its normal, designed completion).
func (c *Console) withKernelMode(fn func() error) error {
	saved := c.CPU.PSL().CurMod()
	c.Engine.SetModeStack(vax.Kernel, false)

	defer c.Engine.SetModeStack(saved, false)

	return fn()
}

// Evaluator returns an Evaluator bound to this console's symbol table,
// radix, and current deposit address.
func (c *Console) Evaluator() *Evaluator {
	return &Evaluator{Symbols: c.Symbols, Radix: c.Radix, Here: c.DepositAddr, Mem: c.Mem, CPU: c.CPU}
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
