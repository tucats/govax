package console

import "testing"

func TestDisassemble_singleInstruction(t *testing.T) {
	c, buf := newTestConsole(t)

	// CLRL R3: 0xD4 0x53.
	if err := c.Deposit("", 0x1000, SizeByte, 0xD4); err != nil {
		t.Fatalf("Deposit: %v", err)
	}
	
	if err := c.Deposit("", 0x1001, SizeByte, 0x53); err != nil {
		t.Fatalf("Deposit: %v", err)
	}

	if err := c.Disassemble(0x1000, 0x1000); err != nil {
		t.Fatalf("Disassemble: %v", err)
	}

	got := buf.String()
	want := "00001000: CLRL R3\n"

	if got != want {
		t.Errorf("Disassemble output = %q, want %q", got, want)
	}
}

func TestDispatch_disassemble(t *testing.T) {
	d, _ := newTestDispatcher(t)

	if err := d.Console.Deposit("", 0x2000, SizeByte, 0xD4); err != nil {
		t.Fatalf("Deposit: %v", err)
	}

	if err := d.Console.Deposit("", 0x2001, SizeByte, 0x53); err != nil {
		t.Fatalf("Deposit: %v", err)
	}

	if err := d.Dispatch("DISASSEMBLE 2000"); err != nil {
		t.Fatalf("Dispatch(DISASSEMBLE): %v", err)
	}

	if err := d.Dispatch("DIS 2000"); err != nil {
		t.Fatalf("Dispatch(DIS): %v", err)
	}
}

// TestDisassemble_entryMask checks that a word at a known .ENTRY address is
// disassembled as a register-save mask, not misdecoded as an instruction --
// the gap reported against testdata/asm/hello.asm's ".entry main, ^m<>":
// Assemble merges the SymEntry flag into Console.Symbols (asm.go), and
// Disassemble/decodeInstruction (disasm.go) consult it by PC, matching the
// C reference's decode_opcode.c SYM_ENTRY scan.
func TestDisassemble_entryMask(t *testing.T) {
	c, buf := newTestConsole(t)

	c.Symbols.SetEntry("MAIN", 0x1000, SymbolUser)

	// Mask word: bit 2 (R2) and bit 14 (DV) set, little-endian.
	if err := c.Deposit("", 0x1000, SizeByte, 0x04); err != nil {
		t.Fatalf("Deposit: %v", err)
	}

	if err := c.Deposit("", 0x1001, SizeByte, 0x40); err != nil {
		t.Fatalf("Deposit: %v", err)
	}

	// CLRL R3 (2 bytes) immediately after the mask word.
	if err := c.Deposit("", 0x1002, SizeByte, 0xD4); err != nil {
		t.Fatalf("Deposit: %v", err)
	}

	if err := c.Deposit("", 0x1003, SizeByte, 0x53); err != nil {
		t.Fatalf("Deposit: %v", err)
	}

	if err := c.Disassemble(0x1000, 0x1003); err != nil {
		t.Fatalf("Disassemble: %v", err)
	}

	got := buf.String()

	want := "00001000: .ENTRY MAIN,^M<R2,DV>\n00001002: CLRL R3\n"
	if got != want {
		t.Errorf("Disassemble output = %q, want %q", got, want)
	}
}

func TestDisassemble_range(t *testing.T) {
	c, buf := newTestConsole(t)

	// CLRL R3 (2 bytes) followed by RET (1 byte).
	for i, b := range []byte{0xD4, 0x53, 0x04} {
		if err := c.Deposit("", 0x1000+uint32(i), SizeByte, uint32(b)); err != nil {
			t.Fatalf("Deposit: %v", err)
		}
	}

	if err := c.Disassemble(0x1000, 0x1002); err != nil {
		t.Fatalf("Disassemble: %v", err)
	}

	got := buf.String()
	want := "00001000: CLRL R3\n00001002: RET\n"

	if got != want {
		t.Errorf("Disassemble output = %q, want %q", got, want)
	}
}
