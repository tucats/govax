package console

import (
	"fmt"
	"io"
	"time"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/librtl"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/rms"
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

// Machine is the emulated system as the operator sees it: the CPU, its
// memory and engine, the console's symbol table and radix, the address
// space VMINIT lays out, the shim stubs, the images loaded in each
// process, and the hooks the debugger sets. Console embeds it, so its
// fields read as the console's (c.CPU, c.Symbols).
type Machine struct {
	// EngineHooks passes the engine's SYS$ services, shims, ASTs, and
	// exceptions to the running process (services.go).
	corevms.EngineHooks

	// Hooks are what the machine asks of the console that runs it.
	Hooks Hooks

	// Debugger is the machine debugger (internal/debugger), installed by
	// cmd/govax; see debugger.go. GO, CALL, STEP, and RUN of a debug
	// image start a session in it. Nil in a console built without it,
	// where the DEBUG command reports that it isn't available.
	Debugger Debugger

	Engine *cpu.Engine
	CPU    *vax.CPU
	Mem    *vm.Memory

	Symbols *SymbolTable

	Radix       int // 8, 10, or 16
	DepositAddr uint32
	Verbose     bool

	// Trace matches vax.console.disasm (SET TRACE/NOTRACE, SHOW TRACE):
	// whether EXEC/GO/CALL/RUN disassemble each instruction as they execute
	// it. STEP's own tracing follows a different, per-mode rule instead of
	// this flag — see step.go's StepInto/StepOver/StepReturn doc comments
	// and docs/PHASE-17.md sub-phases 6-7, docs/PHASE-18.md.
	Trace bool

	VMInitValid bool // set by VMINIT (vminit.go); cleared by INIT/ZERO

	// Regions holds the P0/P1/S0 region bookkeeping VMInit computes (index
	// 0/1/2, matching vax.h's own struct VMREGION array) — kept around
	// purely so a later SHOW MEMORY can display it, matching that struct's
	// own doc comment ("stored in the virtual machine so a SHOW VM command
	// can display them"). Not to be confused with RTL's own RegionSize
	// (corevms.Environment.RegionSize): that's SYS$EXPREG/image-activation
	// high-water-mark bookkeeping for SHOW REGIONS, an unrelated concept
	// that happens to share a name in the C source (see ShowRegions' own
	// doc comment in show.go).
	Regions [3]vmRegion

	// Devices is Phase 09's device table (internal/io) -- separate from
	// the vax_init-gated machine state above, matching the C source's own
	// devices global, which exists independent of alloc_vax and is never
	// reset by ZERO.
	Devices *iodev.DeviceTable

	// Mounts is docs/PHASE-22.md's device-name -> mounted-ODS-2-volume
	// table (internal/rms.MountTable): which VAX device names currently
	// have a real disk-image container mounted on them, populated by the
	// console MOUNT/DISMOUNT commands (internal/dclcmd/device.go). Built
	// once, here, alongside Devices/Logicals rather than by Init/Zero,
	// since a mounted volume — like a defined device or logical name — is
	// operator/session state that has nothing to do with the emulated
	// VAX's own address space, and must survive an INIT/ZERO/VMINIT that
	// wipes that address space (see RTL's own doc comment below for the
	// contrast: RTL is recreated on every one of those, Mounts is not).
	// Injected into each new corevms.Environment the same way Devices/Logicals
	// already are (init.go, vminit.go).
	Mounts *rms.MountTable

	// sourceDirs is the debugger's SET SOURCE search list, and
	// sourceCache the source files read so far, by the file name the
	// debug symbol table gives (source.go).
	sourceDirs  []string
	sourceCache map[string][]string

	// instructionLimit and timeLimit are govax's --instruction-limit and
	// --time-limit options (SetRunLimits, misc.go). They are held here,
	// not set on the Engine at once, because the boot script (vax.init)
	// runs the microkernel's own initialization, which the limits mustn't
	// cut short. They go onto the Engine (cpu.Engine.SetLimits) just
	// before the command given on govax's command line runs
	// (RunCommandLine), and before the interactive prompt starts.
	instructionLimit int
	timeLimit        time.Duration

	// limitStop is the limit error (cpu.ErrInstructionLimitExceeded or
	// ErrTimeLimitExceeded) that most recently stopped a run, recorded by
	// ReportStop (execute.go), which shows the message and otherwise treats
	// the stop as benign. RunCommandLine clears it before the one-shot
	// command and reads it after, so a one-shot run cut short by a limit
	// fails, and govax exits nonzero.
	limitStop error

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
	// "@#lib$put_output" resolving to internal/bootdata/files/kernel.asm's own
	// .ENTRY lib$put_output, once kernel.asm has been ASMed first in the
	// same session) exactly as vax.init's own boot sequence relies on.
	// Reset (nil, so the next ASM lazily creates a fresh one) by VMInit,
	// since its symbol table and deposit pointer would otherwise reference
	// a since-wiped address space.
	asmSession *asm.Assembler

	// assemblerMode is vax.console.assembler_mode (docs/PHASE-19.md): once a
	// bare "ASM" command sets this, Dispatcher.Dispatch hands every
	// subsequent line straight to Console.AssembleInteractiveLine instead of
	// the normal verb table/DCL grammar. Reset alongside asmSession (see above).
	assemblerMode bool

	// kernelPlaced is set once ASM has defined EXE$INITIALIZE, the
	// microkernel's entry; from then on the console's own ASM command
	// refuses, and ASM is the debugger's. Reset with asmSession.
	kernelPlaced bool

	// proc1Images is process 1's image state (images.go): its loaded-image
	// list (Phase 13's ICB list), activation symbols, and driver. Reset by
	// RUN itself (matching reset_icb_list, called at the start of every
	// RUN) rather than by Zero/Init, since a loaded image's memory
	// survives independently of the ICB bookkeeping describing it.
	// otherImages is every other process's (Phase 43), by Environment.
	proc1Images *imageProcess
	otherImages map[*corevms.Environment]*imageProcess

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

	// OnUnhandled, when set, is told of every condition no handler
	// continued, before VMS's catch-all acts on it. Returning true pauses
	// the program there. The debugger sets it (docs/PHASE-42.md, subtask 5).
	OnUnhandled func(UnhandledException) bool

	// OnSignal, when set, is told of every condition as it starts being
	// dispatched to the program's handlers, handled or not. The debugger
	// uses it for SET BREAK/EXCEPTION.
	OnSignal func(UnhandledException)

	// DebugBuiltins, when set, resolves the debugger's built-in symbols in
	// expressions (%CURLOC, %CURVAL, "^", "\\", ...; Evaluator.Builtins).
	// The debugger sets it (docs/PHASE-42.md, subtask 20).
	DebugBuiltins func(name string) (uint32, bool, error)

	// processOne is process 1, the console's own (newRTL), and
	// processOneLogicals its logical names, which outlive it: made once,
	// by Console.New, and shared with process 1's DCL.
	processOne         *corevms.Environment
	processOneLogicals *lnm.Database

	// imageActive is set while an image process 1's RUN started is
	// running (or stopped at a breakpoint): the RET that ends its main
	// routine is its image exit, when user-mode logical names are run
	// down (see imageRundown). imageStatus is the completion status of
	// the last image RUN ran to its end, and imageEnded says one did
	// (run.go). runHost is RUN's /HOST: the main image is a host file
	// (readImage).
	imageActive bool
	imageStatus uint32
	imageEnded  bool
	runHost     bool
}

// Hooks are what the machine asks of the console that runs it, about
// the DCL whose command is running: process 1's, or a subprocess's
// (subcli.go). Console.New sets them all.
type Hooks struct {
	// Output is where the machine's messages go: the DCL's output.
	Output func() io.Writer

	// Session is the DCL's default directory, in which RUN finds a main
	// image (readImage) and the debugger a source file (findSource).
	Session func() *rms.Session

	// ProcessOneOnly is VAX_NOTSUBPROC for what in a subprocess's DCL,
	// and nil in process 1's.
	ProcessOneOnly func(what string) error

	// CommandStatus makes status, an image's completion status, the
	// command's ($STATUS), returning the failure to report, or nil for a
	// success.
	CommandStatus func(status uint32) error

	// ControlYTaken says a Ctrl/Y that stopped the run is a command
	// procedure's to act on (dclcontroly.go), so the stop is quiet.
	ControlYTaken func() bool

	// CLIImageEnded is told when env, a process other than process 1,
	// returns from its outermost call: if that was an image its CLI ran,
	// the CLI goes on (true); otherwise the process has ended.
	CLIImageEnded func(env *corevms.Environment) bool

	// NewProcessOne makes process 1 on a new System (newRTL), with its
	// terminal streams, logical names, and session.
	NewProcessOne func(sys *corevms.System) *corevms.Environment

	// ProcessDeleted is told of each process the System deletes.
	ProcessDeleted func(env *corevms.Environment)

	// Input is the terminal's input, for XFC$CONSOLE_READ.
	Input func() io.Reader

	// ConsoleCommand runs a command line a program gives
	// XFC$CONSOLE_CMD, returning 0 on success and 1 on failure.
	ConsoleCommand func(line string) uint32

	// RequestQuit ends the session, for XFC$QUIT_EMULATION.
	RequestQuit func()

	// HandleAttention is offered an attention key (Ctrl/C, Ctrl/Y) typed
	// while the machine runs (cpu.AttentionHandler), returning true if
	// it has dealt with it. Without one, the key is the running
	// process's (its Ctrl/C or Ctrl/Y AST), or else stops the machine.
	HandleAttention func(key byte) bool
}

// consoleTerminal is the physical device name of process 1's terminal,
// the console, as the process-permanent logical names give it.
const consoleTerminal = "_TTA0:"

// NewMachine returns a Machine with no CPU or memory yet (an INIT must
// make them), whose messages go to out. Process 1's logical names are
// made now, the standard VMS directories and tables for
// corevms.NominalUIC (the same UIC the RTL reports for the process) and
// the process-permanent terminal names, and outlive every INIT. Its
// Hooks are a machine's alone: no DCL, a default directory of its own,
// no terminal input. The console replaces them (Console.New).
func NewMachine(out io.Writer) *Machine {
	logicals := lnm.NewDatabase(corevms.NominalUIC)
	if err := logicals.DefineProcessNames(consoleTerminal); err != nil {
		// Only reachable if the fixed names above were invalid.
		panic(err)
	}

	mounts := rms.NewMountTable()
	session := rms.NewSession(mounts)
	session.Logicals = logicals

	c := &Machine{
		Symbols:            NewSymbolTable(),
		Radix:              16,   // alloc_vax's own default
		Verbose:            true, // CONSOLE_EXPAND | CONSOLE_VERBOSE default
		Devices:            iodev.NewDeviceTable(),
		Mounts:             mounts,
		processOneLogicals: logicals,
	}

	logicals.Trace = c.traceLogicals
	c.EngineHooks = corevms.EngineHooks{Home: func() *corevms.Environment { return c.processOne }}
	c.Hooks = Hooks{
		Output:         func() io.Writer { return out },
		Session:        func() *rms.Session { return session },
		ProcessOneOnly: func(string) error { return nil },
		CommandStatus: func(status uint32) error {
			if status&1 != 0 {
				return nil
			}

			return vmserrors.ConditionError{Status: status, Text: fmt.Sprintf("image exit status %%X%08X", status)}
		},
		ControlYTaken: func() bool { return false },
		CLIImageEnded: func(*corevms.Environment) bool { return false },
		NewProcessOne: func(sys *corevms.System) *corevms.Environment {
			env, err := corevms.NewEnvironment(sys, logicals, nil, out)
			if err != nil {
				panic(err) // the first process on a new System
			}

			env.Session = session

			return env
		},
		Input: func() io.Reader { return nil },
		HandleAttention: func(key byte) bool {
			env := c.running()

			return env != nil && env.AttentionAny(env, key)
		},
	}

	return c
}

// newRTL returns a fresh System for the current CPU and memory, sharing
// the machine's devices and mounts, on the engine's system clock, and the
// RTL environment of its first process, process 1, made by the console
// (Hooks.NewProcessOne). The files the old process 1 left open are
// closed first, so what it wrote reaches the volumes (they outlive the
// machine).
func (c *Machine) newRTL() *corevms.Environment {
	if c.processOne != nil {
		c.processOne.CloseFiles()
	}

	sys := corevms.NewSystem(c.CPU, c.Mem, c.Devices, c.Mounts)
	librtl.Register(sys.Shims()) // LIBRTL.EXE's routines (docs/PHASE-34.md)

	// The old System's other processes are gone with it, and so is their
	// image state; process 1's is the console's and stays (images.go).
	c.otherImages = nil

	sys.SetProcessSettings(processSettings())

	// A process $CREPRC creates activates its image with the console's
	// image activator (docs/PHASE-45.md, subtask 5).
	sys.ActivateImage = c.activateCreatedImage

	// A deleted process's image state goes with it (docs/PHASE-45.md,
	// subtask 6), and so does whatever the console keeps for it.
	sys.ProcessDeleted = func(env *corevms.Environment) {
		delete(c.otherImages, env)

		if c.Hooks.ProcessDeleted != nil {
			c.Hooks.ProcessDeleted(env)
		}
	}

	// $SETIMR's timers run on the engine's system time, the same time
	// base as the interval clock (docs/PHASE-26.md subtask 11).
	if c.Engine != nil {
		sys.Clock = c.Engine.SystemTime
		sys.BootTime = sys.Clock() // $GETSYI's SYI$_BOOTTIME: now, on that clock

		// With vax.process.scheduler on, the System is the engine's
		// scheduling hook (docs/PHASE-44.md); off, the engine has none
		// and never pays for one.
		if sys.ProcessSettings.Scheduler {
			sys.InstallScheduler(c.Engine)
		}
	}

	env := c.Hooks.NewProcessOne(sys)
	c.processOne = env

	// A condition nobody handled goes to the debugger first, if there is
	// one that wants it. The hook reads c.OnUnhandled each time, so a
	// debugger installed later still gets it.
	env.OnSignal = func(u corevms.UnhandledCondition) {
		if c.OnSignal != nil {
			c.OnSignal(UnhandledException{Condition: u.Condition, PC: u.PC, Preceding: u.Preceding})
		}
	}

	env.OnUnhandled = func(u corevms.UnhandledCondition) bool {
		return c.OnUnhandled != nil && c.OnUnhandled(UnhandledException{
			Condition: u.Condition, PC: u.PC, Preceding: u.Preceding,
		})
	}

	return env
}

// Initialized reports whether an INIT command has allocated a machine yet
// (matching the C source's vax_init flag).
func (c *Machine) Initialized() bool { return c.Engine != nil }

// requireInit is the common guard nearly every command runs first.
func (c *Machine) requireInit() error {
	if !c.Initialized() {
		return vmserrors.New(vmserrors.CLI_NOVAX)
	}

	return nil
}

// requireKernelMode matches the handful of commands (VMINIT, INIT/ROM, ...)
// that additionally refuse to run outside kernel mode.
func (c *Machine) requireKernelMode() error {
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
func (c *Machine) withKernelMode(fn func() error) error {
	saved := c.CPU.PSL().CurMod()
	c.Engine.SetModeStack(vax.Kernel, false)

	defer c.Engine.SetModeStack(saved, false)

	return fn()
}

// Evaluator returns an Evaluator bound to this console's symbol table,
// radix, and current deposit address.
func (c *Machine) Evaluator() *Evaluator {
	return &Evaluator{
		Symbols: c.Symbols, Radix: c.Radix, Here: c.DepositAddr, Mem: c.Mem, CPU: c.CPU,
		Debug:    imageNames{c: c},
		Builtins: c.DebugBuiltins,
		Load: func(addr uint32) (uint32, error) {
			v, err := c.loadSized(addr, SizeLongword)
			if err != nil {
				return 0, vmserrors.New(vmserrors.DBG_NOACCESSR, addr)
			}

			return v, nil
		},
		LoadSized: func(addr, size uint32) (uint32, error) {
			sz := SizeLongword

			switch size {
			case 1:
				sz = SizeByte
			case 2:
				sz = SizeWord
			}

			v, err := c.loadSized(addr, sz)
			if err != nil {
				return 0, vmserrors.New(vmserrors.DBG_NOACCESSR, addr)
			}

			return v, nil
		},
	}
}

// Printf writes to the console's output stream, matching the C source's
// direct printf calls.
func (c *Machine) Printf(format string, args ...any) {
	fmt.Fprintf(c.Hooks.Output(), format, args...)
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
