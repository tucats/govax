package cpu

import (
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// crc16ArcTable is the 16-longword CRC table LIB$CRC_TABLE would generate
// for polynomial 0xA001 (vax_instr_set.pdf's Note 5 "CRC-16" example, poly
// 120001 octal == 0xA001), the well-known CRC-16/ARC polynomial.
var crc16ArcTable = [16]uint32{
	0x0000, 0xCC01, 0xD801, 0x1400, 0xF001, 0x3C00, 0x2800, 0xE401,
	0xA001, 0x6C00, 0x7800, 0xB401, 0x5000, 0x9C01, 0x8801, 0x4400,
}

func TestEmulCrc(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	tblAddr := uint32(0x4000)
	for i, v := range crc16ArcTable {
		if err := mem.StoreLongword(cpu, tblAddr+uint32(i*4), v); err != nil {
			t.Fatalf("StoreLongword: %v", err)
		}
	}
	streamAddr := uint32(0x2000)
	putBytes(t, cpu, mem, streamAddr, []byte("123456789")...)

	bytes := []byte{0x0B}
	bytes = append(bytes, absoluteMode(tblAddr)...)
	bytes = append(bytes, 0x8F, 0, 0, 0, 0) // inicrc = 0 (immediate longword)
	bytes = append(bytes, 9)                // strlen
	bytes = append(bytes, absoluteMode(streamAddr)...)
	stepInstruction(t, e, bytes...)

	// Known-good CRC-16/ARC test vector: CRC("123456789") == 0xBB3D.
	if got := cpu.GPR(vax.R0) & 0xFFFF; got != 0xBB3D {
		t.Errorf("R0 (low 16 bits) = %#x, want 0xBB3D", got)
	}

	if cpu.GPR(vax.R1) != 0 || cpu.GPR(vax.R2) != 0 {
		t.Errorf("R1/R2 = %#x/%#x, want both 0", cpu.GPR(vax.R1), cpu.GPR(vax.R2))
	}

	if want := streamAddr + 9; cpu.GPR(vax.R3) != want {
		t.Errorf("R3 = %#x, want %#x (one past the stream)", cpu.GPR(vax.R3), want)
	}

	if cpu.PSL().Z() {
		t.Error("Z = true, want false (nonzero result)")
	}
}

func TestEmulCrcZeroLengthReturnsInitialCrc(t *testing.T) {
	cpu, mem := fixture()
	e := NewEngine(cpu, mem)

	tblAddr := uint32(0x4000)
	for i, v := range crc16ArcTable {
		if err := mem.StoreLongword(cpu, tblAddr+uint32(i*4), v); err != nil {
			t.Fatalf("StoreLongword: %v", err)
		}
	}

	bytes := []byte{0x0B}
	bytes = append(bytes, absoluteMode(tblAddr)...)
	bytes = append(bytes, 0x8F, 0x34, 0x12, 0, 0) // inicrc = 0x1234
	bytes = append(bytes, 0)                      // strlen = 0
	bytes = append(bytes, absoluteMode(0x2000)...)
	stepInstruction(t, e, bytes...)

	if cpu.GPR(vax.R0) != 0x1234 {
		t.Errorf("R0 = %#x, want 0x1234 (initial CRC unchanged)", cpu.GPR(vax.R0))
	}
	
	if cpu.GPR(vax.R3) != 0x2000 {
		t.Errorf("R3 = %#x, want 0x2000 (stream address unchanged)", cpu.GPR(vax.R3))
	}
}
