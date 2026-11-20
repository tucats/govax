package fortran_test

import (
	"testing"
)

// The listings of Fortran 90's programs (docs/planning/PHASE-110 - Fortran 90
// compiler usability.md, subtask 5, finding 6): generic names, operators, and
// assignment as rows of their own, in the module that defines them and in each
// unit using them; a module procedure defined in its module's cross reference;
// attributes in the map and the cross reference, and a descriptor named as one;
// no compiler temporaries in the map; and an internal or module procedure's host
// names in its cross reference.

// listing90 is a module with a generic name, an operator, and assignment, and a
// program using them, with an internal procedure.
const listing90 = `module shapes
  type point
    real :: x, y
  end type point
  interface operator(.plus.)
    module procedure addp
  end interface
  interface assignment(=)
    module procedure setp
  end interface
  interface norm
    module procedure normp, normr
  end interface
  integer, private :: hidden = 3
contains
  function addp(a, b)
    type(point), intent(in) :: a, b
    type(point) :: addp
    addp%x = a%x + b%x
    addp%y = a%y + b%y
  end function addp
  subroutine setp(p, r)
    type(point), intent(out) :: p
    real, intent(in) :: r
    p%x = r
    p%y = r + hidden
  end subroutine setp
  real function normp(p)
    type(point), intent(in) :: p
    normp = sqrt(p%x**2 + p%y**2)
  end function normp
  real function normr(r)
    real, intent(in) :: r
    normr = abs(r)
  end function normr
  real function dist(p, q)
    type(point), intent(in) :: p, q
    dist = normp(p .plus. q)
  end function dist
end module shapes
program probe
  use shapes
  type(point) :: a, b, c
  real, allocatable :: v(:)
  real, target :: t(4)
  real, pointer :: pt(:)
  integer :: line, i
  allocate (v(4))
  v = 1.0
  t = 2.0
  pt => t
  where (v > 0.5)
    v = 2.0
    t = 1.0
  end where
  a = 1.0
  b = 2.0
  c = a .plus. b
  print *, norm(c), dist(a, b), norm(3.0), pt(1)
  call show(v)
contains
  subroutine show(w)
    real, intent(in), optional :: w(:)
    do i = 1, size(w)
      line = line + 1
    end do
    print *, w, line
  end subroutine show
end program probe`

func TestListing90(t *testing.T) {
	r := compileFree("PROBE", listing90, nil, withListing)
	if r.Errors > 0 {
		t.Fatal(messageList(r))
	}

	listingHas(t, r.Listing,
		// The module: its generic name, operator, and assignment defined by
		// their INTERFACE statements; each module procedure by its own
		// statement; the operator's use in DIST counted here too; a private
		// name said to be.
		"  HIDDEN  INTEGER, PRIVATE  $MOD$SHAPES+0  SHAPES.HIDDEN",
		"  HIDDEN  INTEGER, PRIVATE  14D  26",
		"  ADDP  TYPE(POINT) function  6  16D",
		"  ASSIGNMENT(=)  assignment  8D",
		"  DIST  REAL function  36D",
		"  NORM  generic  11D",
		"  NORMP  REAL function  12  28D  38",
		"  NORMR  REAL function  12  32D",
		"  OPERATOR(.PLUS.)  operator  5D  38",
		"  SETP  subroutine  9  22D",

		// A module procedure: its dummies' INTENT, and its host's names.
		"  P  TYPE(POINT), INTENT(OUT)  argument 1  @SHAPES.SETP.P(AP)",
		"  HIDDEN  INTEGER, PRIVATE from SHAPES  26",
		"  NORMP  REAL function from SHAPES  38",
		"  OPERATOR(.PLUS.)  operator from SHAPES  38",

		// The program: attributes, descriptors, and what a USE gave it.
		"  V  REAL, ALLOCATABLE (:)  descriptor at $LOCAL+24  PROBE.V",
		"  T  REAL, TARGET (4)  $LOCAL+56  PROBE.T",
		"  PT  REAL, POINTER (:)  descriptor at $LOCAL+72  PROBE.PT",
		"  PT  REAL array, POINTER  46D  51A  59",
		"  T  REAL array, TARGET  45D  50A  51  54A",
		"  V  REAL array, ALLOCATABLE  44D  48A  49A  52  53A  60",
		"  ASSIGNMENT(=)  assignment from SHAPES  42D  56  57",
		"  NORM  generic from SHAPES  42D  59",
		"  OPERATOR(.PLUS.)  operator from SHAPES  42D  58",
		"  DIST  REAL function from SHAPES  42D  59",
		"  SHOW  subroutine  60  62D",

		// The internal procedure: OPTIONAL, and the host's names it reaches.
		"  W  REAL, INTENT(IN), OPTIONAL (:)  an argument  @PROBE.SHOW.$A1",
		"  I  INTEGER from PROBE  64A",
		"  LINE  INTEGER from PROBE  65A  65  67",
		"  W  REAL array, INTENT(IN), OPTIONAL  62D  63D  64  67",
	)

	// A specific procedure reached only through a generic name, an operator,
	// or assignment has no row where only they reach it; a WHERE construct's
	// mask, the compiler's own, isn't in the map.
	listingLacks(t, r.Listing, "ADDP TYPE(POINT) function from", "SETP subroutine from", "NORMR REAL function from", "$W")
}
