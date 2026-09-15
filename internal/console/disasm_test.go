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
