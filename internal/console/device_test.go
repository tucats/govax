package console

import (
	"strings"
	"testing"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vax"
)

// wantLogical fails t unless name translates, in table, to value.
func wantLogical(t *testing.T, c *Console, table, name, value string) {
	t.Helper()

	e, err := c.Logicals.Translate(table, name, lnm.User, 0)
	if err != nil {
		t.Fatalf("Translate(%s, %s): %v", table, name, err)
	}

	if got := e.Equivalences[0].Value; got != value {
		t.Errorf("%s in %s = %q, want %q", name, table, got, value)
	}
}

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

	if buf.String() != "" {
		t.Errorf("ShowDevices for a nonexistent name printed output %q, want none (matches show_device.c: no fallback message)", buf.String())
	}
}

func TestConsoleDefineAndShowLogicals(t *testing.T) {
	c, buf := newTestConsole(t)

	if err := c.DefineLogical("LNM$PROCESS", "MY_LOGICAL", "some value"); err != nil {
		t.Fatalf("DefineLogical: %v", err)
	}

	if err := c.ShowLogicals("LNM$PROCESS", ""); err != nil {
		t.Fatalf("ShowLogicals: %v", err)
	}

	if !strings.Contains(buf.String(), "MY_LOGICAL [LNM$PROCESS_TABLE]") || !strings.Contains(buf.String(), "some value") {
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

	// The process table is the default when /TABLE isn't given.
	wantLogical(t, c, "LNM$PROCESS_TABLE", "MYNAME", "MYVALUE")

	// /TABLE names an existing table; unlike eVAX, DEFINE doesn't create
	// one.
	if err := d.Dispatch(`DEFINE/LOGICAL/TABLE=MYTABLE OTHERNAME "OTHERVALUE"`); err == nil {
		t.Errorf("DEFINE/LOGICAL/TABLE=MYTABLE with no such table succeeded")
	}

	if _, _, err := c.Logicals.CreateTable("MYTABLE", lnm.ProcessTableName, lnm.Supervisor, 0); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}

	if err := d.Dispatch(`DEFINE/LOGICAL/TABLE=MYTABLE OTHERNAME "OTHERVALUE"`); err != nil {
		t.Fatalf("DEFINE/LOGICAL/TABLE: %v", err)
	}

	wantLogical(t, c, "MYTABLE", "OTHERNAME", "OTHERVALUE")

	if err := d.Dispatch("SHOW LOGICAL_NAMES"); err != nil {
		t.Fatalf("SHOW LOGICAL_NAMES: %v", err)
	}
}

func TestConsoleLogicalsTrace(t *testing.T) {
	c, buf := newTestConsole(t)
	c.CPU.SetDebug(vax.DebugLogicals)

	if err := c.DefineLogical("LNM$PROCESS", "FOO", "BAR"); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "DEBUG: LNM: define FOO [super] in LNM$PROCESS_TABLE") {
		t.Errorf("output = %q, want the database's own define trace", buf.String())
	}
}

func TestConsoleShowLogicalsSearchList(t *testing.T) {
	c, buf := newTestConsole(t)

	eqv := []lnm.Equivalence{{Value: "A:"}, {Value: "B:"}}
	if _, err := c.Logicals.Define("LNM$PROCESS", "LIST", lnm.Supervisor, 0, eqv); err != nil {
		t.Fatal(err)
	}

	if err := c.ShowLogicals("", "LIST"); err != nil {
		t.Fatal(err)
	}

	if got, want := buf.String(), "LIST [LNM$PROCESS_TABLE] = \"A:\"\n    = \"B:\"\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
