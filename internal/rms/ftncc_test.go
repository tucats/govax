package rms

import (
	"bytes"
	"testing"
)

// TestFTNPrintControl checks the print-file fixed control each FORTRAN
// carriage control character stands for, when an FTN accessor writes a
// record to a print file.
func TestFTNPrintControl(t *testing.T) {
	cases := map[byte][]byte{
		' ': {0x01, 0x8D},
		'0': {0x02, 0x8D},
		'1': {0x8C, 0x8D},
		'+': {0x00, 0x8D},
		'$': {0x01, 0x00},
		0:   {0x00, 0x00},
		'X': {0x01, 0x8D}, // any other character is a blank
	}

	for c, want := range cases {
		if got := ftnPrintControl(c); !bytes.Equal(got, want) {
			t.Errorf("%q: % X, want % X", c, got, want)
		}
	}
}
