package corevms

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmserrors"
)

// Environment is one VAX "process" worth of RTL state: the calling-convention
// plumbing (SYS$/LIB$ registries) plus the state individual services and
// shims need — common event flag clusters, region-size bookkeeping shared
// with Phase 13's image loader and this phase's memory allocator, channels,
// the emulated process record (process.go, which grew out of
// docs/PHASE-10.md's "minimal process stub"), and the RMS file table. It
// is created per-Console (see
// internal/console), not a package-level singleton, matching this project's
// state model (docs/PLAN.md).
type Environment struct {
	mem *vm.Memory
	cpu *vax.CPU

	// OnUnhandled, when set, is told of every condition no handler
	// continued, after the catch-all has shown its message and before it
	// acts (ends the image, or lets the program continue). If it returns
	// true the program is paused there, for a debugger to look at, and
	// carries on with the catch-all's action when it next runs. See
	// condition.go's catchAll.
	OnUnhandled func(UnhandledCondition) bool

	// OnSignal, when set, is told of every condition as its dispatch to
	// the program's handlers begins, whether or not a handler will go on
	// to handle it (the VMS debugger's SET BREAK/EXCEPTION). The dispatch
	// is already set up; the program is at SYS$SRCHANDLER's stub, and
	// carries on into the handler search whenever it next runs. See
	// condition.go's startDispatch.
	OnSignal func(UnhandledCondition)

	shims    *ShimTable
	services *ServiceTable

	// Devices is Phase 09's device table (internal/io) and Logicals the
	// process's logical-name database (internal/lnm, Phase 25), both
	// injected rather than owned here — the console and this Environment
	// both need to see the same tables.
	Devices  *iodev.DeviceTable
	Logicals *lnm.Database

	// Session is the console's rms.Session, whose default directory
	// SYS$CREATE/SYS$OPEN resolve a spec in (docs/PHASE-25.md); nil means
	// the master file directory.
	Session *rms.Session

	// Mounts is docs/PHASE-22.md's device-name -> mounted-ODS-2-volume
	// table (internal/rms.MountTable), injected the same way Devices/
	// Logicals are: it's owned by internal/console's Console (constructed
	// once, alongside Devices/Logicals — a MOUNT command's effect must
	// still be visible after a later VMInit/Zero rebuilds this Environment
	// from scratch), not by this Environment itself.
	Mounts *rms.MountTable

	// files is internal/rms's own "internal file index" table (rms.c's
	// ifi[256]) — unlike Mounts, this really is one-per-process state (a
	// freshly opened file has no business surviving a VMInit/Zero that
	// wipes the address space the FAB/RAB describing it lived in), so
	// NewEnvironment builds a fresh one on every call rather than taking
	// it as a constructor parameter, the same way Phase 10's now-removed
	// ifiFiles/nextIFI fields used to be constructed fresh each time.
	files *rms.FileTable

	// RegionSize holds the P0/P1/S0 region high-water marks (index 0/1/2),
	// the Go-native replacement for get_region_size/set_region_size's
	// indirection through a VAX memory cell addressed by an EXE$P0_RGN-
	// style symbol (see docs/PHASE-10.md's open questions) — this phase's
	// allocator (memory.go) and Phase 13's image loader both read/write it
	// directly.
	RegionSize [3]uint32

	// EventFlagClusters is the system-wide table of common event flag
	// clusters $ASCEFC creates and associates (eventflags.go). A process's
	// own event flags and associations are in Process.
	EventFlagClusters *CommonEventFlags

	channels    []*channel
	nextChannel uint32

	// Process is the emulated VMS process this Environment runs images in:
	// its PID, username, UIC and quota state (process.go,
	// docs/PHASE-26.md). Built fresh by NewEnvironment.
	Process *Process

	// consoleIn/consoleInBuf back DECC$GETS/EXE$INPUT/EXE$READ's console
	// line reading (input.go). consoleOut is also where non-RMS console
	// writes go (print.go, file.go); RMS's own "internal file index" table
	// (rms.c's ifi[256]) now lives in internal/rms (docs/PHASE-22.md), not
	// here — Phase 10's stopgap version of that table was removed along
	// with the rest of internal/rtl/rms.go.
	consoleIn    io.Reader
	consoleInBuf *bufio.Reader
	consoleOut   io.Writer

	// memAllocated/memFreed back the LIB$GET_VM/malloc allocator (memory.go).
	memAllocated, memFreed []*memBlock

	// openFiles/nextFID back exe_open/close/read/write's raw file-descriptor
	// shims (file.go) — a synthetic descriptor space (starting past the
	// conventional 0/1/2 stdin/stdout/stderr numbers, matching a real
	// process's own next-available-fd convention) rather than real host
	// file descriptors, since Go doesn't expose files as bare ints.
	openFiles map[uint32]*os.File
	nextFID   uint32

	// Clock returns the current system time in VMS format (100ns units
	// since 17-Nov-1858), what $SETIMR's timers run on. NewEnvironment
	// sets it to the host clock; the console replaces it with its
	// Engine's SystemTime, the time base the interval clock also uses
	// (timers.go).
	Clock func() uint64

	// BootTime is when the system "booted", in VMS time: $GETSYI's
	// SYI$_BOOTTIME. NewEnvironment sets it to Clock's time; the console
	// resets it after rebinding Clock, so it's the time of the INIT,
	// VMINIT, or ZERO that built this Environment.
	BootTime uint64

	// NodeName is the system's node name ($GETSYI's SYI$_NODENAME).
	NodeName string

	// CommandLine is the text of the command that ran the current image,
	// after its verb: what LIB$GET_FOREIGN returns. A foreign command
	// sets it (internal/console's RunOptions.CommandLine); RUN leaves it
	// empty, as RUN takes no parameters.
	CommandLine string

	// timers is the process's $SETIMR timer queue (timers.go).
	timers []*timerRequest

	// attentionASTs are the CTRL/C and CTRL/Y AST requests enabled on
	// the process's terminal channels (ctrlast.go).
	attentionASTs []attentionAST

	// pendingIO are the $QIO requests a driver has kept to complete
	// later, and qiowWaits the $QIOWs waiting for theirs (qio.go).
	pendingIO []*ioRequest
	qiowWaits []qiowWait

	// Operator is OPCOM's state: the console's operator classes and the
	// outstanding operator requests (operator.go). System state, like
	// Mailboxes.
	Operator *operatorState

	// Mailboxes are the mailboxes $CREMBX has created (mailbox.go). Like
	// common event flag clusters they're system state, in system memory
	// on VMS, so INIT/VMINIT/ZERO start with none.
	Mailboxes *MailboxTable

	// waitingPC is the P1-vector address of a service currently waiting
	// (ErrWait), so SystemService traces only its first attempt; 0 when
	// no service is waiting.
	waitingPC uint32
}

// NewEnvironment returns an Environment for one VAX process, driving mem/cpu
// and sharing devices/logicals/mounts with whatever else (the console) also
// uses them. consoleOut is where non-RMS console writes (print.go, file.go)
// and the internal/rms package's own TTA0: special case go — typically the
// same io.Writer as Console.Out; consoleIn is where DECC$GETS/EXE$INPUT read
// from — typically the console's own input stream. mounts is the shared
// MountTable a MOUNT command populates (internal/console) — see the Mounts
// field's own doc comment for why it's injected rather than owned here.
func NewEnvironment(cpu *vax.CPU, mem *vm.Memory, devices *iodev.DeviceTable, logicals *lnm.Database, mounts *rms.MountTable, consoleIn io.Reader, consoleOut io.Writer) *Environment {
	env := &Environment{
		mem:      mem,
		cpu:      cpu,
		shims:    NewShimTable(),
		services: NewServiceTable(),
		Devices:  devices,
		Logicals: logicals,
		Mounts:   mounts,
		files:    rms.NewFileTable(consoleOut),
		Process:  NewProcess(),

		EventFlagClusters: NewCommonEventFlags(),
		Mailboxes:         NewMailboxTable(),
		Operator:          newOperatorState(),
		Clock:             wallClock,
		consoleIn:         consoleIn,
		consoleOut:        consoleOut,
		openFiles:         map[uint32]*os.File{},
		nextFID:           3,
	}
	env.BootTime = env.Clock()
	env.removeStaleMailboxes()
	env.NodeName = nominalNodeName

	registerShims(env.shims)
	registerServices(env.services)

	return env
}

// rmsContext bundles this Environment's memory/CPU/mount-table/file-table/
// logical-name-table/console-output into the self-contained *rms.Context
// every internal/rms handler function expects (see internal/rms/context.go's
// own doc comment on why that package can't just take *Environment
// directly: internal/rtl imports internal/rms, so the reverse import would
// make the two packages depend on each other, which Go refuses to build).
// Called fresh by each rms.go wrapper closure rather than cached in the
// struct, since it's cheap to build and this keeps Environment itself free
// of a field whose only reader is this one method.
func (env *Environment) rmsContext() *rms.Context {
	return &rms.Context{
		Mem:       env.mem,
		CPU:       env.cpu,
		Mounts:    env.Mounts,
		Files:     env.files,
		Logicals:  env.Logicals,
		Session:   env.Session,
		Console:   env.consoleOut,
		ConsoleIn: env.consoleReader(),
		NodeName:  env.NodeName,
	}
}

// readArgs reads a VAX argument list off ap: a leading argument-count
// longword, then that many longword arguments — the calling convention
// CALLS/CALLG build and shim()/call_service() both read verbatim (their own
// argc/argv loop over vax.AP).
func readArgs(cpu *vax.CPU, mem *vm.Memory, ap uint32) ([]uint32, error) {
	argc, err := mem.LoadLongword(cpu, ap)
	if err != nil {
		return nil, err
	}

	argv := make([]uint32, argc)

	for n := uint32(0); n < argc; n++ {
		v, err := mem.LoadLongword(cpu, ap+(n+1)*4)
		if err != nil {
			return nil, err
		}

		argv[n] = v
	}

	return argv, nil
}

// callHandler invokes fn(env, argv), recovering a panic into an error.
// Real VAX system services/shims never bounds-check argc against what a
// handler expects to index (the C source has no equivalent concept — an
// under-supplied argument list just reads whatever garbage was on the
// native argv array), so every handler here indexes argv directly the same
// way. In Go that turns "garbage in" into a slice-bounds panic instead, and
// a bug in one handler (or a genuinely malformed calling program supplying
// too few arguments) shouldn't be able to crash the whole process — this is
// the one safety net at the dispatch boundary that keeps every handler free
// to index its own expected arguments without its own recover().
func callHandler(fn func(*Environment, []uint32) (uint32, error), env *Environment, argv []uint32) (r0 uint32, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = vmserrors.New(vmserrors.LIB_PANIC, fmt.Sprintf("%v", p))
		}
	}()

	return fn(env, argv)
}

// Shim implements cpu.SystemServices' RTL half (XFC$SHIM): dispatch a LIB$/
// CRTL shim call by numeric code, with its argument list read from AP —
// matching shim()'s own argc/argv-from-vax.AP marshaling.
func (env *Environment) Shim(code uint32) (uint32, bool, error) {
	fn, ok := env.shims.Lookup(code)
	if !ok {
		return 0, false, nil
	}

	argv, err := readArgs(env.cpu, env.mem, env.cpu.GPR(vax.AP))
	if err != nil {
		return 0, true, err
	}

	r0, err := callHandler(fn, env, argv)

	return r0, true, err
}

// HasShim reports whether code has a registered ShimFunc, without
// invoking it — for a diagnostic caller (SHOW SHIM, internal/console) that
// wants to know whether a numeric dispatch code is live, not run it.
func (env *Environment) HasShim(code uint32) bool {
	_, ok := env.shims.Lookup(code)

	return ok
}

// SystemService implements cpu.SystemServices' RTL half (XFC$P1VECTOR):
// dispatch a SYS$ system-service call whose calling instruction is at pc,
// with its argument list read from AP — matching call_service's own
// pc-to-name lookup (p1vector.go) and argc/argv-from-vax.AP marshaling.
func (env *Environment) SystemService(pc uint32) (uint32, bool, error) {
	entry, ok := lookupP1Vector(pc)
	if !ok {
		return 0, false, nil
	}

	fn, ok := env.services.Lookup(entry.Name)
	if !ok {
		// Matches call_service's own "Unimplemented native service"
		// path: a real, known P1-vector address with no Go handler
		// registered yet is reported the same as an address that
		// doesn't exist at all — from a calling program's perspective
		// both mean "this service isn't implemented."
		return 0, false, nil
	}

	// A service reached by something other than CALLS/CALLG (the AST exit,
	// SYS$CLRAST) has no argument list: AP is whatever the interrupted
	// code had.
	var (
		argv []uint32
		err  error
	)

	if env.services.ReadsArgs(entry.Name) {
		if argv, err = readArgs(env.cpu, env.mem, env.cpu.GPR(vax.AP)); err != nil {
			return 0, true, err
		}
	}

	r0, err := callHandler(fn, env, argv)

	// A waiting service is called again on every instruction step until
	// its wait is satisfied (ErrWait); only its first attempt is traced.
	waiting := errors.Is(err, ErrWait)

	var call *CallRequest

	retry := waiting && env.waitingPC == pc

	if waiting {
		env.waitingPC = pc
	} else {
		env.waitingPC = 0
	}

	// DBG_SERVICES: matches p1_vector.c:433's own "Debug P1 system service
	// calls?" trace -- this port's table-driven service dispatch (see
	// docs/PHASE-17.md sub-phase 4) gives every SYS$ call a single choke
	// point, unlike the C source's switch-based call_service.
	if env.cpu.DebugEnabled(vax.DebugServices) && !retry {
		args := make([]string, len(argv))
		for i, a := range argv {
			args[i] = fmt.Sprintf("%08X", a)
		}

		result := fmt.Sprintf("returns %08X", r0)

		switch {
		case waiting:
			result = "waits"
		case errors.As(err, &call):
			result = fmt.Sprintf("calls %08X", call.Routine)
		case errors.Is(err, ErrExit):
			result = fmt.Sprintf("exits with status %08X", r0)
		}

		fmt.Fprintf(env.cpu.DebugWriter(), "DEBUG(SERVICES): %s( %s ), %s\n",
			entry.Name, strings.Join(args, ", "), result)
	}

	return r0, true, err
}
