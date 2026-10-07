package corevms

import (
	"encoding/binary"
	"fmt"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// The termination message (docs/PHASE-45.md, subtask 7).
//
// A process that creates another with $CREPRC can name a mailbox, by its
// unit number (mbxunt), to be told when the new process is deleted. When
// it is, VMS writes an accounting message to that mailbox, as from the
// deleted process: the creator reads it like any other message, and the
// read's I/O status block has the deleted process's PID in its second
// longword. The System Services Reference Manual ($CREPRC, mbxunt) lays
// it out, and the $ACCDEF names are its fields:
//
//	 0  ACC$W_MSGTYP    MSG$_DELPROC (a word; the next word is unused)
//	 4  ACC$L_FINALSTS  the process's final exit status
//	 8  ACC$L_PID       its PID
//	12  ACC$L_JOBID     unused (the manual and the Internals book say so)
//	16  ACC$Q_TERMTIME  the system time it was deleted at
//	24  ACC$T_ACCOUNT   its account name, blank filled (8 bytes)
//	32  ACC$T_USERNAME  its user name, blank filled (12 bytes)
//	44  ACC$L_CPUTIM    the CPU time it used, in 10-millisecond units
//	48  ACC$L_PAGEFLTS  page faults
//	52  ACC$L_PGFLPEAK  peak paging-file use
//	56  ACC$L_WSPEAK    peak working set size
//	60  ACC$L_BIOCNT    buffered I/O operations
//	64  ACC$L_DIOCNT    direct I/O operations
//	68  ACC$L_VOLUMES   volumes it mounted
//	72  ACC$Q_LOGIN     the system time it was created ("logged in") at
//	80  ACC$L_OWNER     its owner's PID (0 for a detached process)
//
// ACC$K_TERMLEN (84) bytes in all. govax has no paging, keeps no I/O
// counts, and mounts volumes for the whole system, so page faults, the
// peaks, the I/O counts, and the volume count are 0.
//
// VAX/VMS Internals and Data Structures (section 22.2.1, step 11, and
// table 22-1) puts the message's sending after the process's channels
// and quotas have been given back and before it leaves the scheduler,
// which is where DeleteProcess sends it. The manual adds that the
// mailbox is assigned "in the context of the terminating process", and
// that a mailbox that no longer exists, can't be assigned, or is full is
// treated as no mailbox at all: the message is lost, and nothing else
// happens. govax writes it as a write with IO$M_NOW would be written,
// never waiting for room, and also loses it if the mailbox's largest
// message is smaller than 84 bytes (a write's SS$_MBTOOSML).

// accTermLen is ACC$K_TERMLEN, the termination message's size.
var accTermLen = int(vmsdef.Symbols["ACC$K_TERMLEN"])

// msgDelProc is MSG$_DELPROC, the termination message's type.
var msgDelProc = vmsdef.Symbols["MSG$_DELPROC"]

// terminationMessage builds env's termination message (see above), as
// of now: its final status, CPU time, and identity, and the time of its
// deletion.
func (sys *System) terminationMessage(env *Environment) []byte {
	p := env.Process
	b := make([]byte, accTermLen)

	long := func(name string, v uint32) {
		binary.LittleEndian.PutUint32(b[vmsdef.Symbols[name]:], v)
	}

	quad := func(name string, v uint64) {
		binary.LittleEndian.PutUint64(b[vmsdef.Symbols[name]:], v)
	}

	text := func(name string, s string, width int) {
		copy(b[vmsdef.Symbols[name]:], padded(s, width)[:width])
	}

	binary.LittleEndian.PutUint16(b[vmsdef.Symbols["ACC$W_MSGTYP"]:], uint16(msgDelProc))
	long("ACC$L_FINALSTS", p.ExitStatus)
	long("ACC$L_PID", p.PID)
	quad("ACC$Q_TERMTIME", sys.Clock())
	text("ACC$T_ACCOUNT", p.Account, 8)
	text("ACC$T_USERNAME", p.Username, 12)
	long("ACC$L_CPUTIM", uint32(sys.CPUTime(env)/cpuQuotaUnit))
	quad("ACC$Q_LOGIN", p.LoginTime)
	long("ACC$L_OWNER", p.Owner)

	return b
}

// sendTerminationMessage writes env's termination message to the mailbox
// $CREPRC named for it, if it named one and the message can go there
// (see above). The message is from env, so its reader's I/O status
// block gets env's PID.
func (sys *System) sendTerminationMessage(env *Environment) {
	unit := env.Process.TerminationMailbox
	if unit == 0 || sys.Devices == nil {
		return
	}

	status := uint32(ssNoSuchDev)

	if d, found := sys.Devices.Find(fmt.Sprintf("MBA%d", unit)); found {
		if m, ok := sys.Mailboxes.For(d); ok {
			status = ssMbTooSml

			if m.MaxMsg >= uint32(accTermLen) {
				req := &ioRequest{modifiers: ioModNow | ioModNoRSWait}
				msg := &mailboxMessage{data: string(sys.terminationMessage(env)), pid: env.Process.PID}
				done, _ := env.send(m, req, msg)
				status = done.status
			}
		}
	}

	if sys.cpu.DebugEnabled(vax.DebugProcess) {
		fmt.Fprintf(sys.cpu.DebugWriter(), "DEBUG(PROCESS): %08X's termination message to MBA%d, status %08X\n",
			env.Process.PID, unit, status)
	}
}
