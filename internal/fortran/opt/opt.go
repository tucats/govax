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

// Passes are the optimizer's passes, in the order they run. Each subtask of
// docs/PHASE-84 - FORTRAN optimizer.md adds to this list.
var Passes = []Pass{
	{"forward", Forward},
	{"unreachable", Unreachable},
	{"dead code", DeadCode},
	{"common subexpressions", CommonSubexpressions},
}

// CodePass is one optimization of a routine's instructions, after the code
// generator has chosen them and before they are written: its name, for the same
// purposes, and the function that improves them.
type CodePass struct {
	Name string
	Run  func(*codegen.Code)
}

// CodePasses are the instruction passes, in the order they run.
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
