! PURE and ELEMENTAL procedures: module, internal, and external, with an
! interface block; an elemental reference scalarized over arrays.
module purem
  implicit none
contains
  pure integer function sq(x)
    integer, intent(in) :: x
    sq = x * x
  end function sq
  elemental real function clip(x, lo, hi)
    real, intent(in) :: x, lo, hi
    clip = max(lo, min(hi, x))
  end function clip
  elemental subroutine swap(a, b)
    integer, intent(inout) :: a, b
    integer :: t
    t = a; a = b; b = t
  end subroutine swap
end module purem

program pure
  use purem
  implicit none
  interface
    pure function twice(x) result(y)
      real, intent(in) :: x
      real :: y
    end function twice
  end interface
  integer :: i, p(4), q(4)
  real :: r(5)
  r = (/ -2.0, -0.5, 0.5, 1.5, 3.0 /)
  print '(5F6.2)', clip(r, -1.0, 1.0)
  print '(5F6.2)', clip(r, 0.0, (/1.,1.,1.,2.,2./))
  p = (/ 1, 2, 3, 4 /); q = (/ 5, 6, 7, 8 /)
  call swap(p, q)
  print '(8I3)', p, q
  forall (i = 1:4) p(i) = sq(i)
  print '(4I4)', p
  print '(F6.2)', twice(1.25)
  print '(4I4)', inc(p)
contains
  elemental integer function inc(k)
    integer, intent(in) :: k
    inc = k + 1
  end function inc
end program pure

pure function twice(x) result(y)
  real, intent(in) :: x
  real :: y
  y = 2.0 * x
end function twice
