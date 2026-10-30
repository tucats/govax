package vmsdef

import (
	"strings"
	"testing"
)

// TestMTHRTLVector checks what docs/PHASE-77 - MTHRTL.md's survey says of
// the table captured from VMS 7.1's MTHRTL.EXE: its size, a few entries,
// and that every entry is a quadword of one transfer vector, two pairs of
// names sharing theirs.
func TestMTHRTLVector(t *testing.T) {
	image, ok := SharedImages["MTHRTL"]
	if !ok {
		t.Fatal("no MTHRTL in SharedImages")
	}

	entries := map[uint32][]string{}
	mth, ots := 0, 0

	for name, sym := range ImageSymbols {
		if sym.Image != "MTHRTL" {
			continue
		}

		if strings.HasPrefix(name, "OTS$") {
			ots++
		} else {
			mth++
		}

		if sym.Value%8 != 0 {
			t.Errorf("%s is at %#x, not a quadword boundary", name, sym.Value)
		}

		entries[sym.Value] = append(entries[sym.Value], name)
	}

	if mth+ots != image.Symbols || mth != 376 || ots != 28 {
		t.Errorf("%d MTH$ and %d OTS$ names; want 376 and 28, of %d", mth, ots, image.Symbols)
	}

	if len(entries) != image.Symbols-2 {
		t.Errorf("%d vector entries for %d names; want two fewer", len(entries), image.Symbols)
	}

	for name, value := range map[string]uint32{
		"MTH$ACOS": 0, "MTH$ACOS_R4": 0x18, "MTH$RANDOM": 0x628, "OTS$POWRR": 0x7c8, "OTS$DIVCD_R3": 0x6f8,
	} {
		if sym := ImageSymbols[name]; sym.Image != "MTHRTL" || sym.Value != value {
			t.Errorf("%s = %+v, want MTHRTL+%#x", name, sym, value)
		}
	}

	for _, pair := range [][2]string{{"MTH$IIFIX", "MTH$IINT"}, {"MTH$JIFIX", "MTH$JINT"}} {
		if ImageSymbols[pair[0]].Value != ImageSymbols[pair[1]].Value {
			t.Errorf("%s and %s are different entries", pair[0], pair[1])
		}
	}
}
