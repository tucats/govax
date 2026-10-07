package corevms

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// Process startup (docs/PHASE-45.md, subtask 5).
//
// $CREPRC only builds a process (creprc.go). The rest of getting its
// image running happens in the new process itself, the first time the
// scheduler gives it the CPU, as on VMS, where a new process begins in a
// kernel-mode routine of its own that sets it up and calls the image
// activator. In govax that routine is Go: switchTo calls startProcess
// once the CPU holds the new process's context (its address space,
// stacks, and registers, from the PCB $CREPRC wrote), instead of running
// the process's first instruction. startProcess
//
//  1. defines SYS$INPUT, SYS$OUTPUT, and SYS$ERROR in the process's own
//     process table, executive mode, from $CREPRC's input, output, and
//     error (each only if given);
//  2. has the console activate the image in the process's P0 and write
//     its IMAGE$INIT driver (System.ActivateImage); and
//  3. calls the driver as RUN calls an image's, in user mode on the
//     process's own user stack, on a frame whose return address is
//     cpu.SentinelReturn: when the image returns (or calls $EXIT, which
//     unwinds to that frame), the process's image ends, as Phase 44
//     already handles for any process but process 1.
//
// Its default device and directory were its creator's when it was
// created (Environment.inheritDefaults). If any step fails, for instance
// because the image doesn't exist, the process ends with that failure's
// status, as VMS ends a process whose image can't be activated: the
// failure reaches the creator only through the process's termination
// message (subtask 7).

// Statuses a failed startup ends its process with.
var (
	// rmsFNF is RMS$_FNF, "file not found": the image doesn't exist.
	rmsFNF = vmsdef.Symbols["RMS$_FNF"]

	// ssAbort is SS$_ABORT, for any other failure to activate the image
	// that has no VMS status of its own (a govax-only error).
	ssAbort = vmsdef.Symbols["SS$_ABORT"]
)

// startProcess starts env, a process $CREPRC created, the first time
// the scheduler switches to it (see above). A failure ends the process
// (StopProcess) and isn't an error to the engine: the scheduler sees the
// process has stopped and chooses another (Schedule). An error is only
// for what leaves the CPU in no state to continue.
func (sys *System) startProcess(e *cpu.Engine, env *Environment) error {
	st := env.Startup
	env.Startup = nil

	driver, err := env.prepareImage(st)
	if err == nil {
		// The CPU is in the process's context, in user mode on its user
		// stack (its initial PCB), so this is the image's own call frame.
		err = e.CallEntry(driver)
	}

	if err != nil {
		status := startupStatus(err)

		if sys.cpu.DebugEnabled(vax.DebugProcess) {
			fmt.Fprintf(sys.cpu.DebugWriter(), "DEBUG(PROCESS): %08X can't start %s: %v (status %08X)\n",
				env.Process.PID, st.Image, err, status)
		}

		env.Process.ExitStatus = status
		sys.StopProcess(env)

		return nil
	}

	if sys.cpu.DebugEnabled(vax.DebugProcess) {
		fmt.Fprintf(sys.cpu.DebugWriter(), "DEBUG(PROCESS): %08X starts %s at %08X\n",
			env.Process.PID, st.Image, driver)
	}

	return nil
}

// prepareImage is startProcess's first two steps: env's SYS$ names, then
// its image, activated by the console. It returns the address of the
// procedure that runs the image.
//
// An image of SYS$SYSTEM:LOGINOUT.EXE asks for a command interpreter in
// the new process, which is Phase 48's; until then it fails with
// SS$_UNSUPPORTED. An empty image name is no file at all: RMS$_FNF.
// (What VMS does with $CREPRC's image omitted is unconfirmed.)
func (env *Environment) prepareImage(st *ProcessStartup) (uint32, error) {
	for _, n := range []struct{ name, value string }{
		{"SYS$INPUT", st.Input}, {"SYS$OUTPUT", st.Output}, {"SYS$ERROR", st.Error},
	} {
		if n.value == "" {
			continue
		}

		eqv := []lnm.Equivalence{{Value: n.value}}
		if _, err := env.Logicals.Define(lnm.ProcessTableName, n.name, lnm.Executive, 0, eqv); err != nil {
			return 0, err
		}
	}

	switch {
	case strings.Contains(strings.ToUpper(st.Image), "LOGINOUT"):
		return 0, vmserrors.New(ssUnsupported)
	case st.Image == "":
		return 0, vmserrors.New(rmsFNF)
	case env.ActivateImage == nil:
		return 0, vmserrors.New(ssUnsupported)
	}

	hibernate := env.Process.CreateFlags&prcHIBER != 0

	return env.ActivateImage(env, st.Image, hibernate)
}

// prcHIBER is PRC$M_HIBER: the new process hibernates before its image
// runs, until another process wakes it ($WAKE).
var prcHIBER = vmsdef.Symbols["PRC$M_HIBER"]

// startupStatus is the status a process whose startup failed with err
// ends with: RMS$_FNF for an image that wasn't found; otherwise the
// first VMS status in err's chain from the system (SS$_) or RMS (RMS$_)
// facility; otherwise SS$_ABORT. (The activator's own errors are
// govax's, with no VMS status to give the creator.)
func startupStatus(err error) uint32 {
	for e := err; e != nil; e = errors.Unwrap(e) {
		if ve, ok := e.(vmserrors.VMSError); ok && ve.Status == vmserrors.RMS_IMAGENOTFOUND {
			return rmsFNF
		}
	}

	for e := err; e != nil; e = errors.Unwrap(e) {
		ve, ok := e.(vmserrors.VMSError)
		if !ok {
			continue
		}

		// A VMS status's facility number is in bits 16-27: 0 for the
		// system, 1 for RMS.
		if facility := ve.Status >> 16 & 0xFFF; facility == 0 || facility == 1 {
			return ve.Status
		}
	}

	return ssAbort
}
