! generics.f90: generic interfaces (docs/planning/PHASE-103 - Fortran 90
! procedures and interfaces.md, subtask 7). SWAP is three subroutines told
! apart by type, TWICE three functions told apart by kind, NORM two told apart
! by rank, and MAX extends the intrinsic function to LOGICAL values while
! MAX(3, 7) stays the intrinsic's. SHOW, an internal procedure, uses its host's
! SWAP and adds a fourth specific for LOGICAL values in itself alone.
program generics
  interface swap
    subroutine swapr(a, b)
      real, intent(inout) :: a, b
    end subroutine swapr
    subroutine swapi(i, j)
      integer, intent(inout) :: i, j
    end subroutine swapi
    subroutine swapc(s, t)
      character*(*), intent(inout) :: s, t
    end subroutine swapc
  end interface

  interface twice
    integer function twicei(n)
      integer, intent(in) :: n
    end function twicei
    real function twicer(x)
      real, intent(in) :: x
    end function twicer
    double precision function twiced(d)
      double precision, intent(in) :: d
    end function twiced
  end interface

  interface norm
    real function norms(x)
      real, intent(in) :: x
    end function norms
    real function normv(v)
      real, intent(in) :: v(3)
    end function normv
  end interface

  interface max
    logical function maxl(p, q)
      logical, intent(in) :: p, q
    end function maxl
  end interface

  real x, y, v(3)
  integer m, n
  character*4 s, t
  double precision d

  x = 1.5
  y = 2.5
  m = 3
  n = 4
  s = 'left'
  t = 'rite'
  call swap(x, y)
  call swap(j=m, i=n)
  call swap(s, t)
  write (*, '(1x, a, 2f5.1, 2i3, 1x, a, 1x, a)') 'swapped:', x, y, m, n, s, t

  d = 1.25d0
  write (*, '(1x, a, i4, f6.2, f8.4)') 'twice:', twice(21), twice(1.25), twice(d)

  v = (/ 3.0, 4.0, 12.0 /)
  write (*, '(1x, a, 2f6.1)') 'norm:', norm(-2.0), norm(v)
  write (*, '(1x, a, 2l2, i3)') 'max:', max(.false., .true.), max(.false., .false.), max(3, 7)

  call show

contains
  subroutine show
    interface swap
      subroutine swapl(p, q)
        logical, intent(inout) :: p, q
      end subroutine swapl
    end interface
    logical p, q
    integer i, j

    p = .true.
    q = .false.
    i = 1
    j = 2
    call swap(p, q)
    call swap(i, j)
    write (*, '(1x, a, 2l2, 2i3)') 'show:', p, q, i, j
  end subroutine show
end program generics

subroutine swapr(a, b)
  real, intent(inout) :: a, b
  real t
  t = a
  a = b
  b = t
end subroutine swapr

subroutine swapi(i, j)
  integer, intent(inout) :: i, j
  integer t
  t = i
  i = j
  j = t
end subroutine swapi

subroutine swapc(s, t)
  character*(*), intent(inout) :: s, t
  character*80 hold
  hold = s
  s = t
  t = hold
end subroutine swapc

subroutine swapl(p, q)
  logical, intent(inout) :: p, q
  logical t
  t = p
  p = q
  q = t
end subroutine swapl

integer function twicei(n)
  integer, intent(in) :: n
  twicei = 2*n
end function twicei

real function twicer(x)
  real, intent(in) :: x
  twicer = 2.0*x
end function twicer

double precision function twiced(d)
  double precision, intent(in) :: d
  twiced = 2.0d0*d
end function twiced

real function norms(x)
  real, intent(in) :: x
  norms = abs(x)
end function norms

real function normv(v)
  real, intent(in) :: v(3)
  normv = sqrt(v(1)**2 + v(2)**2 + v(3)**2)
end function normv

logical function maxl(p, q)
  logical, intent(in) :: p, q
  maxl = p .or. q
end function maxl
