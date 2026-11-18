package sema

import "testing"

// TestScopeChain checks a scope inside a unit's, as a statement function's
// is: its own names hide the host's, other names are found in the host, a
// new name is entered in the host's table and list, and the implicit rules
// are the host's.
func TestScopeChain(t *testing.T) {
	u := &Unit{}
	unit := newUnitScope(u, defaultImplicit(IntegerType), nil)
	x := &Symbol{Name: "X"}
	unit.enter(x)

	inner := newInnerScope(unit)
	dummy := &Symbol{Name: "X"}
	inner.define(dummy)

	if got := inner.find("X"); got != dummy {
		t.Errorf("inner X is %p; want the dummy %p", got, dummy)
	}

	if got := unit.find("X"); got != x {
		t.Errorf("unit X is %p; want the unit's %p", got, x)
	}

	y := &Symbol{Name: "Y"}
	inner.enter(y)

	if unit.find("Y") != y || len(u.Symbols) != 2 || u.Symbols[1] != y {
		t.Errorf("Y entered from the inner scope isn't the unit's: list %v", u.Symbols)
	}

	if inner.find("Z") != nil {
		t.Error("find entered Z")
	}

	if inner.rules() != unit.implicit {
		t.Error("the inner scope's implicit rules aren't the unit's")
	}
}
