! PURE procedures (Phase 112, subtask 6): module, internal, and external ones
! with an interface block, a PURE subroutine changing only its INTENT(OUT)
! and INTENT(INOUT) dummies, a PURE recursive function, a PURE function
! passed to a PURE dummy procedure, internal files in a PURE procedure, and
! PURE functions as specification functions: an automatic array's bound and
! a CHARACTER length, computed at each entry.
module puremod
  implicit none
  integer, parameter :: scale = 10
contains
  pure integer function sq(x)
    integer, intent(in) :: x
    sq = x * x
  end function sq

  pure integer function width(n)
    integer, intent(in) :: n
    width = 2 * n + 1
  end function width

  pure recursive integer function fact(n) result(r)
    integer, intent(in) :: n
    if (n <= 1) then
      r = 1
    else
      r = n * fact(n - 1)
    end if
  end function fact

  pure subroutine minmax(v, lo, hi)
    integer, intent(in) :: v(:)
    integer, intent(out) :: lo, hi
    integer :: i
    lo = v(1)
    hi = v(1)
    do i = 2, size(v)
      lo = min(lo, v(i))
      hi = max(hi, v(i))
    end do
  end subroutine minmax

  pure subroutine bump(k, by)
    integer, intent(inout) :: k
    integer, intent(in) :: by
    k = k + by * scale
  end subroutine bump

  ! A PURE procedure may write an internal file.
  pure function label(n) result(text)
    integer, intent(in) :: n
    character(len=8) :: text
    write (text, '(A,I3.3)') 'item', n
  end function label

  ! g must be PURE: the interface says so.
  pure integer function total(g, n)
    interface
      pure integer function g(k)
        integer, intent(in) :: k
      end function g
    end interface
    integer, intent(in) :: n
    integer :: i
    total = 0
    do i = 1, n
      total = total + g(i)
    end do
  end function total
end module puremod

program pures
  use puremod
  implicit none
  interface
    pure function twice(x) result(y)
      real, intent(in) :: x
      real :: y
    end function twice
  end interface
  integer :: lo, hi, k
  integer :: v(6) = (/ 4, -2, 9, 0, 7, 3 /)

  print '(A,3I5)', ' squares', sq(3), sq(-4), sq(sq(2))
  print '(A,I8)', ' 7! =', fact(7)
  call minmax(v, lo, hi)
  print '(A,2I4)', ' min, max', lo, hi
  k = 5
  call bump(k, 3)
  print '(A,I5)', ' bumped', k
  print '(2A)', ' ', label(42)
  print '(A,I6)', ' sum of squares', total(sq, 4)
  print '(A,F6.2)', ' twice', twice(1.25)
  print '(A,I4)', ' cubed', cube(3)
  call show(2)
  call show(4)
contains
  pure integer function cube(n)
    integer, intent(in) :: n
    cube = n * sq(n)
  end function cube

  ! s's length and w's extent are specification functions' values.
  subroutine show(n)
    integer, intent(in) :: n
    character(len=width(n)) :: s
    integer :: w(width(n) - 1)
    integer :: i
    s = repeat('*', len(s))
    w = (/ (sq(i), i = 1, size(w)) /)
    print '(1X,A,1X,I2,1X,10I4)', s, size(w), w
  end subroutine show
end program pures

pure function twice(x) result(y)
  real, intent(in) :: x
  real :: y
  y = 2.0 * x
end function twice
