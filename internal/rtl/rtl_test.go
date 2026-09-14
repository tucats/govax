package rtl

import (
	"bytes"
	"testing"

	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// fixture returns an Environment with 1MB of memory (virtual memory
// disabled, so addresses are used directly, matching internal/vm and
// internal/cpu's own test convention) and a buffer standing in for the
// console's output stream (RMS internal file index 1).
func fixture() (*Environment, *bytes.Buffer) {
	cpu := vax.New()
	mem := vm.NewMemory(1 << 20)
	devices := iodev.NewDeviceTable()
	logicals := iodev.NewLogicalNameTable()
	logicals.InitLogicals()
	out := &bytes.Buffer{}
	env := NewEnvironment(cpu, mem, devices, logicals, out)
	return env, out
}

func putLongword(t *testing.T, env *Environment, addr, v uint32) {
	t.Helper()
	if err := env.mem.StoreLongword(env.cpu, addr, v); err != nil {
		t.Fatalf("StoreLongword(%#x): %v", addr, err)
	}
}

func putString(t *testing.T, env *Environment, addr uint32, s string) {
	t.Helper()
	for i := 0; i < len(s); i++ {
		if err := env.mem.StoreByte(env.cpu, addr+uint32(i), s[i]); err != nil {
			t.Fatalf("StoreByte(%#x): %v", addr+uint32(i), err)
		}
	}
	if err := env.mem.StoreByte(env.cpu, addr+uint32(len(s)), 0); err != nil {
		t.Fatalf("StoreByte(%#x): %v", addr+uint32(len(s)), err)
	}
}

// putDescriptor writes a VAX string descriptor (word length, word class/
// type placeholder, longword address) at addr, with the string itself
// stored at strAddr, matching every str_get/str_put caller's own layout.
func putDescriptor(t *testing.T, env *Environment, addr uint32, strAddr uint32, s string) {
	t.Helper()
	if err := env.mem.StoreWord(env.cpu, addr, uint16(len(s))); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreWord(env.cpu, addr+2, 0); err != nil {
		t.Fatal(err)
	}
	putLongword(t, env, addr+4, strAddr)
	putString(t, env, strAddr, s)
}

// putArgs writes a VAX argument list (argc, then each argv[n]) at ap and
// points the AP register at it, matching CALLS/CALLG's own layout.
func putArgs(t *testing.T, env *Environment, ap uint32, argv []uint32) {
	t.Helper()
	putLongword(t, env, ap, uint32(len(argv)))
	for n, v := range argv {
		putLongword(t, env, ap+uint32(n+1)*4, v)
	}
	env.cpu.SetGPR(vax.AP, ap)
}

func TestReadArgs(t *testing.T) {
	env, _ := fixture()
	putArgs(t, env, 0x2000, []uint32{0x11, 0x22, 0x33})

	argv, err := readArgs(env.cpu, env.mem, 0x2000)
	if err != nil {
		t.Fatalf("readArgs: %v", err)
	}
	want := []uint32{0x11, 0x22, 0x33}
	if len(argv) != len(want) {
		t.Fatalf("argv = %v, want %v", argv, want)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Errorf("argv[%d] = %#x, want %#x", i, argv[i], want[i])
		}
	}
}

func TestEnvironmentShim(t *testing.T) {
	env, _ := fixture()
	putArgs(t, env, 0x2000, []uint32{7})

	r0, handled, err := env.Shim(32) // DECC$TIME
	if err != nil {
		t.Fatalf("Shim: %v", err)
	}
	if !handled {
		t.Fatal("Shim(32) not handled, want DECC$TIME registered")
	}
	if r0 == 0 {
		t.Error("r0 = 0, want a nonzero Unix timestamp")
	}
}

func TestEnvironmentShimUnregisteredCode(t *testing.T) {
	env, _ := fixture()
	putArgs(t, env, 0x2000, nil)

	_, handled, err := env.Shim(250)
	if err != nil {
		t.Fatalf("Shim: %v", err)
	}
	if handled {
		t.Error("Shim(250) handled, want no such shim registered")
	}
}

func TestEnvironmentSystemService(t *testing.T) {
	env, _ := fixture()
	putArgs(t, env, 0x2000, []uint32{5})
	putLongword(t, env, 0x3000, 0)

	r0, handled, err := env.SystemService(0x7FFEE000) // SYS$SETEF
	if err != nil {
		t.Fatalf("SystemService: %v", err)
	}
	if !handled {
		t.Fatal("SystemService(SYS$SETEF's address) not handled")
	}
	if r0 != ssNormal {
		t.Errorf("r0 = %d, want ssNormal", r0)
	}
}

func TestEnvironmentSystemServiceUnknownAddress(t *testing.T) {
	env, _ := fixture()
	putArgs(t, env, 0x2000, nil)

	_, handled, err := env.SystemService(0xDEADBEEF)
	if err != nil {
		t.Fatalf("SystemService: %v", err)
	}
	if handled {
		t.Error("SystemService(0xdeadbeef) handled, want no such address in p1Vector")
	}
}

func TestEnvironmentSystemServiceKnownAddressUnregisteredHandler(t *testing.T) {
	env, _ := fixture()
	putArgs(t, env, 0x2000, nil)

	// SYS$OPEN is a real p1Vector entry with no registered handler yet.
	_, handled, err := env.SystemService(0x7FFEE208)
	if err != nil {
		t.Fatalf("SystemService: %v", err)
	}
	if handled {
		t.Error("SystemService(SYS$OPEN) handled, want unimplemented")
	}
}

// TestRegisteredServicesExistInP1Vector verifies every name this package
// ever calls ServiceTable.Register with is a real p1Vector entry — a
// service registered under a name lookupP1Vector can never match would be
// unreachable through SystemService, a copy-paste mistake this test would
// catch immediately.
func TestRegisteredServicesExistInP1Vector(t *testing.T) {
	known := map[string]bool{}
	for _, e := range p1Vector {
		known[e.Name] = true
	}

	env, _ := fixture()
	for name := range env.services.entries {
		if !known[name] {
			t.Errorf("registered service %q has no p1Vector entry", name)
		}
	}
}
