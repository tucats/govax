package disasm

import "testing"

// mapSymbolizer names the addresses in a map.
type mapSymbolizer map[uint32]string

func (m mapSymbolizer) Symbolize(addr uint32) (string, bool) {
	name, ok := m[addr]

	return name, ok
}

// TestDebuggerStyle checks StyleDebugger's text for each kind of operand,
// with and without a symbolizer, against the forms the VMS debugger's
// EXAMINE/INSTRUCTION printed in the probe's sessions (testdata/dbg/vax).
// Each instruction is at 0x1000.
func TestDebuggerStyle(t *testing.T) {
	const pc = 0x1000

	names := mapSymbolizer{
		0x200:  `DBGDIS\COUNT`,
		0x1007: `DBGDIS\START\LOOP`,
	}

	cases := []struct {
		name  string
		bytes []byte
		sym   Symbolizer
		want  string
	}{
		{"short literal", []byte{0xD0, 0x0A, 0x52}, nil, "MOVL     S^#0A,R2"},
		{"floating short literal", []byte{0x50, 0x0C, 0x51}, nil, "MOVF     S^#1.500000,R1"},
		{"immediate longword", []byte{0xD0, 0x8F, 0xE8, 0x03, 0x00, 0x00, 0x53}, nil, "MOVL     I^#000003E8,R3"},
		{"immediate word", []byte{0xB0, 0x8F, 0x16, 0x9F, 0x8A}, nil, "MOVW     I^#9F16,(R10)+"},
		{"displacement", []byte{0xD0, 0xA1, 0x0A, 0x50}, nil, "MOVL     B^0A(R1),R0"},
		{"displacement off AP", []byte{0xD0, 0xAC, 0x04, 0x50}, nil, "MOVL     B^04(AP),R0"},
		{"register modes", []byte{0xD0, 0x62, 0x72}, nil, "MOVL     (R2),-(R2)"},
		{"autoincrement deferred", []byte{0xD0, 0x92, 0x82}, nil, "MOVL     @(R2)+,(R2)+"},
		// MOVL L^00000200,R0: the displacement -0xE06 ends at 0x1006.
		{"relative, no name", []byte{0xD0, 0xEF, 0xFA, 0xF1, 0xFF, 0xFF, 0x50}, nil, "MOVL     L^00000200,R0"},
		{"relative, named", []byte{0xD0, 0xEF, 0xFA, 0xF1, 0xFF, 0xFF, 0x50}, names, `MOVL     L^DBGDIS\COUNT,R0`},
		// A word displacement still shows an eight-digit address.
		{"word relative, no name", []byte{0xD0, 0xCF, 0xFC, 0xF1, 0x50}, nil, "MOVL     W^00000200,R0"},
		{"absolute, named", []byte{0xD0, 0x9F, 0x00, 0x02, 0x00, 0x00, 0x50}, names, `MOVL     @#DBGDIS\COUNT,R0`},
		{"absolute, no name", []byte{0xD0, 0x9F, 0x00, 0x00, 0x00, 0x00, 0x50}, names, "MOVL     @#00000000,R0"},
		{"deferred relative", []byte{0xD0, 0xDF, 0xFC, 0xF1, 0x50}, names, `MOVL     @W^DBGDIS\COUNT,R0`},
		// The name is the base's; the index follows it.
		{"indexed", []byte{0xD0, 0x44, 0xEF, 0xF9, 0xF1, 0xFF, 0xFF, 0x50}, names, `MOVL     L^DBGDIS\COUNT[R4],R0`},
		// BEQL to 0x1007: a branch has no prefix.
		{"branch, named", []byte{0x13, 0x05}, names, `BEQL     DBGDIS\START\LOOP`},
		{"branch, no name", []byte{0x13, 0x05}, nil, "BEQL     00001007"},
		{"BGEQU, not BCC", []byte{0x1E, 0x05}, nil, "BGEQU    00001007"},
		{"no operands", []byte{0x04}, nil, "RET     "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := make(SliceReader, pc+len(tc.bytes))
			copy(r[pc:], tc.bytes)

			dec, err := Disassemble(r, pc)
			if err != nil {
				t.Fatal(err)
			}

			if got := dec.Format(Options{Style: StyleDebugger, Symbolizer: tc.sym}); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestDebuggerMask checks an entry mask's debugger text: IV before DV.
func TestDebuggerMask(t *testing.T) {
	r := SliceReader{0xFC, 0xCF}
	dec := EntryMask(r, 0, "SUB2")

	want := "entry mask ^M<R2,R3,R4,R5,R6,R7,R8,R9,R10,R11,IV,DV>"
	if got := dec.Format(Options{Style: StyleDebugger}); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestFormatAssemblerStyle checks that Format in the assembler style is
// String without a symbolizer, and with one names only addresses, not
// the caller's Decoded.
func TestFormatAssemblerStyle(t *testing.T) {
	const pc = 0x1000

	r := make(SliceReader, pc+7)
	copy(r[pc:], []byte{0xD0, 0xEF, 0xFA, 0xF1, 0xFF, 0xFF, 0x50})

	dec, err := Disassemble(r, pc)
	if err != nil {
		t.Fatal(err)
	}

	if got := dec.Format(Options{}); got != dec.String() {
		t.Errorf("Format = %q, String = %q", got, dec.String())
	}

	if got, want := dec.Format(Options{Symbolizer: mapSymbolizer{0x200: "COUNT"}}), "MOVL L^COUNT,R0"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	if dec.Operands[0].Symbol != "" {
		t.Errorf("Format changed the caller's operand: Symbol %q", dec.Operands[0].Symbol)
	}
}

// mapCells names the fixup cells in a map.
type mapCells map[uint32]string

func (m mapCells) Cell(addr uint32) (string, bool) {
	name, ok := m[addr]

	return name, ok
}

// TestCells checks the cell option: a deferred relative operand through a
// fixup cell is shown as its G^ reference when Cells names the cell, and
// as the debugger shows it (the cell's address, named or not) otherwise.
func TestCells(t *testing.T) {
	const pc = 0x1000

	// CALLS #1,@L^00002000 (the displacement 0xFF9 ends at 0x1007), and
	// MOVL L^00002000,R0 (0xFFA, ending at 0x1006), which isn't deferred.
	calls := []byte{0xFB, 0x01, 0xFF, 0xF9, 0x0F, 0x00, 0x00}
	movl := []byte{0xD0, 0xEF, 0xFA, 0x0F, 0x00, 0x00, 0x50}

	cells := mapCells{0x2000: "LIB$PUT_OUTPUT"}
	names := mapSymbolizer{0x2000: "SUB2+0F0"}

	cases := []struct {
		name  string
		bytes []byte
		opts  Options
		want  string
	}{
		{"debugger, no cells", calls, Options{Style: StyleDebugger, Symbolizer: names}, "CALLS    S^#01,@L^SUB2+0F0"},
		{"debugger, cells", calls, Options{Style: StyleDebugger, Symbolizer: names, Cells: cells}, "CALLS    S^#01,G^LIB$PUT_OUTPUT"},
		{"assembler, cells", calls, Options{Cells: cells}, "CALLS S^#1,G^LIB$PUT_OUTPUT"},
		{"not a cell", calls, Options{Style: StyleDebugger, Symbolizer: names, Cells: mapCells{}}, "CALLS    S^#01,@L^SUB2+0F0"},
		{"not deferred", movl, Options{Style: StyleDebugger, Symbolizer: names, Cells: cells}, "MOVL     L^SUB2+0F0,R0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := make(SliceReader, pc+len(tc.bytes))
			copy(r[pc:], tc.bytes)

			dec, err := Disassemble(r, pc)
			if err != nil {
				t.Fatal(err)
			}

			if got := dec.Format(tc.opts); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}

			if dec.Operands[1].Cell != "" {
				t.Errorf("Format changed the caller's operand: Cell %q", dec.Operands[1].Cell)
			}
		})
	}
}
