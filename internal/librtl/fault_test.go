package librtl

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vm"
)

// An access violation's reason mask is the translation fault's own, as the
// hardware's would be: its length, PTE reference, and write bits, with the
// write bit added for an error marked as a write's.
func TestFaultParameters(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		va, reason uint32
		ok         bool
	}{
		{"a read past a region's length", &vm.TranslationFault{Kind: vm.AccessViolation, Addr: 0x200, Mask: vm.MaskLength}, 0x200, 1, true},
		{"a write the page's protection refuses", &vm.TranslationFault{Kind: vm.ProtectionViolation, Addr: 0x400, Mask: vm.MaskWrite}, 0x400, 4, true},
		{"a process PTE out of reach", &vm.TranslationFault{Kind: vm.AccessViolation, Addr: 0x600, Mask: vm.MaskLength | vm.MaskPTERef}, 0x600, 3, true},
		{"a read marked as a write's", writing(&vm.TranslationFault{Kind: vm.ProtectionViolation, Addr: 0x800}), 0x800, 4, true},
		{"no memory there", writing(&vm.PhysicalAddressError{Addr: 0xA00}), 0xA00, 4, true},
		{"not a memory error", errors.New("something else"), 0, 0, false},
	}

	for _, c := range cases {
		va, reason, ok := faultParameters(c.err)
		if va != c.va || reason != c.reason || ok != c.ok {
			t.Errorf("%s: %08X, mask %d, %v; want %08X, mask %d, %v", c.name, va, reason, ok, c.va, c.reason, c.ok)
		}
	}
}
