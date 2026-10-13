package cpu

import (
	"math/big"
	"testing"
)

// TestReadPacked: values and the well-formedness the RTL checks: -123 is
// ^X12,^X3D; +1234 is ^X01,^X23,^X4C; a digit over 9, a sign under ^XA,
// or an even length's unused nibble not 0 is ill-formed (and read as the
// instructions read it).
func TestReadPacked(t *testing.T) {
	c, m := fixture()

	cases := []struct {
		bytes  []byte
		length int
		want   int64
		valid  bool
	}{
		{[]byte{0x12, 0x3D}, 3, -123, true},
		{[]byte{0x01, 0x23, 0x4C}, 4, 1234, true},
		{[]byte{0x0C}, 0, 0, true},
		{[]byte{0x1C, 0x00}, 3, 1*100 + 12*10 + 0, false}, // a digit ^XC, and a sign 0
		{[]byte{0x12, 0x35}, 3, 123, false},               // a sign under ^XA
		{[]byte{0x11, 0x23, 0x4C}, 4, 1234, false},        // the unused nibble 1
		{[]byte{0x12, 0x3F}, 3, 123, true},                // ^XF is a plus
	}

	for _, tc := range cases {
		putBytes(t, c, m, 0x1000, tc.bytes...)

		v, valid, err := ReadPacked(c, m, 0x1000, tc.length)
		if err != nil || v.Int64() != tc.want || valid != tc.valid {
			t.Errorf("% X (length %d): %v, valid %v, %v; want %d, %v", tc.bytes, tc.length, v, valid, err, tc.want, tc.valid)
		}
	}
}

// TestWritePacked: the preferred signs, an even length's zero nibble,
// and overflow keeping the low-order digits.
func TestWritePacked(t *testing.T) {
	c, m := fixture()

	cases := []struct {
		v        int64
		length   int
		want     []byte
		overflow bool
	}{
		{-123, 3, []byte{0x12, 0x3D}, false},
		{1234, 4, []byte{0x01, 0x23, 0x4C}, false},
		{12345, 3, []byte{0x34, 0x5C}, true},
		{0, 1, []byte{0x0C}, false},
	}

	for _, tc := range cases {
		overflow, err := WritePacked(c, m, 0x1000, tc.length, big.NewInt(tc.v))
		if err != nil || overflow != tc.overflow {
			t.Errorf("%d: overflow %v, %v", tc.v, overflow, err)
		}

		for i, want := range tc.want {
			if b, _ := m.LoadByte(c, 0x1000+uint32(i)); b != want {
				t.Errorf("%d: byte %d is %02X, want %02X", tc.v, i, b, want)
			}
		}
	}
}
