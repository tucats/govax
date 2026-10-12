package vm

import (
	"testing"

	"github.com/tucats/govax/internal/registers"
)

func TestPTEFieldRoundTrip(t *testing.T) {
	var p PTE

	p.SetPFN(0x1FFFFF)
	p.SetSoftware(0x3)
	p.SetOwner(0x3)
	p.SetModified(true)
	p.SetProtection(ProtUR)
	p.SetValid(true)

	if got := p.PFN(); got != 0x1FFFFF {
		t.Errorf("PFN() = %#x, want 0x1FFFFF", got)
	}

	if got := p.Software(); got != 0x3 {
		t.Errorf("Software() = %#x, want 0x3", got)
	}

	if got := p.Owner(); got != 0x3 {
		t.Errorf("Owner() = %#x, want 0x3", got)
	}

	if !p.Modified() {
		t.Error("Modified() = false, want true")
	}

	if got := p.Protection(); got != ProtUR {
		t.Errorf("Protection() = %v, want ProtUR", got)
	}

	if !p.Valid() {
		t.Error("Valid() = false, want true")
	}

	// Clear each field in turn and verify neighbors are undisturbed —
	// guards against a future refactor silently shifting a bit offset
	// (same technique as vax/psl_test.go).
	p.SetValid(false)

	if p.Valid() {
		t.Error("Valid() = true after SetValid(false)")
	}

	if got := p.Protection(); got != ProtUR {
		t.Errorf("Protection() disturbed by SetValid: got %v, want ProtUR", got)
	}

	p.SetProtection(ProtNA)

	if got := p.Protection(); got != ProtNA {
		t.Errorf("Protection() = %v, want ProtNA", got)
	}

	if !p.Modified() {
		t.Error("Modified() disturbed by SetProtection")
	}

	p.SetModified(false)

	if p.Modified() {
		t.Error("Modified() = true after SetModified(false)")
	}

	if got := p.Owner(); got != 0x3 {
		t.Errorf("Owner() disturbed by SetModified: got %#x, want 0x3", got)
	}

	p.SetOwner(0)

	if got := p.Owner(); got != 0 {
		t.Errorf("Owner() = %#x, want 0", got)
	}

	if got := p.Software(); got != 0x3 {
		t.Errorf("Software() disturbed by SetOwner: got %#x, want 0x3", got)
	}

	p.SetSoftware(0)

	if got := p.Software(); got != 0 {
		t.Errorf("Software() = %#x, want 0", got)
	}

	if got := p.PFN(); got != 0x1FFFFF {
		t.Errorf("PFN() disturbed by SetSoftware: got %#x, want 0x1FFFFF", got)
	}
}

// TestPTEBitOffsets is a direct bit-offset check against pte.h's non-
// BIGENDIAN struct PTEBITS layout, independent of the accessor methods.
func TestPTEBitOffsets(t *testing.T) {
	cases := []struct {
		name string
		pte  PTE
		want uint32
	}{
		{"valid bit is bit 31", 1 << 31, 1 << 31},
		{"prot occupies bits 27-30", 0xF << 27, 0xF << 27},
		{"modify is bit 26", 1 << 26, 1 << 26},
		{"mbz is bit 25", 1 << 25, 1 << 25},
		{"owner occupies bits 23-24", 0x3 << 23, 0x3 << 23},
		{"software occupies bits 21-22", 0x3 << 21, 0x3 << 21},
		{"pfn occupies bits 0-20", 0x1FFFFF, 0x1FFFFF},
	}

	for _, c := range cases {
		if uint32(c.pte) != c.want {
			t.Errorf("%s: %#08x != %#08x", c.name, uint32(c.pte), c.want)
		}
	}

	var p PTE

	p.SetPFN(0x1FFFFF)
	
	if uint32(p) != 0x1FFFFF {
		t.Errorf("SetPFN placed bits at %#08x, want 0x1FFFFF", uint32(p))
	}

	p = 0
	p.SetProtection(ProtUR) // 0xF

	if uint32(p) != 0xF<<27 {
		t.Errorf("SetProtection placed bits at %#08x, want %#08x", uint32(p), uint32(0xF)<<27)
	}
}

func TestProtectionAllows(t *testing.T) {
	cases := []struct {
		name   string
		prot   Protection
		mode   registers.AccessMode
		access AccessType
		want   bool
	}{
		{"NA denies kernel read", ProtNA, registers.Kernel, AccessRead, false},
		{"NA denies kernel write", ProtNA, registers.Kernel, AccessWrite, false},

		{"KW allows kernel write", ProtKW, registers.Kernel, AccessWrite, true},
		{"KW allows kernel read", ProtKW, registers.Kernel, AccessRead, true},
		{"KW denies exec write", ProtKW, registers.Executive, AccessWrite, false},
		{"KW denies exec read", ProtKW, registers.Executive, AccessRead, false},

		{"KR allows kernel read", ProtKR, registers.Kernel, AccessRead, true},
		{"KR denies kernel write", ProtKR, registers.Kernel, AccessWrite, false},
		{"KR denies exec read", ProtKR, registers.Executive, AccessRead, false},

		{"UW (ALL) allows user write", ProtUW, registers.User, AccessWrite, true},
		{"UW (ALL) allows user read", ProtUW, registers.User, AccessRead, true},

		{"SW allows super write", ProtSW, registers.Supervisor, AccessWrite, true},
		{"SW denies user write", ProtSW, registers.User, AccessWrite, false},
		{"SW denies user read", ProtSW, registers.User, AccessRead, false},

		{"UR allows user read", ProtUR, registers.User, AccessRead, true},
		{"UR denies user write", ProtUR, registers.User, AccessWrite, false},
		{"UR allows kernel read", ProtUR, registers.Kernel, AccessRead, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.prot.allows(c.mode, c.access); got != c.want {
				t.Errorf("Protection(%d).allows(%v, %v) = %v, want %v", c.prot, c.mode, c.access, got, c.want)
			}
		})
	}
}
