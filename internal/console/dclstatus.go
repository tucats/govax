package console

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/console/dcl"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// $STATUS, $SEVERITY, and ON (docs/PHASE-50 - DCL command procedures.md,
// subtask 10), as the OpenVMS User's Manual describes them (13.8 to
// 13.15), with what VMS 7.3 showed in testdata/dcl50's probe.
//
// Each command a DCL command interpreter runs ends with a condition value:
// a longword whose low three bits are a severity (0 warning, 1 success, 2
// error, 3 informational, 4 severe), bits 3 to 27 the message and the
// facility that defined it, and bit 28 (STS$M_INHIB_MSG) a flag saying
// the message has already been shown. DCL keeps it in the reserved global
// symbol $STATUS, and its severity in $SEVERITY, both as strings:
//
//	$STATUS == "%X00030001"
//	$SEVERITY == "1"
//
// A command the console's grammar runs reports success with nil, and
// failure with a Go error (most often a vmserrors.VMSError). govax's
// status codes are its own, not VMS's, so conditionValue turns an error
// into the condition value VMS has for the same message, found by name
// (CLI_IVVERB is CLI$_IVVERB, %X00038090), keeping govax's severity. A
// message with no VMS counterpart keeps govax's own code. Success is
// CLI$_NORMAL (%X00030001), DCL's own success status; a few commands
// leave $STATUS as it was when they succeed (13.15: SHOW SYMBOL, IF,
// CONTINUE), and some name the status themselves: @ (the procedure's
// exit status) and RUN (the image's).
//
// In a command procedure, DCL looks at each command's status. By default
// an error or severe error ends the procedure (13.8); ON chooses the
// severity that acts and the command run instead, once (13.9), and SET
// NOON turns the checking off (13.10). Each command level has its own
// (onAction); the terminal's never acts.

// stsInhibitMsg is STS$M_INHIB_MSG, bit 28 of a condition value: its
// message has already been shown, so nothing shows it again.
const stsInhibitMsg = 0x10000000

// The condition values DCL gives a command: CLI$_NORMAL for one that
// succeeded, SS$_NORMAL for $STATUS before any command, and SS$_ABORT
// for a failure govax has no status for (a plain Go error).
var (
	cliNormal = vmsdef.LibrarySymbols["CLI$_NORMAL"]
	ssNormal  = vmsdef.Symbols["SS$_NORMAL"]
	ssAbort   = vmsdef.Symbols["SS$_ABORT"]
)

// conditionPrefixes are the prefixes of VMS's names for the condition
// values of govax's facilities (vmsdef's tables): a govax status whose
// message identifier is IVVERB, in the CLI facility, is CLI$_IVVERB.
var conditionPrefixes = map[uint32]string{
	vmserrors.SYSFacility:   "SS$_",
	vmserrors.RMSFacility:   "RMS$_",
	vmserrors.CLIFacility:   "CLI$_",
	vmserrors.DBGFacility:   "DBG$_",
	vmserrors.LIBFacility:   "LIB$_",
	vmserrors.MOUNTFacility: "MOUNT$_",
}

// vmsCondition returns VMS's condition value for status, a govax status
// code: the value of VMS's symbol for the same message, with govax's
// severity, or status itself if VMS has none.
func vmsCondition(status uint32) uint32 {
	prefix, ok := conditionPrefixes[(status&vmserrors.Facility)>>vmserrors.FacilityPosition]
	ident, named := vmserrors.MessageNames[status]

	if !ok || !named {
		return status
	}

	for _, table := range []map[string]uint32{vmsdef.Symbols, vmsdef.LibrarySymbols} {
		if v, found := table[prefix+ident]; found {
			return v&^vmserrors.Severity | status&vmserrors.Severity
		}
	}

	return status
}

// conditionValue returns the condition value of err, a command's
// failure: a statusError's own, a VMSError's (vmsCondition), or SS$_ABORT
// for any other error. One whose message has been shown
// (vmserrors.InhibitMessage) has STS$M_INHIB_MSG set.
func conditionValue(err error) uint32 {
	var (
		se statusError
		ve vmserrors.VMSError
		v  uint32
	)

	switch {
	case errors.As(err, &se):
		v = se.status
	case errors.As(err, &ve):
		v = vmsCondition(ve.Status)
	default:
		v = ssAbort
	}

	if vmserrors.MessageInhibited(err) {
		v |= stsInhibitMsg
	}

	return v
}

// statusError is a command's failure given as a VMS condition value
// rather than a govax status: an image's exit status, or a procedure's
// EXIT status. Its message is the system message file's text for the
// value (statusText).
type statusError struct {
	status uint32
	text   string
}

func (e statusError) Error() string { return e.text }

// statusFailure returns the error for a command that failed with
// condition value v: a statusError, marked as shown when v has
// STS$M_INHIB_MSG set.
func (c *Console) statusFailure(v uint32) error {
	err := statusError{status: v, text: c.statusText(v)}
	if v&stsInhibitMsg != 0 {
		return vmserrors.InhibitMessage(err)
	}

	return err
}

// statusText is the message for condition value v, without its leading
// "%": "SYSTEM-F-ABORT, abort", or, for a value the message file doesn't
// have, "NONAME-E-NOMSG, Message number 00000002", as VMS 7.3 showed for
// EXIT 2 (testdata/dcl50).
func (c *Console) statusText(v uint32) string {
	if c.RTL != nil {
		return strings.TrimPrefix(c.RTL.StatusText(v), "%")
	}

	letter := [8]string{"W", "S", "E", "I", "F", "?", "?", "?"}[v&vmserrors.Severity]

	if m, ok := vmsdef.LookupMessage(v); ok {
		return m.Facility + "-" + letter + "-" + m.Ident + ", " + m.Text
	}

	facility, ok := vmsdef.MessageFacilities[v>>16&0xFFF]
	if !ok || v&0x0FFFFFF8 == 0 {
		facility = "NONAME"
	}

	return fmt.Sprintf("%s-%s-NOMSG, Message number %08X", facility, letter, v&^stsInhibitMsg)
}

// commandStatus is what the command being dispatched says about
// $STATUS, besides its error: set when it names its status (value, and
// cause, the error behind it, if any), keep when, succeeding, it leaves
// $STATUS alone.
type commandStatus struct {
	set   bool
	value uint32
	cause error
	keep  bool
}

// Status returns $STATUS: the condition value of the last command that
// set it.
func (c *Console) Status() uint32 { return c.status }

// setStatus sets $STATUS to v, and $SEVERITY to its severity: reserved
// global symbols, whose values are strings (VMS 7.3 showed
// $STATUS == "%X00030001" and $SEVERITY == "1"). cause is the error the
// status came from, nil for none: a procedure that ends with the status
// fails with it, so its message is govax's, arguments and all.
func (c *Console) setStatus(v uint32, cause error) {
	c.status, c.statusErr = v, cause
	c.statusSet = true

	g := c.dclSymbols.globals()
	g["$STATUS"] = dclSymbol{name: "$STATUS", minLength: len("$STATUS"), value: fmt.Sprintf("%%X%08X", v), global: true}
	g["$SEVERITY"] = dclSymbol{name: "$SEVERITY", minLength: len("$SEVERITY"), value: strconv.Itoa(int(v & vmserrors.Severity)), global: true}
}

// setCommandStatus says the command being dispatched ends with condition
// value v, whatever error it returns; cause is the error v came from, if
// any (setStatus).
func (c *Console) setCommandStatus(v uint32, cause error) {
	c.commandStatus = commandStatus{set: true, value: v, cause: cause}
}

// keepStatus says the command being dispatched, if it succeeds, leaves
// $STATUS as it was (the User's Manual's 13.15: SHOW SYMBOL, IF, ...).
func (c *Console) keepStatus() { c.commandStatus.keep = true }

// statusOf runs one command line (run) and sets $STATUS from what it
// did. Each line DispatchConsole reads goes through it, so a command
// inside another (IF's THEN, an alias) is part of the same command, and a
// procedure's commands, each a line of their own, set $STATUS one after
// another, before @ sets it last.
func (c *Console) statusOf(run func() error) error {
	saved := c.commandStatus
	c.commandStatus = commandStatus{}

	err := run()

	cs := c.commandStatus
	c.commandStatus = saved

	switch {
	case cs.set:
		c.setStatus(cs.value, cs.cause)
	case err != nil:
		c.setStatus(conditionValue(err), err)
	case !cs.keep:
		c.setStatus(cliNormal, nil)
	}

	return err
}

// Severity ranks for ON: how bad a condition value's severity is, for
// comparing it with ON's. Success and informational statuses rank 0
// (never acted on).
const (
	rankNone = iota
	rankWarning
	rankError
	rankSevere
)

// severityRank returns v's severity's rank.
func severityRank(v uint32) int {
	switch v & vmserrors.Severity {
	case vmserrors.StatusWarning:
		return rankWarning
	case vmserrors.StatusError:
		return rankError
	case vmserrors.StatusSevere:
		return rankSevere
	}

	return rankNone
}

// onAction is a command level's error handling: what ON and SET [NO]ON
// set (the User's Manual, 13.9 to 13.12). The zero value is DCL's
// default: an error or a severe error ends the procedure.
type onAction struct {
	// rank and command are the last ON's: a status of rank or worse runs
	// command. command "" is the default action, EXIT at rankError.
	rank    int
	command string

	// off is SET NOON: no status is acted on, until SET ON or an ON.
	off bool

	// controlY is ON CONTROL_Y's command, "" for none: what a Ctrl/Y
	// during the procedure runs in place of the default (13.12).
	controlY string
}

// take returns what a command that ended with status v makes the level
// do: run command, or (exit) end the procedure, or nothing (act false).
// An ON's command is run only once: taking it puts the default action
// back (13.9.1).
func (a *onAction) take(v uint32) (command string, exit, act bool) {
	rank := severityRank(v)
	if a.off || rank == rankNone {
		return "", false, false
	}

	if a.command == "" {
		return "", true, rank >= rankError
	}

	if rank < a.rank {
		return "", false, false
	}

	command = a.command
	a.rank, a.command = 0, ""

	return command, false, true
}

// onKeywords are ON's conditions, as console.dcl's on_conditions type
// names them, and the rank each acts at; CONTROL_Y is rankNone.
var onKeywords = map[string]int{
	"WARNING":      rankWarning,
	"ERROR":        rankError,
	"SEVERE_ERROR": rankSevere,
	"CONTROL_Y":    rankNone,
}

// onCommand is ON condition THEN [$] command. In a command procedure it
// sets the level's action for statuses of the condition's severity or
// worse (and turns SET NOON's off), or, for CONTROL_Y, its Ctrl/Y
// action; at the terminal, where no status is acted on (13.10), it is
// accepted and does nothing. Without THEN it is CLI_NOTHEN, as DCL has
// it, and with nothing after THEN CLI_INSFPRM. (A condition ON doesn't
// have is the grammar's error, not DCL's IVKEYW.)
func (d *Dispatcher) onCommand(id int64, r *dcl.Result) error {
	rank := onKeywords[r.Keyword("CONDITION")]

	then, command := readCommandVerb(strings.TrimSpace(r.String("COMMAND")))
	if !strings.EqualFold(then, "THEN") {
		return vmserrors.New(vmserrors.CLI_NOTHEN)
	}

	command = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(command), "$"))
	if command == "" {
		return vmserrors.New(vmserrors.CLI_INSFPRM)
	}

	level := d.Console.currentLevel()
	if level == nil {
		return nil
	}

	if r.Keyword("CONDITION") == "CONTROL_Y" {
		level.on.controlY = command

		return nil
	}

	level.on.rank, level.on.command, level.on.off = rank, command, false

	return nil
}

// SetOn is SET ON (on true) and SET NOON: whether the current command
// procedure acts on the statuses of its commands (13.10). At the
// terminal it has no meaning, and does nothing.
func (c *Console) SetOn(on bool) error {
	if level := c.currentLevel(); level != nil {
		level.on.off = !on
	}

	return nil
}

// ControlYAction is the current command level's ON CONTROL_Y command,
// "" for none.
func (c *Console) ControlYAction() string {
	if level := c.currentLevel(); level != nil {
		return level.on.controlY
	}

	return ""
}

// exitCommand is EXIT [status] (Console.Exit): status is a DCL
// expression, whose integer value is the procedure's status. An empty
// one, as "EXIT 'P1'" is when P1 is "", gives none (VMS 7.3 kept $STATUS
// for it, testdata/dcl50).
func (d *Dispatcher) exitCommand(id int64, r *dcl.Result) error {
	text := strings.TrimSpace(r.String("STATUS"))
	if text == "" {
		return d.Console.Exit(nil)
	}

	v, err := evaluateDCLExpression(text, &d.Console.dclSymbols, d.Console)
	if err != nil {
		return err
	}

	status := uint32(v.Int())

	return d.Console.Exit(&status)
}
