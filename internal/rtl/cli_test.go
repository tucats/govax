package rtl

import (
	"errors"
	"testing"
)

func putRequest(t *testing.T, env *Environment, addr uint32, request, subrequest byte, length, ptr uint32) {
	t.Helper()

	if err := env.mem.StoreByte(env.cpu, addr, request); err != nil {
		t.Fatal(err)
	}

	if err := env.mem.StoreByte(env.cpu, addr+1, subrequest); err != nil {
		t.Fatal(err)
	}

	putLongword(t, env, addr+4, length)
	putLongword(t, env, addr+8, ptr)
}

func TestServiceSysCliGetSymbol(t *testing.T) {
	env, _ := fixture()
	reqAddr, symAddr := uint32(0x1000), uint32(0x1100)
	putString(t, env, symAddr, "FOO")
	putRequest(t, env, reqAddr, 0x05, 0x13, 3, symAddr) // reqword 0x1305

	r0, err := serviceSysCli(env, []uint32{reqAddr})
	if err != nil {
		t.Fatal(err)
	}

	if r0 != cliUndefinedSymbol {
		t.Errorf("r0 = %#x, want CLI$_UNDSYM (%#x)", r0, cliUndefinedSymbol)
	}
}

func TestServiceSysCliUnknownRequestHalts(t *testing.T) {
	env, _ := fixture()
	reqAddr := uint32(0x1000)
	putRequest(t, env, reqAddr, 0x05, 0x01, 0, 0) // reqword 0x0105, listed but unimplemented

	r0, err := serviceSysCli(env, []uint32{reqAddr})
	if !errors.Is(err, ErrHalt) {
		t.Errorf("err = %v, want ErrHalt", err)
	}
	
	if r0 != ssInvArg {
		t.Errorf("r0 = %d, want ssInvArg", r0)
	}
}

func TestRegisterCLIService(t *testing.T) {
	env, _ := fixture()
	if _, ok := env.services.Lookup("SYS$CLI"); !ok {
		t.Error("SYS$CLI not registered")
	}
}
