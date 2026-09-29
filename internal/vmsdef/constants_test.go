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

	for name := range DEVConstants {
		if !strings.HasPrefix(name, "DEV$") {
			t.Errorf("unexpected %s in DEVConstants", name)
		}
	}

	for name := range DVIConstants {
		if !strings.HasPrefix(name, "DVI$") {
			t.Errorf("unexpected %s in DVIConstants", name)
		}
	}

	for name := range TTConstants {
		if !strings.HasPrefix(name, "TT$") && !strings.HasPrefix(name, "TT2$") {
			t.Errorf("unexpected %s in TTConstants", name)
		}
	}
}

// TestDVIAndTTConstants_values pins $DVIDEF item codes and $TTDEF bits
// $GETDVI (docs/PHASE-26.md subtask 28) depends on, from the VMS I/O
// User's Reference Manual and the System Services Reference Manual.
func TestDVIAndTTConstants_values(t *testing.T) {
	want := map[string]uint32{
		"DVI$_DEVCHAR":     2,
		"DVI$_DEVCLASS":    4,
		"DVI$_DEVNAM":      32,
		"DVI$_TT_NOECHO":   128,
		"DVI$_FULLDEVNAM":  232,
		"DVI$M_SECONDARY":  1,
		"DVI$M_NOREDIRECT": 0x8000,
	}

	for name, v := range want {
		if got, ok := DVIConstants[name]; !ok || got != v {
			t.Errorf("DVIConstants[%s] = %#x, %v; want %#x", name, got, ok, v)
		}
	}

	want = map[string]uint32{
		"TT$M_NOECHO":   0x2,
		"TT$M_LOWER":    0x80,
		"TT2$M_ANSICRT": 0x1000000,
		"TT2$M_DECCRT":  0x20000000,
	}

	for name, v := range want {
		if got, ok := TTConstants[name]; !ok || got != v {
			t.Errorf("TTConstants[%s] = %#x, %v; want %#x", name, got, ok, v)
		}
	}
}

// TestDEVConstants_values pins the $DEVDEF DEVCHAR bits $ALLOC
// (docs/PHASE-26.md) depends on, plus one DEVCHAR2 bit to show the second
// union member numbers its bits from 0 again.
func TestDEVConstants_values(t *testing.T) {
	want := map[string]uint32{
		"DEV$M_TRM": 0x4,
		"DEV$M_SPL": 0x40,
		"DEV$M_SHR": 0x10000,
		"DEV$M_AVL": 0x40000,
		"DEV$M_MNT": 0x80000,
		"DEV$M_MBX": 0x100000,
		"DEV$V_ALL": 23,
		"DEV$M_ALL": 0x800000,
		"DEV$M_CLU": 0x1,
		"DEV$M_2P":  0x10,
	}

	for name, v := range want {
		got, ok := DEVConstants[name]
		if !ok {
			t.Errorf("%s missing from DEVConstants", name)

			continue
		}

		if got != v {
			t.Errorf("%s = %#x, want %#x", name, got, v)
		}
	}
}

// TestIOConstants_values pins the $IODEF function codes and terminal
// modifier bits terminal $QIO (docs/PHASE-26.md subtask 17) depends on.
// The values are the VMS I/O User's Reference Manual's; the modifiers sit
// above the 6-bit function code (IO$M_FCODE), and the read and write
// modifiers are different union members that reuse the same bits
// (IO$M_NOECHO and IO$M_CANCTRLO are both bit 6).
func TestIOConstants_values(t *testing.T) {
	want := map[string]uint32{
		"IO$M_FCODE":      0x3F,
		"IO$_WRITEPBLK":   11,
		"IO$_READPBLK":    12,
		"IO$_SETCHAR":     26,
		"IO$_SENSECHAR":   27,
		"IO$_WRITELBLK":   32,
		"IO$_READLBLK":    33,
		"IO$_SETMODE":     35,
		"IO$_SENSEMODE":   39,
		"IO$_WRITEVBLK":   48,
		"IO$_READVBLK":    49,
		"IO$_READPROMPT":  55,
		"IO$_TTYREADALL":  58,
		"IO$_TTYREADPALL": 59,
		"IO$M_NOECHO":     0x40,
		"IO$M_TIMED":      0x80,
		"IO$M_CVTLOW":     0x100,
		"IO$M_PURGE":      0x800,
		"IO$M_CANCTRLO":   0x40,
		"IO$M_NOFORMAT":   0x100,
		"IO$K_LOOPTEST":   0xE000,
	}

	for name, v := range want {
		got, ok := IOConstants[name]
		if !ok {
			t.Errorf("%s missing from IOConstants", name)

			continue
		}

		if got != v {
			t.Errorf("%s = %#x, want %#x", name, got, v)
		}
	}
}

// TestATRConstants_values pins the $ATRDEF codes and sizes the disk
// driver's attribute lists (docs/PHASE-26.md subtask 42) depend on, as
// the VMS I/O User's Reference lists them, and checks that the two sizes
// the listings disagree on were left out.
func TestATRConstants_values(t *testing.T) {
	want := map[string]uint32{
		"ATR$C_UCHAR":     3,
		"ATR$C_RECATTR":   4,
		"ATR$C_HEADER":    10,
		"ATR$C_ASCNAME":   16,
		"ATR$C_CREDATE":   17,
		"ATR$C_REVDATE":   18,
		"ATR$C_EXPDATE":   19,
		"ATR$C_BAKDATE":   20,
		"ATR$C_UIC":       21,
		"ATR$C_FPRO":      22,
		"ATR$C_BACKLINK":  28,
		"ATR$C_HIGHWATER": 41,
		"ATR$S_RECATTR":   32,
		"ATR$S_UCHAR":     4,
		"ATR$S_HEADER":    512,
		"ATR$W_SIZE":      0,
		"ATR$W_TYPE":      2,
		"ATR$L_ADDR":      4,
		"ATR$S_ATRDEF":    8,
	}

	for name, v := range want {
		if got, ok := ATRConstants[name]; !ok || got != v {
			t.Errorf("ATRConstants[%s] = %#x, %v; want %#x", name, got, ok, v)
		}
	}

	for _, name := range []string{"ATR$S_ASCDATES", "ATR$S_FILE_SPEC"} {
		if _, ok := ATRConstants[name]; ok {
			t.Errorf("ATRConstants has %s, which the listings disagree on", name)
		}
	}
}
