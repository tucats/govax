package rtl

import (
	"strings"
	"testing"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

const testTerminalDevice = "_TTA0:"

func defineTestDevice(env *Environment, name string, class iodev.DeviceClass) *iodev.Device {
	return env.Devices.Define(name, iodev.DeviceOptions{DevClass: class, DevBufSize: 512})
}

func TestServiceSysAssign(t *testing.T) {
	env, _ := fixture()
	defineTestDevice(env, "TTA0", iodev.DeviceClassTT)

	nameAddr, descAddr, chanAddr := uint32(0x1000), uint32(0x1100), uint32(0x1200)
	putDescriptor(t, env, descAddr, nameAddr, "TTA0")

	r0, err := serviceSysAssign(env, []uint32{descAddr, chanAddr})
	if err != nil {
		t.Fatal(err)
	}

	if r0 != ssNormal {
		t.Fatalf("r0 = %d, want ssNormal", r0)
	}

	chanNum, err := env.mem.LoadWord(env.cpu, chanAddr)
	if err != nil {
		t.Fatal(err)
	}

	if chanNum != 8 {
		t.Errorf("channel number = %d, want 8 (first channel, next_channel += 8)", chanNum)
	}

	c, found := env.findChannel(uint32(chanNum))
	if !found {
		t.Fatal("channel not found after SYS$ASSIGN")
	}

	if c.Device.Name != "TTA0" {
		t.Errorf("channel device = %q, want TTA0", c.Device.Name)
	}

	if c.Device.RefCnt != 1 {
		t.Errorf("device RefCnt = %d, want 1", c.Device.RefCnt)
	}

	if c.Device.PID != nominalPID || c.Device.OwnUIC != NominalUIC {
		t.Errorf("device PID/UIC = %#x/%#x, want the process stub's own %#x/%#x",
			c.Device.PID, c.Device.OwnUIC, nominalPID, NominalUIC)
	}
}

func TestServiceSysAssignNoSuchDevice(t *testing.T) {
	env, _ := fixture()
	nameAddr, descAddr, chanAddr := uint32(0x1000), uint32(0x1100), uint32(0x1200)
	putDescriptor(t, env, descAddr, nameAddr, "NOPE")

	r0, err := serviceSysAssign(env, []uint32{descAddr, chanAddr})
	if err != nil {
		t.Fatal(err)
	}

	if r0 != ssIvDevNam {
		t.Errorf("r0 = %d, want ssIvDevNam", r0)
	}
}

func TestServiceSysAssignArgCounts(t *testing.T) {
	env, _ := fixture()
	if r0, err := serviceSysAssign(env, []uint32{1}); err != nil || r0 != ssInsfArg {
		t.Errorf("1 arg: r0=%d err=%v, want ssInsfArg", r0, err)
	}

	if r0, err := serviceSysAssign(env, []uint32{1, 2, 3, 4, 5, 6}); err != nil || r0 != ssTooManyArgs {
		t.Errorf("6 args: r0=%d err=%v, want ssTooManyArgs", r0, err)
	}
}

// TestServiceSysAssignAndGetdviTranslateLogicalNames: $ASSIGN and
// $GETDVIW translate a device name's logical names first, so SYS$OUTPUT
// reaches the terminal, and "_" suppresses that (docs/PHASE-25.md).
func TestServiceSysAssignAndGetdviTranslateLogicalNames(t *testing.T) {
	env, _ := fixture()
	defineTestDevice(env, "TTA0", iodev.DeviceClassTT)
	a := newArena(t, env)

	defineLogical(t, env, "LNM$PROCESS", "LOOP1", lnm.Supervisor, 0, "LOOP2:")
	defineLogical(t, env, "LNM$PROCESS", "LOOP2", lnm.Supervisor, 0, "LOOP1:")

	for _, tt := range []struct {
		name string
		want uint32
	}{
		{"SYS$OUTPUT", ssNormal},
		{"TT:", ssNormal},
		{testTerminalDevice, ssNormal},
		{"_SYS$OUTPUT", ssIvDevNam},
		{"LOOP1:", vmserrors.SS_TOOMANYLNAM},
	} {
		wantR0(t, callLNM(t, env, serviceSysAssign, a.desc(tt.name), a.alloc(2)), tt.want)
	}

	buf := a.alloc(4)
	argv := make([]uint32, 8)
	argv[2] = a.desc("SYS$COMMAND")
	argv[3] = a.items(item{code: dviCode(t, "DVI$_DEVBUFSIZ"), buflen: 4, buf: buf})

	wantR0(t, callLNM(t, env, serviceSysGetdvi, argv...), ssNormal)

	if a.readLong(buf) != 512 {
		t.Errorf("$GETDVIW SYS$COMMAND devbufsiz = %d", a.readLong(buf))
	}
}

// allocCall runs $ALLOC on name with a 16-byte phybuf, returning R0 and
// the physical name returned.
func allocCall(t *testing.T, env *Environment, a *arena, name string, acmode, flags uint32) (uint32, string) {
	t.Helper()

	phylen := a.alloc(2)
	phybuf, buf := a.outDesc(16)
	r0 := callLNM(t, env, serviceSysAlloc, a.desc(name), phylen, phybuf, acmode, flags)

	n, err := env.mem.LoadWord(env.cpu, phylen)
	if err != nil {
		t.Fatal(err)
	}

	return r0, a.readString(buf, n)
}

func TestServiceSysAlloc(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	dp := defineTestDevice(env, "TTA0", iodev.DeviceClassTT)

	r0, phy := allocCall(t, env, a, "TTA0", 0, 0)
	wantR0(t, r0, ssNormal)

	if phy != testTerminalDevice {
		t.Errorf("physical name = %q, want _TTA0:", phy)
	}

	if !dp.Allocated() || dp.PID != env.Process.PID || dp.AllocMode != uint32(vax.Kernel) {
		t.Errorf("device allocated=%v PID=%#x mode=%d, want allocated to %#x in kernel mode (the caller's)",
			dp.Allocated(), dp.PID, dp.AllocMode, env.Process.PID)
	}

	// Allocating it again succeeds with SS$_DEVALRALLOC. A logical name
	// (SYS$OUTPUT translates to _TTA0:) reaches the same device.
	r0, phy = allocCall(t, env, a, "SYS$OUTPUT", 0, 0)
	wantR0(t, r0, ssDevAlrAlloc)

	if phy != testTerminalDevice {
		t.Errorf("physical name via SYS$OUTPUT = %q, want _TTA0:", phy)
	}

	// Only devnam is required.
	dp.Deallocate()
	wantR0(t, callLNM(t, env, serviceSysAlloc, a.desc("TTA0:")), ssNormal)

	if !dp.Allocated() {
		t.Error("device not allocated by a devnam-only call")
	}
}

func TestServiceSysAllocAccessMode(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	user := defineTestDevice(env, "TTA1", iodev.DeviceClassTT)
	kernel := defineTestDevice(env, "TTA2", iodev.DeviceClassTT)

	// acmode is maximized with the caller's mode.
	setCurMod(env, vax.Supervisor)
	r0, _ := allocCall(t, env, a, "TTA2", 0, 0)
	wantR0(t, r0, ssNormal)

	if kernel.AllocMode != uint32(vax.Supervisor) {
		t.Errorf("AllocMode = %d, want supervisor (kernel maximized with the caller's mode)", kernel.AllocMode)
	}

	r0, _ = allocCall(t, env, a, "TTA1", 3, 0)
	wantR0(t, r0, ssNormal)

	// Image rundown deallocates the user-mode allocation only.
	env.ImageRundown()

	if user.Allocated() {
		t.Error("user-mode allocation survived image rundown")
	}

	if !kernel.Allocated() {
		t.Error("supervisor-mode allocation was deallocated by image rundown")
	}
}

func TestServiceSysAllocErrors(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	other := defineTestDevice(env, "TTA1", iodev.DeviceClassTT)
	other.Allocate(0x999, 0)

	defineTestDevice(env, "DUA0", iodev.DeviceClassDisk).DevChar |= devMounted
	defineTestDevice(env, "MBA1", iodev.DeviceClassNone).DevChar |= devMailbox

	cases := []struct {
		name  string
		flags uint32
		want  uint32
	}{
		{"TTA1", 0, ssDevAlloc},
		{"DUA0", 0, ssDevMount},
		{"MBA1", 0, ssDevMount},
		{"NOSUCH0", 0, ssNoSuchDev},
		{"", 0, ssIvLogNam},
		{strings.Repeat("X", 64), 0, ssIvLogNam},
		{"TTA0", 2, ssIvStsFlg},
	}

	for _, c := range cases {
		r0, _ := allocCall(t, env, a, c.name, 0, c.flags)
		if r0 != c.want {
			t.Errorf("$ALLOC(%q, flags %d) = %#x (%v), want %#x (%v)",
				c.name, c.flags, r0, vmserrors.New(r0), c.want, vmserrors.New(c.want))
		}
	}

	wantR0(t, callLNM(t, env, serviceSysAlloc, 0), ssIvDevNam)

	if other.PID != 0x999 {
		t.Errorf("other process's device PID = %#x, want it untouched", other.PID)
	}

	// $ASSIGN also refuses a device allocated to another process.
	wantR0(t, callLNM(t, env, serviceSysAssign, a.desc("TTA1"), a.alloc(2)), ssDevAlloc)
}

func TestServiceSysAllocBufferOverflow(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	dp := defineTestDevice(env, "TTA0", iodev.DeviceClassTT)

	phylen := a.alloc(2)
	phybuf, buf := a.outDesc(3)
	wantR0(t, callLNM(t, env, serviceSysAlloc, a.desc("TTA0"), phylen, phybuf), ssBufferOvf)

	if n, _ := env.mem.LoadWord(env.cpu, phylen); n != 3 || a.readString(buf, 3) != "_TT" {
		t.Errorf("phylen = %d, phybuf = %q, want 3 and the truncated _TT", n, a.readString(buf, 3))
	}

	if !dp.Allocated() {
		t.Error("device not allocated despite SS$_BUFFEROVF (a success status)")
	}
}

func TestServiceSysAllocGeneric(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)

	ra81 := func(name string) *iodev.Device {
		return env.Devices.Define(name, iodev.DeviceOptions{DevClass: iodev.DeviceClassDisk, DevType: 21})
	}

	dua2, dua0, dua1 := ra81("DUA2"), ra81("DUA0"), ra81("DUA1")
	dua0.Allocate(0x999, 0)

	// DUA0 belongs to another process, so DUA1 is the first available.
	r0, phy := allocCall(t, env, a, "RA81", 0, allocGeneric)
	wantR0(t, r0, ssNormal)

	if phy != "_DUA1:" || !dua1.Allocated() {
		t.Errorf("generic RA81 got %q, want _DUA1:", phy)
	}

	r0, phy = allocCall(t, env, a, "ra81", 0, allocGeneric)
	wantR0(t, r0, ssNormal)

	if phy != "_DUA2:" || !dua2.Allocated() {
		t.Errorf("second generic RA81 got %q, want _DUA2:", phy)
	}

	// None left: those this process holds don't count as available.
	r0, _ = allocCall(t, env, a, "RA81", 0, allocGeneric)
	wantR0(t, r0, ssNoDevAvl)

	r0, _ = allocCall(t, env, a, "TU58", 0, allocGeneric)
	wantR0(t, r0, ssNoSuchDev)
}

func TestServiceSysDalloc(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	dp := defineTestDevice(env, "TTA0", iodev.DeviceClassTT)

	r0, _ := allocCall(t, env, a, "TTA0", 0, 0)
	wantR0(t, r0, ssNormal)

	// A logical name reaches the device, as for $ALLOC.
	wantR0(t, callLNM(t, env, serviceSysDalloc, a.desc("SYS$OUTPUT")), ssNormal)

	if dp.Allocated() {
		t.Fatal("TTA0 still allocated after $DALLOC")
	}

	wantR0(t, callLNM(t, env, serviceSysDalloc, a.desc("TTA0")), ssDevNotAlloc)
}

func TestServiceSysDallocErrors(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	kernel := defineTestDevice(env, "TTA1", iodev.DeviceClassTT)
	other := defineTestDevice(env, "TTA2", iodev.DeviceClassTT)
	assigned := defineTestDevice(env, "TTA3", iodev.DeviceClassTT)
	mbx := defineTestDevice(env, "MBA1", iodev.DeviceClassNone)
	mbx.DevChar |= devMailbox

	kernel.Allocate(env.Process.PID, uint32(vax.Kernel))
	other.Allocate(0x999, uint32(vax.User))
	assigned.Allocate(env.Process.PID, uint32(vax.Kernel))
	wantR0(t, callLNM(t, env, serviceSysAssign, a.desc("TTA3"), a.alloc(2)), ssNormal)

	cases := []struct {
		name string
		want uint32
	}{
		{"TTA2", ssDevNotAlloc},
		{"TTA3", ssDevAssign},
		{"MBA1", ssNormal},
		{"NOSUCH0", ssNoSuchDev},
		{"", ssIvLogNam},
		{strings.Repeat("X", 64), ssIvLogNam},
	}

	for _, c := range cases {
		if r0 := callLNM(t, env, serviceSysDalloc, a.desc(c.name)); r0 != c.want {
			t.Errorf("$DALLOC(%q) = %#x (%v), want %#x (%v)", c.name, r0, vmserrors.New(r0), c.want, vmserrors.New(c.want))
		}
	}

	// A kernel-mode allocation can't be released from supervisor mode,
	// even when acmode asks for kernel (it's maximized).
	setCurMod(env, vax.Supervisor)
	wantR0(t, callLNM(t, env, serviceSysDalloc, a.desc("TTA1"), 0), ssNoPriv)

	if !kernel.Allocated() || !other.Allocated() || !assigned.Allocated() {
		t.Error("a failed $DALLOC released a device")
	}
}

func TestServiceSysDallocAll(t *testing.T) {
	env, _ := fixture()
	kernel := defineTestDevice(env, "TTA1", iodev.DeviceClassTT)
	super := defineTestDevice(env, "TTA2", iodev.DeviceClassTT)
	user := defineTestDevice(env, "TTA3", iodev.DeviceClassTT)
	other := defineTestDevice(env, "TTA4", iodev.DeviceClassTT)

	kernel.Allocate(env.Process.PID, uint32(vax.Kernel))
	super.Allocate(env.Process.PID, uint32(vax.Supervisor))
	user.Allocate(env.Process.PID, uint32(vax.User))
	other.Allocate(0x999, uint32(vax.User))

	// No devnam: release everything allocated in supervisor mode or a less
	// privileged one; kernel's and another process's stay.
	wantR0(t, callLNM(t, env, serviceSysDalloc, 0, uint32(vax.Supervisor)), ssNormal)

	if !kernel.Allocated() || super.Allocated() || user.Allocated() || !other.Allocated() {
		t.Errorf("after $DALLOC(acmode=super): kernel=%v super=%v user=%v other=%v, want true false false true", //nolint:dupword
			kernel.Allocated(), super.Allocated(), user.Allocated(), other.Allocated())
	}

	// From kernel mode with the default acmode, the rest of ours goes too.
	wantR0(t, callLNM(t, env, serviceSysDalloc), ssNormal)

	if kernel.Allocated() || !other.Allocated() {
		t.Error("$DALLOC with no arguments didn't release exactly the process's kernel allocation")
	}
}

// assignCall runs $ASSIGN on name with acmode, returning the channel.
func assignCall(t *testing.T, env *Environment, a *arena, name string, acmode uint32) uint32 {
	t.Helper()

	chanAddr := a.alloc(2)
	wantR0(t, callLNM(t, env, serviceSysAssign, a.desc(name), chanAddr, acmode), ssNormal)

	n, err := env.mem.LoadWord(env.cpu, chanAddr)
	if err != nil {
		t.Fatal(err)
	}

	return uint32(n)
}

func TestServiceSysDassgn(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	dp := defineTestDevice(env, "TTA0", iodev.DeviceClassTT)

	c1 := assignCall(t, env, a, "TTA0", 0)
	c2 := assignCall(t, env, a, "TTA0", 0)

	// Only the low word of chan counts.
	wantR0(t, callLNM(t, env, serviceSysDassgn, 0xFFFF0000|c1), ssNormal)

	if _, found := env.findChannel(c1); found || dp.RefCnt != 1 || dp.PID != env.Process.PID {
		t.Errorf("after one $DASSGN: channel found=%v RefCnt=%d PID=%#x, want gone, 1, still owned", found, dp.RefCnt, dp.PID)
	}

	// Releasing the last channel frees the device's owner.
	wantR0(t, callLNM(t, env, serviceSysDassgn, c2), ssNormal)

	if dp.RefCnt != 0 || dp.PID != 0 {
		t.Errorf("after the last $DASSGN: RefCnt=%d PID=%#x, want 0 and 0", dp.RefCnt, dp.PID)
	}

	wantR0(t, callLNM(t, env, serviceSysDassgn, c2), ssNoPriv) // no longer assigned
	wantR0(t, callLNM(t, env, serviceSysDassgn, 0), ssIvChan)
	wantR0(t, callLNM(t, env, serviceSysDassgn, 0x10000), ssIvChan)
}

func TestServiceSysDassgnModes(t *testing.T) {
	env, _ := fixture()
	a := newArena(t, env)
	dp := defineTestDevice(env, "TTA0", iodev.DeviceClassTT)

	kernel := assignCall(t, env, a, "TTA0", 0)
	user := assignCall(t, env, a, "TTA0", 3)

	// A kernel-mode channel can't be released from supervisor mode.
	setCurMod(env, vax.Supervisor)
	wantR0(t, callLNM(t, env, serviceSysDassgn, kernel), ssNoPriv)

	// $ALLOC then $DALLOC: blocked by the channels until they're gone.
	setCurMod(env, vax.Kernel)
	dp.Allocate(env.Process.PID, uint32(vax.User))
	wantR0(t, callLNM(t, env, serviceSysDalloc, a.desc("TTA0")), ssDevAssign)

	// Image rundown deassigns the user-mode channel, but the kernel one
	// still keeps the user-mode allocation in place.
	env.ImageRundown()

	if !dp.Allocated() {
		t.Error("image rundown released an allocation a kernel channel still holds")
	}

	if _, found := env.findChannel(user); found {
		t.Error("user-mode channel survived image rundown")
	}

	if _, found := env.findChannel(kernel); !found {
		t.Error("kernel-mode channel was deassigned by image rundown")
	}

	wantR0(t, callLNM(t, env, serviceSysDassgn, kernel), ssNormal)
	wantR0(t, callLNM(t, env, serviceSysDalloc, a.desc("TTA0")), ssNormal)
}
