package asm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// rop is the kind of an rexpr node.
type rop byte

const (
	// rConst is the constant v.
	rConst rop = iota
	// rBase is the base address of the relocatable section sect, plus v.
	// A label in a relocatable section has this value.
	rBase
	// rSym is the value of a symbol that wasn't defined when the
	// expression was read: key names it in the symbol table, and sym is
	// the symbol once queueFixup has created it. A symbol still undefined
	// when assembly finishes is external, and the linker supplies it.
	rSym
	// rNeg is -l.
	rNeg
	// rCom is ^C l, the one's complement.
	rCom
	// rBinary is l op r, op being one of MACRO-32's binary operators
	// (see exprTop).
	rBinary
	// rMask is the register save mask of the entry point key, which the
	// linker supplies (.MASK; the object language's STA_EPM).
	rMask
)

// rexpr is a value the assembler can't finish when it reads it: one that
// uses a symbol not yet defined (forward or external), or the base of a
// relocatable section, whose address only the linker knows. Its shape is
// the source expression's, because real MACRO-32 hands such expressions to
// the linker as written: EXT+4 is STA_GBL EXT, STA_UB 4, OPR_ADD, and
// <C-A>*2 is a subtraction and then a multiplication, even though C and A
// are in the same psect (docs/PHASE-27.md, subtask 3's log). The one
// simplification is the manual's (§3.5): the difference of two symbols
// already defined in the same psect is absolute (see binaryVal).
type rexpr struct {
	op   rop
	bin  byte // rBinary's operator
	v    uint32
	sect *section
	// dot marks an rBase that is ".", which real MACRO pushes with the
	// longword form STA_PL, where a label gets the shortest form.
	dot  bool
	key  string
	sym  *symbol
	l, r *rexpr
}

func constNode(v uint32) *rexpr { return &rexpr{op: rConst, v: v} }

func baseNode(s *section, offset uint32) *rexpr {
	return &rexpr{op: rBase, sect: s, v: offset}
}

// hasSymbols reports whether t uses a symbol that wasn't yet defined.
func (t *rexpr) hasSymbols() bool {
	switch t.op {
	case rSym:
		return true
	case rNeg, rCom:
		return t.l.hasSymbols()
	case rBinary:
		return t.l.hasSymbols() || t.r.hasSymbols()
	}

	return false
}

// leaves calls fn for every rSym leaf in t.
func (t *rexpr) leaves(fn func(*rexpr)) {
	switch t.op {
	case rSym:
		fn(t)
	case rNeg, rCom:
		t.l.leaves(fn)
	case rBinary:
		t.l.leaves(fn)
		t.r.leaves(fn)
	}
}

// placeholder evaluates t with every undefined symbol, and every section
// base, taken as zero. It's what's stored where t's value goes until the
// value is known. For a sum of symbols and a constant it's the constant.
func (t *rexpr) placeholder() uint32 {
	switch t.op {
	case rConst, rBase:
		return t.v
	case rNeg:
		return -t.l.placeholder()
	case rCom:
		return ^t.l.placeholder()
	case rBinary:
		return applyOp(t.bin, t.l.placeholder(), t.r.placeholder())
	}

	return 0
}

// resolved returns t with every symbol defined since it was read replaced
// by its value. If that leaves no symbol or section base, it's the
// constant t's value; otherwise its shape is kept as written, operations
// on constants included, as real MACRO hands it to the linker (see
// exprVal).
func (t *rexpr) resolved() *rexpr {
	s := t.substituted()
	if s.isConstant() && !s.dividesByZero() {
		return constNode(s.placeholder())
	}

	return s
}

// substituted returns t with every symbol defined since it was read
// replaced by its value, and nothing folded.
func (t *rexpr) substituted() *rexpr {
	switch t.op {
	case rSym:
		if t.sym == nil || !t.sym.defined() {
			return t
		}

		if t.sym.sect != nil {
			return baseNode(t.sym.sect, t.sym.value)
		}

		return constNode(t.sym.value)

	case rNeg, rCom:
		return &rexpr{op: t.op, l: t.l.substituted()}

	case rBinary:
		return &rexpr{op: rBinary, bin: t.bin, l: t.l.substituted(), r: t.r.substituted()}
	}

	return t
}

// isConstant reports whether t uses no symbol, section base, or entry
// mask: whether the assembler can compute it.
func (t *rexpr) isConstant() bool {
	switch t.op {
	case rConst:
		return true
	case rNeg, rCom:
		return t.l.isConstant()
	case rBinary:
		return t.l.isConstant() && t.r.isConstant()
	}

	return false
}

// dividesByZero reports whether t, a constant tree, divides by zero
// anywhere, which is left for the linker to report.
func (t *rexpr) dividesByZero() bool {
	switch t.op {
	case rNeg, rCom:
		return t.l.dividesByZero()
	case rBinary:
		return (t.bin == '/' && t.r.placeholder() == 0) || t.l.dividesByZero() || t.r.dividesByZero()
	}

	return false
}

// simpleRelocatable reports whether t is a section base plus a constant
// (A, A+4, A-4, or 4+A), the form a symbol's own value can take, and if
// so which section and offset.
func (t *rexpr) simpleRelocatable() (*section, uint32, bool) {
	switch {
	case t.op == rBase:
		return t.sect, t.v, true

	case t.op != rBinary:
		return nil, 0, false

	case t.bin == '+' && t.l.op == rBase && t.r.isConstant():
		return t.l.sect, t.l.v + t.r.placeholder(), true

	case t.bin == '+' && t.l.isConstant() && t.r.op == rBase:
		return t.r.sect, t.l.placeholder() + t.r.v, true

	case t.bin == '-' && t.l.op == rBase && t.r.isConstant():
		return t.l.sect, t.l.v - t.r.placeholder(), true
	}

	return nil, 0, false
}

// String writes t in postfix order, the order the linker's stack machine
// evaluates it in: "EXT1 4 +", "DATA:0 OTHER:0 -". A section base is
// section:offset (hexadecimal), and a constant is decimal.
func (t *rexpr) String() string {
	switch t.op {
	case rConst:
		return fmt.Sprintf("%d", int32(t.v))
	case rBase:
		return fmt.Sprintf("%s:%X", t.sect.name, t.v)
	case rSym:
		return t.key
	case rNeg:
		return t.l.String() + " NEG"
	case rCom:
		return t.l.String() + " COM"
	case rMask:
		return "MASK(" + t.key + ")"
	}

	return t.l.String() + " " + t.r.String() + " " + string(t.bin)
}

// applyOp applies a binary operator to two known values. Division is
// unsigned, as the reference tool's was. A shift count is signed:
// positive shifts left, negative shifts right arithmetically.
func applyOp(op byte, v1, v2 uint32) uint32 {
	switch op {
	case '+':
		return v1 + v2

	case '-':
		return v1 - v2

	case '*':
		return v1 * v2

	case '/':
		// A division by zero is the linker's to report; until then,
		// real MACRO's listing shows the dividend (10/0 is 0000000A',
		// errors.lis).
		if v2 == 0 {
			return v1
		}

		return v1 / v2

	case '@':
		n := int32(v2)

		switch {
		case n >= 32:
			return 0
		case n >= 0:
			return v1 << uint(n)
		case n <= -32:
			return uint32(int32(v1) >> 31)
		default:
			return uint32(int32(v1) >> uint(-n))
		}

	case '&':
		return v1 & v2

	case '!':
		return v1 | v2
	}

	return v1 ^ v2
}

// relocation is a value the assembler left for the linker: at offset in
// sect, store expr, which uses a relocatable section's base or an external
// symbol, in the way kind says (a byte, word, or longword; a displacement
// from the end of the field; or a position-independent address). The
// object emitter turns each one into TIR stack commands.
type relocation struct {
	sect   *section
	offset uint32
	kind   fixupKind
	expr   *rexpr
	// prefix is how many bytes of the operand specifier come before the
	// field (see fixup), which real MACRO stores after the value's stack
	// program.
	prefix int
	// stmt is the statement that stored the field, and superseded says a
	// later statement stored over it (see overwrite.go).
	stmt       int
	superseded bool
}

func (r relocation) String() string {
	return fmt.Sprintf("%s+%X %s %s", r.sect.name, r.offset, fixupKindNames[r.kind], r.expr)
}

var fixupKindNames = map[fixupKind]string{
	fixAddrB:   "B",
	fixAddrW:   "W",
	fixAddrL:   "L",
	fixDispB:   "BD",
	fixDispW:   "WD",
	fixDispL:   "LD",
	fixBranchB: "BD",
	fixBranchW: "WD",
	fixBranchL: "LD",
	fixCaseW:   "CASE",
	fixAddress: "PIDR",
	fixPICR:    "PICR",
	fixSignedB: "SB",
	fixSignedW: "SW",
}

// Relocations returns the relocations assembled so far, one per line, for
// tests and diagnostics: "DATA+4 L EXT1 4 +".
func (a *Assembler) Relocations() []string {
	out := make([]string, 0, len(a.relocs))

	for _, r := range a.relocs {
		if !r.superseded {
			out = append(out, r.String())
		}
	}

	return out
}

// isDisplacement reports whether kind stores a displacement from the end
// of its field rather than a value.
func isDisplacement(kind fixupKind) bool {
	switch kind {
	case fixDispB, fixDispW, fixDispL, fixBranchB, fixBranchW, fixBranchL:
		return true
	}

	return false
}

// isAddrFixup reports whether kind stores a value in a byte, word, or
// longword. In the MACRO dialect, one that waited for a symbol defined
// later is left to the linker even when its value turns out absolute, as
// real MACRO leaves $RAB's USZ=BUFSIZ, BUFSIZ assigned after it
// (testdata/mar/macros/vax/rmscopy.obj: STA_UW 0x200, STO_W).
func isAddrFixup(kind fixupKind) bool {
	return kind == fixAddrB || kind == fixAddrW || kind == fixAddrL
}

// isBranch reports whether kind is a branch instruction's displacement.
func isBranch(kind fixupKind) bool {
	return kind == fixBranchB || kind == fixBranchW || kind == fixBranchL
}

// completeFixup finishes f once none of its symbols is still pending:
// its value, if that's now a constant, is stored as it always was;
// otherwise it becomes a relocation. Two cases follow real MACRO's
// objects (docs/PHASE-27.md, subtask 3's log):
//   - A branch to a location in its own psect is finished here, whatever
//     the psect's base, but an operand's displacement to a label defined
//     after it is left to the linker (STO_LD).
//   - A displacement from a relocatable psect to an absolute address
//     depends on where the psect goes, so the linker finishes it too.
func (a *Assembler) completeFixup(f *fixup) error {
	t := f.expr.resolved()

	switch {
	case f.dead && t.op == rConst && f.kind != fixPICR && f.kind != fixAddress:
		// A later statement stored over the field (see overwrite.go).
		return nil

	case f.dead:
		// Its relocation is still written where the field was stored,
		// as real MACRO writes it; the later store wins.
		a.relocs = append(a.relocs, relocation{sect: f.sect, offset: f.location, kind: f.kind, expr: t, prefix: f.prefix, stmt: f.stmt, superseded: true})

		return nil

	case t.op == rConst && f.kind != fixPICR && f.kind != fixAddress && !(isDisplacement(f.kind) && f.sect.relocatable) &&
		!(a.dialect == DialectMACRO && isAddrFixup(f.kind)):
		return a.applyFixup(f, t.v)

	case t.op == rBase && t.sect == f.sect && isBranch(f.kind):
		return a.applyFixup(f, t.v)
	}

	a.relocs = append(a.relocs, relocation{sect: f.sect, offset: f.location, kind: f.kind, expr: t, prefix: f.prefix, stmt: f.stmt})

	// The linker writes the field, so it holds zeros, not the placeholder.
	for i := uint32(0); i < uint32(fixupSize(f.kind)); i++ {
		if err := f.sect.img.storeByte(f.location+i, 0); err != nil {
			return err
		}
	}

	return nil
}

// flushReady completes the fixups queued with nothing pending (a value
// using a section base, but no undefined symbol). They wait until the end
// of their statement, since an operand parser may still move or change a
// fixup it just queued (see lastFixup).
func (a *Assembler) flushReady() error {
	ready := a.ready
	a.ready = nil

	for _, f := range ready {
		if err := a.completeFixup(f); err != nil {
			return err
		}
	}

	return nil
}

// finish ends a MACRO-dialect assembly. Every symbol still undefined is
// external, so every fixup waiting on one becomes a relocation for the
// linker to finish. That's any symbol .GLOBAL, .EXTERNAL, .WEAK, or .MASK
// named, and, while .ENABLE GLOBAL is in effect (the default), any other
// symbol too. With GLOBAL disabled, an undefined symbol not declared
// external is an error.
func (a *Assembler) finish() error {
	if err := a.flushReady(); err != nil {
		return err
	}

	// A local label can't be external (.END checks this too, but a source
	// may have no .END).
	if err := a.closeLocalBlock(); err != nil {
		return err
	}

	waiting := map[*fixup]bool{}

	var undeclared []string

	for name, s := range a.symbols.byName {
		if len(s.forward) == 0 && s.flags&SymUndefined == 0 {
			continue
		}

		if s.flags&SymGlobal == 0 && a.enabled&enableGlobal == 0 {
			undeclared = append(undeclared, name)

			continue
		}

		for _, f := range s.forward {
			waiting[f] = true
		}

		s.forward = nil
		s.flags = s.flags&^SymUndefined | SymExternal
	}

	if len(undeclared) > 0 {
		sort.Strings(undeclared)

		return vmserrors.New(vmserrors.VAX_UNDEFSYM, undeclared[0])
	}

	fixups := make([]*fixup, 0, len(waiting))
	for f := range waiting {
		fixups = append(fixups, f)
	}

	for _, f := range fixups {
		if err := a.completeFixup(f); err != nil {
			return err
		}
	}

	sort.SliceStable(a.relocs, func(i, j int) bool {
		ri, rj := a.relocs[i], a.relocs[j]
		if ri.sect.index != rj.sect.index {
			return ri.sect.index < rj.sect.index
		}

		return ri.offset < rj.offset
	})

	return nil
}

// externals returns the names of the symbols finish found undefined.
func (a *Assembler) externals() []string {
	var out []string

	for name, s := range a.symbols.byName {
		if s.flags&SymExternal != 0 {
			out = append(out, name)
		}
	}

	sort.Strings(out)

	return out
}

// Externals reports the symbols a MACRO-dialect assembly left for the
// linker, for tests and diagnostics, as one comma-separated list.
func (a *Assembler) Externals() string { return strings.Join(a.externals(), ",") }
