package rtl

import (
	"bytes"
	"strings"
	"testing"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vax"
)

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
	if c.Device.PID != nominalPID || c.Device.OwnUIC != nominalUIC {
		t.Errorf("device PID/UIC = %#x/%#x, want the process stub's own %#x/%#x",
			c.Device.PID, c.Device.OwnUIC, nominalPID, nominalUIC)
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

func TestServiceSysGetdviwByChannel(t *testing.T) {
	env, _ := fixture()
	dp := defineTestDevice(env, "DKA0", iodev.DeviceClassDisk)
	c := &channel{Name: "DKA0", Number: 8, Device: dp}
	env.channels = append(env.channels, c)

	itemList, buf := uint32(0x2000), uint32(0x3000)
	putWord(t, env, itemList, 1)
	putWord(t, env, itemList+2, dviDevClass)
	putLongword(t, env, itemList+4, buf)
	putLongword(t, env, itemList+8, 0)
	putLongword(t, env, itemList+12, 0)

	argv := make([]uint32, 8)
	argv[1] = 8 // channel number
	argv[3] = itemList

	r0, err := serviceSysGetdviw(env, argv)
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssNormal {
		t.Fatalf("r0 = %d, want ssNormal", r0)
	}
	class, err := env.mem.LoadByte(env.cpu, buf)
	if err != nil {
		t.Fatal(err)
	}
	if iodev.DeviceClass(class) != iodev.DeviceClassDisk {
		t.Errorf("class = %d, want DeviceClassDisk", class)
	}
}

func TestServiceSysGetdviwDebugDevicesTrace(t *testing.T) {
	env, _ := fixture()
	dp := defineTestDevice(env, "DKA0", iodev.DeviceClassDisk)
	c := &channel{Name: "DKA0", Number: 8, Device: dp}
	env.channels = append(env.channels, c)

	itemList, buf := uint32(0x2000), uint32(0x3000)
	putWord(t, env, itemList, 1)
	putWord(t, env, itemList+2, dviDevClass)
	putLongword(t, env, itemList+4, buf)
	putLongword(t, env, itemList+8, 0)
	putLongword(t, env, itemList+12, 0)

	argv := make([]uint32, 8)
	argv[1] = 8
	argv[3] = itemList

	var traceBuf bytes.Buffer
	env.cpu.SetDebugWriter(&traceBuf)
	env.cpu.SetDebug(vax.DebugDevices)

	if _, err := serviceSysGetdviw(env, argv); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(traceBuf.String(), "DEBUG: SYS$GETDVIW looks up device DKA0") {
		t.Errorf("output = %q, want a SYS$GETDVIW lookup trace", traceBuf.String())
	}
}

func TestServiceSysGetdviwByName(t *testing.T) {
	env, _ := fixture()
	defineTestDevice(env, "MUA0", iodev.DeviceClassDisk)

	nameAddr, descAddr := uint32(0x1000), uint32(0x1100)
	putDescriptor(t, env, descAddr, nameAddr, "MUA0")

	itemList, buf := uint32(0x2000), uint32(0x3000)
	putWord(t, env, itemList, 4)
	putWord(t, env, itemList+2, dviDevBufSize)
	putLongword(t, env, itemList+4, buf)
	putLongword(t, env, itemList+8, 0)
	putLongword(t, env, itemList+12, 0)

	argv := make([]uint32, 8)
	argv[2] = descAddr
	argv[3] = itemList

	r0, err := serviceSysGetdviw(env, argv)
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssNormal {
		t.Fatalf("r0 = %d, want ssNormal", r0)
	}
	bufSize, err := env.mem.LoadLongword(env.cpu, buf)
	if err != nil {
		t.Fatal(err)
	}
	if bufSize != 512 {
		t.Errorf("devbufsiz = %d, want 512", bufSize)
	}
}

func TestServiceSysGetdviwInvalidChannel(t *testing.T) {
	env, _ := fixture()
	argv := make([]uint32, 8)
	argv[1] = 42
	r0, err := serviceSysGetdviw(env, argv)
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssIvChan {
		t.Errorf("r0 = %d, want ssIvChan", r0)
	}
}
