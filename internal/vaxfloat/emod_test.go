package vaxfloat

import (
	"errors"
	"testing"
)

// TestEMOD checks EMOD's integer and fraction parts, with values worked
// out by hand and, where marked, VMS's results for the Phase 35 probe.
func TestEMOD(t *testing.T) {
	for _, tc := range []struct {
		name       string
		f          Format
		mulr       float64
		ext        uint16
		muld       float64
		integer    int64
		fract      float64
		underflows bool
	}{
		{"1.5 x 2", F, 1.5, 0, 2, 3, 0, false},
		{"-2.5 x 1.5", F, -2.5, 0, 1.5, -3, -0.75, false},
		{"0.75 x 0.5", D, 0.75, 0, 0.5, 0, 0.375, false},
		{"G", G, 10.25, 0, 4, 41, 0, false},
		{"H", H, -7.5, 0, 0.5, -3, -0.75, false},
		// F's smallest value times 0.5: the fraction underflows.
		{"underflow", F, 0, 0, 0, 0, 0, true},
	} {
		mulr, muld := FromFloat64(tc.mulr), FromFloat64(tc.muld)

		if tc.underflows {
			mulr = ShortLiteral(0) // 0.5
			muld, _ = Unpack(F, Bits{Lo: 0x80})
		}

		i, fract, err := EMOD(tc.f, mulr, tc.ext, muld)

		if tc.underflows {
			if !errors.Is(err, ErrUnderflow) || i.Sign() != 0 || !fract.IsZero() {
				t.Errorf("%s: %v, %v, %v; want zeros and ErrUnderflow", tc.name, i, fract, err)
			}

			continue
		}

		if err != nil || i.Int64() != tc.integer || fract.Float64() != tc.fract {
			t.Errorf("%s: %v, %v, %v; want %d, %v", tc.name, i, fract, err, tc.integer, tc.fract)
		}
	}
}

// TestEMODExtension checks the extension bits take part, and the
// product's truncation: F's 1/3 rounds up (^XAAAAAB * 2^-25), so 1/3 x 3
// is 1 + 2^-25. An extension of ^XFF adds 255 * 2^-33 to the multiplier,
// so 765 * 2^-33 to the exact product; truncating the product's fraction
// (0.75 times the extended multiplier's, before normalizing) to 32 bits
// leaves a fraction part of 255 * 2^-31.
func TestEMODExtension(t *testing.T) {
	third, _ := Unpack(F, Bits{Lo: 0xAAAB3FAA})
	three := FromInt(3)

	_, plain, _ := EMOD(F, third, 0, three)
	_, extended, _ := EMOD(F, third, 0xFF, three)

	if want := FromFloat64(0x1p-25); plain.Cmp(want) != 0 {
		t.Errorf("1/3 x 3 fraction = %v, want 2^-25", plain)
	}

	if want := FromFloat64(255 * 0x1p-31); extended.Cmp(want) != 0 {
		t.Errorf("1/3 (ext ^XFF) x 3 fraction = %v, want %v", extended, want)
	}

	// G ignores the extension word's low 5 bits, H its low bit.
	gThird, _ := Pack(G, FromFloat64(1.0/3))
	g, _ := Unpack(G, gThird)
	_, a, _ := EMOD(G, g, 0x001F, three)
	_, b, _ := EMOD(G, g, 0, three)

	if a.Cmp(b) != 0 {
		t.Errorf("EMODG: extension bits 4:0 changed the result")
	}
}
