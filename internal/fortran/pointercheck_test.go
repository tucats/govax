package fortran_test

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran"
	"github.com/tucats/govax/internal/fortran/sema"
)

// /CHECK=POINTERS in the MACRO file (docs/planning/PHASE-110 - Fortran 90
// compiler usability.md, subtask 6): each use of a pointer's target, an
// allocatable array's element, the whole array, or its bounds tests the address
// first, and calls FTNRTL's routine if it is 0; nothing else does, and nothing
// at all without the check. internal/console's TestFortranCheckPointers runs
// such programs.

const pointerCheck = `program pc
  integer, pointer :: p
  integer, target :: n
  real, allocatable, target :: a(:)
  real, pointer :: q(:)
  real s
  p => n
  nullify (p)
  p => n
  if (associated (p)) p = 5
  allocate (a(3))
  q => a
  s = a(2) + q(1) + size(a)
  deallocate (a)
end program
`

func TestPointerCheckCode(t *testing.T) {
	for _, optimize := range []bool{false, true} {
		for _, checked := range []bool{false, true} {
			r := compileFree("PC", pointerCheck, nil, func(o *fortran.Options) {
				o.Optimize = optimize
				o.Checks = sema.Checks{Pointers: checked}
			})
			if r.Errors > 0 {
				t.Fatal(messageList(r))
			}

			text := strings.Join(r.MACRO, "\n")
			code := text[strings.Index(text, ".ENTRY"):]

			want := map[string]int{"G^FTN$NOTASSOC": 0, "G^FTN$UNALLOCATED": 0}
			if checked {
				// P = 5 and Q(1); Q => A (a pointer's allocatable target
				// must be allocated, the standard's 7.5.2), A(2), and SIZE(A).
				want = map[string]int{"G^FTN$NOTASSOC": 2, "G^FTN$UNALLOCATED": 3}
			}

			for routine, n := range want {
				if got := strings.Count(code, routine); got != n {
					t.Errorf("optimize %v, checked %v: %d calls of %s, want %d:\n%s", optimize, checked, got, routine, n, code)
				}
			}
		}
	}
}

func TestPointerCheckKeyword(t *testing.T) {
	for _, c := range []struct {
		words []string
		want  bool
	}{
		{nil, true},
		{[]string{"ALL"}, true},
		{[]string{"POINTERS"}, true},
		{[]string{"POI"}, true},
		{[]string{"ALL", "NOPOINTERS"}, false},
		{[]string{"BOUNDS"}, false},
		{[]string{"NONE"}, false},
	} {
		got, err := sema.Checks{}.With(c.words)
		if err != nil || got.Pointers != c.want {
			t.Errorf("%v: %+v, %v; want Pointers %v", c.words, got, err, c.want)
		}
	}
}
