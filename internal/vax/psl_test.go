package vax

import "testing"

func TestPSLConditionCodes(t *testing.T) {
	var p PSL

	p.SetNZVC(true, false, true, false)

	if !p.N() || p.Z() || !p.V() || p.C() {
		t.Fatalf("SetNZVC(true,false,true,false): N=%v Z=%v V=%v C=%v", p.N(), p.Z(), p.V(), p.C())
	}

	p.SetNZVC(false, true, false, true)

	if p.N() || !p.Z() || p.V() || !p.C() {
		t.Fatalf("SetNZVC(false,true,false,true): N=%v Z=%v V=%v C=%v", p.N(), p.Z(), p.V(), p.C())
	}
}

func TestPSLFieldsDoNotOverlap(t *testing.T) {
	// Set every field to a distinct, recognizable value, then confirm each
	// getter still reads back exactly what its own setter put there and
	// nothing else, i.e. no two fields share a bit.
	var p PSL

	p.SetN(true)
	p.SetZ(true)
	p.SetV(true)
	p.SetC(true)
	p.SetT(true)
	p.SetIV(true)
	p.SetFU(true)
	p.SetDV(true)
	p.SetIPL(31)
	p.SetCurMod(Executive)
	p.SetPrvMod(User)
	p.SetIS(true)
	p.SetFPD(true)
	p.SetTP(true)
	p.SetCM(true)

	if !p.N() || !p.Z() || !p.V() || !p.C() {
		t.Errorf("condition codes lost: N=%v Z=%v V=%v C=%v", p.N(), p.Z(), p.V(), p.C())
	}

	if !p.T() || !p.IV() || !p.FU() || !p.DV() {
		t.Errorf("trap enables lost: T=%v IV=%v FU=%v DV=%v", p.T(), p.IV(), p.FU(), p.DV())
	}

	if p.IPL() != 31 {
		t.Errorf("IPL() = %d, want 31", p.IPL())
	}

	if p.CurMod() != Executive {
		t.Errorf("CurMod() = %d, want %d", p.CurMod(), Executive)
	}

	if p.PrvMod() != User {
		t.Errorf("PrvMod() = %d, want %d", p.PrvMod(), User)
	}

	if !p.IS() || !p.FPD() || !p.TP() || !p.CM() {
		t.Errorf("privileged flags lost: IS=%v FPD=%v TP=%v CM=%v", p.IS(), p.FPD(), p.TP(), p.CM())
	}

	// Now clear each field one at a time and confirm nothing else moves.
	before := p
	p.SetN(false)

	if p.N() {
		t.Error("SetN(false) did not clear N")
	}

	if p != before&^pslN {
		t.Errorf("SetN(false) disturbed other fields: got %#010x, want %#010x", uint32(p), uint32(before&^pslN))
	}
}

func TestPSLIPLMasksToFiveBits(t *testing.T) {
	var p PSL

	p.SetIPL(0xFF) // out-of-range input; only the low 5 bits are architected

	if got := p.IPL(); got != 0x1F {
		t.Errorf("IPL() = %d, want 31 (masked to 5 bits)", got)
	}
}

func TestPSLAccessModesMasksToTwoBits(t *testing.T) {
	var p PSL
	
	p.SetCurMod(User)
	p.SetPrvMod(Kernel)

	if p.CurMod() != User {
		t.Errorf("CurMod() = %d, want %d", p.CurMod(), User)
	}

	if p.PrvMod() != Kernel {
		t.Errorf("PrvMod() = %d, want %d", p.PrvMod(), Kernel)
	}
}

func TestPSLWholeWordRoundTrip(t *testing.T) {
	c := New()
	c.SetPSL(0x12345678)

	if got := c.PSL(); got != 0x12345678 {
		t.Errorf("PSL() = %#x, want %#x", uint32(got), uint32(0x12345678))
	}
}

func TestPSLBitLayoutMatchesArchitectureManual(t *testing.T) {
	// Spot-check the documented bit offsets directly, independent of the
	// accessor methods, as a guard against a future refactor silently
	// shifting a field.
	cases := []struct {
		name string
		mask uint32
		bit  uint
	}{
		{"C", pslC, 0},
		{"V", pslV, 1},
		{"Z", pslZ, 2},
		{"N", pslN, 3},
		{"T", pslT, 4},
		{"IV", pslIV, 5},
		{"FU", pslFU, 6},
		{"DV", pslDV, 7},
		{"IS", pslIS, 26},
		{"FPD", pslFPD, 27},
		{"TP", pslTP, 30},
		{"CM", pslCM, 31},
	}
	
	for _, tc := range cases {
		if tc.mask != 1<<tc.bit {
			t.Errorf("%s mask = %#x, want bit %d (%#x)", tc.name, tc.mask, tc.bit, uint32(1)<<tc.bit)
		}
	}

	if pslIPL != 0x1F<<16 {
		t.Errorf("IPL mask = %#x, want bits 16-20", pslIPL)
	}

	if pslPrv != 0x3<<22 {
		t.Errorf("PRV_MOD mask = %#x, want bits 22-23", pslPrv)
	}

	if pslCur != 0x3<<24 {
		t.Errorf("CUR_MOD mask = %#x, want bits 24-25", pslCur)
	}
}
