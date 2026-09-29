package obj

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// Op is a TIR command code (TIR$C_xxx), or OpStoreImmediate.
type Op int

// OpStoreImmediate is the STORE IMMEDIATE command. It has no TIR$C_ code:
// any command byte with bit 7 set is STORE IMMEDIATE, and its absolute
// value (1 to 128) is how many data bytes follow, to be stored as they
// are.
const OpStoreImmediate Op = -1

// MaxImmediate is the most bytes one STORE IMMEDIATE command can store.
const MaxImmediate = 128

// operandFormat is how a TIR command's operand bytes are laid out.
type operandFormat int

const (
	opNone     operandFormat = iota
	opName                   // a counted symbol name
	opByte                   // one byte
	opWord                   // two bytes
	opLong                   // four bytes
	opPsectB                 // a byte psect index and a byte offset
	opPsectW                 // a byte psect index and a word offset
	opPsectL                 // a byte psect index and a longword offset
	opWPsectB                // a word psect index and a byte offset
	opWPsectW                // a word psect index and a word offset
	opWPsectL                // a word psect index and a longword offset
	opEnvName                // a word environment index and a counted name
	opIndex                  // a one-byte literal index
	opField                  // STO_VPS: a bit position byte and a size byte
	opBytes                  // STO_RIVB: a count byte and that many bytes
	opCheckArg               // STA_CKARG: a name, an argument index, and a descriptor
)

// opInfo describes one TIR command.
type opInfo struct {
	name   string // the TIR$C_ name without its prefix: "STA_PL"
	format operandFormat
	signed bool // a byte or word value is sign-extended when stacked
	pop    int  // longwords the command removes from the linker's stack
	push   int  // longwords it then adds
}

// ops describes every TIR command, and opsByName finds one by its TIR$C_
// name without the prefix. They're built by a variable initializer, not an
// init function, so the package-level Op variables below that read
// opsByName are initialized after it.
var ops, opsByName = buildOps()

func buildOps() (map[Op]opInfo, map[string]Op) {
	byOp, byName := map[Op]opInfo{}, map[string]Op{}

	table := []opInfo{
		// Stack commands (section 7.4.1).
		{"STA_GBL", opName, false, 0, 1},
		{"STA_SB", opByte, true, 0, 1},
		{"STA_SW", opWord, true, 0, 1},
		{"STA_LW", opLong, false, 0, 1},
		{"STA_PB", opPsectB, true, 0, 1},
		{"STA_PW", opPsectW, true, 0, 1},
		{"STA_PL", opPsectL, false, 0, 1},
		{"STA_UB", opByte, false, 0, 1},
		{"STA_UW", opWord, false, 0, 1},
		{"STA_BFI", opNone, false, 1, 1},
		{"STA_WFI", opNone, false, 1, 1},
		{"STA_LFI", opNone, false, 1, 1},
		{"STA_EPM", opName, false, 0, 1},
		{"STA_CKARG", opCheckArg, false, 0, 1},
		{"STA_WPB", opWPsectB, true, 0, 1},
		{"STA_WPW", opWPsectW, true, 0, 1},
		{"STA_WPL", opWPsectL, false, 0, 1},
		{"STA_LSY", opEnvName, false, 0, 1},
		{"STA_LIT", opIndex, false, 0, 1},
		{"STA_LEPM", opEnvName, false, 0, 1},

		// Store commands (section 7.4.2).
		{"STO_SB", opNone, false, 1, 0},
		{"STO_SW", opNone, false, 1, 0},
		{"STO_L", opNone, false, 1, 0},
		{"STO_BD", opNone, false, 1, 0},
		{"STO_WD", opNone, false, 1, 0},
		{"STO_LD", opNone, false, 1, 0},
		{"STO_LI", opNone, false, 1, 0},
		{"STO_PIDR", opNone, false, 1, 0},
		{"STO_PICR", opNone, false, 1, 0},
		{"STO_RSB", opNone, false, 2, 0},
		{"STO_RSW", opNone, false, 2, 0},
		{"STO_RL", opNone, false, 2, 0},
		{"STO_VPS", opField, false, 1, 0},
		{"STO_USB", opNone, false, 1, 0},
		{"STO_USW", opNone, false, 1, 0},
		{"STO_RUB", opNone, false, 2, 0},
		{"STO_RUW", opNone, false, 2, 0},
		{"STO_B", opNone, false, 1, 0},
		{"STO_W", opNone, false, 1, 0},
		{"STO_RB", opNone, false, 2, 0},
		{"STO_RW", opNone, false, 2, 0},
		{"STO_RIVB", opBytes, false, 1, 0},
		{"STO_PIRR", opNone, false, 2, 0},

		// Operator commands (section 7.4.3).
		{"OPR_NOP", opNone, false, 0, 0},
		{"OPR_ADD", opNone, false, 2, 1},
		{"OPR_SUB", opNone, false, 2, 1},
		{"OPR_MUL", opNone, false, 2, 1},
		{"OPR_DIV", opNone, false, 2, 1},
		{"OPR_AND", opNone, false, 2, 1},
		{"OPR_IOR", opNone, false, 2, 1},
		{"OPR_EOR", opNone, false, 2, 1},
		{"OPR_NEG", opNone, false, 1, 1},
		{"OPR_COM", opNone, false, 1, 1},
		{"OPR_INSV", opField, false, 2, 1},
		{"OPR_ASH", opNone, false, 2, 1},
		{"OPR_USH", opNone, false, 2, 1},
		{"OPR_ROT", opNone, false, 2, 1},
		{"OPR_SEL", opNone, false, 3, 1},
		{"OPR_REDEF", opName, false, 0, 0},
		{"OPR_DFLIT", opIndex, false, 1, 0},

		// Control commands (section 7.4.4).
		{"CTL_SETRB", opNone, false, 1, 0},
		{"CTL_AUGRB", opLong, false, 0, 0},
		{"CTL_DFLOC", opNone, false, 1, 0},
		{"CTL_STLOC", opNone, false, 1, 0},
		{"CTL_STKDL", opNone, false, 1, 1},
	}

	for _, info := range table {
		op := Op(objConst("TIR$C_" + info.name))
		byOp[op] = info
		byName[info.name] = op
	}

	return byOp, byName
}

// Frequently used commands, for the assembler and tests.
var (
	OpStackGlobal      = opsByName["STA_GBL"]
	OpStackLong        = opsByName["STA_LW"]
	OpStackPsectLong   = opsByName["STA_PL"]
	OpStoreLong        = opsByName["STO_L"]
	OpStoreWord        = opsByName["STO_W"]
	OpStoreByte        = opsByName["STO_B"]
	OpStoreLongDisp    = opsByName["STO_LD"]
	OpStoreWordDisp    = opsByName["STO_WD"]
	OpStoreByteDisp    = opsByName["STO_BD"]
	OpStorePICodeRef   = opsByName["STO_PICR"]
	OpStorePIDataRef   = opsByName["STO_PIDR"]
	OpStoreRepeatByte  = opsByName["STO_RB"]
	OpAdd              = opsByName["OPR_ADD"]
	OpSubtract         = opsByName["OPR_SUB"]
	OpSetRelocBase     = opsByName["CTL_SETRB"]
	OpAugmentRelocBase = opsByName["CTL_AUGRB"]
)

// OpByName returns the command whose TIR$C_ name (without the prefix) is
// name, such as "STA_PL".
func OpByName(name string) (Op, bool) {
	op, ok := opsByName[strings.ToUpper(name)]

	return op, ok
}

func (op Op) String() string {
	if op == OpStoreImmediate {
		return "STO_IMM"
	}

	if info, ok := ops[op]; ok {
		return info.name
	}

	return fmt.Sprintf("TIR command %d", int(op))
}

// StackEffect reports how many longwords the command removes from the
// linker's stack and then adds.
func (op Op) StackEffect() (pop, push int) {
	info := ops[op]

	return info.pop, info.push
}

// Command is one TIR (or DBG or TBT) command. Which fields mean anything
// depends on Op's operand format; the rest are zero. Value holds a byte,
// word, or longword operand as it appears in the record, zero-extended;
// StackedValue applies a signed command's sign extension.
type Command struct {
	Op    Op
	Name  string // STA_GBL, STA_EPM, STA_LSY, STA_LEPM, STA_CKARG, OPR_REDEF
	Psect uint16 // STA_PB and friends
	Env   uint16 // STA_LSY, STA_LEPM: the environment index
	Value uint32 // the constant, or the offset from the psect base
	Index byte   // STA_LIT, OPR_DFLIT: the literal; STA_CKARG: the argument
	Pos   byte   // STO_VPS, OPR_INSV: the first bit
	Size  byte   // STO_VPS, OPR_INSV: the field width
	Data  []byte // STORE IMMEDIATE and STO_RIVB data; STA_CKARG's descriptor
}

// StackedValue is the longword the command pushes for a constant or a psect
// offset: Value, sign-extended from its operand width for a signed command.
func (c Command) StackedValue() uint32 {
	info := ops[c.Op]
	if !info.signed {
		return c.Value
	}

	switch info.format {
	case opByte, opPsectB, opWPsectB:
		return uint32(int32(int8(c.Value)))
	case opWord, opPsectW, opWPsectW:
		return uint32(int32(int16(c.Value)))
	}

	return c.Value
}

// decodeCommand reads one command from the front of b, returning it and
// the number of bytes it used.
func decodeCommand(b []byte) (Command, int, error) {
	if len(b) == 0 {
		return Command{}, 0, fmt.Errorf("missing TIR command")
	}

	if b[0]&0x80 != 0 {
		// The byte is -n in two's complement: 0xFF stores 1 byte, 0x80
		// stores 128.
		n := 0x100 - int(b[0])

		if len(b) < 1+n {
			return Command{}, 0, fmt.Errorf("STORE IMMEDIATE of %d bytes has only %d", n, len(b)-1)
		}

		return Command{Op: OpStoreImmediate, Data: append([]byte(nil), b[1:1+n]...)}, 1 + n, nil
	}

	op := Op(b[0])

	info, ok := ops[op]
	if !ok {
		return Command{}, 0, fmt.Errorf("unknown TIR command %d", b[0])
	}

	c := Command{Op: op}
	r := reader{b: b, pos: 1}

	switch info.format {
	case opNone:
	case opName:
		c.Name = r.counted()
	case opByte:
		c.Value = uint32(r.byte())
	case opWord:
		c.Value = uint32(r.word())
	case opLong:
		c.Value = r.long()
	case opPsectB, opPsectW, opPsectL:
		c.Psect = uint16(r.byte())
		c.Value = r.sized(info.format - opPsectB)
	case opWPsectB, opWPsectW, opWPsectL:
		c.Psect = r.word()
		c.Value = r.sized(info.format - opWPsectB)
	case opEnvName:
		c.Env = r.word()
		c.Name = r.counted()
	case opIndex:
		c.Index = r.byte()
	case opField:
		c.Pos, c.Size = r.byte(), r.byte()
	case opBytes:
		c.Data = r.bytes(int(r.byte()))
	case opCheckArg:
		// A name, the argument's index, then an argument descriptor:
		// its validation control byte, a count, and that many bytes
		// (section 7.3.4's formal argument descriptor format).
		c.Name = r.counted()
		c.Index = r.byte()
		valctl := r.byte()
		count := r.byte()
		c.Data = append([]byte{valctl, count}, r.bytes(int(count))...)
	}

	if r.err != nil {
		return Command{}, 0, fmt.Errorf("%s: %w", op, r.err)
	}

	return c, r.pos, nil
}

// encode appends the command's bytes to b.
func (c Command) encode(b []byte) ([]byte, error) {
	if c.Op == OpStoreImmediate {
		if len(c.Data) < 1 || len(c.Data) > MaxImmediate {
			return nil, fmt.Errorf("STORE IMMEDIATE of %d bytes (must be 1 to %d)", len(c.Data), MaxImmediate)
		}

		return append(append(b, byte(0x100-len(c.Data))), c.Data...), nil
	}

	info, ok := ops[c.Op]
	if !ok {
		return nil, fmt.Errorf("unknown TIR command %d", int(c.Op))
	}

	b = append(b, byte(c.Op))

	var err error

	switch info.format {
	case opNone:
	case opName:
		b, err = appendCounted(b, c.Name)
	case opByte:
		b = append(b, byte(c.Value))
	case opWord:
		b = binary.LittleEndian.AppendUint16(b, uint16(c.Value))
	case opLong:
		b = binary.LittleEndian.AppendUint32(b, c.Value)
	case opPsectB, opPsectW, opPsectL:
		if c.Psect > 0xFF {
			return nil, fmt.Errorf("%s: psect index %d needs a word-psect command", c.Op, c.Psect)
		}

		b = appendSized(append(b, byte(c.Psect)), info.format-opPsectB, c.Value)
	case opWPsectB, opWPsectW, opWPsectL:
		b = appendSized(binary.LittleEndian.AppendUint16(b, c.Psect), info.format-opWPsectB, c.Value)
	case opEnvName:
		b, err = appendCounted(binary.LittleEndian.AppendUint16(b, c.Env), c.Name)
	case opIndex:
		b = append(b, c.Index)
	case opField:
		b = append(b, c.Pos, c.Size)
	case opBytes:
		if len(c.Data) > 0xFF {
			return nil, fmt.Errorf("%s: %d data bytes (at most 255)", c.Op, len(c.Data))
		}

		b = append(append(b, byte(len(c.Data))), c.Data...)
	case opCheckArg:
		if len(c.Data) < 2 || int(c.Data[1]) != len(c.Data)-2 {
			return nil, fmt.Errorf("%s: malformed argument descriptor", c.Op)
		}

		if b, err = appendCounted(b, c.Name); err == nil {
			b = append(append(b, c.Index), c.Data...)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("%s: %w", c.Op, err)
	}

	return b, nil
}

// appendSized appends v as a byte (width 0), word (1), or longword (2).
func appendSized(b []byte, width operandFormat, v uint32) []byte {
	switch width {
	case 0:
		return append(b, byte(v))
	case 1:
		return binary.LittleEndian.AppendUint16(b, uint16(v))
	}

	return binary.LittleEndian.AppendUint32(b, v)
}

// appendCounted appends a counted ("standard name format") string: a
// length byte, then the characters.
func appendCounted(b []byte, s string) ([]byte, error) {
	if len(s) > 0xFF {
		return nil, fmt.Errorf("name %q is longer than 255 characters", s)
	}

	return append(append(b, byte(len(s))), s...), nil
}
