! interfaces.f90: interface blocks (docs/planning/PHASE-103 - Fortran 90
! procedures and interfaces.md, subtask 5). The main program describes four
! external procedures this file defines, so their references are checked as
! an internal procedure's are and their types are the bodies', whatever the
! program's implicit rules say (LABEL would be REAL by its letter). BISECT's
! body describes its dummy procedure F with an interface block of its own; an
! internal procedure's interface block describes DOUBLE (with RESULT, which
! the definition hasn't: the names may differ), and passes it, with no
! EXTERNAL statement, to APPLY, whose interface block makes its dummy G a
! function.
program interfaces
  interface
    real function bisect(f, lo, hi, tol)
      interface
        real function f(x)
          real, intent(in) :: x
        end function f
      end interface
      real, intent(in) :: lo, hi, tol
    end function bisect

    subroutine tally(n, total)
      integer, intent(in) :: n
      integer, intent(out) :: total
    end subroutine tally

    real function cubic(x)
      real, intent(in) :: x
    end function cubic

    character*5 function label(i)
      integer i
    end function label

    subroutine shout(s)
      character*(*), intent(in) :: s
    end subroutine shout
  end interface

  integer t
  call tally(10, t)
  write (*, '(1x, a, i5)') 'tally:', t
  write (*, '(1x, a, f8.4)') 'root:', bisect(cubic, 0.0, 2.0, 1.0e-5)
  write (*, '(1x, 3a)') '[', label(7), ']'
  call shout('interfaces')
  call twice(3)

contains
  subroutine twice(n)
    integer n
    interface
      integer function double(k) result(d)
        integer k
      end function double
      subroutine apply(g, n)
        interface
          integer function g(k)
            integer k
          end function g
        end interface
        integer n
      end subroutine apply
    end interface
    write (*, '(1x, a, i4)') 'double:', double(n)
    call apply(double, n)
  end subroutine twice
end program interfaces

real function bisect(f, lo, hi, tol)
  interface
    real function f(x)
      real, intent(in) :: x
    end function f
  end interface
  real, intent(in) :: lo, hi, tol
  real a, b, m
  a = lo
  b = hi
  do while (b - a > tol)
    m = (a + b) / 2.0
    if (f(a) * f(m) <= 0.0) then
      b = m
    else
      a = m
    end if
  end do
  bisect = (a + b) / 2.0
end function bisect

subroutine tally(n, total)
  integer, intent(in) :: n
  integer, intent(out) :: total
  total = 0
  do i = 1, n
    total = total + i
  end do
end subroutine tally

real function cubic(x)
  real, intent(in) :: x
  cubic = x**3 - 2.0
end function cubic

character*5 function label(i)
  integer i
  label = 'item' // char(ichar('0') + i)
end function label

subroutine shout(s)
  character*(*), intent(in) :: s
  write (*, '(1x, a, i3)') s, len(s)
end subroutine shout

integer function double(k)
  integer k
  double = 2 * k
end function double

subroutine apply(g, n)
  interface
    integer function g(k)
      integer k
    end function g
  end interface
  integer n
  write (*, '(1x, a, i4)') 'applied:', g(n) + 1
end subroutine apply
