package asm

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/vmserrors"
)

// The Phase 35 assembler fixture (testdata/insn35/asm): asm35.mar holds
// the directives and operand forms govax's assembler had to decide
// without seeing VAX MACRO's output, and vax/asm35.obj is VAX MACRO's
// object for it (VMS 7.1, simh VAX 8600; vax/asm35.log is the run).

var asm35Dir = filepath.Join("..", "..", "testdata", "insn35", "asm")

// asm35Refused are the fixture's lines VAX MACRO refused, with
// "%MACRO-E-DIRSYNX, Directive syntax error", and what it stored for
// them: zeros of the directive's size.
var asm35Refused = map[string]string{
	"Q1:\t.QUAD\t-1": "Q1:\t.QUAD\t0",
	"O1:\t.OCTA\t-1": "O1:\t.OCTA\t0",
}

// TestAsm35Refused checks govax's MACRO refuses the lines VAX MACRO did:
// a negative number in .QUAD or .OCTA. (The console dialect still takes
// them, as eVAX's fixtures write delta times so.)
func TestAsm35Refused(t *testing.T) {
	for line := range asm35Refused {
		_, err := macroAssembler().Assemble(line + "\n")
		requireCode(t, err, vmserrors.VAX_DIRSYNX)

		if _, err := New(true).Assemble(line + "\n"); err != nil {
			t.Errorf("console dialect %q: %v", line, err)
		}
	}
}

// TestAsm35Object compares govax's object for the fixture with VAX
// MACRO's, record for record, after two adjustments to VAX MACRO's, each
// for a reason:
//
//   - VAX MACRO's end-of-module record has severity ERROR, from the two
//     lines it refused; govax assembles the fixture with those lines as
//     the zeros VAX MACRO stored for them, without error.
//   - VAX MACRO stored .OCTA NEG (NEG = -3) as ^XFFFFFFFD, zeros, and then,
//     in its high quadword, the value of the .OCTA before it
//     (^X0123456789ABCDEF): a stale register, as far as one sample shows.
//     govax zero-extends, as for .QUAD NEG (docs/DEVIATIONS.md).
func TestAsm35Object(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(asm35Dir, "asm35.mar"))
	if err != nil {
		t.Fatal(err)
	}

	text := string(src)
	for from, to := range asm35Refused {
		if !strings.Contains(text, from) {
			t.Fatalf("asm35.mar has no line %q", from)
		}

		text = strings.Replace(text, from, to, 1)
	}

	a := macroAssembler()
	if _, err := a.Assemble(text); err != nil {
		t.Fatalf("assemble: %v", err)
	}

	real := readObjectFile(t, filepath.Join(asm35Dir, "vax", "asm35.obj"))

	// .OCTA NEG's stale high quadword, followed by the .PACKED -12 after
	// it (^X01, ^X2D).
	stale := []byte{0xFD, 0xFF, 0xFF, 0xFF, 0, 0, 0, 0, 0xEF, 0xCD, 0xAB, 0x89, 0x67, 0x45, 0x23, 0x01, 0x01, 0x2D}
	patched := false

	for _, rec := range real.Records {
		switch r := rec.(type) {
		case *obj.EOM:
			r.Severity = obj.SeveritySuccess
		case *obj.TIR:
			for _, c := range r.Commands {
				if i := bytes.Index(c.Data, stale); c.Op == obj.OpStoreImmediate && i >= 0 {
					copy(c.Data[i+8:i+16], make([]byte, 8))
					patched = true
				}
			}
		}
	}

	if !patched {
		t.Fatal("didn't find .OCTA NEG's stale high quadword in VAX MACRO's object")
	}

	requireSameObject(t, a, real)
}
