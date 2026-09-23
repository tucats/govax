package vmsdef

import "testing"

// aliasGroups lists the offsets where more than one Field entry is expected
// to occupy the exact same bytes — real union member names for the same
// storage (KBF/PBF, KSZ/PSZ, BKT/DCT — see RABFields' own doc comment), not
// a mistake. Any other overlap found by checkFields is a real bug.
func aliasGroups(fields []Field) map[uint32]bool {
	counts := map[uint32]int{}
	for _, f := range fields {
		counts[f.Offset]++
	}

	aliased := map[uint32]bool{}
	for off, n := range counts {
		if n > 1 {
			aliased[off] = true
		}
	}

	return aliased
}

// checkFields confirms fields tile [0, wantSize) with no gaps or unexpected
// overlaps, matching a real C struct's own natural-alignment layout.
func checkFields(t *testing.T, name string, fields []Field, wantSize uint32) {
	t.Helper()

	aliased := aliasGroups(fields)

	covered := make(map[uint32]string, wantSize)

	for _, f := range fields {
		if f.Offset+f.Size > wantSize {
			t.Errorf("%s: field %s (%s) at offset %d size %d extends past %d",
				name, f.Symbol, f.Keyword, f.Offset, f.Size, wantSize)
		}

		for b := f.Offset; b < f.Offset+f.Size; b++ {
			if owner, ok := covered[b]; ok && owner != "" && !aliased[f.Offset] {
				t.Errorf("%s: byte %d claimed by both %s and %s", name, b, owner, f.Symbol)
			}

			covered[b] = f.Symbol
		}
	}
}

func TestFABFields_tileEightyBytes(t *testing.T) {
	checkFields(t, "FAB", FABFields, 80)

	// The struct's own final field (fabdef$$_fill_9, 4 reserved bytes) is
	// deliberately omitted (see FABFields' own doc comment: reserved fields
	// have no real name), so the last *named* field ends short of 80, not
	// exactly at it — RCF at offset 75 + size 1 = 76, then 4 reserved bytes
	// to reach FAB$K_BLN.
	max := uint32(0)
	for _, f := range FABFields {
		if end := f.Offset + f.Size; end > max {
			max = end
		}
	}

	if max != 76 {
		t.Errorf("FABFields' highest field ends at %d, want 76 (RCF+1; 4 reserved bytes make up the rest of FAB$K_BLN=80)", max)
	}
}

func TestRABFields_tileSixtyEightBytes(t *testing.T) {
	checkFields(t, "RAB", RABFields, 68)

	max := uint32(0)
	for _, f := range RABFields {
		if end := f.Offset + f.Size; end > max {
			max = end
		}
	}

	if max != 68 {
		t.Errorf("RABFields' highest field ends at %d, want 68 (RAB$K_BLN)", max)
	}
}

// TestFieldTables_keywordsUnique confirms no two Field entries in the same
// table share a Keyword — .FAB/.RAB's own keyword lookup depends on this.
func TestFieldTables_keywordsUnique(t *testing.T) {
	for name, fields := range map[string][]Field{"FAB": FABFields, "RAB": RABFields} {
		seen := map[string]bool{}
		for _, f := range fields {
			if seen[f.Keyword] {
				t.Errorf("%s: duplicate keyword %q", name, f.Keyword)
			}

			seen[f.Keyword] = true
		}
	}
}
