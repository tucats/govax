package asm

import (
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

func macroAssembler() *Assembler {
	a := New(true)
	a.SetDialect(DialectMACRO)

	return a
}

func TestDialectDefaultsToConsole(t *testing.T) {
	if d := New(true).Dialect(); d != DialectConsole {
		t.Fatalf("Dialect() = %v, want DialectConsole", d)
	}
}

// TestMACRODialectRejectsConsoleDirectives checks that every directive the
// table marks console-only is an error in the MACRO dialect, and still
// assembles (or at least isn't rejected as non-MACRO) in the console one.
func TestMACRODialectRejectsConsoleDirectives(t *testing.T) {
	for name, d := range directives {
		if d.dialects.has(DialectMACRO) {
			continue
		}

		t.Run(name, func(t *testing.T) {
			_, err := macroAssembler().Assemble("." + name + " 4")
			requireCode(t, err, vmserrors.VAX_NOTMACRO)
		})
	}
}

func TestMACRODialectAcceptsSharedDirectives(t *testing.T) {
	a := macroAssembler()

	out, err := a.Assemble(`.ENTRY MAIN,^M<R2>
	.BYTE 1,2
	.WORD 3
	.LONG 4
	.ASCII /AB/
	.BLKB 2
	.IF EQ,0
	.BYTE 5
	.ENDC
	.IIF NE,0, .BYTE 6
	RET
	.END MAIN`)
	if err != nil {
		t.Fatal(err)
	}

	requireBytes(t, out,
		0x04, 0x00, // entry mask
		0x01, 0x02, // .BYTE
		0x03, 0x00, // .WORD
		0x04, 0x00, 0x00, 0x00, // .LONG
		'A', 'B', // .ASCII
		0x00, 0x00, // .BLKB
		0x05, // .IF EQ,0
		0x04, // RET
	)
}

// TestMACRODialectNeedsDot checks that MACRO-32 directives need their
// leading ".": without one, JEQL is an unknown instruction, not the
// console's long-jump alias, and BYTE is a symbol, not a directive.
func TestMACRODialectNeedsDot(t *testing.T) {
	_, err := macroAssembler().Assemble("JEQL 0")
	requireCode(t, err, vmserrors.VAX_BADOPCODE)

	_, err = macroAssembler().Assemble("BYTE 1")
	requireCode(t, err, vmserrors.VAX_BADOPCODE)

	// The console dialect still takes both without a dot.
	requireBytes(t, assembleBytes(t, "BYTE 1"), 0x01)
}

func TestConsoleDialectAcceptsConsoleDirectives(t *testing.T) {
	a := New(true)
	a.SetMicrokernel(true)

	if _, err := a.Assemble(".REGION S0\n.BYTE 7\n.REGION P0\n.BASE 300\n.BYTE 8"); err != nil {
		t.Fatal(err)
	}

	if got := a.ByteAt(a.S0Origin()); got != 7 {
		t.Errorf("S0 byte = %d, want 7", got)
	}

	if got := a.ByteAt(300); got != 8 {
		t.Errorf("byte at 300 = %d, want 8", got)
	}

	if got := a.S0End(); got != a.S0Origin()+1 {
		t.Errorf("S0End() = %#x, want %#x", got, a.S0Origin()+1)
	}

	if got := a.Deposit(); got != 301 {
		t.Errorf("Deposit() = %d, want 301", got)
	}
}
