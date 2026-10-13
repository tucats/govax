package librtl

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// fakeCLI is a corevms.CLI over two maps, recording the calls made.
type fakeCLI struct {
	local, global map[string]string
	calls         []string

	// chained and program are the last Chain's.
	chained string
	program bool

	// keys are the control keys enabled, as a LIB$M_CLI_ mask.
	keys uint32

	// pauseStatus and pauseErr are what Pause returns.
	pauseStatus uint32
	pauseErr    error

	// attachStatus is what Attach returns.
	attachStatus uint32
}

func (f *fakeCLI) Pause() (uint32, error) {
	f.calls = append(f.calls, "pause")

	return f.pauseStatus, f.pauseErr
}

func (f *fakeCLI) Attach(pid uint32) uint32 {
	f.calls = append(f.calls, fmt.Sprintf("attach %08X", pid))

	return f.attachStatus
}

func (f *fakeCLI) ControlKeys(enable, disable uint32) uint32 {
	f.calls = append(f.calls, "control")
	old := f.keys
	f.keys = (f.keys | enable) &^ disable

	return old
}

func (f *fakeCLI) Chain(text string, program bool) {
	f.calls = append(f.calls, "chain "+text)
	f.chained, f.program = text, program
}

func newFakeCLI() *fakeCLI {
	return &fakeCLI{local: map[string]string{}, global: map[string]string{}, keys: corevms.CLICtrlY}
}

func (f *fakeCLI) Symbol(name string) (string, uint32, uint32) {
	f.calls = append(f.calls, "get "+name)

	if v, ok := f.local[name]; ok {
		return v, corevms.CLILocalSymbol, ssNormal
	}

	if v, ok := f.global[name]; ok {
		return v, corevms.CLIGlobalSymbol, ssNormal
	}

	return "", 0, libNoSuchSymTest
}

func (f *fakeCLI) table(t uint32) map[string]string {
	if t == corevms.CLIGlobalSymbol {
		return f.global
	}

	return f.local
}

func (f *fakeCLI) SetSymbol(name, value string, table uint32) uint32 {
	f.calls = append(f.calls, "set "+name)
	f.table(table)[name] = value

	return ssNormal
}

func (f *fakeCLI) DeleteSymbol(name string, table uint32) uint32 {
	f.calls = append(f.calls, "delete "+name)

	if _, ok := f.table(table)[name]; !ok {
		return libNoSuchSymTest
	}

	delete(f.table(table), name)

	return ssNormal
}

// libNoSuchSymTest stands for LIB$_NOSUCHSYM, which the CLI, not this
// package, returns.
const libNoSuchSymTest = 0x158364

// cliFixture is fixture with a fakeCLI as the process's CLI.
func cliFixture(t *testing.T) (*corevms.Environment, *fakeCLI, *arena) {
	env := fixture(t)
	cli := newFakeCLI()
	env.CLI = cli

	return env, cli, newArena(t, env)
}

// TestLibCLI_noCLI: in a process with no CLI each CLI routine returns
// LIB$_NOCLI, even for a name it would refuse.
func TestLibCLI_noCLI(t *testing.T) {
	env := fixture(t)
	a := newArena(t, env)

	for _, c := range []struct {
		name string
		argv []uint32
	}{
		{"LIB$GET_SYMBOL", []uint32{a.desc("1X"), a.desc("          ")}},
		{"LIB$SET_SYMBOL", []uint32{a.desc("X"), a.desc("v")}},
		{"LIB$DELETE_SYMBOL", []uint32{a.desc("X")}},
		{"LIB$DISABLE_CTRL", []uint32{a.long(1)}},
		{"LIB$ENABLE_CTRL", []uint32{a.long(corevms.CLICtrlY)}},
		{"LIB$PAUSE", nil},
		{"LIB$ATTACH", []uint32{a.long(0x10203)}},
	} {
		if r0 := call(t, env, c.name, c.argv...); r0 != libNoCLI {
			t.Errorf("%s: status %#x, want LIB$_NOCLI", c.name, r0)
		}
	}

	// The control-key routines write old-mask as 0 even so (VMS V7.1).
	old := a.long(0x55555555)
	if r0 := call(t, env, "LIB$DISABLE_CTRL", a.long(corevms.CLICtrlY), old); r0 != libNoCLI || longword(t, env, old) != 0 {
		t.Errorf("DISABLE_CTRL: status %#x, old-mask %#x; want LIB$_NOCLI, 0", r0, longword(t, env, old))
	}
}

// TestLibGetSymbol: the name is uppercased and its trailing blanks
// dropped; the value goes into a fixed string, with its length and the
// table it was in; a value too long is LIB$_STRTRU.
func TestLibGetSymbol(t *testing.T) {
	env, cli, a := cliFixture(t)
	cli.global["GREETING"] = "hello there"

	result, length, table := a.desc(strings.Repeat("x", 16)), a.word(0), a.long(0)

	if r0 := call(t, env, "LIB$GET_SYMBOL", a.desc("greeting  "), result, length, table); r0 != ssNormal {
		t.Fatalf("status %#x, want SS$_NORMAL", r0)
	}

	if got := descText(t, env, result); got != "hello there     " {
		t.Errorf("result = %q", got)
	}

	if got := word(t, env, length); got != 11 {
		t.Errorf("length = %d, want 11", got)
	}

	if got := longword(t, env, table); got != corevms.CLIGlobalSymbol {
		t.Errorf("table = %d, want 2", got)
	}

	short := a.desc("xxxxx")
	if r0 := call(t, env, "LIB$GET_SYMBOL", a.desc("GREETING"), short, length); r0 != libStrTru {
		t.Errorf("short result: status %#x, want LIB$_STRTRU", r0)
	}

	if got, n := descText(t, env, short), word(t, env, length); got != "hello" || n != 5 {
		t.Errorf("short result = %q, length %d", got, n)
	}

	if r0 := call(t, env, "LIB$GET_SYMBOL", a.desc("NOSUCH"), result); r0 != libNoSuchSymTest {
		t.Errorf("missing symbol: status %#x, want LIB$_NOSUCHSYM", r0)
	}
}

// TestLibCLI_symbolNames: the names each routine refuses with
// LIB$_INVSYMNAM, without asking the CLI: empty or blank, longer than 255
// characters, or starting with a character the routine doesn't allow
// (GET_SYMBOL: a letter or "$"; SET and DELETE also "_").
func TestLibCLI_symbolNames(t *testing.T) {
	env, cli, a := cliFixture(t)
	out := a.desc(strings.Repeat(" ", 8))

	cases := []struct {
		routine, name string
		ok            bool
	}{
		{"LIB$GET_SYMBOL", "", false},
		{"LIB$GET_SYMBOL", "   ", false},
		{"LIB$GET_SYMBOL", "1X", false},
		{"LIB$GET_SYMBOL", "-X", false},
		{"LIB$GET_SYMBOL", "_X", true}, // VMS V7.1, testdata/lib57
		{"LIB$GET_SYMBOL", "$X", true},
		{"LIB$GET_SYMBOL", strings.Repeat("A", 255), true},
		{"LIB$GET_SYMBOL", strings.Repeat("A", 256), false},
		{"LIB$SET_SYMBOL", "_X", true},
		{"LIB$SET_SYMBOL", "1X", false},
		{"LIB$DELETE_SYMBOL", "_X", true},
		{"LIB$DELETE_SYMBOL", "-X", false},
	}

	for _, c := range cases {
		cli.calls = nil

		var argv []uint32

		switch c.routine {
		case "LIB$GET_SYMBOL":
			argv = []uint32{a.desc(c.name), out}
		case "LIB$SET_SYMBOL":
			argv = []uint32{a.desc(c.name), a.desc("v")}
		default:
			argv = []uint32{a.desc(c.name)}
		}

		r0 := call(t, env, c.routine, argv...)

		if refused := r0 == libInvSymNam; refused == c.ok {
			t.Errorf("%s of %q: status %#x", c.routine, c.name, r0)
		}

		if asked := len(cli.calls) > 0; asked != c.ok {
			t.Errorf("%s of %q: CLI calls %v", c.routine, c.name, cli.calls)
		}
	}
}

// TestLibSetDeleteSymbol: SET_SYMBOL keeps the value's trailing blanks,
// in the local table by default and the global one for indicator 2;
// another indicator, or a value past 4096 characters, is LIB$_INVARG.
// DELETE_SYMBOL deletes from the table named, LIB$_NOSUCHSYM if it isn't
// there.
func TestLibSetDeleteSymbol(t *testing.T) {
	env, cli, a := cliFixture(t)

	if r0 := call(t, env, "LIB$SET_SYMBOL", a.desc("loc"), a.desc("a value  ")); r0 != ssNormal {
		t.Fatalf("SET local: status %#x", r0)
	}

	if r0 := call(t, env, "LIB$SET_SYMBOL", a.desc("GLO"), a.desc("g"), a.long(2)); r0 != ssNormal {
		t.Fatalf("SET global: status %#x", r0)
	}

	if cli.local["LOC"] != "a value  " || cli.global["GLO"] != "g" {
		t.Errorf("local %v, global %v", cli.local, cli.global)
	}

	for _, indicator := range []uint32{0, 3} {
		if r0 := call(t, env, "LIB$SET_SYMBOL", a.desc("X"), a.desc("v"), a.long(indicator)); r0 != libInvArg {
			t.Errorf("SET with indicator %d: status %#x, want LIB$_INVARG", indicator, r0)
		}

		if r0 := call(t, env, "LIB$DELETE_SYMBOL", a.desc("LOC"), a.long(indicator)); r0 != libInvArg {
			t.Errorf("DELETE with indicator %d: status %#x, want LIB$_INVARG", indicator, r0)
		}
	}

	if r0 := call(t, env, "LIB$SET_SYMBOL", a.desc("X"), a.desc(strings.Repeat("v", 4096))); r0 != ssNormal {
		t.Errorf("4096-character value: status %#x", r0)
	}

	if r0 := call(t, env, "LIB$SET_SYMBOL", a.desc("X"), a.desc(strings.Repeat("v", 4097))); r0 != libInvArg {
		t.Errorf("4097-character value: status %#x, want LIB$_INVARG", r0)
	}

	// An omitted value-string, or GET's resultant-string, is
	// LIB$_INVSTRDES, returned (VMS V7.1).
	if r0 := call(t, env, "LIB$SET_SYMBOL", a.desc("X")); r0 != libInvStrDes {
		t.Errorf("SET with one argument: status %#x, want LIB$_INVSTRDES", r0)
	}

	if r0 := call(t, env, "LIB$GET_SYMBOL", a.desc("X")); r0 != libInvStrDes {
		t.Errorf("GET with one argument: status %#x, want LIB$_INVSTRDES", r0)
	}

	if r0 := call(t, env, "LIB$DELETE_SYMBOL", a.desc("GLO")); r0 != libNoSuchSymTest {
		t.Errorf("DELETE of a global from the local table: status %#x, want LIB$_NOSUCHSYM", r0)
	}

	if r0 := call(t, env, "LIB$DELETE_SYMBOL", a.desc("glo"), a.long(2)); r0 != ssNormal {
		t.Errorf("DELETE global: status %#x", r0)
	}

	if _, ok := cli.global["GLO"]; ok {
		t.Error("GLO wasn't deleted")
	}
}

// TestLibCLI_controlMasks: the mask bits are $LIBCLIDEF's.
func TestLibCLI_controlMasks(t *testing.T) {
	for name, want := range map[string]uint32{
		"LIB$M_CLI_CTRLT": corevms.CLICtrlT,
		"LIB$M_CLI_CTRLY": corevms.CLICtrlY,
	} {
		if got := vmsdef.LibrarySymbols[name]; got != want {
			t.Errorf("%s = %#x, want %#x", name, got, want)
		}
	}
}

// TestLibControlKeys: DISABLE_CTRL and ENABLE_CTRL change the keys their
// mask names and write the keys enabled before to old-mask, which
// ENABLE_CTRL can take back; an invalid bit is LIB$_INVARG, nothing
// changed and old-mask not written.
func TestLibControlKeys(t *testing.T) {
	env, cli, a := cliFixture(t)
	old := a.long(0xFFFFFFFF)

	if r0 := call(t, env, "LIB$DISABLE_CTRL", a.long(corevms.CLICtrlY), old); r0 != ssNormal {
		t.Fatalf("DISABLE_CTRL: status %#x", r0)
	}

	if got := longword(t, env, old); got != corevms.CLICtrlY {
		t.Errorf("DISABLE_CTRL old-mask = %#x, want Ctrl/Y", got)
	}

	if cli.keys != 0 {
		t.Errorf("after DISABLE_CTRL keys = %#x, want 0", cli.keys)
	}

	saved := longword(t, env, old)

	if r0 := call(t, env, "LIB$ENABLE_CTRL", a.long(corevms.CLICtrlT), old); r0 != ssNormal {
		t.Fatalf("ENABLE_CTRL T: status %#x", r0)
	}

	if got := longword(t, env, old); got != 0 {
		t.Errorf("ENABLE_CTRL old-mask = %#x, want 0", got)
	}

	// The old mask given back restores Ctrl/Y; no old-mask argument is
	// fine.
	if r0 := call(t, env, "LIB$ENABLE_CTRL", a.long(saved)); r0 != ssNormal {
		t.Fatalf("ENABLE_CTRL saved: status %#x", r0)
	}

	if want := uint32(corevms.CLICtrlT | corevms.CLICtrlY); cli.keys != want {
		t.Errorf("keys = %#x, want %#x", cli.keys, want)
	}

	// A zero mask changes nothing, but reports the keys.
	if r0 := call(t, env, "LIB$DISABLE_CTRL", a.long(0), old); r0 != ssNormal || longword(t, env, old) != cli.keys {
		t.Errorf("zero mask: status %#x, old-mask %#x", r0, longword(t, env, old))
	}

	// An invalid bit changes nothing, but old-mask is written (VMS V7.1).
	for _, routine := range []string{"LIB$DISABLE_CTRL", "LIB$ENABLE_CTRL"} {
		keys := cli.keys
		untouched := a.long(0x12345678)

		if r0 := call(t, env, routine, a.long(corevms.CLICtrlY|1<<3), untouched); r0 != libInvArg {
			t.Errorf("%s of an invalid bit: status %#x, want LIB$_INVARG", routine, r0)
		}

		if cli.keys != keys || longword(t, env, untouched) != keys {
			t.Errorf("%s of an invalid bit: keys %#x (were %#x), old-mask %#x", routine, cli.keys, keys, longword(t, env, untouched))
		}
	}
}

// TestLibPause: LIB$PAUSE returns what the CLI's Pause does (CONTINUE's
// SS$_NORMAL); a pause that ends the image (ErrExit) ends it.
func TestLibPause(t *testing.T) {
	env, cli, _ := cliFixture(t)
	cli.pauseStatus = ssNormal

	if r0 := call(t, env, "LIB$PAUSE"); r0 != ssNormal || len(cli.calls) != 1 || cli.calls[0] != "pause" {
		t.Errorf("status %#x, calls %v; want SS$_NORMAL, one pause", r0, cli.calls)
	}

	cli.pauseStatus, cli.pauseErr = 0x651, corevms.ErrExit

	r0, err := shim(t, env, "LIB$PAUSE")(env, nil)
	if !errors.Is(err, corevms.ErrExit) || r0 != 0x651 {
		t.Errorf("an ended pause: status %#x, %v; want SS$_CONTROLY, ErrExit", r0, err)
	}
}

// TestLibAttach: LIB$ATTACH hands the PID to the CLI and returns the
// CLI's refusal as it is; an unreadable process-id is an access violation,
// signaled. (The terminal's moves and waits are tested with real
// processes, in internal/console's libattach_test.go.)
func TestLibAttach(t *testing.T) {
	env, cli, a := cliFixture(t)

	for _, st := range []uint32{vmsdef.Symbols["SS$_NONEXPR"], vmsdef.LibrarySymbols["LIB$_ATTREQREF"]} {
		cli.attachStatus, cli.calls = st, nil

		if r0 := call(t, env, "LIB$ATTACH", a.long(0x10203)); r0 != st || len(cli.calls) != 1 || cli.calls[0] != "attach 00010203" {
			t.Errorf("status %#x, calls %v; want %#x, one attach of 00010203", r0, cli.calls, st)
		}
	}

	// A PID of 0 is SS$_IVLOGNAM, the CLI not asked (VMS V7.1).
	cli.calls = nil

	if r0 := call(t, env, "LIB$ATTACH", a.long(0)); r0 != vmsdef.Symbols["SS$_IVLOGNAM"] || len(cli.calls) != 0 {
		t.Errorf("PID 0: status %#x, calls %v; want SS$_IVLOGNAM, none", r0, cli.calls)
	}

	if _, err := shim(t, env, "LIB$ATTACH")(env, []uint32{unmapped}); err == nil || len(cli.calls) != 0 {
		t.Errorf("an unreadable process-id: %v, calls %v; want an access violation, no call", err, cli.calls)
	}
}
