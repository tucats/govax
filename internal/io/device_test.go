package io

import (
	"strings"
	"testing"
)

func TestDeviceClassName(t *testing.T) {
	cases := []struct {
		class DeviceClass
		want  string
	}{
		{DeviceClassNone, "none"},
		{DeviceClassDisk, "disk"},
		{DeviceClassTT, "terminal"},
		{DeviceClass(200), "<unknown>"},
	}
	for _, c := range cases {
		if got := DeviceClassName(c.class); got != c.want {
			t.Errorf("DeviceClassName(%d) = %q, want %q", c.class, got, c.want)
		}
	}
}

func TestDeviceTableDefineAndFind(t *testing.T) {
	dt := NewDeviceTable()

	dt.Define("TTA0", DeviceOptions{DevClass: DeviceClassTT})
	dt.Define("DKA0", DeviceOptions{
		DevClass:  DeviceClassDisk,
		Cylinders: 1024,
		VolName:   "SYSTEM",
	})

	d, ok := dt.Find("dka0")
	if !ok {
		t.Fatalf("Find(dka0): not found")
	}

	if d.Cylinders != 1024 || d.VolName != "SYSTEM" || d.DevClass != DeviceClassDisk {
		t.Errorf("Find(dka0) = %+v, unexpected fields", d)
	}

	// Trailing ':' must be stripped, matching find_device.
	if _, ok := dt.Find("TTA0:"); !ok {
		t.Errorf("Find(TTA0:) not found")
	}

	if _, ok := dt.Find("NOSUCH"); ok {
		t.Errorf("Find(NOSUCH) unexpectedly found")
	}
}

func TestDeviceTableDefineDuplicateNameShadowsOlder(t *testing.T) {
	dt := NewDeviceTable()
	dt.Define("DKA0", DeviceOptions{Cylinders: 100})
	dt.Define("DKA0", DeviceOptions{Cylinders: 200})

	d, ok := dt.Find("DKA0")
	if !ok {
		t.Fatalf("Find(DKA0): not found")
	}

	if d.Cylinders != 200 {
		t.Errorf("Find(DKA0).Cylinders = %d, want 200 (most recent definition should win)", d.Cylinders)
	}

	if len(dt.All()) != 2 {
		t.Errorf("All() len = %d, want 2 (both definitions kept, not replaced)", len(dt.All()))
	}
}

func TestDeviceTableAllOrder(t *testing.T) {
	dt := NewDeviceTable()
	dt.Define("FIRST", DeviceOptions{})
	dt.Define("SECOND", DeviceOptions{})

	all := dt.All()
	if len(all) != 2 || all[0].Name != "SECOND" || all[1].Name != "FIRST" {
		t.Errorf("All() = %v, want [SECOND, FIRST] (most-recently-Defined first)", names(all))
	}
}

func names(ds []*Device) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Name
	}
	
	return out
}

func TestDeviceTableRemove(t *testing.T) {
	tab := NewDeviceTable()
	a := tab.Define("MBA1", DeviceOptions{DevClass: DeviceClassMailbox})
	b := tab.Define("MBA2", DeviceOptions{DevClass: DeviceClassMailbox})

	if !tab.Remove(a) || tab.Remove(a) {
		t.Fatal("Remove: want true once, then false")
	}

	if _, found := tab.Find("MBA1"); found {
		t.Error("MBA1 still found")
	}

	if d, found := tab.Find("MBA2"); !found || d != b {
		t.Error("MBA2 lost")
	}

	if got := DeviceClassName(DeviceClassMailbox); got != "mailbox" {
		t.Errorf("DeviceClassName(mailbox) = %q", got)
	}
}

// TestSortedOrdersByClassThenUnit checks that SHOW DEVICE's order groups
// devices by class and puts units in numeric order, whatever order they
// were defined in.
func TestSortedOrdersByClassThenUnit(t *testing.T) {
	table := NewDeviceTable()

	for _, name := range []string{"TTA0", "DUA2", "MBA10", "DUA10", "NLA0", "DUA1", "MBA2", "DUA0", "DUB0", "OPA0"} {
		table.Define(name, DeviceOptions{})
	}

	var got []string
	for _, d := range table.Sorted() {
		got = append(got, d.Name)
	}

	want := []string{"DUA0", "DUA1", "DUA2", "DUA10", "DUB0", "MBA2", "MBA10", "NLA0", "OPA0", "TTA0"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("Sorted() = %v, want %v", got, want)
	}
}
