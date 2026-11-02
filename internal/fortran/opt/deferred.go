package opt

import "github.com/tucats/govax/internal/fortran/ir"

// Deferred indexes: an adjustable array's element without its address in a register.
//
//	      SUBROUTINE S(A, N)
//	      REAL A(N)
//	      ...
//	      T = A(J)
//
// A0, where the array's element of subscript 0 would be, is in a descriptor the entry
// point fills in, so the element's address isn't known until the routine runs. Lowering
// says that plainly, and the code generator writes it plainly:
//
//	      MOVL    S.$B1+16,R0
//	      MOVL    S.J,R1
//	      MOVF    (R0)[R1],S.T
//
// The VAX can look through a longword in memory and index what it finds, all in one
// operand:
//
//	      MOVL    S.J,R1
//	      MOVF    @S.$B1+16[R1],S.T
//
// That is one instruction fewer for every reference, and the same memory reads. The mode
// has no room for an offset, so A(J+1), at 4(R0)[R1], becomes ADDL3 #1,S.J,R1 and
// @S.$B1+16[R1], which is still one fewer.
//
// This pass only marks the places (Mem.Deferred). The code generator decides where the
// address is such a longword (codegen's deferredIndex): a packed field's isn't.

// DeferredIndexes is the pass.
func DeferredIndexes(r *ir.Routine) {
	walkMems(r, func(m *ir.Mem) {
		if m.Ptr != nil && m.Index != nil && m.Offset%m.T.Size() == 0 {
			m.Deferred = true
		}
	})
}
