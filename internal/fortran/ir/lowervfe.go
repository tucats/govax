package ir

import (
	"slices"
	"strconv"

	"github.com/tucats/govax/internal/fortran/sema"
)

// Variable format expressions (section 8.3.2 of the language manual):
//
//	100 FORMAT (I<MAX(I,5)>)
//
// The width isn't known until the format is scanned, and is computed again
// each time the scan reaches it, so it is code, which FTNRTL has to call. Each
// FORMAT with such expressions gets one routine of a form that takes the
// expression's number, n, and the AP and FP of the routine doing the I/O, and
// returns the value in R0 (Decision 5 of docs/PHASE-80 - FORTRAN formatted IO.md,
// and Decision 6 of docs/PHASE-89 - reentrant FORTRAN.md):
//
//	FMT.$V100:
//	  .WORD   ^M<R2>
//	  MOVL    4(AP),R0                which expression
//	  MOVL    12(AP),R11              the routine's own FP, if it has a frame
//	  MOVL    8(AP),AP                the routine's own AP
//	  CASEL   R0,#1,#0
//	  ...                             the expression's code
//	  RET
//
// It needs that AP because a dummy argument is reached through it (@N(AP)), and
// a dummy argument may be in the expression. RET puts FTNRTL's back. It needs
// that FP because a frame cell, an AUTOMATIC variable or anything of a RECURSIVE
// unit's, is a displacement from it, and it may be any activation's: only FTNRTL,
// which noted it when the statement began, can say which. FP itself can't be made
// that routine's as AP is: FP is what RET finds its frame by, and what a signal's
// search starts from. So the code generator writes a frame cell in these blocks
// as a displacement from R11 (routineGen.frameBase), and the IR is as it is in any
// other block: to the optimizer a frame cell is one place wherever it is read.
//
// The code runs in the middle of an I/O statement, between its calls. So it
// may not share a temporary with any statement, or the I/O statement's implied
// DO loops and descriptors would change under it; layOutVFEs gives the
// expressions temporaries of their own. They may share those among themselves,
// since an activation is in one statement at a time, and the only way for a
// second FORMAT to be scanned during an expression is a function's I/O, which
// uses its own: another unit's temporaries, or in a RECURSIVE unit another
// activation's, since its temporaries are in its frame, and an expression's are
// in the frame of the activation it computes for. In a unit that isn't RECURSIVE
// they are static, as all its compiler's cells are.

// ExpressionBlocks are the blocks of routine r's variable format expressions, which
// run when FTNRTL calls them, in the middle of an I/O statement.
func ExpressionBlocks(r *Routine) map[*Block]bool {
	blocks := map[*Block]bool{}

	var mark func(*Block)

	mark = func(b *Block) {
		if !blocks[b] {
			blocks[b] = true

			for _, next := range Successors(b.End) {
				mark(next)
			}
		}
	}

	for _, v := range r.VFEs {
		for _, b := range v.Cases {
			mark(b)
		}
	}

	return blocks
}

// Successors are the blocks a terminator names, each once, in the order it names
// them. An assigned GO TO can also go elsewhere, which only the optimizer's Graph
// knows.
func Successors(t Terminator) []*Block {
	var list []*Block

	switch t := t.(type) {
	case *Jump:
		list = []*Block{t.To}
	case *Branch:
		list = []*Block{t.True, t.False}
	case *Switch:
		list = append(append(list, t.Targets...), t.Else)
	case *Sign:
		list = []*Block{t.Neg, t.Zero, t.Pos}
	case *IndirectJump:
		list = t.Targets
	}

	var once []*Block

	for _, b := range list {
		if !slices.Contains(once, b) {
			once = append(once, b)
		}
	}

	return once
}

// formatVFE is a FORMAT with expressions and its routine.
type formatVFE struct {
	format  *sema.Format
	routine *VFERoutine
}

// vfeRoutine is FORMAT f's routine, made at its block's first use; nil if it
// has no expressions.
func (l *lowerer) vfeRoutine(f *sema.Format) *VFERoutine {
	if len(f.VFEs) == 0 {
		return nil
	}

	v := &VFERoutine{Name: "$V" + strconv.Itoa(f.Label), Label: f.Label}
	l.vfes = append(l.vfes, formatVFE{f, v})
	l.r.VFEs = append(l.r.VFEs, v)

	return v
}

// layOutVFEs lays out, after everything else in the routine, each expression's
// blocks.
func (l *lowerer) layOutVFEs() {
	if len(l.vfes) == 0 {
		return
	}

	l.free = map[Type][]*Cell{}
	l.freeDesc = map[string][]*Cell{}
	l.freeText = map[int][]*Cell{}
	l.inVFE = true

	for _, f := range l.vfes {
		l.from = f.format.AST

		for i, e := range f.format.VFEs {
			b := l.newBlock()
			b.Note = "FORMAT " + strconv.Itoa(f.format.Label) + "'s expression " + strconv.Itoa(i+1)
			l.begin(b)
			f.routine.Cases = append(f.routine.Cases, b)

			value := l.expr(e)

			// What the expression must do last (free a concatenation's
			// dynamic string) comes after its value is computed.
			if len(l.after) > 0 {
				t := l.temp(Long)
				l.emit(&Assign{origin: l.origin(), Dst: t, Src: value})
				l.flush()
				value = &Load{t}
			}

			l.terminate(&Return{origin: l.origin(), Value: value, VFE: true})
			l.release()
		}
	}
}
