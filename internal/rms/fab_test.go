package rms

import "testing"

// fabBlockLength is FAB_K_BLN, the real, fixed total size of a FAB in
// bytes — kept as a test-only local constant (rather than a new exported
// fab.go constant) since nothing outside this test currently needs it.
const fabBlockLength = 80

// TestFABOffsetsFitWithinBlock is a structural regression test over
// fab.go's offset constants: for every field this package knows about, its
// (offset, size-in-bytes) pair must land entirely inside the 80-byte FAB —
// never partially or fully past the end of the block. This is exactly the
// kind of mistake a typo'd offset constant would produce (see fabFNS's own
// doc comment for a real example of a wrong offset the project shipped
// once already, in now-deleted code), and it's cheap to catch here rather
// than only when some future handler test happens to exercise the
// specific field.
func TestFABOffsetsFitWithinBlock(t *testing.T) {
	fields := []struct {
		name         string
		offset, size uint32
	}{
		{"fabIFI", fabIFI, 2},
		{"fabSTS", fabSTS, 4},
		{"fabSTV", fabSTV, 4},
		{"fabFAC", fabFAC, 1},
		{"fabORG", fabORG, 1},
		{"fabRAT", fabRAT, 1},
		{"fabRFM", fabRFM, 1},
		{"fabFNA", fabFNA, 4},
		{"fabFNS", fabFNS, 1},
		{"fabMRS", fabMRS, 2},
	}

	for _, f := range fields {
		if f.offset+f.size > fabBlockLength {
			t.Errorf("%s: offset %d + size %d = %d, exceeds the %d-byte FAB",
				f.name, f.offset, f.size, f.offset+f.size, fabBlockLength)
		}
	}
}

// TestFABOffsetsDoNotOverlap confirms no two of the fields this package
// actually reads/writes claim overlapping bytes within the FAB — real
// VMS's own $FABDEF never overlaps two *different* fields this way (the
// overlays fab.h/starlet.req define are always alternate views of the
// *same* bytes, such as a byte's bit-flag interpretation alongside its
// plain-value interpretation, never two unrelated fields sharing space),
// so an overlap here would mean this package mistranscribed an offset.
func TestFABOffsetsDoNotOverlap(t *testing.T) {
	type span struct {
		name         string
		offset, size uint32
	}

	fields := []span{
		{"fabIFI", fabIFI, 2},
		{"fabSTS", fabSTS, 4},
		{"fabSTV", fabSTV, 4},
		{"fabFAC", fabFAC, 1},
		{"fabORG", fabORG, 1},
		{"fabRAT", fabRAT, 1},
		{"fabRFM", fabRFM, 1},
		{"fabFNA", fabFNA, 4},
		{"fabFNS", fabFNS, 1},
		{"fabMRS", fabMRS, 2},
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
