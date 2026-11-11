package opt

import (
	"fmt"

	"github.com/tucats/govax/internal/fortran/codegen"
	"github.com/tucats/govax/internal/fortran/ir"
)

// Pass is one optimization: its name, for the MACRO file's header and for a
// message if it breaks a routine, and the function that improves a routine.
type Pass struct {
	Name string
	Run  func(*ir.Routine)
}

// Passes are the optimizer's passes, in the order they run (docs/PHASE-84 -
// FORTRAN optimizer.md). Each pass's file opens with a FORTRAN example and
// the code before and after; in a line each:
//
//   - do limits (dolimit.go): a DO loop ended by comparing its variable with
//     the limit (AOBLEQ) rather than by counting trips down as well;
//   - one character (onechar.go): a one-character string's move or
//     comparison made a byte's, MOVB or CMPB, not MOVC3 or CMPC3;
//   - forward (forward.go): constants carried from where they're stored to
//     where they're used, even across blocks, and branches whose outcome
//     that settles made jumps;
//   - unreachable (unreachable.go): blocks nothing can reach taken out;
//   - dead code (deadcode.go): stores whose value nothing reads taken out;
//   - deferred indexes (deferred.go): an adjustable array's element reached
//     through its address in memory, @X[R1], without loading it first;
//   - loop invariants (invariant.go): what every trip of a loop computes the
//     same moved to before the loop;
//   - common subexpressions (cse.go): a value a block computes twice computed
//     once;
//   - registers (registers.go): variables and the compiler's cells kept in
//     R6 to R11 rather than memory;
//   - rotate (rotate.go): a loop's test moved from its top to its bottom, so
//     each trip takes one branch, not two.
//
// The order matters: earlier passes simplify what later ones look at (do
// limits must see lowering's loops as written; registers comes after the
// passes that recognize places by their form in memory, which a place
// moved to a register no longer has).
var Passes = []Pass{
	{"do limits", DoLimits},
	{"one character", OneCharacter},
	{"forward", Forward},
	{"unreachable", Unreachable},
	{"dead code", DeadCode},
	{"deferred indexes", DeferredIndexes},
	{"loop invariants", LoopInvariants},
	{"common subexpressions", CommonSubexpressions},
	{"registers", Registers},
	{"rotate", Rotate},
}

// CodePass is one optimization of a routine's instructions, after the code
// generator has chosen them and before they are written: its name, for the same
// purposes, and the function that improves them.
type CodePass struct {
	Name string
	Run  func(*codegen.Code)
}

// CodePasses are the instruction passes, in the order they run: peephole
// (peephole.go), single instructions made shorter (MOVL #0,R0 is CLRL R0);
// and branches (branches.go), short branch forms where the target is near,
// jumps to jumps threaded, and loops closed with SOBGTR, AOBLEQ, or ACBL.
var CodePasses = []CodePass{
	{"peephole", Peephole},
	{"branches", Branches},
}

// Improve runs every instruction pass on a routine's code: what the code generator
// is given to call under /OPTIMIZE.
func Improve(c *codegen.Code) {
	improve(c, CodePasses)
}

// improve runs the given passes on a routine's code, checking it after each, as
// run does a routine's IR.
func improve(c *codegen.Code, passes []CodePass) {
	for _, p := range passes {
		p.Run(c)

		if err := c.Check(); err != nil {
			panic(fmt.Sprintf("opt: after %s: %v", p.Name, err))
		}
	}
}

// Optimize runs every pass on each routine.
func Optimize(routines []*ir.Routine) {
	run(routines, Passes)
}

// run runs the given passes on each routine, checking the routine first and
// after each pass. A routine that fails the check is a mistake of the compiler's
// and can't be compiled, so that is a panic, as it is in the code generator: a
// message naming the pass is better than a program that silently goes wrong.
func run(routines []*ir.Routine, passes []Pass) {
	for _, r := range routines {
		check(r, "lowering")

		for _, p := range passes {
			p.Run(r)
			check(r, p.Name)
		}
	}
}

// check panics if routine r isn't correct after the step named by.
func check(r *ir.Routine, by string) {
	if err := ir.Check(r); err != nil {
		panic(fmt.Sprintf("opt: %s: after %s: %v", r.Name, by, err))
	}
}

// Names are the names of the passes that run, in order.
func Names() []string {
	names := make([]string, 0, len(Passes)+len(CodePasses))

	for _, p := range Passes {
		names = append(names, p.Name)
	}

	for _, p := range CodePasses {
		names = append(names, p.Name)
	}

	return names
}
