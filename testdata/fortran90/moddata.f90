! A module's data in the same file as its users (docs/planning/PHASE-106 -
! Fortran 90 modules.md, subtask 2): constants, an initialized variable, an
! array, a derived type, a constant of it, a variable of it, and a COMMON
! block, shared by a main program and two external subroutines that each USE
! the module.
module consts
  implicit none
  real, parameter :: pi = 3.14159
  integer :: calls = 0
  real(8) :: table(3)
  type point
    real :: x, y
  end type point
  type(point), parameter :: unit_x = point(1.0, 0.0)
  type(point) :: origin
  integer :: hits
  common /tally/ hits
end module consts

program moddata
  use consts
  implicit none
  type(point) :: p
  integer :: i
  hits = 0
  do i = 1, 3
    call bump(i)
  end do
  print '(1X, I4)', calls
  print '(1X, 3F6.2)', table
  p = unit_x
  origin = point(p%x + 1.0, 3.0)
  call show
  print '(1X, F7.5)', pi
  print '(1X, I4)', hits
end program moddata

subroutine bump(i)
  use consts
  implicit none
  integer :: i
  calls = calls + 1
  hits = hits + 2
  table(i) = i * 1.5d0
end subroutine bump

subroutine show
  use consts
  implicit none
  print '(1X, 2F5.1)', origin%x, origin%y
end subroutine show
