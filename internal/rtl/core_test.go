package rtl

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestServiceSysSetastAndDclexh(t *testing.T) {
	env, _ := fixture()

	if _, err := serviceSysSetast(env, []uint32{1}); err != nil {
		t.Fatal(err)
	}
	if !env.astEnabled {
		t.Error("astEnabled = false, want true")
	}

	if _, err := serviceSysDclexh(env, []uint32{0x1234}); err != nil {
		t.Fatal(err)
	}
	if env.exitHandler != 0x1234 {
		t.Errorf("exitHandler = %#x, want 0x1234", env.exitHandler)
	}
}

func TestServiceSysClrefSetefReadef(t *testing.T) {
	env, _ := fixture()

	if r0, err := serviceSysSetef(env, []uint32{3}); err != nil || r0 != ssNormal {
		t.Fatalf("SETEF: r0=%d err=%v", r0, err)
	}
	if r0, err := serviceSysReadef(env, []uint32{3}); err != nil || r0 != ssWasSet {
		t.Fatalf("READEF after SETEF: r0=%d err=%v, want ssWasSet", r0, err)
	}

	if r0, err := serviceSysClref(env, []uint32{3}); err != nil || r0 != ssNormal {
		t.Fatalf("CLREF: r0=%d err=%v", r0, err)
	}
	if r0, err := serviceSysReadef(env, []uint32{3}); err != nil || r0 != ssWasClr {
		t.Fatalf("READEF after CLREF: r0=%d err=%v, want ssWasClr", r0, err)
	}
}

func TestServiceSysReadefReturnsWholeWord(t *testing.T) {
	env, _ := fixture()
	if _, err := serviceSysSetef(env, []uint32{5}); err != nil {
		t.Fatal(err)
	}

	addr := uint32(0x1000)
	if _, err := serviceSysReadef(env, []uint32{5, addr}); err != nil {
		t.Fatal(err)
	}
	word, err := env.mem.LoadLongword(env.cpu, addr)
	if err != nil {
		t.Fatal(err)
	}
	if word&(1<<5) == 0 {
		t.Errorf("returned event-flag word = %#x, want bit 5 set", word)
	}
}

func TestServiceSysExpreg(t *testing.T) {
	env, _ := fixture()
	env.cpu.SetPSL(env.cpu.PSL()) // ensure Kernel mode (zero value)

	retAddr := uint32(0x1000)
	r0, err := serviceSysExpreg(env, []uint32{2, retAddr, uint32(vax.Kernel), 0})
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssNormal {
		t.Fatalf("r0 = %d, want ssNormal", r0)
	}

	start, err := env.mem.LoadLongword(env.cpu, retAddr)
	if err != nil {
		t.Fatal(err)
	}
	end, err := env.mem.LoadLongword(env.cpu, retAddr+4)
	if err != nil {
		t.Fatal(err)
	}
	if start != 0 || end != 1023 { // 2 pages * 512 bytes - 1
		t.Errorf("region = [%d,%d], want [0,1023]", start, end)
	}
	if got := env.cpu.GPR(vax.R1); got != start {
		t.Errorf("R1 = %#x, want %#x (start, the documented side effect)", got, start)
	}
	if env.RegionSize[0] != 1024 {
		t.Errorf("RegionSize[0] = %d, want 1024", env.RegionSize[0])
	}

	// A second call should start where the first left off.
	r0, err = serviceSysExpreg(env, []uint32{1, 0, uint32(vax.Kernel), 0})
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssNormal {
		t.Fatalf("r0 = %d, want ssNormal", r0)
	}
	if got := env.cpu.GPR(vax.R1); got != 1024 {
		t.Errorf("R1 = %d, want 1024 (continuing from the first call's end)", got)
	}
}

func TestServiceSysExpregInvalidRegion(t *testing.T) {
	env, _ := fixture()
	r0, err := serviceSysExpreg(env, []uint32{1, 0, 0, 3})
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssInvArg {
		t.Errorf("r0 = %d, want ssInvArg", r0)
	}
}

func TestServiceSysGetjpiw(t *testing.T) {
	env, _ := fixture()
	itemList := uint32(0x2000)
	accountBuf := uint32(0x3000)
	clinameBuf := uint32(0x3100)

	// Two item-list entries (ACCOUNT, CLINAME) then a zero terminator.
	putWord(t, env, itemList, 8)
	putWord(t, env, itemList+2, jpiAccount)
	putLongword(t, env, itemList+4, accountBuf)
	putLongword(t, env, itemList+8, 0)

	putWord(t, env, itemList+12, 4)
	putWord(t, env, itemList+14, jpiCliName)
	putLongword(t, env, itemList+16, clinameBuf)
	putLongword(t, env, itemList+20, 0)

	putLongword(t, env, itemList+24, 0) // terminator (bufflen=0,itemcode=0)

	argv := make([]uint32, 7)
	argv[3] = itemList
	r0, err := serviceSysGetjpiw(env, argv)
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssNormal {
		t.Fatalf("r0 = %d, want ssNormal", r0)
	}

	account, err := loadString(env, accountBuf, 8)
	if err != nil {
		t.Fatal(err)
	}
	if account != "USER    " {
		t.Errorf("account = %q, want \"USER    \" (8 bytes, space-padded, no NUL terminator)", account)
	}

	cliname, err := loadString(env, clinameBuf, 4)
	if err != nil {
		t.Fatal(err)
	}
	if cliname != "DCL" {
		t.Errorf("cliname = %q, want \"DCL\"", cliname)
	}
}

func TestServiceSysGetjpiwDebugProcessTrace(t *testing.T) {
	env, _ := fixture()
	nameAddr, nameStr := uint32(0x1000), uint32(0x1100)
	putDescriptor(t, env, nameAddr, nameStr, "MYPROC")

	var buf bytes.Buffer
	env.cpu.SetDebugWriter(&buf)
	env.cpu.SetDebug(vax.DebugProcess)

	argv := make([]uint32, 7)
	argv[0] = 5 // EFN
	argv[2] = nameAddr

	if _, err := serviceSysGetjpiw(env, argv); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), `DEBUG: SYS$GETJPIW EFN=5 PRCNAM="MYPROC"`) {
		t.Errorf("output = %q, want a SYS$GETJPIW trace naming EFN and PRCNAM", buf.String())
	}
}

func TestServiceSysGetjpiwWrongArgCount(t *testing.T) {
	env, _ := fixture()
	r0, err := serviceSysGetjpiw(env, []uint32{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssInsfArg {
		t.Errorf("r0 = %d, want ssInsfArg", r0)
	}
}

func TestServiceSysGetjpiwUnknownItemCode(t *testing.T) {
	env, _ := fixture()
	itemList := uint32(0x2000)
	putWord(t, env, itemList, 4)
	putWord(t, env, itemList+2, 9999)
	putLongword(t, env, itemList+4, 0)
	putLongword(t, env, itemList+8, 0)

	argv := make([]uint32, 7)
	argv[3] = itemList
	r0, err := serviceSysGetjpiw(env, argv)
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssBadParam {
		t.Errorf("r0 = %d, want ssBadParam", r0)
	}
}

func putWord(t *testing.T, env *Environment, addr uint32, v uint16) {
	t.Helper()
	if err := env.mem.StoreWord(env.cpu, addr, v); err != nil {
		t.Fatalf("StoreWord(%#x): %v", addr, err)
	}
}
