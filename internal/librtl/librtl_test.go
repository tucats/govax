package librtl

import (
	"bytes"
	"slices"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// fixture is an RTL environment with 1MB of memory, used directly
// (virtual memory off), with this package's routines registered.
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

func putLongword(t *testing.T, env *corevms.Environment, addr, v uint32) {
	t.Helper()

	if err := env.Memory().StoreLongword(env.CPU(), addr, v); err != nil {
		t.Fatalf("StoreLongword(%#x): %v", addr, err)
	}
}

func longword(t *testing.T, env *corevms.Environment, addr uint32) uint32 {
	t.Helper()

	v, err := env.Memory().LoadLongword(env.CPU(), addr)
	if err != nil {
		t.Fatalf("LoadLongword(%#x): %v", addr, err)
	}

	return v
}

// putDescriptor writes a fixed-length string descriptor for s at addr,
// with the string at strAddr.
func putDescriptor(t *testing.T, env *corevms.Environment, addr, strAddr uint32, s string) {
	t.Helper()

	mem, cpu := env.Memory(), env.CPU()

	if err := mem.StoreWord(cpu, addr, uint16(len(s))); err != nil {
		t.Fatal(err)
	}

	if err := mem.StoreWord(cpu, addr+2, 0x010E); err != nil { // DSC$K_DTYPE_T, DSC$K_CLASS_S
		t.Fatal(err)
	}

	putLongword(t, env, addr+4, strAddr)

	for i := 0; i < len(s); i++ {
		if err := mem.StoreByte(cpu, strAddr+uint32(i), s[i]); err != nil {
			t.Fatal(err)
		}
	}
}

// shim returns the routine registered as name, as a program's call
// would reach it through the shim table.
func shim(t *testing.T, env *corevms.Environment, name string) corevms.ShimFunc {
	t.Helper()

	for _, r := range Routines {
		if r.Name != name {
			continue
		}

		fn, ok := env.Shims().Lookup(r.Code)
		if !ok {
			t.Fatalf("%s (code %d) isn't registered", name, r.Code)
		}

		return fn
	}

	t.Fatalf("no routine %s", name)

	return nil
}

// call runs the routine registered as name through the shim table, as a
// program's call would reach it.
func call(t *testing.T, env *corevms.Environment, name string, argv ...uint32) uint32 {
	t.Helper()

	r0, err := shim(t, env, name)(env, argv)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}

	return r0
}

// TestRoutinesMatchLIBRTL: every routine is an entry in VMS 7.3's
// LIBRTL.EXE transfer vector, as LINK knows it (so Offset has one), names
// and Go routines' codes are distinct, and a routine has Go code exactly
// when it has a code (code 0 is the microkernel's).
func TestRoutinesMatchLIBRTL(t *testing.T) {
	codes := map[uint32]string{}
	names := map[string]bool{}

	for _, r := range Routines {
		if sym, ok := vmsdef.ImageSymbols[r.Name]; !ok || !slices.Contains(libraries, sym.Image) {
			t.Errorf("%s: not in %s's or %s's transfer vector (vmsdef.ImageSymbols)", r.Name, Library, Library2)
		}

		if (r.Code == 0) != (r.Fn == nil) {
			t.Errorf("%s: code %d with Fn nil %v; want Go code exactly when the code isn't 0", r.Name, r.Code, r.Fn == nil)
		}

		if other, dup := codes[r.Code]; dup && r.Code != 0 {
			t.Errorf("%s and %s share code %d", r.Name, other, r.Code)
		}

		if names[r.Name] {
			t.Errorf("%s is listed twice", r.Name)
		}

		codes[r.Code], names[r.Name] = r.Name, true
	}
}

func TestStrUpcase(t *testing.T) {
	env := fixture(t)
	putDescriptor(t, env, 0x1000, 0x1100, "Hello, World!")

	if r0 := call(t, env, "STR$UPCASE", 0x1000); r0 != 1 {
		t.Errorf("r0 = %d, want 1", r0)
	}

	got, err := readString(env, 0x1000)
	if err != nil || got != "HELLO, WORLD!" {
		t.Errorf("upcased = %q, %v; want HELLO, WORLD!", got, err)
	}
}

// TestLibEstablishRevert: LIB$ESTABLISH and LIB$REVERT change the handler
// of the caller's frame, whose FP the running shim's frame saved at 12.
func TestLibEstablishRevert(t *testing.T) {
	env := fixture(t)

	const shimFP, callerFP = 0x2000, 0x3000

	env.CPU().SetGPR(registers.FP, shimFP)
	putLongword(t, env, shimFP+12, callerFP)
	putLongword(t, env, callerFP, 0x1111)

	if old := call(t, env, "LIB$ESTABLISH", 0x5555); old != 0x1111 {
		t.Errorf("LIB$ESTABLISH = %#x, want the old handler 0x1111", old)
	}

	if got := longword(t, env, callerFP); got != 0x5555 {
		t.Errorf("caller's handler = %#x, want 0x5555", got)
	}

	if old := call(t, env, "LIB$REVERT"); old != 0x5555 {
		t.Errorf("LIB$REVERT = %#x, want 0x5555", old)
	}

	if got := longword(t, env, callerFP); got != 0 {
		t.Errorf("caller's handler = %#x, want 0", got)
	}
}

// TestLibSigToRetOutsideHandler: with no condition being handled, or too
// few arguments, LIB$SIG_TO_RET returns SS$_NOSIGNAL.
func TestLibSigToRetOutsideHandler(t *testing.T) {
	env := fixture(t)

	if r0 := call(t, env, "LIB$SIG_TO_RET"); r0 != ssNoSignal {
		t.Errorf("no arguments: %#x, want SS$_NOSIGNAL", r0)
	}

	if r0 := call(t, env, "LIB$SIG_TO_RET", 0x1000, 0x1100); r0 != ssNoSignal {
		t.Errorf("no condition: %#x, want SS$_NOSIGNAL", r0)
	}
}

func TestLibSignalNeedsCondition(t *testing.T) {
	env := fixture(t)

	fn, _ := env.Shims().Lookup(33)
	if _, err := fn(env, nil); err == nil {
		t.Error("LIB$SIGNAL with no condition value: want an error")
	}
}

// callJSB runs the JSB routine registered as name through the shim table
// with R0, R1, ... set to in (the rest 0), and returns the registers it
// leaves.
func callJSB(t *testing.T, env *corevms.Environment, name string, in ...uint32) corevms.JSBRegisters {
	t.Helper()

	for _, r := range JSBRoutines {
		if r.Name != name {
			continue
		}

		fn, ok := env.Shims().LookupJSB(r.Code)
		if !ok {
			t.Fatalf("%s (code %d) isn't registered", name, r.Code)
		}

		var regs corevms.JSBRegisters

		copy(regs[:], in)

		if err := fn(env, &regs); err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		return regs
	}

	t.Fatalf("no JSB routine %s", name)

	return corevms.JSBRegisters{}
}
