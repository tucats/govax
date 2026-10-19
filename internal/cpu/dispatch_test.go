package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vm"
)

func TestTableHandlerForDefaultsToUnimplemented(t *testing.T) {
	inst := &Instruction{Name: "TESTOP"}
	table := newTable([]*Instruction{inst})

	h := table.HandlerFor(inst)
	err := h(nil, nil)

	var f *Fault
	if !errors.As(err, &f) || f.Code != ExcPrivileged {
		t.Fatalf("unimplemented handler err = %v, want *Fault{Code: ExcPrivileged}", err)
	}
}

func TestTableSetHandler(t *testing.T) {
	inst := &Instruction{Name: "TESTOP"}
	table := newTable([]*Instruction{inst})

	called := false

	table.SetHandler(inst, func(e *Engine, d *Decoded) error {
		called = true
		
		return nil
	})

	if err := table.HandlerFor(inst)(nil, nil); err != nil {
		t.Fatalf("registered handler returned %v", err)
	}

	if !called {
		t.Error("registered handler was not invoked")
	}
}

func TestWrapMemErrorTranslationNotValid(t *testing.T) {
	src := &vm.TranslationFault{Kind: vm.TranslationNotValid, Addr: 0x1000}
	err := wrapMemError(src)

	var f *Fault

	if !errors.As(err, &f) {
		t.Fatalf("err = %v, want *Fault", err)
	}

	if f.Code != ExcTranslationNV {
		t.Errorf("Code = %#x, want ExcTranslationNV", f.Code)
	}

	if len(f.Args) != 2 || f.Args[0] != 0x1000 || f.Args[1] != 0 {
		t.Errorf("Args = %v, want [0x1000, 0]", f.Args)
	}
}

func TestWrapMemErrorAccessViolation(t *testing.T) {
	src := &vm.TranslationFault{Kind: vm.AccessViolation, Addr: 0x2000, Mask: vm.MaskLength}
	err := wrapMemError(src)

	var f *Fault

	if !errors.As(err, &f) {
		t.Fatalf("err = %v, want *Fault", err)
	}

	if f.Code != ExcAccessViol {
		t.Errorf("Code = %#x, want ExcAccessViol", f.Code)
	}

	if len(f.Args) != 2 || f.Args[0] != 0x2000 || f.Args[1] != 1 {
		t.Errorf("Args = %v, want [0x2000, 1]", f.Args)
	}
}

// TestWrapMemErrorProtectionViolation checks that a protection
// violation's reason mask is the architecture's: no length bit, and the
// write bit for a write (Phase 67; it reported 2 before, see
// docs/DEVIATIONS.md).
func TestWrapMemErrorProtectionViolation(t *testing.T) {
	src := &vm.TranslationFault{Kind: vm.ProtectionViolation, Addr: 0x2000, Mask: vm.MaskWrite}
	err := wrapMemError(src)

	var f *Fault

	if !errors.As(err, &f) {
		t.Fatalf("err = %v, want *Fault", err)
	}

	if f.Code != ExcAccessViol {
		t.Errorf("Code = %#x, want ExcAccessViol", f.Code)
	}

	if len(f.Args) != 2 || f.Args[0] != 0x2000 || f.Args[1] != vm.MaskWrite {
		t.Errorf("Args = %v, want [0x2000, 4]", f.Args)
	}
}

func TestWrapMemErrorPhysicalAddress(t *testing.T) {
	src := &vm.PhysicalAddressError{Addr: 0x3000}
	err := wrapMemError(src)

	var f *Fault

	if !errors.As(err, &f) {
		t.Fatalf("err = %v, want *Fault", err)
	}

	if f.Code != ExcAccessViol || len(f.Args) != 2 || f.Args[0] != 0x3000 {
		t.Errorf("f = %+v, want {ExcAccessViol, [0x3000, 1]}", f)
	}
}

func TestWrapMemErrorPassesThroughOther(t *testing.T) {
	original := &Fault{Code: ExcReservedAddr}
	if got := wrapMemError(original); got != error(original) {
		t.Errorf("wrapMemError(*Fault) = %v, want unchanged", got)
	}
	
	if wrapMemError(nil) != nil {
		t.Error("wrapMemError(nil) != nil")
	}
}
