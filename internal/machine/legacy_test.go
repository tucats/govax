package machine

import (
	"encoding/binary"
	"testing"

	"github.com/tucats/govax/internal/vmsimage"
)

// legacyImage is a VMS V1 image (format 02.01) with one two-page
// section at ^X200, from VBN 2, holding stub at ^X233: a header with a
// fixed part of ^X28 bytes, an activation block whose transfer
// addresses are ^X233 and ^X210, and the ISD.
func legacyImage(stub []byte) []byte {
	le := binary.LittleEndian
	data := make([]byte, 3*vmsimage.BlockSize)

	le.PutUint16(data[0x00:], 0x38) // the ISDs
	le.PutUint16(data[0x02:], 0x28) // the activation block
	copy(data[0x0C:], "0201")
	data[0x10], data[0x11] = 1, vmsimage.TypeExecutable

	le.PutUint32(data[0x28:], 0x233)
	le.PutUint32(data[0x2C:], 0x210)

	isd := data[0x38:]
	le.PutUint16(isd[0:], vmsimage.ISDPrivateLength)
	le.PutUint16(isd[2:], 2)
	le.PutUint32(isd[4:], 1)  // VPN 1, ^X200
	le.PutUint32(isd[12:], 2) // VBN 2

	copy(data[vmsimage.BlockSize+0x33:], stub)

	return data
}

// TestLegacyStartupStub recognizes a V1 image's image startup stub, a
// jump past the old vector's SYS$IMGSTA entry mask, and nothing else.
func TestLegacyStartupStub(t *testing.T) {
	if legacyImgsta != 0x80000168 {
		t.Fatalf("legacyImgsta = %X, want 80000168", legacyImgsta)
	}

	for _, tc := range []struct {
		name string
		stub []byte
		want bool
	}{
		{"stub", []byte{0, 0, 0x17, 0x9F, 0x6A, 0x01, 0x00, 0x80}, true},
		{"other target", []byte{0, 0, 0x17, 0x9F, 0x6E, 0x01, 0x00, 0x80}, false},
		{"entry mask", []byte{0x0C, 0, 0x17, 0x9F, 0x6A, 0x01, 0x00, 0x80}, false},
		{"not a jump", []byte{0, 0, 0x16, 0x9F, 0x6A, 0x01, 0x00, 0x80}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := legacyImage(tc.stub)

			img, err := vmsimage.ReadImage(data)
			if err != nil {
				t.Fatal(err)
			}

			if got := legacyStartupStub(img, data, img.Transfers[0]); got != tc.want {
				t.Errorf("legacyStartupStub = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestMainTransferLegacy: an image whose first transfer address is the
// startup stub runs from its second.
func TestMainTransferLegacy(t *testing.T) {
	icb := &ICB{Transfer: [4]uint32{0x233, 0x210}}

	if addr, _ := mainTransferAddress(icb); addr != 0x233 {
		t.Errorf("without the stub, main = %X, want 233", addr)
	}

	icb.LegacyStartup = true

	if addr, _ := mainTransferAddress(icb); addr != 0x210 {
		t.Errorf("with the stub, main = %X, want 210", addr)
	}
}
