! The small ones: END INTERFACE with its generic name, comments in
! namelist input, a PURE function in a specification expression.
module miscm
  implicit none
  interface norm
    module procedure inorm, rnorm
  end interface norm
contains
  pure integer function inorm(x)
    integer, intent(in) :: x(:)
    inorm = sum(abs(x))
  end function inorm
  pure real function rnorm(x)
    real, intent(in) :: x(:)
    rnorm = sum(abs(x))
  end function rnorm
  pure integer function width(n)
    integer, intent(in) :: n
    width = 2 * n
  end function width
end module miscm

program misc95
  use miscm
  implicit none
  integer :: k, j
  namelist /grp/ k, j
  print *, norm((/ 1, -2, 3 /)), norm((/ 1.5, -2.5 /))
  call show(3)
  read (*, nml=grp)
  print *, k, j
contains
  subroutine show(n)
    integer, intent(in) :: n
    character(len=width(n)) :: s
    s = repeat('*', len(s))
    print *, s
  end subroutine show
end program misc95
