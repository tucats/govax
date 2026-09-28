package rtl

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

func syiCode(t *testing.T, name string) uint16 {
	t.Helper()

	code, ok := vmsdef.SYIConstants[name]
	if !ok {
		t.Fatalf("no $SYIDEF code %s", name)
	}

	return uint16(code)
}

// getsyi calls $GETSYIW with a 7-argument list and no AST.
func getsyi(t *testing.T, env *Environment, efn, csidadr, nodename, itmlst, iosb uint32) uint32 {
	t.Helper()

	return callLNM(t, env, serviceSysGetsyi, efn, csidadr, nodename, itmlst, iosb, 0, 0)
}

func TestServiceSysGetsyiItems(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	env.cpu.SetPR(vax.SID, 0x01234567)
	env.BootTime = 0x0123456789ABCDEF

	wants := []struct {
		name string
		want string // the bytes returned
	}{
		{"SYI$_VERSION", "V7.3    "},
		{"SYI$_NODE_SWVERS", "V7.3"},
		{"SYI$_NODE_SWTYPE", "VMS "},
		{"SYI$_NODENAME", "GOVAX"},
		{"SYI$_SID", "\x67\x45\x23\x01"},
		{"SYI$_CPU", "\x01\x00\x00\x00"},
		{"SYI$_BOOTTIME", "\xEF\xCD\xAB\x89\x67\x45\x23\x01"},
		{"SYI$_CLUSTER_MEMBER", "\x00"},
		{"SYI$_NODE_CSID", "\x00\x00\x00\x00"},
		{"SYI$_MINWSCNT", string([]byte{byte(env.Process.MinWSCount), 0, 0, 0})},
	}

	items := make([]item, len(wants))
	for i, w := range wants {
		items[i] = item{code: syiCode(t, w.name), buflen: 16, buf: a.alloc(16), ret: a.alloc(2)}
	}

	iosb := a.alloc(8)
	wantR0(t, getsyi(t, env, 0, 0, 0, a.items(items...), iosb), ssNormal)

	for i, w := range wants {
		n, err := env.mem.LoadWord(env.cpu, items[i].ret)
		if err != nil {
			t.Fatal(err)
		}

		if got := a.readString(items[i].buf, n); got != w.want {
			t.Errorf("%s = %q, want %q", w.name, got, w.want)
		}
	}

	if got := a.readLong(iosb); got != ssNormal {
		t.Errorf("IOSB status = %#x, want SS$_NORMAL", got)
	}

	// A short buffer gets the start of the value: a 4-byte SYI$_VERSION.
	buf, ret := a.alloc(4), a.alloc(2)
	wantR0(t, getsyi(t, env, 0, 0, 0, a.items(item{code: syiCode(t, "SYI$_VERSION"), buflen: 4, buf: buf, ret: ret}), 0), ssNormal)

	if got := a.readString(buf, 4); got != "V7.3" {
		t.Errorf("SYI$_VERSION in 4 bytes = %q, want \"V7.3\"", got)
	}
}

func TestServiceSysGetsyiNodeSelection(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	itmlst := a.items(item{code: syiCode(t, "SYI$_NODENAME"), buflen: 16, buf: a.alloc(16)})

	// This node, by CSID (0, written back) or by name.
	csid := a.long(0)
	wantR0(t, getsyi(t, env, 0, csid, 0, itmlst, 0), ssNormal)

	if got := a.readLong(csid); got != 0 {
		t.Errorf("csidadr after $GETSYI = %#x, want this node's CSID, 0", got)
	}

	wantR0(t, getsyi(t, env, 0, 0, a.desc("GOVAX"), itmlst, 0), ssNormal)
	wantR0(t, getsyi(t, env, 0, csid, a.desc("GOVAX"), itmlst, 0), ssNormal)

	// A wildcard scan: this node, then SS$_NOMORENODE.
	wild := a.long(0xFFFFFFFF)
	wantR0(t, getsyi(t, env, 0, wild, 0, itmlst, 0), ssNormal)
	wantR0(t, getsyi(t, env, 0, wild, 0, itmlst, 0), ssNoMoreNode)

	// Other nodes and bad arguments.
	wantR0(t, getsyi(t, env, 0, a.long(5), 0, itmlst, 0), ssNoSuchNode)
	wantR0(t, getsyi(t, env, 0, 0, a.desc("OTHER"), itmlst, 0), ssNoSuchNode)
	wantR0(t, getsyi(t, env, 0, 0, a.desc("GOVAX "), itmlst, 0), ssNoSuchNode) // no trailing blanks
	wantR0(t, getsyi(t, env, 0, 0, a.desc(""), itmlst, 0), ssIvLogNam)
	wantR0(t, getsyi(t, env, 0, 0, a.desc("SIXTEEN_CHARS_XX"), itmlst, 0), ssIvLogNam)
	wantR0(t, getsyi(t, env, 0, badAddr, 0, itmlst, 0), ssAccVio)
	wantR0(t, getsyi(t, env, 0, a.long(5), a.desc("GOVAX"), itmlst, 0), ssNoSuchNode) // must agree
}

func TestServiceSysGetsyiCompletion(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	iosb := a.alloc(8)
	itmlst := a.items(item{code: syiCode(t, "SYI$_NODENAME"), buflen: 16, buf: a.alloc(16)})

	// Success: the flag is set, the IOSB written, and the AST queued in
	// the caller's mode with astprm.
	setCurMod(env, vax.Supervisor)
	env.Process.LocalEventFlags[0] = 0
	wantR0(t, callLNM(t, env, serviceSysGetsyi, 6, 0, 0, itmlst, iosb, 0x4000, 99), ssNormal)

	if !flagSet(env, 6) || a.readLong(iosb) != ssNormal {
		t.Errorf("flag 6 set=%v, IOSB=%#x; want set and SS$_NORMAL", flagSet(env, 6), a.readLong(iosb))
	}

	q := env.Process.ast.queue
	if len(q) != 1 || q[0].routine != 0x4000 || q[0].param != 99 || q[0].mode != uint32(vax.Supervisor) {
		t.Errorf("AST queue = %+v, want one supervisor-mode AST at 0x4000 with 99", q)
	}

	// An unsupported item still completes, with SS$_BADPARAM.
	env.Process.ast.queue = nil
	env.Process.LocalEventFlags[0] = 0
	bad := a.items(item{code: syiCode(t, "SYI$_MAXBUF"), buflen: 4, buf: a.alloc(4)})
	wantR0(t, callLNM(t, env, serviceSysGetsyi, 6, 0, 0, bad, iosb, 0x4000, 99), ssBadParam)

	if !flagSet(env, 6) || a.readLong(iosb) != ssBadParam || env.PendingASTs() != 1 {
		t.Errorf("after SS$_BADPARAM: flag %v, IOSB %#x, %d ASTs; want the request completed", flagSet(env, 6), a.readLong(iosb), env.PendingASTs())
	}

	// Rejected before starting: nothing completes.
	env.Process.ast.queue = nil
	env.Process.LocalEventFlags[0] = 0
	wantR0(t, callLNM(t, env, serviceSysGetsyi, 6, a.long(5), 0, itmlst, iosb, 0x4000, 99), ssNoSuchNode)

	if flagSet(env, 6) || env.PendingASTs() != 0 {
		t.Error("a rejected request set its flag or queued its AST")
	}

	wantR0(t, callLNM(t, env, serviceSysGetsyi, 6, 0, 0), ssInsfArg)
	wantR0(t, callLNM(t, env, serviceSysGetsyi, 200, 0, 0, itmlst), ssIllEfc)
	wantR0(t, callLNM(t, env, serviceSysGetsyi, 6, 0, 0, itmlst, badAddr), ssAccVio)

	// The optional arguments may be left off entirely.
	wantR0(t, callLNM(t, env, serviceSysGetsyi, 0, 0, 0, itmlst), ssNormal)
}

func TestSYIItemsRegistry(t *testing.T) {
	for name := range syiItemsByName {
		if _, ok := vmsdef.SYIConstants[name]; !ok {
			t.Errorf("%s is not a $SYIDEF item code", name)
		}
	}

	// Two codes from the VMS manual's listing, as a spot check of the
	// generated table.
	if syiCode(t, "SYI$_VERSION") != 4096 || syiCode(t, "SYI$_CPU") != 8192 {
		t.Error("SYI$_VERSION/CPU don't have their $SYIDEF codes (4096/8192)")
	}
}
