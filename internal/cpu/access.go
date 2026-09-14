package cpu

// AccessKind describes how an instruction may use one of its operands,
// matching vax.h's OP_NL/OP_RD/OP_WR/OP_MD/OP_AD/OP_VA/OP_BR/OP_IM constants.
// Each Instruction table entry carries one AccessKind per operand slot.
type AccessKind int

const (
	AccessNone      AccessKind = iota // OP_NL: operand slot unused
	AccessRead                        // OP_RD: operand is read only
	AccessWrite                       // OP_WR: operand is written only
	AccessModify                      // OP_MD: operand is read, then written in place
	AccessAddress                     // OP_AD: operand is an address, used directly (not dereferenced)
	AccessVarField                    // OP_VA: operand is a variable bitfield base address
	AccessBranch                      // OP_BR: operand is a branch displacement
	AccessImmediate                   // OP_IM: operand is an implicit immediate value
)

// ShortLiteralType says how an instruction's short-literal operands (VAX
// addressing modes 0-3) should be interpreted: as a raw integer bit pattern,
// or as an index into the short_double floating-point literal table.
// Matches vaxinstr.h's OP_TYPE_INT/OP_TYPE_FLOAT.
type ShortLiteralType int

const (
	ShortLiteralInt   ShortLiteralType = iota // OP_TYPE_INT
	ShortLiteralFloat                         // OP_TYPE_FLOAT
)
