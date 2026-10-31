package ir

import (
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
// expression's number, n, and the AP of the routine doing the I/O, and returns
// the value in R0 (Decision 5 of docs/PHASE-80 - FORTRAN formatted IO.md):
//
//	FMT.$V100:
//	  .WORD   ^M<R2>
//	  MOVL    4(AP),R0                which expression
//	  MOVL    8(AP),AP                the routine's own AP
//	  CASEL   R0,#1,#0
//	  ...                             the expression's code
//	  RET
//
// It needs that AP because a dummy argument is reached through it (@N(AP)), and
// a dummy argument may be in the expression. RET puts FTNRTL's back. Nothing
// else about the routine's own frame matters to compiled code: its variables,
// temporaries, and spill cells are all static.
//
// The code runs in the middle of an I/O statement, between its calls. So it
// may not share a temporary with any statement, or the I/O statement's implied
// DO loops and descriptors would change under it; layOutVFEs gives the
// expressions temporaries of their own. They may share those among themselves,
// since a routine has only one call active (section 6.2), and the only way for
// a second FORMAT to be scanned during an expression is a function's I/O, which
// uses its own.

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
				l.emit(&Assign{l.origin(), t, value})
				l.flush()
				value = &Load{t}
			}

			l.terminate(&Return{origin: l.origin(), Value: value, VFE: true})
			l.release()
		}
	}
}
