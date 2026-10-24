package machine

import (
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// TestLegacyVectorStubs: after VMINIT, each P1 vector entry but a data
// cell has its stub in the old S0 vector, at the same offset: an empty
// entry mask and JMP @# the P1 entry's XFC, or for SYS$SRCHANDLER the
// jump alone, to the entry.
func TestLegacyVectorStubs(t *testing.T) {
	c := newBootableMachine(t)

	stubs := 0

	for _, e := range vmsdef.P1VectorTable {
		offset, want, ok := legacyStub(e)
		if !ok {
			continue
		}

		stubs++

		for i, b := range want {
			got, err := c.Mem.LoadByte(c.CPU, legacyVectorBase+offset+uint32(i))
			if err != nil || got != b {
				t.Fatalf("%s: byte %d of its stub at %X = %02X (%v), want %02X",
					e.Name, i, legacyVectorBase+offset, got, err, b)
			}
		}
	}

	if stubs != len(vmsdef.P1VectorTable)-2 {
		t.Errorf("%d stubs, want every entry but the two data cells", stubs)
	}

	if legacyImgsta != 0x80000168 || legacyVectorPages != 6 {
		t.Errorf("SYS$IMGSTA's old entry %X, %d pages; want 80000168, 6", legacyImgsta, legacyVectorPages)
	}
}

// TestLegacyStubForms: a procedure's stub and the JMP entry's.
func TestLegacyStubForms(t *testing.T) {
	for _, tc := range []struct {
		e      vmsdef.P1VectorEntry
		offset uint32
		code   []byte
	}{
		{vmsdef.P1VectorEntry{Name: "SYS$IMGSTA", Addr: 0x7FFEDF68}, 0x168, []byte{0, 0, 0x17, 0x9F, 0x6A, 0xDF, 0xFE, 0x7F}},
		{vmsdef.P1VectorEntry{Name: "SYS$SRCHANDLER", Addr: 0x7FFEE118, Jmp: true}, 0x318, []byte{0x17, 0x9F, 0x18, 0xE1, 0xFE, 0x7F}},
	} {
		offset, code, ok := legacyStub(tc.e)
		if !ok || offset != tc.offset || string(code) != string(tc.code) {
			t.Errorf("%s: offset %X, code % X; want %X, % X", tc.e.Name, offset, code, tc.offset, tc.code)
		}
	}

	if _, _, ok := legacyStub(vmsdef.P1VectorEntry{Name: "SYS$GL_COMMON", Addr: 0x7FFEE114}); ok {
		t.Error("SYS$GL_COMMON, a data cell, has a stub")
	}
}
