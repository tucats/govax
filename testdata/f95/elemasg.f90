! An elemental defined assignment, in an array assignment and in WHERE.
module em
  type t
    integer :: v
  end type
  interface assignment(=)
    module procedure asg
  end interface
contains
  elemental subroutine asg(a, b)
    type(t), intent(out) :: a
    integer, intent(in) :: b
    a%v = 2*b
  end subroutine asg
end module em
program elemasg
  use em
  type(t) :: x(4)
  integer :: n(4) = (/1,2,3,4/)
  x = n
  where (n > 2) x = n + 10
  print *, x%v
end program elemasg
