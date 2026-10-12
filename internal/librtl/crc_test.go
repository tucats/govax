package librtl

import "testing"

// crcTableAddr is where the CRC tests put the 16-longword table.
const crcTableAddr = 0x3000

// crc16 is CRC-16's polynomial, octal 120001 (the manual's example).
const crc16 = 0o120001

func TestLibCrcTable(t *testing.T) {
	env := fixture(t)
	putLongword(t, env, 0x1800, crc16)

	if got := call(t, env, "LIB$CRC_TABLE", 0x1800, crcTableAddr); got != 16 {
		t.Errorf("LIB$CRC_TABLE = %#x, want 16, as VMS returns", got)
	}

	// CRC-16's nibble table, entry by entry.
	want := []uint32{
		0x0000, 0xCC01, 0xD801, 0x1400, 0xF001, 0x3C00, 0x2800, 0xE401,
		0xA001, 0x6C00, 0x7800, 0xB401, 0x5000, 0x9C01, 0x8801, 0x4400,
	}

	for i, w := range want {
		if got := longword(t, env, crcTableAddr+4*uint32(i)); got != w {
			t.Errorf("entry %d = %#x, want %#x", i, got, w)
		}
	}
}

func TestLibCrc(t *testing.T) {
	for _, c := range []struct {
		stream  string
		initial uint32
		want    uint32
	}{
		// The manual's example: "DOVE" and "HOSE" have the same CRC.
		{"DOVE", 0, 0x74DB},
		{"HOSE", 0, 0x74DB},
		{"123456789", 0, 0xBB3D}, // CRC-16/ARC's check value
		{"", 0x1234, 0x1234},     // an empty stream: the initial CRC
	} {
		env := fixture(t)
		putLongword(t, env, 0x1800, crc16)
		call(t, env, "LIB$CRC_TABLE", 0x1800, crcTableAddr)
		putLongword(t, env, 0x1804, c.initial)
		putDescriptor(t, env, descA, textA, c.stream)

		if got := call(t, env, "LIB$CRC", crcTableAddr, 0x1804, descA); got != c.want {
			t.Errorf("LIB$CRC(%q) = %#x, want %#x", c.stream, got, c.want)
		}
	}
}
