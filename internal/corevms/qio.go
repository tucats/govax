package corevms

import (
	"fmt"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// $QIO, $QIOW, and $CANCEL (docs/PHASE-26.md subtask 17): device I/O
// through a channel.
//
// # How VMS device I/O works
//
// A VMS program doesn't read or write a device directly. It first asks
// $ASSIGN for a *channel* to the device (devices.go), a small number that
// stands for "my connection to TTA0:". Then it hands $QIO ("queue I/O
// request") a request for that channel:
//
//	SYS$QIO [efn] ,chan ,func [,iosb] [,astadr] [,astprm]
//	        [,p1] [,p2] [,p3] [,p4] [,p5] [,p6]
//
//   - func says what to do. Its low 6 bits are the *function code*
//     (IO$_READVBLK "read", IO$_WRITEVBLK "write", ...), and the bits
//     above are *function modifiers* that adjust it (IO$M_NOECHO "don't
//     echo what's typed", ...). Each device class has its own functions.
//   - p1-p6 are the function's parameters, their meaning depending on the
//     function: for a read, p1 is the buffer's address and p2 its size.
//   - iosb is the address of an *I/O status block*, a quadword the
//     request's outcome is written to when it finishes: a 16-bit
//     condition value, a 16-bit transfer count, and 32 bits of
//     device-specific information.
//   - efn is an event flag set when the request finishes, and astadr an
//     optional AST routine (ast.go) called then, with astprm.
//
// $QIO's own return value (R0) only says whether the request was
// *accepted*. How the I/O itself went is in the IOSB. So a read that hits
// end of file returns SS$_NORMAL in R0 and SS$_ENDOFFILE in the IOSB. A
// request rejected outright (a bad channel, an unreadable buffer, a
// function the device doesn't have) returns the error in R0, sets the
// event flag, and writes nothing to the IOSB and queues no AST.
//
// $QIO returns as soon as the request is queued; the program then waits
// for the event flag, or $SYNCH, or lets the AST tell it. $QIOW ("and
// wait") is $QIO followed by that wait.
//
// # How govax does it
//
// Most requests complete *during* the $QIO call: the IOSB is written,
// the event flag set, and the AST queued before $QIO returns — the same
// shortcut $GETJPI takes. VMS allows this (a request may finish before
// $QIO returns), and a correctly written program can't tell the
// difference. Every terminal request does.
//
// A request can also stay *pending*: a mailbox read with no message to
// read waits for a write (mbxdriver.go, docs/PHASE-26.md subtask 29). The
// driver keeps the request and completes it later with completeIO, which
// does what completion always does: IOSB, event flag, AST. $QIOW then
// waits for that, $CANCEL completes a channel's pending requests with
// SS$_CANCEL, and so does $DASSGN.
//
// Which functions a device has is table-driven: ioDrivers maps a device
// class to its driver's function table, which maps a function code to
// the Go function that performs it. Terminals (ttdriver.go), mailboxes
// (mbxdriver.go), and disks (diskdriver.go) have drivers.

// Status codes the I/O services return.
var (
	ssIllIoFunc = vmsdef.Symbols["SS$_ILLIOFUNC"]
	ssTimeout   = vmsdef.Symbols["SS$_TIMEOUT"]
	ssEndOfFile = vmsdef.Symbols["SS$_ENDOFFILE"]
	ssCancel    = vmsdef.Symbols["SS$_CANCEL"]
)

// ioFunctionCodeMask selects a func argument's function code (IO$M_FCODE,
// its low 6 bits). Everything above it, in the low word, is modifiers.
var ioFunctionCodeMask = vmsdef.Symbols["IO$M_FCODE"]

// ioCode returns the $IODEF value called name, panicking if there isn't
// one: the function tables are built from names when the package loads,
// so a misspelling is caught by any test.
func ioCode(name string) uint32 {
	v, ok := vmsdef.Symbols[name]
	if !ok {
		panic("rtl: no $IODEF symbol " + name)
	}

	return v
}

// ioRequest is one $QIO request, as a driver function sees it.
type ioRequest struct {
	channel   *channel
	function  uint32    // the function code (IO$_...), without modifiers
	modifiers uint32    // the IO$M_ modifier bits (func's bits 6-15)
	p         [6]uint32 // p1-p6

	// How the request completes: the event flag to set, the IOSB to
	// write (0 for none), the AST to queue (astadr 0 for none) and the
	// access mode it runs in (the caller's).
	efn, iosb, astadr, astprm, mode uint32

	// done is set once the request has completed (or been cancelled);
	// cancelled says it was cancelled. A driver holding a pending
	// request drops it once it's done.
	done, cancelled bool
}

// modified reports whether the request has every modifier bit in m.
func (r *ioRequest) modified(m uint32) bool { return r.modifiers&m == m }

// ioStatus is what a completed request writes to its I/O status block:
// the condition value (low word of the first longword), the transfer
// count (high word), and the device-specific second longword.
type ioStatus struct {
	status uint32
	count  uint16
	info   uint32
}

// ioFunc performs one I/O function. It returns the request's completion
// status, or, if reject isn't 0, rejects the request instead: reject is
// then $QIO's R0 (typically SS$_ACCVIO for a buffer the caller can't
// access), and the request doesn't complete. reject ioPending means the
// driver has kept the request, to complete later with completeIO.
type ioFunc func(env *Environment, req *ioRequest) (done ioStatus, reject uint32)

// ioPending is an ioFunc's "not done yet" (no VMS status is all ones).
// ioResourceWait is its "can't start yet": the caller must wait for a
// resource (room in a full mailbox) and then make the request again. The
// request hasn't been queued; $QIO and $QIOW return ErrWait, so their
// XFC runs again, the process in a resource wait meanwhile.
const (
	ioPending      = ^uint32(0)
	ioResourceWait = ^uint32(1)
)

// ioDrivers is the driver registry: for each device class, the function
// codes its driver implements. A device of a class that isn't here, or a
// function its driver doesn't list, is SS$_ILLIOFUNC.
var ioDrivers = map[iodev.DeviceClass]map[uint32]ioFunc{
	iodev.DeviceClassTT:      terminalFunctions,
	iodev.DeviceClassMailbox: mailboxFunctions,
	iodev.DeviceClassDisk:    diskFunctions,
}

// serviceSysQio is SYS$QIO:
//
//	SYS$QIO [efn] ,chan ,func [,iosb] [,astadr] [,astprm]
//	        [,p1] [,p2] [,p3] [,p4] [,p5] [,p6]
//
// See queueIO; $QIO returns as soon as the request is queued (or
// rejected), whether or not it has completed.
func serviceSysQio(env *Environment, argv []uint32) (uint32, error) {
	status, _ := env.queueIO(argv)
	if status == ioResourceWait {
		return 0, ErrWait
	}

	return status, nil
}

// qiowWait is a $QIOW waiting for its request: fp is the SYS$QIOW stub's
// frame pointer, which is the same each time its XFC runs again.
type qiowWait struct {
	fp  uint32
	req *ioRequest
}

// serviceSysQiow is SYS$QIOW, $QIO and wait: the same arguments, but it
// returns only when the request has completed. A request that completes
// during the call (every terminal request) returns at once. One left
// pending waits, re-executing the service's XFC (ErrWait) until the
// driver completes it; ASTs and interrupts are delivered meanwhile. The
// waits are a stack (Environment.qiowWaits), since an AST routine may
// issue a $QIOW of its own while one is waiting.
//
// Like $QIO, it returns SS$_NORMAL once the request is queued, whatever
// the I/O's own status (which is in the IOSB).
func serviceSysQiow(env *Environment, argv []uint32) (uint32, error) {
	fp := env.cpu.GPR(vax.FP)

	if n := len(env.qiowWaits); n > 0 && env.qiowWaits[n-1].fp == fp {
		if req := env.qiowWaits[n-1].req; !req.done {
			return 0, env.waitOnIO(req)
		}

		env.qiowWaits = env.qiowWaits[:n-1]

		return ssNormal, nil
	}

	status, pending := env.queueIO(argv)

	switch {
	case status == ioResourceWait:
		return 0, ErrWait
	case pending == nil:
		return status, nil
	}

	env.qiowWaits = append(env.qiowWaits, qiowWait{fp: fp, req: pending})

	return 0, env.waitOnIO(pending)
}

// waitOnIO is a $QIOW's wait for its request: in the LEF state (VMS's
// $QIOW waits on the request's event flag), until the driver completes
// it (waits.go).
func (env *Environment) waitOnIO(req *ioRequest) error {
	return env.waitOn(sched.StateLEF, sched.ResourceNone, eventFlagBoost, func() bool { return req.done })
}

// queueIO is the body of $QIO and $QIOW. It returns $QIO's status, and
// the request if it's pending (nil otherwise). The steps are the ones
// VMS's $QIO takes:
//
//  1. Clear event flag efn (default 0): SS$_ILLEFC or SS$_UNASEFC if it
//     isn't one the process can use.
//  2. Check the channel (only chan's low word counts): SS$_IVCHAN for 0,
//     SS$_NOPRIV if it isn't assigned or was assigned from a more
//     privileged access mode than the caller's.
//  3. Clear the IOSB, if given: SS$_ACCVIO if it can't be written.
//  4. Find the function (func's low 6 bits) in the device's driver:
//     SS$_ILLIOFUNC if it has none.
//  5. Perform it. The driver may still reject the request (SS$_ACCVIO
//     for a buffer the caller can't access), or keep it pending.
//
// A failure at steps 2-5 sets the event flag, as the manual says ("the
// specified event flag is set if the service terminates without queuing
// an I/O request"). Otherwise the status is SS$_NORMAL, whatever the
// I/O's own, and the request completes (completeIO) now or, if pending,
// when the driver says.
//
// Not implemented: the BIOLM, DIOLM, BYTLM, and ASTLM quotas
// (SS$_EXQUOTA); SS$_INSFMEM; network links; SS$_DEVOFFLINE.
func (env *Environment) queueIO(argv []uint32) (uint32, *ioRequest) {
	efn, number, function := optArg(argv, 0), optArg(argv, 1)&0xFFFF, optArg(argv, 2)&0xFFFF
	iosb, astadr, astprm := optArg(argv, 3), optArg(argv, 4), optArg(argv, 5)

	// Step 1: the event flag.
	flags, bit, st := env.eventFlagWord(efn)
	if st != 0 {
		return st, nil
	}

	*flags &^= 1 << bit

	// reject ends a request that won't complete: the flag is set, and st
	// is the service's status.
	reject := func(st uint32) (uint32, *ioRequest) {
		*flags |= 1 << bit

		return st, nil
	}

	// Step 2: the channel.
	if number == 0 {
		return reject(ssIvChan)
	}

	c, found := env.findChannel(number)
	if !found || c.Mode < uint32(env.cpu.PSL().CurMod()) {
		return reject(ssNoPriv)
	}

	// Step 3: the IOSB.
	if iosb != 0 && !env.storeQuad(iosb, 0) {
		return reject(ssAccVio)
	}

	// Step 4: the function.
	req := &ioRequest{
		channel:   c,
		function:  function & ioFunctionCodeMask,
		modifiers: function &^ ioFunctionCodeMask,
		efn:       efn,
		iosb:      iosb,
		astadr:    astadr,
		astprm:    astprm,
		mode:      uint32(env.cpu.PSL().CurMod()),
	}

	for i := range req.p {
		req.p[i] = optArg(argv, 6+i)
	}

	if env.cpu.DebugEnabled(vax.DebugDevices) {
		fmt.Fprintf(env.cpu.DebugWriter(), "DEBUG: SYS$QIO channel %d (%s) function %02X modifiers %04X\n",
			number, c.Device.Name, req.function, req.modifiers)
	}

	perform, ok := ioDrivers[c.Device.DevClass][req.function]
	if !ok {
		return reject(ssIllIoFunc)
	}

	// Step 5: perform it.
	done, rejected := perform(env, req)

	switch rejected {
	case 0:
		env.completeIO(req, done)

		return ssNormal, nil

	case ioPending:
		env.pendingIO = append(env.pendingIO, req)

		return ssNormal, req

	case ioResourceWait:
		return ioResourceWait, nil
	}

	return reject(rejected)
}

// completeIO completes req with status done: its IOSB is written, its
// event flag set, and its AST queued, in the mode it was requested from.
// A request already completed is left alone. The IOSB was writable when
// the request was queued, so a failure here can only be a program that
// unmapped it meanwhile, and VMS would lose the status too. An event
// flag in a common cluster the process has since disassociated isn't
// set.
func (env *Environment) completeIO(req *ioRequest, done ioStatus) {
	if req.done {
		return
	}

	req.done = true

	for i, r := range env.pendingIO {
		if r == req {
			env.pendingIO = append(env.pendingIO[:i], env.pendingIO[i+1:]...)

			break
		}
	}

	if req.iosb != 0 {
		_ = env.mem.StoreLongword(env.cpu, req.iosb, done.status&0xFFFF|uint32(done.count)<<16)
		_ = env.mem.StoreLongword(env.cpu, req.iosb+4, done.info)
	}

	if flags, bit, st := env.flagWord(req.efn); st == 0 {
		*flags |= 1 << bit
	}

	if req.astadr != 0 {
		env.queueAST(req.astadr, req.astprm, req.mode)
	}
}

// cancelIO completes every request pending on channel c with
// SS$_CANCEL, marking them cancelled so their driver drops them: what
// $CANCEL and $DASSGN do.
func (env *Environment) cancelIO(c *channel) {
	env.cancelAttention(c)

	for _, r := range append([]*ioRequest(nil), env.pendingIO...) {
		if r.channel == c {
			r.cancelled = true
			env.completeIO(r, ioStatus{status: ssCancel})
		}
	}
}

// PendingIO reports how many $QIO requests are waiting to complete.
func (env *Environment) PendingIO() int { return len(env.pendingIO) }

// serviceSysCancel is SYS$CANCEL:
//
//	SYS$CANCEL chan
//
// It cancels the I/O requests outstanding on a channel: each completes
// with SS$_CANCEL in its IOSB, its event flag set and its AST queued
// (cancelIO). It checks the channel with $QIO's rules (SS$_IVCHAN for 0,
// SS$_NOPRIV if it isn't assigned or was assigned from a more privileged
// mode), and also cancels the channel's CTRL/C and CTRL/Y ASTs
// (ctrlast.go).
func serviceSysCancel(env *Environment, argv []uint32) (uint32, error) {
	number := optArg(argv, 0) & 0xFFFF
	if number == 0 {
		return ssIvChan, nil
	}

	c, found := env.findChannel(number)
	if !found || c.Mode < uint32(env.cpu.PSL().CurMod()) {
		return ssNoPriv, nil
	}

	env.cancelIO(c)
	env.disarmChannel(c.Number)

	return ssNormal, nil
}

// accessible reports whether the caller could access (read or write, as
// access says) all n bytes at addr: every page they touch must translate
// in the caller's mode and lie in memory. It's how a driver checks a
// buffer before starting the I/O, so a bad buffer rejects the request
// (SS$_ACCVIO) before any input is consumed or output written. n of 0 is
// always accessible.
func (env *Environment) accessible(addr, n uint32, access vm.AccessType) bool {
	if n == 0 {
		return true
	}

	const pageSize = 512

	last := addr + n - 1
	if last < addr { // wraps past the top of the address space
		return false
	}

	// One address in each page is enough: protection is per page. Start
	// with addr itself, then each following page's first byte.
	for p := addr; ; p = (p &^ (pageSize - 1)) + pageSize {
		phys, err := env.mem.ProbeTranslate(env.cpu, p, access)
		if err != nil || phys >= env.mem.Size() {
			return false
		}

		if p&^(pageSize-1) == last&^(pageSize-1) {
			return true
		}
	}
}

func registerQIOServices(t *ServiceTable) {
	t.Register("SYS$QIO", serviceSysQio)
	t.Register("SYS$QIOW", serviceSysQiow)
	t.Register("SYS$CANCEL", serviceSysCancel)
}
