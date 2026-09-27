package vmsdef

import (
	"strings"
	"testing"
)

// TestLNMConstants_values pins the $LNMDEF values docs/PHASE-25.md's
// logical-name design depends on. Besides lnmdef.sdl itself, the attribute
// bits and item codes agree with what internal/rtl/logicals.go and the
// since-deleted internal/io/logical.go used before this table existed.
func TestLNMConstants_values(t *testing.T) {
	want := map[string]uint32{
		// Logical name attributes (byte 0).
		"LNM$M_NO_ALIAS": 0x1,
		"LNM$M_CONFINE":  0x2,
		"LNM$M_CRELOG":   0x4,
		"LNM$M_TABLE":    0x8,

		// Translation attributes (byte 1).
		"LNM$M_CONCEALED": 0x100,
		"LNM$M_TERMINAL":  0x200,
		"LNM$M_EXISTS":    0x400,

		// Table characteristics (byte 2).
		"LNM$M_SHAREABLE": 0x10000,

		// Service options (byte 3).
		"LNM$M_CREATE_IF":  0x1000000,
		"LNM$M_CASE_BLIND": 0x2000000,

		// Limits.
		"LNM$C_TABNAMLEN": 31,
		"LNM$C_NAMLENGTH": 255,
		"LNM$C_MAXDEPTH":  10,

		// Item codes.
		"LNM$_INDEX":      1,
		"LNM$_STRING":     2,
		"LNM$_ATTRIBUTES": 3,
		"LNM$_TABLE":      4,
		"LNM$_LENGTH":     5,
		"LNM$_ACMODE":     6,
		"LNM$_MAX_INDEX":  7,
		"LNM$_PARENT":     8,
		"LNM$_CHAIN":      0xFFFFFFFF,
	}

	for name, v := range want {
		got, ok := LNMConstants[name]
		if !ok {
			t.Errorf("%s missing", name)

			continue
		}

		if got != v {
			t.Errorf("%s = %#x, want %#x", name, got, v)
		}
	}
}

// TestLNMConstants_maskMatchesBit confirms every LNM$M_ mask is exactly
// 1 << its LNM$V_ bit position ($LNMDEF has only single-bit fields).
func TestLNMConstants_maskMatchesBit(t *testing.T) {
	for name, mask := range LNMConstants {
		field, ok := strings.CutPrefix(name, "LNM$M_")
		if !ok {
			continue
		}

		bit, ok := LNMConstants["LNM$V_"+field]
		if !ok {
			t.Errorf("%s has no LNM$V_%s", name, field)

			continue
		}

		if mask != 1<<bit {
			t.Errorf("%s = %#x, want 1<<%d", name, mask, bit)
		}
	}
}

// TestSSConstants_values pins the $SSDEF completion codes the logical-name
// services return, plus the codes internal/rtl/status.go already carried
// as literals, which ssdef.txt independently confirms.
func TestSSConstants_values(t *testing.T) {
	want := map[string]uint32{
		"SS$_NORMAL":      1,
		"SS$_ACCVIO":      12,
		"SS$_BADPARAM":    20,
		"SS$_NOPRIV":      36,
		"SS$_DUPLNAM":     148,
		"SS$_INSFARG":     276,
		"SS$_INSFMEM":     292,
		"SS$_IVCHAN":      316,
		"SS$_IVDEVNAM":    324,
		"SS$_IVLOGNAM":    340,
		"SS$_IVLOGTAB":    348,
		"SS$_NOLOGNAM":    444,
		"SS$_RESULTOVF":   532,
		"SS$_TOOMANYLNAM": 884,
		"SS$_BUFFEROVF":   1537,
		"SS$_NOTRAN":      1577,
		"SS$_SUPERSEDE":   1585,
		"SS$_NOSUCHDEV":   2312,
		"SS$_NOSUCHFILE":  2320,
		"SS$_EXLNMQUOTA":  8780,
		"SS$_NOLOGTAB":    8852,
	}

	for name, v := range want {
		got, ok := SSConstants[name]
		if !ok {
			t.Errorf("%s missing", name)

			continue
		}

		if got != v {
			t.Errorf("%s = %d, want %d", name, got, v)
		}
	}
}

// TestConstants_onlyRMSFamilies guards the reason LNMConstants and
// SSConstants are separate maps: .RMSDEF (internal/asm) defines every
// Constants entry as an assembler symbol, so nothing but FAB$/RAB$/RMS$
// names may appear there.
func TestConstants_onlyRMSFamilies(t *testing.T) {
	for name := range Constants {
		if !strings.HasPrefix(name, "FAB$") && !strings.HasPrefix(name, "RAB$") && !strings.HasPrefix(name, "RMS$") {
			t.Errorf("unexpected %s in Constants", name)
		}
	}

	for name := range LNMConstants {
		if !strings.HasPrefix(name, "LNM$") {
			t.Errorf("unexpected %s in LNMConstants", name)
		}
	}

	for name := range SSConstants {
		if !strings.HasPrefix(name, "SS$_") {
			t.Errorf("unexpected %s in SSConstants", name)
		}
	}
}
