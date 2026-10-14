package librtl

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
)

// The user routines' addresses in the tests: never run, only named in
// the tail calls asked for.
const (
	userGet    = 0x3000
	userFree   = 0x3100
	userReset  = 0x3200
	userDelete = 0x3300
	userArg    = 0x1300 // the user argument's cell
)

// userFixture is a page fixture with a stack, for the tail calls' argument
// lists.
func userFixture(t *testing.T) *corevms.Environment {
	t.Helper()

	e := pageFixture(t)
	e.CPU().SetGPR(registers.SP, 0xF0000)

	return e
}

// createUserZone calls LIB$CREATE_USER_VM_ZONE with a user argument of
// value and the routines given (0 omits one), and returns the zone ID.
func createUserZone(t *testing.T, env *corevms.Environment, value uint32, routines ...uint32) uint32 {
	t.Helper()

	putLongword(t, env, userArg, value)
	putLongword(t, env, vmZoneCell, 0xDEADBEEF)

	if r0 := call(t, env, "LIB$CREATE_USER_VM_ZONE", append([]uint32{vmZoneCell, userArg}, routines...)...); r0 != ssNormal {
		t.Fatalf("LIB$CREATE_USER_VM_ZONE = %#x, want SS$_NORMAL", r0)
	}

	return longword(t, env, vmZoneCell)
}

// tailCallOf runs the routine name with argv and returns the tail call
// it asks for, failing the test unless it asks for one whose argument
// list is at SP, below where SP was.
func tailCallOf(t *testing.T, env *corevms.Environment, name string, argv ...uint32) *corevms.CallRequest {
	t.Helper()

	sp := env.CPU().GPR(registers.SP)
	defer env.CPU().SetGPR(registers.SP, sp)

	_, err := shim(t, env, name)(env, argv)

	var req *corevms.CallRequest
	if !errors.As(err, &req) || !req.Tail {
		t.Fatalf("%s: %v, want a tail call", name, err)
	}

	if req.ArgList != env.CPU().GPR(registers.SP) || req.ArgList >= sp {
		t.Fatalf("%s: argument list at %#x, SP %#x (was %#x)", name, req.ArgList, env.CPU().GPR(registers.SP), sp)
	}

	return req
}

// argList returns the longwords of req's argument list after its count,
// failing the test unless the count is n.
func argList(t *testing.T, env *corevms.Environment, req *corevms.CallRequest, n uint32) []uint32 {
	t.Helper()

	if c := longword(t, env, req.ArgList); c != n {
		t.Fatalf("argument count %d, want %d", c, n)
	}

	args := make([]uint32, n)
	for i := range args {
		args[i] = longword(t, env, req.ArgList+4+4*uint32(i))
	}

	return args
}

// A user-defined zone's heap routines call its routines: GET_VM and
// FREE_VM with their own number-of-bytes and base-address and the user
// argument's copy in the zone's control block, RESET with that alone.
func TestLibUserZoneRoutines(t *testing.T) {
	e := userFixture(t)
	id := createUserZone(t, e, 0x1234, userGet, userFree, userReset, userDelete)

	if id == 0 || id == 0xDEADBEEF || longword(t, e, id) != id {
		t.Fatalf("zone ID %#x, holding %#x; want a control block holding its address", id, longword(t, e, id))
	}

	// The zone keeps a copy: changing the program's cell changes nothing.
	putLongword(t, e, userArg, 99)

	putLongword(t, e, vmSizeCell, 40)
	putLongword(t, e, vmZoneCell, id)

	req := tailCallOf(t, e, "LIB$GET_VM", vmSizeCell, vmAddrCell, vmZoneCell)
	args := argList(t, e, req, 3)

	if req.Routine != userGet || args[0] != vmSizeCell || args[1] != vmAddrCell || longword(t, e, args[2]) != 0x1234 {
		t.Fatalf("GET_VM calls %#x with %#x (user argument %#x); want %#x with the size's and address's cells and 1234",
			req.Routine, args, longword(t, e, args[2]), userGet)
	}

	argAddr := args[2]

	req = tailCallOf(t, e, "LIB$FREE_VM", 0, vmAddrCell, vmZoneCell)
	if args := argList(t, e, req, 3); req.Routine != userFree || args[0] != 0 || args[1] != vmAddrCell || args[2] != argAddr {
		t.Errorf("FREE_VM without a size calls %#x with %#x; want %#x with 0, the address's cell, %#x", req.Routine, args, userFree, argAddr)
	}

	req = tailCallOf(t, e, "LIB$RESET_VM_ZONE", vmZoneCell)
	if args := argList(t, e, req, 1); req.Routine != userReset || args[0] != argAddr {
		t.Errorf("RESET_VM_ZONE calls %#x with %#x; want %#x with %#x", req.Routine, args, userReset, argAddr)
	}

	// GET_VM checks its size first.
	putLongword(t, e, vmSizeCell, 0)

	if r0 := call(t, e, "LIB$GET_VM", vmSizeCell, vmAddrCell, vmZoneCell); r0 != libBadBlockSize {
		t.Errorf("GET_VM of 0 bytes = %#x, want LIB$_BADBLOSIZ", r0)
	}

	// The zone isn't the RTL's to manage.
	if _, ok := e.ZoneSettingsOf(id); ok {
		t.Error("ZoneSettingsOf a user-defined zone: ok, want not")
	}

	// DELETE forgets the zone, then calls its routine with a copy of the
	// user argument (the control block is gone).
	req = tailCallOf(t, e, "LIB$DELETE_VM_ZONE", vmZoneCell)
	if args := argList(t, e, req, 1); req.Routine != userDelete || longword(t, e, args[0]) != 0x1234 || args[0] == argAddr {
		t.Errorf("DELETE_VM_ZONE calls %#x with %#x; want %#x with a copy of 1234", req.Routine, args, userDelete)
	}

	if r0 := zoneCall(t, e, "LIB$DELETE_VM_ZONE", id); r0 != libBadBlockAddress {
		t.Errorf("DELETE_VM_ZONE again = %#x, want LIB$_BADBLOADR", r0)
	}

	if longword(t, e, id) == id {
		t.Error("the deleted zone's control block still holds its ID")
	}
}

// A routine not given is LIB$_INVOPEZON; without a delete routine the
// zone can't be deleted. Omitted arguments are a user argument of 0 and
// no name.
func TestLibUserZoneMissingRoutines(t *testing.T) {
	e := userFixture(t)

	putLongword(t, e, vmZoneCell, 0)

	if r0 := call(t, e, "LIB$CREATE_USER_VM_ZONE", vmZoneCell); r0 != ssNormal {
		t.Fatalf("LIB$CREATE_USER_VM_ZONE with only zone-id = %#x", r0)
	}

	id := longword(t, e, vmZoneCell)

	u, argAddr, ok := e.UserZoneOf(id)
	if !ok || u.Name != "" || longword(t, e, argAddr) != 0 {
		t.Fatalf("UserZoneOf = %+v, %#x, %v; want no name, an argument of 0", u, argAddr, ok)
	}

	putLongword(t, e, vmSizeCell, 8)

	for _, c := range []struct {
		name string
		argv []uint32
	}{
		{"LIB$GET_VM", []uint32{vmSizeCell, vmAddrCell, vmZoneCell}},
		{"LIB$FREE_VM", []uint32{vmSizeCell, vmAddrCell, vmZoneCell}},
		{"LIB$RESET_VM_ZONE", []uint32{vmZoneCell}},
		{"LIB$DELETE_VM_ZONE", []uint32{vmZoneCell}},
	} {
		if r0 := call(t, e, c.name, c.argv...); r0 != libInvOpeZon {
			t.Errorf("%s = %#x, want LIB$_INVOPEZON", c.name, r0)
		}
	}

	if _, _, ok := e.UserZoneOf(id); !ok {
		t.Error("the zone is gone after a DELETE_VM_ZONE without its routine")
	}
}

// A user-defined zone's name is read as any string; a bad descriptor is
// LIB$_INVSTRDES, and no zone is made.
func TestLibUserZoneName(t *testing.T) {
	e := userFixture(t)

	putLongword(t, e, vmNameDesc, 4|0x010E<<16)
	putLongword(t, e, vmNameDesc+4, vmNameText)

	putBytes(t, e, vmNameText, "USER")

	if r0 := call(t, e, "LIB$CREATE_USER_VM_ZONE", vmZoneCell, 0, 0, 0, 0, 0, vmNameDesc); r0 != ssNormal {
		t.Fatalf("LIB$CREATE_USER_VM_ZONE named = %#x", r0)
	}

	if u, _, _ := e.UserZoneOf(longword(t, e, vmZoneCell)); u.Name != "USER" {
		t.Errorf("name %q, want USER", u.Name)
	}

	putLongword(t, e, vmNameDesc, 4|0x0F0E<<16) // class 15: no such class
	putLongword(t, e, vmZoneCell, 0)

	if r0 := call(t, e, "LIB$CREATE_USER_VM_ZONE", vmZoneCell, 0, 0, 0, 0, 0, vmNameDesc); r0 != libInvStrDes {
		t.Errorf("a bad name's descriptor = %#x, want LIB$_INVSTRDES", r0)
	}

	if longword(t, e, vmZoneCell) != 0 {
		t.Error("zone-id written after an error")
	}
}
