! Default initialization (Fortran 95): a derived type's components given
! values that every object of the type starts with, wherever it is made: a
! main program's and a module's variables once, a subprogram's locals at each
! call (no SAVE implied), its INTENT(OUT) dummies and its value at each call,
! and what ALLOCATE makes. A component of a type with defaults has its type's
! unless it gives its own; a pointer component may start disassociated.
module shapes
  implicit none
  type point
    real :: x = 0.5, y = 1.5
  end type point
  type label
    character(len=6) :: text = 'none'
    integer :: hits = 0
    integer :: marks(3) = (/ 1, 2, 3 /)
    type(point) :: at = point(1.0, -1.0)
    type(point) :: home
    type(label), pointer :: next => null()
  end type label
  type(label), save :: shared
contains
  subroutine visit(l)
    type(label), intent(inout) :: l
    l%hits = l%hits + 1
  end subroutine visit
end module shapes

program defaults
  use shapes
  implicit none
  type(label) :: a, b(2)
  type(label), allocatable :: many(:)
  type(label), pointer :: p
  type(point) :: q = point(5.0, 6.0)
  integer :: k

  call show('A', a)
  call show('B(2)', b(2))
  print '(A,2F5.1)', ' Q', q%x, q%y
  a%hits = 9
  call visit(a)
  call show('A', a)

  allocate (many(3), stat=k)
  print '(A,I2)', ' stat', k
  call show('MANY(3)', many(3))
  allocate (p)
  call show('P', p)
  allocate (p%next)
  p%next%text = 'second'
  call show('P%NEXT', p%next)
  print '(A,L2)', ' P%NEXT%NEXT', associated(p%next%next)

  a%hits = 7
  call clear(a)
  call show('cleared', a)
  b%hits = 4
  call clear_all(b)
  print '(A,2I3)', ' cleared', b%hits
  a%hits = 5
  call maybe(a)
  call maybe()
  print '(A,I3)', ' A%HITS', a%hits

  do k = 1, 2
    call fresh(k)
  end do
  call depth(3)
  call autos(2)
  call show('MADE', made(4))
  q = origin()
  print '(A,2F5.1)', ' origin', q%x, q%y

  call visit(shared)
  call visit(shared)
  print '(1X,A6,I3)', shared%text, shared%hits

  a%hits = 8
  call tally(a)
  print '(A,I3)', ' tally', a%hits
  call tally2(5, a)
  print '(A,I3)', ' tally2', a%hits

contains

  subroutine show(what, l)
    character(len=*), intent(in) :: what
    type(label), intent(in) :: l
    print '(1X,A8,A6,I3,3I2,4F5.1,L2)', what, l%text, l%hits, l%marks, l%at, l%home, associated(l%next)
  end subroutine show

  subroutine clear(l)
    type(label), intent(out) :: l
  end subroutine clear

  subroutine clear_all(x)
    type(label), intent(out) :: x(:)
  end subroutine clear_all

  subroutine maybe(l)
    type(label), intent(out), optional :: l
    if (present(l)) then
      print '(A,I3)', ' maybe', l%hits
      l%hits = 6
    else
      print '(A)', ' maybe, absent'
    end if
  end subroutine maybe

  subroutine fresh(k)
    integer, intent(in) :: k
    type(label) :: t
    type(label), save :: kept
    t%hits = t%hits + k
    kept%hits = kept%hits + k
    print '(A,2I3)', ' fresh', t%hits, kept%hits
  end subroutine fresh

  recursive subroutine depth(n)
    integer, intent(in) :: n
    type(label) :: d
    d%hits = d%hits + n
    if (n > 1) call depth(n - 1)
    print '(A,I3)', ' depth', d%hits
  end subroutine depth

  subroutine autos(n)
    integer, intent(in) :: n
    type(label) :: w(n)
    print '(A,I3,2F5.1)', ' autos', size(w), w(n)%at%x, w(n)%home%y
  end subroutine autos

  function made(n) result(r)
    integer, intent(in) :: n
    type(label) :: r
    r%hits = r%hits + n
    r%text = 'made'
  end function made

  function origin() result(o)
    type(point) :: o
    o%y = o%y + 2.0
  end function origin

end program defaults

! An external subroutine with an entry point: each sets its own list's
! INTENT(OUT) dummy to its defaults.
subroutine tally(l)
  use shapes
  implicit none
  type(label), intent(out) :: l
  integer, intent(in) :: n
  l%hits = l%hits + 1
  return
  entry tally2(n, l)
  l%hits = l%hits + n
end subroutine tally
