package mthrtl

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// fixture is an RTL environment with 1MB of memory, used directly (virtual
// memory off), with this package's routines registered.
func fixture(t *testing.T) *corevms.Environment {
	t.Helper()

	logicals := lnm.NewDatabase(corevms.NominalUIC)
	if err := logicals.DefineProcessNames("_TTA0:"); err != nil {
		t.Fatal(err)
	}

	env, err := corevms.NewEnvironment(corevms.NewSystem(registers.New(), vm.NewMemory(1<<20), iodev.NewDeviceTable(),
		rms.NewMountTable()), logicals, bytes.NewReader(nil), &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}

	Register(env.Shims())

	return env
}

// TestVector holds the stubs to the survey's account of MTHRTL.EXE: 404
// names, 262 of them CALL entries and 142 JSB entries, on 402 entries of 8
// bytes, the two aliases sharing one stub, and each stub registered in its
// kind.
func TestVector(t *testing.T) {
	env := fixture(t)
	links := Links()

	if len(links) != 402 {
		t.Errorf("%d stubs, want 402", len(links))
	}

	jsb := 0
	codes := map[uint32]string{}

	var previous uint32

	for i, l := range links {
		if l.Offset%entrySize != 0 {
			t.Errorf("%s: offset %#x isn't a multiple of %d", l.Name, l.Offset, entrySize)
		}

		if i > 0 && l.Offset <= previous {
			t.Errorf("%s: offset %#x is out of order", l.Name, l.Offset)
		}

		previous = l.Offset

		if other, dup := codes[l.Code]; dup {
			t.Errorf("%s and %s share code %d", l.Name, other, l.Code)
		}

		codes[l.Code] = l.Name

		if l.Code < codeBase || l.Code >= 10000 {
			t.Errorf("%s: code %d is out of MTHRTL's range", l.Name, l.Code)
		}

		if l.Library != Library || l.Code != codeOf(l.Name) {
			t.Errorf("%+v isn't its own entry", l)
		}

		_, isCall := env.Shims().Lookup(l.Code)
		_, isJSB := env.Shims().LookupJSB(l.Code)

		if isCall == l.JSB || isJSB != l.JSB {
			t.Errorf("%s: JSB %v, but registered CALL %v, JSB %v", l.Name, l.JSB, isCall, isJSB)
		}

		if l.JSB {
			jsb++
		}
	}

	if jsb != 142 {
		t.Errorf("%d JSB stubs, want 142", jsb)
	}

	for name, sym := range vmsdef.ImageSymbols {
		if sym.Image != Library {
			continue
		}

		if _, ok := codes[codeOf(name)]; !ok {
			t.Errorf("%s has no stub", name)
		}

		if strings.HasPrefix(name, "OTS$") && jsbName.MatchString(name) {
			t.Errorf("%s: an OTS$ name is a CALL entry", name)
		}
	}

	for _, pair := range [][2]string{{"MTH$IIFIX", "MTH$IINT"}, {"MTH$JIFIX", "MTH$JINT"}} {
		if codeOf(pair[0]) != codeOf(pair[1]) {
			t.Errorf("%s and %s have different stubs", pair[0], pair[1])
		}

		if codes[codeOf(pair[0])] != pair[0] {
			t.Errorf("their stub goes by %s, want %s", codes[codeOf(pair[0])], pair[0])
		}
	}

	if !jsbName.MatchString("MTH$HSINCOS_R7") || jsbName.MatchString("OTS$POWHH_R3") || jsbName.MatchString("MTH$SIN") {
		t.Error("jsbName misjudges a name")
	}
}

// TestTables: every name in the tables is one of the image's, of the kind
// its table says, and listed once (under either alias).
func TestTables(t *testing.T) {
	seen := map[uint32]string{}

	note := func(name string, wantJSB bool) {
		sym, ok := vmsdef.ImageSymbols[name]
		if !ok || sym.Image != Library {
			t.Errorf("%s is not in %s's transfer vector", name, Library)

			return
		}

		if jsbName.MatchString(name) != wantJSB {
			t.Errorf("%s is in the wrong table", name)
		}

		if other, dup := seen[codeOf(name)]; dup {
			t.Errorf("%s and %s are the same entry", name, other)
		}

		seen[codeOf(name)] = name
	}

	for _, r := range Routines {
		note(r.Name, false)
	}

	for _, r := range JSBRoutines {
		note(r.Name, true)
	}
}

// TestRegisterLayouts: a value is laid out in registers as it is in memory,
// the register numbers going up as the addresses do. Each format's value is
// one whose low-order bits are set, so a swapped word shows.
func TestRegisterLayouts(t *testing.T) {
	env := fixture(t)

	for _, f := range []vaxfloat.Format{vaxfloat.F, vaxfloat.D, vaxfloat.G, vaxfloat.H} {
		x, err := vaxfloat.Parse("-1234.56789012345678901234567890123456")
		if err != nil {
			t.Fatal(err)
		}

		v, err := vaxfloat.Round(f, x)
		if err != nil {
			t.Fatal(err)
		}

		const addr = 0x2000

		if err := storeResult(env, f, addr, v); err != nil {
			t.Fatal(err)
		}

		words := registerWords(f, v)
		if len(words) != f.Size()/4 {
			t.Errorf("%v: %d words", f, len(words))
		}

		for i, w := range words {
			got, err := env.Memory().LoadLongword(env.CPU(), addr+uint32(4*i))
			if err != nil {
				t.Fatal(err)
			}

			if got != w {
				t.Errorf("%v: register word %d is %#x, memory's %#x", f, i, w, got)
			}
		}

		// R0 to R(n) by returnFloat, what a CALL entry returns.
		for r := range 4 {
			env.CPU().SetGPR(registers.Reg(r), 0xDEADBEEF)
		}

		r0 := returnFloat(env, f, v)
		if r0 != words[0] {
			t.Errorf("%v: returnFloat = %#x, want %#x", f, r0, words[0])
		}

		for i := 1; i < 4; i++ {
			want := uint32(0xDEADBEEF)
			if i < len(words) {
				want = words[i]
			}

			if got := env.CPU().GPR(registers.Reg(i)); got != want {
				t.Errorf("%v: R%d = %#x, want %#x", f, i, got, want)
			}
		}

		// And through a JSB entry's registers, second result at R4.
		var regs corevms.JSBRegisters

		storeRegisters(f, &regs, 4, v)

		back, err := loadRegisters(f, regs[:], 4)
		if err != nil || back.Cmp(v) != 0 {
			t.Errorf("%v: loadRegisters = %v, %v; want %v", f, back, err, v)
		}

		byRef, err := loadArg(env, f, addr)
		if err != nil || byRef.Cmp(v) != 0 {
			t.Errorf("%v: loadArg = %v, %v; want %v", f, byRef, err, v)
		}
	}

	// The reserved operand, in registers.
	if _, err := loadRegisters(vaxfloat.D, []uint32{reservedR0, 0}, 0); err != vaxfloat.ErrReserved {
		t.Errorf("reserved operand in registers: %v", err)
	}

	if b := vaxfloat.Reserved(vaxfloat.H); b.Lo != reservedR0 || b.Hi != 0 {
		t.Errorf("reservedR0 isn't the reserved operand's first longword: %+v", b)
	}
}

// TestArg: an argument the call didn't pass is the address 0.
func TestArg(t *testing.T) {
	argv := []uint32{7}

	if arg(argv, 0) != 7 || arg(argv, 1) != 0 {
		t.Errorf("arg: %d, %d", arg(argv, 0), arg(argv, 1))
	}
}
