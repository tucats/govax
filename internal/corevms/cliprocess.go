package corevms

import (
	"strings"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// Processes that run a command interpreter (docs/PHASE-48.md).
//
// On VMS, an interactive or spawned process doesn't run one image and
// end: it runs a command language interpreter (CLI), normally DCL, which
// reads commands and runs an image for each, one after another, in the
// same process, until it logs out. LIB$SPAWN creates such a process to
// run one command (or the commands it reads from its SYS$INPUT), and
// $CREPRC creates one when the image it's given is LOGINOUT.EXE, the
// image that logs a process in and starts its CLI.
//
// govax's CLI for these processes is a small Go command interpreter, the
// "subprocess CLI" (Decision 8 in docs/PHASE-43.md). It belongs to the
// console, as DCL's symbols and image activation do, so corevms reaches
// it through the CommandInterpreter the console installs on the System.
// Everything else about the process (its creation, its SYS$ names, its
// deletion when the CLI logs out) is the same as for a process created
// to run an image.

// CommandInterpreter is the subprocess CLI, as the System sees it. The
// console provides it (System.Interpreter).
type CommandInterpreter interface {
	// Start sets up the CLI in env, a new process the scheduler has just
	// given the CPU, as cli asks, and returns the address of the
	// procedure that runs it: the process's first call, as an image's
	// IMAGE$INIT driver is for a process that runs an image. The
	// procedure ends with a $EXIT of the CLI's final status when the CLI
	// logs out, which deletes the process.
	Start(env *Environment, cli *CLIStartup) (uint32, error)

	// InheritSymbols gives child, a process LIB$SPAWN is creating,
	// copies of parent's CLI symbols.
	InheritSymbols(parent, child *Environment)
}

// CLIStartup is what a process's command interpreter is to do first.
type CLIStartup struct {
	// Command is the one command the CLI runs before it logs out, as
	// LIB$SPAWN's command-string gives it. Empty: the CLI reads its
	// commands from SYS$INPUT until the end of the input or LOGOUT.
	Command string

	// Prompt is the prompt the CLI writes before reading a command from
	// the terminal; empty for DCL's "$ ".
	Prompt string

	// Login is set for LOGINOUT's CLI, which logs a job in rather than
	// running a spawned command: reading a file or a mailbox, it echoes
	// each command (DCL's verify, on in a job that isn't interactive),
	// and it ends with LOGOUT's report (LogoutReport).
	Login bool
}

// isLoginout reports whether image names LOGINOUT.EXE, the image
// $CREPRC is given to start a process with a CLI (on VMS,
// SYS$SYSTEM:LOGINOUT.EXE). Only the name counts: govax has no
// SYS$SYSTEM to look in.
func isLoginout(image string) bool {
	name := strings.ToUpper(image)

	if i := strings.LastIndexAny(name, ":]>"); i >= 0 {
		name = name[i+1:]
	}

	name, _, _ = strings.Cut(name, ";")
	name = strings.TrimSuffix(name, ".")

	return name == "LOGINOUT" || name == "LOGINOUT.EXE"
}

// startInterpreter is process startup's step for a process that runs a
// CLI (see prepareImage): the process has a CLI from now on (HasCLI),
// and the System's interpreter is started in it. cli nil is LOGINOUT's
// CLI, which reads SYS$INPUT. SS$_UNSUPPORTED if the console installed
// no interpreter.
func (env *Environment) startInterpreter(cli *CLIStartup) (uint32, error) {
	if env.Interpreter == nil {
		return 0, vmserrors.New(ssUnsupported)
	}

	if cli == nil {
		cli = &CLIStartup{Login: true}
	}

	env.HasCLI = true

	// The CLI's SYS$OUTPUT, if it's a file, is made now (outfile.go).
	env.openOutputFile()

	return env.Interpreter.Start(env, cli)
}

// CommandInput is where a CLI reads its commands: the process's
// SYS$INPUT (OpenCommandInput).
type CommandInput interface {
	// ReadLine reads the next command line. ok is false at the end of
	// the input. err is ErrWait while the line hasn't come yet (the
	// terminal's next line, a mailbox's next message): the caller, a
	// shim, returns it, and is called again.
	ReadLine(prompt string) (line string, ok bool, err error)

	// Interactive reports whether the input is the terminal, which is
	// prompted for.
	Interactive() bool
}

// maxCommandLength is the longest command line a CLI reads.
const maxCommandLength = 1024

// OpenCommandInput opens env's SYS$INPUT for its CLI to read commands
// from, as DCL opens SYS$INPUT when a process logs in:
//
//   - the terminal, when SYS$INPUT isn't defined in the process table
//     (the process shares its creator's terminal) or names a terminal;
//   - a mailbox or NL:, read a message at a time, as RMS reads one
//     (recdevice.go); NL: is at its end at once;
//   - otherwise a file, whose records are the lines, read whole when
//     it's opened.
//
// It returns the input and SS$_NORMAL, or nil and the failure's status
// (RMS$_FNF for a file that isn't there, or the device's).
func (env *Environment) OpenCommandInput() (CommandInput, uint32) {
	e, err := env.Logicals.Translate(lnm.ProcessTableName, "SYS$INPUT", lnm.User, 0)
	if err != nil || len(e.Equivalences) == 0 {
		return terminalInput{env}, ssNormal
	}

	if device, st := env.deviceName("SYS$INPUT"); st == 0 {
		if d, found := env.Devices.Find(device); found {
			switch d.DevClass {
			case iodev.DeviceClassTT:
				return terminalInput{env}, ssNormal
			case iodev.DeviceClassMailbox:
				dev, _, st := env.OpenRecordDevice(device, fabFACGet)
				if st != ssNormal {
					return nil, st
				}

				return recordInput{dev.(*recordDevice)}, ssNormal
			}
		}
	}

	name := e.Equivalences[0].Value

	if env.Session == nil {
		return nil, rmsFNF
	}

	loc, err := env.Session.Locate(name, false)
	if err != nil {
		return nil, rmsFNF
	}

	records, _, err := env.Session.ReadRecordFile(loc, rms.TextRecords)
	if err != nil {
		return nil, rmsFNF
	}

	lines := make([]string, len(records))
	for i, r := range records {
		lines[i] = string(r)
	}

	return &fileInput{lines: lines}, ssNormal
}

// terminalInput reads command lines from the terminal (ReadInputLine),
// prompting for each.
type terminalInput struct{ env *Environment }

func (t terminalInput) ReadLine(prompt string) (string, bool, error) {
	return t.env.ReadInputLine(prompt, maxCommandLength)
}

func (terminalInput) Interactive() bool { return true }

// recordInput reads command lines from a mailbox or NL:, a message each.
type recordInput struct{ dev *recordDevice }

func (r recordInput) ReadLine(string) (string, bool, error) {
	data, st, err := r.dev.Get()
	if err != nil {
		return "", false, err
	}

	if st != ssNormal {
		return "", false, nil
	}

	return string(data), true, nil
}

func (recordInput) Interactive() bool { return false }

// fileInput reads command lines from a file's records, read when it was
// opened.
type fileInput struct {
	lines []string
	next  int
}

func (f *fileInput) ReadLine(string) (string, bool, error) {
	if f.next >= len(f.lines) {
		return "", false, nil
	}

	f.next++

	return f.lines[f.next-1], true, nil
}

func (*fileInput) Interactive() bool { return false }

// StartupStatus is the status a command that couldn't activate its image
// ends with, as a process whose image can't be activated is deleted with
// it (startupStatus): RMS$_FNF for an image that isn't there, a system
// or RMS status the error carries, or SS$_ABORT.
func StartupStatus(err error) uint32 { return startupStatus(err) }
