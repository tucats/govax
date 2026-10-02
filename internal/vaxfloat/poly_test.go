package vaxfloat

import "testing"

// TestPOLYStep checks one Horner step: exact cases, and the truncation of
// the product to 31 bits for F before the add. arg 1 + 2^-23 times acc
// 1 + 2^-23 is 1 + 2^-22 + 2^-46: as fractions, 0.5 + 2^-23 + 2^-47 at
// 2^2, whose 2^-47 is beyond the 31 bits kept, so adding -1 leaves 2^-22
// exactly (rounding the exact sum would too, here; the point is the
// truncated bits don't reappear).
func TestPOLYStep(t *testing.T) {
	for _, tc := range []struct {
		name           string
		f              Format
		arg, acc, coef float64
		want           float64
	}{
		{"2*0.25 + 0.5", F, 2, 0.25, 0.5, 1},
		{"zero argument", D, 0, 3, -1.5, -1.5},
		{"zero coefficient", G, 3, 0.5, 0, 1.5},
		{"cancels to zero", H, 2, 0.5, -1, 0},
		{"truncated product", F, 1 + 0x1p-23, 1 + 0x1p-23, -1, 0x1p-22},
	} {
		got, err := POLYStep(tc.f, FromFloat64(tc.arg), FromFloat64(tc.acc), FromFloat64(tc.coef))
		if err != nil || got.Float64() != tc.want {
			t.Errorf("%s: %v, %v; want %v", tc.name, got, err, tc.want)
		}
	}
}
