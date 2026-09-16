package cpu

import "sort"

//go:generate go run ./gen -in ../../reference/eVAX/eVAX/Headers/instruction_table.h -out instructions_table.go

// Opcode identifies an instruction: the extended-opcode prefix byte (0 for
// an ordinary single-byte opcode) and the opcode byte itself. Matches
// struct OPCODE's extended/function fields and struct INSTRUCTION's
// extended/opcode fields.
type Opcode struct {
	Extended byte
	Function byte
}

// Instruction is one entry from the VAX instruction table: everything known
// about an opcode's operands, independent of any particular decode. This is
// the static-data subset of the C source's struct INSTRUCTION — its
// `routine` (dispatch handler), `use_count` and `debugdata` fields are
// runtime/profiling concerns attached separately (see docs/PHASE-03.md's
// design notes on the dispatch mechanism), not table data.
type Instruction struct {
	Name         string
	Opcode       Opcode
	OperandCount int
	// Scale gives each operand's natural size in bytes (1, 2, 4, or 8);
	// entries beyond OperandCount are 0 and unused, matching the C source's
	// scale[6] array.
	Scale [6]int
	// Access gives each operand's AccessKind; entries beyond OperandCount
	// are AccessNone.
	Access [6]AccessKind
	// Type says how this instruction's short-literal operands (addressing
	// modes 0-3) are interpreted, matching struct INSTRUCTION's type field.
	Type ShortLiteralType
}

// Table is the VAX instruction table: every opcode the decoder knows about,
// indexed for fast lookup by Opcode. Single-byte opcodes (Extended == 0)
// index a flat array directly, matching the C source's instruction[]; the
// far smaller set of extended (two-byte) opcodes use a map instead of the C
// source's linear scan — same result, O(1) instead of O(n), a pure lookup-
// strategy improvement (see docs/PHASE-03.md).
type Table struct {
	single   [256]*Instruction
	extended map[uint16]*Instruction
	// handlers holds each Instruction's dispatch Handler, populated by
	// SetHandler (Phases 04-07 register into it at package init) rather
	// than at table-generation time — see dispatch.go.
	handlers map[*Instruction]Handler
	// byName lazily indexes instructions by mnemonic for ByName; built on
	// first use rather than at table-generation time, same rationale as
	// handlers above.
	byName map[string]*Instruction
}

func extendedKey(op Opcode) uint16 {
	return uint16(op.Extended)<<8 | uint16(op.Function)
}

// newTable builds a Table from a flat list of instructions, as produced by
// the generator in internal/cpu/gen. Panics on a duplicate opcode, since
// that can only mean the generated table (or its source) is corrupt.
func newTable(instructions []*Instruction) *Table {
	t := &Table{extended: make(map[uint16]*Instruction)}

	for _, inst := range instructions {
		if inst.Opcode.Extended == 0 {
			if t.single[inst.Opcode.Function] != nil {
				panic("cpu: duplicate opcode " + inst.Name)
			}

			t.single[inst.Opcode.Function] = inst

			continue
		}

		key := extendedKey(inst.Opcode)
		if _, exists := t.extended[key]; exists {
			panic("cpu: duplicate extended opcode " + inst.Name)
		}

		t.extended[key] = inst
	}

	return t
}

// Lookup returns the Instruction for op, or nil if op is not a defined
// opcode (a reserved-to-Digital/privileged-instruction fault at decode
// time, matching decode_opcode.c's "not found" path for extended opcodes).
func (t *Table) Lookup(op Opcode) *Instruction {
	if op.Extended == 0 {
		return t.single[op.Function]
	}

	return t.extended[extendedKey(op)]
}

// ByName returns the Instruction with the given mnemonic (matched exactly,
// case-sensitively — callers such as internal/asm are responsible for any
// case-folding), or nil if no opcode has that name. Used by the assembler
// (Phase 11) to translate a source mnemonic into its opcode/operand
// metadata without duplicating the instruction table.
func (t *Table) ByName(name string) *Instruction {
	if t.byName == nil {
		t.byName = make(map[string]*Instruction)
		for _, inst := range t.single {
			if inst != nil {
				t.byName[inst.Name] = inst
			}
		}

		for _, inst := range t.extended {
			t.byName[inst.Name] = inst
		}
	}

	return t.byName[name]
}

// All returns every defined Instruction, single-byte opcodes first (in
// ascending opcode order), then extended (two-byte) opcodes (in ascending
// Opcode order) — a deterministic enumeration for callers that need to
// walk the whole table (SHOW INSTRUCTIONS), not the exact declaration
// order of the C source's instruction[] array.
func (t *Table) All() []*Instruction {
	out := make([]*Instruction, 0, 256+len(t.extended))

	for _, inst := range t.single {
		if inst != nil {
			out = append(out, inst)
		}
	}

	keys := make([]uint16, 0, len(t.extended))
	for k := range t.extended {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	for _, k := range keys {
		out = append(out, t.extended[k])
	}

	return out
}

// Implemented reports whether inst has a real registered Handler, as
// opposed to defaulting to unimplementedHandler — the Go equivalent of
// console_show.c's own `instruction[i].routine == 0L` check (SHOW
// INSTRUCTIONS' implemented/unimplemented split).
func (t *Table) Implemented(inst *Instruction) bool {
	_, ok := t.handlers[inst]
	
	return ok
}

// Instructions returns the built-in VAX instruction table, for use by
// callers outside this package (such as internal/asm's assembler/
// disassembler) that need to resolve a mnemonic or Opcode to its operand
// metadata.
func Instructions() *Table { return instructionTable }
