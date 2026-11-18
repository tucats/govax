! operators.f90: defined operators and defined assignment
! (docs/planning/PHASE-103 - Fortran 90 procedures and interfaces.md, subtask
! 8). .DIST. is a binary defined operator on points, and .SD. on REALs, which
! binds less tightly than any other (1.0 + 2.0 .SD. 10.0 - 4.0 is 3.0 .SD.
! 6.0); .INV. a unary one, which binds more tightly (2.0 * .INV. 4.0 is 2.0 *
! 0.25). + is extended to LOGICAL operands, // to a CHARACTER and an INTEGER,
! and .AND. to INTEGERs, taking over VAX FORTRAN's bitwise meaning (12 .AND. 7
! would be 4); + of two INTEGERs is still Fortran's. ASSIGNMENT(=) gives a
! CHARACTER variable a LOGICAL value as 'yes' or 'no'. An internal procedure
! uses its host's operators.
program operators
  interface operator(.dist.)
    real function dist(p, q)
      real, intent(in) :: p(2), q(2)
    end function dist
  end interface

  interface operator(.sd.)
    real function sdist(x, y)
      real, intent(in) :: x, y
    end function sdist
  end interface

  interface operator(.inv.)
    real function inv(x)
      real, intent(in) :: x
    end function inv
  end interface

  interface operator(+)
    logical function lplus(p, q)
      logical, intent(in) :: p, q
    end function lplus
  end interface

  interface operator(//)
    character*12 function catint(s, n)
      character*(*), intent(in) :: s
      integer, intent(in) :: n
    end function catint
  end interface

  interface operator(.and.)
    integer function least(i, j)
      integer, intent(in) :: i, j
    end function least
  end interface

  interface assignment(=)
    subroutine yesno(s, l)
      character*(*), intent(out) :: s
      logical, intent(in) :: l
    end subroutine yesno
  end interface

  real a(2), b(2)
  logical t, f
  character*3 w1, w2

  a = (/ 0.0, 0.0 /)
  b = (/ 3.0, 4.0 /)
  write (*, '(1x, a, f6.2)') 'dist:', a .dist. b
  write (*, '(1x, a, f6.2)') 'sd:', 1.0 + 2.0 .sd. 10.0 - 4.0
  write (*, '(1x, a, f6.2)') 'inv:', 2.0 * .inv. 4.0
  t = .true.
  f = .false.
  write (*, '(1x, a, 3l2)') 'plus:', t + f, f + f, (f + f) + t
  write (*, '(1x, a, a, a)') 'cat: ', 'n=' // 42, '|'
  write (*, '(1x, a, 2i4)') 'and:', 12 .and. 7, 3 + 4
  w1 = t
  w2 = 1 > 2
  write (*, '(1x, a, 1x, a, 1x, a)') 'assign:', w1, w2
  call inner

contains
  subroutine inner
    character*3 w
    w = (a .dist. b) > 4.0
    write (*, '(1x, a, 1x, a)') 'inner:', w
  end subroutine inner
end program operators

real function dist(p, q)
  real, intent(in) :: p(2), q(2)
  dist = sqrt((p(1) - q(1))**2 + (p(2) - q(2))**2)
end function dist

real function sdist(x, y)
  real, intent(in) :: x, y
  sdist = abs(x - y)
end function sdist

real function inv(x)
  real, intent(in) :: x
  inv = 1.0/x
end function inv

logical function lplus(p, q)
  logical, intent(in) :: p, q
  lplus = p .or. q
end function lplus

character*12 function catint(s, n)
  character*(*), intent(in) :: s
  integer, intent(in) :: n
  catint = s // char(48 + n/10) // char(48 + mod(n, 10))
end function catint

integer function least(i, j)
  integer, intent(in) :: i, j
  least = min(i, j)
end function least

subroutine yesno(s, l)
  character*(*), intent(out) :: s
  logical, intent(in) :: l
  if (l) then
    s = 'yes'
  else
    s = 'no'
  end if
end subroutine yesno
