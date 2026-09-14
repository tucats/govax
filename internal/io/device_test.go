package io

import "testing"

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
