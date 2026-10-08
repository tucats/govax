package corevms

import (
	"fmt"
	"strings"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vmsdef"
)

// Operator and broadcast messages (docs/PHASE-26.md subtask 39): $SNDOPR,
// $BRKTHRU, and $BRKTHRUW.
//
// # Operators
//
// A VMS system has *operators*: people at designated terminals who mount
// tapes, load printers, and watch for trouble. Programs talk to them
// through OPCOM, the operator communication process, by calling $SNDOPR
// with a message buffer whose first byte is a request code:
//
//	OPC$_RQ_RQST    a request ("please mount tape 17"), shown on the
//	                operator terminals enabled for the classes it names
//	OPC$_RQ_CANCEL  cancel an earlier request
//	OPC$_RQ_REPLY   an operator's reply to a request
//	OPC$_RQ_TERME   enable or disable a terminal as an operator terminal
//	                for some classes
//	OPC$_RQ_STATUS  show which classes an operator terminal has
//	OPC$_RQ_LOGI    start a new operator log file
//
// Operator *classes* (CENTRAL, TAPES, DISKS, ...) say what a request is
// about; a request goes to the terminals enabled for any of its classes.
// A program that wants an answer passes a mailbox channel: the request
// stays outstanding, numbered, and OPCOM puts the operator's reply in the
// mailbox. A message on an operator terminal looks like
//
//	%%%%%%%%%%% OPCOM    30-DEC-1988 12:57:32.25
//	Request 1285, from user ROSS on NODE_NAME
//	Please mount device _NODE$DMA0:
//
// govax has one terminal, the console, and it is the operator terminal,
// enabled for every class at first (as a VMS console is). There is no
// REPLY command, so the only replies a request gets are one a program
// sends itself with OPC$_RQ_REPLY, and OPC$_NOPERATOR when no class the
// request names is enabled.
//
// # Broadcasts
//
// $BRKTHRU writes a message to terminals *through* whatever they're doing
// — interrupting a read, say — which is how SHUTDOWN, MAIL's "new mail"
// notice, and REPLY/ALL reach users. The message goes to one terminal,
// one user's terminals, all users' terminals, or every terminal (sndtyp,
// $BRKDEF), with carriage control like a terminal write's, and completes
// like a $QIO: event flag, I/O status block, AST. In govax every
// terminal is the console, so the message is written there once; the
// I/O status block counts the terminals it was sent to.

// $OPCMSG request codes: the first byte of a $SNDOPR message buffer.
// Only OPC$_RQ_RQST's value (3) appears in the VMS 7.3 source archive's
// listings; OPCOM's own dispatch (opcom/lis/opcommain.lis) lists the six
// in this order in a CASE running from 1, which gives the others.
const (
	opcRqTerme  = 1
	opcRqLogi   = 2
	opcRqRqst   = 3
	opcRqReply  = 4
	opcRqCancel = 5
	opcRqStatus = 6
)

// opcNoOperator is OPC$_NOPERATOR, the reply status "no operator terminal
// is enabled for the request" (its value is in the archive's listings).
const opcNoOperator = 0x00058061

// maxOperatorMessage is the longest $SNDOPR message buffer, in bytes.
const maxOperatorMessage = 986

// operatorClasses are the operator classes (OPC$M_NM_), in bit order, as
// OPCOM's status display names them. CENTRL, TAPES, DISKS, DEVICE,
// NTWORK, and CLUSTER have their values confirmed by the archive's
// listings; the rest are in VMS's documented order. OPER1-OPER12 are
// bits 12-23.
var operatorClasses = []string{
	"CENTRAL", "PRINTER", "TAPES", "DISKS", "DEVICES", "CARDS", "NETWORK",
	"CLUSTER", "SECURITY", "REPLY", "SOFTWARE", "LICENSE",
	"OPER1", "OPER2", "OPER3", "OPER4", "OPER5", "OPER6",
	"OPER7", "OPER8", "OPER9", "OPER10", "OPER11", "OPER12",
}

// allOperatorClasses is every class bit: the 24-bit target mask.
const allOperatorClasses = 1<<24 - 1

// operatorRequest is an outstanding OPC$_RQ_RQST, waiting for a reply.
type operatorRequest struct {
	number  uint32   // OPCOM's request number, shown to the operator
	rqstid  uint32   // the requester's own code, OPC$L_MS_RQSTID
	channel *channel // the mailbox channel the reply goes to
}

// operatorState is OPCOM's state: the classes the console is enabled
// for, and the outstanding requests. It's system state, so INIT, VMINIT,
// and ZERO start afresh.
type operatorState struct {
	enabled     uint32
	requests    []*operatorRequest
	nextRequest uint32
}

// newOperatorState returns OPCOM as it starts: the console enabled for
// every class, no requests.
func newOperatorState() *operatorState {
	return &operatorState{enabled: allOperatorClasses, nextRequest: 1}
}

// operatorTerminal is the operator terminal's full name, as OPCOM shows
// it: _NODE$TTA0:.
func (env *Environment) operatorTerminal() string {
	return "_" + env.NodeName + "$" + strings.Trim(env.Process.Terminal, "_:") + ":"
}

// opcomMessage writes lines to the operator terminal under OPCOM's
// banner, stamped with the current time.
func (env *Environment) opcomMessage(lines ...string) {
	stamp, _ := formatVMSTime(env.Clock(), false)
	env.writeConsole("\n%%%%%%%%%%% OPCOM    " + stamp + "\n" + strings.Join(lines, "\n") + "\n")
}

// serviceSysSndopr is SYS$SNDOPR:
//
//	SYS$SNDOPR msgbuf ,[chan]
//
// msgbuf describes the message buffer (see this file's opening comment
// and the request functions below). chan, if not 0, is a channel to a
// mailbox for the reply.
//
// It returns SS$_NORMAL; SS$_BADPARAM for a buffer of 0 or more than 986
// bytes, too short for its request, or an unknown request code; SS$_ACCVIO
// if the buffer can't be read; SS$_IVCHAN, SS$_NOPRIV, or SS$_DEVNOTMBX
// for a bad chan; SS$_NOPRIV for a request that needs the OPER privilege
// (reply, enable, status, log file) without it.
func serviceSysSndopr(env *Environment, argv []uint32) (uint32, error) {
	desc := optArg(argv, 0)
	if desc == 0 {
		return ssAccVio, nil
	}

	msg, ok, err := strGet(env, desc, maxOperatorMessage)

	switch {
	case err != nil:
		return ssAccVio, nil
	case !ok || msg == "":
		return ssBadParam, nil
	}

	var mbx *channel

	if number := optArg(argv, 1) & 0xFFFF; number != 0 {
		c, found := env.findChannel(number)
		if !found || c.Mode < uint32(env.cpu.PSL().CurMod()) {
			return ssNoPriv, nil
		}

		if _, isMailbox := env.Mailboxes.For(c.Device); !isMailbox {
			return ssDevNotMbx, nil
		}

		mbx = c
	}

	b := []byte(msg)

	switch b[0] {
	case opcRqRqst:
		return env.operatorRequest(b, mbx), nil
	case opcRqCancel:
		return env.operatorCancel(b, mbx), nil
	}

	// The rest are the operator's own functions.
	if !env.Process.hasPrivilege(privOPER) {
		return ssNoPriv, nil
	}

	switch b[0] {
	case opcRqReply:
		return env.operatorReply(b), nil
	case opcRqTerme:
		return env.operatorEnable(b), nil
	case opcRqStatus:
		env.operatorStatus()

		return ssNormal, nil
	case opcRqLogi:
		env.opcomMessage("Logfile has been initialized by operator " + env.operatorTerminal())

		return ssNormal, nil
	}

	return ssBadParam, nil
}

// le32 reads a little-endian longword from b at offset i.
func le32(b []byte, i int) uint32 {
	return uint32(b[i]) | uint32(b[i+1])<<8 | uint32(b[i+2])<<16 | uint32(b[i+3])<<24
}

// operatorRequest is OPC$_RQ_RQST:
//
//	+0  OPC$B_MS_TYPE    OPC$_RQ_RQST
//	+1  OPC$B_MS_TARGET  the classes to send it to (3 bytes)
//	+4  OPC$L_MS_RQSTID  the requester's code for it
//	+8  OPC$L_MS_TEXT    the text
//
// Shown on the console if it's enabled for any of the classes, as
// "Request n, from user ..." if a reply mailbox was given (the request
// is then outstanding, numbered n) or "Message from user ..." if not.
// With a mailbox and no class enabled, the reply is OPC$_NOPERATOR.
func (env *Environment) operatorRequest(b []byte, mbx *channel) uint32 {
	if len(b) < 8 {
		return ssBadParam
	}

	target, rqstid, text := le32(b, 0)>>8, le32(b, 4), string(b[8:])
	opr := env.Operator
	from := fmt.Sprintf("from user %s on %s", env.Process.Username, env.NodeName)

	if target&opr.enabled == 0 {
		if mbx != nil {
			env.operatorReplyTo(mbx, opcNoOperator, rqstid, "")
		}

		return ssNormal
	}

	if mbx == nil {
		env.opcomMessage("Message "+from, text)

		return ssNormal
	}

	r := &operatorRequest{number: opr.nextRequest, rqstid: rqstid, channel: mbx}
	opr.nextRequest++
	opr.requests = append(opr.requests, r)

	env.opcomMessage(fmt.Sprintf("Request %d, %s", r.number, from), text)

	return ssNormal
}

// operatorCancel is OPC$_RQ_CANCEL:
//
//	+0  OPC$B_MS_TYPE    OPC$_RQ_CANCEL
//	+1  OPC$B_MS_TARGET  the classes (3 bytes)
//	+4  OPC$L_MS_RQSTID  the code of the request to cancel
//
// It needs the mailbox the requests were made with: each outstanding
// request with that code and mailbox is withdrawn, and the operator is
// told.
func (env *Environment) operatorCancel(b []byte, mbx *channel) uint32 {
	if len(b) < 8 || mbx == nil {
		return ssBadParam
	}

	rqstid := le32(b, 4)
	opr := env.Operator
	kept := opr.requests[:0]

	for _, r := range opr.requests {
		if r.channel == mbx && r.rqstid == rqstid {
			env.opcomMessage(fmt.Sprintf("Request %d was canceled", r.number))

			continue
		}

		kept = append(kept, r)
	}

	opr.requests = kept

	return ssNormal
}

// operatorReply is OPC$_RQ_REPLY, an operator's answer:
//
//	+0  OPC$B_MS_TYPE    OPC$_RQ_REPLY
//	+2  OPC$W_MS_STATUS  the reply status's low word
//	+4  OPC$L_MS_RPLYID  the number of the request answered
//	+8  OPC$W_MS_OUNIT, OPC$T_MS_ONAME, OPC$L_MS_OTEXT: the terminal, and
//	    the operator's text
//
// The request is answered: its mailbox gets the reply (see
// operatorReplyTo) with the status, whose facility and severity come
// from OPCOM's (the high word of OPC$_NOPERATOR's), and the text. A
// number with no outstanding request is ignored, as OPCOM would.
func (env *Environment) operatorReply(b []byte) uint32 {
	if len(b) < 10 {
		return ssBadParam
	}

	status := uint32(b[2]) | uint32(b[3])<<8 | opcNoOperator&0xFFFF0000
	number := le32(b, 4)

	// The text follows the counted terminal name.
	text := ""

	if len(b) > 10 {
		if end := 11 + int(b[10]); end <= len(b) {
			text = string(b[end:])
		}
	}

	opr := env.Operator

	for i, r := range opr.requests {
		if r.number == number {
			opr.requests = append(opr.requests[:i], opr.requests[i+1:]...)

			env.operatorReplyTo(r.channel, status, r.rqstid, text)

			break
		}
	}

	return ssNormal
}

// operatorReplyTo puts OPCOM's reply in the mailbox channel c is assigned
// to, in the OPC$_RQ_REPLY layout the manual gives for "the reply found
// in the user's mailbox": the request code, a reserved byte, the status's
// low word, the requester's code (rqstid), the operator terminal's unit
// and counted name, then the text.
func (env *Environment) operatorReplyTo(c *channel, status, rqstid uint32, text string) {
	m, ok := env.Mailboxes.For(c.Device)
	if !ok {
		return
	}

	name := strings.TrimPrefix(env.operatorTerminal(), "_")

	var b []byte
	b = append(b, opcRqReply, 0, byte(status), byte(status>>8))
	b = append(b, byte(rqstid), byte(rqstid>>8), byte(rqstid>>16), byte(rqstid>>24))
	b = append(b, 0, 0, byte(len(name)))
	b = append(b, name...)
	b = append(b, text...)

	env.postMailboxMessage(m, string(b))
}

// operatorEnable is OPC$_RQ_TERME:
//
//	+0  OPC$B_MS_TYPE   OPC$_RQ_TERME
//	+1  OPC$B_MS_ENAB   nonzero to enable, 0 to disable (3 bytes)
//	+4  OPC$L_MS_MASK   the classes
//	+8  OPC$W_MS_OUNIT, OPC$T_MS_ONAME: the terminal (the console, the
//	    only one govax has, whatever is named)
func (env *Environment) operatorEnable(b []byte) uint32 {
	if len(b) < 8 {
		return ssBadParam
	}

	enable, mask := le32(b, 0)>>8 != 0, le32(b, 4)&allOperatorClasses
	opr := env.Operator
	verb := "enabled"

	if enable {
		opr.enabled |= mask
	} else {
		opr.enabled &^= mask
		verb = "disabled"
	}

	env.opcomMessage(fmt.Sprintf("Operator %s has been %s, username %s", env.operatorTerminal(), verb, env.Process.Username))

	return ssNormal
}

// operatorStatus is OPC$_RQ_STATUS: the console's classes, listed.
func (env *Environment) operatorStatus() {
	var names []string

	for i, name := range operatorClasses {
		if env.Operator.enabled&(1<<i) != 0 {
			names = append(names, name)
		}
	}

	env.opcomMessage("Operator status for operator "+env.operatorTerminal(), strings.Join(names, ", "))
}

// postMailboxMessage puts a message from the system (OPCOM's reply) in
// mailbox m, as an IO$M_NOW write would: a waiting read gets it, or it's
// queued. It never waits; a full mailbox loses it.
func (env *Environment) postMailboxMessage(m *Mailbox, data string) {
	req := &ioRequest{modifiers: ioModNow | ioModNoRSWait}
	_, _ = env.send(m, req, &mailboxMessage{data: data})
}

// $BRKDEF values.
var (
	brkDevice    = vmsdef.Symbols["BRK$C_DEVICE"]
	brkUsername  = vmsdef.Symbols["BRK$C_USERNAME"]
	brkAllUsers  = vmsdef.Symbols["BRK$C_ALLUSERS"]
	brkAllTerms  = vmsdef.Symbols["BRK$C_ALLTERMS"]
	brkMaxSendTy = vmsdef.Symbols["BRK$C_MAXSENDTYPE"]
)

// Limits $BRKTHRU checks.
const (
	maxBreakthroughMessage = 16350 // bytes of message text
	maxBreakthroughReqID   = 63    // the highest requestor class
	defaultBreakthroughCC  = 32    // carcon: a space, "new line, text, return"
)

// ssNoOper is SS$_NOOPER, a broadcast to several terminals without OPER.
var ssNoOper = vmsdef.Symbols["SS$_NOOPER"]

// serviceSysBrkthru is SYS$BRKTHRU:
//
//	SYS$BRKTHRU [efn] ,msgbuf [,sendto] [,sndtyp] [,iosb] [,carcon]
//	            [,flags] [,reqid] [,timout] [,astadr] [,astprm]
//
// It sends the message msgbuf describes (by descriptor) to terminals
// chosen by sndtyp (0 or omitted is the caller's own terminal, as
// BRK$C_DEVICE with its name would be):
//
//	BRK$C_DEVICE    the terminal sendto names (SS$_NOSUCHDEV if it isn't
//	                a terminal)
//	BRK$C_USERNAME  the terminals of the user sendto names; needs WORLD
//	BRK$C_ALLUSERS  every logged-in user's terminal; needs OPER
//	BRK$C_ALLTERMS  every terminal; needs OPER
//
// carcon is carriage control, as for a terminal $QIO's p4 (default 32: a
// new line, the message, a return). flags (screen formatting) are
// accepted and ignored. The request completes at once: efn (cleared
// first) is set, iosb gets SS$_NORMAL and the number of terminals sent to,
// and astadr's AST is queued with astprm.
//
// It returns SS$_NORMAL; SS$_BADPARAM for a message over 16,350 bytes, a
// timout of 1-4, a reqid over 63, or an unknown sndtyp; SS$_ACCVIO for a
// message, sendto, or iosb it can't access; SS$_NOOPER or SS$_NOPRIV for a
// missing privilege; SS$_NOSUCHDEV; or $QIO's event-flag errors.
func serviceSysBrkthru(env *Environment, argv []uint32) (uint32, error) {
	return env.breakthrough(argv)
}

// serviceSysBrkthruw is SYS$BRKTHRUW: $BRKTHRU, returning once the
// message has been written — which, with every message written during
// the call, is the same thing.
func serviceSysBrkthruw(env *Environment, argv []uint32) (uint32, error) {
	return env.breakthrough(argv)
}

// breakthrough is the body of $BRKTHRU and $BRKTHRUW (see serviceSysBrkthru).
func (env *Environment) breakthrough(argv []uint32) (uint32, error) {
	efn, msgbuf, sendto, sndtyp := optArg(argv, 0), optArg(argv, 1), optArg(argv, 2), optArg(argv, 3)
	iosb, carcon, reqid, timout := optArg(argv, 4), optArg(argv, 5), optArg(argv, 7), optArg(argv, 8)
	astadr, astprm := optArg(argv, 9), optArg(argv, 10)

	if msgbuf == 0 {
		return ssAccVio, nil
	}

	msg, ok, err := strGet(env, msgbuf, maxBreakthroughMessage)

	switch {
	case err != nil:
		return ssAccVio, nil
	case !ok, timout > 0 && timout < 5, reqid > maxBreakthroughReqID, sndtyp > brkMaxSendTy:
		return ssBadParam, nil
	}

	terminals, st := env.breakthroughTerminals(sndtyp, sendto)
	if st != 0 {
		return st, nil
	}

	// Complete it as a $QIO would: clear the flag and the IOSB, write,
	// then set the flag, fill in the IOSB, and queue the AST.
	flags, bit, st := env.eventFlagWord(efn)
	if st != 0 {
		return st, nil
	}

	*flags &^= 1 << bit

	if iosb != 0 && !env.storeQuad(iosb, 0) {
		return ssAccVio, nil
	}

	if terminals > 0 {
		if carcon == 0 {
			carcon = defaultBreakthroughCC
		}

		prefix, postfix := carriageControl(carcon)
		env.writeConsole(prefix + msg + postfix)
	}

	env.postFlag(efn, sched.ClassIOCompletion)

	if iosb != 0 {
		// Status, terminals sent to, timed out (0), set NOBROADCAST (0).
		_ = env.mem.StoreLongword(env.cpu, iosb, uint32(ssNormal)|uint32(terminals)<<16)
		_ = env.mem.StoreLongword(env.cpu, iosb+4, 0)
	}

	if astadr != 0 {
		env.queueAST(astadr, astprm, uint32(env.cpu.PSL().CurMod()))
	}

	return ssNormal, nil
}

// breakthroughTerminals counts the terminals a broadcast of type sndtyp
// reaches (sendto naming the device or user where the type needs one),
// checking the privileges the type needs. Every terminal is the console,
// so the count is all the caller learns (in the IOSB).
func (env *Environment) breakthroughTerminals(sndtyp, sendto uint32) (int, uint32) {
	p := env.Process

	switch sndtyp {
	case 0: // the caller's own terminal
		return 1, 0

	case brkDevice:
		name, st := env.breakthroughName(sendto)
		if st != 0 {
			return 0, st
		}

		device, st := env.deviceName(name)
		if st != 0 {
			return 0, ssNoSuchDev
		}

		if d, found := env.Devices.Find(device); !found || d.DevClass != iodev.DeviceClassTT {
			return 0, ssNoSuchDev
		}

		return 1, 0

	case brkUsername:
		if !p.hasPrivilege(privWORLD) {
			return 0, ssNoPriv
		}

		name, st := env.breakthroughName(sendto)
		if st != 0 {
			return 0, st
		}

		// Only this process's user is logged in, at one terminal.
		if strings.EqualFold(strings.TrimSpace(name), p.Username) {
			return 1, 0
		}

		return 0, 0

	case brkAllUsers, brkAllTerms:
		if !p.hasPrivilege(privOPER) {
			return 0, ssNoOper
		}

		if sndtyp == brkAllUsers {
			return 1, 0 // this process's user's terminal
		}

		// The terminal devices; at least the one this process is logged
		// in at, even if the device table doesn't list it.
		n := 0

		for _, d := range env.Devices.All() {
			if d.DevClass == iodev.DeviceClassTT {
				n++
			}
		}

		return max(n, 1), 0
	}

	return 0, ssBadParam
}

// breakthroughName reads $BRKTHRU's sendto descriptor: SS$_ACCVIO if it's
// missing or can't be read, SS$_BADPARAM if it's too long for a name.
func (env *Environment) breakthroughName(sendto uint32) (string, uint32) {
	if sendto == 0 {
		return "", ssAccVio
	}

	name, ok, err := strGet(env, sendto, maxOperatorMessage)

	switch {
	case err != nil:
		return "", ssAccVio
	case !ok:
		return "", ssBadParam
	}

	return name, 0
}

func registerOperatorServices(t *ServiceTable) {
	t.Register("SYS$SNDOPR", serviceSysSndopr)
	t.Register("SYS$BRKTHRU", serviceSysBrkthru)
	t.Register("SYS$BRKTHRUW", serviceSysBrkthruw)
}
