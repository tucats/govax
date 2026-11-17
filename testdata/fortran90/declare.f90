! declare.f90: Fortran 90's declarations (docs/planning/PHASE-100 - Fortran 90
! declarations and kinds.md, subtask 3): the :: form, PARAMETER with ::,
! initial values given with =, one value for every element of an array, the
! SAVE an initialization implies, DIMENSION as an attribute and as a
! statement with ::, INTENT, EXTERNAL, and SAVE ::. Kinds come with subtask 4.
program declare
  implicit none
  integer, parameter :: n = 4, m = n * 3
  character*(*), parameter :: title = "tallies"
  double precision, parameter :: half = 0.5d0
  real, dimension(n) :: weights = 0.25
  integer :: counts(n) = 0, total, i, k, j
  character*6 :: label = "bin"
  logical :: done = .false.
  real :: grid
  dimension :: grid(2, 3)
  save :: total
  real, external :: sum4

  write (*, '(1x, a, 2i3)') title, n, m
  do i = 1, m
     k = mod(i * i + i / 3, n) + 1
     counts(k) = counts(k) + 1
     call tick(total)
  end do
  write (*, '(1x, a, 4i3)') label // title, counts
  write (*, '(1x, a, i3)') "ticks:", total
  write (*, '(1x, a, f6.2)') "weighted:", sum4(weights, counts)
  do j = 1, 3
     do i = 1, 2
        grid(i, j) = i * 10 + j
     end do
  end do
  write (*, '(1x, 6f5.0)') grid
  done = total == m
  write (*, '(1x, a, l2, f5.2)') "done:", done, half
end program declare

! tick counts its calls in a variable whose initialization gives it SAVE.
subroutine tick(count)
  integer, intent(out) :: count
  integer :: calls = 0
  calls = calls + 1
  count = calls
end subroutine tick

! sum4 is the sum of four weighted counts.
real function sum4(w, c)
  real, intent(in) :: w(4)
  integer, intent(in) :: c(4)
  integer :: i
  sum4 = 0
  do i = 1, 4
     sum4 = sum4 + w(i) * c(i)
  end do
end function sum4
