package corevms

import (
	"encoding/binary"
	"testing"

	"github.com/tucats/govax/internal/lck"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// lkiCode is the item code named name.
func lkiCode(name string) uint16 { return uint16(vmsdef.Symbols[name]) }

// memAt reads n bytes of env's memory at addr.
func memAt(t *testing.T, env *Environment, addr, n uint32) []byte {
	t.Helper()

	b := make([]byte, n)
	if err := env.mem.Load(env.cpu, addr, b); err != nil {
		t.Fatal(err)
	}

	return b
}

// getlki calls $GETLKIW for c's process on the lock ID at lkidadr with
// one item (buffer size n), returning the status, the item's bytes, and
// its return length longword.
func (c *lockCaller) getlki(lkidadr uint32, code uint16, n uint16) (uint32, []byte, uint32) {
	c.t.Helper()

	buf, ret := c.a.alloc(uint32(n)), c.a.alloc(4)
	list := c.a.items(item{code: code, buflen: n, buf: buf, ret: ret})
	iosb := c.a.alloc(8)

	r0 := callLNM(c.t, c.env, serviceSysGetlki, 0, lkidadr, list, iosb, 0, 0, 0)
	if got := binary.LittleEndian.Uint32(memAt(c.t, c.env, iosb, 4)); got != r0 {
		c.t.Errorf("IOSB status %#x, R0 %#x", got, r0)
	}

	retlen := binary.LittleEndian.Uint32(memAt(c.t, c.env, ret, 4))

	return r0, memAt(c.t, c.env, buf, uint32(retlen&0xFFFF)), retlen
}

// TestGetlki_items: a lock's state, owner, name, queues, and the locks
// it blocks and is blocked by.
func TestGetlki_items(t *testing.T) {
	a, b := newLockPair(t)

	wantR0(t, a.enq("RES", lck.EX, 0), ssNormal)
	wantR0(t, b.enq("RES", lck.PR, 0), ssNormal) // waits for A's
	aID, bID := a.a.long(a.id()), b.a.long(b.id())

	st, data, _ := a.getlki(aID, lkiCode("LKI$_PID"), 4)
	if st != ssNormal || binary.LittleEndian.Uint32(data) != a.env.Process.PID {
		t.Errorf("LKI$_PID: %#x, %x", st, data)
	}

	if _, data, _ := b.getlki(bID, lkiCode("LKI$_STATE"), 3); string(data) != "\x03\x00\xff" {
		t.Errorf("B's LKI$_STATE %x, want PR requested, none granted, waiting", data)
	}

	if _, data, _ := a.getlki(aID, lkiCode("LKI$_STATE"), 3); string(data) != "\x05\x05\x01" {
		t.Errorf("A's LKI$_STATE %x, want EX, EX, granted", data)
	}

	if _, data, _ := a.getlki(aID, lkiCode("LKI$_RESNAM"), 31); string(data) != "RES" {
		t.Errorf("LKI$_RESNAM %q", data)
	}

	for name, want := range map[string]uint32{"LKI$_GRANTCOUNT": 1, "LKI$_CVTCOUNT": 0, "LKI$_WAITCOUNT": 1} {
		if _, data, _ := a.getlki(aID, lkiCode(name), 4); binary.LittleEndian.Uint32(data) != want {
			t.Errorf("%s = %x, want %d", name, data, want)
		}
	}

	_, data, retlen := a.getlki(aID, lkiCode("LKI$_BLOCKEDBY"), 100)
	if retlen != 24|24<<16 || binary.LittleEndian.Uint32(data[4:]) != b.env.Process.PID || data[12] != 3 || data[14] != 0xff {
		t.Errorf("A's LKI$_BLOCKEDBY: return length %#x, %x", retlen, data)
	}

	_, data, retlen = b.getlki(bID, lkiCode("LKI$_BLOCKING"), 100)
	if retlen != 24|24<<16 || binary.LittleEndian.Uint32(data) != a.id() || data[13] != 5 || data[14] != 1 {
		t.Errorf("B's LKI$_BLOCKING: return length %#x, %x", retlen, data)
	}

	// Too small a buffer for one entry: none written, bit 31 set.
	if st, _, retlen := a.getlki(aID, lkiCode("LKI$_LOCKS"), 30); st != ssNormal || retlen != 24|24<<16|1<<31 {
		t.Errorf("LKI$_LOCKS in 30 bytes: %#x, return length %#x", st, retlen)
	}

	if st, _, _ := a.getlki(a.a.long(0x7777), lkiCode("LKI$_PID"), 4); st != ssIvLockID {
		t.Errorf("an unknown lock ID: %#x, want SS$_IVLOCKID", st)
	}

	if st, _, _ := a.getlki(aID, 0x7FFF, 4); st != ssBadParam {
		t.Errorf("an unknown item: %#x, want SS$_BADPARAM", st)
	}
}

// TestGetlki_access: a wildcard scan returns each lock the caller may
// look at, then SS$_NOMORELOCK; a lock of a more privileged access mode
// is SS$_IVMODE, a system-wide one without SYSLCK SS$_NOSYSLCK, another
// group's without WORLD SS$_NOWORLD; the scan skips them.
func TestGetlki_access(t *testing.T) {
	a, b := newLockPair(t)

	// Unprivileged user-mode callers.
	setMode(a.env, vax.User, vax.User, a.env.cpu.GPR(vax.SP))
	a.env.Process.CurrentPrivileges, b.env.Process.CurrentPrivileges = 0, 0

	wantR0(t, a.enq("ONE", lck.EX, 0), ssNormal)
	wantR0(t, b.enq("TWO", lck.EX, 0), ssNormal)

	exec, _, err := a.env.Locks.Enqueue(lck.Request{Owner: lck.Owner(a.env.Process.PID), Mode: lck.NL, Name: "EXEC", Group: 1, AccessMode: 1})
	if err != nil {
		t.Fatal(err)
	}

	system, _, err := a.env.Locks.Enqueue(lck.Request{Owner: lck.Owner(a.env.Process.PID), Mode: lck.NL, Name: "SYS", AccessMode: 3})
	if err != nil {
		t.Fatal(err)
	}

	pid := lkiCode("LKI$_PID")

	if st, _, _ := a.getlki(a.a.long(uint32(exec.ID)), pid, 4); st != ssIvMode {
		t.Errorf("an executive-mode lock: %#x, want SS$_IVMODE", st)
	}

	if st, _, _ := a.getlki(a.a.long(uint32(system.ID)), pid, 4); st != ssNoSysLck {
		t.Errorf("a system-wide lock: %#x, want SS$_NOSYSLCK", st)
	}

	scan := func(c *lockCaller) []uint32 {
		ctx := c.a.long(0xFFFFFFFF)

		var pids []uint32

		for range 10 {
			st, data, _ := c.getlki(ctx, pid, 4)
			if st == ssNoMoreLock {
				return pids
			}

			if st != ssNormal {
				t.Fatalf("scan: %#x", st)
			}

			pids = append(pids, binary.LittleEndian.Uint32(data))
		}

		t.Fatal("the scan didn't end")

		return nil
	}

	if got := scan(a); len(got) != 2 || got[0] != a.env.Process.PID || got[1] != b.env.Process.PID {
		t.Errorf("A's scan: %x, want A's lock and B's", got)
	}

	b.env.Process.UIC = (b.env.Process.UICGroup()+1)<<16 | 1

	if st, _, _ := b.getlki(b.a.long(a.id()), pid, 4); st != ssNoWorld {
		t.Errorf("another group's lock: %#x, want SS$_NOWORLD", st)
	}

	if got := scan(b); len(got) != 1 || got[0] != b.env.Process.PID {
		t.Errorf("B's scan from another group: %x, want its own lock", got)
	}
}
