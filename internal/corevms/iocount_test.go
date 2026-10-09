package corevms

import (
	"encoding/binary"
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// TestIOCount_terminal: each terminal $QIO that completes is a buffered
// I/O, a cancelled one too; one $QIO rejects isn't; $GETJPI's
// JPI$_BUFIO and the termination message report the count.
func TestIOCount_terminal(t *testing.T) {
	env, _, a, ch := qioFixture(t, "")
	iosb := a.alloc(8)

	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnWriteVBlk, iosb: iosb, p: [6]uint32{a.str("hi"), 2}}), ssNormal)
	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnWriteVBlk, iosb: iosb, p: [6]uint32{badAddr, 2}}), ssAccVio)

	if env.Process.BufferedIO != 1 || env.Process.DirectIO != 0 {
		t.Fatalf("after a write and a rejected one: buffered %d, direct %d; want 1, 0", env.Process.BufferedIO, env.Process.DirectIO)
	}

	// A pending read, cancelled.
	withScheduler(env)
	env.consoleIn = &scriptedTerminal{}

	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnReadVBlk, iosb: iosb, p: [6]uint32{a.alloc(10), 10}}), ssNormal)

	if env.Process.BufferedIO != 1 {
		t.Errorf("a pending read counted before it completed: %d", env.Process.BufferedIO)
	}

	wantR0(t, callLNM(t, env, serviceSysCancel, ch), ssNormal)

	if env.Process.BufferedIO != 2 {
		t.Errorf("after the cancelled read: buffered %d, want 2", env.Process.BufferedIO)
	}

	if v := jpiItemsByName["JPI$_BUFIO"](env).data; binary.LittleEndian.Uint32([]byte(v)) != 2 {
		t.Errorf("JPI$_BUFIO %x, want 2", v)
	}

	msg := env.terminationMessage(env)
	if got := binary.LittleEndian.Uint32(msg[vmsdef.Symbols["ACC$L_BIOCNT"]:]); got != 2 {
		t.Errorf("ACC$L_BIOCNT %d, want 2", got)
	}
}

// TestIOCount_disk: a disk $QIO is a direct I/O.
func TestIOCount_disk(t *testing.T) {
	env, a, ch := diskFixture(t, true)
	fibd, _ := newFIB(a, 64, 0)
	resd, _ := a.outDesc(40)

	diskQIO(t, env, a, ch, fnAccessOp, fibd, a.desc("data.bin"), a.alloc(2), resd)
	diskQIO(t, env, a, ch, fnReadVBlk, a.alloc(1024), 1024, 2)

	if env.Process.DirectIO != 2 || env.Process.BufferedIO != 0 {
		t.Errorf("buffered %d, direct %d; want 0, 2", env.Process.BufferedIO, env.Process.DirectIO)
	}

	if v := jpiItemsByName["JPI$_DIRIO"](env).data; binary.LittleEndian.Uint32([]byte(v)) != 2 {
		t.Errorf("JPI$_DIRIO %x, want 2", v)
	}
}

// TestIOCount_volume: an RMS service's block operations on a mounted
// volume are the caller's direct I/O.
func TestIOCount_volume(t *testing.T) {
	env, _, _ := diskFixture(t, true)

	before := env.Process.DirectIO
	ops := env.Mounts.TotalOperations()

	env.countVolumeIO(func() {
		vol, _ := env.Mounts.Lookup("DUA0")
		vol.Devices[0].CountOperation(3) // as three block reads would
	})

	if done := env.Mounts.TotalOperations() - ops; done == 0 || env.Process.DirectIO-before != uint32(done) {
		t.Errorf("%d block operations, direct I/O %d more; want them counted", done, env.Process.DirectIO-before)
	}
}
