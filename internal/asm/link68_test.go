package asm

import (
	"os"
	"path/filepath"
	"testing"
)

// link68Sources are the sources of Phase 68's probe (testdata/link68),
// whose objects and listings real MACRO (VMS V7.1's, V5.4-3) made:
// XFR1's transfer vector (.TRANSFER and .MASK), the shareable images'
// routines, and the programs that use them.
var link68Sources = []string{
	"xfr1", "shr1", "shr2", "shr3", "shr6",
	"main1", "main2", "main3", "main6", "exe1", "exe1b",
}

// TestLink68Objects checks govax's object for each of the probe's
// sources against real MACRO's, record for record, as
// TestFixtureLadderObjects does the ladder's.
func TestLink68Objects(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "link68")

	for _, name := range link68Sources {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(dir, name+".mar"))
			if err != nil {
				t.Fatal(err)
			}

			requireSameObject(t, macroAssemble(t, string(src)), readObjectFile(t, filepath.Join(dir, "vax", name+".obj")))
		})
	}
}

// TestLink68Listings checks govax's listing of each of the probe's
// sources against real MACRO's, as TestListingPages does the earlier
// probes': a .TRANSFER line shows only the location counter, and a
// symbol .TRANSFER names is global.
func TestLink68Listings(t *testing.T) {
	dir := filepath.Join("..", "..", "testdata", "link68")

	for _, name := range link68Sources {
		t.Run(name, func(t *testing.T) {
			listing := filepath.Join(dir, "vax", name+".lis")

			var libs []MacroLibrary
			for _, lib := range listedLibraries(readListing(t, listing)) {
				libs = append(libs, NamedMacroLibrary(newMapLibrary(nil), lib))
			}

			checkListing(t, filepath.Join(dir, name+".mar"), listing, libs, listingCheck{})
		})
	}
}
