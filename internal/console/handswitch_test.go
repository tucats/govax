package console_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/respath"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 43's closing test (docs/PHASE-43.md, subtask 12): two processes,
// switched by hand.
//
// The test does by hand what Phase 44's scheduler will do: boots as
// cmd/govax does (vax.init: VMINIT and the microkernel), runs a little
// user-mode code in process 1, builds a second process (an address space,
// stacks, and a hardware PCB starting in its P0), and switches the CPU to
// it and back with Engine.SaveContext and LoadContext, which have the
// effect of SVPCTX and LDPCTX. Both processes run code at the same P0
// address, 0x400, and store to the same P0 address, 0x600, and each sees
// its own, as each sees its own value pushed on its user stack at the same
// P1 address; both read one S0 longword and the P1 vector, and see the
// same.

const (
	codeAddr = 0x400 // where each process's program is, in its own P0
	dataAddr = 0x600 // the P0 longword each program stores to

	oneValue    = 0x11111111 // what process 1 stores
	twoValue    = 0x22222222 // what process 2 stores
	sharedValue = 0xCAFEBABE // the S0 longword both can read
)

// movlImm is MOVL #v, Rn: the opcode, an immediate (PC autoincrement)
// longword operand, and register mode.
func movlImm(v uint32, reg byte) []byte {
	return []byte{0xD0, 0x8F, byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24), 0x50 | reg}
}

// movlToAbs is MOVL Rn, @#addr: register mode, then absolute mode.
func movlToAbs(reg byte, addr uint32) []byte {
	return []byte{0xD0, 0x50 | reg, 0x9F, byte(addr), byte(addr >> 8), byte(addr >> 16), byte(addr >> 24)}
}

// fromAbs is opcode @#addr, Rn (MOVL or MOVZWL).
func fromAbs(op byte, addr uint32, reg byte) []byte {
	return []byte{op, 0x9F, byte(addr), byte(addr >> 8), byte(addr >> 16), byte(addr >> 24), 0x50 | reg}
}

// pushR0 is PUSHL R0.
var pushR0 = []byte{0xDD, 0x50}

// brbSelf is BRB to itself: the program's end, where it spins.
var brbSelf = []byte{0x11, 0xFE}

// program joins instruction byte strings, returning the code and the
// address of its last instruction (the spin).
func program(parts ...[]byte) ([]byte, uint32) {
	code := bytes.Join(parts, nil)

	return code, codeAddr + uint32(len(code)-len(parts[len(parts)-1]))
}

// step runs n instructions, failing the test on an error.
func step(t testing.TB, c *console.Console, n int) {
	t.Helper()

	for range n {
		if err := c.Engine.Step(); err != nil {
			t.Fatalf("Step at PC %08X: %v", c.CPU.GPR(vax.PC), err)
		}
	}
}

func TestHandSwitchTwoProcesses(t *testing.T) {
	var out bytes.Buffer

	c := console.New(&out)
	c.Paths = respath.New(nil, bootdata.FS)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	consoletest.InstallDebugger(t, c, d)

	if err := c.RunHostProcedure("vax.init", d.Dispatch); err != nil {
		t.Fatalf("vax.init: %v\n%s", err, out.String())
	}

	one := c.RTL
	sys := one.System
	mem, vcpu := c.Mem, c.CPU
	oneSpace := one.Space.AddressSpace

	// A longword in S0 every process maps, readable from user mode
	// (S0's default protection, URKW).
	shared, err := sys.AllocateS0(1, one.Process.PID, "hand-switch test")
	if err != nil {
		t.Fatal(err)
	}

	if err := mem.StoreLongwordIn(vcpu, oneSpace, shared, sharedValue); err != nil {
		t.Fatal(err)
	}

	// The P1 vector's SYS$EXIT entry: its entry mask, then XFC (0xFC)
	// with selector 0x7A, the word 0x7AFC.
	var vector uint32

	for _, e := range vmsdef.P1VectorTable {
		if e.Name == "SYS$EXIT" {
			vector = e.Addr
		}
	}

	// Process 1: store oneValue at dataAddr, push it, and spin.
	oneCode, oneSpin := program(movlImm(oneValue, 0), movlToAbs(0, dataAddr), pushR0, brbSelf)

	if err := mem.StoreIn(vcpu, oneSpace, codeAddr, oneCode); err != nil {
		t.Fatal(err)
	}

	// Run it in user mode, on its user stack, at IPL 0.
	c.Engine.SetModeStack(vax.User, false)

	psl := vcpu.PSL()
	psl.SetPrvMod(vax.User)
	psl.SetIPL(0)
	vcpu.SetPSL(psl)
	vcpu.SetPR(vax.IPL, 0)
	vcpu.SetGPR(vax.PC, codeAddr)

	step(t, c, 4)

	if vcpu.GPR(vax.PC) != oneSpin || vcpu.GPR(vax.R0) != oneValue {
		t.Fatalf("process 1 at PC %08X with R0 %08X, want %08X and %08X", vcpu.GPR(vax.PC), vcpu.GPR(vax.R0), oneSpin, uint32(oneValue))
	}

	oneUSP := vcpu.GPR(vax.SP)

	// Process 1's PCB. SVPCTX (and SaveContext) never write the PCB's
	// memory-management longwords -- software keeps them there -- so
	// they're written first, as VMS writes them when it builds a process.
	pcb1, err := sys.EnsurePCB(one)
	if err != nil {
		t.Fatal(err)
	}

	first := cpu.PCB{P0BR: oneSpace.P0BR, P0LR: oneSpace.P0LR, P1BR: oneSpace.P1BR, P1LR: oneSpace.P1LR, ASTLVL: 4}
	if err := cpu.WritePCB(mem, pcb1, &first); err != nil {
		t.Fatal(err)
	}

	// Process 2: an address space, stacks, a PCB, and a program at the
	// same P0 address as process 1's, written through its address space.
	two, err := corevms.NewEnvironment(sys, c.Logicals, nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	pid := two.Process.PID

	if two.Space, err = sys.BuildAddressSpace(pid, 64, corevms.MinP1Pages); err != nil {
		t.Fatal(err)
	}

	if two.Stacks, err = sys.BuildStacks(pid, 4, 2, 2); err != nil {
		t.Fatal(err)
	}

	twoCode, twoSpin := program(
		movlImm(twoValue, 0), movlToAbs(0, dataAddr), pushR0,
		fromAbs(0xD0, shared, 1),   // MOVL @#shared, R1
		fromAbs(0x3C, vector+2, 2), // MOVZWL @#vector+2, R2
		brbSelf)

	if err := mem.StoreIn(vcpu, two.Space.AddressSpace, codeAddr, twoCode); err != nil {
		t.Fatal(err)
	}

	var userPSL vax.PSL

	userPSL.SetCurMod(vax.User)
	userPSL.SetPrvMod(vax.User)

	start := corevms.InitialPCB(two.Space, two.Stacks, codeAddr, userPSL)
	if err := cpu.WritePCB(mem, two.Stacks.PCBB, &start); err != nil {
		t.Fatal(err)
	}

	// Switch to process 2: save process 1, point PCBB at process 2's
	// PCB, and load it.
	switchTo := func(pcbb uint32, env *corevms.Environment) {
		t.Helper()

		if err := c.Engine.SaveContext(); err != nil {
			t.Fatalf("SaveContext: %v", err)
		}

		vcpu.SetPR(vax.PCBB, pcbb)

		if err := c.Engine.LoadContext(); err != nil {
			t.Fatalf("LoadContext: %v", err)
		}

		sys.SetCurrent(env)
	}

	vcpu.SetPR(vax.PCBB, pcb1)
	switchTo(two.Stacks.PCBB, two)

	if vm.CurrentAddressSpace(vcpu) != two.Space.AddressSpace || vcpu.PSL().CurMod() != vax.User ||
		vcpu.GPR(vax.PC) != codeAddr || vcpu.GPR(vax.SP) != corevms.UserStackTop {
		t.Fatalf("after the switch: space %+v, mode %d, PC %08X, SP %08X",
			vm.CurrentAddressSpace(vcpu), vcpu.PSL().CurMod(), vcpu.GPR(vax.PC), vcpu.GPR(vax.SP))
	}

	step(t, c, 6)

	if got := [4]uint32{vcpu.GPR(vax.PC), vcpu.GPR(vax.R0), vcpu.GPR(vax.R1), vcpu.GPR(vax.R2)}; got != [4]uint32{twoSpin, twoValue, sharedValue, 0x7AFC} {
		t.Errorf("process 2's PC, R0, R1, R2 = %08X, want %08X %08X %08X %08X", got, twoSpin, uint32(twoValue), uint32(sharedValue), 0x7AFC)
	}

	// Each process's dataAddr holds its own value. Through the CPU's
	// registers, process 2's is seen now.
	if v, _ := mem.LoadLongword(vcpu, dataAddr); v != twoValue {
		t.Errorf("process 2's %08X = %08X through the registers", dataAddr, v)
	}

	if v, _ := mem.LoadLongwordIn(vcpu, oneSpace, dataAddr); v != oneValue {
		t.Errorf("process 1's %08X = %08X while process 2 runs, want it untouched", dataAddr, v)
	}

	// The same for the user stacks: one P1 address, two pages.
	top := uint32(corevms.UserStackTop - 4)

	if vcpu.GPR(vax.SP) != top || oneUSP != top {
		t.Errorf("user SPs: process 1 %08X, process 2 %08X; want %08X for both", oneUSP, vcpu.GPR(vax.SP), top)
	}

	if v, _ := mem.LoadLongwordIn(vcpu, oneSpace, top); v != oneValue {
		t.Errorf("process 1's stack top = %08X, want %08X", v, uint32(oneValue))
	}

	if v, _ := mem.LoadLongword(vcpu, top); v != twoValue {
		t.Errorf("process 2's stack top = %08X, want %08X", v, uint32(twoValue))
	}

	// Process 1's PCB holds what it was doing.
	saved, err := cpu.ReadPCB(mem, pcb1)
	if err != nil {
		t.Fatal(err)
	}

	if saved.PC != oneSpin || saved.R[0] != oneValue || saved.SP[vax.User] != oneUSP || saved.PSL.CurMod() != vax.User || saved.P0BR != oneSpace.P0BR {
		t.Errorf("process 1's PCB: %+v", saved)
	}

	// And back to process 1.
	switchTo(pcb1, one)

	if vm.CurrentAddressSpace(vcpu) != oneSpace || vcpu.GPR(vax.PC) != oneSpin || vcpu.GPR(vax.R0) != oneValue ||
		vcpu.GPR(vax.SP) != oneUSP || vcpu.PSL().CurMod() != vax.User {
		t.Fatalf("back in process 1: space %+v, PC %08X, R0 %08X, SP %08X",
			vm.CurrentAddressSpace(vcpu), vcpu.GPR(vax.PC), vcpu.GPR(vax.R0), vcpu.GPR(vax.SP))
	}

	if v, _ := mem.LoadLongword(vcpu, dataAddr); v != oneValue {
		t.Errorf("process 1's %08X = %08X back in process 1", dataAddr, v)
	}

	// Process 2's PCB holds its registers, and its address space
	// unchanged.
	saved, err = cpu.ReadPCB(mem, two.Stacks.PCBB)
	if err != nil {
		t.Fatal(err)
	}

	if saved.PC != twoSpin || saved.R[0] != twoValue || saved.R[1] != sharedValue || saved.R[2] != 0x7AFC ||
		saved.SP[vax.User] != top || saved.AddressSpace() != two.Space.AddressSpace {
		t.Errorf("process 2's PCB: %+v", saved)
	}

	// Process 1 carries on.
	step(t, c, 2)

	if vcpu.GPR(vax.PC) != oneSpin {
		t.Errorf("process 1 at %08X, want it spinning at %08X", vcpu.GPR(vax.PC), oneSpin)
	}

	// Process 2's code is in its P0 and not in process 1's: the bytes at
	// codeAddr differ.
	got := make([]byte, len(twoCode))
	if err := mem.LoadIn(vcpu, oneSpace, codeAddr, got); err != nil || bytes.Equal(got, twoCode) {
		t.Errorf("process 1's code at %08X is process 2's (%v)", codeAddr, err)
	}

	if binary.LittleEndian.Uint32(got) != binary.LittleEndian.Uint32(oneCode) {
		t.Errorf("process 1's code at %08X = % x, want % x", codeAddr, got[:4], oneCode[:4])
	}
}
