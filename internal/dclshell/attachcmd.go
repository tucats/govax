package dclshell

import (
	"encoding/binary"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// The console's ATTACH command (docs/PHASE-57, subtask 9; the DCL
// Dictionary's ATTACH).
//
// ATTACH [process-name] or ATTACH/IDENTIFICATION=pid gives the terminal
// to another process of the job, one that gave it away and waits for it
// to come back: a SPAWN's parent, waiting for its subprocess, or a
// process hibernating after an ATTACH of its own. The process that types
// ATTACH then hibernates in turn, until some process attaches the
// terminal back to it, or the process it gave it to logs out (the
// terminal goes back to that process's parent; corevms's attach.go has
// the rules). When it comes back, DCL says so (%DCL-S-RETURNED), as when
// a SPAWN that waited is over.
//
// In process 1, ATTACH calls LIB$ATTACH in the process, as SPAWN calls
// LIB$SPAWN (spawncmd.go), so process 1 waits in LIB$ATTACH while the
// machine runs the process that has the terminal. In a subprocess, its
// DCL calls the same Go LIB$ATTACH uses (corevms.CLI's Attach) and waits
// for the terminal in the scheduler, as its SPAWN waits.
//
// ATTACH itself shows nothing, and the process the terminal comes back
// to says so (%DCL-S-RETURNED), as VMS V7.1's did (testdata/lib57). Its
// status is SS$_NORMAL once the terminal is back. Choices, each
// unconfirmed: a process name is looked for in the caller's UIC group,
// uppercase, as $GETJPI's prcnam is; and at the Ctrl/Y level in process 1, where the console is inside an image's
// shim, ATTACH ends the interrupted image first (as a command that runs
// an image does; dclcontroly.go).

// ssNonExpr is SS$_NONEXPR, ATTACH's status for a process that isn't
// there; cliRefused CLI$_REFUSED, its status for one the terminal can't
// go to (LIB$ATTACH's LIB$_ATTREQREF), shown as %DCL-E-REFUSED (VMS
// V7.1, for the caller itself and a SPAWN/NOWAIT subprocess:
// testdata/lib57's interactive part).
var (
	ssNonExpr    = vmsdef.Symbols["SS$_NONEXPR"]
	cliRefused   = vmsdef.LibrarySymbols["CLI$_REFUSED"]
	libAttReqRef = vmsdef.LibrarySymbols["LIB$_ATTREQREF"]
)

// Attach is ATTACH (see above): name is the process-name parameter and
// id the /IDENTIFICATION qualifier's value, each "" when not given.
func (c *Interpreter) Attach(name, id string) error {
	if err := c.machine.RequireInit(); err != nil {
		return err
	}

	if c.RTL == nil {
		return vmserrors.New(vmserrors.CLI_NEEDRTL, "ATTACH")
	}

	pid, err := c.attachTarget(name, id)
	if err != nil {
		return err
	}

	var status uint32

	if cli := c.subCLI; cli != nil {
		status = c.RTL.CLI.Attach(pid)

		for status == ssNormal {
			_, err := c.RTL.AwaitAttach()
			if err == nil {
				break
			}

			cli.handBack(cliYield{kind: yieldWait, err: err})
		}
	} else {
		if c.controlYLevelActive != nil {
			return errLeaveControlY
		}

		if status, err = c.callAttach(pid); err != nil {
			return err
		}
	}

	if status == libAttReqRef {
		status = cliRefused
	}

	c.SetCommandStatus(status, nil)

	if status&1 == 0 {
		return c.statusFailure(status)
	}

	c.returnedMessage()

	return nil
}

// attachTarget is the PID of the process ATTACH names: /IDENTIFICATION's
// hexadecimal PID, or the process of that name in the caller's group.
// Both, or neither, is an error of the command's; a name no process has
// is SS$_NONEXPR.
func (c *Interpreter) attachTarget(name, id string) (uint32, error) {
	switch {
	case name != "" && id != "":
		return 0, vmserrors.New(vmserrors.CLI_CONFLICT)
	case id != "":
		pid, err := strconv.ParseUint(strings.TrimSpace(id), 16, 32)
		if err != nil {
			return 0, vmserrors.NewSegment(vmserrors.CLI_IVVALU, id)
		}

		return uint32(pid), nil
	case name == "":
		return 0, vmserrors.New(vmserrors.CLI_INSFPRM)
	}

	env, ok := c.RTL.FindProcessName(c.RTL.Process.UICGroup(), strings.ToUpper(strings.TrimSpace(name)))
	if !ok {
		c.SetCommandStatus(ssNonExpr, nil)

		return 0, c.statusFailure(ssNonExpr)
	}

	return env.Process.PID, nil
}

// callAttach calls LIB$ATTACH for pid in process 1 (see this file's
// opening comment), on SPAWN's procedure page: the procedure, its
// one-argument list, and the PID. It returns LIB$ATTACH's status, once
// the terminal is back.
func (c *Interpreter) callAttach(pid uint32) (uint32, error) {
	if err := c.machine.EnsureShims(); err != nil {
		return 0, err
	}

	entry, ok := c.machine.Symbols.Get("LIB$ATTACH")
	if !ok {
		return 0, vmserrors.New(vmserrors.LIB_UNRESOLVED, "LIB$ATTACH")
	}

	page, err := c.spawnProcedurePage()
	if err != nil {
		return 0, err
	}

	b := make([]byte, spawnStrings)
	copy(b, callProcedureCode(page+spawnArgList, entry))
	binary.LittleEndian.PutUint32(b[spawnArgList:], 1)
	binary.LittleEndian.PutUint32(b[spawnArgList+4:], page+spawnFlagsOffset)
	binary.LittleEndian.PutUint32(b[spawnFlagsOffset:], pid)

	if err := c.machine.StoreBytesIn(nil, page, b); err != nil {
		return 0, err
	}

	if err := c.machine.Call(page, false); err != nil {
		return 0, err
	}

	return c.machine.CPU.GPR(0), nil
}

// callProcedureCode is the procedure SPAWN and ATTACH write in process 1
// to call a LIBRTL routine: an entry mask, CALLG @#argList, @#entry, and
// RET.
func callProcedureCode(argList, entry uint32) []byte {
	code := []byte{0x00, 0x00, 0xFA, 0x9F}
	code = binary.LittleEndian.AppendUint32(code, argList)
	code = append(code, 0x9F)
	code = binary.LittleEndian.AppendUint32(code, entry)

	return append(code, 0x04)
}

// returnedMessage is DCL's word that the terminal is back with the
// process running it, after a SPAWN that waited or an ATTACH.
func (c *Interpreter) returnedMessage() {
	c.Printf("%%DCL-S-RETURNED, control returned to process %s\n", c.RTL.Process.Name)
}
