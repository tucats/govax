package rtl

import (
	"errors"
	"testing"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// Mailbox function codes, spelled out for the tests.
var (
	fnWriteNow = vmsdef.IOConstants["IO$_WRITEVBLK"] | vmsdef.IOConstants["IO$M_NOW"]
	fnReadNow  = vmsdef.IOConstants["IO$_READVBLK"] | vmsdef.IOConstants["IO$M_NOW"]
	fnWriteEOF = vmsdef.IOConstants["IO$_WRITEOF"]
)

// crembx calls $CREMBX and returns R0 and the channel.
func crembx(t *testing.T, env *Environment, a *arena, prmflg, maxmsg, bufquo uint32, lognam string) (uint32, uint32) {
	t.Helper()

	chanAdr := a.alloc(2)

	name := uint32(0)
	if lognam != "" {
		name = a.desc(lognam)
	}

	r0 := callLNM(t, env, serviceSysCrembx, prmflg, chanAdr, maxmsg, bufquo, 0, 0, name)

	n, err := env.mem.LoadWord(env.cpu, chanAdr)
	if err != nil {
		t.Fatal(err)
	}

	return r0, uint32(n)
}

// mailboxOn returns the mailbox a channel is assigned to.
func mailboxOn(t *testing.T, env *Environment, ch uint32) *Mailbox {
	t.Helper()

	c, ok := env.findChannel(ch)
	if !ok {
		t.Fatalf("no channel %d", ch)
	}

	m, ok := env.Mailboxes.For(c.Device)
	if !ok {
		t.Fatalf("channel %d's %s isn't a mailbox", ch, c.Device.Name)
	}

	return m
}

func TestCrembx(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	r0, ch := crembx(t, env, a, 0, 0, 0, "MYMBX")
	wantR0(t, r0, ssNormal)

	m := mailboxOn(t, env, ch)
	d := m.Device

	if d.Name != "MBA1" || d.DevClass != iodev.DeviceClassMailbox || d.DevType != mailboxDevType || d.RefCnt != 1 {
		t.Errorf("device %s class %d type %d refcnt %d; want MBA1, mailbox, DT$_MBX, 1", d.Name, d.DevClass, d.DevType, d.RefCnt)
	}

	if d.DevChar != mailboxDevChar || d.DevBufSize != defaultMailboxMaxMsg {
		t.Errorf("DEVCHAR %#x, DEVBUFSIZ %d", d.DevChar, d.DevBufSize)
	}

	if m.Permanent || m.MaxMsg != defaultMailboxMaxMsg || m.BufQuo != defaultMailboxBufQuo {
		t.Errorf("mailbox %+v, want temporary with the default sizes", m)
	}

	// The logical name, in LNM$TEMPORARY_MAILBOX (the process table).
	e, err := env.Logicals.Translate("LNM$PROCESS", "MYMBX", lnm.User, 0)
	if err != nil || e.Equivalences[0].Value != "MBA1:" || e.Equivalences[0].Attrs&lnm.AttrTerminal == 0 {
		t.Fatalf("MYMBX = %+v, %v; want \"MBA1:\", terminal", e, err)
	}

	// Creating it again by name assigns a channel to the same mailbox, as
	// does $ASSIGN through the logical name.
	r0, ch2 := crembx(t, env, a, 0, 0, 0, "MYMBX")
	wantR0(t, r0, ssNormal)

	if mailboxOn(t, env, ch2) != m || ch2 == ch {
		t.Error("the second $CREMBX didn't reach the first mailbox on a new channel")
	}

	ch3 := assignCall(t, env, a, "MYMBX", uint32(vax.User))

	if mailboxOn(t, env, ch3) != m || d.RefCnt != 3 {
		t.Errorf("$ASSIGN MYMBX: refcnt %d, want 3", d.RefCnt)
	}

	// A new one without a name is the next unit, with its own sizes.
	r0, ch4 := crembx(t, env, a, 1, 64, 100, "")
	wantR0(t, r0, ssNormal)

	if m4 := mailboxOn(t, env, ch4); m4.Device.Name != "MBA2" || !m4.Permanent || m4.MaxMsg != 64 || m4.BufQuo != 100 {
		t.Errorf("second mailbox %s %+v", m4.Device.Name, m4)
	}
}

func TestCrembx_errors(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	cases := []struct {
		name string
		argv []uint32
		want uint32
	}{
		{"prmflg 2", []uint32{2, a.alloc(2)}, ssIvStsFlg},
		{"bufquo too large", []uint32{0, a.alloc(2), 0, maxMailboxBufQuo + 1}, ssBadParam},
		{"no chan", []uint32{0, 0}, ssAccVio},
		{"unwritable chan", []uint32{0, badAddr}, ssAccVio},
		{"empty name", []uint32{0, a.alloc(2), 0, 0, 0, 0, a.desc("")}, ssIvLogNam},
		{"unreadable name", []uint32{0, a.alloc(2), 0, 0, 0, 0, badAddr}, ssAccVio},
	}

	for _, c := range cases {
		if got := callLNM(t, env, serviceSysCrembx, c.argv...); got != c.want {
			t.Errorf("%s: R0 = %#x, want %#x", c.name, got, c.want)
		}
	}

	if n := len(env.Mailboxes.All()); n != 0 {
		t.Errorf("%d mailboxes left by failed calls", n)
	}
}

func TestMailbox_deletion(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	// A temporary mailbox goes with its last channel, and its name too.
	_, ch := crembx(t, env, a, 0, 0, 0, "TEMP")
	ch2 := assignCall(t, env, a, "TEMP", uint32(vax.User))

	wantR0(t, callLNM(t, env, serviceSysDassgn, ch), ssNormal)

	if _, found := env.Devices.Find("MBA1"); !found {
		t.Fatal("deleted with a channel still assigned")
	}

	wantR0(t, callLNM(t, env, serviceSysDassgn, ch2), ssNormal)

	if _, found := env.Devices.Find("MBA1"); found || len(env.Mailboxes.All()) != 0 {
		t.Error("the temporary mailbox outlived its last channel")
	}

	if _, err := env.Logicals.Translate("LNM$PROCESS", "TEMP", lnm.User, 0); err == nil {
		t.Error("its logical name outlived it")
	}

	// A permanent one stays until $DELMBX, then goes with its last channel.
	_, ch = crembx(t, env, a, 1, 0, 0, "PERM")
	wantR0(t, callLNM(t, env, serviceSysDassgn, ch), ssNormal)

	if _, found := env.Devices.Find("MBA2"); !found {
		t.Fatal("the permanent mailbox went with its channel")
	}

	if _, err := env.Logicals.Translate("LNM$SYSTEM", "PERM", lnm.User, 0); err != nil {
		t.Errorf("PERM not in LNM$PERMANENT_MAILBOX (LNM$SYSTEM): %v", err)
	}

	ch = assignCall(t, env, a, "PERM", uint32(vax.User))
	wantR0(t, callLNM(t, env, serviceSysDelmbx, ch), ssNormal)

	if _, found := env.Devices.Find("MBA2"); !found {
		t.Fatal("$DELMBX deleted it with a channel assigned")
	}

	wantR0(t, callLNM(t, env, serviceSysDassgn, ch), ssNormal)

	if _, found := env.Devices.Find("MBA2"); found {
		t.Error("the marked mailbox outlived its last channel")
	}

	// $DELMBX's errors.
	defineTestDevice(env, "TTA0", iodev.DeviceClassTT)
	tt := assignCall(t, env, a, "TTA0", uint32(vax.Kernel))
	wantR0(t, callLNM(t, env, serviceSysDelmbx, tt), ssDevNotMbx)
	wantR0(t, callLNM(t, env, serviceSysDelmbx, 0), ssIvChan)
	wantR0(t, callLNM(t, env, serviceSysDelmbx, 0x7F0), ssNoPriv)

	setMode(env, vax.User, vax.User, 0x8000)
	wantR0(t, callLNM(t, env, serviceSysDelmbx, tt), ssNoPriv)

	// Image rundown deassigns user channels, deleting a temporary mailbox.
	_, _ = crembx(t, env, a, 0, 0, 0, "")
	env.ImageRundown()

	if len(env.Mailboxes.All()) != 0 {
		t.Error("image rundown kept a user-mode temporary mailbox")
	}
}

// TestMailbox_newEnvironment: an Environment built over a device table
// holding a previous Environment's mailbox removes it.
func TestMailbox_newEnvironment(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	_, _ = crembx(t, env, a, 1, 0, 0, "")

	next := NewEnvironment(env.cpu, env.mem, env.Devices, env.Logicals, env.Mounts, nil, nil)

	if _, found := next.Devices.Find("MBA1"); found {
		t.Error("the old mailbox device survived a new Environment")
	}
}

// mbxQIO is a $QIO on a mailbox channel with a buffer.
func mbxQIO(t *testing.T, env *Environment, efn, ch, fn, iosb, astadr, buf, size uint32) uint32 {
	t.Helper()

	return callQIO(t, env, qioArgs{efn: efn, channel: ch, function: fn, iosb: iosb, astadr: astadr, astprm: efn, p: [6]uint32{buf, size}})
}

func TestMailboxDriver_queuedMessages(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	_, ch := crembx(t, env, a, 0, 16, 40, "")
	m := mailboxOn(t, env, ch)
	iosb := a.alloc(8)

	// Two IO$M_NOW writes complete at once and are queued.
	wantR0(t, mbxQIO(t, env, 0, ch, fnWriteNow, iosb, 0, a.str("hello"), 5), ssNormal)

	if st, n, info := readIOSB(a, iosb); st != ssNormal || n != 5 || info != 0 {
		t.Errorf("write IOSB = %#x, %d, %#x; want SS$_NORMAL, 5, 0", st, n, info)
	}

	wantR0(t, mbxQIO(t, env, 0, ch, fnWriteNow, 0, 0, a.str("a longer one"), 12), ssNormal)
	wantR0(t, mbxQIO(t, env, 0, ch, fnWriteEOF|vmsdef.IOConstants["IO$M_NOW"], 0, 0, 0, 0), ssNormal)

	// Sense mode counts them.
	wantR0(t, mbxQIO(t, env, 0, ch, fnSenseMode, iosb, 0, 0, 0), ssNormal)

	if _, n, info := readIOSB(a, iosb); n != 3 || info != 17 {
		t.Errorf("sense: %d messages, %d bytes; want 3, 17", n, info)
	}

	// Reads take them in order: a whole message, a truncated one, EOF.
	buf := a.alloc(16)
	wantR0(t, mbxQIO(t, env, 0, ch, fnReadVBlk, iosb, 0, buf, 16), ssNormal)

	if st, n, info := readIOSB(a, iosb); st != ssNormal || n != 5 || info != env.Process.PID || a.readString(buf, 5) != "hello" {
		t.Errorf("read 1: %#x, %d, %#x, %q", st, n, info, a.readString(buf, n))
	}

	wantR0(t, mbxQIO(t, env, 0, ch, fnReadVBlk, iosb, 0, buf, 4), ssNormal)

	if st, n, _ := readIOSB(a, iosb); st != uint16(ssBufferOvf) || n != 4 || a.readString(buf, 4) != "a lo" {
		t.Errorf("read 2: %#x, %d, %q; want SS$_BUFFEROVF, 4, \"a lo\"", st, n, a.readString(buf, 4))
	}

	wantR0(t, mbxQIO(t, env, 0, ch, fnReadVBlk, iosb, 0, buf, 16), ssNormal)

	if st, n, _ := readIOSB(a, iosb); st != uint16(ssEndOfFile) || n != 0 {
		t.Errorf("read 3: %#x, %d; want SS$_ENDOFFILE, 0", st, n)
	}

	// Empty, IO$M_NOW: end of file at once.
	wantR0(t, mbxQIO(t, env, 0, ch, fnReadNow, iosb, 0, buf, 16), ssNormal)

	if st, _, _ := readIOSB(a, iosb); st != uint16(ssEndOfFile) || m.Messages() != 0 {
		t.Errorf("empty read NOW: %#x; want SS$_ENDOFFILE", st)
	}

	// Too long for the mailbox: rejected. Past its buffer space: MBFULL.
	wantR0(t, mbxQIO(t, env, 0, ch, fnWriteNow, iosb, 0, a.str("0123456789ABCDEFG"), 17), ssMbTooSml)

	for i := 0; i < 2; i++ {
		wantR0(t, mbxQIO(t, env, 0, ch, fnWriteNow, iosb, 0, a.str("0123456789ABCDEF"), 16), ssNormal)
	}

	wantR0(t, mbxQIO(t, env, 0, ch, fnWriteNow, iosb, 0, a.str("0123456789"), 10), ssNormal)

	if st, _, _ := readIOSB(a, iosb); st != uint16(ssMbFull) || m.Messages() != 2 {
		t.Errorf("third write: %#x with %d messages; want SS$_MBFULL, 2", st, m.Messages())
	}

	// An unwritable read buffer is rejected before anything is taken.
	wantR0(t, mbxQIO(t, env, 0, ch, fnReadVBlk, iosb, 0, badAddr, 16), ssAccVio)

	if m.Messages() != 2 {
		t.Error("a rejected read took a message")
	}
}

func TestMailboxDriver_waitingRequests(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	_, ch := crembx(t, env, a, 0, 0, 0, "")
	riosb, wiosb, buf := a.alloc(8), a.alloc(8), a.alloc(32)

	// A read of an empty mailbox waits: flag clear, IOSB clear.
	wantR0(t, mbxQIO(t, env, 5, ch, fnReadVBlk, riosb, 0x4000, buf, 32), ssNormal)

	if flagSet(env, 5) || a.readLong(riosb) != 0 || env.PendingIO() != 1 {
		t.Fatal("the read didn't wait")
	}

	// A plain write hands its message over: both complete, with flags and
	// ASTs.
	wantR0(t, mbxQIO(t, env, 6, ch, fnWriteVBlk, wiosb, 0x4100, a.str("ping"), 4), ssNormal)

	if st, n, info := readIOSB(a, riosb); st != ssNormal || n != 4 || info != env.Process.PID || a.readString(buf, 4) != "ping" {
		t.Errorf("read: %#x, %d, %#x, %q", st, n, info, a.readString(buf, 4))
	}

	if st, n, info := readIOSB(a, wiosb); st != ssNormal || n != 4 || info != env.Process.PID {
		t.Errorf("write: %#x, %d, %#x", st, n, info)
	}

	if !flagSet(env, 5) || !flagSet(env, 6) || env.PendingASTs() != 2 || env.PendingIO() != 0 {
		t.Errorf("flags %v %v, %d ASTs, %d pending; want set, set, 2, 0", flagSet(env, 5), flagSet(env, 6), env.PendingASTs(), env.PendingIO())
	}

	// A plain write with no reader waits until it's read.
	wantR0(t, mbxQIO(t, env, 6, ch, fnWriteVBlk, wiosb, 0, a.str("pong"), 4), ssNormal)

	if flagSet(env, 6) || a.readLong(wiosb) != 0 {
		t.Fatal("the write didn't wait for a reader")
	}

	wantR0(t, mbxQIO(t, env, 5, ch, fnReadVBlk, riosb, 0, buf, 32), ssNormal)

	if !flagSet(env, 6) || a.readLong(wiosb)&0xFFFF != ssNormal || a.readString(buf, 4) != "pong" {
		t.Error("reading didn't complete the waiting write")
	}
}

func TestMailboxDriver_cancel(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	_, ch := crembx(t, env, a, 1, 0, 0, "")
	ch2 := assignCall(t, env, a, "MBA1", uint32(vax.User))
	m := mailboxOn(t, env, ch)
	riosb, wiosb, buf := a.alloc(8), a.alloc(8), a.alloc(8)

	// $CANCEL completes a waiting read with SS$_CANCEL, and a later write
	// isn't given to it.
	wantR0(t, mbxQIO(t, env, 5, ch, fnReadVBlk, riosb, 0, buf, 8), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysCancel, ch), ssNormal)

	if st, _, _ := readIOSB(a, riosb); st != uint16(ssCancel) || !flagSet(env, 5) {
		t.Errorf("cancelled read: %#x, flag %v; want SS$_CANCEL, set", st, flagSet(env, 5))
	}

	wantR0(t, mbxQIO(t, env, 0, ch2, fnWriteNow, 0, 0, a.str("x"), 1), ssNormal)

	if m.Messages() != 1 {
		t.Errorf("%d messages, want the write queued, not given to the cancelled read", m.Messages())
	}

	// Deassigning a channel cancels its waiting write, and its message.
	wantR0(t, mbxQIO(t, env, 6, ch2, fnWriteVBlk, wiosb, 0, a.str("y"), 1), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysDassgn, ch2), ssNormal)

	if st, _, _ := readIOSB(a, wiosb); st != uint16(ssCancel) || m.Messages() != 1 {
		t.Errorf("deassigned write: %#x with %d messages; want SS$_CANCEL, 1", st, m.Messages())
	}

	// $CANCEL on another channel leaves this one's requests alone.
	wantR0(t, mbxQIO(t, env, 0, ch, fnReadVBlk, riosb, 0, buf, 8), ssNormal)

	if env.PendingIO() != 0 {
		t.Errorf("%d pending, want the read satisfied by the queued message", env.PendingIO())
	}
}

// TestQiow_waits: $QIOW on an empty mailbox waits (ErrWait) until a
// write arrives, then returns SS$_NORMAL; a request that completes at
// once returns at once.
func TestQiow_waits(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	_, ch := crembx(t, env, a, 0, 0, 0, "")
	iosb, buf := a.alloc(8), a.alloc(8)
	env.cpu.SetGPR(vax.FP, 0x7000)

	argv := []uint32{0, ch, fnReadVBlk, iosb, 0, 0, buf, 8}

	for i := 0; i < 2; i++ {
		if _, err := serviceSysQiow(env, argv); !errors.Is(err, ErrWait) {
			t.Fatalf("try %d: err = %v, want ErrWait", i, err)
		}
	}

	if env.PendingIO() != 1 {
		t.Fatalf("%d requests pending, want 1 (the retry mustn't queue another)", env.PendingIO())
	}

	// An AST routine writes (a different frame).
	env.cpu.SetGPR(vax.FP, 0x6000)
	wantR0(t, callLNM(t, env, serviceSysQiow, 0, ch, fnWriteNow, 0, 0, 0, a.str("hi"), 2), ssNormal)

	env.cpu.SetGPR(vax.FP, 0x7000)

	r0, err := serviceSysQiow(env, argv)
	if err != nil || r0 != ssNormal || a.readString(buf, 2) != "hi" || len(env.qiowWaits) != 0 {
		t.Errorf("after the write: %#x, %v, %q, %d waits", r0, err, a.readString(buf, 2), len(env.qiowWaits))
	}
}
