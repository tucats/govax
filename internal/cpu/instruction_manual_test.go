package cpu

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
)

// manualForm renders an instruction's operands in the VAX Architecture
// Reference Manual's notation, "rl,rl,wl": per operand, an access letter
// (r, w, m, a, v, b, or govax's i) and a data-type letter (see
// datatype.go). It is how the tests below compare the generated table with
// the manual.
func manualForm(inst *Instruction) string {
	accessLetter := map[AccessKind]string{
		AccessRead: "r", AccessWrite: "w", AccessModify: "m", AccessAddress: "a",
		AccessVarField: "v", AccessBranch: "b", AccessImmediate: "i",
	}

	ops := make([]string, inst.OperandCount)
	for i := range ops {
		ops[i] = accessLetter[inst.Access[i]] + inst.DataType[i].Letter()
	}

	return strings.Join(ops, ",")
}

// phase35Instructions lists, from the manual's opcode table and format
// lines, each instruction Phase 35 implements, and each whose table entry
// it corrected. It is transcribed separately from the generator's own
// gen/operands.go, so a mistake in the generator (or in that file's
// opcodes, for the instructions the C header lacked) shows up here as a
// disagreement.
var phase35Instructions = []struct {
	name     string
	ext, opc byte // ext is 0 for a one-byte opcode, else the prefix (0xFD)
	operands string
}{
	// Packed decimal.
	{"CVTPS", 0, 0x08, "rw,ab,rw,ab"},
	{"CVTSP", 0, 0x09, "rw,ab,rw,ab"},
	{"ADDP4", 0, 0x20, "rw,ab,rw,ab"},
	{"ADDP6", 0, 0x21, "rw,ab,rw,ab,rw,ab"},
	{"SUBP4", 0, 0x22, "rw,ab,rw,ab"},
	{"SUBP6", 0, 0x23, "rw,ab,rw,ab,rw,ab"},
	{"CVTPT", 0, 0x24, "rw,ab,ab,rw,ab"},
	{"MULP", 0, 0x25, "rw,ab,rw,ab,rw,ab"},
	{"CVTTP", 0, 0x26, "rw,ab,ab,rw,ab"},
	{"DIVP", 0, 0x27, "rw,ab,rw,ab,rw,ab"},
	{"MOVP", 0, 0x34, "rw,ab,ab"},
	{"CMPP3", 0, 0x35, "rw,ab,ab"},
	{"CVTPL", 0, 0x36, "rw,ab,wl"},
	{"CMPP4", 0, 0x37, "rw,ab,rw,ab"},
	{"EDITPC", 0, 0x38, "rw,ab,ab,ab"},
	{"ASHP", 0, 0xF8, "rb,rw,ab,rb,rw,ab"},
	{"CVTLP", 0, 0xF9, "rl,rw,ab"},

	// F_floating and D_floating.
	{"EMODF", 0, 0x54, "rf,rb,rf,wl,wf"},
	{"POLYF", 0, 0x55, "rf,rw,ab"},
	{"CVTFD", 0, 0x56, "rf,wd"},
	{"EMODD", 0, 0x74, "rd,rb,rd,wl,wd"},
	{"POLYD", 0, 0x75, "rd,rw,ab"},
	{"CVTDF", 0, 0x76, "rd,wf"},

	// Conversions among the floating formats.
	{"CVTDH", 0xFD, 0x32, "rd,wh"},
	{"CVTGF", 0xFD, 0x33, "rg,wf"},
	{"CVTGH", 0xFD, 0x56, "rg,wh"},
	{"CVTHG", 0xFD, 0x76, "rh,wg"},
	{"CVTFH", 0xFD, 0x98, "rf,wh"},
	{"CVTFG", 0xFD, 0x99, "rf,wg"},
	{"CVTHF", 0xFD, 0xF6, "rh,wf"},
	{"CVTHD", 0xFD, 0xF7, "rh,wd"},

	// G_floating.
	{"ADDG2", 0xFD, 0x40, "rg,mg"},
	{"ADDG3", 0xFD, 0x41, "rg,rg,wg"},
	{"SUBG2", 0xFD, 0x42, "rg,mg"},
	{"SUBG3", 0xFD, 0x43, "rg,rg,wg"},
	{"MULG2", 0xFD, 0x44, "rg,mg"},
	{"MULG3", 0xFD, 0x45, "rg,rg,wg"},
	{"DIVG2", 0xFD, 0x46, "rg,mg"},
	{"DIVG3", 0xFD, 0x47, "rg,rg,wg"},
	{"CVTGB", 0xFD, 0x48, "rg,wb"},
	{"CVTGW", 0xFD, 0x49, "rg,ww"},
	{"CVTGL", 0xFD, 0x4A, "rg,wl"},
	{"CVTRGL", 0xFD, 0x4B, "rg,wl"},
	{"CVTBG", 0xFD, 0x4C, "rb,wg"},
	{"CVTWG", 0xFD, 0x4D, "rw,wg"},
	{"CVTLG", 0xFD, 0x4E, "rl,wg"},
	{"ACBG", 0xFD, 0x4F, "rg,rg,mg,bw"},
	{"MOVG", 0xFD, 0x50, "rg,wg"},
	{"CMPG", 0xFD, 0x51, "rg,rg"},
	{"MNEGG", 0xFD, 0x52, "rg,wg"},
	{"TSTG", 0xFD, 0x53, "rg"},
	{"EMODG", 0xFD, 0x54, "rg,rw,rg,wl,wg"},
	{"POLYG", 0xFD, 0x55, "rg,rw,ab"},

	// H_floating.
	{"ADDH2", 0xFD, 0x60, "rh,mh"},
	{"ADDH3", 0xFD, 0x61, "rh,rh,wh"},
	{"SUBH2", 0xFD, 0x62, "rh,mh"},
	{"SUBH3", 0xFD, 0x63, "rh,rh,wh"},
	{"MULH2", 0xFD, 0x64, "rh,mh"},
	{"MULH3", 0xFD, 0x65, "rh,rh,wh"},
	{"DIVH2", 0xFD, 0x66, "rh,mh"},
	{"DIVH3", 0xFD, 0x67, "rh,rh,wh"},
	{"CVTHB", 0xFD, 0x68, "rh,wb"},
	{"CVTHW", 0xFD, 0x69, "rh,ww"},
	{"CVTHL", 0xFD, 0x6A, "rh,wl"},
	{"CVTRHL", 0xFD, 0x6B, "rh,wl"},
	{"CVTBH", 0xFD, 0x6C, "rb,wh"},
	{"CVTWH", 0xFD, 0x6D, "rw,wh"},
	{"CVTLH", 0xFD, 0x6E, "rl,wh"},
	{"ACBH", 0xFD, 0x6F, "rh,rh,mh,bw"},
	{"MOVH", 0xFD, 0x70, "rh,wh"},
	{"CMPH", 0xFD, 0x71, "rh,rh"},
	{"MNEGH", 0xFD, 0x72, "rh,wh"},
	{"TSTH", 0xFD, 0x73, "rh"},
	{"EMODH", 0xFD, 0x74, "rh,rw,rh,wl,wh"},
	{"POLYH", 0xFD, 0x75, "rh,rw,ab"},

	// Octaword.
	{"CLRO", 0xFD, 0x7C, "wo"},
	{"MOVO", 0xFD, 0x7D, "ro,wo"},
	{"MOVAO", 0xFD, 0x7E, "ao,wl"},
	{"PUSHAO", 0xFD, 0x7F, "ao"},

	// Entries the generator's manualCorrections fixed.
	{"CVTWL", 0, 0x32, "rw,wl"},
	{"CVTWB", 0, 0x33, "rw,wb"},
	{"CVTBL", 0, 0x98, "rb,wl"},
	{"CVTBW", 0, 0x99, "rb,ww"},
	{"CVTLB", 0, 0xF6, "rl,wb"},
	{"CVTLW", 0, 0xF7, "rl,ww"},
	{"BISW3", 0, 0xA9, "rw,rw,ww"},
	{"PROBER", 0, 0x0C, "rb,rw,ab"},
	{"PROBEW", 0, 0x0D, "rb,rw,ab"},
	{"INSQUE", 0, 0x0E, "ab,ab"},
	{"REMQUE", 0, 0x0F, "ab,wl"},
	{"CALLG", 0, 0xFA, "ab,ab"},
	{"CALLS", 0, 0xFB, "rl,ab"},
	{"ACBF", 0, 0x4F, "rf,rf,mf,bw"},
}

// TestInstructionTableMatchesManual checks each phase35Instructions entry
// is in the table, at its opcode, with the manual's operands.
func TestInstructionTableMatchesManual(t *testing.T) {
	for _, want := range phase35Instructions {
		inst := instructionTable.Lookup(Opcode{Extended: want.ext, Function: want.opc})
		if inst == nil {
			t.Errorf("%s: no table entry at opcode %02X%02X", want.name, want.ext, want.opc)

			continue
		}

		if inst.Name != want.name {
			t.Errorf("opcode %02X%02X is %s, want %s", want.ext, want.opc, inst.Name, want.name)
		}

		if got := manualForm(inst); got != want.operands {
			t.Errorf("%s operands = %q, want %q", want.name, got, want.operands)
		}
	}
}

// TestInstructionTableConsistent checks, for every entry, that the
// generated columns agree with each other: each operand's Scale is its
// data type's size, unused slots are empty, and the short-literal Type is
// floating exactly when the instruction reads a floating operand. (ACBF
// once had an integer Type although it reads only floating operands.)
func TestInstructionTableConsistent(t *testing.T) {
	for _, inst := range instructionTable.All() {
		readsFloat := false

		for i := 0; i < 6; i++ {
			dt := inst.DataType[i]

			if i >= inst.OperandCount {
				if dt != DataNone || inst.Scale[i] != 0 || inst.Access[i] != AccessNone {
					t.Errorf("%s: unused operand slot %d isn't empty", inst.Name, i)
				}

				continue
			}

			if dt == DataNone {
				t.Errorf("%s: operand %d has no data type", inst.Name, i)
			}

			if inst.Scale[i] != dt.Size() {
				t.Errorf("%s: operand %d scale %d, but its type %v is %d bytes",
					inst.Name, i, inst.Scale[i], dt, dt.Size())
			}

			if (inst.Access[i] == AccessRead || inst.Access[i] == AccessModify) && dt.IsFloat() {
				readsFloat = true
			}
		}

		if got := inst.Type == ShortLiteralFloat; got != readsFloat {
			t.Errorf("%s: short-literal type floating = %v, want %v", inst.Name, got, readsFloat)
		}
	}
}

// TestNewInstructionsDecodeAsReserved checks that each Phase 35
// instruction decodes (its operands are known now), and until it has a
// handler, executes as a reserved-instruction fault, as every
// unimplemented instruction does. Each read operand is an immediate (I^#,
// the bytes of the value inline in the instruction stream), the widest
// form there is, so the decoder must step over 16 bytes for an H_floating
// or octaword operand without failing.
func TestNewInstructionsDecodeAsReserved(t *testing.T) {
	for _, want := range phase35Instructions {
		inst := instructionTable.Lookup(Opcode{Extended: want.ext, Function: want.opc})
		if inst == nil || instructionTable.Implemented(inst) {
			continue // already reported, or a corrected entry that runs
		}

		t.Run(want.name, func(t *testing.T) {
			code := instructionBytes(inst)

			cpu, mem := fixture()
			cpu.SetGPR(vax.PC, base)
			putBytes(t, cpu, mem, base, code...)

			d, err := decodeInstruction(cpu, mem, instructionTable)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			if got, want := d.NextPC-base, uint32(len(code)); got != want {
				t.Errorf("decoded length %d, want %d", got, want)
			}

			e := &Engine{cpu: cpu, mem: mem, table: instructionTable}
			err = instructionTable.HandlerFor(inst)(e, &d)
			if f, ok := err.(*Fault); !ok || f.Code != ExcPrivileged {
				t.Errorf("executing = %v, want a reserved-instruction fault", err)
			}
		})
	}
}

// instructionBytes encodes inst with simple operands: an immediate (I^#,
// mode 8 on the PC, 0x8F, then the value's bytes) for each operand it
// reads, register deferred "(R1)" (0x61) for an address, register R2
// (0x52) for anything written or modified, and a zero displacement of the
// right size for a branch.
func instructionBytes(inst *Instruction) []byte {
	var b []byte

	if inst.Opcode.Extended != 0 {
		b = append(b, inst.Opcode.Extended)
	}

	b = append(b, inst.Opcode.Function)

	for i := 0; i < inst.OperandCount; i++ {
		switch inst.Access[i] {
		case AccessRead:
			b = append(b, 0x8F)
			b = append(b, make([]byte, inst.Scale[i])...)
		case AccessAddress, AccessVarField:
			b = append(b, 0x61)
		case AccessBranch:
			b = append(b, make([]byte, inst.Scale[i])...)
		default:
			b = append(b, 0x52)
		}
	}

	return b
}
