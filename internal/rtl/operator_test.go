package rtl

import (
	"strings"
	"testing"

	iodev "github.com/tucats/govax/internal/io"
)

// opcBuf builds a $SNDOPR message buffer: the request code, a 3-byte
// field, a longword, then the rest.
func opcBuf(code byte, field, long uint32, rest string) string {
	b := []byte{code, byte(field), byte(field >> 8), byte(field >> 16),
		byte(long), byte(long >> 8), byte(long >> 16), byte(long >> 24)}

	return string(b) + rest
}

// operatorFixture returns an Environment with a console terminal
// (TTA0, channel tch), a mailbox (channel mch), and its output.
func operatorFixture(t *testing.T) (*Environment, *strings.Builder, *arena, uint32, uint32) {
	t.Helper()

	env, _, a, tch := qioFixture(t, "")
	out := &strings.Builder{}
	env.consoleOut = out
	_, mch := crembx(t, env, a, 0, 0, 0, "")

	return env, out, a, tch, mch
}

// lastMailboxMessage returns the newest message waiting in the mailbox
// on channel ch, or "".
func lastMailboxMessage(t *testing.T, env *Environment, ch uint32) string {
	t.Helper()

	m := mailboxOn(t, env, ch)
	if len(m.messages) == 0 {
		return ""
	}

	return m.messages[len(m.messages)-1].data
}

func TestSndopr_request(t *testing.T) {
	env, out, a, _, mch := operatorFixture(t)

	// A message: no reply wanted.
	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(opcBuf(opcRqRqst, 1, 7, "Hello, operator"))), ssNormal)

	if s := out.String(); !strings.HasPrefix(s, "\n%%%%%%%%%%% OPCOM    ") ||
		!strings.HasSuffix(s, "\nMessage from user SYSTEM on GOVAX\nHello, operator\n") {
		t.Errorf("message output %q", s)
	}

	// A request, with a reply mailbox: numbered and outstanding.
	out.Reset()
	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(opcBuf(opcRqRqst, 8, 42, "Mount tape 17")), mch), ssNormal)

	if s := out.String(); !strings.Contains(s, "\nRequest 1, from user SYSTEM on GOVAX\nMount tape 17\n") {
		t.Errorf("request output %q", s)
	}

	if r := env.Operator.requests; len(r) != 1 || r[0].number != 1 || r[0].rqstid != 42 {
		t.Fatalf("requests %+v", r)
	}

	// Cancelled: withdrawn and announced. Cancel needs the mailbox.
	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(opcBuf(opcRqCancel, 8, 42, ""))), ssBadParam)

	out.Reset()
	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(opcBuf(opcRqCancel, 8, 42, "")), mch), ssNormal)

	if len(env.Operator.requests) != 0 || !strings.Contains(out.String(), "Request 1 was canceled") {
		t.Errorf("after cancel: %d requests, output %q", len(env.Operator.requests), out.String())
	}
}

func TestSndopr_replyAndNoOperator(t *testing.T) {
	env, out, a, _, mch := operatorFixture(t)

	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(opcBuf(opcRqRqst, 1, 42, "Anyone?")), mch), ssNormal)

	// The operator replies to request 1 with status 0x8061 and text.
	reply := string([]byte{opcRqReply, 0, 0x61, 0x80, 1, 0, 0, 0, 0, 0, 4}) + "OPA0" + "Done"
	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(reply)), ssNormal)

	name := "GOVAX$TTA0:"
	want := string([]byte{opcRqReply, 0, 0x61, 0x80, 42, 0, 0, 0, 0, 0, byte(len(name))}) + name + "Done"

	if got := lastMailboxMessage(t, env, mch); got != want {
		t.Errorf("reply %q\nwant  %q", got, want)
	}

	if len(env.Operator.requests) != 0 {
		t.Error("the answered request should be gone")
	}

	// No class enabled: OPC$_NOPERATOR in the mailbox, nothing shown.
	env.Operator.enabled = 0
	out.Reset()
	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(opcBuf(opcRqRqst, 1, 43, "Hello?")), mch), ssNormal)

	if got := lastMailboxMessage(t, env, mch); len(got) < 8 || got[2] != 0x61 || got[3] != 0x80 || got[4] != 43 {
		t.Errorf("no-operator reply %q", got)
	}

	if out.Len() != 0 || len(env.Operator.requests) != 0 {
		t.Errorf("output %q, %d requests; want neither", out.String(), len(env.Operator.requests))
	}
}

func TestSndopr_operatorFunctions(t *testing.T) {
	env, out, a, _, _ := operatorFixture(t)

	// Disable TAPES and DISKS, then look.
	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(opcBuf(opcRqTerme, 0, 0xC, ""))), ssNormal)

	if env.Operator.enabled != allOperatorClasses&^0xC ||
		!strings.Contains(out.String(), "Operator _GOVAX$TTA0: has been disabled, username SYSTEM") {
		t.Errorf("enabled %#x, output %q", env.Operator.enabled, out.String())
	}

	out.Reset()
	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(opcBuf(opcRqStatus, 0, 0, ""))), ssNormal)

	if s := out.String(); !strings.Contains(s, "Operator status for operator _GOVAX$TTA0:\nCENTRAL, PRINTER, DEVICES,") || strings.Contains(s, "TAPES") {
		t.Errorf("status output %q", s)
	}

	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(opcBuf(opcRqTerme, 1, 0x4, ""))), ssNormal)

	if env.Operator.enabled&0x4 == 0 {
		t.Error("TAPES should be enabled again")
	}

	out.Reset()
	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(opcBuf(opcRqLogi, 0, 0, ""))), ssNormal)

	if !strings.Contains(out.String(), "Logfile has been initialized by operator _GOVAX$TTA0:") {
		t.Errorf("log file output %q", out.String())
	}

	// Without OPER, the operator functions are refused; requests aren't.
	env.Process.CurrentPrivileges &^= privOPER

	for _, code := range []byte{opcRqTerme, opcRqStatus, opcRqLogi, opcRqReply} {
		wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(opcBuf(code, 0, 0, "xx"))), ssNoPriv)
	}

	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(opcBuf(opcRqRqst, 1, 0, "hi"))), ssNormal)
}

func TestSndopr_errors(t *testing.T) {
	env, _, a, tch, _ := operatorFixture(t)

	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc("")), ssBadParam)
	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(strings.Repeat("x", 987))), ssBadParam)
	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(opcBuf(99, 0, 0, ""))), ssBadParam)
	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(string([]byte{opcRqRqst, 1}))), ssBadParam)
	wantR0(t, callLNM(t, env, serviceSysSndopr, badAddr), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(opcBuf(opcRqRqst, 1, 0, "x")), tch), ssDevNotMbx)
	wantR0(t, callLNM(t, env, serviceSysSndopr, a.desc(opcBuf(opcRqRqst, 1, 0, "x")), 999), ssNoPriv)
}

func TestBrkthru(t *testing.T) {
	env, out, a, _, _ := operatorFixture(t)
	iosb := a.alloc(8)
	msg := a.desc("System going down")

	// To the caller's terminal, default carriage control: flag, IOSB,
	// AST.
	wantR0(t, callLNM(t, env, serviceSysBrkthru, 5, msg, 0, 0, iosb, 0, 0, 0, 0, 0x5000, 9), ssNormal)

	if out.String() != "\nSystem going down\r" {
		t.Errorf("output %q", out.String())
	}

	if st, n, info := readIOSB(a, iosb); st != ssNormal || n != 1 || info != 0 {
		t.Errorf("IOSB %#x, %d, %#x; want SS$_NORMAL, 1 terminal", st, n, info)
	}

	if !flagSet(env, 5) || env.PendingASTs() != 1 {
		t.Error("the event flag and AST")
	}

	// A named terminal, carriage control "0" (two new lines).
	out.Reset()
	wantR0(t, callLNM(t, env, serviceSysBrkthruw, 0, msg, a.desc("TTA0:"), brkDevice, 0, '0'), ssNormal)

	if out.String() != "\n\nSystem going down\r" {
		t.Errorf("output %q", out.String())
	}

	// A user: SYSTEM is logged in here; nobody else is.
	out.Reset()
	wantR0(t, callLNM(t, env, serviceSysBrkthru, 0, msg, a.desc("SYSTEM"), brkUsername, iosb), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysBrkthru, 0, msg, a.desc("GUEST"), brkUsername, iosb), ssNormal)

	if _, n, _ := readIOSB(a, iosb); n != 0 || strings.Count(out.String(), "System going down") != 1 {
		t.Errorf("GUEST: %d terminals, output %q", n, out.String())
	}

	// Every terminal: the fixture has one terminal device.
	defineTestDevice(env, "TTA1", iodev.DeviceClassTT)
	wantR0(t, callLNM(t, env, serviceSysBrkthru, 0, msg, 0, brkAllTerms, iosb), ssNormal)

	if _, n, _ := readIOSB(a, iosb); n != 2 {
		t.Errorf("all terminals: %d, want 2", n)
	}
}

func TestBrkthru_errors(t *testing.T) {
	env, _, a, _, _ := operatorFixture(t)
	msg := a.desc("x")
	p := env.Process

	wantR0(t, callLNM(t, env, serviceSysBrkthru, 0, msg, a.desc("NOSUCH0:"), brkDevice), ssNoSuchDev)
	wantR0(t, callLNM(t, env, serviceSysBrkthru, 0, msg, a.desc("MBA1:"), brkDevice), ssNoSuchDev) // not a terminal
	wantR0(t, callLNM(t, env, serviceSysBrkthru, 0, msg, 0, brkDevice), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysBrkthru, 0, msg, 0, 9), ssBadParam)
	wantR0(t, callLNM(t, env, serviceSysBrkthru, 0, msg, 0, 0, 0, 0, 0, 64), ssBadParam)    // reqid
	wantR0(t, callLNM(t, env, serviceSysBrkthru, 0, msg, 0, 0, 0, 0, 0, 0, 3), ssBadParam)  // timout
	wantR0(t, callLNM(t, env, serviceSysBrkthru, 0, a.desc(strings.Repeat("x", 16351))), ssBadParam)
	wantR0(t, callLNM(t, env, serviceSysBrkthru, 0, badAddr), ssAccVio)
	wantR0(t, callLNM(t, env, serviceSysBrkthru, 0, msg, 0, 0, badAddr), ssAccVio)

	p.CurrentPrivileges &^= privOPER | privWORLD
	wantR0(t, callLNM(t, env, serviceSysBrkthru, 0, msg, 0, brkAllUsers), ssNoOper)
	wantR0(t, callLNM(t, env, serviceSysBrkthru, 0, msg, 0, brkAllTerms), ssNoOper)
	wantR0(t, callLNM(t, env, serviceSysBrkthru, 0, msg, a.desc("SYSTEM"), brkUsername), ssNoPriv)
}
