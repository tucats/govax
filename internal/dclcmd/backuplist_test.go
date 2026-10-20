package dclcmd

import "testing"

// TestBackupListingFields: the listing's fields the probe's save sets
// don't reach (testdata/backup69 has the rest, TestBackupListOracle).
func TestBackupListingFields(t *testing.T) {
	// A file number past 65535 keeps its high byte in the third word's
	// high byte; the volume number is the low byte.
	if got := listingFileID([3]uint16{0x0005, 2, 0x0101}); got != "(65541,2,1)" {
		t.Errorf("listingFileID: %s", got)
	}

	if got := listingProtection(0xFF00); got != "System:RWED, Owner:RWED, Group:, World:" {
		t.Errorf("listingProtection: %s", got)
	}

	if got := listingUIC(0o200<<16 | 0o201); got != "[000200,000201]" {
		t.Errorf("listingUIC: %s", got)
	}

	for rat, want := range map[uint8]string{
		0:                          "None",
		ratCarriage:                "Carriage return",
		ratPrint:                   "Print file format",
		ratNoSpanned:               "Non-spanned",
		ratCarriage | ratNoSpanned: "Carriage return, Non-spanned",
	} {
		if got := listingRecordAttributes(rat); got != want {
			t.Errorf("listingRecordAttributes(%#x): %q, want %q", rat, got, want)
		}
	}

	if got := operatingSystemName(0x0400); got != "OpenVMS VAX" {
		t.Errorf("operatingSystemName: %s", got)
	}
}
