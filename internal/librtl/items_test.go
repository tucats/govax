package librtl

import (
	"fmt"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// Addresses the item routine tests use: the item code, a value, a
// process ID or CSID, a name's descriptor and text, a channel word, and
// the output string's descriptor and text. jacketLength is the result
// length's word, as for the jackets.
const (
	itemCode     = 0x4000
	itemValue    = 0x4010
	itemID       = 0x4020
	itemName     = 0x4100
	itemNameText = 0x4200
	itemChannel  = 0x4300
	itemOut      = 0x5000
	itemOutText  = 0x5100
)

// itemFixture is fixture with the item code called name at itemCode,
// an empty dynamic output string, and markers in the value and length.
func itemFixture(t *testing.T, name string) *corevms.Environment {
	t.Helper()

	env := fixture(t)
	setItem(t, env, name)
	putDesc(t, env, itemOut, 0, dscClassD, 0)
	putLongword(t, env, itemValue, 0xEEEEEEEE)
	putLongword(t, env, itemValue+4, 0xEEEEEEEE)
	putLongword(t, env, jacketLength, 0xFFFFFFFF)

	return env
}

// setItem puts the code of the item called name at itemCode.
func setItem(t *testing.T, env *corevms.Environment, name string) {
	t.Helper()

	code, ok := vmsdef.Symbols[name]
	if !ok {
		t.Fatalf("no item %s", name)
	}

	putLongword(t, env, itemCode, code)
}

// itemCase checks a routine's status, its text, and its result length.
func itemCase(t *testing.T, env *corevms.Environment, what string, got, wantStatus uint32, wantText string) {
	t.Helper()

	text := descText(t, env, itemOut)
	if got != wantStatus || text != wantText || resultLength(t, env)&0xFFFF != uint32(len(wantText)) {
		t.Errorf("%s: %#x, %q, length %d; want %#x, %q", what, got, text, resultLength(t, env)&0xFFFF, wantStatus, wantText)
	}
}

// TestGetjpiNumeric: a numeric item's value goes to the longword and its
// text to the string: the PID as eight hexadecimal digits, a count in
// unsigned decimal.
func TestGetjpiNumeric(t *testing.T) {
	env := itemFixture(t, "JPI$_PID")
	pid := env.Process.PID

	got := call(t, env, "LIB$GETJPI", itemCode, 0, 0, itemValue, itemOut, jacketLength)
	itemCase(t, env, "PID", got, ssNormal, fmt.Sprintf("%08X", pid))

	if v := longword(t, env, itemValue); v != pid || longword(t, env, itemValue+4) != 0xEEEEEEEE {
		t.Errorf("PID value %#x, then %#x", v, longword(t, env, itemValue+4))
	}

	setItem(t, env, "JPI$_ASTLM")

	got = call(t, env, "LIB$GETJPI", itemCode, 0, 0, itemValue, itemOut, jacketLength)
	itemCase(t, env, "ASTLM", got, ssNormal, fmt.Sprint(env.Process.ASTLimit))
}

// TestGetjpiForms: the text of Table LIB-5's special items.
func TestGetjpiForms(t *testing.T) {
	env := itemFixture(t, "JPI$_CURPRIV")
	env.Process.CurrentPrivileges = 1<<vmsdef.Symbols["PRV$V_TMPMBX"] | 1<<vmsdef.Symbols["PRV$V_NETMBX"]

	got := call(t, env, "LIB$GETJPI", itemCode, 0, 0, itemValue, itemOut, jacketLength)
	itemCase(t, env, "CURPRIV", got, ssNormal, "TMPMBX,NETMBX")

	// A privilege mask is a quadword.
	if lo, hi := longword(t, env, itemValue), longword(t, env, itemValue+4); uint64(hi)<<32|uint64(lo) != env.Process.CurrentPrivileges {
		t.Errorf("CURPRIV value %#x %#x", hi, lo)
	}

	for _, c := range []struct{ item, want string }{
		{"JPI$_MODE", "INTERACTIVE"},
		{"JPI$_STATE", "CUR"},
		{"JPI$_UIC", env.IdentifierText(env.Process.UIC)},
	} {
		setItem(t, env, c.item)

		got := call(t, env, "LIB$GETJPI", itemCode, 0, 0, 0, itemOut, jacketLength)
		itemCase(t, env, c.item, got, ssNormal, c.want)
	}
}

// TestGetjpiString: a string item goes to the string only, without its
// trailing blanks; with no string argument it is LIB$_INVARG.
func TestGetjpiString(t *testing.T) {
	env := itemFixture(t, "JPI$_USERNAME")
	env.Process.Username = "SYSTEM"

	got := call(t, env, "LIB$GETJPI", itemCode, 0, 0, itemValue, itemOut, jacketLength)
	itemCase(t, env, "USERNAME", got, ssNormal, "SYSTEM")

	if v := longword(t, env, itemValue); v != 0xEEEEEEEE {
		t.Errorf("a string item wrote the value: %#x", v)
	}

	if got := call(t, env, "LIB$GETJPI", itemCode, 0, 0, itemValue); got != libInvArg {
		t.Errorf("no string: %#x, want LIB$_INVARG", got)
	}

	// A fixed string too short for the name is LIB$_STRTRU.
	putDesc(t, env, itemOut, 3, dscClassS, itemOutText)

	if got := call(t, env, "LIB$GETJPI", itemCode, 0, 0, 0, itemOut, jacketLength); got != libStrTru || bytesAt(t, env, itemOutText, 3) != "SYS" || resultLength(t, env)&0xFFFF != 3 {
		t.Errorf("truncated: %#x, %q", got, bytesAt(t, env, itemOutText, 3))
	}
}

// TestGetjpiProcess: a zero process-id gets the caller's PID; a
// process-name that names no process is $GETJPI's SS$_NONEXPR.
func TestGetjpiProcess(t *testing.T) {
	env := itemFixture(t, "JPI$_PRCNAM")
	putLongword(t, env, itemID, 0)

	if got := call(t, env, "LIB$GETJPI", itemCode, itemID, 0, 0, itemOut); got != ssNormal || longword(t, env, itemID) != env.Process.PID {
		t.Errorf("zero PID: %#x, PID %#x", got, longword(t, env, itemID))
	}

	putDescriptor(t, env, itemName, itemNameText, "NOBODY")

	if got := call(t, env, "LIB$GETJPI", itemCode, 0, itemName, 0, itemOut); got != vmsdef.Symbols["SS$_NONEXPR"] {
		t.Errorf("no such process: %#x, want SS$_NONEXPR", got)
	}
}

// TestItemArguments: an item code the service lacks is SS$_BADPARAM, and
// no item code or too many arguments LIB$_WRONUMARG, for each routine.
func TestItemArguments(t *testing.T) {
	env := itemFixture(t, "JPI$_PID")

	for _, name := range []string{"LIB$GETJPI", "LIB$GETDVI", "LIB$GETSYI"} {
		if got := call(t, env, name); got != libWroNumArg {
			t.Errorf("%s(): %#x, want LIB$_WRONUMARG", name, got)
		}

		if got := call(t, env, name, itemCode, 0, 0, 0, 0, 0, 0); got != libWroNumArg {
			t.Errorf("%s with 7: %#x, want LIB$_WRONUMARG", name, got)
		}
	}

	putLongword(t, env, itemCode, 0x7FFF)

	if got := call(t, env, "LIB$GETJPI", itemCode, 0, 0, 0, itemOut); got != vmsdef.Symbols["SS$_BADPARAM"] {
		t.Errorf("unknown item: %#x, want SS$_BADPARAM", got)
	}
}

// TestGetdvi: LIB$GETDVI by channel (a word, by reference) and by name:
// a mailbox's device name, its protection as text, a Boolean, the ACP
// type of a device with none; neither argument is SS$_IVDEVNAM, and a
// name that is no device SS$_NOSUCHDEV.
func TestGetdvi(t *testing.T) {
	env := itemFixture(t, "DVI$_DEVNAM")

	if st, err := env.CallService("SYS$CREMBX", []uint32{0, itemChannel, 0, 0, 0xFF00, 0, 0}); err != nil || st != ssNormal {
		t.Fatalf("$CREMBX: %#x, %v", st, err)
	}

	got := call(t, env, "LIB$GETDVI", itemCode, itemChannel, 0, itemValue, itemOut, jacketLength)
	devnam := descText(t, env, itemOut)

	if got != ssNormal || len(devnam) < 5 || devnam[:4] != "_MBA" {
		t.Fatalf("DEVNAM: %#x, %q", got, devnam)
	}

	for _, c := range []struct{ item, want string }{
		{"DVI$_VPROT", "SYSTEM=RWLP,OWNER=RWLP,GROUP,WORLD"},
		{"DVI$_MBX", "TRUE"},
		{"DVI$_ACPTYPE", "NONE"},
	} {
		setItem(t, env, c.item)

		got := call(t, env, "LIB$GETDVI", itemCode, itemChannel, 0, itemValue, itemOut, jacketLength)
		itemCase(t, env, c.item, got, ssNormal, c.want)
	}

	if v := longword(t, env, itemValue); v != 0 {
		t.Errorf("ACPTYPE value %d, want 0", v)
	}

	// By name, the colon and what follows ignored.
	setItem(t, env, "DVI$_DEVNAM")
	putDescriptor(t, env, itemName, itemNameText, devnam[1:])

	got = call(t, env, "LIB$GETDVI", itemCode, 0, itemName, 0, itemOut, jacketLength)
	itemCase(t, env, "by name", got, ssNormal, devnam)

	if got := call(t, env, "LIB$GETDVI", itemCode, 0, 0, 0, itemOut); got != vmsdef.Symbols["SS$_IVDEVNAM"] {
		t.Errorf("neither: %#x, want SS$_IVDEVNAM", got)
	}

	putDescriptor(t, env, itemName, itemNameText, "XYZ0:")

	if got := call(t, env, "LIB$GETDVI", itemCode, 0, itemName, 0, itemOut); got != vmsdef.Symbols["SS$_NOSUCHDEV"] {
		t.Errorf("no such device: %#x, want SS$_NOSUCHDEV", got)
	}
}

// TestGetsyi: LIB$GETSYI's strings lose their trailing blanks, numbers
// are unsigned decimal (the boot time's quadword too), a zero CSID is
// written back, and another node's name is SS$_NOSUCHNODE.
func TestGetsyi(t *testing.T) {
	env := itemFixture(t, "SYI$_VERSION")

	got := call(t, env, "LIB$GETSYI", itemCode, 0, itemOut, jacketLength)
	itemCase(t, env, "VERSION", got, ssNormal, corevms.VersionString)

	setItem(t, env, "SYI$_BOOTTIME")
	env.BootTime = 0x00A1B2C3D4E5F607

	got = call(t, env, "LIB$GETSYI", itemCode, itemValue, itemOut, jacketLength)
	itemCase(t, env, "BOOTTIME", got, ssNormal, fmt.Sprint(env.BootTime))

	if lo, hi := longword(t, env, itemValue), longword(t, env, itemValue+4); uint64(hi)<<32|uint64(lo) != env.BootTime {
		t.Errorf("BOOTTIME value %#x %#x", hi, lo)
	}

	setItem(t, env, "SYI$_NODENAME")
	putLongword(t, env, itemID, 0)
	putDescriptor(t, env, itemName, itemNameText, env.NodeName)

	got = call(t, env, "LIB$GETSYI", itemCode, 0, itemOut, jacketLength, itemID, itemName)
	itemCase(t, env, "NODENAME", got, ssNormal, env.NodeName)

	putDescriptor(t, env, itemName, itemNameText, "OTHER")

	if got := call(t, env, "LIB$GETSYI", itemCode, 0, itemOut, 0, 0, itemName); got != vmsdef.Symbols["SS$_NOSUCHNODE"] {
		t.Errorf("another node: %#x, want SS$_NOSUCHNODE", got)
	}
}
