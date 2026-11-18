package ir

import (
	"slices"
	"strconv"

	"github.com/tucats/govax/internal/fortran/sema"
)

// Array-valued functions whose value's bounds are known only when the program
// runs (docs/planning/PHASE-104 - Fortran 90 dynamic arrays.md, finding 12, and
// the author's answer 3):
//
//	FUNCTION RAMP(N)
//	INTEGER N
//	REAL RAMP(N)
//
// The caller can't make the value's temporary, since only the function knows
// what its bounds mean (they may use its COMMON, its host's variables, or SIZE
// of its dummies). So the hidden first argument is the address of a class NCA
// descriptor of the caller's, its prototype block filled in and its POINTER 0,
// and the function, at entry, computes the bounds, takes the storage from the
// heap, and fills the descriptor in. The caller reads the value through it as it
// reads an allocatable array, and gives the storage back at the statement's end.
//
// In the function. The value is an automatic array (lowerautomatic.go) whose
// storage comes from FTN$GET_TEMP rather than the stack, since it must outlive
// the function's frame: its own class A descriptor cell, standing among the
// adjustable arrays', computed at entry, which every use of the value in the
// function reads. Then the entry writes the caller's descriptor from it:
//
//	MOVL    #^X0A0A0004,@4(AP)       ; length 4, DTYPE_F, CLASS_NCA
//	MOVL    RAMP.RAMP+4,R0
//	MOVL    R0,@4(AP)[...]          ; POINTER ... and so on: ARSIZE, A0,
//	                                ; S1 = 4, L1, U1 = L1 + extent - 1
//
// The value is contiguous, so a caller in MACRO may use POINTER and ARSIZE alone.
// An upper bound below the lower gives the caller an extent of 0, never less.
//
// In the caller. Each reference is a $Gn cell, a class NCA descriptor read as an
// allocatable array's is (sema.NewValueArray). Its POINTER is 0 whenever no
// statement holds a value there: a static cell's initial value, a frame cell's
// cleared at each entry point (fillEntryValues), and each statement's end giving
// the block back with FTN$DEALLOCATE, which clears POINTER. A call finding a
// block still there (one an unwind left, or an earlier pass of a loop in the
// same statement, as in PRINT *, (SUM(RAMP(I)), I = 1, 3)) gives it back first.
// A statement calls each reference once, however many times its lowering asks
// for the value (a temporary's extents, then its elements).

// valueCell makes the function's own value, whose bounds aren't constants, an
// automatic array of its own cell, with p, the hidden first argument, the
// caller's descriptor the entry fills in. The argument is named $VALUE, the
// value's own name being its cell's.
func (l *lowerer) valueCell(p *Param) {
	s := l.unit.Result
	l.valueParam = p
	p.Name = "$VALUE"

	c := l.newCell(s.Name, ResultCell, Long, s.Size())
	c.Sym = s
	l.automaticCell(s, c)
	l.cells[s] = c
}

// publishValue writes the caller's class NCA descriptor of the function's value
// s from c, s's class A descriptor, just computed (computeAutomatic): the
// prototype block, POINTER, ARSIZE, and A0 as they are, each stride the element's
// size times the extents before it, and the bounds, the upper made the lower plus
// the extent less 1, so that U - L + 1 is never below 0.
func (l *lowerer) publishValue(s *sema.Symbol, c *Cell) {
	d := &Descriptor{DType: c.Desc.DType, Class: ClassNCA, Length: c.Desc.Length, Dims: c.Desc.Dims}

	to := func(offset int) *Mem {
		return &Mem{Param: l.valueParam, Offset: offset, T: Long}
	}

	set := func(offset int, x Expr) {
		l.emit(&Assign{origin: l.origin(), Dst: to(offset), Src: x})
	}

	from := func(offset int) Expr {
		return &Load{&Mem{Cell: c, Offset: offset, T: Long}}
	}

	for _, f := range d.Fixed() {
		if f.Offset != PointerOffset {
			set(f.Offset, LongConst(int32(f.Value)))
		}
	}

	set(PointerOffset, from(PointerOffset))
	set(ArsizeOffset, from(ArsizeOffset))
	set(A0Offset, from(A0Offset))

	stride := Expr(LongConst(int32(s.Type.Size)))

	for i := range s.Dims {
		set(d.StrideOffset(i), stride)

		lower := l.lowerBound(s, c, i)
		set(d.LowerOffset(i), lower)
		set(d.LowerOffset(i)+Long.Size(), arith(Sub, Long, arith(Add, Long, lower, l.extent(s, c, i)), LongConst(1)))

		stride = arith(Mul, Long, &Load{to(d.StrideOffset(i))}, l.extent(s, c, i))
	}
}

// valueResult is the value of f, a reference to an array-valued function whose
// value's extents are known only when the program runs, as a whole array: the
// function called now, its value described in a $Gn cell, unless the statement
// has called it already.
func (l *lowerer) valueResult(f *sema.FuncRef) *sema.ArrayRef {
	temps := &l.runtime
	if ref := temps.values[f]; ref != nil {
		return ref
	}

	sym := sema.NewValueArray("$G", f.T, len(f.S))

	d := &Descriptor{DType: l.dtypeOf(f.T), Class: ClassNCA, Length: f.T.Size}
	for range f.S {
		d.Dims = append(d.Dims, Bound{Set: true})
	}

	key := descriptorKey(d)

	var c *Cell

	if free := l.freeValues[key]; len(free) > 0 {
		c = free[len(free)-1]
		l.freeValues[key] = free[:len(free)-1]
	} else {
		l.values++
		c = l.newCell("$G"+strconv.Itoa(l.values), DescriptorCell, Long, d.Size())
		c.Desc = d

		if c.Frame {
			l.entryValues.cells = append(l.entryValues.cells, c)
		}
	}

	c.Sym = sym
	l.busy = append(l.busy, c)

	if l.adjustable == nil {
		l.adjustable = map[*sema.Symbol]*Cell{}
	}

	l.adjustable[sym] = c

	l.freeValue(c)

	args, backs := l.args(f.Args)
	l.callWith(f, append([]Arg{{Ref: &Mem{Cell: c, T: Long}}}, args...), nil, backs)

	ref := sema.NewArrayRef(sym, f.Pos)

	if temps.values == nil {
		temps.values = map[*sema.FuncRef]*sema.ArrayRef{}
	}

	temps.values[f] = ref
	temps.valueCells = append(temps.valueCells, c)

	return ref
}

// freeValue gives back the block the value cell c describes, if it has one:
//
//	TSTL    $G1+4                   ; POINTER
//	BEQL    1$
//	PUSHL   #0
//	PUSHAB  $G1
//	CALLS   #2,G^FTN$DEALLOCATE     ; which clears POINTER
//	1$:
func (l *lowerer) freeValue(c *Cell) {
	free, next := l.newBlock(), l.newBlock()
	l.branch(&Cmp{Op: Ne, X: &Load{&Mem{Cell: c, Offset: PointerOffset, T: Long}}, Y: LongConst(0)}, free, next)

	l.begin(free)
	l.emit(&Call{origin: l.origin(), Routine: ftnDeallocate, Args: []Arg{{Ref: &Mem{Cell: c, T: Long}}, {Value: LongConst(0)}}})
	l.jump(next)

	l.begin(next)
}

// isValueCell reports whether c is a $Gn cell, the descriptor of an array-valued
// function's value of run-time shape (valueResult).
func isValueCell(c *Cell) bool {
	return c.Kind == DescriptorCell && c.Sym != nil && c.Sym.Allocatable && c.Sym.Name == "$G"
}

// entryFill is where, in the main entry point's code, the $Gn cells in the frame
// are cleared: a frame cell's bytes are whatever an earlier activation left, and
// a POINTER other than 0 would be given back (freeValue). The cells are made as
// the body is lowered, after the entry's code, so the clearing is put in at the
// end (fillEntryValues). ENTRY's entry points, laid out after the body, clear
// them with the allocatable arrays (clearAllocatables).
type entryFill struct {
	block *Block
	at    int
	cells []*Cell
}

// fillEntryValues puts, in the main entry point's code, a POINTER of 0 in each
// $Gn cell in the frame.
func (l *lowerer) fillEntryValues() {
	e := l.entryValues
	if len(e.cells) == 0 {
		return
	}

	var clear []Stmt

	for _, c := range e.cells {
		clear = append(clear, &Assign{Dst: &Mem{Cell: c, Offset: PointerOffset, T: Long}, Src: LongConst(0)})
	}

	e.block.Stmts = slices.Insert(e.block.Stmts, e.at, clear...)
}
