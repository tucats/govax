package rms

import "testing"

// rabBlockLength is RAB_K_BLN, the real, fixed total size of a RAB in
// bytes — see fabBlockLength's own comment (fab_test.go) for why this is
// a test-only local constant rather than a new rab.go export.
const rabBlockLength = 68

// TestRABOffsetsFitWithinBlock is rab.go's counterpart to
// TestFABOffsetsFitWithinBlock (fab_test.go) — see that test's doc
// comment for why this check exists.
func TestRABOffsetsFitWithinBlock(t *testing.T) {
	fields := []struct {
		name         string
		offset, size uint32
	}{
		{"rabISI", rabISI, 2},
		{"rabSTS", rabSTS, 4},
		{"rabSTV", rabSTV, 4},
		{"rabRAC", rabRAC, 1},
		{"rabRSZ", rabRSZ, 2},
		{"rabRBF", rabRBF, 4},
		{"rabUBF", rabUBF, 4},
		{"rabUSZ", rabUSZ, 2},
		{"rabFAB", rabFAB, 4},
	}

	for _, f := range fields {
		if f.offset+f.size > rabBlockLength {
			t.Errorf("%s: offset %d + size %d = %d, exceeds the %d-byte RAB",
				f.name, f.offset, f.size, f.offset+f.size, rabBlockLength)
		}
	}
}

// TestRABOffsetsDoNotOverlap is rab.go's counterpart to
// TestFABOffsetsDoNotOverlap (fab_test.go) — see that test's doc comment
// for why an overlap here would indicate a mistranscribed offset.
func TestRABOffsetsDoNotOverlap(t *testing.T) {
	type span struct {
		name         string
		offset, size uint32
	}

	fields := []span{
		{"rabISI", rabISI, 2},
		{"rabSTS", rabSTS, 4},
		{"rabSTV", rabSTV, 4},
		{"rabRAC", rabRAC, 1},
		{"rabRSZ", rabRSZ, 2},
		{"rabRBF", rabRBF, 4},
		{"rabUBF", rabUBF, 4},
		{"rabUSZ", rabUSZ, 2},
		{"rabFAB", rabFAB, 4},
	}

	for i, a := range fields {
		for _, b := range fields[i+1:] {
			if a.offset < b.offset+b.size && b.offset < a.offset+a.size {
				t.Errorf("%s (offset %d, size %d) overlaps %s (offset %d, size %d)",
					a.name, a.offset, a.size, b.name, b.offset, b.size)
			}
		}
	}
}
