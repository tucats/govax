package vax

import "testing"

func TestAliasesMatchArchitecturalIndices(t *testing.T) {
	cases := []struct {
		name string
		got  Reg
		want Reg
	}{
		{"AP", AP, R12},
		{"FP", FP, R13},
		{"SP", SP, R14},
		{"PC", PC, R15},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}

func TestGPRReadWrite(t *testing.T) {
	c := New()

	for r := R0; r <= R15; r++ {
		v := uint32(r)*0x1000 + 1
		c.SetGPR(r, v)

		if got := c.GPR(r); got != v {
			t.Errorf("GPR(%d) = %#x, want %#x", r, got, v)
		}
	}

	// Highest legal temporary index shouldn't panic or alias a named register.
	c.SetGPR(MaxReg, 0xdeadbeef)

	if got := c.GPR(MaxReg); got != 0xdeadbeef {
		t.Errorf("GPR(MaxReg) = %#x, want 0xdeadbeef", got)
	}

	if got := c.GPR(PC); got != uint32(PC)*0x1000+1 {
		t.Errorf("writing MaxReg disturbed PC: GPR(PC) = %#x", got)
	}
}

func TestAliasesShareStorage(t *testing.T) {
	c := New()

	c.SetGPR(PC, 0x12345678)
	
	if got := c.GPR(R15); got != 0x12345678 {
		t.Errorf("PC and R15 do not share storage: GPR(R15) = %#x", got)
	}

	c.SetGPR(SP, 0x1000)
	c.SetGPR(FP, 0x2000)
	c.SetGPR(AP, 0x3000)

	if got := c.GPR(R14); got != 0x1000 {
		t.Errorf("SP and R14 do not share storage: GPR(R14) = %#x", got)
	}

	if got := c.GPR(R13); got != 0x2000 {
		t.Errorf("FP and R13 do not share storage: GPR(R13) = %#x", got)
	}

	if got := c.GPR(R12); got != 0x3000 {
		t.Errorf("AP and R12 do not share storage: GPR(R12) = %#x", got)
	}
}

func TestPrivRegReadWrite(t *testing.T) {
	c := New()

	named := []PrivReg{
		KSP, ESP, SSP, USP, ISP, P0BR, P0LR, P1BR, P1LR, SBR, SLR,
		PCBB, SCBB, IPL, ASTLVL, SIRR, SISR, ICCS, NICR, ICR, TODR,
		RXCS, RXDB, TXCS, TXDB, TBDR, MAPEN, TBIA, TBIS, PMR, SID, TBCHK,
	}
	for _, r := range named {
		v := uint32(r)*0x100 + 7
		c.SetPR(r, v)

		if got := c.PR(r); got != v {
			t.Errorf("PR(%d) = %#x, want %#x", r, got, v)
		}
	}

	c.SetPR(MaxPrivReg, 0xcafef00d)

	if got := c.PR(MaxPrivReg); got != 0xcafef00d {
		t.Errorf("PR(MaxPrivReg) = %#x, want 0xcafef00d", got)
	}
}

func TestNamedPrivRegsAreDistinctSlots(t *testing.T) {
	c := New()

	named := []PrivReg{
		KSP, ESP, SSP, USP, ISP, P0BR, P0LR, P1BR, P1LR, SBR, SLR,
		PCBB, SCBB, IPL, ASTLVL, SIRR, SISR, ICCS, NICR, ICR, TODR,
		RXCS, RXDB, TXCS, TXDB, TBDR, MAPEN, TBIA, TBIS, PMR, SID, TBCHK,
	}

	for i, r := range named {
		c.SetPR(r, uint32(i)+1)
	}

	for i, r := range named {
		if got := c.PR(r); got != uint32(i)+1 {
			t.Errorf("PR(%d) = %d, want %d (named privileged registers overlap)", r, got, i+1)
		}
	}
}

func TestReset(t *testing.T) {
	c := New()
	c.SetGPR(R0, 1)
	c.SetGPR(PC, 2)
	c.SetPR(KSP, 3)

	c.Reset()

	if got := c.GPR(R0); got != 0 {
		t.Errorf("after Reset, GPR(R0) = %d, want 0", got)
	}

	if got := c.GPR(PC); got != 0 {
		t.Errorf("after Reset, GPR(PC) = %d, want 0", got)
	}

	if got := c.PR(KSP); got != 0 {
		t.Errorf("after Reset, PR(KSP) = %d, want 0", got)
	}
}
