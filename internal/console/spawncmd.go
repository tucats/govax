package console

import (
	"encoding/binary"
	"fmt"

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
// process other than process 1, runs only while the machine does: at the
// next RUN, GO, CALL, or the like (docs/PHASE-43.md, Decision 4).
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
	spawnFlags       = 64
	spawnDescriptors = 80
	spawnStrings     = 128
	spawnPageBytes   = 2 * 512
	spawnArguments   = 11 // through prompt-string
	maxSpawnText     = 255
)

// Spawn is SPAWN (see above). It reports LIB$SPAWN's failure as an
// error; when a /NOWAIT subprocess has been made, it says so as DCL does
// (%DCL-S-SPAWNED), and when process 1 has waited for one, that control
// has come back (%DCL-S-RETURNED).
func (c *Console) Spawn(o SpawnOptions) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if c.RTL == nil {
		return vmserrors.New(vmserrors.CLI_NEEDRTL, "SPAWN")
	}

	if err := c.ensureShims(); err != nil {
		return err
	}

	entry, ok := c.Symbols.Get("LIB$SPAWN")
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

	var flags uint32

	for _, f := range []struct {
		set  bool
		name string
	}{{o.NoWait, "CLI$M_NOWAIT"}, {o.NoSymbols, "CLI$M_NOCLISYM"}, {o.NoLogicalNames, "CLI$M_NOLOGNAM"}} {
		if f.set {
			flags |= vmsdef.LibrarySymbols[f.name]
		}
	}

	put(spawnFlags, flags)
	args[3] = page + spawnFlags

	put(spawnArgList, spawnArguments)

	for i, a := range args {
		put(spawnArgList+4+uint32(i)*4, a)
	}

	if err := c.images().storeBytes(page, b); err != nil {
		return err
	}

	// The processes there are now, to tell which one LIB$SPAWN made.
	before := map[uint32]bool{}
	for _, env := range c.RTL.Processes() {
		before[env.Process.PID] = true
	}

	if err := c.Call(page, false); err != nil {
		return err
	}

	if status := c.CPU.GPR(0); status&1 == 0 {
		return fmt.Errorf("%s", trimPercent(c.RTL.StatusText(status)))
	}

	if o.NoWait {
		name := ""

		for _, env := range c.RTL.Processes() {
			if !before[env.Process.PID] {
				name = env.Process.Name
			}
		}

		c.Printf("%%DCL-S-SPAWNED, process %s spawned\n", name)

		return nil
	}

	if name := c.RTL.Process.Name; name != "" {
		c.Printf("%%DCL-S-RETURNED, control returned to process %s\n", name)
	}

	return nil
}

// spawnProcedurePage is where SPAWN writes its procedure and LIB$SPAWN's
// arguments: two pages of the S0 pool, the system's own, allocated the
// first time on each machine. (Not CONSOLE$SCRATCH, where RUN's driver
// is: an image stopped mid-run may still return into it.)
func (c *Console) spawnProcedurePage() (uint32, error) {
	if c.spawnPage == 0 {
		page, err := c.RTL.AllocateS0(spawnPageBytes/512, 0, "console SPAWN")
		if err != nil {
			return 0, err
		}

		c.spawnPage = page
	}

	return c.spawnPage, nil
}

// trimPercent drops a message's leading "%", which the console adds back
// when it reports an error.
func trimPercent(s string) string {
	if len(s) > 0 && s[0] == '%' {
		return s[1:]
	}

	return s
}
