package asm

import "testing"

// TestForwardReferenceFixups exercises every fixup kind's patch-in
// behavior directly against the symbol table, independent of the operand
// encoder that normally drives it — matching set_symbol()'s fixup switch
// in asm_symbols.c.
func TestForwardReferenceFixups(t *testing.T) {
	cases := []struct {
		name   string
		kind   fixupKind
		loc    uint32
		value  uint32
		verify func(t *testing.T, a *Assembler)
	}{
		{
			name: "addr byte truncates the value itself",
			kind: fixAddrB, loc: 0x300, value: 0x1FF, // truncates to 0xFF
			verify: func(t *testing.T, a *Assembler) {
				if got := a.ByteAt(0x300); got != 0xFF {
					t.Errorf("byte = %#x, want 0xFF", got)
				}
			},
		},
		{
			name: "disp byte is relative to the fixup location",
			kind: fixDispB, loc: 0x300, value: 0x305,
			verify: func(t *testing.T, a *Assembler) {
				if got := int8(a.ByteAt(0x300)); got != 5 {
					t.Errorf("disp = %d, want 5", got)
				}
			},
		},
		{
			name: "branch byte subtracts the field size too",
			kind: fixBranchB, loc: 0x300, value: 0x306, // 0x306 - 0x300 - 1 = 5
			verify: func(t *testing.T, a *Assembler) {
				if got := int8(a.ByteAt(0x300)); got != 5 {
					t.Errorf("branch disp = %d, want 5", got)
				}
			},
		},
		{
			name: "addr word stores the absolute value",
			kind: fixAddrW, loc: 0x300, value: 0x1234,
			verify: func(t *testing.T, a *Assembler) {
				got := uint16(a.ByteAt(0x300)) | uint16(a.ByteAt(0x301))<<8
				if got != 0x1234 {
					t.Errorf("word = %#x, want 0x1234", got)
				}
			},
		},
		{
			name: "addr long stores the absolute value",
			kind: fixAddrL, loc: 0x300, value: 0xDEADBEEF,
			verify: func(t *testing.T, a *Assembler) {
				got := uint32(a.ByteAt(0x300)) | uint32(a.ByteAt(0x301))<<8 |
					uint32(a.ByteAt(0x302))<<16 | uint32(a.ByteAt(0x303))<<24
				if got != 0xDEADBEEF {
					t.Errorf("long = %#x, want 0xDEADBEEF", got)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := New()
			if _, _, err := a.getSymbol("FWD", true, tc.loc, tc.kind); err != nil {
				t.Fatalf("getSymbol (forward): %v", err)
			}
			if err := a.setSymbol("FWD", tc.value, SymNone, false); err != nil {
				t.Fatalf("setSymbol: %v", err)
			}

			tc.verify(t, a)
		})
	}
}

func TestForwardReferenceOutOfRange(t *testing.T) {
	a := New()

	if _, _, err := a.getSymbol("FWD", true, 0x300, fixDispB); err != nil {
		t.Fatal(err)
	}

	if err := a.setSymbol("FWD", 0x1000, SymNone, false); err == nil {
		t.Fatal("expected an out-of-range byte displacement error")
	}
}

func TestUndefinedSymbolWithoutForward(t *testing.T) {
	a := New()

	if _, _, err := a.getSymbol("NOPE", false, 0, fixNone); err == nil {
		t.Fatal("expected an undefined-symbol error")
	}
}

func TestDuplicateSymbolDefinition(t *testing.T) {
	a := New()

	if err := a.setSymbol("FOO", 1, SymLabel, true); err != nil {
		t.Fatal(err)
	}

	if err := a.setSymbol("FOO", 2, SymLabel, true); err == nil {
		t.Fatal("expected a duplicate-definition error")
	}
}

func TestLocalSymbolScoping(t *testing.T) {
	a := New()

	a.curEntry = "MAIN"
	if err := a.setSymbol("_LOOP", 0x100, SymLabel, true); err != nil {
		t.Fatal(err)
	}

	if _, ok := a.symbols.find("MAIN_LOOP"); !ok {
		t.Fatal("expected _LOOP to resolve under the MAIN_LOOP scoped name")
	}

	a.curEntry = "OTHER"
	if err := a.setSymbol("_LOOP", 0x200, SymLabel, true); err != nil {
		t.Fatal(err)
	}

	if _, ok := a.symbols.find("OTHER_LOOP"); !ok {
		t.Fatal("expected the second _LOOP to scope independently under OTHER_LOOP")
	}

	// A double-underscore name is never scoped, matching scope_name()'s
	// "not with '_' followed by '_'" carve-out for __ENTRY/__FIRST/etc.
	if err := a.setSymbol("__ENTRY", 0x300, SymNone, false); err != nil {
		t.Fatal(err)
	}

	if _, ok := a.symbols.find("__ENTRY"); !ok {
		t.Fatal("expected __ENTRY to be stored unscoped")
	}
}

func TestBuiltinSymbolsSeeded(t *testing.T) {
	a := New()

	sym, ok := a.symbols.find("EXC$CHMK")
	if !ok {
		t.Fatal("expected EXC$CHMK to be a predefined system symbol")
	}

	if sym.value != 0x40 {
		t.Errorf("EXC$CHMK = %#x, want 0x40", sym.value)
	}

	sym, ok = a.symbols.find("OPC$_HALT")
	if !ok {
		t.Fatal("expected OPC$_HALT to be predefined from the instruction table")
	}

	if sym.value != 0 {
		t.Errorf("OPC$_HALT = %#x, want 0", sym.value)
	}
}

// TestSymbols checks Assembler.Symbols (Phase 12, added for the console's
// ASM command to merge a program's own labels into Console.Symbols):
// builtin/seeded symbols and anything still forward-unresolved must be
// excluded, but the program's own labels must come through.
func TestSymbols(t *testing.T) {
	a := New()
	if _, err := a.Assemble("FOO:\tHALT\n\t.SET BAR,^X10\n"); err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	syms := a.Symbols()
	if _, ok := syms["EXC$CHMK"]; ok {
		t.Error("expected a builtin symbol (EXC$CHMK) to be excluded from Symbols()")
	}

	if _, ok := syms["OPC$_HALT"]; ok {
		t.Error("expected a builtin symbol (OPC$_HALT) to be excluded from Symbols()")
	}

	if v, ok := syms["FOO"]; !ok || v.Value != a.origin {
		t.Errorf("FOO = (%#x, %v), want (%#x, true)", v.Value, ok, a.origin)
	}

	if v, ok := syms["FOO"]; !ok || v.Entry {
		t.Errorf("FOO.Entry = %v, want false (it's a label, not a .ENTRY)", v.Entry)
	}

	if v, ok := syms["BAR"]; !ok || v.Value != 0x10 {
		t.Errorf("BAR = (%#x, %v), want (0x10, true)", v.Value, ok)
	}
}

// TestSymbolsEntryFlag checks that Symbols() reports Entry=true for a name
// defined by .ENTRY, and false for an ordinary label -- the flag the
// console's ASM command (asm.go) relies on to merge entry-point-ness into
// its own symbol table for the disassembler's entry-mask detection.
func TestSymbolsEntryFlag(t *testing.T) {
	a := New()

	if _, err := a.Assemble("\t.ENTRY\tMAIN,^M<R2>\n\tRET\nOTHER:\tHALT\n"); err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	syms := a.Symbols()
	if v, ok := syms["MAIN"]; !ok || !v.Entry {
		t.Errorf("MAIN.Entry = (%v, %v), want (true, true)", v.Entry, ok)
	}

	if v, ok := syms["OTHER"]; !ok || v.Entry {
		t.Errorf("OTHER.Entry = (%v, %v), want (false, true)", v.Entry, ok)
	}
}

// TestTakeEntry checks the one-shot consume-and-clear semantics
// Console.Assemble relies on when reusing one Assembler across several
// files (a later file with a bare ".END" must not re-report an earlier
// file's named entry).
func TestTakeEntry(t *testing.T) {
	a := New()

	if _, err := a.Assemble("MAIN:\tHALT\n\t.END\tMAIN\n"); err != nil {
		t.Fatalf("Assemble: %v", err)
	}

	addr, ok := a.TakeEntry()
	if !ok || addr != a.origin {
		t.Fatalf("TakeEntry = (%#x, %v), want (%#x, true)", addr, ok, a.origin)
	}

	if _, ok := a.TakeEntry(); ok {
		t.Error("expected a second TakeEntry to report no entry")
	}

	if _, err := a.Assemble("\tHALT\n\t.END\n"); err != nil {
		t.Fatalf("Assemble (second file): %v", err)
	}

	if _, ok := a.TakeEntry(); ok {
		t.Error("expected a bare .END (no name) not to report an entry")
	}
}

// TestSetS0Origin checks that a relocated S0 origin is honored both by
// S0Origin() and by subsequently assembled S0-region content — the console
// (internal/console/asm.go) relies on this to keep a live ASM session's S0
// deposits out of the address range VMINIT's own page tables occupy.
func TestSetS0Origin(t *testing.T) {
	a := New()
	a.SetMicrokernel(true)

	const newBase = 0x80100000

	a.SetS0Origin(newBase)

	if got := a.S0Origin(); got != newBase {
		t.Fatalf("S0Origin() = %#x, want %#x", got, newBase)
	}

	if _, err := a.Assemble(".REGION SYSTEM\nX:\t.BLKL\t1\n"); err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	
	if v, ok := a.Symbols()["X"]; !ok || v.Value != newBase {
		t.Errorf("X = (%#x, %v), want (%#x, true)", v.Value, ok, newBase)
	}
}
