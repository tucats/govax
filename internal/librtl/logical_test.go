package librtl

import (
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vmsdef"
)

var (
	ssSupersedeTest = vmsdef.Symbols["SS$_SUPERSEDE"]
	ssNoLogNamTest  = vmsdef.Symbols["SS$_NOLOGNAM"]
	ssBadParamTest  = vmsdef.Symbols["SS$_BADPARAM"]
	ssIvLogNamTest  = vmsdef.Symbols["SS$_IVLOGNAM"]
	lnmStringItem   = vmsdef.Symbols["LNM$_STRING"]
	lnmAttrItem     = vmsdef.Symbols["LNM$_ATTRIBUTES"]
)

// itemList writes a $CRELNM item list of LNM$_STRING items, one per
// value, each preceded by an LNM$_ATTRIBUTES item when its attrs entry
// isn't 0, and returns its address.
func (a *arena) itemList(values []string, attrs []uint32) uint32 {
	var list []uint32

	for i, v := range values {
		if attrs[i] != 0 {
			list = append(list, 4|lnmAttrItem<<16, a.long(attrs[i]), 0)
		}

		list = append(list, uint32(len(v))|lnmStringItem<<16, a.desc(v)+8, 0)
	}

	list = append(list, 0)

	addr := a.next
	a.next += uint32(len(list)) * 4

	for i, v := range list {
		putLongword(a.t, a.env, addr+uint32(i)*4, v)
	}

	return addr
}

// lookupLogical returns name's entry in the tables tabnam designates, or nil.
func lookupLogical(t *testing.T, env *corevms.Environment, tabnam, name string) *lnm.Entry {
	t.Helper()

	e, err := env.Logicals.Translate(tabnam, name, lnm.User, 0)
	if err != nil {
		return nil
	}

	return e
}

// TestLibLogical_noCLI: in a process with no CLI both routines return
// LIB$_NOCLI, even with neither a value nor an item list.
func TestLibLogical_noCLI(t *testing.T) {
	env := fixture(t)
	a := newArena(t, env)

	if r0 := call(t, env, "LIB$SET_LOGICAL", a.desc("X")); r0 != libNoCLI {
		t.Errorf("LIB$SET_LOGICAL: status %#x, want LIB$_NOCLI", r0)
	}

	if r0 := call(t, env, "LIB$DELETE_LOGICAL", a.desc("X")); r0 != libNoCLI {
		t.Errorf("LIB$DELETE_LOGICAL: status %#x, want LIB$_NOCLI", r0)
	}
}

// TestLibSetLogical: the name goes into LNM$PROCESS at supervisor mode,
// its case kept; defining it again is SS$_SUPERSEDE; a named table is
// used; with no value nor item list it's LIB$_INVARG.
func TestLibSetLogical(t *testing.T) {
	env, _, a := cliFixture(t)

	if r0 := call(t, env, "LIB$SET_LOGICAL", a.desc("MyName"), a.desc("Disk:[Dir]")); r0 != ssNormal {
		t.Fatalf("status %#x, want SS$_NORMAL", r0)
	}

	e := lookupLogical(t, env, "LNM$PROCESS", "MyName")
	if e == nil {
		t.Fatal("MyName isn't defined")
	}

	if e.Mode != lnm.Supervisor || e.Table.Name != lnm.ProcessTableName ||
		len(e.Equivalences) != 1 || e.Equivalences[0].Value != "Disk:[Dir]" {
		t.Errorf("entry: mode %s, table %s, values %v", e.Mode, e.Table.Name, e.Equivalences)
	}

	if r0 := call(t, env, "LIB$SET_LOGICAL", a.desc("MyName"), a.desc("Other")); r0 != ssSupersedeTest {
		t.Errorf("second definition: status %#x, want SS$_SUPERSEDE", r0)
	}

	if r0 := call(t, env, "LIB$SET_LOGICAL", a.desc("JOBNAME"), a.desc("v"), a.desc("LNM$JOB")); r0 != ssNormal {
		t.Errorf("LNM$JOB: status %#x", r0)
	} else if e := lookupLogical(t, env, "LNM$JOB", "JOBNAME"); e == nil {
		t.Error("JOBNAME isn't in LNM$JOB")
	}

	if r0 := call(t, env, "LIB$SET_LOGICAL", a.desc("NONE")); r0 != libInvArg {
		t.Errorf("no value: status %#x, want LIB$_INVARG", r0)
	}
}

// TestLibSetLogical_attributes: without an item list the attributes
// longword holds the name's and the value's attributes; with one, only
// the name's (the values' are in the list), the value-string ignored.
func TestLibSetLogical_attributes(t *testing.T) {
	env, _, a := cliFixture(t)

	attrs := a.long(lnm.AttrConfine | lnm.AttrConcealed)
	if r0 := call(t, env, "LIB$SET_LOGICAL", a.desc("HIDDEN"), a.desc("DUA0:"), 0, attrs); r0 != ssNormal {
		t.Fatalf("status %#x", r0)
	}

	e := lookupLogical(t, env, "LNM$PROCESS", "HIDDEN")
	if e == nil || e.Attrs&lnm.AttrConfine == 0 || e.Equivalences[0].Attrs != lnm.AttrConcealed {
		t.Errorf("HIDDEN: %+v", e)
	}

	list := a.itemList([]string{"ONE", "TWO"}, []uint32{0, lnm.AttrTerminal})
	if r0 := call(t, env, "LIB$SET_LOGICAL", a.desc("LIST"), a.desc("ignored"), 0, 0, list); r0 != ssNormal {
		t.Fatalf("item list: status %#x", r0)
	}

	e = lookupLogical(t, env, "LNM$PROCESS", "LIST")
	if e == nil || len(e.Equivalences) != 2 || e.Equivalences[0].Value != "ONE" ||
		e.Equivalences[1].Value != "TWO" || e.Equivalences[1].Attrs != lnm.AttrTerminal {
		t.Errorf("LIST: %+v", e)
	}

	if r0 := call(t, env, "LIB$SET_LOGICAL", a.desc("LIST2"), 0, 0, attrs, list); r0 != ssBadParamTest {
		t.Errorf("value attributes with an item list: status %#x, want SS$_BADPARAM", r0)
	}

	if r0 := call(t, env, "LIB$SET_LOGICAL", a.desc("BAD"), a.desc("v"), 0, a.long(0x80000000)); r0 != ssBadParamTest {
		t.Errorf("unknown attribute: status %#x, want SS$_BADPARAM", r0)
	}
}

// TestLibSetLogical_accvio: a descriptor that can't be read is SS$_ACCVIO
// returned, not signaled, as the manual's status list has it.
func TestLibSetLogical_accvio(t *testing.T) {
	env, _, a := cliFixture(t)

	if r0 := call(t, env, "LIB$SET_LOGICAL", unmapped, a.desc("v")); r0 != ssAccVio {
		t.Errorf("status %#x, want SS$_ACCVIO", r0)
	}

	if r0 := call(t, env, "LIB$DELETE_LOGICAL", unmapped); r0 != ssAccVio {
		t.Errorf("LIB$DELETE_LOGICAL: status %#x, want SS$_ACCVIO", r0)
	}
}

// TestLibDeleteLogical: the supervisor-mode name goes, and a user-mode one
// of the same name with it (as DEASSIGN does); an executive-mode name
// stays, so deleting it is SS$_NOLOGNAM; an empty name is SS$_IVLOGNAM,
// not "every name".
func TestLibDeleteLogical(t *testing.T) {
	env, _, a := cliFixture(t)

	for _, d := range []struct {
		name string
		mode lnm.Mode
	}{{"SUPER", lnm.Supervisor}, {"USER", lnm.User}, {"EXEC", lnm.Executive}} {
		if _, err := env.Logicals.Define(lnm.ProcessTableName, d.name, d.mode, 0,
			[]lnm.Equivalence{{Value: "x"}}); err != nil {
			t.Fatal(err)
		}
	}

	for _, name := range []string{"SUPER", "USER"} {
		if r0 := call(t, env, "LIB$DELETE_LOGICAL", a.desc(name)); r0 != ssNormal {
			t.Errorf("%s: status %#x, want SS$_NORMAL", name, r0)
		}

		if lookupLogical(t, env, "LNM$PROCESS", name) != nil {
			t.Errorf("%s is still defined", name)
		}
	}

	if r0 := call(t, env, "LIB$DELETE_LOGICAL", a.desc("SUPER")); r0 != ssNoLogNamTest {
		t.Errorf("again: status %#x, want SS$_NOLOGNAM", r0)
	}

	if r0 := call(t, env, "LIB$DELETE_LOGICAL", a.desc("EXEC")); r0 != ssNoLogNamTest {
		t.Errorf("EXEC: status %#x, want SS$_NOLOGNAM", r0)
	}

	if lookupLogical(t, env, "LNM$PROCESS", "EXEC") == nil {
		t.Error("EXEC was deleted")
	}

	if r0 := call(t, env, "LIB$DELETE_LOGICAL", a.desc("")); r0 != ssIvLogNamTest {
		t.Errorf("empty name: status %#x, want SS$_IVLOGNAM", r0)
	}

	if lookupLogical(t, env, "LNM$PROCESS", "TT") == nil {
		t.Error("the empty name deleted TT")
	}
}
