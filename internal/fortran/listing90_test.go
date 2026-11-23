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

// listing95 has Fortran 95's forms: PURE and ELEMENTAL procedures, a module's and an
// external one's, NULL(), and FORALL indexes, one named as a variable of the unit's is.
const listing95 = `module ops
  type cell
    integer :: n = 7
    integer, pointer :: p => null()
  end type cell
contains
  pure integer function sq(x)
    integer, intent(in) :: x
    sq = x * x
  end function sq
  elemental real function half(x)
    real, intent(in) :: x
    half = x / 2.0
  end function half
end module ops
program probe
  use ops
  interface
    pure integer function twice(x)
      integer, intent(in) :: x
    end function twice
  end interface
  integer :: i, k(4), m(4,4)
  integer, pointer :: q => null()
  real :: r(4)
  type(cell) :: c
  i = 9
  forall (i = 1:4) k(i) = sq(i) + twice(i)
  forall (i = 1:4, j = 1:4, i /= j)
    m(i, j) = i + j
  end forall
  r = half(real(k))
  q => null()
  print *, k, m, r, c%n, i
end program probe
pure integer function twice(x)
  integer, intent(in) :: x
  twice = 2 * x
end function twice`

// TestListing95 is the cross reference's Fortran 95 (docs/planning/PHASE-112 - Fortran
// 95.md, subtask 13): a PURE or ELEMENTAL procedure says so, in its module, in its own
// unit, through a USE, and through an interface body; NULL is an intrinsic function's
// row; and a FORALL's index, a name of its statement's own, has a row of its own after
// the unit's variable of its name, its mentions those of every FORALL with an index of
// the name.
func TestListing95(t *testing.T) {
	r := compileFree("PROBE", listing95, nil, withListing)
	if r.Errors > 0 {
		t.Fatal(messageList(r))
	}

	listingHas(t, r.Listing,
		"  HALF  REAL function, ELEMENTAL  11D",
		"  NULL  intrinsic  4",
		"  SQ  INTEGER function, PURE  7D",
		"  SQ  INTEGER function, PURE  7D  9A",
		"  HALF  REAL function, ELEMENTAL  11D  13A",
		"  I  INTEGER  23D  27A  34",
		"  I  INTEGER FORALL index  28A  28  29A  29  30",
		"  J  INTEGER FORALL index  29A  29  30",
		"  HALF  REAL function, ELEMENTAL from OPS  17D  32",
		"  NULL  intrinsic  24  33",
		"  SQ  INTEGER function, PURE from OPS  17D  28",
		"  TWICE  INTEGER function, PURE  28",
		"  TWICE  INTEGER function, PURE  36D  38A",
	)

	// J is only a FORALL's index: the unit has no variable of its own by that name.
	listingLacks(t, r.Listing, "J INTEGER 29")
}
