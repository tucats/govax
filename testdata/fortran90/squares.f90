! squares.f90: loops and decisions in free form (docs/planning/PHASE-99 -
! Fortran 90.md, subtask 6): DO ... END DO, DO WHILE, a block IF with ELSE IF
! written with >= and <, labels first on their lines, and a function and a
! subroutine whose ENDs name them. All of it is FORTRAN-77 (with DEC's END DO
! and DO WHILE, which Fortran 90 made standard); only the spelling is new.
program squares
  implicit none
  integer n, total, k, cube
  double precision root

  write (*, '(1x, a)') "  n  n**2  n**3"
  do n = 1, 5
     write (*, 100) n, n*n, cube(n)
  end do
100 format (1x, i3, i6, i6)

  ! The first square past 50, by DO WHILE.
  n = 1
  do while (n*n <= 50)
     n = n + 1
  end do
  write (*, 110) n, n*n
110 format (1x, "first square past 50: ", i2, "**2 = ", i3)

  ! A sum with a labeled DO and its CONTINUE, as FORTRAN-77 writes it.
  total = 0
  do 200 k = 2, 10, 2
     total = total + k
200 continue
  write (*, 120) total
120 format (1x, "2 + 4 + ... + 10 = ", i3)

  do k = -1, 2
     call classify(k)
  end do

  ! Newton's method for the square root of 2, in DOUBLE PRECISION.
  root = 1d0
  do k = 1, 6
     root = (root + 2d0/root) / 2d0
  end do
  write (*, 130) root
130 format (1x, "sqrt(2) = ", f12.10)
end program squares

integer function cube(n)
  integer n
  cube = n**3
end function cube

subroutine classify(k)
  integer k
  if (k < 0) then
     write (*, '(1x, i2, a)') k, " is negative"
  else if (k == 0) then
     write (*, '(1x, i2, a)') k, " is zero"
  else if (k >= 2) then
     write (*, '(1x, i2, a)') k, " is two or more"
  else
     write (*, '(1x, i2, a)') k, " is one"
  end if
end subroutine classify
