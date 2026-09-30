package rtl

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

func jpiCode(t *testing.T, name string) uint16 {
	t.Helper()

	code, ok := vmsdef.Symbols[name]
	if !ok {
		t.Fatalf("no $JPIDEF code %s", name)
	}

	return uint16(code)
}

// getjpi calls $GETJPIW with a 7-argument list.
func getjpi(t *testing.T, env *Environment, efn, pidadr, prcnam, itmlst, iosb uint32) uint32 {
	t.Helper()

	return callLNM(t, env, serviceSysGetjpi, efn, pidadr, prcnam, itmlst, iosb, 0, 0)
}

func TestServiceSysGetjpiItems(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	p := env.Process
	// Bit 0 is left clear: the request clears its event flag (efn 0 here)
	// when it starts, before the items are read.
	p.LocalEventFlags = [2]uint32{0x10, 0x22}

	type want struct {
		name string
		str  string
		long uint32
	}

	wants := []want{
		{name: "JPI$_ACCOUNT", str: "SYSTEM  "},
		{name: "JPI$_CLINAME", str: "DCL"},
		{name: "JPI$_USERNAME", str: "SYSTEM      "},
		{name: "JPI$_PRCNAM", str: "SYSTEM"},
		{name: "JPI$_TERMINAL", str: "TTA0:"},
		{name: "JPI$_PID", long: p.PID},
		{name: "JPI$_MASTER_PID", long: p.PID},
		{name: "JPI$_OWNER", long: 0},
		{name: "JPI$_UIC", long: 0x00010004},
		{name: "JPI$_GRP", long: 1},
		{name: "JPI$_MEM", long: 4},
		{name: "JPI$_MODE", long: vmsdef.Symbols["JPI$K_INTERACTIVE"]},
		{name: "JPI$_JOBTYPE", long: vmsdef.Symbols["JPI$K_LOCAL"]},
		{name: "JPI$_EFCS", long: 0x10},
		{name: "JPI$_EFCU", long: 0x22},
		{name: "JPI$_DFWSCNT", long: p.WSDefault},
		{name: "JPI$_WSQUOTA", long: p.WSQuota},
		{name: "JPI$_WSAUTH", long: p.WSQuota},
		{name: "JPI$_WSEXTENT", long: p.WSExtent},
		{name: "JPI$_WSAUTHEXT", long: p.WSExtent},
		{name: "JPI$_WSSIZE", long: p.WSLimit},
		{name: "JPI$_ASTLM", long: 24},
		{name: "JPI$_ASTCNT", long: 24},
		{name: "JPI$_ASTEN", long: 0xF},
		{name: "JPI$_ASTACT", long: 0},
		{name: "JPI$_PRI", long: 4},
		{name: "JPI$_PRIB", long: 4},
		{name: "JPI$_STATE", long: 14}, // SCH$C_CUR
	}

	items := make([]item, len(wants))
	for i, w := range wants {
		items[i] = item{code: jpiCode(t, w.name), buflen: 16, buf: a.alloc(16), ret: a.alloc(2)}
	}

	iosb := a.alloc(8)
	wantR0(t, getjpi(t, env, 0, 0, 0, a.items(items...), iosb), ssNormal)

	for i, w := range wants {
		n, err := env.mem.LoadWord(env.cpu, items[i].ret)
		if err != nil {
			t.Fatal(err)
		}

		if w.str != "" {
			if got := a.readString(items[i].buf, n); got != w.str {
				t.Errorf("%s = %q, want %q", w.name, got, w.str)
			}

			continue
		}

		if n != 4 || a.readLong(items[i].buf) != w.long {
			t.Errorf("%s = %#x (length %d), want %#x (length 4)", w.name, a.readLong(items[i].buf), n, w.long)
		}
	}

	if got := a.readLong(iosb); got != ssNormal {
		t.Errorf("IOSB status = %#x, want SS$_NORMAL", got)
	}
}

func TestServiceSysGetjpiTruncationAndChain(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	// A short buffer truncates, string or longword alike.
	user, userRet := a.alloc(4), a.alloc(2)
	uic, uicRet := a.alloc(4), a.alloc(2)
	putLongword(t, env, uic, 0xAAAAAAAA)

	// JPI$_CHAIN continues with a second list.
	pid := a.alloc(4)
	second := a.items(item{code: jpiCode(t, "JPI$_PID"), buflen: 4, buf: pid})
	first := a.items(
		item{code: jpiCode(t, "JPI$_USERNAME"), buflen: 3, buf: user, ret: userRet},
		item{code: jpiCode(t, "JPI$_UIC"), buflen: 2, buf: uic, ret: uicRet},
		item{code: jpiChain, buflen: 12, buf: second},
	)

	wantR0(t, getjpi(t, env, 0, 0, 0, first, 0), ssNormal)

	if n, _ := env.mem.LoadWord(env.cpu, userRet); n != 3 || a.readString(user, 3) != "SYS" {
		t.Errorf("truncated USERNAME = %q (length %d), want SYS (3)", a.readString(user, n), n)
	}

	if n, _ := env.mem.LoadWord(env.cpu, uicRet); n != 2 || a.readLong(uic) != 0xAAAA0004 {
		t.Errorf("truncated UIC = %#x (length %d), want only the low word 0x0004 written", a.readLong(uic), n)
	}

	if a.readLong(pid) != env.Process.PID {
		t.Errorf("chained PID = %#x, want %#x", a.readLong(pid), env.Process.PID)
	}
}

func TestServiceSysGetjpiProcessSelection(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	p := env.Process
	list := a.items(item{code: jpiCode(t, "JPI$_PID"), buflen: 4, buf: a.alloc(4)})

	// PID 0 means the caller, and is replaced by its PID.
	pidadr := a.long(0)
	wantR0(t, getjpi(t, env, 0, pidadr, 0, list, 0), ssNormal)

	if got := a.readLong(pidadr); got != p.PID {
		t.Errorf("pidadr = %#x, want the caller's PID %#x written back", got, p.PID)
	}

	// Its own PID works; another doesn't. A nonzero PID wins over prcnam.
	wantR0(t, getjpi(t, env, 0, a.long(p.PID), a.desc("OTHER"), list, 0), ssNormal)
	wantR0(t, getjpi(t, env, 0, a.long(0x999), 0, list, 0), ssNonExpr)

	// By name: exactly the process name; the PID is returned when pidadr
	// holds 0.
	pidadr = a.long(0)
	wantR0(t, getjpi(t, env, 0, pidadr, a.desc("SYSTEM"), list, 0), ssNormal)

	if got := a.readLong(pidadr); got != p.PID {
		t.Errorf("pidadr after a by-name call = %#x, want %#x", got, p.PID)
	}

	wantR0(t, getjpi(t, env, 0, 0, a.desc("SYST"), list, 0), ssNonExpr)
	wantR0(t, getjpi(t, env, 0, 0, a.desc("system"), list, 0), ssNonExpr)
	wantR0(t, getjpi(t, env, 0, 0, a.desc(""), list, 0), ssIvLogNam)
	wantR0(t, getjpi(t, env, 0, 0, a.desc(strings.Repeat("P", 16)), list, 0), ssIvLogNam)

	// Wildcard: this process once, then SS$_NOMOREPROC.
	pidadr = a.long(0xFFFFFFFF)
	wantR0(t, getjpi(t, env, 0, pidadr, 0, list, 0), ssNormal)
	wantR0(t, getjpi(t, env, 0, pidadr, 0, list, 0), ssNoMoreProc)
}

func TestServiceSysGetjpiCompletion(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	// The event flag is set on completion (efn 0 by default)...
	list := a.items(item{code: jpiCode(t, "JPI$_PID"), buflen: 4, buf: a.alloc(4)})
	wantR0(t, getjpi(t, env, 5, 0, 0, list, 0), ssNormal)

	if env.Process.LocalEventFlags[0]&(1<<5) == 0 {
		t.Error("event flag 5 not set on completion")
	}

	wantR0(t, getjpi(t, env, 0, 0, 0, list, 0), ssNormal)

	if env.Process.LocalEventFlags[0]&1 == 0 {
		t.Error("event flag 0 not set on completion with efn omitted")
	}

	// ...and the IOSB and R0 carry an item-list failure.
	iosb := a.alloc(8)
	bad := a.items(item{code: 9999, buflen: 4, buf: a.alloc(4)})
	wantR0(t, getjpi(t, env, 0, 0, 0, bad, iosb), ssBadParam)

	if got := a.readLong(iosb); got != ssBadParam {
		t.Errorf("IOSB status = %#x, want SS$_BADPARAM", got)
	}

	// An unassociated common event flag cluster is refused up front.
	wantR0(t, getjpi(t, env, 64, 0, 0, list, 0), ssUnasEfc)

	// Too short an argument list.
	wantR0(t, callLNM(t, env, serviceSysGetjpi, 1, 2), ssInsfArg)
}

func TestServiceSysGetjpiDebugProcessTrace(t *testing.T) {
	var buf bytes.Buffer

	env, _ := fixture()
	a := newArena(t, env)

	env.cpu.SetDebugWriter(&buf)
	env.cpu.SetDebug(vax.DebugProcess)

	getjpi(t, env, 5, 0, a.desc("MYPROC"), a.items(), 0)

	if !strings.Contains(buf.String(), `DEBUG: SYS$GETJPIW EFN=5 PRCNAM="MYPROC"`) {
		t.Errorf("output = %q, want a SYS$GETJPIW trace naming EFN and PRCNAM", buf.String())
	}
}

// TestJPIItemsRegistry checks every registry entry names a real $JPIDEF
// item code (jpiItems panics at init otherwise, but this names the
// culprit) and that the pre-existing ACCOUNT/CLINAME codes are unchanged.
// TestServiceSysGetjpiASTItems: the AST items follow Process.ast and the
// timer queue. ASTCNT is the quota less every queued AST and every
// $SETIMR timer that will queue one; ASTEN and ASTACT are per-mode bit
// vectors, bit 0 kernel through bit 3 user.
func TestServiceSysGetjpiASTItems(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	p := env.Process

	env.queueAST(0x4000, 1, uint32(vax.User))
	env.queueAST(0x4000, 2, uint32(vax.User))
	env.timers = append(env.timers,
		&timerRequest{expiry: ^uint64(0), astadr: 0x5000}, // will queue an AST
		&timerRequest{expiry: ^uint64(0)},                 // won't
		&timerRequest{expiry: ^uint64(0), wake: true},     // a $SCHDWK wakeup
	)

	p.ast.enabled = [4]bool{true, false, true, false}
	p.ast.active = [4]bool{false, true, false, true}

	get := func(name string) uint32 {
		t.Helper()

		buf := a.alloc(4)
		wantR0(t, getjpi(t, env, 0, 0, 0, a.items(item{code: jpiCode(t, name), buflen: 4, buf: buf}), 0), ssNormal)

		return a.readLong(buf)
	}

	if got := get("JPI$_ASTCNT"); got != 21 {
		t.Errorf("JPI$_ASTCNT = %d, want 21 (24 less two queued ASTs and one timer AST)", got)
	}

	if got := get("JPI$_ASTEN"); got != 0x5 {
		t.Errorf("JPI$_ASTEN = %#x, want 0x5 (kernel and supervisor)", got)
	}

	if got := get("JPI$_ASTACT"); got != 0xA {
		t.Errorf("JPI$_ASTACT = %#x, want 0xA (executive and user)", got)
	}

	// More outstanding than the quota (it isn't enforced) is 0, not a
	// wrapped-around count.
	p.ASTLimit = 1
	
	if got := get("JPI$_ASTCNT"); got != 0 {
		t.Errorf("JPI$_ASTCNT over quota = %d, want 0", got)
	}
}

func TestJPIItemsRegistry(t *testing.T) {
	for name := range jpiItemsByName {
		if _, ok := vmsdef.Symbols[name]; !ok {
			t.Errorf("%s is not a $JPIDEF item code", name)
		}
	}

	if jpiCode(t, "JPI$_ACCOUNT") != 515 || jpiCode(t, "JPI$_CLINAME") != 522 {
		t.Error("JPI$_ACCOUNT/CLINAME don't match the codes eVAX's sys_getjpiw used (515/522)")
	}
}
