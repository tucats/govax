package machine

import (
	"bytes"
	"slices"
	"testing"

	"github.com/tucats/govax/internal/corevms"
)

// TestLegacyImageNames checks the shareable images a V1 image's global
// section ISDs name: DUNGEON.EXE's three VMSRTL ISDs are one image, and a
// name without a "_nnn" suffix, or a private ISD's, names none.
func TestLegacyImageNames(t *testing.T) {
	icb := &ICB{ISDList: []*ISD{
		{Name: "<NONE>"},
		{Name: "VMSRTL_001", Flags: isdGBL},
		{Name: "VMSRTL_002", Flags: isdGBL},
		{Name: "OTHER_001", Flags: isdGBL},
		{Name: "vmsrtl_003", Flags: isdGBL},
		{Name: "NOSUFFIX", Flags: isdGBL},
		{Name: "TRAILING_", Flags: isdGBL},
		{Name: "PRIVATE_001"},
	}}

	if got, want := legacyImageNames(icb), []string{"VMSRTL", "OTHER"}; !slices.Equal(got, want) {
		t.Errorf("legacyImageNames = %q, want %q", got, want)
	}

	if s := linkedSection(icb, "VMSRTL"); s == nil || s.Name != "VMSRTL_001" {
		t.Errorf("the first VMSRTL section is %+v, want VMSRTL_001", s)
	}
}

// TestUnresolvedRuntimeStub checks that a G^ cell into a run-time library
// govax stands for, at an offset no shim has, resolves to a stub of its
// own (unresolved.go): a CALL stub whose code the System's shims have,
// the same one for the same entry, the next for another; and that a cell
// into an image of the user's still doesn't resolve.
func TestUnresolvedRuntimeStub(t *testing.T) {
	c := newBootableMachine(t)
	p := c.images()

	first, err := p.resolveFixupTarget(&SHR{Name: "BASRTL"}, 0x88)
	if err != nil {
		t.Fatalf("BASRTL+88: %v", err)
	}

	again, err := p.resolveFixupTarget(&SHR{Name: "basrtl"}, 0x88)
	if err != nil || again != first {
		t.Errorf("BASRTL+88 again: %X (%v), want %X", again, err, first)
	}

	next, err := p.resolveFixupTarget(&SHR{Name: "COBRTL"}, 0x10)
	if err != nil || next != first+corevms.ShimStubSize {
		t.Errorf("COBRTL+10: %X (%v), want %X", next, err, first+corevms.ShimStubSize)
	}

	if first < 0x80000000 {
		t.Errorf("stub at %X, want it in S0", first)
	}

	got, err := p.loadBytes(first, corevms.ShimStubSize)
	if want := corevms.ShimStub(unresolvedCodeBase); err != nil || !bytes.Equal(got, want) {
		t.Errorf("stub = % X (%v), want % X", got, err, want)
	}

	for _, code := range []uint32{unresolvedCodeBase, unresolvedCodeBase + 1} {
		if _, ok := c.processOne.Shims().Lookup(code); !ok {
			t.Errorf("code %d isn't registered", code)
		}
	}

	if _, err := p.resolveFixupTarget(&SHR{Name: "MYSHR"}, 0x88); err == nil {
		t.Error("MYSHR+88 resolved, want LIB$_UNRESOLVED")
	}
}
