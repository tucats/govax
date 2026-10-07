package corevms

import (
	"testing"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vax"
)

// The null device (docs/PHASE-45.md, subtask 11).

func TestNull_qio(t *testing.T) {
	env, out := fixture()
	defineTestDevice(env, "NLA0", iodev.DeviceClassMisc)

	a := newArena(t, env)
	iosb := a.alloc(8)

	// NL: and NLA0: both name it, and a second process may assign it at
	// the same time.
	ch := assignCall(t, env, a, "NL:", uint32(vax.User))
	other := newProcess(t, env)
	assignCall(t, other, newArena(t, other), "NLA0", uint32(vax.User))

	// A write is discarded but counted.
	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnWriteVBlk, iosb: iosb, p: [6]uint32{a.str("gone"), 4}}), ssNormal)

	if st, n, _ := readIOSB(a, iosb); st != uint16(ssNormal) || n != 4 {
		t.Errorf("write IOSB = %d, %d; want SS$_NORMAL, 4", st, n)
	}

	if out.Len() != 0 {
		t.Errorf("the terminal got %q", out.String())
	}

	// A read is at end of file, and leaves the buffer alone.
	buf := a.long(0x55555555)
	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnReadVBlk, iosb: iosb, p: [6]uint32{buf, 4}}), ssNormal)

	if st, n, _ := readIOSB(a, iosb); st != uint16(ssEndOfFile) || n != 0 || a.readLong(buf) != 0x55555555 {
		t.Errorf("read IOSB = %d, %d; want SS$_ENDOFFILE, 0", st, n)
	}

	// A write from memory the process can't read is refused.
	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnWriteVBlk, p: [6]uint32{badAddr, 4}}), ssAccVio)

	// Sense mode reports the class; set mode and a function it doesn't
	// have: success and SS$_ILLIOFUNC.
	sense := a.alloc(8)
	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnSenseMode, iosb: iosb, p: [6]uint32{sense, 8}}), ssNormal)

	if got := a.readLong(sense); byte(got) != byte(iodev.DeviceClassMisc) {
		t.Errorf("sensed class %d, want %d", byte(got), byte(iodev.DeviceClassMisc))
	}

	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnSetMode, p: [6]uint32{sense, 8}}), ssNormal)
	wantR0(t, callQIO(t, env, qioArgs{channel: ch, function: fnReadPrompt}), ssIllIoFunc)
}
