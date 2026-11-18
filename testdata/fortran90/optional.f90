! optional.f90: keyword arguments, OPTIONAL dummies, PRESENT, and INTENT
! (docs/planning/PHASE-103 - Fortran 90 procedures and interfaces.md,
! subtask 6). The main program describes REPORT, POLY, FILL, TOTAL, and HOST
! with an interface block, and calls them with arguments left out, given by
! keyword in any order, or both. RELAY, an internal procedure, passes its own
! OPTIONAL dummy on to REPORT, absent or not. TOTAL's OPTIONAL dummy is an
! adjustable array, never touched when absent. HOST's internal procedure asks
! PRESENT of its host's dummy.
program optional
  interface
    subroutine report(x, n, label, scale)
      real, intent(in) :: x
      integer, intent(in), optional :: n
      character*(*), intent(in), optional :: label
      real, intent(in), optional :: scale
    end subroutine report

    real function poly(x, a, b, c)
      real, intent(in) :: x
      real, intent(in), optional :: a, b, c
    end function poly

    subroutine fill(v, m, val)
      integer, intent(in) :: m
      real, intent(out) :: v(m)
      real, intent(in), optional :: val
    end subroutine fill

    real function total(v, m, w)
      integer, intent(in) :: m
      real, intent(in) :: v(m)
      real, intent(in), optional :: w(m)
    end function total

    subroutine host(k)
      integer, intent(in), optional :: k
    end subroutine host
  end interface

  real u(4), w(4)

  call report(1.5)
  call report(2.5, 3)
  call report(3.5, label='keyed')
  call report(x=4.5, scale=2.0, n=2)
  call report(5.5, 1, 'all', 10.0)
  write (*, '(1x, a, 4f6.1)') 'poly:', poly(2.0), poly(2.0, 1.0), poly(2.0, c=1.0), &
      poly(x=2.0, b=3.0, a=1.0)

  call fill(u, 4)
  call fill(val=7.0, m=4, v=w)
  write (*, '(1x, a, 8f5.1)') 'fill:', u, w
  write (*, '(1x, a, 2f6.1)') 'total:', total(u, 4), total(u, 4, w)

  call relay(1.0)
  call relay(2.0, 5)
  call host(9)
  call host()

contains
  subroutine relay(x, n)
    real, intent(in) :: x
    integer, intent(in), optional :: n

    call report(x, n)
    call report(x, n, label='relayed')
  end subroutine relay
end program optional

subroutine report(x, n, label, scale)
  real, intent(in) :: x
  integer, intent(in), optional :: n
  character*(*), intent(in), optional :: label
  real, intent(in), optional :: scale
  integer times
  real s
  character*8 name

  times = 1
  if (present(n)) times = n
  s = 1.0
  if (present(scale)) s = scale
  name = '-'
  if (present(label)) name = label
  write (*, '(1x, a8, f6.1, i3)') name, x*s, times
end subroutine report

real function poly(x, a, b, c)
  real, intent(in) :: x
  real, intent(in), optional :: a, b, c

  poly = 0.0
  if (present(a)) poly = a
  if (present(b)) poly = poly + b*x
  if (present(c)) poly = poly + c*x*x
end function poly

subroutine fill(v, m, val)
  integer, intent(in) :: m
  real, intent(out) :: v(m)
  real, intent(in), optional :: val
  integer i

  do i = 1, m
    if (present(val)) then
      v(i) = val
    else
      v(i) = i
    end if
  end do
end subroutine fill

real function total(v, m, w)
  integer, intent(in) :: m
  real, intent(in) :: v(m)
  real, intent(in), optional :: w(m)
  integer i

  total = 0.0
  do i = 1, m
    total = total + v(i)
    if (present(w)) total = total + w(i)
  end do
end function total

subroutine host(k)
  integer, intent(in), optional :: k

  call inner

contains
  subroutine inner
    if (present(k)) then
      write (*, '(1x, a, i3)') 'host has', k
    else
      write (*, '(1x, a)') 'host has none'
    end if
  end subroutine inner
end subroutine host
