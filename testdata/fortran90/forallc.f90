! The FORALL construct (Phase 112, subtask 9): each statement done for every
! tuple before the next, a mask computed once though a statement changes what
! it reads, nested FORALLs (a triangle, a mask on the outer index, a mask the
! nested statements change), WHERE and ELSEWHERE inside, pointer assignments,
! a PURE function and a PURE defined assignment, and statements whose values
! go through temporaries.
module money
  implicit none
  type cents
    integer :: c
  end type cents
  interface assignment(=)
    module procedure from_dollars
  end interface
contains
  pure subroutine from_dollars(m, d)
    type(cents), intent(out) :: m
    integer, intent(in) :: d
    m%c = 100 * d
  end subroutine from_dollars

  pure integer function twice(k)
    integer, intent(in) :: k
    twice = 2 * k
  end function twice
end module money

program forallc
  use money
  implicit none
  type ref
    integer, pointer :: p
  end type ref
  integer :: a(6), b(6), v(8), w(8), i, j
  integer :: tri(4,4), g(3,3), h(3,3), m(3,3)
  real :: r(4,4)
  integer, target :: obj(6)
  type(ref) :: refs(6)
  type(cents) :: purse(3)

  ! The mask is the values as the FORALL starts: B gets A's new value only
  ! where A was positive.
  a = (/ 3, -1, 4, -1, 5, -9 /)
  b = 0
  forall (i = 1:6, a(i) > 0)
    a(i) = -a(i)
    b(i) = a(i) + 100
  end forall
  print '(6I5)', a
  print '(6I5)', b

  ! Each statement wholly before the next: W sees V already shifted.
  v = (/ (i, i = 1, 8) /)
  w = 0
  shift: forall (i = 2:8)
    v(i) = v(i-1)
    w(i) = v(i) * 10
  end forall shift
  print '(8I4)', v
  print '(8I4)', w

  ! A triangle: the inner bounds use the outer index.
  tri = 0
  forall (i = 1:4)
    forall (j = 1:i) tri(i,j) = 10*i + j
  end forall
  print '(4I4)', (tri(i,:), i = 1, 4)

  ! A mask on the outer index, then a WHERE over each row.
  r = 0.0
  forall (i = 1:4)
    forall (j = 1:4, j > i) r(i,j) = 10.0*i + j
    where (r(i,:) == 0.0) r(i,:) = -1.0
  end forall
  print '(4F6.1)', (r(i,:), i = 1, 4)

  ! A WHERE with ELSEWHERE in each tuple.
  forall (i = 1:4)
    where (r(i,:) > 0.0)
      r(i,:) = r(i,:) / 2.0
    elsewhere
      r(i,:) = 0.0
    end where
  end forall
  print '(4F6.1)', (r(i,:), i = 1, 4)

  ! A nested mask its own statements change: H is set where G was positive.
  g = reshape((/ 1, -2, 3, -4, 5, -6, 7, -8, 9 /), (/ 3, 3 /))
  h = 0
  forall (i = 1:3)
    forall (j = 1:3, g(i,j) > 0)
      g(i,j) = 0
      h(i,j) = 1
    end forall
  end forall
  print '(3I3)', (g(i,:), i = 1, 3)
  print '(3I3)', (h(i,:), i = 1, 3)

  ! A transpose in place, through temporaries, in a nested FORALL.
  m = reshape((/ (i, i = 1, 9) /), (/ 3, 3 /))
  forall (i = 1:3)
    forall (j = 1:3) m(i,j) = m(j,i)
  end forall
  print '(3I3)', (m(i,:), i = 1, 3)

  ! Pointer assignments, a statement and in a construct, and NULL().
  obj = (/ (10*i, i = 1, 6) /)
  forall (i = 1:6) refs(i)%p => obj(7-i)
  print '(6I4)', (refs(i)%p, i = 1, 6)
  forall (i = 1:6)
    refs(i)%p => obj(i)
    obj(i) = obj(i) + i
  end forall
  print '(6I4)', (refs(i)%p, i = 1, 6)
  forall (i = 1:6, mod(i, 2) == 0) refs(i)%p => null()
  print '(6L2)', (associated(refs(i)%p), i = 1, 6)

  ! A PURE function and a PURE defined assignment.
  forall (i = 1:3)
    purse(i) = twice(i)
    m(i,1) = twice(purse(i)%c)
  end forall
  print '(3I6)', purse%c
  print '(3I6)', m(:,1)

  ! Empty index sets do nothing.
  forall (i = 1:0)
    a(i) = 99
    forall (j = 1:3) m(j,1) = 99
  end forall
  forall (i = 1:3)
    forall (j = 3:1) m(i,j) = 99
  end forall
  print '(6I5)', a
  print '(3I6)', m(:,1)
end program forallc
