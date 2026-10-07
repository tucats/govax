package librtl

import (
	"bytes"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	iodev "github.com/tucats/govax/internal/io"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vax"
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

	env, err := corevms.NewEnvironment(corevms.NewSystem(vax.New(), vm.NewMemory(1<<20), iodev.NewDeviceTable(),
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

// call runs the routine registered as name through the shim table, as a
// program's call would reach it.
func call(t *testing.T, env *corevms.Environment, name string, argv ...uint32) uint32 {
	t.Helper()

	for _, r := range Routines {
		if r.Name != name {
			continue
		}

		fn, ok := env.Shims().Lookup(r.Code)
		if !ok {
			t.Fatalf("%s (code %d) isn't registered", name, r.Code)
		}

		r0, err := fn(env, argv)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		return r0
	}

	t.Fatalf("no routine %s", name)

	return 0
}

// TestRoutinesMatchLIBRTL: every routine's offset is its entry in VMS
// 7.3's LIBRTL.EXE transfer vector, as LINK knows it, and codes and names
// are distinct.
func TestRoutinesMatchLIBRTL(t *testing.T) {
	codes := map[uint32]string{}
	names := map[string]bool{}

	for _, r := range Routines {
		sym, ok := vmsdef.ImageSymbols[r.Name]

		switch {
		case !ok:
			t.Errorf("%s: not in vmsdef.ImageSymbols", r.Name)
		case sym.Image != Library || sym.Value != r.Offset:
			t.Errorf("%s: offset %s+%#x, but LINK has %s+%#x", r.Name, Library, r.Offset, sym.Image, sym.Value)
		}

		if other, dup := codes[r.Code]; dup {
			t.Errorf("%s and %s share code %d", r.Name, other, r.Code)
		}

		if names[r.Name] {
			t.Errorf("%s is listed twice", r.Name)
		}

		codes[r.Code], names[r.Name] = r.Name, true
	}
}

func TestLibAdawi(t *testing.T) {
	for _, c := range []struct {
		sum, base, wantBase, wantSign uint32
	}{
		{5, 10, 15, 1},
		{0, 0, 0, 0},
		{0xFFFFFFFF, 0, 0xFFFFFFFF, 0xFFFFFFFF},
	} {
		env := fixture(t)
		putLongword(t, env, 0x1000, c.sum)
		putLongword(t, env, 0x1004, c.base)

		if r0 := call(t, env, "LIB$ADAWI", 0x1000, 0x1004, 0x1008); r0 != 1 {
			t.Errorf("r0 = %d, want 1", r0)
		}

		if got := longword(t, env, 0x1008); got != c.wantSign {
			t.Errorf("%d+%d: sign = %#x, want %#x", c.sum, c.base, got, c.wantSign)
		}
	}
}

func TestStrUpcase(t *testing.T) {
	env := fixture(t)
	putDescriptor(t, env, 0x1000, 0x1100, "Hello, World!")

	if r0 := call(t, env, "STR$UPCASE", 0x1000); r0 != 1 {
		t.Errorf("r0 = %d, want 1", r0)
	}

	got, ok, err := env.StringDescriptor(0x1000, 64)
	if err != nil || !ok || got != "HELLO, WORLD!" {
		t.Errorf("upcased = %q, %v, %v; want HELLO, WORLD!", got, ok, err)
	}
}

func TestLibGetFreeVM(t *testing.T) {
	env := fixture(t)
	env.RegionSize[0] = 0x4000

	const sizeAddr, retAddr = 0x1000, 0x1004

	putLongword(t, env, sizeAddr, 100)

	if r0 := call(t, env, "LIB$GET_VM", sizeAddr, retAddr); r0 != ssNormal {
		t.Fatalf("LIB$GET_VM = %#x, want SS$_NORMAL", r0)
	}

	if addr := longword(t, env, retAddr); addr != 0x4000 {
		t.Errorf("allocated at %#x, want 0x4000", addr)
	}

	if r0 := call(t, env, "LIB$FREE_VM", sizeAddr, retAddr); r0 != ssNormal {
		t.Errorf("LIB$FREE_VM = %#x, want SS$_NORMAL", r0)
	}

	if r0 := call(t, env, "LIB$FREE_VM", sizeAddr, retAddr); r0 != statusFreeVMBadBlock {
		t.Errorf("LIB$FREE_VM again = %d, want %d", r0, statusFreeVMBadBlock)
	}
}

func TestLibDeleteVMZone(t *testing.T) {
	env := fixture(t)
	env.RegionSize[0] = 0x4000

	blocks := make([]uint32, 0, 3)

	for _, zone := range []uint32{5, 5, 9} {
		addr, err := env.AllocateVM(16, zone)
		if err != nil || addr == 0 {
			t.Fatalf("AllocateVM: %#x, %v", addr, err)
		}

		blocks = append(blocks, addr)
	}

	putLongword(t, env, 0x1000, 5)

	if r0 := call(t, env, "LIB$DELETE_VM_ZONE", 0x1000); r0 != ssNormal {
		t.Fatalf("LIB$DELETE_VM_ZONE = %#x, want SS$_NORMAL", r0)
	}

	if env.FreeVM(blocks[0]) || env.FreeVM(blocks[1]) {
		t.Error("a zone-5 block survived")
	}

	if !env.FreeVM(blocks[2]) {
		t.Error("the zone-9 block was freed")
	}
}

// TestLibEstablishRevert: LIB$ESTABLISH and LIB$REVERT change the handler
// of the caller's frame, whose FP the running shim's frame saved at 12.
func TestLibEstablishRevert(t *testing.T) {
	env := fixture(t)

	const shimFP, callerFP = 0x2000, 0x3000

	env.CPU().SetGPR(vax.FP, shimFP)
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
