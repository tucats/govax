package console

import (
	"strings"
	"testing"

	iodev "github.com/tucats/govax/internal/io"
)

func TestConsoleDefineAndShowDevices(t *testing.T) {
	c, buf := newTestConsole(t)

	c.DefineDevice("DKA0", iodev.DeviceOptions{
		DevClass:  iodev.DeviceClassDisk,
		Cylinders: 512,
		VolName:   "SYSTEM",
	})

	if err := c.ShowDevices("", false); err != nil {
		t.Fatalf("ShowDevices: %v", err)
	}

	if !strings.Contains(buf.String(), "Device DKA0") {
		t.Errorf("ShowDevices output = %q, want it to mention Device DKA0", buf.String())
	}

	buf.Reset()

	if err := c.ShowDevices("DKA0", true); err != nil {
		t.Fatalf("ShowDevices full: %v", err)
	}
	
	out := buf.String()

	if !strings.Contains(out, "Disk DKA0:, is online, file-oriented device.") {
		t.Errorf("ShowDevices /FULL output missing the VMS-style disk header: %q", out)
	}

	if !strings.Contains(out, "Volume label") || !strings.Contains(out, `"SYSTEM"`) {
		t.Errorf("ShowDevices /FULL output missing the (static, unmounted) volume label: %q", out)
	}

	buf.Reset()

	if err := c.ShowDevices("NOSUCH", false); err != nil {
		t.Fatalf("ShowDevices NOSUCH: %v", err)
	}

	if want := "%SYSTEM-W-NOSUCHDEV, no such device available\n"; buf.String() != want {
		t.Errorf("ShowDevices for a nonexistent name printed output %q, want VMS's %q", buf.String(), want)
	}
}

func TestDispatch_defineAndShowDeviceViaDCL(t *testing.T) {
	d, c := newTestDispatcher(t)

	if err := d.Dispatch(`DEFINE/DEVICE DKA0 /DEVCLASS=DISK /CYLINDERS=1024 /VOLNAME="SYSTEM"`); err != nil {
		t.Fatalf("DEFINE/DEVICE: %v", err)
	}

	dev, ok := c.Devices.Find("DKA0")
	if !ok {
		t.Fatalf("device DKA0 not registered")
	}

	if dev.DevClass != iodev.DeviceClassDisk || dev.Cylinders != 1024 || dev.VolName != "SYSTEM" {
		t.Errorf("DKA0 = %+v, unexpected fields", dev)
	}

	if err := d.Dispatch("SHOW DEVICES"); err != nil {
		t.Fatalf("SHOW DEVICES: %v", err)
	}
}


// TestShowDevices_allocated checks SHOW DEVICE/FULL reports a device
// $ALLOC has allocated, and that image rundown releases a user-mode
// allocation (docs/PHASE-26.md).
func TestShowDevices_allocated(t *testing.T) {
	c, buf := newTestConsole(t)

	disk := c.DefineDevice("DKA0", iodev.DeviceOptions{DevClass: iodev.DeviceClassDisk})
	term := c.DefineDevice("TTA1", iodev.DeviceOptions{DevClass: iodev.DeviceClassTT})
	disk.Allocate(c.RTL.Process.PID, 0)
	term.Allocate(c.RTL.Process.PID, 3)

	if err := c.ShowDevices("", true); err != nil {
		t.Fatalf("ShowDevices: %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, "Disk DKA0:, is online, allocated, file-oriented device.") {
		t.Errorf("SHOW DEVICE/FULL output missing the allocated disk header: %q", out)
	}

	if !strings.Contains(out, "Terminal TTA1:, device type unknown, is online, allocated.") {
		t.Errorf("SHOW DEVICE/FULL output missing the allocated terminal: %q", out)
	}

	c.imageRundown()

	if term.Allocated() {
		t.Error("user-mode allocation survived image rundown")
	}

	if !disk.Allocated() {
		t.Error("kernel-mode allocation was released by image rundown")
	}
}

// TestShowDevices_prefix: SHOW DEVICE's name is a prefix, as on VMS: "DU"
// shows every DU device, and a name no device begins with is
// %SYSTEM-W-NOSUCHDEV.
func TestShowDevices_prefix(t *testing.T) {
	c, buf := newTestConsole(t)

	for _, name := range []string{"DUA0", "DUA1", "DKA0"} {
		c.DefineDevice(name, iodev.DeviceOptions{DevClass: iodev.DeviceClassDisk})
	}

	if err := c.ShowDevices("du", false); err != nil {
		t.Fatal(err)
	}

	if out := buf.String(); !strings.Contains(out, "DUA0") || !strings.Contains(out, "DUA1") || strings.Contains(out, "DKA0") {
		t.Errorf("SHOW DEVICE DU: %q, want DUA0 and DUA1 only", out)
	}

	buf.Reset()

	if err := c.ShowDevices("MUA:", false); err != nil {
		t.Fatal(err)
	}

	if out := buf.String(); out != "%SYSTEM-W-NOSUCHDEV, no such device available\n" {
		t.Errorf("SHOW DEVICE MUA: %q, want NOSUCHDEV", out)
	}
}
