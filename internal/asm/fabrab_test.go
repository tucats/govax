package asm

import (
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// TestPseudoFAB_autoBIDBLN confirms a bare ".FAB" (no parameters) still
// writes the auto BID/BLN identification bytes, matching a real $FAB
// macro's own expansion, and advances the deposit location by 80 bytes.
func TestPseudoFAB_autoBIDBLN(t *testing.T) {
	a := New(true)
	a.SetOrigin(0x1000)

	if _, err := a.Assemble(".FAB"); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	if got := a.ByteAt(0x1000); got != byte(vmsdef.Constants["FAB$C_BID"]) {
		t.Errorf("BID = %#x, want %#x", got, vmsdef.Constants["FAB$C_BID"])
	}

	if got := a.ByteAt(0x1001); got != byte(vmsdef.Constants["FAB$K_BLN"]) {
		t.Errorf("BLN = %#x, want %#x", got, vmsdef.Constants["FAB$K_BLN"])
	}

	if a.Deposit() != 0x1000+80 {
		t.Errorf("Deposit() = %#x, want %#x", a.Deposit(), 0x1000+80)
	}
}

// TestPseudoRAB_autoBIDBLN is TestPseudoFAB_autoBIDBLN's RAB counterpart.
func TestPseudoRAB_autoBIDBLN(t *testing.T) {
	a := New(true)
	a.SetOrigin(0x2000)

	if _, err := a.Assemble(".RAB"); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	if got := a.ByteAt(0x2000); got != byte(vmsdef.Constants["RAB$C_BID"]) {
		t.Errorf("BID = %#x, want %#x", got, vmsdef.Constants["RAB$C_BID"])
	}

	if got := a.ByteAt(0x2001); got != byte(vmsdef.Constants["RAB$K_BLN"]) {
		t.Errorf("BLN = %#x, want %#x", got, vmsdef.Constants["RAB$K_BLN"])
	}

	if a.Deposit() != 0x2000+68 {
		t.Errorf("Deposit() = %#x, want %#x", a.Deposit(), 0x2000+68)
	}
}

// TestPseudoFAB_keywordsPlaceValues builds a real FAB with several keyword
// parameters (matching rms_roundtrip.asm's own field set) and confirms
// each value landed at its real field offset, at the right width.
func TestPseudoFAB_keywordsPlaceValues(t *testing.T) {
	a := New(true)
	a.SetOrigin(0x1000)

	// FNS uses "^D13" (decimal), not a bare "13" -- this assembler's
	// default radix is hex (docs/PHASE-11.md), so a plain "13" here would
	// mean 0x13 (19), matching testdata/asm/rms_roundtrip.asm's own
	// identical convention for the same field.
	src := ".RMSDEF\n" +
		"FAB:\t.FAB FAC=FAB$M_PUT, ORG=FAB$C_SEQ, RFM=FAB$C_FIX, MRS=4, FNS=^D13\n"

	if _, err := a.Assemble(src); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	base, _, err := a.getSymbol("FAB", false, 0, fixNone)
	if err != nil {
		t.Fatalf("FAB label: %v", err)
	}

	if got := a.ByteAt(base + 22); got != 1 { // FAC
		t.Errorf("FAC = %d, want 1", got)
	}

	if got := a.ByteAt(base + 29); got != 0 { // ORG (FAB$C_SEQ = 0)
		t.Errorf("ORG = %d, want 0", got)
	}

	if got := a.ByteAt(base + 31); got != 1 { // RFM (FAB$C_FIX = 1)
		t.Errorf("RFM = %d, want 1", got)
	}

	gotMRS := uint16(a.ByteAt(base+54)) | uint16(a.ByteAt(base+55))<<8
	if gotMRS != 4 {
		t.Errorf("MRS = %d, want 4", gotMRS)
	}

	if got := a.ByteAt(base + 52); got != 13 { // FNS
		t.Errorf("FNS = %d, want 13", got)
	}
}

// TestPseudoFAB_forwardReferenceValue confirms a keyword's value supports
// a forward reference, matching .LONG's own support for one (e.g.
// FNA=fspec where fspec is defined later in the file).
func TestPseudoFAB_forwardReferenceValue(t *testing.T) {
	a := New(true)
	a.SetOrigin(0x1000)

	src := "FAB:\t.FAB FNA=fspec\n" +
		"fspec:\t.ascii \"DUA0:TEST.DAT\"\n"

	if _, err := a.Assemble(src); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	fspecAddr, _, err := a.getSymbol("FSPEC", false, 0, fixNone)
	if err != nil {
		t.Fatalf("fspec label: %v", err)
	}

	base, _, err := a.getSymbol("FAB", false, 0, fixNone)
	if err != nil {
		t.Fatalf("FAB label: %v", err)
	}

	got := a.BytesRange(base+44, base+48)
	want := []byte{byte(fspecAddr), byte(fspecAddr >> 8), byte(fspecAddr >> 16), byte(fspecAddr >> 24)}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("FNA bytes = % X, want % X", got, want)
		}
	}
}

// TestPseudoFAB_unknownKeywordErrors confirms a typo'd keyword is reported
// rather than silently ignored.
func TestPseudoFAB_unknownKeywordErrors(t *testing.T) {
	a := New(true)

	if _, err := a.Assemble(".FAB NOTAREALFIELD=1"); err == nil {
		t.Fatal("expected an error for an unknown .FAB keyword")
	}
}

// TestPseudoFAB_unsettableFieldErrors confirms RAB$W_RFA-style fields
// (Size not 1/2/4) can't be set via a single .RAB keyword value.
func TestPseudoFAB_unsettableFieldErrors(t *testing.T) {
	a := New(true)

	if _, err := a.Assemble(".RAB RFA=1"); err == nil {
		t.Fatal("expected an error setting RAB$W_RFA (Size 6) via a keyword")
	}
}

// TestPseudoFAB_unwrittenFieldsAreZero confirms every field not named in
// the parameter list reads back as zero.
func TestPseudoFAB_unwrittenFieldsAreZero(t *testing.T) {
	a := New(true)
	a.SetOrigin(0x1000)

	if _, err := a.Assemble(".FAB FAC=1"); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	// FAB$L_STS (offset 8), never named, should read back zero.
	got := a.BytesRange(0x1000+8, 0x1000+12)
	for _, b := range got {
		if b != 0 {
			t.Fatalf("STS bytes = % X, want all zero", got)
		}
	}
}
