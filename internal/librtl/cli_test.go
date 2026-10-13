package librtl

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
)

// fakeCLI is a corevms.CLI over two maps, recording the calls made.
type fakeCLI struct {
	local, global map[string]string
	calls         []string
}

func newFakeCLI() *fakeCLI {
	return &fakeCLI{local: map[string]string{}, global: map[string]string{}}
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

// TestLibCLI_noCLI: in a process with no CLI each symbol routine returns
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
	} {
		if r0 := call(t, env, c.name, c.argv...); r0 != libNoCLI {
			t.Errorf("%s: status %#x, want LIB$_NOCLI", c.name, r0)
		}
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
		{"LIB$GET_SYMBOL", "_X", false},
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
// another indicator, or a value past 255 characters, is LIB$_INVARG.
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

	if r0 := call(t, env, "LIB$SET_SYMBOL", a.desc("X"), a.desc(strings.Repeat("v", 255))); r0 != ssNormal {
		t.Errorf("255-character value: status %#x", r0)
	}

	if r0 := call(t, env, "LIB$SET_SYMBOL", a.desc("X"), a.desc(strings.Repeat("v", 256))); r0 != libInvArg {
		t.Errorf("256-character value: status %#x, want LIB$_INVARG", r0)
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
