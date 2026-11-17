package ir

import "github.com/tucats/govax/internal/fortran/sema"

// Fortran 90's EXIT and CYCLE (the standard's 8.1.4.4): jumps out of a DO
// loop, or on to its next pass. Semantic analysis has already said which
// loop each belongs to (sema.LoopExit's Loop); here each loop being lowered
// keeps the two blocks they go to, on a stack as the loops nest:
//
//   - exit, the block after the loop, where the loop goes when its count
//     runs out or its test fails;
//   - next, where the next pass starts: a DO's step code (the variable
//     stepped, the count decreased, the test), a DO WHILE's test, a DO
//     with no loop control's first block.
//
// Neither is anything new to the code after it: a GO TO to the statement
// after a loop, or to a labeled END DO, makes the same jumps, and the
// optimizer sees them as it sees those.
//
// A DO's step code follows its body's in the same block, so a CYCLE makes it
// a block of its own (pending), laid out when the body is done. A loop
// without CYCLE is lowered as before, block for block. If the loop ends at
// a labeled END DO, that statement's block is already the next pass's (a
// jump to END DO goes on to the next pass), and CYCLE goes there.

// loopTargets are where EXIT and CYCLE of one loop go.
type loopTargets struct {
	loop sema.Stmt
	exit *Block
	next *Block

	// pending is set when next was made for a CYCLE and still has to be
	// laid out (endPass).
	pending bool

	// endDo is the END DO's label, 0 for none, whose block a counted
	// loop's CYCLE goes to.
	endDo int
}

// openLoop starts lowering the body of loop, whose EXIT goes to exit and
// CYCLE to next (nil until a CYCLE needs one, for a counted DO).
func (l *lowerer) openLoop(loop sema.Stmt, exit, next *Block) *loopTargets {
	t := &loopTargets{loop: loop, exit: exit, next: next}

	if d, ok := loop.(*sema.Do); ok && d.EndDo != nil {
		t.endDo = d.EndDo.Base().Label
	}

	l.loopStack = append(l.loopStack, t)

	return t
}

// endPass ends the innermost loop's body: the block a CYCLE made for the
// next pass is laid out here, so the step code that follows goes in it.
func (l *lowerer) endPass() {
	t := l.loopStack[len(l.loopStack)-1]
	l.loopStack = l.loopStack[:len(l.loopStack)-1]

	if t.pending {
		l.begin(t.next)
	}
}

// loopTarget is where the EXIT or CYCLE s goes.
func (l *lowerer) loopTarget(s *sema.LoopExit) *Block {
	var t *loopTargets

	for i := len(l.loopStack) - 1; i >= 0 && t == nil; i-- {
		if l.loopStack[i].loop == s.Loop {
			t = l.loopStack[i]
		}
	}

	if !s.Cycle {
		return t.exit
	}

	if t.next == nil {
		if t.endDo != 0 {
			t.next = l.labelBlock(t.endDo)
		} else {
			t.next = l.newBlock()
			t.pending = true
		}
	}

	return t.next
}
