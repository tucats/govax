package cpu

import (
	"errors"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vm"
)

// The octaword instructions are two-byte opcodes: the escape byte 0xFD,
// then 0x7C (CLRO), 0x7D (MOVO), 0x7E (MOVAO), or 0x7F (PUSHAO).
const (
	opEscapeFD = 0xFD
	opCLRO     = 0x7C
	opMOVO     = 0x7D
	opMOVAO    = 0x7E
	opPUSHAO   = 0x7F
)

// testOcta is a recognizable 16-byte value: each byte differs, and bit 127
// (the sign bit) is set.
var testOcta = Octaword{Lo: 0x0807060504030201, Hi: 0x8F0E0D0C0B0A0908}

// octaBytes returns o's 16 bytes in memory order (low-order byte first).
func octaBytes(o Octaword) []byte {
	b := make([]byte, 16)

	for i := 0; i < 8; i++ {
		b[i] = byte(o.Lo >> (8 * i))
		b[8+i] = byte(o.Hi >> (8 * i))
	}

	return b
}

// readOcta reads 16 bytes of memory at addr, failing the test on a fault.
func readOcta(t *testing.T, cpu *vax.CPU, mem *vm.Memory, addr uint32) Octaword {
	t.Helper()

	o, err := loadOctaword(cpu, mem, addr)
	if err != nil {
		t.Fatalf("loadOctaword(%#x): %v", addr, err)
	}

	return o
}

// setOctaRegs puts o in Rn..Rn+3, low-order longword in Rn.
func setOctaRegs(cpu *vax.CPU, r vax.Reg, o Octaword) {
	cpu.SetGPR(r, uint32(o.Lo))
	cpu.SetGPR(r+1, uint32(o.Lo>>32))
	cpu.SetGPR(r+2, uint32(o.Hi))
	cpu.SetGPR(r+3, uint32(o.Hi>>32))
}

// octaRegs reads Rn..Rn+3 back as an octaword.
func octaRegs(cpu *vax.CPU, r vax.Reg) Octaword {
	return Octaword{
		Lo: uint64(cpu.GPR(r)) | uint64(cpu.GPR(r+1))<<32,
		Hi: uint64(cpu.GPR(r+2)) | uint64(cpu.GPR(r+3))<<32,
	}
}

// checkMovePSL checks a move's condition codes: N and Z as given, V clear,
// and C left as the test set it (true).
func checkMovePSL(t *testing.T, cpu *vax.CPU, n, z bool) {
	t.Helper()

	psl := cpu.PSL()
	if psl.N() != n || psl.Z() != z || psl.V() || !psl.C() {
		t.Errorf("NZVC = %v%v%v%v, want N=%v Z=%v V=false C=true (unaffected)",
			psl.N(), psl.Z(), psl.V(), psl.C(), n, z)
	}
}

func TestMovoRegisterToRegister(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	setOctaRegs(cpu, vax.R0, testOcta)
	setC(cpu, true) // so the test can check the instruction leaves C alone

	// MOVO R0,R4: R0-R3 to R4-R7.
	stepInstruction(t, e, opEscapeFD, opMOVO, regMode(vax.R0), regMode(vax.R4))

	if got := octaRegs(cpu, vax.R4); got != testOcta {
		t.Errorf("R4-R7 = %#x, want %#x", got, testOcta)
	}

	checkMovePSL(t, cpu, true, false) // bit 127 set: negative
}

func TestMovoImmediateToMemory(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	setC(cpu, true) // so the test can check the instruction leaves C alone

	const dst = 0x2000

	// MOVO I^#testOcta,@#^X2000: 16 bytes of immediate data inline.
	bytes := []byte{opEscapeFD, opMOVO}
	bytes = append(bytes, immediate(octaBytes(testOcta)...)...)
	bytes = append(bytes, absoluteMode(dst)...)
	stepInstruction(t, e, bytes...)

	if got := readOcta(t, cpu, mem, dst); got != testOcta {
		t.Errorf("memory = %#x, want %#x", got, testOcta)
	}

	if got, want := cpu.GPR(vax.PC), uint32(base+len(bytes)); got != want {
		t.Errorf("PC = %#x, want %#x (past all 16 immediate bytes)", got, want)
	}
}

func TestMovoShortLiteralZeroExtends(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	setOctaRegs(cpu, vax.R4, testOcta)
	setC(cpu, true) // so the test can check the instruction leaves C alone

	// MOVO S^#5,R4: a short literal is a 6-bit integer in the specifier
	// byte itself; as an octaword it's 5 with 120 zero bits above it.
	stepInstruction(t, e, opEscapeFD, opMOVO, shortLiteral(5), regMode(vax.R4))

	if got, want := octaRegs(cpu, vax.R4), (Octaword{Lo: 5}); got != want {
		t.Errorf("R4-R7 = %#x, want %#x", got, want)
	}

	checkMovePSL(t, cpu, false, false)
}

func TestMovoAutoincrementAndDecrement(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	const src, dst = 0x2000, 0x3000

	putBytes(t, cpu, mem, src, octaBytes(testOcta)...)
	cpu.SetGPR(vax.R1, src)
	cpu.SetGPR(vax.R2, dst+16)

	// MOVO (R1)+,-(R2): R1 advances by 16 after the read, and R2 drops by
	// 16 before the write.
	stepInstruction(t, e, opEscapeFD, opMOVO, 0x81, 0x72)

	if got := readOcta(t, cpu, mem, dst); got != testOcta {
		t.Errorf("memory = %#x, want %#x", got, testOcta)
	}

	if cpu.GPR(vax.R1) != src+16 || cpu.GPR(vax.R2) != dst {
		t.Errorf("R1, R2 = %#x, %#x, want %#x, %#x", cpu.GPR(vax.R1), cpu.GPR(vax.R2), src+16, dst)
	}
}

func TestMovoIndexedScalesBy16(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	const table = 0x2000

	putBytes(t, cpu, mem, table+3*16, octaBytes(testOcta)...)
	cpu.SetGPR(vax.R1, table)
	cpu.SetGPR(vax.R2, 3)

	// MOVO (R1)[R2],R4: indexed mode (0x42 is "[R2]") adds R2 times the
	// operand size, 16, to the base operand's address (0x61 is "(R1)").
	stepInstruction(t, e, opEscapeFD, opMOVO, 0x42, 0x61, regMode(vax.R4))

	if got := octaRegs(cpu, vax.R4); got != testOcta {
		t.Errorf("R4-R7 = %#x, want %#x (element 3, at table+48)", got, testOcta)
	}
}

func TestMovoZero(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)
	setOctaRegs(cpu, vax.R4, testOcta)
	setC(cpu, true) // so the test can check the instruction leaves C alone

	// MOVO R0,R4 with R0-R3 all zero.
	stepInstruction(t, e, opEscapeFD, opMOVO, regMode(vax.R0), regMode(vax.R4))

	if got := octaRegs(cpu, vax.R4); !got.IsZero() {
		t.Errorf("R4-R7 = %#x, want 0", got)
	}

	checkMovePSL(t, cpu, false, true)
}

// TestOctawordRegisterLimit checks an octaword register operand can start
// in R11 at the highest (R11-SP), and that R12 and above is a reserved
// addressing-mode fault, at decode time, whether the operand is read or
// written.
func TestOctawordRegisterLimit(t *testing.T) {
	cases := []struct {
		name  string
		bytes []byte
		fault bool
	}{
		{"source R11", []byte{opEscapeFD, opMOVO, regMode(vax.R11), regMode(vax.R0)}, false},
		{"source R12", []byte{opEscapeFD, opMOVO, regMode(vax.R12), regMode(vax.R0)}, true},
		{"destination SP", []byte{opEscapeFD, opMOVO, regMode(vax.R0), regMode(vax.SP)}, true},
		{"CLRO PC", []byte{opEscapeFD, opCLRO, regMode(vax.PC)}, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cpu, mem := fixture()
			cpu.SetGPR(vax.PC, base)
			putBytes(t, cpu, mem, base, c.bytes...)

			_, err := decodeInstructionValue(cpu, mem, instructionTable)

			var f *Fault
			faulted := errors.As(err, &f) && f.Code == ExcReservedAddr

			if faulted != c.fault {
				t.Errorf("decode error = %v, want reserved-addressing fault %v", err, c.fault)
			}
		})
	}
}

func TestClro(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	const dst = 0x2000

	putBytes(t, cpu, mem, dst, octaBytes(testOcta)...)
	setOctaRegs(cpu, vax.R4, testOcta)
	setC(cpu, true) // so the test can check the instruction leaves C alone

	// CLRO R4: all four registers.
	stepInstruction(t, e, opEscapeFD, opCLRO, regMode(vax.R4))

	if got := octaRegs(cpu, vax.R4); !got.IsZero() {
		t.Errorf("R4-R7 = %#x, want 0", got)
	}

	checkMovePSL(t, cpu, false, true)

	// CLRO @#^X2000: all 16 bytes.
	stepInstruction(t, e, append([]byte{opEscapeFD, opCLRO}, absoluteMode(dst)...)...)

	if got := readOcta(t, cpu, mem, dst); !got.IsZero() {
		t.Errorf("memory = %#x, want 0", got)
	}
}

func TestMovaoAndPushao(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	const table, stack = 0x2000, 0x4000

	cpu.SetGPR(vax.R1, table)
	cpu.SetGPR(vax.R2, 3)
	cpu.SetGPR(vax.SP, stack)

	// MOVAO (R1)[R2],R0: the address of element 3 of an octaword array.
	stepInstruction(t, e, opEscapeFD, opMOVAO, 0x42, 0x61, regMode(vax.R0))

	if got, want := cpu.GPR(vax.R0), uint32(table+3*16); got != want {
		t.Errorf("MOVAO: R0 = %#x, want %#x", got, want)
	}

	// PUSHAO (R1)+: pushes R1's address, then R1 advances by 16.
	stepInstruction(t, e, opEscapeFD, opPUSHAO, 0x81)

	if got := mustLongword(t, cpu, mem, stack-4); got != table {
		t.Errorf("PUSHAO pushed %#x, want %#x", got, table)
	}

	if cpu.GPR(vax.R1) != table+16 || cpu.GPR(vax.SP) != stack-4 {
		t.Errorf("R1, SP = %#x, %#x, want %#x, %#x", cpu.GPR(vax.R1), cpu.GPR(vax.SP), table+16, stack-4)
	}
}

// TestStoreOctawordAcrossInvalidPage writes an octaword whose second half
// falls on an invalid page. The write must fault without changing the
// first half, so the instruction can be run again once the page is made
// valid. Memory mapping is on: system virtual page 0 (0x80000000) is
// valid, page 1 isn't.
func TestStoreOctawordAcrossInvalidPage(t *testing.T) {
	cpu, mem := fixture()

	const (
		sbr      = 0x3000     // physical address of the system page table
		pagePFN  = 0x10       // virtual page 0 -> physical page 0x10 (0x2000)
		pagePhys = 0x2000     //
		dstVA    = 0x800001F8 // the last 8 bytes of page 0
	)

	var pte vm.PTE

	pte.SetValid(true)
	pte.SetProtection(vm.ProtKW)
	pte.SetPFN(pagePFN)
	putLongword(t, cpu, mem, sbr, uint32(pte)) // page 0: valid
	putLongword(t, cpu, mem, sbr+4, 0)         // page 1: invalid

	original := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	putBytes(t, cpu, mem, pagePhys+0x1F8, original...)

	cpu.SetPR(vax.SBR, sbr)
	cpu.SetPR(vax.SLR, 2)
	cpu.SetPR(vax.MAPEN, 1)

	err := storeOctaword(cpu, mem, dstVA, testOcta)

	var tf *vm.TranslationFault
	if !errors.As(err, &tf) {
		t.Fatalf("storeOctaword = %v, want a translation fault", err)
	}

	cpu.SetPR(vax.MAPEN, 0) // read the page back physically

	for i, want := range original {
		if got, _ := mem.LoadByte(cpu, pagePhys+0x1F8+uint32(i)); got != want {
			t.Fatalf("byte %d = %#x, want %#x unchanged", i, got, want)
		}
	}
}
