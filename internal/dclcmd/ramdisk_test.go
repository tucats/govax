package dclcmd

import "testing"

// TestParseRAMDiskBlocks: a RAM disk key is a number of blocks, decimal or
// %X, at least 100.
func TestParseRAMDiskBlocks(t *testing.T) {
	for _, c := range []struct {
		text string
		want uint32
		ok   bool
	}{
		{"512000", 512000, true},
		{" 100 ", 100, true},
		{"%X100", 256, true},
		{"%x3A", 0, false}, // 58, under the minimum
		{"99", 0, false},
		{"0", 0, false},
		{"-5", 0, false},
		{"lots", 0, false},
		{"", 0, false},
	} {
		got, ok := ParseRAMDiskBlocks(c.text)
		if got != c.want || ok != c.ok {
			t.Errorf("ParseRAMDiskBlocks(%q) = %d, %v; want %d, %v", c.text, got, ok, c.want, c.ok)
		}
	}
}

// TestRAMDiskName: MDc and MDcu, with "_" and ":" or without.
func TestRAMDiskName(t *testing.T) {
	for text, want := range map[string]string{"MDA": "MDA", "_mda0:": "MDA0", "MDB12": "MDB12"} {
		if got, ok := RAMDiskName(text); !ok || got != want {
			t.Errorf("RAMDiskName(%q) = %q, %v; want %q, true", text, got, ok, want)
		}
	}

	for _, text := range []string{"DUA0", "MD", "MDA0B", "MD0", "MDAA0"} {
		if _, ok := RAMDiskName(text); ok {
			t.Errorf("RAMDiskName(%q) is an MD device", text)
		}
	}
}
