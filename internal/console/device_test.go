package console

import (
	"strings"
	"testing"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vax"
)

func TestDefineLogicalDebugLogicalsTrace(t *testing.T) {
	c, buf := newTestConsole(t)
	c.CPU.SetDebug(vax.DebugLogicals)

	if err := c.DefineLogical("LNM$FILE_DEV", "FOO", "BAR"); err != nil {
		t.Fatalf("DefineLogical: %v", err)
	}

	if !strings.Contains(buf.String(), `DEFINE/LOGICAL FOO/TABLE=LNM$FILE_DEV "BAR"`) {
		t.Errorf("output = %q, want a DEFINE/LOGICAL trace", buf.String())
	}
}

func TestDefineLogicalNoDebugTraceWhenFlagClear(t *testing.T) {
	c, buf := newTestConsole(t)
	c.CPU.SetDebug(0)

	if err := c.DefineLogical("LNM$FILE_DEV", "FOO", "BAR"); err != nil {
		t.Fatalf("DefineLogical: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("output = %q, want no trace output with DebugLogicals clear", buf.String())
	}
}

func TestDefineLogicalBeforeInitDoesNotPanic(t *testing.T) {
	c := New(nil)
	if err := c.DefineLogical("LNM$FILE_DEV", "FOO", "BAR"); err != nil {
		t.Fatalf("DefineLogical: %v", err)
	}
}

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
	if !strings.Contains(out, "CYLINDERS=512") || !strings.Contains(out, "VOLNAME=SYSTEM") {
		t.Errorf("ShowDevices /FULL output missing detail fields: %q", out)
	}

	buf.Reset()
	if err := c.ShowDevices("NOSUCH", false); err != nil {
		t.Fatalf("ShowDevices NOSUCH: %v", err)
	}
	if buf.String() != "" {
		t.Errorf("ShowDevices for a nonexistent name printed output %q, want none (matches show_device.c: no fallback message)", buf.String())
	}
}

func TestConsoleDefineAndShowLogicals(t *testing.T) {
	c, buf := newTestConsole(t)

	if err := c.DefineLogical("LNM_PROCESS", "MY_LOGICAL", "some value"); err != nil {
		t.Fatalf("DefineLogical: %v", err)
	}

	if err := c.ShowLogicals("LNM_PROCESS", ""); err != nil {
		t.Fatalf("ShowLogicals: %v", err)
	}
	if !strings.Contains(buf.String(), "MY_LOGICAL [LNM_PROCESS]") || !strings.Contains(buf.String(), "some value") {
		t.Errorf("ShowLogicals output = %q, missing the defined name/value", buf.String())
	}

	buf.Reset()
	if err := c.ShowLogicals("NOSUCHTABLE", ""); err != nil {
		t.Fatalf("ShowLogicals NOSUCHTABLE: %v", err)
	}
	if !strings.Contains(buf.String(), "No matching logical names.") {
		t.Errorf("ShowLogicals with no matches = %q, want the no-match message", buf.String())
	}
}

func TestConsoleLogicalsSeededAtConstruction(t *testing.T) {
	c, buf := newTestConsole(t)

	if err := c.ShowLogicals("LNM$FILE_DEV", "SYS$COMMAND"); err != nil {
		t.Fatalf("ShowLogicals: %v", err)
	}
	if !strings.Contains(buf.String(), "TTA0:") {
		t.Errorf("ShowLogicals(LNM$FILE_DEV, SYS$COMMAND) = %q, want it to show the seeded TTA0: value", buf.String())
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

func TestDispatch_defineAndShowLogicalViaDCL(t *testing.T) {
	d, c := newTestDispatcher(t)

	if err := d.Dispatch(`DEFINE/LOGICAL MYNAME "MYVALUE"`); err != nil {
		t.Fatalf("DEFINE/LOGICAL: %v", err)
	}

	// define_logical.c's own default table when /TABLE isn't given.
	ln, ok := c.Logicals.Get("LNM_PROCESS", "MYNAME", 0)
	if !ok || ln.Value != "MYVALUE" {
		t.Errorf("Get(LNM_PROCESS, MYNAME) = %v, %v, want MYVALUE, true", ln, ok)
	}

	if err := d.Dispatch(`DEFINE/LOGICAL/TABLE=MYTABLE OTHERNAME "OTHERVALUE"`); err != nil {
		t.Fatalf("DEFINE/LOGICAL/TABLE: %v", err)
	}
	ln, ok = c.Logicals.Get("MYTABLE", "OTHERNAME", 0)
	if !ok || ln.Value != "OTHERVALUE" {
		t.Errorf("Get(MYTABLE, OTHERNAME) = %v, %v, want OTHERVALUE, true", ln, ok)
	}

	if err := d.Dispatch("SHOW LOGICAL_NAMES"); err != nil {
		t.Fatalf("SHOW LOGICAL_NAMES: %v", err)
	}
}
