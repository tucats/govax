package dclshell

import (
	"encoding/binary"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// The console's SPAWN command (docs/PHASE-48.md, subtask 4).
//
// SPAWN [command] creates a subprocess of process 1 running the
// subprocess CLI (subcli.go), as DCL's SPAWN does, by calling LIB$SPAWN
// in process 1: the console writes a small procedure that calls it, with
// its arguments, and calls the procedure as CALL would. So the
// subprocess is made exactly as a program's LIB$SPAWN makes one, and,
// unless /NOWAIT, process 1 waits for it in LIB$SPAWN while the machine
// runs, the subprocess's commands reading and writing the terminal.
//
// With /NOWAIT the call returns at once, and the subprocess, like any
// process other than process 1, runs only while the machine does: during
// a RUN, GO, CALL, or the like, and while process 1's DCL lends it the
// CPU between commands, during WAIT, and at the prompt (dcllend.go).
//
// In a subprocess, SPAWN makes the new process a subprocess of the
// subprocess, through the same Go that LIB$SPAWN uses
// (corevms.Environment.Spawn), called from the subprocess's DCL. The
// subprocess's DCL then waits for it in the scheduler, as LIB$SPAWN does
// (spawnInSubprocess).
//
// The qualifiers are DCL's: /INPUT and /OUTPUT (the subprocess's
// SYS$INPUT and SYS$OUTPUT), /PROCESS (its name), /PROMPT, /NOSYMBOLS
// and /NOLOGICAL_NAMES (CLI$M_NOCLISYM, CLI$M_NOLOGNAM), and /NOWAIT.

// SpawnOptions are SPAWN's command and qualifiers.
type SpawnOptions struct {
	Command, Input, Output, Process, Prompt string
	NoWait, NoSymbols, NoLogicalNames       bool
}

// The layout of the console's SPAWN procedure page (spawnPage): the
// procedure, LIB$SPAWN's argument list, its flags, the string
// descriptors, and the strings. Process 1 may run in any mode, and only
// kernel mode may write the page, so it holds nothing LIB$SPAWN writes:
// the process-id and completion-status arguments are omitted.
const (
	spawnArgList     = 16
	spawnFlagsOffset = 64
	spawnDescriptors = 80
	spawnStrings     = 128
	spawnPageBytes   = 2 * 512
	spawnArguments   = 11 // through prompt-string
	maxSpawnText     = 255
)

// Spawn is SPAWN (see above). It reports LIB$SPAWN's failure as an
// error; when the subprocess has been made, it says so as DCL does
// (%DCL-S-SPAWNED, and %DCL-S-ATTACHED if it waits), and when process 1
// has waited for one, that control has come back (%DCL-S-RETURNED).
func (c *Interpreter) Spawn(o SpawnOptions) error {
	if err := c.machine.RequireInit(); err != nil {
		return err
	}

	if c.RTL == nil {
		return vmserrors.New(vmserrors.CLI_NEEDRTL, "SPAWN")
	}

	if cli := c.subCLI; cli != nil {
		return c.spawnInSubprocess(cli, o)
	}

	if err := c.machine.EnsureShims(); err != nil {
		return err
	}

	entry, ok := c.machine.Symbols.Get("LIB$SPAWN")
	if !ok {
		return vmserrors.New(vmserrors.LIB_UNRESOLVED, "LIB$SPAWN")
	}

	page, err := c.spawnProcedurePage()
	if err != nil {
		return err
	}

	b := make([]byte, spawnPageBytes)
	put := func(off, v uint32) { binary.LittleEndian.PutUint32(b[off:], v) }

	// The procedure: an entry mask, CALLG @#arglist, @#LIB$SPAWN, RET.
	code := []byte{0x00, 0x00, 0xFA, 0x9F}
	code = binary.LittleEndian.AppendUint32(code, page+spawnArgList)
	code = append(code, 0x9F)
	code = binary.LittleEndian.AppendUint32(code, entry)
	code = append(code, 0x04)
	copy(b, code)

	// The strings, each with a fixed-length descriptor; an empty one is
	// an omitted argument.
	args := make([]uint32, spawnArguments)
	desc, text := uint32(spawnDescriptors), uint32(spawnStrings)

	for _, s := range []struct {
		index int
		value string
	}{{0, o.Command}, {1, o.Input}, {2, o.Output}, {4, o.Process}, {10, o.Prompt}} {
		if s.value == "" {
			continue
		}

		if len(s.value) > maxSpawnText {
			return vmserrors.New(vmserrors.CLI_EXPSYN, s.value)
		}

		binary.LittleEndian.PutUint16(b[desc:], uint16(len(s.value)))
		b[desc+2], b[desc+3] = 14, 1 // DSC$K_DTYPE_T, DSC$K_CLASS_S
		put(desc+4, page+text)
		copy(b[text:], s.value)
		args[s.index] = page + desc
		desc += 8
		text += uint32(len(s.value))
	}

	put(spawnFlagsOffset, spawnFlags(o))
	args[3] = page + spawnFlagsOffset

	put(spawnArgList, spawnArguments)

	for i, a := range args {
		put(spawnArgList+4+uint32(i)*4, a)
	}

	if err := c.machine.StoreBytesIn(nil, page, b); err != nil {
		return err
	}

	// DCL's messages (spawnNotice), and, when SPAWN has waited, that
	// control is back (spawnReturned).
	var child *corevms.Environment

	c.RTL.SpawnNotice = func(spawned *corevms.Environment, wait bool) {
		child = spawned

		c.spawnNotice(spawned, wait)
	}

	defer func() { c.RTL.SpawnNotice = nil }()

	if err := c.machine.Call(page, false); err != nil {
		return err
	}

	// A failure is LIB$SPAWN's status, $STATUS too.
	if status := c.machine.CPU.GPR(0); status&1 == 0 {
		c.SetCommandStatus(status, nil)

		return c.statusFailure(status)
	}

	if o.NoWait || child == nil {
		return nil
	}

	return c.spawnReturned(child)
}

// spawnFlags are the LIB$SPAWN flags SPAWN's qualifiers o ask for.
func spawnFlags(o SpawnOptions) uint32 {
	var flags uint32

	for _, f := range []struct {
		set  bool
		name string
	}{{o.NoWait, "CLI$M_NOWAIT"}, {o.NoSymbols, "CLI$M_NOCLISYM"}, {o.NoLogicalNames, "CLI$M_NOLOGNAM"}} {
		if f.set {
			flags |= vmsdef.LibrarySymbols[f.name]
		}
	}

	return flags
}

// spawnNotice shows DCL's messages when SPAWN has made the subprocess,
// before it runs: that it was, and, if SPAWN waits, that the terminal is
// now the subprocess's. VMS's DCL wrote them so in testdata/mp/probe4's
// run.
func (c *Interpreter) spawnNotice(spawned *corevms.Environment, wait bool) {
	c.Printf("%%DCL-S-SPAWNED, process %s spawned\n", spawned.Process.Name)

	if wait {
		c.Printf("%%DCL-S-ATTACHED, terminal now attached to process %s\n", spawned.Process.Name)
	}
}

// spawnInSubprocess is SPAWN in a subprocess's DCL (cli's): it makes the
// new process with corevms's Spawn, LIB$SPAWN's own Go, in the
// subprocess, and, unless /NOWAIT, waits until it has gone, handing the
// CPU back to the subprocess's shim with LIB$SPAWN's wait
// (corevms.AwaitSpawn: LEF, until the new process is deleted), so the
// machine runs the others meanwhile. A failure to make it is LIB$SPAWN's
// status (unconfirmed: VMS's DCL may word some of them its own way).
func (c *Interpreter) spawnInSubprocess(cli *subprocessCLI, o SpawnOptions) error {
	env := c.RTL

	for _, text := range []string{o.Command, o.Input, o.Output, o.Process, o.Prompt} {
		if len(text) > maxSpawnText {
			return vmserrors.New(vmserrors.CLI_EXPSYN, text)
		}
	}

	env.SpawnNotice = c.spawnNotice
	child, status := env.Spawn(corevms.SpawnRequest{
		Command: o.Command, Input: o.Input, Output: o.Output, Name: o.Process, Prompt: o.Prompt,
		Flags: spawnFlags(o),
	})
	env.SpawnNotice = nil

	if child == nil {
		c.SetCommandStatus(status, nil)

		return c.statusFailure(status)
	}

	if o.NoWait {
		return nil
	}

	for {
		_, err := env.AwaitSpawn()
		if err == nil {
			break
		}

		cli.handBack(cliYield{kind: yieldWait, err: err})
	}

	return c.spawnReturned(child)
}

// spawnReturned ends a SPAWN that waited for child: DCL says control is
// back, and SPAWN's status is the subprocess's final status.
func (c *Interpreter) spawnReturned(child *corevms.Environment) error {
	c.Printf("%%DCL-S-RETURNED, control returned to process %s\n", c.RTL.Process.Name)

	// SPAWN's status is the subprocess's final status, its message shown
	// if it is a failure not shown already (VMS 7.3 showed IVVERB again
	// after SPAWN FROBNICATE, testdata/dcl50's round 5).
	status := child.Process.ExitStatus
	c.SetCommandStatus(status, nil)

	if status&1 != 0 {
		return nil
	}

	// A status of govax's own (a command the subprocess's DCL refused)
	// isn't in the message file: the subprocess has shown its message,
	// with its arguments, which aren't here to show it again.
	if _, ok := vmsdef.LookupMessage(status); !ok {
		return vmserrors.InhibitMessage(c.statusFailure(status))
	}

	return c.statusFailure(status)
}

// spawnProcedurePage is where SPAWN writes its procedure and LIB$SPAWN's
// arguments: two pages of the S0 pool, the system's own, allocated the
// first time on each machine. (Not CONSOLE$SCRATCH, where RUN's driver
// is: an image stopped mid-run may still return into it.)
func (c *Interpreter) spawnProcedurePage() (uint32, error) {
	if c.spawnPage == 0 {
		page, err := c.RTL.AllocateS0(spawnPageBytes/512, 0, "console SPAWN")
		if err != nil {
			return 0, err
		}

		c.spawnPage = page
	}

	return c.spawnPage, nil
}
