! numbers.f90: Phase 103's features together (docs/planning/PHASE-103 -
! Fortran 90 procedures and interfaces.md, subtask 9). .GCD., .LCM., and
! .CHOOSE. are defined operators on INTEGERs (GCD a RECURSIVE function with
! RESULT); SHOW is a generic name for an INTEGER's and a REAL's display, each
! with an OPTIONAL width or number of digits, given by keyword or not; PASCAL,
! an internal procedure, fills its host's array with a row of Pascal's triangle
! for its host's loop variable N, and its own internal-to-the-host sibling
! TRIANGLE's total checks every row.
program numbers
  interface operator(.gcd.)
    recursive integer function gcd(m, n) result(g)
      integer, intent(in) :: m, n
    end function gcd
  end interface

  interface operator(.lcm.)
    integer function lcm(m, n)
      integer, intent(in) :: m, n
    end function lcm
  end interface

  interface operator(.choose.)
    integer function choose(n, k)
      integer, intent(in) :: n, k
    end function choose
  end interface

  interface show
    subroutine showi(label, n, width)
      character*(*), intent(in) :: label
      integer, intent(in) :: n
      integer, intent(in), optional :: width
    end subroutine showi
    subroutine showr(label, x, digits)
      character*(*), intent(in) :: label
      real, intent(in) :: x
      integer, intent(in), optional :: digits
    end subroutine showr
  end interface

  integer row(0:6), n, sums

  call show('gcd', 84 .gcd. 36)
  call show('lcm', 4 .lcm. 6 .lcm. 10)
  call show('wide', 7, width=4)
  call show(label='pi', x=3.14159, digits=2)
  call show('e', 2.71828)

  sums = 0
  do n = 0, 6
    call pascal
    sums = sums + triangle()
  end do
  call show('sums', sums)

contains
  subroutine pascal
    integer j

    do j = 0, n
      row(j) = n .choose. j
    end do
    write (*, '(1x, 7i4)') (row(j), j = 0, n)
  end subroutine pascal

  integer function triangle()
    integer j

    triangle = 0
    do j = 0, n
      triangle = triangle + row(j)
    end do
  end function triangle
end program numbers

recursive integer function gcd(m, n) result(g)
  integer, intent(in) :: m, n

  if (n == 0) then
    g = m
  else
    g = gcd(n, mod(m, n))
  end if
end function gcd

integer function lcm(m, n)
  integer, intent(in) :: m, n
  interface operator(.gcd.)
    recursive integer function gcd(m, n) result(g)
      integer, intent(in) :: m, n
    end function gcd
  end interface

  lcm = m/(m .gcd. n)*n
end function lcm

integer function choose(n, k)
  integer, intent(in) :: n, k
  integer i

  choose = 1
  do i = 1, k
    choose = choose*(n - i + 1)/i
  end do
end function choose

subroutine showi(label, n, width)
  character*(*), intent(in) :: label
  integer, intent(in) :: n
  integer, intent(in), optional :: width
  character*12 form
  integer w

  w = 6
  if (present(width)) w = width
  form = '(1x, a, i' // char(48 + w) // ')'
  write (*, form) label, n
end subroutine showi

subroutine showr(label, x, digits)
  character*(*), intent(in) :: label
  real, intent(in) :: x
  integer, intent(in), optional :: digits
  character*14 form
  integer d

  d = 4
  if (present(digits)) d = digits
  form = '(1x, a, f10.' // char(48 + d) // ')'
  write (*, form) label, x
end subroutine showr
