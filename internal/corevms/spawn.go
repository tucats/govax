package corevms

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// Spawning a subprocess (docs/PHASE-48.md, subtask 2): the process side
// of LIB$SPAWN, which internal/librtl reads the arguments for.
//
// LIB$SPAWN (the RTL Library manual) creates a subprocess of its caller
// that runs the command language interpreter: here the subprocess CLI
// (cliprocess.go). The subprocess is made as $CREPRC makes one
// (CreateProcess), and it gets more of its parent than $CREPRC gives:
//
//   - a name: the one asked for, or by default the parent's user name
//     followed by "_n", n the smallest number no process in the group has
//     yet (spawnName);
//   - its SYS$INPUT, SYS$OUTPUT, and SYS$ERROR: the files asked for, or by
//     default the parent's own (SYS$ERROR follows an output-file);
//   - the parent's base priority and current privileges;
//   - copies of the parent's DCL symbols, unless CLI$M_NOCLISYM;
//   - copies of the parent's process logical names, unless
//     CLI$M_NOLOGNAM: those in user and supervisor mode that aren't
//     CONFINE (DCL's SPAWN help says names created in executive or
//     kernel mode, and CONFINE ones, aren't copied).
//
// The subprocess runs command-string, if there is one, and logs out;
// otherwise it reads its commands from SYS$INPUT until the end of the
// input or LOGOUT. When it ends (DeleteProcess), its final status is
// written to the parent's completion-status-address, the parent's event
// flag is set, and its AST is queued (completeSpawn), each if LIB$SPAWN
// asked. Unless CLI$M_NOWAIT, LIB$SPAWN doesn't return until then: its
// caller waits (LEF) for the subprocess's deletion (AwaitSpawn).
//
// govax tells the parent of the end directly, in Go, rather than through
// a termination mailbox, as VMS's LIB$SPAWN does: the parent can't tell
// the difference, and nothing is left for it to clean up.
//
// Unconfirmed against VMS: the default name's form when the user name is
// long (it's shortened to fit 15 characters); that the event flag is
// cleared when the subprocess is created; the copied logical names'
// modes (the parent's); and that CLI$M_NOTIFY, NOCONTROL, NOKEYPAD,
// TRUSTED, AUTHPRIV, and SUBSYSTEM, the cli argument, and table-name
// change nothing govax does.

// The LIB$SPAWN flags (CLI$M_ in $CLIDEF) govax acts on, and the bits
// LIB$SPAWN accepts at all.
var (
	cliNoWait     = vmsdef.LibrarySymbols["CLI$M_NOWAIT"]
	cliNoCLISym   = vmsdef.LibrarySymbols["CLI$M_NOCLISYM"]
	cliNoLogNam   = vmsdef.LibrarySymbols["CLI$M_NOLOGNAM"]
	validSpawnSet = vmsdef.LibrarySymbols["CLI$M_SUBSYSTEM"]<<1 - 1
)

// Status codes LIB$SPAWN returns, besides $CREPRC's.
var (
	libNoCLI  = vmsdef.LibrarySymbols["LIB$_NOCLI"]
	libInvArg = vmsdef.LibrarySymbols["LIB$_INVARG"]
)

// SpawnRequest is a LIB$SPAWN call's arguments, read from the caller's
// memory.
type SpawnRequest struct {
	// Command is command-string; Input and Output are input-file and
	// output-file; Name is process-name; Prompt is prompt-string. Each is
	// empty when omitted.
	Command, Input, Output, Name, Prompt string

	// Flags is the flags longword (CLI$M_ bits).
	Flags uint32

	// StatusAddress is completion-status-address, 0 for none.
	StatusAddress uint32

	// EventFlag is byte-integer-event-flag-num, if HasEventFlag.
	EventFlag    uint32
	HasEventFlag bool

	// AST is AST-address (0 for none), and ASTParameter its argument.
	AST, ASTParameter uint32
}

// spawnCompletion is what a subprocess LIB$SPAWN created tells its
// parent when it ends (completeSpawn).
type spawnCompletion struct {
	parent *Environment

	statusAddress uint32
	eventFlag     uint32
	hasEventFlag  bool
	ast, astParam uint32

	// astMode is the access mode LIB$SPAWN was called from, which the
	// AST is delivered in.
	astMode uint32
}

// Spawn creates the subprocess req asks for (see this file's opening
// comment), as env's process's LIB$SPAWN. It returns the new process and
// SS$_NORMAL, or nil and the status of the first check that failed:
// SS$_UNSUPPORTED with the scheduler off (docs/PHASE-43.md, Part A, rule
// 2); LIB$_NOCLI if env has no CLI (a process $CREPRC created to run an
// image has none, as on VMS); LIB$_INVARG for a flag LIB$SPAWN doesn't
// define; SS$_IVLOGNAM for a name that's too long; or any of
// CreateProcess's. When it doesn't wait (CLI$M_NOWAIT), the caller goes
// on at once; otherwise AwaitSpawn is the caller's wait.
func (env *Environment) Spawn(req SpawnRequest) (*Environment, uint32) {
	switch {
	case !env.ProcessSettings.Scheduler:
		return nil, ssUnsupported
	case !env.HasCLI:
		return nil, libNoCLI
	case req.Flags&^validSpawnSet != 0:
		return nil, libInvArg
	}

	if req.HasEventFlag {
		if _, _, st := env.flagWord(req.EventFlag); st != 0 {
			return nil, st
		}
	}

	name := req.Name
	if name == "" {
		name = env.spawnName()
	}

	input, output, errorOutput := req.Input, req.Output, req.Output
	if input == "" {
		input = env.processName("SYS$INPUT")
	}

	if output == "" {
		output, errorOutput = env.processName("SYS$OUTPUT"), env.processName("SYS$ERROR")
	}

	p := env.Process

	child, st := env.CreateProcess(CreateRequest{
		Name:          name,
		Input:         input,
		Output:        output,
		Error:         errorOutput,
		BasePriority:  p.BasePriority,
		Privileges:    p.CurrentPrivileges,
		HasPrivileges: true,
	})
	if st != ssNormal {
		return nil, st
	}

	child.Startup.CLI = &CLIStartup{Command: req.Command, Prompt: req.Prompt}

	if req.Flags&cliNoLogNam == 0 {
		env.copyProcessNames(child)
	}

	if req.Flags&cliNoCLISym == 0 && env.Interpreter != nil {
		env.Interpreter.InheritSymbols(env, child)
	}

	child.spawn = &spawnCompletion{
		parent:        env,
		statusAddress: req.StatusAddress,
		eventFlag:     req.EventFlag,
		hasEventFlag:  req.HasEventFlag,
		ast:           req.AST,
		astParam:      req.ASTParameter,
		astMode:       uint32(env.cpu.PSL().CurMod()),
	}

	if req.HasEventFlag {
		word, bit, _ := env.flagWord(req.EventFlag)
		*word &^= 1 << bit
	}

	if req.Flags&cliNoWait == 0 {
		env.spawnWait = child
	}

	if env.cpu.DebugEnabled(vax.DebugProcess) {
		fmt.Fprintf(env.cpu.DebugWriter(), "DEBUG(PROCESS): %08X spawns %08X (%s), command %q\n",
			p.PID, child.Process.PID, name, req.Command)
	}

	return child, ssNormal
}

// AwaitSpawn is LIB$SPAWN's wait for the subprocess it created, unless
// CLI$M_NOWAIT: waiting is false when there's nothing to wait for (this
// call isn't LIB$SPAWN called again during a wait). While the subprocess
// lives, err is ErrWait, the caller waiting (LEF) for its deletion; once
// it's gone, err is nil and the wait is over.
func (env *Environment) AwaitSpawn() (waiting bool, err error) {
	child := env.spawnWait
	if child == nil {
		return false, nil
	}

	if child.Deleted {
		env.spawnWait = nil

		return true, nil
	}

	return true, env.waitOn(sched.StateLEF, sched.ResourceNone, eventFlagBoost, func() bool { return child.Deleted })
}

// completeSpawn tells the parent of env, a subprocess LIB$SPAWN created,
// that env has ended (DeleteProcess): its final status goes to the
// completion-status-address, through the parent's address space, the
// event flag is set, and the AST is queued, each if LIB$SPAWN asked; and
// the parent's wait, if it's waiting, is over. A parent that has itself
// been deleted is told nothing.
func (sys *System) completeSpawn(env *Environment) {
	s := env.spawn
	if s == nil || s.parent.Deleted {
		return
	}

	env.spawn = nil
	parent := s.parent

	if s.statusAddress != 0 && parent.Space != nil {
		_ = sys.mem.StoreLongwordIn(sys.cpu, parent.Space.AddressSpace, s.statusAddress, env.Process.ExitStatus)
	}

	if s.hasEventFlag {
		parent.postFlag(s.eventFlag, eventFlagBoost)
	}

	if s.ast != 0 {
		parent.queueAST(s.ast, s.astParam, s.astMode)
	}

	parent.reportEvent(eventFlagBoost)
}

// spawnName is a spawned subprocess's default name: env's user name and
// "_n", n the smallest positive number no process in env's group has
// with that name. A user name too long for the name to fit 15
// characters is shortened.
func (env *Environment) spawnName() string {
	user := strings.TrimRight(env.Process.Username, " ")
	group := env.Process.UICGroup()

	for n := 1; ; n++ {
		suffix := fmt.Sprintf("_%d", n)
		name := user

		if len(name)+len(suffix) > maxProcessNameLength {
			name = name[:maxProcessNameLength-len(suffix)]
		}

		name += suffix

		if _, taken := env.FindProcessName(group, name); !taken {
			return name
		}
	}
}

// processName returns the equivalence string of name in env's process
// table, or "" when it has none there.
func (env *Environment) processName(name string) string {
	e, err := env.Logicals.Translate(lnm.ProcessTableName, name, lnm.User, 0)
	if err != nil || len(e.Equivalences) == 0 {
		return ""
	}

	return e.Equivalences[0].Value
}

// copyProcessNames copies env's process logical names into child's
// process table, as LIB$SPAWN does (see this file's opening comment): the
// user- and supervisor-mode names, but not CONFINE ones, and not the
// SYS$ names child's startup defines from its own input and output.
func (env *Environment) copyProcessNames(child *Environment) {
	tables, err := env.Logicals.ResolveTables(lnm.ProcessTableName, lnm.User)
	if err != nil || len(tables) == 0 {
		return
	}

	for _, e := range tables[0].Entries() {
		switch {
		case e.IsTable(),
			e.Mode < lnm.Supervisor,
			e.Attrs&lnm.AttrConfine != 0,
			slices.Contains([]string{"SYS$INPUT", "SYS$OUTPUT", "SYS$ERROR"}, e.Name):
			continue
		}

		_, _ = child.Logicals.Define(lnm.ProcessTableName, e.Name, e.Mode, e.Attrs, slices.Clone(e.Equivalences))
	}
}
