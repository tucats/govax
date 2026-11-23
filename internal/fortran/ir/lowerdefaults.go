package ir

import "github.com/tucats/govax/internal/fortran/sema"

// Default initialization (Fortran 95; docs/planning/PHASE-112 - Fortran
// 95.md, A.3's 4): semantic analysis lists the objects each entry point sets to
// their type's defaults (sema.Unit.Defaults), each with the assignment that
// does it, a record constant, the type's default bytes, moved into the object,
// or into each of an array's elements:
//
//	MOVC3  #24,S.$K3,S.T        TYPE(REPORT) :: T, at each entry
//
// An entry point sets only the dummy arguments its own list has, and only the
// automatic arrays whose bounds it defines; an OPTIONAL dummy only if an
// argument was given for it.

// entryDefaults sets, at the entry point whose list is params, each object the
// unit sets to its defaults there. It follows the allocatable arrays' and
// pointers' clearing (clearAllocatables), so an automatic array's elements
// exist.
func (l *lowerer) entryDefaults(params []*sema.Symbol) {
	for _, d := range l.unit.Defaults {
		s := d.Sym

		if (s.Dummy != 0 || s.AutomaticObject) && !defines(s, params) {
			continue
		}

		if d.Present == nil {
			l.assign(d.Assign)

			continue
		}

		set, next := l.newBlock(), l.newBlock()
		l.branch(l.present(d.Present), set, next)

		l.begin(set)
		set.Note = "sets " + s.Name + " to its defaults"
		l.assign(d.Assign)
		l.begin(next)
	}
}

// defaulted reports whether each entry point sets s to its defaults, which
// leaves its pointers disassociated as clearing them would.
func (l *lowerer) defaulted(s *sema.Symbol) bool {
	for _, d := range l.unit.Defaults {
		if d.Sym == s {
			return true
		}
	}

	return false
}

// allocationDefault sets what ALLOCATE made for obj to its type's defaults:
// after the call, and with STAT= only if it succeeded.
func (l *lowerer) allocationDefault(obj *sema.Allocation, status *Mem) {
	if obj.Default == nil {
		return
	}

	if status == nil {
		l.assign(obj.Default)

		return
	}

	set, next := l.newBlock(), l.newBlock()
	l.branch(&Cmp{Op: Eq, X: &Load{status}, Y: LongConst(0)}, set, next)

	l.begin(set)
	set.Note = "sets the allocation to its defaults"
	l.assign(obj.Default)
	l.begin(next)
}
