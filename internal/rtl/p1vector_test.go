package rtl

import "testing"

// TestLookupP1VectorSkipsDataCells checks that SYS$GL_ASTRET, a data cell
// at SYS$CLRAST_2's own address, doesn't shadow that service, and that
// SYS$GL_COMMON isn't a dispatch target at all.
func TestLookupP1VectorSkipsDataCells(t *testing.T) {
	e, ok := lookupP1Vector(0x7FFEE110)
	if !ok || e.Name != "SYS$CLRAST_2" {
		t.Fatalf("lookupP1Vector(0x7FFEE110) = %q, %v; want SYS$CLRAST_2", e.Name, ok)
	}

	if e, ok := lookupP1Vector(0x7FFEE114); ok {
		t.Fatalf("lookupP1Vector(0x7FFEE114) = %q; want no match", e.Name)
	}
}
