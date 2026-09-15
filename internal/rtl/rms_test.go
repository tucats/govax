package rtl

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

func TestRMSCreateConnectPutToConsole(t *testing.T) {
	env, out := fixture()

	fabAddr, fnaAddr := uint32(0x1000), uint32(0x1100)
	putString(t, env, fnaAddr, "TTA0:")
	if err := env.mem.StoreByte(env.cpu, fabAddr+fabFAC, fabFACPut); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreLongword(env.cpu, fabAddr+fabFNA, fnaAddr); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreByte(env.cpu, fabAddr+fabFNS, 5); err != nil {
		t.Fatal(err)
	}

	if r0, err := serviceSysCreate(env, []uint32{fabAddr}); err != nil || r0 != ssNormal {
		t.Fatalf("SYS$CREATE: r0=%d err=%v", r0, err)
	}
	ifi, err := env.mem.LoadWord(env.cpu, fabAddr+fabIFI)
	if err != nil {
		t.Fatal(err)
	}
	if ifi != 1 {
		t.Errorf("IFI = %d, want 1 (TTA0: maps to console)", ifi)
	}

	rabAddr := uint32(0x1200)
	if err := env.mem.StoreLongword(env.cpu, rabAddr+rabFAB, fabAddr); err != nil {
		t.Fatal(err)
	}
	if r0, err := serviceSysConnect(env, []uint32{rabAddr}); err != nil || r0 != ssNormal {
		t.Fatalf("SYS$CONNECT: r0=%d err=%v", r0, err)
	}
	isi, err := env.mem.LoadWord(env.cpu, rabAddr+rabISI)
	if err != nil {
		t.Fatal(err)
	}
	if isi != 1 {
		t.Errorf("ISI = %d, want 1 (copied from FAB's IFI)", isi)
	}

	recBuf := uint32(0x1300)
	record := "HELLO, WORLD"
	putString(t, env, recBuf, record)
	if err := env.mem.StoreByte(env.cpu, rabAddr+rabRAC, rabRACSeq); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreWord(env.cpu, rabAddr+rabRSZ, uint16(len(record))); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreLongword(env.cpu, rabAddr+rabRBF, recBuf); err != nil {
		t.Fatal(err)
	}

	if r0, err := serviceSysPut(env, []uint32{rabAddr}); err != nil || r0 != ssNormal {
		t.Fatalf("SYS$PUT: r0=%d err=%v", r0, err)
	}

	if got := out.String(); got != record+"\n" {
		t.Errorf("console output = %q, want %q (record plus the console-IFI newline)", got, record+"\n")
	}
}

func TestRMSCreateConnectPutDebugRMSTrace(t *testing.T) {
	env, _ := fixture()

	fabAddr, fnaAddr := uint32(0x1000), uint32(0x1100)
	putString(t, env, fnaAddr, "TTA0:")
	if err := env.mem.StoreByte(env.cpu, fabAddr+fabFAC, fabFACPut); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreLongword(env.cpu, fabAddr+fabFNA, fnaAddr); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreByte(env.cpu, fabAddr+fabFNS, 5); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	env.cpu.SetDebugWriter(&buf)
	env.cpu.SetDebug(vax.DebugRMS)

	if r0, err := serviceSysCreate(env, []uint32{fabAddr}); err != nil || r0 != ssNormal {
		t.Fatalf("SYS$CREATE: r0=%d err=%v", r0, err)
	}
	if !strings.Contains(buf.String(), "In SYS$CREATE function") {
		t.Errorf("output = %q, want a SYS$CREATE trace", buf.String())
	}

	rabAddr := uint32(0x1200)
	if err := env.mem.StoreLongword(env.cpu, rabAddr+rabFAB, fabAddr); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if r0, err := serviceSysConnect(env, []uint32{rabAddr}); err != nil || r0 != ssNormal {
		t.Fatalf("SYS$CONNECT: r0=%d err=%v", r0, err)
	}
	if !strings.Contains(buf.String(), "In SYS$CONNECT function") {
		t.Errorf("output = %q, want a SYS$CONNECT trace", buf.String())
	}

	recBuf := uint32(0x1300)
	putString(t, env, recBuf, "HI")
	if err := env.mem.StoreByte(env.cpu, rabAddr+rabRAC, rabRACSeq); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreWord(env.cpu, rabAddr+rabRSZ, 2); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreLongword(env.cpu, rabAddr+rabRBF, recBuf); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if r0, err := serviceSysPut(env, []uint32{rabAddr}); err != nil || r0 != ssNormal {
		t.Fatalf("SYS$PUT: r0=%d err=%v", r0, err)
	}
	if !strings.Contains(buf.String(), "In SYS$PUT function") {
		t.Errorf("output = %q, want a SYS$PUT trace", buf.String())
	}
}

func TestRMSCreateRealFile(t *testing.T) {
	env, _ := fixture()
	dir := t.TempDir()
	fn := dir + "/output.txt"

	fabAddr, fnaAddr := uint32(0x1000), uint32(0x1100)
	putString(t, env, fnaAddr, fn)
	if err := env.mem.StoreByte(env.cpu, fabAddr+fabFAC, fabFACPut); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreLongword(env.cpu, fabAddr+fabFNA, fnaAddr); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreByte(env.cpu, fabAddr+fabFNS, byte(len(fn))); err != nil {
		t.Fatal(err)
	}

	if r0, err := serviceSysCreate(env, []uint32{fabAddr}); err != nil || r0 != ssNormal {
		t.Fatalf("SYS$CREATE: r0=%d err=%v", r0, err)
	}
	ifi, err := env.mem.LoadWord(env.cpu, fabAddr+fabIFI)
	if err != nil {
		t.Fatal(err)
	}
	if ifi != 4 {
		t.Errorf("IFI = %d, want 4 (the first dynamically allocated slot)", ifi)
	}

	rabAddr, recBuf := uint32(0x1200), uint32(0x1300)
	record := "a real file record"
	putString(t, env, recBuf, record)
	if err := env.mem.StoreLongword(env.cpu, rabAddr+rabFAB, fabAddr); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreByte(env.cpu, rabAddr+rabRAC, rabRACSeq); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreWord(env.cpu, rabAddr+rabRSZ, uint16(len(record))); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreLongword(env.cpu, rabAddr+rabRBF, recBuf); err != nil {
		t.Fatal(err)
	}

	if r0, err := serviceSysPut(env, []uint32{rabAddr}); err != nil || r0 != ssNormal {
		t.Fatalf("SYS$PUT: r0=%d err=%v", r0, err)
	}
	if closer, ok := env.ifiFiles[ifi].(*os.File); ok {
		closer.Close()
	}

	data, err := os.ReadFile(fn)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != record {
		t.Errorf("file contents = %q, want %q (no trailing newline: not the console IFI)", data, record)
	}
}

func TestRMSCreateLogicalNameResolution(t *testing.T) {
	env, out := fixture()
	env.Logicals.Set("LNM$FILE_DEV", "MYOUT", "TTA0:", 0)

	fabAddr, fnaAddr := uint32(0x1000), uint32(0x1100)
	putString(t, env, fnaAddr, "MYOUT")
	if err := env.mem.StoreByte(env.cpu, fabAddr+fabFAC, fabFACPut); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreLongword(env.cpu, fabAddr+fabFNA, fnaAddr); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreByte(env.cpu, fabAddr+fabFNS, 5); err != nil {
		t.Fatal(err)
	}

	if r0, err := serviceSysCreate(env, []uint32{fabAddr}); err != nil || r0 != ssNormal {
		t.Fatalf("SYS$CREATE: r0=%d err=%v", r0, err)
	}
	ifi, err := env.mem.LoadWord(env.cpu, fabAddr+fabIFI)
	if err != nil {
		t.Fatal(err)
	}
	if ifi != 1 {
		t.Errorf("IFI = %d, want 1 (MYOUT resolves to TTA0: via the logical name)", ifi)
	}
	_ = out
}

func TestRMSCreateUnsupportedFAC(t *testing.T) {
	env, _ := fixture()
	fabAddr := uint32(0x1000)
	if err := env.mem.StoreByte(env.cpu, fabAddr+fabFAC, 2 /* FAB_M_GET */); err != nil {
		t.Fatal(err)
	}
	r0, err := serviceSysCreate(env, []uint32{fabAddr})
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssNoSuchFac {
		t.Errorf("r0 = %d, want ssNoSuchFac", r0)
	}
}

func TestRMSPutInvalidRAC(t *testing.T) {
	env, _ := fixture()
	fabAddr, rabAddr := uint32(0x1000), uint32(0x1200)
	if err := env.mem.StoreWord(env.cpu, fabAddr+fabIFI, 1); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreLongword(env.cpu, rabAddr+rabFAB, fabAddr); err != nil {
		t.Fatal(err)
	}
	if err := env.mem.StoreByte(env.cpu, rabAddr+rabRAC, 99); err != nil {
		t.Fatal(err)
	}

	r0, err := serviceSysPut(env, []uint32{rabAddr})
	if err != nil {
		t.Fatal(err)
	}
	if r0 != ssInvArg {
		t.Errorf("r0 = %d, want ssInvArg (fixed misplaced-brace bug: always reported, not gated on a debug flag)", r0)
	}
}

func TestRegisterRMSServices(t *testing.T) {
	env, _ := fixture()
	for _, name := range []string{"SYS$CREATE", "SYS$CONNECT", "SYS$PUT"} {
		if _, ok := env.services.Lookup(name); !ok {
			t.Errorf("%s not registered", name)
		}
	}
}
